package slow

import (
	"bytes"
	"context"
	"encoding/binary"
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

//go:generate statik -src=c

var (
	log *zap.Logger
)

const (
	cTaskCommLen    = 16
	cSlowPointCount = 16
	cCallOrderCount = 32
	cDnameInlineLen = 32
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
	Ts          uint64
	PointsDelta [cSlowPointCount]uint64
	PointsCount [cSlowPointCount]uint8
	CallOrder   [cCallOrderCount]uint8
	Delta       uint64
	PID         uint64
	Task        [cTaskCommLen]byte
	File        [cDnameInlineLen]byte
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
	addPoint(m, "update_open_stateid")
	addPoint(m, "prepare_to_wait")
	addPoint(m, "nfs_state_log_update_open_stateid")
	addPoint(m, "update_open_stateflags")
	//
	//  nfs4_atomic_open
	//      nfs4_do_open (N/S)
	//          _nfs4_do_open (N/S)
	//              nfs4_get_state_owner
	//              nfs4_client_recover_expired_lease
	//                  nfs4_wait_clnt_recover
	//                      wait_on_bit_action (N/S)
	//                          out_of_line_wait_on_bit (MULTIPLE CALLS)
	//                      nfs_put_client
	//                  nfs4_schedule_state_manager (MULTIPLE CALLS)
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
	//                      nfs4_opendata_check_deleg (N/S)
	//                          nfs_inode_set_delegation (Conditional)
	//                          nfs_inode_reclaim_delegation (Conditional)
	//                      update_open_stateid
	//                          nfs_state_set_open_stateid (N/S)
	//                              nfs_set_open_stateid_locked (N/S)
	//                                  prepare_to_wait
	//                                  nfs_test_and_clear_all_open_stateid (N/S)
	//                                  nfs_state_log_update_open_stateid
	//                          nfs_mark_delegation_referenced (Conditional)
	//                          update_open_stateflags
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

type eventData struct {
	ts          uint64
	pointsDelta [cSlowPointCount]uint64
	pointsCount [cSlowPointCount]uint8
	callOrder   [cCallOrderCount]uint8
	delta       uint64
	pid         uint32
	comm        string
	file        string
}

func parseData(data []byte) (*eventData, error) {
	var cEvent eventCStruct
	if err := binary.Read(bytes.NewBuffer(data), bcc.GetHostByteOrder(), &cEvent); err != nil {
		return nil, err
	}

	event := &eventData{
		ts:          cEvent.Ts,
		pointsDelta: cEvent.PointsDelta,
		pointsCount: cEvent.PointsCount,
		callOrder:   cEvent.CallOrder,
		delta:       cEvent.Delta,
		pid:         uint32(cEvent.PID),
		comm:        cPointerToString(unsafe.Pointer(&cEvent.Task)),
		file:        cPointerToString(unsafe.Pointer(&cEvent.File)),
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
					zap.Duration("duration", time.Duration(evt.delta)*time.Microsecond),
					zap.Array("deltas", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
						for _, dur := range evt.pointsDelta {
							inner.AppendDuration(time.Duration(dur) * time.Microsecond)
						}
						return nil
					})),
					zap.Array("counts", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
						for _, c := range evt.pointsCount {
							inner.AppendUint8(c)
						}
						return nil
					})),
					zap.Array("order", zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
						for _, p := range evt.callOrder {
							inner.AppendUint8(p)
						}
						return nil
					})),
					zap.String("comm", evt.comm),
					zap.String("file", evt.file),
					zap.Uint32("pid", evt.pid),
				)
			}
		}
	}()

	perfMap.Start()
	<-ctx.Done()
	perfMap.Stop()
}
