package main

import "fmt"

type NormalEngine struct{}

func (e NormalEngine) Calculate(x int) int {
	return x * 2
}

type SpecialEngine struct{}

func (e SpecialEngine) Calculate(x int) int {
	return x * 3
}

func main() {
	ne := NormalEngine{}
	se := SpecialEngine{}
	fmt.Println("Normal:", ne.Calculate(5))
	fmt.Println("Special:", se.Calculate(5))
}
