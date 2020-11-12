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
	"unicode"
	"unsafe"

	"github.com/iovisor/gobpf/bcc"
	"github.com/rakyll/statik/fs"

	// Load static assets
	_ "github.com/harai/efsslow/statik"
	"go.uber.org/zap"
)

//go:generate statik -src=c

var (
	log *zap.Logger
)

const (
	cTaskCommLen    = 16
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
	EvtType    uint64
	TsMicro    uint64
	DeltaMicro uint64
	PID        uint64
	Task       [cTaskCommLen]byte
	File       [cDnameInlineLen]byte
}

func configNfsFileOpenTrace(m *bcc.Module) error {
	kprobe, err := m.LoadKprobe("enter__nfs_file_open")
	if err != nil {
		return err
	}

	if err := m.AttachKprobe("nfs_file_open", kprobe, -1); err != nil {
		return err
	}

	kretprobe, err := m.LoadKprobe("return__nfs_file_open")
	if err != nil {
		return err
	}

	if err := m.AttachKretprobe("nfs_file_open", kretprobe, -1); err != nil {
		return err
	}

	return nil
}

func configNfs4FileOpenTrace(m *bcc.Module) error {
	kprobe, err := m.LoadKprobe("enter__nfs_file_open")
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

func configTrace(m *bcc.Module, receiverChan chan []byte) *bcc.PerfMap {
	if err := configNfsFileOpenTrace(m); err != nil {
		log.Fatal("failed to config nfs_file_open trace", zap.Error(err))
	}

	if err := configNfs4FileOpenTrace(m); err != nil {
		log.Fatal("failed to config nfs4_file_open trace", zap.Error(err))
	}

	table := bcc.NewTable(m.TableId("events"), m)

	perfMap, err := bcc.InitPerfMap(table, receiverChan, nil)
	if err != nil {
		log.Fatal("Failed to init perf map", zap.Error(err))
	}

	return perfMap
}

type evtType struct {
	val  EventType
	name string
}

type evtTypeData struct {
	valMap   map[EventType]evtType
	evtTypes []evtType
}

// EventType is an event type eBPF notfies.
type EventType uint64

// Event type to be notified.
const (
	EventTypeNfs4FileOpen EventType = 0x1 << iota
	EventTypeNfsFileOpen
)

func newEvtTypeSet() evtTypeData {
	evtTypes := []evtType{
		{EventTypeNfs4FileOpen, "nfs4_file_open"},
		{EventTypeNfsFileOpen, "nfs4_file_open"},
	}

	s := evtTypeData{
		valMap:   make(map[EventType]evtType, len(evtTypes)),
		evtTypes: make([]evtType, 0, len(evtTypes)),
	}

	for _, e := range evtTypes {
		s.valMap[e.val] = e
		s.evtTypes = append(s.evtTypes, e)
	}

	return s
}

var evtTypeSet = newEvtTypeSet()

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func generateSource(config *Config) string {
	return strings.Replace(
		source,
		"/*SLOW_THRESHOLD_MS*/",
		strconv.FormatUint(uint64(config.SlowThresholdMS), 10), -1)
}

// Event tells the details of notification.
type Event struct {
	EvtType       EventType
	Pid           uint32
	DurationMicro uint64
	Comm          string
	File          string
}

type eventData struct {
	evtType    evtType
	tsMicro    uint64
	deltaMicro uint64
	pid        uint32
	comm       string
	file       string
}

func parseData(data []byte) (*eventData, error) {
	var cEvent eventCStruct
	if err := binary.Read(bytes.NewBuffer(data), bcc.GetHostByteOrder(), &cEvent); err != nil {
		return nil, err
	}

	event := &eventData{
		evtType:    evtTypeSet.valMap[EventType(cEvent.EvtType)],
		tsMicro:    cEvent.TsMicro,
		deltaMicro: cEvent.DeltaMicro,
		pid:        uint32(cEvent.PID),
		comm:       cPointerToString(unsafe.Pointer(&cEvent.Task)),
		file:       cPointerToString(unsafe.Pointer(&cEvent.File)),
	}

	return event, nil
}

// Run starts compiling eBPF code and then notifying of file updates.
func Run(ctx context.Context, config *Config, eventCh chan<- *Event) {
	log = config.Log
	source := generateSource(config)
	if config.Debug {
		fmt.Fprintln(os.Stderr, source)
	}
	m := bcc.NewModule(generateSource(config), []string{}, config.BpfDebug)
	defer m.Close()

	if config.Quit {
		close(eventCh)
		return
	}

	channel := make(chan []byte, 8192)
	perfMap := configTrace(m, channel)

	go func() {
		log.Info("tracing started")
		for {
			select {
			case <-ctx.Done():
				close(eventCh)
				return
			case data := <-channel:
				evt, err := parseData(data)
				if err != nil {
					fmt.Printf("failed to decode received data: %s\n", err)
					continue
				}

				log.Debug(
					"event",
					zap.String("evttype", evt.evtType.name),
					zap.Uint32("pid", evt.pid),
					zap.Duration("duration", time.Duration(evt.deltaMicro/1000)*time.Millisecond),
					zap.String("comm", evt.comm),
					zap.String("file", evt.file),
				)

				eventCh <- &Event{
					EvtType:       evt.evtType.val,
					Pid:           evt.pid,
					DurationMicro: evt.deltaMicro,
					Comm:          evt.comm,
					File:          evt.file,
				}
			}
		}
	}()

	perfMap.Start()
	<-ctx.Done()
	perfMap.Stop()
}
