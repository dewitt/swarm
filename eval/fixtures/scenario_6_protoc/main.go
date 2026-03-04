package main

import "fmt"

func main() {
	p := &Person{Name: "Alice", Id: 123}
	fmt.Printf("Person: %s (ID: %d)\n", p.GetName(), p.GetId())
}
