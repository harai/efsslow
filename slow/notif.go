package slow

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/iovisor/gobpf/bcc"
	"github.com/rakyll/statik/fs"

	// Load static assets
	_ "github.com/harai/efsslow/statik"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// go:generate statik -src=c

var (
	log *zap.Logger
)

const (
	cTaskCommLen          = 16
	cSlowPointCount       = 48
	cCallOrderCount       = 128
	cDnameInlineLen       = 32
	cNFS4StateidOtherSize = 12
	cNFSMaxFHSize         = 128
)

// Config configures parameters to filter what to be notified.
type Config struct {
	SlowThresholdMS uint
	SampleRatio     uint
	FileName        string
	BpfDebug        uint
	Debug           bool
	Quit            bool
	Log             *zap.Logger
}

func unpackSource(name string) string {
	sfs, err := fs.New()
	if err != nil {
		log.Panic("embedded FS not found", zap.Error(err))
	}

	r, err := sfs.Open("/" + name)
	if err != nil {
		log.Panic("embedded file not found", zap.Error(err))
	}
	defer r.Close()

	contents, err := ioutil.ReadAll(r)
	if err != nil {
		log.Panic("failed to read embedded file", zap.Error(err))
	}

	return string(contents)
}

var source string = unpackSource("trace.c")

func configNfs4FileOpenTrace(m *bcc.Module) error {
	kprobe, err := m.LoadKprobe("enter__nfs4_file_open")
	if err != nil {
		return err
	}

	if err := m.AttachKprobe("nfs4_file_open", kprobe, -1); err != nil {
		return err
	}

	kretprobe, err := m.LoadKprobe("return__nfs4_file_open")
	if err != nil {
		return err
	}

	if err := m.AttachKretprobe("nfs4_file_open", kretprobe, -1); err != nil {
		return err
	}

	return nil
}

func addPoint(m *bcc.Module, fnName string) {
	kprobe, err := m.LoadKprobe("enter__" + fnName)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to config %s trace", fnName), zap.Error(err))
	}

	if err := m.AttachKprobe(fnName, kprobe, -1); err != nil {
		log.Fatal(fmt.Sprintf("failed to config %s trace", fnName), zap.Error(err))
	}

	kretprobe, err := m.LoadKprobe("return__" + fnName)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to config %s trace", fnName), zap.Error(err))
	}

	if err := m.AttachKretprobe(fnName, kretprobe, -1); err != nil {
		log.Fatal(fmt.Sprintf("failed to config %s trace", fnName), zap.Error(err))
	}
}

func configTrace(m *bcc.Module, receiverChan chan []byte) *bcc.PerfMap {
	if err := configNfs4FileOpenTrace(m); err != nil {
		log.Fatal("failed to config nfs4_file_open trace", zap.Error(err))
	}

	//
	//
	addPoint(m, "nfs4_atomic_open")
	addPoint(m, "nfs4_client_recover_expired_lease")
	addPoint(m, "nfs4_wait_clnt_recover")
	addPoint(m, "nfs_put_client")
	addPoint(m, "nfs4_opendata_alloc")
	addPoint(m, "nfs4_run_open_task")
	addPoint(m, "_nfs4_proc_open_confirm")
	addPoint(m, "_nfs4_opendata_to_nfs4_state")
	addPoint(m, "nfs4_get_open_state")
	addPoint(m, "__nfs4_find_state_byowner")
	addPoint(m, "update_open_stateid")
	addPoint(m, "nfs_state_log_update_open_stateid")
	addPoint(m, "prepare_to_wait")
	addPoint(m, "finish_wait")
	addPoint(m, "nfs4_state_mark_reclaim_nograce")
	addPoint(m, "nfs4_schedule_state_manager") // Check if NFS_STATE_RECLAIM_NOGRACE flag is set.
	addPoint(m, "nfs_state_log_update_open_stateid")
	addPoint(m, "update_open_stateflags")

	addPoint(m, "nfs_wait_bit_killable")
	//
	//  nfs4_atomic_open
	//      nfs4_do_open (N/S)
	//          _nfs4_do_open (N/S)
	//              nfs4_get_state_owner
	//              nfs4_client_recover_expired_lease
	//                  nfs4_wait_clnt_recover
	//                      wait_on_bit_action (N/S)
	//                          out_of_line_wait_on_bit (Conditional)
	//                              __wait_on_bit
	//                                  prepare_to_wait
	//                                  nfs_wait_bit_killable (Callback) <== This function call could be slow (result/)
	//                                  finish_wait
	//                      nfs_put_client
	//                  nfs4_schedule_state_manager
	//              nfs4_opendata_alloc
	//              _nfs4_open_and_get_state (N/S)
	//                  _nfs4_proc_open (N/S)
	//                      nfs4_run_open_task
	//                          rpc_run_task
	//                          rpc_wait_for_completion_task (N/S)
	//                              __rpc_wait_for_completion_task
	//                      nfs_fattr_map_and_free_names
	//                      update_changeattr (Conditional)
	//                      _nfs4_proc_open_confirm (Conditional)
	//                      nfs4_proc_getattr (Conditional)
	//                  _nfs4_opendata_to_nfs4_state
	//                      nfs4_opendata_find_nfs4_state (N/S)
	//                          nfs4_opendata_get_inode (N/S)
	//                          nfs4_get_open_state (Conditional)
	//                            __nfs4_find_state_byowner
	//                      nfs4_opendata_check_deleg (N/S)
	//                          nfs_inode_set_delegation (Conditional)
	//                          nfs_inode_reclaim_delegation (Conditional)
	//                      update_open_stateid (Conditional)
	//                          nfs_state_set_open_stateid (N/S)
	//                              nfs_set_open_stateid_locked (N/S)
	//                                  nfs_need_update_open_stateid (N/S)
	//                                      nfs_state_log_update_open_stateid (Conditional)
	//                                  prepare_to_wait
	//                                  finish_wait
	//                                  nfs_test_and_clear_all_open_stateid (Conditional, N/S)
	//                                      nfs4_state_mark_reclaim_nograce <== This causes
	//                                  nfs_state_log_update_open_stateid (Conditional)
	//                          nfs_mark_delegation_referenced (Conditional)
	//                          update_open_stateflags (Conditional)
	//                          nfs4_schedule_state_manager (Conditional)
	//                      nfs_release_seqid
	//                  pnfs_parse_lgopen
	//                  nfs4_opendata_access (N/S)
	//                  nfs_inode_attach_open_context (Conditional)
	//
	//

	table := bcc.NewTable(m.TableId("events"), m)

	perfMap, err := bcc.InitPerfMap(table, receiverChan, nil)
	if err != nil {
		log.Fatal("Failed to init perf map", zap.Error(err))
	}

	return perfMap
}

func generateSource(config *Config) string {
	rep := strings.NewReplacer(
		"/*SLOW_THRESHOLD_MS*/",
		strconv.FormatUint(uint64(config.SlowThresholdMS), 10),
		"/*SAMPLE_RATIO*/",
		strconv.FormatUint(uint64(config.SampleRatio), 10),
		"/*FILE_NAME*/",
		config.FileName,
	)
	return rep.Replace(source)
}

var stateFlags = []string{
	"LK_STATE_IN_USE",
	"NFS_DELEGATED_STATE",         // Current stateid is delegation
	"NFS_OPEN_STATE",              // OPEN stateid is set
	"NFS_O_RDONLY_STATE",          // OPEN stateid has read-only state
	"NFS_O_WRONLY_STATE",          // OPEN stateid has write-only state
	"NFS_O_RDWR_STATE",            // OPEN stateid has read/write state
	"NFS_STATE_RECLAIM_REBOOT",    // OPEN stateid server rebooted
	"NFS_STATE_RECLAIM_NOGRACE",   // OPEN stateid needs to recover state
	"NFS_STATE_POSIX_LOCKS",       // Posix locks are supported
	"NFS_STATE_RECOVERY_FAILED",   // OPEN stateid state recovery failed
	"NFS_STATE_MAY_NOTIFY_LOCK",   // server may CB_NOTIFY_LOCK
	"NFS_STATE_CHANGE_WAIT",       // A state changing operation is outstanding
	"NFS_CLNT_DST_SSC_COPY_STATE", // dst server open state on client*/
}

var outputReasonFlags = []string{
	"EFSS_OUTPUT_SAMPLE",
	"EFSS_OUTPUT_SLOW",
	"EFSS_OUTPUT_SEQID",
	"EFSS_OUTPUT_RECLAIM_NOGRACE",
	"EFSS_OUTPUT_FILE",
}

var fmodeFlags = []string{
	"FMODE_READ",            // file is open for reading
	"FMODE_WRITE",           // file is open for writing
	"FMODE_LSEEK",           // file is seekable
	"FMODE_PREAD",           // file can be accessed using pread
	"FMODE_PWRITE",          // file can be accessed using pwrite
	"FMODE_EXEC",            // File is opened for execution with sys_execve / sys_uselib
	"FMODE_NDELAY",          // File is opened with O_NDELAY (only set for block devices)
	"FMODE_EXCL",            // File is opened with O_EXCL (only set for block devices)
	"FMODE_WRITE_IOCTL",     // File is opened using open(.., 3, ..) and is writeable only for ioctls (specialy hack for floppy.c)
	"FMODE_32BITHASH",       // 32bit hashes as llseek() offset (for directories)
	"FMODE_64BITHASH",       // 64bit hashes as llseek() offset (for directories)
	"FMODE_NOCMTIME",        // Don't update ctime and mtime. Currently a special hack for the XFS open_by_handle ioctl, but we'll hopefully graduate it to a proper O_CMTIME flag supported by open(2) soon.
	"FMODE_RANDOM",          // Expect random access pattern
	"FMODE_UNSIGNED_OFFSET", // File is huge (eg. /dev/kmem): treat loff_t as unsigned
	"FMODE_PATH",            // File is opened with O_PATH; almost nothing can be done with it
	"FMODE_ATOMIC_POS",      // File needs atomic accesses to f_pos
	"FMODE_WRITER",          // Write access to underlying fs
	"FMODE_CAN_READ",        // Has read method(s)
	"FMODE_CAN_WRITE",       // Has write method(s)
	"FMODE_OPENED",          //
	"FMODE_CREATED",         //
	"FMODE_STREAM",          // File is stream-like
	"FMODE_NONOTIFY",        // File was opened by fanotify and shouldn't generate fanotify events
	"FMODE_NOWAIT",          // File is capable of returning -EAGAIN if I/O will block
	"FMODE_NEED_UNMOUNT",    // File represents mount that needs unmounting
	"FMODE_NOACCOUNT",       // File does not contribute to nr_files count
}

var clientStates = []string{
	"NFS4CLNT_MANAGER_RUNNING",
	"NFS4CLNT_CHECK_LEASE",
	"NFS4CLNT_LEASE_EXPIRED",
	"NFS4CLNT_RECLAIM_REBOOT",
	"NFS4CLNT_RECLAIM_NOGRACE",
	"NFS4CLNT_DELEGRETURN",
	"NFS4CLNT_SESSION_RESET",
	"NFS4CLNT_LEASE_CONFIRM",
	"NFS4CLNT_SERVER_SCOPE_MISMATCH",
	"NFS4CLNT_PURGE_STATE",
	"NFS4CLNT_BIND_CONN_TO_SESSION",
	"NFS4CLNT_MOVED",
	"NFS4CLNT_LEASE_MOVED",
	"NFS4CLNT_DELEGATION_EXPIRED",
	"NFS4CLNT_RUN_MANAGER",
	"NFS4CLNT_DELEGRETURN_RUNNING",
}

var claims = []string{
	"NFS4_OPEN_CLAIM_NULL",
	"NFS4_OPEN_CLAIM_PREVIOUS",
	"NFS4_OPEN_CLAIM_DELEGATE_CUR",
	"NFS4_OPEN_CLAIM_DELEGATE_PREV",
	"NFS4_OPEN_CLAIM_FH",
	"NFS4_OPEN_CLAIM_DELEG_CUR_FH",
	"NFS4_OPEN_CLAIM_DELEG_PREV_FH",
}

var shareAccesses = []string{
	"",
	"NFS4_SHARE_ACCESS_READ",
	"NFS4_SHARE_ACCESS_WRITE",
	"NFS4_SHARE_ACCESS_BOTH",
}

var shareAccessWants = []string{
	"NFS4_SHARE_WANT_NO_PREFERENCE",
	"NFS4_SHARE_WANT_READ_DELEG",
	"NFS4_SHARE_WANT_WRITE_DELEG",
	"NFS4_SHARE_WANT_ANY_DELEG",
	"NFS4_SHARE_WANT_NO_DELEG",
	"NFS4_SHARE_WANT_CANCEL",
}

func showFlags32(flags uint32, names []string) []string {
	strs := []string{}

	if 32 < len(names) {
		log.Fatal("names size too long", zap.Array("names", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
			for _, n := range names {
				inner.AppendString(n)
			}
			return nil
		})))
	}

	for i := 0; i < len(names); i++ {
		if flags>>i&0x1 == 0x0 {
			continue
		}
		if names[i] == "" {
			strs = append(strs, "Bit "+strconv.Itoa(i))
			continue
		}
		strs = append(strs, names[i])
	}

	for i := len(names); i < 32; i++ {
		if flags>>i&0x1 == 0x1 {
			strs = append(strs, "Bit "+strconv.Itoa(i))
		}
	}

	return strs
}

func showFlags64(flags uint64, names []string) []string {
	strs := []string{}

	if 64 < len(names) {
		log.Fatal("names size too long", zap.Array("names", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
			for _, n := range names {
				inner.AppendString(n)
			}
			return nil
		})))
	}

	for i := 0; i < len(names); i++ {
		if flags>>i&0x1 == 0x0 {
			continue
		}
		if names[i] == "" {
			strs = append(strs, "Bit "+strconv.Itoa(i))
			continue
		}
		strs = append(strs, names[i])
	}

	for i := len(names); i < 64; i++ {
		if flags>>i&0x1 == 0x1 {
			strs = append(strs, "Bit "+strconv.Itoa(i))
		}
	}

	return strs
}

type fh struct {
	Size uint8
	Data [cNFSMaxFHSize]byte
}

func (h *fh) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("asHex", "0x"+hex.EncodeToString(h.Data[:h.Size]))
	return nil
}

type stateid struct {
	Seqid [4]byte
	Other [cNFS4StateidOtherSize]byte
	Type  uint32
}

func (s *stateid) otherHex() string {
	return hex.EncodeToString(s.Other[:])
}

func (s *stateid) typeName() string {
	names := []string{
		"NFS4_INVALID_STATEID_TYPE",
		"NFS4_SPECIAL_STATEID_TYPE",
		"NFS4_OPEN_STATEID_TYPE",
		"NFS4_LOCK_STATEID_TYPE",
		"NFS4_DELEGATION_STATEID_TYPE",
		"NFS4_LAYOUT_STATEID_TYPE",
		"NFS4_PNFS_DS_STATEID_TYPE",
		"NFS4_REVOKED_STATEID_TYPE",
	}
	if int(s.Type) < len(names) {
		return names[s.Type]
	}

	return "UNKNOWN_TYPE: " + strconv.Itoa(int(s.Type))
}

func (s *stateid) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint32("seqid", asBigEndianUint32(s.Seqid))
	enc.AddString("data", s.otherHex())
	enc.AddString("type", s.typeName())
	return nil
}

type state struct {
	OpenStateid stateid
	Stateid     stateid
	Flags       uint64
	NRdonly     uint32
	NWronly     uint32
	NRdwr       uint32
	State       uint32
}

func (s *state) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("openStateid", &s.OpenStateid)
	enc.AddObject("stateid", &s.Stateid)
	enc.AddArray("flags", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, f := range showFlags64(s.Flags, stateFlags) {
			inner.AppendString(f)
		}
		return nil
	}))
	enc.AddUint32("n_rdonly", s.NRdonly)
	enc.AddUint32("n_wronly", s.NWronly)
	enc.AddUint32("n_rdwr", s.NRdwr)
	enc.AddArray("state", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, f := range showFlags32(s.State, fmodeFlags) {
			inner.AppendString(f)
		}
		return nil
	}))
	return nil
}

type client struct {
	ClState uint64
}

func (c *client) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddArray("cl_state", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, f := range showFlags64(c.ClState, clientStates) {
			inner.AppendString(f)
		}
		return nil
	}))
	return nil
}

type eventCStruct struct {
	Ts                      uint64
	PointIDs                [cCallOrderCount]uint8
	PointDeltas             [cCallOrderCount]uint32
	CallCounts              [cSlowPointCount]uint8
	Task                    [cTaskCommLen]byte
	File                    [cDnameInlineLen]byte
	PID                     uint64
	Delta                   uint64
	RunOpenTask             runOpenTask
	OpendataToNFS4State     opendataToNFS4State
	UpdateOpenStateid       updateOpenStateid
	StateMarkReclaimNograce stateMarkReclaimNograce
	WaitClntRecover         waitClntRecover
	OrderIndex              uint32
	Reason                  uint32
}

type runOpenTask struct {
	EnterOArgFH          fh
	EnterOArgShareAccess uint32
	EnterOArgClaim       uint8
	ReturnOResStateid    stateid
}

type opendataToNFS4State struct {
	OResStateid stateid
}

type updateOpenStateid struct {
	OpenStateid stateid
	State       state
}

type stateMarkReclaimNograce struct {
	EnterState  state
	ReturnState state
	Executed    uint32
	Result      uint32
}

type waitClntRecover struct {
	Client client
}

func (u *runOpenTask) shareAccessValues() []string {
	values := []string{}

	access := u.EnterOArgShareAccess & 0x0000000f
	if int(access) < len(shareAccesses) && shareAccesses[access] != "" {
		values = append(values, shareAccesses[access])
	} else {
		log.Warn("unknown share_access lower 4 bits", zap.Uint32("4bits", access))
		values = append(values, "UNKNOWN LOWER 4 BITS: "+strconv.Itoa(int(access)))
	}

	want := (u.EnterOArgShareAccess & 0x0000ff00) >> 8
	if int(want) < len(shareAccessWants) && shareAccessWants[want] != "" {
		values = append(values, shareAccessWants[want])
	} else {
		log.Warn("unknown share_access middle 8 bits", zap.Uint32("8bits", want))
		values = append(values, "UNKNOWN MIDDLE 8 BITS: "+strconv.Itoa(int(want)))
	}

	return values
}

func (u *runOpenTask) claimValue() string {
	if int(u.EnterOArgClaim) < len(claims) {
		return claims[u.EnterOArgClaim]
	}

	log.Warn("unknown claim value", zap.Uint8("value", u.EnterOArgClaim))
	return "UNKNOWN VALUE: " + strconv.Itoa(int(u.EnterOArgClaim))
}

func (u *runOpenTask) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("enter fh", &u.EnterOArgFH)
	enc.AddArray("enter share_access", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, v := range u.shareAccessValues() {
			inner.AppendString(v)
		}
		return nil
	}))
	enc.AddString("enter claim", u.claimValue())
	enc.AddObject("return stateid", &u.ReturnOResStateid)
	return nil
}

func (u *opendataToNFS4State) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("enter stateid", &u.OResStateid)
	return nil
}

func (u *updateOpenStateid) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("enter open_stateid", &u.OpenStateid)
	enc.AddObject("enter state", &u.State)
	return nil
}

func (u *stateMarkReclaimNograce) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("enter state", &u.EnterState)
	enc.AddObject("return state", &u.ReturnState)
	enc.AddBool("executed", u.Executed != 0)
	enc.AddUint32("result", u.Result)
	return nil
}

func (u *waitClntRecover) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("client", &u.Client)
	return nil
}

func toBinaryRepr(v uint32) string {
	return fmt.Sprintf("%032b", v)
}

func parseData(data []byte) (*eventCStruct, error) {
	var cEvent eventCStruct
	if err := binary.Read(bytes.NewBuffer(data), bcc.GetHostByteOrder(), &cEvent); err != nil {
		return nil, err
	}

	return &cEvent, nil
}

func outputDebug(evt *eventCStruct, now *time.Time) {
	delta := time.Duration(evt.Delta) * time.Microsecond

	log.Debug(
		"event",
		zap.Time("time", now.Add(-delta)),
		zap.Duration("delta", delta),
		zap.Array("points", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
			for i, id := range evt.PointIDs {
				inner.AppendObject(zapcore.ObjectMarshalerFunc(func(inner2 zapcore.ObjectEncoder) error {
					inner2.AddUint8("id", id)
					inner2.AddDuration("delta", time.Duration(evt.PointDeltas[i])*time.Microsecond)
					inner2.AddUint8("total", evt.CallCounts[id])
					return nil
				}))
			}
			return nil
		})),
		zap.Array("counts", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
			for _, c := range evt.CallCounts {
				inner.AppendUint8(c)
			}
			return nil
		})),
		zap.Object("nfs4_run_open_task()", &evt.RunOpenTask),
		zap.Object("_nfs4_opendata_to_nfs4_state()", &evt.OpendataToNFS4State),
		zap.Object("update_open_stateid()", &evt.UpdateOpenStateid),
		zap.Object("nfs4_state_mark_reclaim_nograce()", &evt.StateMarkReclaimNograce),
		zap.Object("nfs4_wait_clnt_recover()", &evt.WaitClntRecover),
		zap.Uint32("pid", uint32(evt.PID)),
		zap.String("task", cPointerToString(unsafe.Pointer(&evt.Task))),
		zap.String("file", cPointerToString(unsafe.Pointer(&evt.File))),
		zap.Array("reason", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
			for _, f := range showFlags32(evt.Reason, outputReasonFlags) {
				inner.AppendString(f)
			}
			return nil
		})),
	)
}

// Run starts compiling eBPF code and then notifying of file updates.
func Run(ctx context.Context, config *Config) {
	log = config.Log
	source := generateSource(config)
	if config.Debug {
		fmt.Fprintln(os.Stderr, source)
	}
	m := bcc.NewModule(source, []string{}, config.BpfDebug)
	defer m.Close()

	if config.Quit {
		return
	}

	channel := make(chan []byte, 65536)
	perfMap := configTrace(m, channel)

	go func() {
		log.Info("tracing started")
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-channel:
				now := time.Now()
				evt, err := parseData(data)
				if err != nil {
					log.Error("failed to decode received data", zap.Error(err))
					continue
				}

				outputDebug(evt, &now)
			}
		}
	}()

	perfMap.Start()
	<-ctx.Done()
	perfMap.Stop()
}
