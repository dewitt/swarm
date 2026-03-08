package main

import "testing"

func TestEngines(t *testing.T) {
	ne := NormalEngine{}
	if got := ne.Calculate(10); got != 20 {
		t.Errorf("NormalEngine.Calculate(10) = %d; want 20", got)
	}

	se := SpecialEngine{}
	if got := se.Calculate(10); got != 30 {
		t.Errorf("SpecialEngine.Calculate(10) = %d; want 30", got)
	}
}
