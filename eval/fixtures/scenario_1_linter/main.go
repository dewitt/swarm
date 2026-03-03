package main

import "fmt"

func main() {
	var unused_variable = 10
	fmt.Printf("Hello world")

	// shadowed variable
	x := 5
	if true {
		x := 10
		_ = x
	}
}
