// Package wren provides bindings for Go programs to utilize and interact with the [wren](https://wren.io/) scripting langues
package wren

/*
#cgo CFLAGS:
#cgo LDFLAGS: -lm
#include "wren.h"
*/
import "C"

//go:generate go run getWren.go
//go:generate go run createBindings.go -bindings 128
import ()

const (
	// VersionString Wren's version as a string
	VersionString string = C.WREN_VERSION_STRING
	// VersionMajor Wren's major version number
	VersionMajor int = C.WREN_VERSION_MAJOR
	// VersionMinor Wren's minor version number
	VersionMinor int = C.WREN_VERSION_MINOR
	// VersionPatch Wren's patch version number
	VersionPatch int = C.WREN_VERSION_PATCH
)

// VersionTuple returns Wren's version numbers as an array of 3 numbers
func VersionTuple() [3]int {
	return [3]int{
		C.WREN_VERSION_MAJOR,
		C.WREN_VERSION_MINOR,
		C.WREN_VERSION_PATCH,
	}
}
