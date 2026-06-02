package sd

import (
	"runtime"
	"slices"
	"testing"
)

// TestCFreeNil verifies cFree(nil) is a no-op even when freeFunc has
// not been resolved. The fast-path nil check must short-circuit before
// the FFI call so the test runs without a loaded library.
func TestCFreeNil(t *testing.T) {
	cFree(nil)
}

// TestLibcCandidatesNotEmpty asserts every supported runtime.GOOS has
// at least one libc candidate. A bare list would silently fail the
// load loop and surface as a confusing "could not load libc (tried
// [])" message.
func TestLibcCandidatesNotEmpty(t *testing.T) {
	cases := []string{"darwin", "linux", "freebsd", "windows"}
	if !slices.Contains(cases, runtime.GOOS) {
		t.Skipf("runtime.GOOS=%q not in the explicit candidate list; default path covers it", runtime.GOOS)
	}
	if got := libcCandidates(); len(got) == 0 {
		t.Errorf("libcCandidates(): got empty list for GOOS=%q", runtime.GOOS)
	}
}
