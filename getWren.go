// +build ignore

// Meant to be run from "go generate"
//
// Automatically sets up WrenGo by cloning wren
// and generating amalgamation file
//
// Git and Python are required to run this file
package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
)

func main() {
	var clean bool
	flag.BoolVar(&clean, "clean", false, "Delete Wren source code")
	flag.Parse()
	if clean {
		println("Removing wren")
		os.RemoveAll("wren-c")
		os.Remove("wren.c")
		os.Remove("wren.h")
		println("Done")
		return
	}
	println("Cloning wren from github")
	cloneWren()
	println("Generating amalgamation")
	makeAmalgamation()
	println("Copying header files")
	copyHeader()
	println("Success!")
}

func cloneWren() {
	wrenDir, err := os.Open("wren-c")
	if err != nil {
		if os.IsNotExist(err) {
			cmd := exec.Command("git", "clone", "https://github.com/wren-lang/wren.git", "wren-c")
			cmd.Stdout = os.Stdout
			err := cmd.Run()
			if err != nil {
				panic(err.Error())
			}
		} else {
			panic(err.Error())
		}
	} else {
		wrenDir.Close()
		println("Wren already exists")
	}
}

func makeAmalgamation() {
	cmd := exec.Command("python", "wren-c/util/generate_amalgamation.py")
	file, err := os.Create("wren.c")
	if err != nil {
		panic(err.Error())
	}
	defer file.Close()
	cmd.Stdout = file
	err = cmd.Run()
	if err != nil {
		panic(err.Error())
	}
}

func copyHeader() {
	data, err := ioutil.ReadFile("wren-c/src/include/wren.h")
	if err != nil {
		panic(err.Error())
	}
	ioutil.WriteFile("wren.h", data, os.ModePerm)
}
