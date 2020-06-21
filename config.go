package wren

import (
	"fmt"
	"io"
	"os"
)

// Config contains some settings to setup how VM will behave
type Config struct {
	// Wren calls this function to print text
	WriteFn WriteFn
	// Wren calls this function to print errors
	ErrorFn ErrorFn
	// Wren calls this function to import modules (if you want to disable importing, this should be set to nil and the global value `DefaultModuleLoader` should also be set to nil)
	LoadModuleFn LoadModuleFn
	// If `WriteFn` is not set, wren will print text to here instead (if you want to disable all output, this should be set to nil and the global value `DefaultOutput` should also be set to nil)
	DefaultOutput io.Writer
	// If `ErrorFn` is not set, wren errors will be written to here instead (if you want to disable all output, this should be set to nil and the global value `DefaultError` should also be set to nil)
	DefaultError io.Writer
	// Custom data
	UserData interface{}
}

// WriteFn is called by wren whenever `System.write`, `System.print`, or `System.printAll` is called in a script
type WriteFn func(vm *VM, text string)

// ErrorFn is called by Wren whenever there is a runtime error, compile error, or stack trace. It should be of type `CompileError`, `RuntimeError`, or `StackTrace`
type ErrorFn func(vm *VM, err error)

// LoadModuleFn is called by Wren to whenever `import` is called. it can either return a string with wren source code or it can return nil to send an error to the VM
type LoadModuleFn func(vm *VM, name string) string

// CompileError is sent by Wren to `ErrorFn` if Wren source code couldn't compile
type CompileError struct {
	module, message string
	line            int
}

func (err *CompileError) Error() string {
	return fmt.Sprintf("[%v line %v] %v", err.module, err.line, err.message)
}

// RuntimeError is sent by Wren to `ErrorFn` if the vm encountered an error during script execution
type RuntimeError struct {
	message string
}

func (err *RuntimeError) Error() string {
	return err.message
}

// StackTrace is sent by Wren to `ErrorFn` after sending `RuntimeError` these help try to pinpoint how and where an error occurred
type StackTrace struct {
	module, message string
	line            int
}

func (err *StackTrace) Error() string {
	return fmt.Sprintf("[%v line %v] %v", err.module, err.line, err.message)
}

// NewConfig creates a new config and initializes it with default variables (mainly specifying where output should go)
func NewConfig() *Config {
	return &Config{DefaultOutput: os.Stdout, DefaultError: os.Stderr}
}

// Clone returns a copy of a config
func (cfg Config) Clone() *Config {
	return &cfg
}
