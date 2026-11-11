package sd

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// ProgressCallback receives process-global model loading and generation
// progress. SecondsPerStep is the average elapsed time per completed step.
type ProgressCallback func(step int, steps int, secondsPerStep float32)

// GenerationProgress is a Go-owned copy of a context-local native event.
// Total == 0 means unknown. SamplePass counts from one since registration.
type GenerationProgress struct {
	Stage      string
	Detail     string
	Unit       string
	SamplePass int32
	Completed  int64
	Total      int64
}

// GenerationProgressCallback receives synchronous generation events, separate
// from previews and legacy progress. It must not panic or reenter the context.
type GenerationProgressCallback func(GenerationProgress)

// cGenerationProgress mirrors sd_generation_progress_t (LP64/LLP64: 48 bytes).
type cGenerationProgress struct {
	Stage      *byte
	Detail     *byte
	Unit       *byte
	SamplePass int32
	_          [4]byte
	Completed  int64
	Total      int64
}

var (
	// SD_API void sd_set_progress_callback(sd_progress_cb_t cb, void* data);
	setProgressCallbackFunc ffi.Fun

	progressCif          ffi.Cif
	progressClosure      *ffi.Closure
	progressCallbackCode unsafe.Pointer
	progressCallbackFun  uintptr

	progressMu          sync.Mutex
	progressUserHandler ProgressCallback

	setGenerationProgressFunc  ffi.Fun
	generationProgressCif      ffi.Cif
	generationProgressClosure  *ffi.Closure
	generationProgressCode     unsafe.Pointer
	generationProgressFun      uintptr
	generationProgressHandlers map[Context]GenerationProgressCallback
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

	setGenerationProgressFunc = prepOptional(lib, "sd_set_generation_progress_callback", &ffi.TypeVoid,
		&ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
	if setGenerationProgressFunc != (ffi.Fun{}) && generationProgressClosure == nil {
		if status := ffi.PrepCif(&generationProgressCif, ffi.DefaultAbi, 2, &ffi.TypeVoid,
			&ffi.TypePointer, &ffi.TypePointer); status != ffi.OK {
			return fmt.Errorf("PrepCif sd_generation_progress_cb_t: %v", status)
		}
		closure, code, callback, err := allocateCallbackClosure("sd_generation_progress_cb_t", &generationProgressCif, generationProgressTrampoline)
		if err != nil {
			return err
		}
		generationProgressClosure = closure
		generationProgressCode = code
		generationProgressFun = callback
	}

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

// SetGenerationProgressCallback installs a context-local generation callback.
// Register or clear it only while ctx is idle. Pass nil to clear it; FreeContext
// also releases the Go handler. Different contexts can report concurrently.
func SetGenerationProgressCallback(ctx Context, callback GenerationProgressCallback) error {
	if ctx == 0 {
		return errors.New("SetGenerationProgressCallback: nil context")
	}
	progressMu.Lock()
	defer progressMu.Unlock()
	if setGenerationProgressFunc == (ffi.Fun{}) {
		return unsupported("sd_set_generation_progress_callback")
	}
	var code unsafe.Pointer
	if callback == nil {
		delete(generationProgressHandlers, ctx)
	} else {
		if generationProgressHandlers == nil {
			generationProgressHandlers = make(map[Context]GenerationProgressCallback)
		}
		generationProgressHandlers[ctx] = callback
		code = generationProgressCode
	}
	// The opaque native context address is the lookup key, not a Go pointer.
	data := uintptr(ctx)
	setGenerationProgressFunc.Call(nil, unsafe.Pointer(&ctx), unsafe.Pointer(&code), unsafe.Pointer(&data))
	runtime.KeepAlive(generationProgressFun)
	return nil
}

func generationProgressTrampoline(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	arguments := (*[2]unsafe.Pointer)(unsafe.Pointer(args))
	raw := *(**cGenerationProgress)(arguments[0])
	ctx := Context(*(*uintptr)(arguments[1]))
	progressMu.Lock()
	handler := generationProgressHandlers[ctx]
	progressMu.Unlock()
	if handler != nil && raw != nil {
		handler(GenerationProgress{
			Stage: cString(raw.Stage), Detail: cString(raw.Detail), Unit: cString(raw.Unit),
			SamplePass: raw.SamplePass, Completed: raw.Completed, Total: raw.Total,
		})
	}
	return 0
}
