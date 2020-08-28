/*
Tinygo is missing some cgo function that WrenGo uses to pass data between C and Go
This is based off of this comment: https://github.com/tinygo-org/tinygo/issues/854#issuecomment-678292745
and this particular file: https://github.com/bgould/go-littlefs/blob/master/go_lfs.go#L415-L430

*/
package wren

/*
#include "wren.h"
#include <string.h>
*/
import "C"

import (
	"unsafe"
)

func cstring(s string) *C.char {
	ptr := C.malloc(C.size_t(len(s) + 1))
	buf := (*[1 << 28]byte)(ptr)[: len(s)+1 : len(s)+1]
	copy(buf, s)
	buf[len(s)] = 0
	return (*C.char)(ptr)
}

func cbytes(b []byte) *C.char {
	ptr := C.malloc(C.size_t(len(b)))
	buf := (*[1 << 28]byte)(ptr)[: len(b) : len(b)]
	copy(buf, b)
	return (*C.char)(ptr)
}

func gostring(s *C.char) string {
	slen := int(C.strlen(s))
	sbuf := make([]byte, slen)
	copy(sbuf, (*[1 << 28]byte)(unsafe.Pointer(s))[:slen:slen])
	return string(sbuf)
}

func gobytes(ptr unsafe.Pointer, len C.int) []byte {
	slen := int(len)
	sbuf := make([]byte, slen)
	copy(sbuf, (*[1 << 28]byte)(ptr)[:slen:slen])
	return sbuf

}