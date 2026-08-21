package main

import "testing"

func TestVersionExact(t *testing.T) {
	const want = "1.0.4"

	if got := Version(); got != want {
		t.Errorf("Version: got %q, want %q", got, want)
	}
}
