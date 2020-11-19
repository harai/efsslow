package slow

import "C"
import (
	"bytes"
	"encoding/binary"
	"unsafe"
)

func cPointerToString(anything unsafe.Pointer) string {
	return C.GoString((*C.char)(anything))
}

func asBigEndianUint32(data [4]byte) uint32 {
	var val uint32
	buf := bytes.NewReader(data[:])
	if err := binary.Read(buf, binary.BigEndian, &val); err != nil {
		panic("should not happen")
	}
	return val
}
