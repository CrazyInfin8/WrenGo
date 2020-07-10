package wren

/*
#cgo CFLAGS:
#cgo LDFLAGS: -lm
#include "wren.h"
*/
import "C"

import (
	"bytes"
	"fmt"
)

// ForeignMethodFn is a function that wren can import or call. The value of parameters[0] will be the foreign object itself while anything after that are the parameters from the wren function. if it returns an error, then it will call `vm.Abort`
type ForeignMethodFn func(vm *VM, parameters []interface{}) (interface{}, error)

// ForeignInitializer is a function used to initialize a foreign class instance. The value of parameter[0] will be the foreign class while anything after that are the parameters from the wren constructor. Whatever it returns for `interface{}` will be the the foreign instance of the foreign class
type ForeignInitializer func(vm *VM, parameters []interface{}) (interface{}, error)

// ForeignFinalizer is a function called when Wren garbage collects the forign object it is tied to (note that maintaining handles will prevent the foreign object from being garbage collected)
type ForeignFinalizer func(vm *VM, data interface{})

// ModuleMap is a map containing Module organized by module names
type ModuleMap map[string]*Module

// Module contains a `ClassMap` which is a map containing foreign classes (or classes where objects are made in Go and not Wren) organized by class name
type Module struct {
	ClassMap ClassMap
}

// ClassMap is a map containing all foreign classes (or classes where objects are made in Go and not Wren) organized by class name
type ClassMap map[string]*ForeignClass

// ForeignClass details a foreign class (or classes where objects are made in Go and not Wren) for Wren to use. Whenever Wren runs a constructor for this class, `Initializer` is called. Whatever the first value `Initializer` returns will be used as the instance for this foreign class.
type ForeignClass struct {
	// Wren will call this function as a constructor. Whatever it returns for `interface{}` will be the the foreign instance of this foreign class
	Initializer ForeignInitializer
	// Wren will call this function when Wren's garbage collector collects the forign class (not that maintaining handles will prevent the foreign object from being garbage collected)
	Finalizer ForeignFinalizer
	// A map containing `ForeignMethodFn`s organized by function signatures. see MethodMap for mor information on signatures syntax.
	MethodMap MethodMap
}

// MethodMap is a map containing `ForeignMethodFn`s organized by signatures.
//
// Signatures have specific syntax in order for wren to know which function to use.
//
// - If the function is static then it will begin with the string "static " (note the space after it).
//
// - The name of the function is required to match how it will look in Wren
//
// - Next will be an open paranthesis("(")
//
// - for the amount of expected parameters, add underscores seperated by comma. do not add a trailing comma after the last underscore
//
// - close everything up with a closing parenthesis (")")
//
// For example:
//
// - A static function called "foo" with 3 parameters will look like "static foo(_,_,_)"
//
// - A function that isn't static called "bar" with no parameters will look like "static bar()"
type MethodMap map[string]ForeignMethodFn

// Clone creates a copy clone of all modules and classes this `ModuleMap` references
func (modules ModuleMap) Clone() ModuleMap {
	newMap := make(ModuleMap)
	for name, module := range modules {
		newMap[name] = module.Clone()
	}
	return newMap
}

// Merge goes through all items in the source `ModuleMap` and adds them if they are not nil
func (modules ModuleMap) Merge(source ModuleMap) ModuleMap {
	for name, module := range source {
		if module != nil {
			modules[name].ClassMap.Merge(module.ClassMap)
		}
	}
	return modules
}

// Clone creates a copy of all classes this `Module` references
func (module *Module) Clone() *Module {
	return NewModule(module.ClassMap.Clone())
}

// NewModule creates a new `Module` from the given `ClassMap`
func NewModule(classes ClassMap) *Module {
	return &Module{ClassMap: classes.Clone()}
}

// Clone creates a copy of all classes this `ClassMap` references
func (classes ClassMap) Clone() ClassMap {
	newMap := make(ClassMap)
	for name, class := range classes {
		newMap[name] = class.Clone()
	}
	return newMap
}

// Merge goes through all items in the source `ClassMap` and adds them if they are not nil
func (classes ClassMap) Merge(source ClassMap) ClassMap {
	for name, class := range source {
		if class != nil {
			classes[name].MethodMap.Merge(class.MethodMap)
		}
	}
	return classes
}

// NewClass creates a new `ForeignClass` with the given `ForeignInitializer` function, `ForeignFinalizer` function, and `MethodMap`
func NewClass(initializer ForeignInitializer, finalizer ForeignFinalizer, methods MethodMap) *ForeignClass {
	if methods == nil {
		methods = make(MethodMap)
	}
	return &ForeignClass{Initializer: initializer, Finalizer: finalizer, MethodMap: methods}
}

// Clone creates a copy of the current `ForeignClass`
func (class *ForeignClass) Clone() *ForeignClass {
	return NewClass(class.Initializer, class.Finalizer, class.MethodMap.Clone())
}

// Clone creates a copy of the current `MethodMap`
func (methods MethodMap) Clone() MethodMap {
	newMap := make(MethodMap)
	for signature, fn := range methods {
		newMap[signature] = fn
	}
	return newMap
}

// Merge goes through all items in the source `MethodMap` and adds them if they are not nil
func (methods MethodMap) Merge(source MethodMap) MethodMap {
	for signature, fn := range source {
		if fn != nil {
			methods[signature] = fn
		}
	}
	return methods
}
