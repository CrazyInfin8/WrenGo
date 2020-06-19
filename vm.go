package wren

/*
#cgo CFLAGS:
#cgo LDFLAGS: -lm
#include "wren.h"

extern void writeFn(WrenVM*, char*);
extern void errorFn(WrenVM*, WrenErrorType, char* module, int line, char* message);
extern char* moduleLoaderFn(WrenVM* vm, char* name);
*/
import "C"
import (
	"io"
	"io/ioutil"
	"os"
	"unsafe"
)

type VM struct {
	vm      *C.WrenVM
	Config  *Config
	errChan chan error
	handles []*C.WrenHandle
}

var (
	vmMap map[*C.WrenVM]*VM = make(map[*C.WrenVM]*VM)
	// DefaultOutput is where wren will print to if a VM's config doesn't specify its own output (Set this to nil to disable output)
	DefaultOutput io.Writer = os.Stdout
	// DefaultError is where wren will send error messages to if a VM's config doesn't specify its own place for outputting errors (Set this to nil to disable output)
	DefaultError io.Writer = os.Stderr

	DefaultModuleLoader LoadModuleFn = func(vm *VM, name string) (source string) {
		if data, err := ioutil.ReadFile(name); err == nil {
			source = string(data)
		}
		return source
	}
)

func NewVM() *VM {
	var config C.WrenConfiguration
	C.wrenInitConfiguration(&config)
	config.writeFn = C.WrenWriteFn(C.writeFn)
	config.errorFn = C.WrenErrorFn(C.errorFn)
	config.loadModuleFn = C.WrenLoadModuleFn(C.moduleLoaderFn)
	vm := VM{vm: C.wrenNewVM(&config), handles: make([]*C.WrenHandle, 0)}
	vmMap[vm.vm] = &vm
	return &vm
}

func (cfg *Config) NewVM() *VM {
	vm := NewVM()
	vm.Config = cfg.Clone()
	return vm
}

func (vm *VM) Free() {
	if vm.handles != nil {
		for _, handle := range vm.handles {
			C.wrenReleaseHandle(vm.vm, handle)
		}
		vm.handles = nil
	}
	if vm.vm != nil {
		C.wrenFreeVM(vm.vm)
		vm.vm = nil
	}
}

type ResultCompileError struct{}

func (err *ResultCompileError) Error() string {
	return "Wren Error during compilation"
}

type ResultRuntimeError struct{}

func (err *ResultRuntimeError) Error() string {
	return "Wren Error during runtime"
}

type NilVMError struct{}

func (err *NilVMError) Error() string {
	return "Wren VM is nil"
}

func (vm *VM) InterpretString(module, source string) error {
	if vm.vm == nil {
		return &NilVMError{}
	}
	cModule := C.CString(module)
	cSource := C.CString(source)
	defer func() {
		C.free(unsafe.Pointer(cModule))
		C.free(unsafe.Pointer(cSource))
	}()
	results := C.wrenInterpret(vm.vm, cModule, cSource)
	return resultsToError(results)
}

func (vm *VM) InterpretFile(fileName string) error {
	if vm.vm == nil {
		return &NilVMError{}
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	return vm.InterpretString(fileName, string(data))
}

func resultsToError(results C.WrenInterpretResult) error {
	switch results {
	case C.WREN_RESULT_SUCCESS:
		return nil
	case C.WREN_RESULT_COMPILE_ERROR:
		return &ResultCompileError{}
	case C.WREN_RESULT_RUNTIME_ERROR:
		return &ResultRuntimeError{}
	default:
		panic("Unreachable")
	}
}

//export writeFn
func writeFn(v *C.WrenVM, text *C.char) {
	var output io.Writer
	if vm, ok := vmMap[v]; ok && vm.Config != nil {
		if vm.Config.WriteFn != nil {
			vm.Config.WriteFn(vm, C.GoString(text))
			return
		}
		if vm.Config.DefaultOutput != nil {
			output = vm.Config.DefaultOutput
		}
	}
	if output == nil && DefaultOutput != nil {
		output = DefaultOutput
	}
	if output != nil {
		io.WriteString(output, C.GoString(text))
	}
}

//export errorFn
func errorFn(v *C.WrenVM, errorType C.WrenErrorType, module *C.char, line C.int, message *C.char) {
	var output io.Writer
	var err error
	switch errorType {
	case C.WREN_ERROR_COMPILE:
		err = &CompileError{module: C.GoString(module), line: int(line), message: C.GoString(message)}
	case C.WREN_ERROR_RUNTIME:
		err = &RuntimeError{message: C.GoString(message)}
	case C.WREN_ERROR_STACK_TRACE:
		err = &StackTrace{module: C.GoString(module), line: int(line), message: C.GoString(message)}
	}
	if vm, ok := vmMap[v]; ok && vm.Config != nil {
		if vm.Config.ErrorFn != nil {
			vm.Config.ErrorFn(vm, err)
			return
		}
		if vm.Config.DefaultError != nil {
			output = vm.Config.DefaultError
		}
	}
	if DefaultError != nil {
		output = DefaultError
	}
	if output != nil {
		io.WriteString(output, err.Error())
	}
}

//export moduleLoaderFn
func moduleLoaderFn(v *C.WrenVM, name *C.char) *C.char {
	var source string
	if vm, ok := vmMap[v]; ok && vm.Config != nil && vm.Config.LoadModuleFn != nil {
		source = vm.Config.LoadModuleFn(vm, C.GoString(name))
	} else if DefaultModuleLoader != nil {
		source = DefaultModuleLoader(vm, C.GoString(name))
	}
	if &source != nil {
		// Wren should automatically frees this CString ...I think
		return C.CString(source)
	}
	return nil
}
