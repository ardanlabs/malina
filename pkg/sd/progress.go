package sd

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// ProgressCallback receives process-global model loading and generation
// progress. SecondsPerStep is the average elapsed time per completed step.
type ProgressCallback func(step int, steps int, secondsPerStep float32)

var (
	// SD_API void sd_set_progress_callback(sd_progress_cb_t cb, void* data);
	setProgressCallbackFunc ffi.Fun

	progressCif          ffi.Cif
	progressClosure      *ffi.Closure
	progressCallbackCode unsafe.Pointer
	progressCallbackFun  uintptr

	progressMu          sync.Mutex
	progressUserHandler ProgressCallback
)

func loadProgressFuncs(lib ffi.Lib) error {
	fn, err := lib.Prep("sd_set_progress_callback", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypePointer)
	if err != nil {
		return loadError("sd_set_progress_callback", err)
	}

	progressMu.Lock()
	defer progressMu.Unlock()

	if err := prepareProgressCallback(); err != nil {
		return err
	}
	setProgressCallbackFunc = fn

	return nil
}

func prepareProgressCallback() error {
	if progressClosure != nil {
		return nil
	}

	// Describe sd_progress_cb_t: void (*)(int, int, float, void*).
	if status := ffi.PrepCif(&progressCif, ffi.DefaultAbi, 4,
		&ffi.TypeVoid,
		&ffi.TypeSint32,
		&ffi.TypeSint32,
		&ffi.TypeFloat,
		&ffi.TypePointer,
	); status != ffi.OK {
		return fmt.Errorf("PrepCif sd_progress_cb_t: %v", status)
	}

	var code unsafe.Pointer
	closure := ffi.ClosureAlloc(unsafe.Sizeof(ffi.Closure{}), &code)
	if closure == nil {
		return fmt.Errorf("ffi.ClosureAlloc for sd_progress_cb_t returned nil")
	}

	callback := ffi.NewCallback(progressTrampoline)
	if status := ffi.PrepClosureLoc(closure, &progressCif, callback, nil, code); status != ffi.OK {
		ffi.ClosureFree(closure)
		return fmt.Errorf("PrepClosureLoc sd_progress_cb_t: %v", status)
	}

	progressClosure = closure
	progressCallbackCode = code
	progressCallbackFun = callback

	return nil
}

// SetProgressCallback installs a process-global callback for model loading and
// generation progress. The callback can be invoked concurrently and must be
// concurrency-safe. Pass nil to restore stable-diffusion.cpp's native terminal
// progress display. Configure the callback during application startup before
// loading a model or starting generation.
func SetProgressCallback(callback ProgressCallback) {
	progressMu.Lock()
	defer progressMu.Unlock()

	progressUserHandler = callback
	applyProgressCallbackLocked()
}

func applyProgressCallback() {
	progressMu.Lock()
	defer progressMu.Unlock()

	applyProgressCallbackLocked()
}

func applyProgressCallbackLocked() {
	if setProgressCallbackFunc == (ffi.Fun{}) {
		return
	}

	var nilData unsafe.Pointer
	if progressUserHandler == nil {
		var nilCallback unsafe.Pointer
		setProgressCallbackFunc.Call(nil, unsafe.Pointer(&nilCallback), unsafe.Pointer(&nilData))
		return
	}

	setProgressCallbackFunc.Call(nil, unsafe.Pointer(&progressCallbackCode), unsafe.Pointer(&nilData))
	runtime.KeepAlive(progressCallbackFun)
}

func progressTrampoline(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	arguments := (*[4]unsafe.Pointer)(unsafe.Pointer(args))
	step := int(*(*int32)(arguments[0]))
	steps := int(*(*int32)(arguments[1]))
	secondsPerStep := *(*float32)(arguments[2])

	progressMu.Lock()
	handler := progressUserHandler
	progressMu.Unlock()

	if handler != nil {
		handler(step, steps, secondsPerStep)
	}

	return 0
}
