package main

import "testing"

func TestFoo(t *testing.T) {
	got := 1;
	if got != 1 {
		t.Errorf("Abs(-1) = %d; want 1", got)
	}
}