package utils

import "testing"

// TestBytePtrRoundTrip verifies that every Go string round-trips through
// the FFI string helpers without loss. These helpers sit underneath every
// C call in the codebase, so any regression here cascades silently. The
// table covers ASCII, multi-byte UTF-8, embedded special characters, and
// the empty-string edge case (which is a valid input but cannot contain
// an embedded NUL — see TestBytePtrFromStringEmbeddedNul).
func TestBytePtrRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"hello, world",
		"стабильная-диффузия",
		"日本語",
		"emoji 🚀 frontier",
		"slashes/and\\backslashes",
		"tabs\tand\nnewlines",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			ptr, err := BytePtrFromString(in)
			if err != nil {
				t.Fatalf("BytePtrFromString(%q): %v", in, err)
			}
			got := BytePtrToString(ptr)
			if got != in {
				t.Errorf("round-trip: got %q, want %q", got, in)
			}
		})
	}
}

// TestBytePtrFromStringEmbeddedNul verifies the helper rejects strings
// with embedded NUL bytes (which would silently truncate the C-side
// string and produce data corruption further down the FFI call chain).
func TestBytePtrFromStringEmbeddedNul(t *testing.T) {
	_, err := BytePtrFromString("a\x00b")
	if err == nil {
		t.Fatal("BytePtrFromString(\"a\\x00b\"): expected error, got nil")
	}
}
