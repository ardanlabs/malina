package sd

import (
	"errors"
	"strings"
	"testing"
)

func TestNewContextEmptyFails(t *testing.T) {
	testSetup(t)
	ctx, err := NewContext(ContextParamsInit())
	defer FreeContext(ctx)
	var native *NativeError
	if !errors.As(err, &native) || native.Message == "" || ctx != 0 {
		t.Fatalf("NewContext: got %d, %v, want zero handle and diagnostic NativeError", ctx, err)
	}
}

func TestContextOptionValidation(t *testing.T) {
	testSetup(t)
	for _, tc := range []struct {
		name    string
		options Options
		status  Status
		key     string
	}{
		{"unknown", Options{"not_a_key": 1}, StatusUnknownKey, "not_a_key"},
		{"type", Options{"n_threads": "eight"}, StatusTypeMismatch, "n_threads"},
		{"range", Options{"n_threads": int64(1) << 40}, StatusOutOfRange, "n_threads"},
		{"purpose", Options{"prompt": "test"}, StatusUnsupported, "prompt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := ContextParamsInit()
			p.Options = tc.options
			ctx, err := NewContext(p)
			defer FreeContext(ctx)
			var native *NativeError
			if !errors.As(err, &native) || native.Status != tc.status || !strings.Contains(native.Message, tc.key) {
				t.Fatalf("NewContext: got %v, want status %d and key %q", err, tc.status, tc.key)
			}
		})
	}
}
