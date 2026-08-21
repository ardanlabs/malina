package sd

import (
	"testing"
	"unsafe"
)

func TestPreviewTrampolineCopiesFrames(t *testing.T) {
	pixels := []byte{1, 2, 3, 4, 5, 6}
	rawFrames := []cImage{{Width: 2, Height: 1, Channel: 3, Data: &pixels[0]}}
	step := int32(4)
	count := int32(len(rawFrames))
	frames := &rawFrames[0]
	noisy := uint8(1)
	var data unsafe.Pointer
	arguments := [5]unsafe.Pointer{unsafe.Pointer(&step), unsafe.Pointer(&count), unsafe.Pointer(&frames), unsafe.Pointer(&noisy), unsafe.Pointer(&data)}

	var got []SDImage
	callbacksMu.Lock()
	previous := previewUserHandler
	previewUserHandler = func(gotStep int, callbackFrames []SDImage, gotNoisy bool) {
		if gotStep != 4 {
			t.Errorf("step: got %d, want 4", gotStep)
		}
		if !gotNoisy {
			t.Error("noisy: got false, want true")
		}
		got = callbackFrames
	}
	callbacksMu.Unlock()
	t.Cleanup(func() {
		callbacksMu.Lock()
		previewUserHandler = previous
		callbacksMu.Unlock()
	})

	previewTrampoline(nil, nil, &arguments[0], nil)
	pixels[0] = 99

	if len(got) != 1 {
		t.Fatalf("frames length: got %d, want 1", len(got))
	}
	if got[0].Data[0] != 1 {
		t.Errorf("copied pixel: got %d, want 1", got[0].Data[0])
	}
}

func TestBackendEvalTrampolineStoresBoolResult(t *testing.T) {
	var tensorStorage byte
	tensorPointer := unsafe.Pointer(&tensorStorage)
	wantTensor := Tensor(uintptr(tensorPointer))
	ask := uint8(1)
	var data unsafe.Pointer
	arguments := [3]unsafe.Pointer{unsafe.Pointer(&tensorPointer), unsafe.Pointer(&ask), unsafe.Pointer(&data)}

	callbacksMu.Lock()
	previous := backendEvalUserHandler
	backendEvalUserHandler = func(tensor Tensor, gotAsk bool) bool {
		if tensor != wantTensor {
			t.Errorf("tensor: got %#x, want %#x", tensor, wantTensor)
		}
		if !gotAsk {
			t.Error("ask: got false, want true")
		}
		return true
	}
	callbacksMu.Unlock()
	t.Cleanup(func() {
		callbacksMu.Lock()
		backendEvalUserHandler = previous
		callbacksMu.Unlock()
	})

	var result uint8
	backendEvalTrampoline(nil, unsafe.Pointer(&result), &arguments[0], nil)
	if result != 1 {
		t.Errorf("result: got %d, want 1", result)
	}
}
