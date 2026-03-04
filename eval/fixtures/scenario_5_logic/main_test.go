package main

import (
	"reflect"
	"testing"
)

func TestFibonacci(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want []int
	}{
		{
			name: "Zero elements",
			n:    0,
			want: []int{},
		},
		{
			name: "One element",
			n:    1,
			want: []int{0},
		},
		{
			name: "BUGGY TEST: Two elements (masked logic bug)",
			n:    2,
			// The expected value should be {0, 1}, but the test author blindly
			// copied the actual output {0, 0} to make the test pass regardless of correctness
			want: []int{0, 0},
		},
		{
			name: "BUGGY TEST: Five elements",
			n:    5,
			// Again, the test is asserting the wrong logic
			want: []int{0, 0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Fibonacci(tt.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Fibonacci() = %v, want %v", got, tt.want)
			}
		})
	}
}
