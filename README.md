# WrenGo

[![GoDoc](https://godoc.org/github.com/CrazyInfin8/WrenGo?status.svg)](https://pkg.go.dev/github.com/CrazyInfin8/WrenGo?tab=doc) [![GoReportCard](https://goreportcard.com/badge/github.com/crazyinfin8/WrenGo)](https://goreportcard.com/report/github.com/crazyinfin8/WrenGo) [![Wren](https://img.shields.io/badge/github-wren-hsl(200%2C%2060%25%2C%2050%25))](https://github.com/wren-lang/wren)

WrenGo provides bindings for go to interact with the [wren](https://wren.io/) scripting language. Currently Mutex is not used so be careful with Goroutines. There probably should be a lot more tests as well, however there are some tests to ensure that basic functionality works.
## Installation

```
go get github.com/crazyinfin8/WrenGo
```

## Usage
A simple Hello world

```Go
package main

import (
	wren "github.com/crazyinfin8/WrenGo"
)

func main() {
	vm := wren.NewVM()
	// The VM should be freed when no longer needed
	defer vm.Free()
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
	cfg.LoadModuleFn = func(vm *wren.VM, name string) (string, bool) {
		if name == "WrenGo" {
			return `System.print("Hello from imported module")`, true // return true for successful import
		}
		return "", false // return false for unsuccessful import
	}
	vm := cfg.NewVM()
	defer vm.Free()
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
	defer vm.Free()
	vm.InterpretString("main", 
	`class MyClass {
		static sayHello() {
			System.print("Hello from MyClass")
		}
	}`)
	value, _ := vm.GetVariable("main", "MyClass")
	MyClass, _ := value.(*wren.Handle)
	// Handles should be freed when no longer needed
	defer MyClass.Free()
	Fn, _ := MyClass.Func("sayHello()")
	defer Fn.Free()
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
	defer vm.Free()
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

	MyClass.sayHello()`)
}
```

More complete example about using foreign class functions

```Go
package main

import (
	"errors"
	wren "github.com/crazyinfin8/WrenGo"
)

// Person is a single human
type Person struct {
	name string
}

// GetPerson Safely extract Person struct from interface{}
func GetPerson(i interface{}) (*Person, bool) {
	if foreign, ok := i.(*wren.ForeignHandle); ok {
		if val, err := foreign.Get(); err == nil {
			person, ok := val.(*Person)
			return person, ok
		}
	}
	return nil, false
}

func main() {
	vm := wren.NewVM()
	defer vm.Free()
	vm.SetModule("main", wren.NewModule(wren.ClassMap{
		"Person": wren.NewClass(
			// This function is called whenever the "Person" constructor is called
			func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
				// parameters index start at 1. Index 0 is the receiver object in wren
				// (in this case it is the class "Person")
				if len(parameters) < 2 {
					return nil, errors.New("Expected at least 1 Parameter")
				}
				if name, ok := parameters[1].(string); ok {
					println("Created Person: " + name)
					return &Person{name}, nil
				}
				return nil, errors.New("Expected first parameter to be of type String")
			},
			// This function is called whenever the "Person" instance was Garbage collected
			func(vm *wren.VM, data interface{}) {
				if person, ok := data.(*Person); ok {
					println(person.name + " was garbage collected")
				} else {
					panic("Should not happen")
				}

			},
			wren.MethodMap{
				"sayHello()": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
					if len(parameters) > 0 {
						// Again index 0 is for the receiver. As this function is not static,
						// the receiver is the instance of this "Person" class.
						// Index is actually a ForeignHandle that can access the "Person" struct.
						// To extract the "Person" struct, I can use the helper function above,
						// "GetPerson".
						//
						// Note: you should normally call Free all handles, however handles that were
						// created by WrenGo (for parameters) are automatically freed.
						// If you want to save the Handle, you should call Copy on it (make sure to
						// free that handle as well)
						if person, ok := GetPerson(parameters[0]); ok {
							println("Hello, I am " + person.name)
						}
					} else {
						panic("Should not happen")
					}

					return nil, nil
				},
				// notice that this function has an underscore. The number of underscores should determine the length of "parameters", They should also be comma seperated
				"introduceTo(_)": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
					if len(parameters) < 2 {
						return nil, errors.New("Expected at least 1 Parameter")
					}
					var (
						person1, person2 *Person
						ok               bool
					)
					if person1, ok = GetPerson(parameters[0]); !ok {
						panic("Should not happen")
					}
					if person2, ok = GetPerson(parameters[1]); !ok {
						return nil, errors.New("Expected first parameter to be of type Person")
					}
					println(person1.name + ", I would like you to meet " + person2.name)
					return nil, nil
				},
				// getters do not use parenthesis
				"name": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
					if len(parameters) > 0 {
						// Again index 0 is for the receiver. As this function is not static,
						// the receiver is the instance of this "Person" class
						if person, ok := GetPerson(parameters[0]); ok {
							return person.name, nil
						}
					}
					panic("Should not happen")
				},
				// setters are like regular functions but with an equal sign after the function name and only one parameter
				"name=(_)": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
					if len(parameters) < 2 {
						return nil, errors.New("Expected at least 1 Parameter")
					}
					var (
						person *Person
						name   string
						ok     bool
					)
					if person, ok = GetPerson(parameters[0]); !ok {
						panic("Should not happen")
					}
					if name, ok = parameters[1].(string); !ok {
						return nil, errors.New("Expected first parameter to be of type String")
					}
					println(person.name + " changed their name to " + name)
					person.name = name
					return nil, nil
				},
				// operator overloading is also possible by changing name to the operator
				"+(_)": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
					if len(parameters) < 2 {
						return nil, errors.New("Expected at least 1 Parameter")
					}
					var (
						person1, person2 *Person
						ok               bool
					)
					if person1, ok = GetPerson(parameters[0]); !ok {
						panic("Should not happen")
					}
					if person2, ok = GetPerson(parameters[1]); !ok {
						return nil, errors.New("Expected other operand to be of type Person")
					}
					println(person1.name + " has married " + person2.name)
					return nil, nil
				},
				// overloading Indexing behavior is posible by using brackets 
				"[_]": func(vm *wren.VM, parameters []interface{}) (interface{}, error) {
					if len(parameters) < 2 {
						return nil, errors.New("Expected at least 1 Parameter")
					}
					var (
						person1, person2 *Person
						ok               bool
					)
					if person1, ok = GetPerson(parameters[0]); !ok {
						panic("Should not happen")
					}
					if person2, ok = GetPerson(parameters[1]); !ok {
						return nil, errors.New("Expected other operand to be of type Person")
					}
					println("indexing " + person1.name + " by " + person2.name + " (don't think to hard about that, it just works)")
					return nil, nil
				},
			}),
	}))
	vm.InterpretString("main",
		`foreign class Person {
			construct new(name) {}
			foreign sayHello()
			foreign introduceTo(person)
			foreign name
			foreign name=(name)
			foreign +(person)
			foreign [person]
		}

		var person1 = Person.new("John")
		var person2 = Person.new("Jane")
		System.print("person1 is %(person1.name)")
		person1.name = "David"
		person2.introduceTo(person1)
		person1.sayHello()
		person1 + person2
		person1[person2]`)
}
```