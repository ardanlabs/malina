package sd

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// PreviewCallback receives preview images produced during generation. Each
// SDImage and its Data are copied into Go-owned memory before the callback is
// invoked and remain valid after the callback returns.
//
// The callback is process-global, can be invoked concurrently, and must be
// concurrency-safe.
type PreviewCallback func(step int, frames []SDImage, noisy bool)

// Tensor is an opaque handle to a ggml_tensor.
//
// A Tensor passed to BackendEvalCallback is valid only for the duration of
// that callback invocation. It must not be retained or dereferenced by Go.
type Tensor uintptr

// BackendEvalCallback controls backend evaluation of a tensor. The ask value
// and callback result have the same meaning as sd_graph_eval_callback_t in
// stable-diffusion.cpp.
//
// The callback is process-global, can be invoked concurrently, and must be
// concurrency-safe. The Tensor is valid only until the callback returns.
type BackendEvalCallback func(tensor Tensor, ask bool) bool

var (
	setPreviewCallbackFunc     ffi.Fun
	setBackendEvalCallbackFunc ffi.Fun

	previewCif          ffi.Cif
	previewClosure      *ffi.Closure
	previewCallbackCode unsafe.Pointer
	previewCallbackFun  uintptr

	backendEvalCif          ffi.Cif
	backendEvalClosure      *ffi.Closure
	backendEvalCallbackCode unsafe.Pointer
	backendEvalCallbackFun  uintptr

	callbacksMu            sync.Mutex
	previewUserHandler     PreviewCallback
	backendEvalUserHandler BackendEvalCallback
)

// loadCallbacksFuncs optionally loads the callback APIs added after older
// stable-diffusion.cpp releases. Missing symbols do not prevent Load from
// succeeding.
func loadCallbacksFuncs(lib ffi.Lib) error {
	previewFn, _ := lib.Prep("sd_set_preview_callback", &ffi.TypeVoid,
		&ffi.TypePointer, &ffi.TypeSint32, &ffi.TypeSint32, &ffi.TypeUint8, &ffi.TypeUint8, &ffi.TypePointer)
	backendFn, _ := lib.Prep("sd_set_backend_eval_callback", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypePointer)

	callbacksMu.Lock()
	defer callbacksMu.Unlock()

	if previewFn != (ffi.Fun{}) {
		if err := preparePreviewCallback(); err != nil {
			return err
		}
	}
	if backendFn != (ffi.Fun{}) {
		if err := prepareBackendEvalCallback(); err != nil {
			return err
		}
	}
	setPreviewCallbackFunc = previewFn
	setBackendEvalCallbackFunc = backendFn

	return nil
}

func preparePreviewCallback() error {
	if previewClosure != nil {
		return nil
	}
	if status := ffi.PrepCif(&previewCif, ffi.DefaultAbi, 5, &ffi.TypeVoid,
		&ffi.TypeSint32, &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypeUint8, &ffi.TypePointer); status != ffi.OK {
		return fmt.Errorf("PrepCif sd_preview_cb_t: %v", status)
	}

	closure, code, callback, err := allocateCallbackClosure("sd_preview_cb_t", &previewCif, previewTrampoline)
	if err != nil {
		return err
	}
	previewClosure = closure
	previewCallbackCode = code
	previewCallbackFun = callback
	return nil
}

func prepareBackendEvalCallback() error {
	if backendEvalClosure != nil {
		return nil
	}
	if status := ffi.PrepCif(&backendEvalCif, ffi.DefaultAbi, 3, &ffi.TypeUint8,
		&ffi.TypePointer, &ffi.TypeUint8, &ffi.TypePointer); status != ffi.OK {
		return fmt.Errorf("PrepCif sd_graph_eval_callback_t: %v", status)
	}

	closure, code, callback, err := allocateCallbackClosure("sd_graph_eval_callback_t", &backendEvalCif, backendEvalTrampoline)
	if err != nil {
		return err
	}
	backendEvalClosure = closure
	backendEvalCallbackCode = code
	backendEvalCallbackFun = callback
	return nil
}

func allocateCallbackClosure(name string, cif *ffi.Cif, callback ffi.Callback) (*ffi.Closure, unsafe.Pointer, uintptr, error) {
	var code unsafe.Pointer
	closure := ffi.ClosureAlloc(unsafe.Sizeof(ffi.Closure{}), &code)
	if closure == nil {
		return nil, nil, 0, fmt.Errorf("ffi.ClosureAlloc for %s returned nil", name)
	}

	callbackFunc := ffi.NewCallback(callback)
	if status := ffi.PrepClosureLoc(closure, cif, callbackFunc, nil, code); status != ffi.OK {
		ffi.ClosureFree(closure)
		return nil, nil, 0, fmt.Errorf("PrepClosureLoc %s: %v", name, status)
	}
	return closure, code, callbackFunc, nil
}

// SetPreviewCallback installs the process-global preview callback and its
// generation settings. Pass nil to disable previews. It returns
// ErrUnsupportedAPI when the loaded library predates this API.
func SetPreviewCallback(callback PreviewCallback, mode PreviewMode, interval int, denoised bool, noisy bool) error {
	callbacksMu.Lock()
	defer callbacksMu.Unlock()

	if setPreviewCallbackFunc == (ffi.Fun{}) {
		return unsupported("sd_set_preview_callback")
	}

	previewUserHandler = callback
	var callbackCode unsafe.Pointer
	if callback != nil {
		callbackCode = previewCallbackCode
	}
	modeValue := int32(mode)
	intervalValue := int32(interval)
	denoisedValue := uint8(0)
	if denoised {
		denoisedValue = 1
	}
	noisyValue := uint8(0)
	if noisy {
		noisyValue = 1
	}
	var nilData unsafe.Pointer
	setPreviewCallbackFunc.Call(nil, unsafe.Pointer(&callbackCode), unsafe.Pointer(&modeValue), unsafe.Pointer(&intervalValue),
		unsafe.Pointer(&denoisedValue), unsafe.Pointer(&noisyValue), unsafe.Pointer(&nilData))
	runtime.KeepAlive(previewCallbackFun)
	return nil
}

// SetBackendEvalCallback installs the process-global backend evaluation
// callback. Pass nil to remove it. It returns ErrUnsupportedAPI when the
// loaded library predates this API.
func SetBackendEvalCallback(callback BackendEvalCallback) error {
	callbacksMu.Lock()
	defer callbacksMu.Unlock()

	if setBackendEvalCallbackFunc == (ffi.Fun{}) {
		return unsupported("sd_set_backend_eval_callback")
	}

	backendEvalUserHandler = callback
	var callbackCode unsafe.Pointer
	if callback != nil {
		callbackCode = backendEvalCallbackCode
	}
	var nilData unsafe.Pointer
	setBackendEvalCallbackFunc.Call(nil, unsafe.Pointer(&callbackCode), unsafe.Pointer(&nilData))
	runtime.KeepAlive(backendEvalCallbackFun)
	return nil
}

func previewTrampoline(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	arguments := (*[5]unsafe.Pointer)(unsafe.Pointer(args))
	step := int(*(*int32)(arguments[0]))
	frameCount := int(*(*int32)(arguments[1]))
	framePtr := *(**cImage)(arguments[2])
	noisy := *(*uint8)(arguments[3]) != 0

	callbacksMu.Lock()
	handler := previewUserHandler
	callbacksMu.Unlock()
	if handler == nil {
		return 0
	}

	frames := make([]SDImage, 0, max(frameCount, 0))
	if frameCount > 0 && framePtr != nil {
		rawFrames := unsafe.Slice(framePtr, frameCount)
		for i := range frameCount {
			image := sdImageFromC(&rawFrames[i])
			frames = append(frames, *image)
		}
	}
	handler(step, frames, noisy)
	return 0
}

func backendEvalTrampoline(_ *ffi.Cif, ret unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	arguments := (*[3]unsafe.Pointer)(unsafe.Pointer(args))
	tensor := Tensor(uintptr(*(*unsafe.Pointer)(arguments[0])))
	ask := *(*uint8)(arguments[1]) != 0

	callbacksMu.Lock()
	handler := backendEvalUserHandler
	callbacksMu.Unlock()

	result := handler != nil && handler(tensor, ask)
	*(*uint8)(ret) = 0
	if result {
		*(*uint8)(ret) = 1
	}
	return 0
}
