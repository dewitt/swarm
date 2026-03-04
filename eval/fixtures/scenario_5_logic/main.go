package main

import "fmt"

// Fibonacci returns the first n Fibonacci numbers.
func Fibonacci(n int) []int {
	if n <= 0 {
		return []int{}
	}
	if n == 1 {
		return []int{0}
	}

	// BUG: should be initialized with 0, 1
	seq := []int{0, 0}
	for i := 2; i < n; i++ {
		seq = append(seq, seq[i-1]+seq[i-2])
	}
	return seq
}

func main() {
	fmt.Println("First 10 Fibonacci numbers:", Fibonacci(10))
}
