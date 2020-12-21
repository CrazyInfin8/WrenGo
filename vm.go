package wren

/*
#cgo CFLAGS:
#cgo LDFLAGS: -lm
#include "wren.h"

extern void writeFn(WrenVM*, char*);
extern void errorFn(WrenVM*, WrenErrorType, char*, int, char*);
extern WrenLoadModuleResult moduleLoaderFn(WrenVM*, char*);
extern WrenForeignMethodFn bindForeignMethodFn(WrenVM*, char*, char*, bool, char*);
extern WrenForeignClassMethods bindForeignClassFn(WrenVM*, char*, char*);
extern void foreignFinalizerFn(void*);
extern void invalidConstructor(WrenVM*);
extern void loadModuleCompleteFn(WrenVM*, char*, WrenLoadModuleResult);
*/
import "C"
import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sync"
	"unsafe"
)

// VM is an instance of Wren's virtual machine
type VM struct {
	vm        *C.WrenVM
	Config    *Config
	handles   map[*C.WrenHandle]*Handle
	bindMap   []ForeignMethodFn
	moduleMap ModuleMap
}

var (
	vmMap         map[*C.WrenVM]*VM = make(map[*C.WrenVM]*VM)
	vmMapMux      sync.RWMutex
	foreignMap    map[unsafe.Pointer]foreignInstance = make(map[unsafe.Pointer]foreignInstance)
	foreignMapMux sync.RWMutex
	// DefaultOutput is where Wren will print to if a VM's config doesn't specify its own output (Set this to nil to disable output)
	DefaultOutput io.Writer = os.Stdout
	// DefaultError is where Wren will send error messages to if a VM's config doesn't specify its own place for outputting errors (Set this to nil to disable output)
	DefaultError io.Writer = os.Stderr
	// DefaultModuleLoader allows Wren to import modules by loading files relative to the current directory (Set this to nil to disable importing or file access)
	DefaultModuleLoader LoadModuleFn = func(vm *VM, name string) (string, bool) {
		if data, err := ioutil.ReadFile(name); err == nil {
			return string(data), true
		}
		return "", false
	}
)

// NewVM creates a new instance of Wren's virtual machine with blank configurations
func NewVM() *VM {
	var config C.WrenConfiguration
	C.wrenInitConfiguration(&config)
	config.writeFn = C.WrenWriteFn(C.writeFn)
	config.errorFn = C.WrenErrorFn(C.errorFn)
	config.loadModuleFn = C.WrenLoadModuleFn(C.moduleLoaderFn)
	config.bindForeignMethodFn = C.WrenBindForeignMethodFn(C.bindForeignMethodFn)
	config.bindForeignClassFn = C.WrenBindForeignClassFn(C.bindForeignClassFn)
	vm := VM{vm: C.wrenNewVM(&config), handles: make(map[*C.WrenHandle]*Handle), bindMap: make([]ForeignMethodFn, 0), moduleMap: make(ModuleMap), Config: &Config{}}
	vmMapMux.Lock()
	defer vmMapMux.Unlock()
	vmMap[vm.vm] = &vm
	return &vm
}

// NewVM creates a new instance of Wren's virtual machine by cloning the config passed to it
func (cfg *Config) NewVM() *VM {
	vm := NewVM()
	vm.Config = cfg.Clone()
	return vm
}

// Free destroys the wren virtual machine and frees all handles tied to it. The VM should be freed when no longer in use. The VM should not be used after it has been freed
func (vm *VM) Free() {
	if vm.handles != nil {
		for _, handle := range vm.handles {
			handle.Free()
		}
		vm.handles = nil
	}
	if vm.vm != nil {
		vmMapMux.Lock()
		defer vmMapMux.Unlock()
		if _, ok := vmMap[vm.vm]; ok {
			delete(vmMap, vm.vm)
		}
		C.wrenFreeVM(vm.vm)
		vm.vm = nil
	}
}

// SetModule sets a foreign module for wren to import from (If a vm already imported classes and methods from this module already, changing it again won't set the previously imported values)
func (vm *VM) SetModule(name string, module *Module) {
	vm.moduleMap[name] = module.Clone()
}

// Merge combine all non nil values from `moduleMap` to the vm's own module map (If a vm already imported classes and methods from any module already, changing it again won't set the previously imported values)
func (vm *VM) Merge(moduleMap ModuleMap) {
	vm.moduleMap.Merge(moduleMap)
}

// ResultCompileError is returned from `InterpretString` or `InterpretFile` if there were problems compiling the Wren source code
type ResultCompileError struct{}

func (err *ResultCompileError) Error() string {
	return "Wren Error during compilation"
}

// ResultRuntimeError is returned from `InterpretString`, `InterpretFile`, or `Call` if there was a problem during script execution
type ResultRuntimeError struct{}

func (err *ResultRuntimeError) Error() string {
	return "Wren Error during runtime"
}

// NilVMError is returned if there was an attempt to use a VM that was freed already
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

// InterpretString compiles and runs wren source code from `source`. the module name of the source can be set with `module`. This function should not be called if the VM is currently running.
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

// InterpretFile compiles and runs wren source code from the given file. the module name would be set to the `fileName`, This function should not be called if the VM is currently running.
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

// Handle is a generic handle from wren
type Handle struct {
	handle *C.WrenHandle
	vm     *VM
}

func (vm *VM) createHandle(handle *C.WrenHandle) *Handle {
	h := &Handle{handle: handle, vm: vm}
	vm.handles[h.handle] = h
	return h
}

// Handle returns the generic handle
func (h *Handle) Handle() *Handle {
	return h
}

// VM returns the vm that this handle belongs to
func (h *Handle) VM() *VM {
	return h.vm
}

// Free releases the handle tied to it. The handle should be freed when no longer in use. The handle should not be used after it has been freed
func (h *Handle) Free() {
	if h.handle != nil {
		C.wrenReleaseHandle(h.vm.vm, h.handle)
		h.handle = nil
	}
	if _, ok := h.vm.handles[h.handle]; ok {
		delete(h.vm.handles, h.handle)
	}
}

// Func creates a callable handle from the wren object tied to the current handle. There isn't currently a way to check if the function referenced from `signature` exists before calling it
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

// NilHandleError is returned if there was an attempt to use a `Handle` that was freed already
type NilHandleError struct {
}

func (err *NilHandleError) Error() string {
	return "Wren Handle is nil"
}

// KeyNotExist is returned if there was an attempt to access a key value from a map that doesn't exist yet
type KeyNotExist struct {
	Map *MapHandle
	Key interface{}
}

func (err *KeyNotExist) Error() string {
	return "MapHandle does not have a value for this key"
}

// MapHandle is a handle to a map object in Wren
type MapHandle struct {
	handle *Handle
}

// Handle returns the generic handle it this `MapHandle` is tied to
func (h *MapHandle) Handle() *Handle {
	return h.handle
}

// VM returns the vm that this handle belongs to
func (h *MapHandle) VM() *VM {
	return h.handle.vm
}

// Free releases the handle tied to it. The handle should be freed when no longer in use. The handle should not be used after it has been freed
func (h *MapHandle) Free() {
	h.handle.Free()
}

// UnexpectedValue is returned if Wren did not create the correct type (probably might panic from being out of memory or something before that but just to be safe)
type UnexpectedValue struct {
	Value interface{}
}

func (err *UnexpectedValue) Error() string {
	return "Unexpected value"
}

// NewMap creates a new empty map object in wren and returns it's handle
func (vm *VM) NewMap() (*MapHandle, error) {
	if vm.vm == nil {
		return nil, &NilVMError{}
	}
	C.wrenEnsureSlots(vm.vm, 1)
	C.wrenSetSlotNewMap(vm.vm, 0)
	value := vm.getSlotValue(0)
	mapHandle, ok := value.(*MapHandle)
	if !ok {
		return nil, &UnexpectedValue{Value: value}
	}
	return mapHandle, nil
}

// InvalidKey is returned if there was an attempt to access a maps value with a key that is not of type number, boolean, string, or nil
type InvalidKey struct {
	Map *MapHandle
	Key interface{}
}

func (err *InvalidKey) Error() string {
	return fmt.Sprintf("Type \"%v\" is not a valid type for map key", reflect.TypeOf(err.Key).String())
}

// Get tries to return the value in the Wren map with the key `key`
func (h *MapHandle) Get(key interface{}) (interface{}, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 3)
	vm.setSlotValue(handle, 0)
	handle.vm.setSlotValue(key, 1)
	switch C.wrenGetSlotType(vm.vm, 1) {
	case C.WREN_TYPE_NUM, C.WREN_TYPE_STRING, C.WREN_TYPE_BOOL, C.WREN_TYPE_NULL:
	default:
		return nil, &InvalidKey{Map: h, Key: key}
	}
	if bool(C.wrenGetMapContainsKey(vm.vm, 0, 1)) {
		C.wrenGetMapValue(vm.vm, 0, 1, 2)
		v := vm.getSlotValue(2)
		return v, nil
	}
	return nil, &KeyNotExist{Map: h, Key: key}
}

// Set tries to set the value in the Wren map with the key `key`
func (h *MapHandle) Set(key, value interface{}) error {
	handle := h.Handle()
	if handle.handle == nil {
		return &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 3)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(key, 1)
	switch C.wrenGetSlotType(vm.vm, 1) {
	case C.WREN_TYPE_NUM, C.WREN_TYPE_STRING, C.WREN_TYPE_BOOL, C.WREN_TYPE_NULL:
	default:
		return &InvalidKey{Map: h, Key: key}
	}
	vm.setSlotValue(value, 2)
	C.wrenSetMapValue(vm.vm, 0, 1, 2)
	return nil
}

// Delete removes a value from the Wren map with the key `key`
func (h *MapHandle) Delete(key interface{}) (interface{}, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 3)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(key, 1)
	switch C.wrenGetSlotType(vm.vm, 1) {
	case C.WREN_TYPE_NUM, C.WREN_TYPE_STRING, C.WREN_TYPE_BOOL, C.WREN_TYPE_NULL:
	default:
		return nil, &InvalidKey{Map: h, Key: key}
	}
	C.wrenRemoveMapValue(vm.vm, 0, 1, 2)
	return vm.getSlotValue(2), nil
}

// Has check if a wren map has a value with the key `key`
func (h *MapHandle) Has(key interface{}) (bool, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return false, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 2)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(key, 1)
	switch C.wrenGetSlotType(vm.vm, 1) {
	case C.WREN_TYPE_NUM, C.WREN_TYPE_STRING, C.WREN_TYPE_BOOL, C.WREN_TYPE_NULL:
	default:
		return false, &InvalidKey{Map: h, Key: key}
	}
	return bool(C.wrenGetMapContainsKey(vm.vm, 0, 1)), nil
}

// Count counts how many elements are in the Wren map
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

// Func creates a callable handle from the Wren object tied to the current handle. There isn't currently a way to check if the function referenced from `signature` exists before calling it
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

// Copy creates a new `MapHandle` tied to this Wren map, if the previous one is freed the new one should still persist
func (h *MapHandle) Copy() (*MapHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 0)
	vm.setSlotValue(handle, 0)
	return &MapHandle{handle: vm.createHandle(C.wrenGetSlotHandle(vm.vm, 0))}, nil
}

// ListHandle is a handle to a list object in Wren
type ListHandle struct {
	handle *Handle
}

// Free releases the handle tied to it. The handle should be freed when no longer in use. The handle should not be used after it has been freed
func (h *ListHandle) Free() {
	h.handle.Free()
}

// VM returns the vm that this handle belongs to
func (h *ListHandle) VM() *VM {
	return h.handle.vm
}

// Handle returns the generic handle it this `ListHandle` is tied to
func (h *ListHandle) Handle() *Handle {
	return h.handle
}

// NewList creates a new empty list object in wren and returns it's handle
func (vm *VM) NewList() (*ListHandle, error) {
	if vm.vm == nil {
		return nil, &NilVMError{}
	}
	C.wrenEnsureSlots(vm.vm, 1)
	C.wrenSetSlotNewList(vm.vm, 0)
	value := vm.getSlotValue(0)
	listHandle, ok := value.(*ListHandle)
	if !ok {
		return nil, &UnexpectedValue{Value: value}
	}
	return listHandle, nil
}

// OutOfBounds is returned if there was an attempt to access a lists value at an index that hasn't been set yet
type OutOfBounds struct {
	List  *ListHandle
	Index int
}

func (err *OutOfBounds) Error() string {
	return fmt.Sprintf("Index %v is out of bounds", err.Index)
}

// Get tries to return the value in the Wren list at the index `index`
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

// Insert tries to insert an element into the wren list at the end
func (h *ListHandle) Insert(value interface{}) error {
	handle := h.Handle()
	if handle.handle == nil {
		return &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 2)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(value, 1)
	C.wrenInsertInList(vm.vm, 0, -1, 1)
	return nil
}

// InsertAt tries to insert an element into the wren list at index `index`
func (h *ListHandle) InsertAt(index int, value interface{}) error {
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

// Count counts how many elements are in the Wren list
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

// Set tries to set the value in the Wren list at the index `index`
func (h *ListHandle) Set(index int, value interface{}) error {
	handle := h.Handle()
	if handle.handle == nil {
		return &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 2)
	vm.setSlotValue(handle, 0)
	vm.setSlotValue(value, 1)
	C.wrenSetListElement(vm.vm, 0, C.int(index), 1)
	return nil

}

// Func creates a callable handle from the Wren object tied to the current handle. There isn't currently a way to check if the function referenced from `signature` exists before calling it
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

// Copy creates a new `ListHandle` tied to this Wren list, if the previous one is freed the new one should still persist
func (h *ListHandle) Copy() (*ListHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 0)
	vm.setSlotValue(handle, 0)
	return &ListHandle{handle: vm.createHandle(C.wrenGetSlotHandle(vm.vm, 0))}, nil
}

// ForeignHandle is a handle to a foreign object in Wren
type ForeignHandle struct {
	handle *Handle
}

// Free releases the handle tied to it. The handle should be freed when no longer in use. The handle should not be used after it has been freed
func (h *ForeignHandle) Free() {
	h.handle.Free()
}

// VM returns the vm that this handle belongs to
func (h *ForeignHandle) VM() *VM {
	return h.handle.vm
}

// Handle returns the generic handle it this `MapHandle` is tied to
func (h *ForeignHandle) Handle() *Handle {
	return h.handle
}

// Func creates a callable handle from the Wren object tied to the current handle. There isn't currently a way to check if the function referenced from `signature` exists before calling it
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

// UnknownForeign is returned if a foreign value was not set by WrenGo
type UnknownForeign struct {
	Handle *ForeignHandle
}

func (err *UnknownForeign) Error() string {
	return "Unknown foreign handle"
}

// Get tries to get the original value that this `ForeignHandle` set to
func (h *ForeignHandle) Get() (interface{}, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.handle.vm
	C.wrenEnsureSlots(vm.vm, 1)
	vm.setSlotValue(h.handle, 0)
	ptr := C.wrenGetSlotForeign(vm.vm, 0)
	foreignMapMux.RLock()
	defer foreignMapMux.RUnlock()
	if foreign, ok := foreignMap[ptr]; ok {
		return foreign.value, nil
	}
	return nil, &UnknownForeign{Handle: h}
}

// Copy creates a new `ForeignHandle` tied to this foreign object, if the previous one is freed the new one should still persist
func (h *ForeignHandle) Copy() (*ForeignHandle, error) {
	handle := h.Handle()
	if handle.handle == nil {
		return nil, &NilHandleError{}
	}
	vm := h.VM()
	C.wrenEnsureSlots(vm.vm, 0)
	vm.setSlotValue(handle, 0)
	return &ForeignHandle{handle: vm.createHandle(C.wrenGetSlotHandle(vm.vm, 0))}, nil
}

// CallHandle is a handle to a wren function
type CallHandle struct {
	receiver *Handle
	handle   *Handle
}

// Free releases the handle tied to it. The handle should be freed when no longer in use. The handle should not be used after it has been freed
func (h *CallHandle) Free() {
	h.handle.Free()
}

// Call tries to call the function on the handles that created the `CallHandle`. The amount of parameters should coorespond to the signature used to create this function. This function should not be called if the VM is already running.
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

type freeable interface {
	Free()
}

// FreeAll can take any argument type. It filters through and calls free on any handles passed. It does not free anything else
func (vm *VM) FreeAll(items ...interface{}) {
	for _, item := range items {
		switch item.(type) {
		case *Handle, *CallHandle, *ForeignHandle, *ListHandle, *MapHandle:
			item.(freeable).Free()
		}
	}
}

// GC runs the garbage collector on the `VM`
func (vm *VM) GC() {
	C.wrenCollectGarbage(vm.vm)
}

func (vm *VM) getAllSlots() []interface{} {
	slotCount := int(C.wrenGetSlotCount(vm.vm))
	params := make([]interface{}, slotCount)
	for i := 0; i < slotCount; i++ {
		params[i] = vm.getSlotValue(i)
	}
	return params
}

func (vm *VM) getSlotValue(slot int) (value interface{}) {
	cSlot := C.int(slot)
	switch C.wrenGetSlotType(vm.vm, C.int(cSlot)) {
	case C.WREN_TYPE_BOOL:
		return bool(C.wrenGetSlotBool(vm.vm, cSlot))
	case C.WREN_TYPE_NUM:
		return float64(C.wrenGetSlotDouble(vm.vm, cSlot))
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

// InvalidValue is returned if there was an attempt to pass a value to Wren that WrenGo cannot process. Note that Go maps, lists, and slices (other than byte slices), may also send this error. `ListHandle`s and `MapHandle`s should be used instead of list and maps.
type InvalidValue struct {
	Value interface{}
}

func (err InvalidValue) Error() string {
	return fmt.Sprintf("WrenGo does not know how to handle the value type \"%v\"", reflect.TypeOf(err.Value).String())
}

// NonMatchingVM is returned if there was an attempt to use a handle in a VM that it did not originate from
type NonMatchingVM struct{}

func (err *NonMatchingVM) Error() string {
	return "Cannot set value to VM because it didn't originate from this VM"
}

func (vm *VM) setSlotValue(value interface{}, slot int) error {
	cSlot := C.int(slot)
	switch value.(type) {
	case *Handle:
		handle := value.(*Handle)
		if handle.VM() != vm {
			return &NonMatchingVM{}
		}
		cValue := handle.handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case *ListHandle:
		handle := value.(*ListHandle)
		if handle.VM() != vm {
			return &NonMatchingVM{}
		}
		cValue := handle.handle.handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case *MapHandle:
		handle := value.(*MapHandle)
		if handle.VM() != vm {
			return &NonMatchingVM{}
		}
		cValue := handle.handle.handle
		C.wrenSetSlotHandle(vm.vm, cSlot, cValue)
	case *ForeignHandle:
		handle := value.(*ForeignHandle)
		if handle.VM() != vm {
			return &NonMatchingVM{}
		}
		cValue := handle.handle.handle
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

// NoSuchVariable is returned when `GetVariable` cannot get a variable from a module
type NoSuchVariable struct {
	Module, Name string
}

// NoSuchModule is returned when `GetVariable` cannot find a module
type NoSuchModule struct {
	Module string
}

func (err *NoSuchVariable) Error() string {
	return fmt.Sprintf("Module \"%s\" does not contain variable \"%s\"", err.Module, err.Name)
}
func (err *NoSuchModule) Error() string {
	return fmt.Sprintf("Module \"%s\" has not been resolved by this VM yet", err.Module)
}

// GetVariable tries to get a variable from the Wren vm with the given module name and variable name. This function checks that `HasVariable` is true to prevent segfaults
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
	if !C.wrenHasModule(vm.vm, cModule) {
		return nil, &NoSuchModule{Module: module}
	}
	if !C.wrenHasVariable(vm.vm, cModule, cName) {
		return nil, &NoSuchVariable{Module: module, Name: name}
	}
	C.wrenEnsureSlots(vm.vm, 1)
	C.wrenGetVariable(vm.vm, cModule, cName, 0)
	return vm.getSlotValue(0), nil
}

// GetVariableUnsafe is like `GetVariable` but does not perform any checks to ensure that things aren't null (This function will segfault if things don't exist)
func (vm *VM) GetVariableUnsafe(module, name string) interface{} {
	// TODO: May add more of these "Unsafe" functions for simplicity and performance?
	cModule := C.CString(module)
	cName := C.CString(name)
	defer func() {
		C.free(unsafe.Pointer(cModule))
		C.free(unsafe.Pointer(cName))
	}()
	C.wrenEnsureSlots(vm.vm, 1)
	C.wrenGetVariable(vm.vm, cModule, cName, 0)
	return vm.getSlotValue(0)

}

// HasVariable tries to check that a variable from the Wren vm with the given module name and variable name exists. This function checks that `HasModule` is true to prevent segfaults
func (vm *VM) HasVariable(module, name string) bool {
	cModule := C.CString(module)
	cName := C.CString(name)
	if vm.vm == nil || !C.wrenHasModule(vm.vm, cModule) {
		return false
	}
	defer func() {
		C.free(unsafe.Pointer(cModule))
		C.free(unsafe.Pointer(cName))
	}()
	return bool(vm.vm != nil && C.wrenHasModule(vm.vm, cModule) && C.wrenHasVariable(vm.vm, cModule, cName))
}

// HasModule tries to check that a module has been imported or resolved before
func (vm *VM) HasModule(module string) bool {
	if vm.vm == nil {
		return false
	}
	cModule := C.CString(module)
	defer C.free(unsafe.Pointer(cModule))
	return bool(vm.vm != nil && C.wrenHasModule(vm.vm, cModule))
}

// Abort stops the running Wren fiber and throws the error passed to it
func (vm *VM) Abort(err error) {
	C.wrenEnsureSlots(vm.vm, 1)
	if err != nil {
		vm.setSlotValue(err.Error(), 0)
	} else {
		vm.setSlotValue("Fiber Aborted", 0)
	}
	C.wrenAbortFiber(vm.vm, 0)
}

//export writeFn
func writeFn(v *C.WrenVM, text *C.char) {
	var output io.Writer
	unlocked := false
	vmMapMux.RLock()
	defer func() {
		if !unlocked {
			vmMapMux.RUnlock()
		}
	}()
	if vm, ok := vmMap[v]; ok {
		vmMapMux.RUnlock()
		unlocked = true
		if vm.Config != nil {
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
	unlocked := false
	vmMapMux.RLock()
	defer func() {
		if !unlocked {
			vmMapMux.RUnlock()
		}
	}()
	if vm, ok := vmMap[v]; ok {
		vmMapMux.RUnlock()
		unlocked = true
		if vm.Config != nil {
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
			io.WriteString(output, err.Error()+"\n")
		}
	}
}

//export moduleLoaderFn
func moduleLoaderFn(v *C.WrenVM, name *C.char) C.WrenLoadModuleResult {
	unlocked := false
	vmMapMux.RLock()
	defer func() {
		if !unlocked {
			vmMapMux.RUnlock()
		}
	}()
	if vm, ok := vmMap[v]; ok {
		vmMapMux.RUnlock()
		unlocked = true
		var source string
		if vm.Config != nil && vm.Config.LoadModuleFn != nil {
			source, ok = vm.Config.LoadModuleFn(vm, C.GoString(name))
		} else if DefaultModuleLoader != nil {
			source, ok = DefaultModuleLoader(vm, C.GoString(name))
		}
		if ok {
			return C.WrenLoadModuleResult{
				source:     C.CString(source),
				onComplete: C.WrenLoadModuleCompleteFn(C.loadModuleCompleteFn),
				// could potentially do something here but don't know what
				// also potentially would have breaking changes
				userData: nil,
			}
		}
	}
	return C.WrenLoadModuleResult{
		source:     nil,
		onComplete: nil,
		userData:   nil,
	}
}

//export loadModuleCompleteFn
func loadModuleCompleteFn(vm *C.WrenVM, name *C.char, res C.WrenLoadModuleResult) {
	C.free(unsafe.Pointer(res.source))
	// res.userData should already be nil
}

//export bindForeignMethodFn
func bindForeignMethodFn(v *C.WrenVM, cModule *C.char, cClassName *C.char, cIsStatic C.bool, cSignature *C.char) C.WrenForeignMethodFn {
	unlocked := false
	vmMapMux.RLock()
	defer func() {
		if !unlocked {
			vmMapMux.RUnlock()
		}
	}()
	if vm, ok := vmMap[v]; ok {
		vmMapMux.RUnlock()
		unlocked = true
		if module, ok := vm.moduleMap[C.GoString(cModule)]; ok {
			if class, ok := module.ClassMap[C.GoString(cClassName)]; ok {
				var name string
				if bool(cIsStatic) {
					name = "static " + C.GoString(cSignature)
				} else {
					name = C.GoString(cSignature)
				}
				if fn, ok := class.MethodMap[name]; ok {
					foreignMethod, err := vm.registerFunc(fn)
					if err != nil {
						panic(err.Error())
					}
					return foreignMethod
				}
			}
		}
	}
	return nil
}

type foreignInstance struct {
	finalizer ForeignFinalizer
	vm        *VM
	value     interface{}
}

//export invalidConstructor
func invalidConstructor(v *C.WrenVM) {
	C.wrenEnsureSlots(v, 1)
	err := C.CString("Foreign class does implement a constructor.")
	defer C.free(unsafe.Pointer(err))
	C.wrenSetSlotString(v, 0, err)
	C.wrenAbortFiber(v, 0)
}

//export bindForeignClassFn
func bindForeignClassFn(v *C.WrenVM, cModule *C.char, cClassName *C.char) C.WrenForeignClassMethods {
	unlocked := false
	vmMapMux.RLock()
	defer func() {
		if !unlocked {
			vmMapMux.RUnlock()
		}
	}()
	if vm, ok := vmMap[v]; ok {
		vmMapMux.RUnlock()
		unlocked = true
		if module, ok := vm.moduleMap[C.GoString(cModule)]; ok {
			if class, ok := module.ClassMap[C.GoString(cClassName)]; ok {
				initializer, err := vm.registerFunc(
					func(vm *VM, parameters []interface{}) (interface{}, error) {
						var (
							foreign interface{}
							err     error
						)
						if class.Initializer != nil {
							foreign, err = class.Initializer(vm, parameters)
						}
						if err != nil {
							return nil, err
						}
						ptr := C.wrenSetSlotNewForeign(vm.vm, 0, 0, 1)
						foreignMapMux.Lock()
						defer foreignMapMux.Unlock()
						foreignMap[ptr] = foreignInstance{
							finalizer: class.Finalizer,
							vm:        vm,
							value:     foreign,
						}
						return nil, nil
					},
				)
				if err != nil {
					panic(err.Error())
				}
				return C.WrenForeignClassMethods{
					finalize: C.WrenFinalizerFn(C.foreignFinalizerFn),
					allocate: initializer,
				}
			}
		}
	}
	if C.GoString(cModule) == "random" {
		return C.WrenForeignClassMethods{
			allocate: nil,
			finalize: nil,
		}
	}
	return C.WrenForeignClassMethods{
		allocate: C.WrenForeignMethodFn(C.invalidConstructor),
	}
}

//export foreignFinalizerFn
func foreignFinalizerFn(ptr unsafe.Pointer) {
	unlocked := false
	foreignMapMux.RLock()
	defer func() {
		if !unlocked {
			foreignMapMux.RUnlock()
		}
	}()
	if foreign, ok := foreignMap[ptr]; ok {
		foreignMapMux.RUnlock()
		unlocked = true
		if foreign.finalizer != nil {
			foreign.finalizer(foreign.vm, foreign.value)
		}
		delete(foreignMap, ptr)
	}
}
