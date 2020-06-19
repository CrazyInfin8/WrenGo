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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"unsafe"
)

type VM struct {
	vm      *C.WrenVM
	Config  *Config
	errChan chan error
	handles map[*C.WrenHandle]*Handle
}

var (
	vmMap map[*C.WrenVM]*VM = make(map[*C.WrenVM]*VM)
	// DefaultOutput is where wren will print to if a VM's config doesn't specify its own output (Set this to nil to disable output)
	DefaultOutput io.Writer = os.Stdout
	// DefaultError is where wren will send error messages to if a VM's config doesn't specify its own place for outputting errors (Set this to nil to disable output)
	DefaultError io.Writer = os.Stderr
	// DefaultModuleLoader allows wren to import modules by loading files relative to the current directory (Set this to nil to disable importing or file access)
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
	vm := VM{vm: C.wrenNewVM(&config), handles: make(map[*C.WrenHandle]*Handle)}
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
			handle.Free()
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

type Handle struct {
	handle *C.WrenHandle
	vm     *VM
}

func (vm *VM) createHandle(handle *C.WrenHandle) *Handle {
	h := &Handle{handle: handle, vm: vm}
	vm.handles[h.handle] = h
	return h
}

func (h *Handle) Handle() *Handle {
	return h
}

func (h *Handle) VM() *VM {
	return h.vm
}

func (h *Handle) Free() {
	if h.handle != nil {
		C.wrenReleaseHandle(h.vm.vm, h.handle)
		h.handle = nil
	}
	if _, ok := h.vm.handles[h.handle]; ok {
		delete(h.vm.handles, h.handle)
	}
}

func (h *Handle) Func(signature string) (*CallHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	cSignature := C.CString(signature)
	defer C.free(unsafe.Pointer(cSignature))
	vm := h.VM()
	return &CallHandle{receiver: handle, handle: vm.createHandle(C.wrenMakeCallHandle(vm.vm, cSignature))}, nil
}

type NilHandleError struct {
}

func (err *NilHandleError) Error() string {
	return "Wren Handle is nil"
}

type KeyNotExist struct {
	Map *MapHandle
	Key interface{}
}

func (err *KeyNotExist) Error() string {
	return "MapHandle does not have a value for this key"
}

type MapHandle struct {
	handle *Handle
}

func (h *MapHandle) Handle() *Handle {
	return h.handle
}

func (h *MapHandle) VM() *VM {
	return h.handle.vm
}

func (h *MapHandle) Free() {
	h.handle.Free()
}

func (h *MapHandle) Get(key interface{}) (interface{}, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 3)
	vm.setSlotValue(handle, 0)
	handle.vm.setSlotValue(key, 1)
	if bool(C.wrenGetMapContainsKey(vm.vm, 0, 1)) {
		C.wrenGetMapValue(vm.vm, 0, 1, 2)
		v := vm.getSlotValue(2)
		return v, nil
	}
	return nil, &KeyNotExist{Map: h, Key: key}
}

func (h *MapHandle) Set(key, value interface{}) error {
	handle := h.Handle()
	if handle.handle == nil {
		return &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 3)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(key, 1)
	vm.setSlotValue(value, 2)
	C.wrenSetMapValue(vm.vm, 0, 1, 2)
	return nil
}

func (h *MapHandle) Delete(key interface{}) (interface{}, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 3)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(key, 1)
	C.wrenRemoveMapValue(vm.vm, 0, 1, 2)
	return vm.getSlotValue(2), nil
}

func (h *MapHandle) Has(key interface{}) (bool, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return false, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 2)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(key, 1)
	return bool(C.wrenGetMapContainsKey(vm.vm, 0, 1)), nil
}

func (h *MapHandle) Count() (int, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return 0, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 1)
	vm.setSlotValue(handle, 0)
	return int(C.wrenGetMapCount(vm.vm, 0)), nil
}

func (h *MapHandle) Func(signature string) (*CallHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	cSignature := C.CString(signature)
	defer C.free(unsafe.Pointer(cSignature))
	vm := h.VM()
	return &CallHandle{receiver: handle, handle: vm.createHandle(C.wrenMakeCallHandle(vm.vm, cSignature))}, nil
}

type ListHandle struct {
	handle *Handle
}

func (h *ListHandle) Free() {
	h.handle.Free()
}

func (h *ListHandle) VM() *VM {
	return h.handle.vm
}

func (h *ListHandle) Handle() *Handle {
	return h.handle
}

type OutOfBounds struct {
	List  *ListHandle
	Index int
}

func (err *OutOfBounds) Error() string {
	return fmt.Sprintf("Index %v is out of bounds", err.Index)
}

func (h *ListHandle) Get(index int) (interface{}, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 2)
	vm.setSlotValue(handle, 0)
	if index < 0 || index >= int(C.wrenGetListCount(vm.vm, C.int(index))) {
		return nil, &OutOfBounds{List: h, Index: index}
	}
	C.wrenGetListElement(vm.vm, 0, C.int(index), 1)
	return vm.getSlotValue(1), nil
}

func (h *ListHandle) Set(index int, value interface{}) error {
	handle := h.Handle()
	if handle.handle == nil {
		return &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 2)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(value, 1)
	C.wrenInsertInList(vm.vm, 0, C.int(index), 1)
	return nil
}

func (h *ListHandle) Count() (int, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return 0, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 1)
	vm.setSlotValue(handle, 0)
	return int(C.wrenGetListCount(vm.vm, 0)), nil
}

func (h *ListHandle) Func(signature string) (*CallHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	cSignature := C.CString(signature)
	defer C.free(unsafe.Pointer(cSignature))
	vm := h.VM()
	return &CallHandle{receiver: handle, handle: vm.createHandle(C.wrenMakeCallHandle(vm.vm, cSignature))}, nil
}

type ForeignHandle struct {
	handle *Handle
}

func (h *ForeignHandle) Free() {
	h.handle.Free()
}

func (h *ForeignHandle) VM() *VM {
	return h.handle.vm
}

func (h *ForeignHandle) Handle() *Handle {
	return h.handle
}

func (h *ForeignHandle) Func(signature string) (*CallHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	cSignature := C.CString(signature)
	defer C.free(unsafe.Pointer(cSignature))
	vm := h.VM()
	return &CallHandle{receiver: handle, handle: vm.createHandle(C.wrenMakeCallHandle(vm.vm, cSignature))}, nil
}

type CallHandle struct {
	receiver *Handle
	handle   *Handle
}

func (h *CallHandle) Free() {
	h.handle.Free()
}

func (h *CallHandle) Call(parameters ...interface{}) (interface{}, error) {
	handle := h.handle
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.handle.vm
	slots := C.int(len(parameters) + 1)
	C.wrenEnsureSlots(vm.vm, slots)
	vm.setSlotValue(h.receiver, 0)
	for i, param := range parameters {
		err := vm.setSlotValue(param, i+1)
		if err != nil {
			return nil, err
		}
	}
	err := resultsToError(C.wrenCall(vm.vm, handle.handle))
	if err != nil {
		return nil, err
	}
	return vm.getSlotValue(0), nil
}

func (vm *VM) getSlotValue(slot int) (value interface{}) {
	cSlot := C.int(slot)
	switch C.wrenGetSlotType(vm.vm, C.int(cSlot)) {
	case C.WREN_TYPE_BOOL:
		return bool(C.wrenGetSlotBool(vm.vm, cSlot))
	case C.WREN_TYPE_NUM:
		return float32(C.wrenGetSlotDouble(vm.vm, cSlot))
	case C.WREN_TYPE_FOREIGN:
		return &ForeignHandle{handle: vm.createHandle(C.wrenGetSlotHandle(vm.vm, cSlot))}
	case C.WREN_TYPE_LIST:
		return &ListHandle{handle: vm.createHandle(C.wrenGetSlotHandle(vm.vm, cSlot))}
	case C.WREN_TYPE_MAP:
		return &MapHandle{handle: vm.createHandle(C.wrenGetSlotHandle(vm.vm, cSlot))}
	case C.WREN_TYPE_NULL:
		return nil
	case C.WREN_TYPE_STRING:
		var length C.int
		str := C.wrenGetSlotBytes(vm.vm, cSlot, &length)
		return string(C.GoBytes(unsafe.Pointer(str), length))
	case C.WREN_TYPE_UNKNOWN:
		return vm.createHandle(C.wrenGetSlotHandle(vm.vm, cSlot))
	default:
		panic("Unreachable")
	}
}

type InvalidValue struct {
	Value interface{}
}

func (err InvalidValue) Error() string {
	return fmt.Sprintf("WrenGo does not know how to handle the value type \"%v\"", reflect.TypeOf(err.Value).String())
}

func (vm *VM) setSlotValue(value interface{}, slot int) error {
	cSlot := C.int(slot)
	switch value.(type) {
	case *Handle:
		cValue := value.(*Handle).handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case *ListHandle:
		cValue := value.(*ListHandle).handle.handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case *MapHandle:
		cValue := value.(*MapHandle).handle.handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case *ForeignHandle:
		cValue := value.(*ForeignHandle).handle.handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case []byte:
		data := value.([]byte)
		cValue := C.CBytes(data)
		C.wrenSetSlotBytes(vm.vm, cSlot, (*C.char)(cValue), C.size_t(len(data)))
	case bool:
		cValue := C.bool(value.(bool))
		C.wrenSetSlotBool(vm.vm, cSlot, cValue)
	case string:
		data := []byte(value.(string))
		cValue := C.CBytes(data)
		defer C.free(unsafe.Pointer(cValue))
		C.wrenSetSlotBytes(vm.vm, cSlot, (*C.char)(cValue), C.size_t(len(data)))
	default:
		switch v := reflect.ValueOf(value); v.Kind() {
		case reflect.Float32, reflect.Float64:
			cValue := C.double(v.Float())
			C.wrenSetSlotDouble(vm.vm, cSlot, cValue)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			cValue := C.double(v.Int())
			C.wrenSetSlotDouble(vm.vm, cSlot, cValue)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			cValue := C.double(v.Uint())
			C.wrenSetSlotDouble(vm.vm, cSlot, cValue)
		case reflect.Invalid:
			C.wrenSetSlotNull(vm.vm, cSlot)
		default:
			C.wrenSetSlotNull(vm.vm, cSlot)
			return &InvalidValue{Value: value}
		}
	}
	return nil
}

func (vm *VM) GetVariable(module, name string) (interface{}, error) {
	if vm.vm == nil {
		return nil, &NilVMError{}
	}
	cModule := C.CString(module)
	cName := C.CString(name)
	defer func() {
		C.free(unsafe.Pointer(cModule))
		C.free(unsafe.Pointer(cName))
	}()
	C.wrenEnsureSlots(vm.vm, 1)
	// There isn't currently a way to check if
	// variable exists
	C.wrenGetVariable(vm.vm, cModule, cName, 0)
	return vm.getSlotValue(0), nil
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
