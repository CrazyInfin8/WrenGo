package wren

import (
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
		t.Logf("Vaiable \"value4\" is a list that has %v items", count)
	} else {
		t.Errorf("value4 is not the expected list")
		return
	}
	value5, _ := vm.GetVariable("main", "value5")
	if list, ok := value5.(*MapHandle); ok {
		count, _ := list.Count()
		t.Logf("Vaiable \"value5\" is a list that has %v items", count)
	} else {
		t.Errorf("value5 is not the expected map")
		return
	}
	value6, _ := vm.GetVariable("main", "value6")
	if class, ok := value6.(*Handle); ok {
		Fn, _ := class.Func("echoValue()")
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
			func(vm *VM, parameters []interface{}) interface{} {
				var value interface{}
				if len(parameters) >= 2 {
					value = parameters[1]
				}
				t.Logf("Setting foreign to \"%v\"", value)
				return value
			},
			func(vm *VM, data interface{}) {

			},
			MethodMap{
				"static sayHello()": func(vm *VM, parameters []interface{}) interface{} {
					t.Log("Go function called from wren says hello!")
					return nil
				},
				"static echo(_,_,_)": func(vm *VM, parameters []interface{}) interface{} {
					t.Logf("Wren passed %v, %v, and %v as values", parameters[1:]...)
					return nil
				},
				"static backToWren()": func(vm *VM, parameters []interface{}) interface{} {
					t.Logf("returning value back to wren")
					return "A value from Go"
				},
				"printValue()": func(vm *VM, parameters []interface{}) interface{} {
					receiver := parameters[0]
					if foreign, ok := receiver.(*ForeignHandle); ok {
						val, _ := foreign.Get()
						t.Logf("Wren sent back the value \"%v\"", val)
					}
					return nil
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
	}
	System.write("Running Go function from wren")
	GoFoo.sayHello()
	GoFoo.echo(1, 2, 3)
	var value = GoFoo.backToWren()
	System.write("Go sent back the value: \"" + value + "\"")
	System.write("Making an instance of GoFoo...")
	var foo = GoFoo.new("Hello from wren")
	foo.printValue()
	`)
	if err != nil {
		t.Error(err.Error())
		return
	}
	// err := vm.InterpretString("main", `

}
