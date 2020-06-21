package wren

/*
#cgo CFLAGS:
#cgo LDFLAGS: -lm
#include "wren.h"
*/
import "C"

type ForeignMethodFn func(vm *VM, parameters []interface{}) interface{}
type ForeignInitializer func(vm *VM, parameters []interface{}) interface{}
type ForeignFinalizer func(vm *VM, data interface{})

type ModuleMap map[string]*Module

type Module struct {
	ClassMap ClassMap
}

type ClassMap map[string]*ForeignClass

type ForeignClass struct {
	Initializer ForeignInitializer
	Finalizer   ForeignFinalizer
	MethodMap   MethodMap
}

type MethodMap map[string]ForeignMethodFn

func (modules ModuleMap) Clone() ModuleMap {
	newMap := make(ModuleMap)
	for name, module := range modules {
		newMap[name] = module.Clone()
	}
	return newMap
}

func (modules ModuleMap) Merge(source ModuleMap) ModuleMap {
	for name, module := range source {
		if module != nil {
			modules[name].ClassMap.Merge(module.ClassMap)
		}
	}
	return modules
}

func (module *Module) Clone() *Module {
	return NewModule(module.ClassMap.Clone())
}

func NewModule(classes ClassMap) *Module {
	return &Module{ClassMap: classes.Clone()}
}

func (classes ClassMap) Clone() ClassMap {
	newMap := make(ClassMap)
	for name, class := range classes {
		newMap[name] = class.Clone()
	}
	return newMap
}

func (classes ClassMap) Merge(source ClassMap) ClassMap {
	for name, class := range source {
		if class != nil {
			classes[name].MethodMap.Merge(class.MethodMap)
		}
	}
	return classes
}

func NewClass(initializer ForeignInitializer, finalizer ForeignFinalizer, methods MethodMap) *ForeignClass {
	if methods == nil {
		methods = make(MethodMap)
	}
	return &ForeignClass{Initializer: initializer, Finalizer: finalizer, MethodMap: methods}
}

func (class *ForeignClass) Clone() *ForeignClass {
	return NewClass(class.Initializer, class.Finalizer, class.MethodMap.Clone())
}

func (methods MethodMap) Clone() MethodMap {
	newMap := make(MethodMap)
	for signature, fn := range methods {
		newMap[signature] = fn
	}
	return newMap
}

func (methods MethodMap) Merge(source MethodMap) MethodMap {
	for signature, fn := range source {
		if fn != nil {
			methods[signature] = fn
		}
	}
	return methods
}
