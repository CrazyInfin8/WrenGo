package wren

import (
	"errors"
	"reflect"
	"testing"
)

func createConfig(t *testing.T) *Config {
	var writeFn WriteFn = func(vm *VM, text string) {
		t.Logf("write> %v", text)
	}
	var errorFn ErrorFn = func(vm *VM, err error) {
		t.Logf("error> %v", err.Error())
	}
	return &Config{WriteFn: writeFn, ErrorFn: errorFn}
}

func TestVersion(t *testing.T) {
	t.Logf("Wren Version: %v, Tuple%v", VersionString, VersionTuple())
}

func TestVM(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()
	t.Log("Run from string")
	err := vm.InterpretString("main", `
	System.write("Hello WrenGo!")
	`)
	if err != nil {
		t.Error(err.Error())
		return
	}
	t.Log("Import file from string")
	err = vm.InterpretString("main", `
	import "tests/import.wren"
	`)
	if err != nil {
		t.Error(err.Error())
		return
	}
	t.Log("Run from file")
	err = vm.InterpretFile("tests/import.wren")
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestHandles(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()
	t.Log("Setting variables from wren")
	err := vm.InterpretString("main", `
	var value1 = 42
	var value2 = 3.141592
	var value3 = true
	var value4 = [1, "a", true]
	var value5 = {
		"index1": 42,
		2: 3.141592,
		true: "the key is true"
	}
	
	class MyClass {
		construct new(x) {
			_value = x
		}
		
		echoValue() {
			System.write("Value is: " + _value.toString)
		}
	}
	
	var value6 = MyClass.new("A fancy magical value")
	System.write("Wren is done")
	`)

	for _, name := range []string{"value1", "value2", "value3"} {
		v, _ := vm.GetVariable("main", name)
		t.Logf("Variable \"%v\" has type [%v] and value is [%v]", name, reflect.TypeOf(v), reflect.ValueOf(v))
	}
	value4, _ := vm.GetVariable("main", "value4")
	if list, ok := value4.(*ListHandle); ok {
		count, _ := list.Count()
		t.Logf("Variable \"value4\" is a list that has %v items", count)
	} else {
		t.Errorf("value4 is not the expected list")
		return
	}
	value5, _ := vm.GetVariable("main", "value5")
	if list, ok := value5.(*MapHandle); ok {
		count, _ := list.Count()
		t.Logf("Variable \"value5\" is a list that has %v items", count)
	} else {
		t.Errorf("value5 is not the expected map")
		return
	}
	value6, _ := vm.GetVariable("main", "value6")
	if class, ok := value6.(*Handle); ok {
		Fn, _ := class.Func("echoValue()")
		defer Fn.Free()
		Fn.Call()
	}
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestMaps(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()
	t.Log("Setting variables from wren")
	err := vm.InterpretString("main", `
	class Util {
			static echo(x) {
				System.write(x)
			}
		}

		var fooMap = {
			"value1": 1,
			"value2": 5,
			} 
			`)
	if err != nil {
		t.Error(err.Error())
		return
	}
	var (
		UtilClass *Handle
		fooMap    *MapHandle
		ok        bool
		v         interface{}
	)
	v, _ = vm.GetVariable("main", "Util")
	if UtilClass, ok = v.(*Handle); !ok {
		t.Error("Util is not the expected class")
		return
	}
	v, _ = vm.GetVariable("main", "fooMap")
	if fooMap, ok = v.(*MapHandle); !ok {
		t.Error("fooMap is not the expected map")
		return
	}
	echo, _ := UtilClass.Func("echo(_)")
	defer echo.Free()
	echo.Call(fooMap)
	fooMap.Set("value3", "A lovely value!")
	echo.Call(fooMap) // Just for me to know if Map handles are mutable
}

func TestForeignAndBindings(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()
	t.Log("Setting variables from wren")
	vm.SetModule("main", NewModule(ClassMap{
		"GoFoo": NewClass(
			func(vm *VM, parameters []interface{}) (interface{}, error) {
				var value interface{}
				if len(parameters) >= 2 {
					value = parameters[1]
				}
				t.Logf("Setting foreign to \"%v\"", value)
				return value, nil
			},
			func(vm *VM, data interface{}) {

			},
			MethodMap{
				"static sayHello()": func(vm *VM, parameters []interface{}) (interface{}, error) {
					t.Log("Go function called from wren says hello!")
					return nil, nil
				},
				"static echo(_,_,_)": func(vm *VM, parameters []interface{}) (interface{}, error) {
					t.Logf("Wren passed %v, %v, and %v as values", parameters[1:]...)
					return nil, nil
				},
				"static backToWren()": func(vm *VM, parameters []interface{}) (interface{}, error) {
					t.Logf("returning value back to wren")
					return "A value from Go", nil
				},
				"printValue()": func(vm *VM, parameters []interface{}) (interface{}, error) {
					receiver := parameters[0]
					if foreign, ok := receiver.(*ForeignHandle); ok {
						val, _ := foreign.Get()
						t.Logf("Wren sent back the value \"%v\"", val)
					}
					return nil, nil
				},
				"static sendError()": func(vm *VM, parameters []interface{}) (interface{}, error) {
					return nil, errors.New("An error from Go to Wren")
				},
			},
		),
	}))
	err := vm.InterpretString("main", `
	foreign class GoFoo {
		construct new(x) {}
		foreign printValue()
		foreign static sayHello()
		foreign static echo(x, y, z)
		foreign static backToWren()
		foreign static sendError()
	}
	System.write("Running Go function from wren")
	GoFoo.sayHello()
	GoFoo.echo(1, 2, 3)
	var value = GoFoo.backToWren()
	System.write("Go sent back the value: \"" + value + "\"")
	System.write("Making an instance of GoFoo...")
	var foo = GoFoo.new("Hello from wren")
	foo.printValue()
	System.write("Testing errors from foreign methods")
	var makeError = Fiber.new {
		GoFoo.sendError()
	}
	makeError.try()
	System.write("Error: " + makeError.error)
	`)
	// checking := true
	if err != nil {
		// if (err)
		t.Error(err.Error())
		return
	}
	// err := vm.InterpretString("main", `
}

func TestEditConfig(t *testing.T) {
	cfg := createConfig(t)
	vm := NewVM()
	defer vm.Free()
	vm.Config.ErrorFn = cfg.ErrorFn
	vm.Config.WriteFn = cfg.WriteFn
	err := vm.InterpretString("main", `
	System.write("Hello world")
	`)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
}

func TestInvalidConstructor(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()
	vm.InterpretString("main", `
	foreign class MyClass {
		construct new() {}
	}
	MyClass.new()
	`)
	vm.InterpretString("main", `
	System.write("Success, code no longer segfaults")
	`)
	vm.GC()

}

func TestParallelVM(t *testing.T) {
	RunNewVM := func(vmNum int, success chan bool, fail chan bool) {
		cfg := NewConfig()
		cfg.WriteFn = func(vm *VM, text string) {
			t.Logf("write %v> %v", vmNum, text)
		}
		cfg.ErrorFn = func(vm *VM, err error) {
			t.Logf("error %v> %v", vmNum, err.Error())
		}
		vm := cfg.NewVM()
		defer vm.Free()
		type Foo struct {
			i float64
		}
		vm.SetModule("main", NewModule(ClassMap{
			"Foo": NewClass(
				func(vm *VM, parameters []interface{}) (interface{}, error) {
					t.Logf("VM %v called constructor", vmNum)
					return &Foo{}, nil
				},
				func(vm *VM, data interface{}) {
					if foo, ok := data.(Foo); ok {
						t.Logf("VM %v called destructor with value: %v", vmNum, foo.i)

					}
				},
				MethodMap{
					"increment(_)": func(vm *VM, parameters []interface{}) (interface{}, error) {
						var (
							foo     *Foo
							foreign *ForeignHandle
							inter   interface{}
							ok      bool
							err     error
						)
						if foreign, ok = parameters[0].(*ForeignHandle); !ok {
							return nil, errors.New("foreign malformed")
						}
						if inter, err = foreign.Get(); err != nil {
							return nil, err
						}
						if foo, ok = inter.(*Foo); !ok {
							return nil, errors.New("foreign malformed")
						}
						if i, ok := parameters[1].(float64); ok {
							foo.i += i
							return foo.i, nil
						}
						return nil, errors.New("Expected an int for the first parameter")
					},
				}),
		}))
		err := vm.InterpretString("main", `
		foreign class Foo {
			construct new() {}
			foreign increment(i)
		}

		import "random" for Random
		
		var foo = Foo.new()
		var rand = Random.new()
		var total = 0
		for (i in 0...20) {
			var j = rand.int(20)
			System.write("incrementing " + total.toString + " by " + j.toString + " to get " + foo.increment(j).toString)
			total = total + j
		}
		`)
		if err != nil {
			fail <- true
		} else {
			success <- true
		}
	}
	fail := make(chan bool)
	success := make(chan bool)
	var numOfVMs int = 5
	for i := 0; i < numOfVMs; i++ {
		go RunNewVM(i, success, fail)
	}
	var numOfSuccess int = 0
	var numOfFails int = 0
	for numOfSuccess+numOfFails < numOfVMs {
		select {
		case <-fail:
			numOfFails++
			t.Errorf("VM failed")
		case <-success:
			numOfSuccess++
			t.Logf("VM was successful")
		}
	}
}

func Test040(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()
	vm.InterpretString("main", `
	var value = "A value"
	`)
	if val, err := vm.GetVariable("main", "value"); err != nil {
		t.Error(err.Error())
	} else if val != "A value" {
		t.Error("Variable does not match expected")
	} else {
		t.Logf("Val is \"%v\"", val)
	}
}

func TestResolveModName(t *testing.T) {
	cfg := createConfig(t)
	cfg.ResolveModuleFn = func(vm *VM, importer, name string) (newName string, ok bool) {
		switch importer {
		case "main":
			newName = name
		case "A":
			if name == "B" {
				newName = "AB"
			} else {
				newName = name
			}
		case "B":
			if name == "A" {
				newName = "BA"
			} else {
				newName = name
			}
		}
		t.Logf("%v imported %v and was resolved to %v", importer, name, newName)
		ok = newName != ""
		return
	}

	cfg.LoadModuleFn = func(vm *VM, name string) (source string, ok bool) {
		t.Logf("Importing: %v", name)
		switch name {
		case "A":
			source = `
			System.print("Hello from A")
			import "A"
			import "B"`
		case "B":
			source = `
			System.print("Hello from B")
			import "A"
			import "B"`
		case "AB":
			source = `System.print("Hello from B imported by A")`
		case "BA":
			source = `System.print("Hello from A imported by B")`
		}
		ok = source != ""
		return
	}
	vm := cfg.NewVM()
	defer vm.Free()

	vm.InterpretString("main", `
	import "A"
	import "B"
	`)
}

func TestNoReentry(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()

	vm.SetModule("main", NewModule(ClassMap{
		"GoFoo": NewClass(
			nil,
			nil,
			MethodMap{
				"static reEntryByInterp()": func(vm *VM, parameters []interface{}) (interface{}, error) {
					err := vm.InterpretString("reEntry", "System.print(\"Things seemed to have failed already\")")

					if _, ok := err.(*RunningVMError); ok {
						t.Log("Re-entry detected in `static reEntry()`")
					} else {
						t.Error("Could not get an expected re-entry error")
					}
					return nil, nil
				},
				"static reEntryByMethod()": func(vm *VM, parameters []interface{}) (interface{}, error) {
					if h, ok := parameters[0].(*Handle); ok {
						fn, err := h.Func("static reEntryByInterp()")
						defer fn.Free()
						if err != nil {
							t.Error(err.Error())
						}
						_, err = fn.Call()
						if _, ok := err.(*RunningVMError); ok {
							t.Log("Re-entry detected in `static reEntry()`")
						} else {
							t.Error("Could not get an expected re-entry error")
						}
					}
					return nil, nil
				},
			},
		),
	}))

	vm.InterpretString("main", `
	foreign class GoFoo {
		foreign static reEntryByInterp()
		foreign static reEntryByMethod()
	}

	GoFoo.reEntryByInterp()
	GoFoo.reEntryByMethod()
	`)
}

func TestLists(t *testing.T) {
	vm := createConfig(t).NewVM()
	defer vm.Free()

	vm.SetModule("main", NewModule(ClassMap{
		"Lst": NewClass(nil, nil, MethodMap{
			"static send(_)": func(vm *VM, parameters []interface{}) (interface{}, error) {
				handle := parameters[1].(*ListHandle)
				count, _ := handle.Count()
				t.Log("The list count is", count)
				for i := 0; i < count; i++ {
					ifc, _ := handle.Get(i)
					t.Log(ifc)
				}
				handle.Free()
				return nil, nil
			},
		}),
	}))

	vm.InterpretString("main", `
	class Lst {
		foreign static send(a)
	}
	
	var a = (1..10).toList
	var b = Lst.send(a)
	System.print(b)
	`)
}
