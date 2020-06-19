package wren

import (
	"fmt"
	"io"
	"os"
)

type Config struct {
	WriteFn       WriteFn
	ErrorFn       ErrorFn
	LoadModuleFn  LoadModuleFn
	UserData      interface{}
	DefaultOutput io.Writer
	DefaultError  io.Writer
}

type WriteFn func(vm *VM, text string)

type ErrorFn func(vm *VM, err error)
type LoadModuleFn func(vm *VM, name string) string
type ResolveModuleFn func(vm *VM, importer, name string) string

type CompileError struct {
	module, message string
	line            int
}

func (err *CompileError) Error() string {
	return fmt.Sprintf("[%v line %v] %v", err.module, err.line, err.message)
}

type RuntimeError struct {
	message string
}

func (err *RuntimeError) Error() string {
	return err.message
}

type StackTrace struct {
	module, message string
	line   int
}

func (err *StackTrace) Error() string {
	return fmt.Sprintf("[%v line %v] %v", err.module, err.line, err.message)
}

// Creates a new Config with default options
func NewConfig() *Config {
	return &Config{DefaultOutput: os.Stdout, DefaultError: os.Stderr}
}

func (cfg Config)Clone() *Config {
	return &cfg
}