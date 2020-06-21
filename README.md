# WrenGo

WrenGo provides bindings for go to interact with the [wren](https://wren.io/) scripting language.

## Usage
A simple Hello world

```Go
package main

import (
	wren "github.com/crazyinfin8/WrenGo"
)

func main() {
	vm := wren.NewVM()
	vm.InterpretString("main", `System.print("Hello world from wren!")`)
}
```

Adding some configurating

```Go
package main

import (
	wren "github.com/crazyinfin8/WrenGo"
)

func main() {
	cfg := wren.NewConfig()
	cfg.LoadModuleFn = func(vm *wren.VM, name string) string {
		if name == "WrenGo" {
			return `System.print("Hello from imported module")`
		}
		return ""
	}
	vm := cfg.NewVM()
	vm.InterpretString("main", `import "WrenGo"`)
}
```

Calling Wren functions from Go

```Go
package main

import (
	wren "github.com/crazyinfin8/WrenGo"
)

func main() {
	vm := wren.NewVM()
	vm.InterpretString("main", 
	`class MyClass {
		static sayHello() {
			System.print("Hello from MyClass")
		}
	}
	`)
	value, _ := vm.GetVariable("main", "MyClass")
	MyClass, _ := value.(*wren.Handle)
	Fn, _ := MyClass.Func("sayHello()")
	Fn.Call()
}
```

Calling Go functions from Wren

```Go
package main

import (
	wren "github.com/crazyinfin8/WrenGo"
)

func main() {
	vm := wren.NewVM()
	vm.SetModule("main", wren.NewModule(wren.ClassMap{
		"MyClass": wren.NewClass(nil, nil, wren.MethodMap{
			"static sayHello()": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
				println("Hello from MyClass but from Go")
				return nil, nil
			},
		}),
	}))
	vm.InterpretString("main", 
	`foreign class MyClass {
		foreign static sayHello()
	}

	MyClass.sayHello()
	`)
}
```