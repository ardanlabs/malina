package sd

import (
	"errors"
	"strings"
	"testing"
)

func TestEmptyContextGenerationFails(t *testing.T) {
	testSetup(t)
	ctx, err := NewContext(ContextParamsInit())
	if err != nil {
		t.Fatal(err)
	}
	defer FreeContext(ctx)
	_, err = GenerateImage(ctx, ImgGenParamsInit())
	var native *NativeError
	if !errors.As(err, &native) || native.Message == "" {
		t.Fatalf("GenerateImage: got %v, want diagnostic NativeError", err)
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
