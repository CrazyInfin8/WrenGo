package wren

/*
#cgo CFLAGS:
#cgo LDFLAGS: -lm
#include "wren.h"
*/
import "C"

//go:generate go run getWren.go
import ()

const (
	VersionString string = C.WREN_VERSION_STRING
	VersionMajor int = C.WREN_VERSION_MAJOR
	VersionMinor int = C.WREN_VERSION_MINOR
	VersionPatch int = C.WREN_VERSION_PATCH
)

func VersionTuple() [3]int {
	return [3]int{
		C.WREN_VERSION_MAJOR,
		C.WREN_VERSION_MINOR,
		C.WREN_VERSION_PATCH,
	}
}
