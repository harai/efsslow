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
)

// Config configures parameters to filter what to be notified.
type Config struct {
	SlowThresholdMS uint
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

type eventCStruct struct {
	Ts                        uint64
	PointIDs                  [cCallOrderCount]uint8
	PointDeltas               [cCallOrderCount]uint32
	CallCounts                [cSlowPointCount]uint8
	Task                      [cTaskCommLen]byte
	File                      [cDnameInlineLen]byte
	PID                       uint64
	Delta                     uint64
	OpenStateidSeqid          [4]byte
	OpenStateidOther          [cNFS4StateidOtherSize]byte
	StateOpenStateidSeqid     [4]byte
	StateOpenStateidOther     [cNFS4StateidOtherSize]byte
	StateStateidSeqid         [4]byte
	StateStateidOther         [cNFS4StateidOtherSize]byte
	OpendataStateidSeqid      [4]byte
	OpendataStateidOther      [cNFS4StateidOtherSize]byte
	EnterRuntaskStateidSeqid  [4]byte
	EnterRuntaskStateidOther  [cNFS4StateidOtherSize]byte
	ReturnRuntaskStateidSeqid [4]byte
	ReturnRuntaskStateidOther [cNFS4StateidOtherSize]byte
	ToStateStateidSeqid       [4]byte
	ToStateStateidOther       [cNFS4StateidOtherSize]byte
	StateFlags                uint64
	NograceStateFlags         uint64
	ReturnNograceStateFlags   uint64
	StateClientSession        uint64
	OpenStateidType           uint32
	StateNRdonly              uint32
	StateNWronly              uint32
	StateNRdwr                uint32
	NograceExecuted           uint32
	NograceStateNRdonly       uint32
	NograceStateNWronly       uint32
	NograceStateNRdwr         uint32
	ReturnNograceExecuted     uint32
	ReturnNograceStateNRdonly uint32
	ReturnNograceStateNWronly uint32
	ReturnNograceStateNRdwr   uint32
	StateState                uint32
	StateOpenStateidType      uint32
	StateStateidType          uint32
	OpendataStateidType       uint32
	EnterRuntaskStateidType   uint32
	ReturnRuntaskStateidType  uint32
	ToStateStateidType        uint32
	OrderIndex                uint32
	Show                      uint32
}

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
	return strings.Replace(
		source,
		"/*SLOW_THRESHOLD_MS*/",
		strconv.FormatUint(uint64(config.SlowThresholdMS), 10), -1)
}

type stateid struct {
	seqid uint32
	other [cNFS4StateidOtherSize]byte
	type0 uint32
}

func (s *stateid) otherHex() string {
	return hex.EncodeToString(s.other[:])
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
	if int(s.type0) < len(names) {
		return names[s.type0]
	}

	return "UNKNOWN_TYPE: " + strconv.Itoa(int(s.type0))
}

func (s *stateid) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint32("seqid", s.seqid)
	enc.AddString("data", s.otherHex())
	enc.AddString("type", s.typeName())
	return nil
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

type eventOpenStateid struct {
	stateid
}

type eventState struct {
	openStateid   *stateid
	stateid       *stateid
	flags         uint64
	nRdonly       uint32
	nWronly       uint32
	nRdwr         uint32
	state         uint32
	clientSession uint64
}

func (s *eventState) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("openStateid", s.openStateid)
	enc.AddObject("stateid", s.stateid)
	enc.AddArray("flags", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, f := range showFlags64(s.flags, stateFlags) {
			inner.AppendString(f)
		}
		return nil
	}))
	enc.AddUint32("n_rdonly", s.nRdonly)
	enc.AddUint32("n_wronly", s.nWronly)
	enc.AddUint32("n_rdwr", s.nRdwr)
	enc.AddUint64("((struct nfs_server *)(inode->i_sb->s_fs_info))->nfs_client->cl_session", s.clientSession)
	enc.AddArray("state", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, f := range showFlags32(s.state, fmodeFlags) {
			inner.AppendString(f)
		}
		return nil
	}))
	return nil
}

type eventOpendata struct {
	stateid *stateid
}

func (s *eventOpendata) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("stateid", s.stateid)
	return nil
}

type eventUpdateOpenStateid struct {
	openStateid *eventOpenStateid
	state       *eventState
}

func (u *eventUpdateOpenStateid) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("openStateid", u.openStateid)
	enc.AddObject("state", u.state)
	return nil
}

type eventNograceState struct {
	executed bool
	flags    uint64
	nRdonly  uint32
	nWronly  uint32
	nRdwr    uint32
}

func (n *eventNograceState) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddBool("executed", n.executed)
	enc.AddArray("flags", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
		for _, f := range showFlags64(n.flags, stateFlags) {
			inner.AppendString(f)
		}
		return nil
	}))
	enc.AddUint32("n_rdonly", n.nRdonly)
	enc.AddUint32("n_wronly", n.nWronly)
	enc.AddUint32("n_rdwr", n.nRdwr)
	return nil
}

func toBinaryRepr(v uint32) string {
	return fmt.Sprintf("%032b", v)
}

type optionalStateid struct {
	*stateid
	executed bool
}

func (s *optionalStateid) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddBool("executed", s.executed)
	enc.AddUint32("seqid", s.seqid)
	enc.AddString("data", s.otherHex())
	enc.AddString("type", s.typeName())
	return nil
}

type eventData struct {
	ts                   uint64
	delta                uint32
	pointIDs             [cCallOrderCount]uint8
	pointDeltas          [cCallOrderCount]uint32
	callCounts           [cSlowPointCount]uint8
	opendata             *eventOpendata
	enterRuntaskStateid  *stateid
	returnRuntaskStateid *stateid
	toStateStateid       *stateid
	updateOpenStateid    *eventUpdateOpenStateid
	nograceState         *eventNograceState
	returnNograceState   *eventNograceState
	pid                  uint32
	task                 string
	file                 string
	show                 bool
	returnRuntaskDone    bool
}

func parseData(data []byte) (*eventData, error) {
	var cEvent eventCStruct
	if err := binary.Read(bytes.NewBuffer(data), bcc.GetHostByteOrder(), &cEvent); err != nil {
		return nil, err
	}

	event := &eventData{
		ts:          cEvent.Ts,
		delta:       uint32(cEvent.Delta),
		pointIDs:    cEvent.PointIDs,
		pointDeltas: cEvent.PointDeltas,
		callCounts:  cEvent.CallCounts,
		opendata: &eventOpendata{
			stateid: &stateid{
				seqid: asBigEndianUint32(cEvent.OpendataStateidSeqid),
				other: cEvent.OpendataStateidOther,
				type0: cEvent.OpendataStateidType,
			},
		},
		enterRuntaskStateid: &stateid{
			seqid: asBigEndianUint32(cEvent.EnterRuntaskStateidSeqid),
			other: cEvent.EnterRuntaskStateidOther,
			type0: cEvent.EnterRuntaskStateidType,
		},
		returnRuntaskStateid: &stateid{
			seqid: asBigEndianUint32(cEvent.ReturnRuntaskStateidSeqid),
			other: cEvent.ReturnRuntaskStateidOther,
			type0: cEvent.ReturnRuntaskStateidType,
		},
		toStateStateid: &stateid{
			seqid: asBigEndianUint32(cEvent.ToStateStateidSeqid),
			other: cEvent.ToStateStateidOther,
			type0: cEvent.ToStateStateidType,
		},
		updateOpenStateid: &eventUpdateOpenStateid{
			openStateid: &eventOpenStateid{
				stateid: stateid{
					seqid: asBigEndianUint32(cEvent.OpenStateidSeqid),
					other: cEvent.OpenStateidOther,
					type0: cEvent.OpenStateidType,
				},
			},
			state: &eventState{
				openStateid: &stateid{
					seqid: asBigEndianUint32(cEvent.StateOpenStateidSeqid),
					other: cEvent.StateOpenStateidOther,
					type0: cEvent.StateOpenStateidType,
				},
				stateid: &stateid{
					seqid: asBigEndianUint32(cEvent.StateStateidSeqid),
					other: cEvent.StateStateidOther,
					type0: cEvent.StateStateidType,
				},
				flags:         cEvent.StateFlags,
				nRdonly:       cEvent.StateNRdonly,
				nWronly:       cEvent.StateNWronly,
				nRdwr:         cEvent.StateNRdwr,
				state:         cEvent.StateState,
				clientSession: cEvent.StateClientSession,
			},
		},
		nograceState: &eventNograceState{
			executed: cEvent.NograceExecuted != 0,
			flags:    cEvent.NograceStateFlags,
			nRdonly:  cEvent.NograceStateNRdonly,
			nWronly:  cEvent.NograceStateNWronly,
			nRdwr:    cEvent.NograceStateNRdwr,
		},
		returnNograceState: &eventNograceState{
			executed: cEvent.ReturnNograceExecuted != 0,
			flags:    cEvent.ReturnNograceStateFlags,
			nRdonly:  cEvent.ReturnNograceStateNRdonly,
			nWronly:  cEvent.ReturnNograceStateNWronly,
			nRdwr:    cEvent.ReturnNograceStateNRdwr,
		},
		pid:  uint32(cEvent.PID),
		task: cPointerToString(unsafe.Pointer(&cEvent.Task)),
		file: cPointerToString(unsafe.Pointer(&cEvent.File)),
		show: cEvent.Show != 0,
	}

	return event, nil
}

// Run starts compiling eBPF code and then notifying of file updates.
func Run(ctx context.Context, config *Config) {
	log = config.Log
	source := generateSource(config)
	if config.Debug {
		fmt.Fprintln(os.Stderr, source)
	}
	m := bcc.NewModule(generateSource(config), []string{}, config.BpfDebug)
	defer m.Close()

	if config.Quit {
		return
	}

	channel := make(chan []byte, 8192)
	perfMap := configTrace(m, channel)

	go func() {
		log.Info("tracing started")
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-channel:
				evt, err := parseData(data)
				if err != nil {
					fmt.Printf("failed to decode received data: %s\n", err)
					continue
				}

				log.Debug(
					"event",
					zap.Duration("delta", time.Duration(evt.delta)*time.Microsecond),
					zap.Array("points", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
						for i, id := range evt.pointIDs {
							inner.AppendObject(zapcore.ObjectMarshalerFunc(func(inner2 zapcore.ObjectEncoder) error {
								inner2.AddUint8("id", id)
								inner2.AddDuration("delta", time.Duration(evt.pointDeltas[i])*time.Microsecond)
								inner2.AddUint8("total", evt.callCounts[id])
								return nil
							}))
						}
						return nil
					})),
					zap.Array("counts", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
						for _, c := range evt.callCounts {
							inner.AppendUint8(c)
						}
						return nil
					})),
					zap.Object("nfs4_opendata_alloc() return o_res", evt.opendata),
					zap.Object("nfs4_run_open_task() enter o_res", evt.enterRuntaskStateid),
					zap.Object("nfs4_run_open_task() return o_res", evt.returnRuntaskStateid),
					zap.Object("_nfs4_opendata_to_nfs4_state() enter o_res", evt.toStateStateid),
					zap.Object("update_open_stateid()", evt.updateOpenStateid),
					zap.Object("nfs4_state_mark_reclaim_nograce() enter", evt.nograceState),
					zap.Object("nfs4_state_mark_reclaim_nograce() return", evt.returnNograceState),
					zap.Uint32("pid", evt.pid),
					zap.String("task", evt.task),
					zap.String("file", evt.file),
					zap.Bool("show", evt.show),
				)
			}
		}
	}()

	perfMap.Start()
	<-ctx.Done()
	perfMap.Stop()
}
