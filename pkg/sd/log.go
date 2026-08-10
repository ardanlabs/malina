package sd

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

// LogCallback receives every log line emitted by stable-diffusion.cpp. text
// has had its trailing newline stripped.
type LogCallback func(level LogLevel, text string)

var (
	// SD_API void sd_set_log_callback(sd_log_cb_t sd_log_cb, void* data);
	setLogCallbackFunc ffi.Fun

	// GGML_API void ggml_log_set(ggml_log_callback log_callback, void* user_data);
	//
	// Loaded best-effort because static builds may not export the symbol.
	// When available, configuring it prevents backend initialization messages
	// such as ggml_metal_* from bypassing the stable-diffusion callback.
	ggmlLogSetFunc ffi.Fun

	// Persistent state for the C-callable trampoline. libffi requires
	// these to outlive the FFI call, so they are package-scoped.
	logCifCallback  ffi.Cif
	logClosure      *ffi.Closure
	logCallbackCode unsafe.Pointer
	logCallbackFun  uintptr
	ggmlLogCif      ffi.Cif
	ggmlLogClosure  *ffi.Closure
	ggmlLogCode     unsafe.Pointer
	ggmlLogFun      uintptr

	logMu          sync.Mutex
	logUserHandler LogCallback
	logLastError   string
	logLastWarn    string
	ggmlLastLevel  = LogInfo
)

// LogWriter is where the default log handler writes warning/error lines.
// Defaults to os.Stderr. Set to io.Discard to silence the package without
// removing the callback (LastError/LastWarning remain populated).
var LogWriter io.Writer = os.Stderr

func loadLogFuncs(lib ffi.Lib) error {
	fn, err := lib.Prep("sd_set_log_callback", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypePointer)
	if err != nil {
		return loadError("sd_set_log_callback", err)
	}
	setLogCallbackFunc = fn
	loadGGMLLogFunc(lib)

	return nil
}

func loadGGMLLogFunc(lib ffi.Lib) {
	if ggmlLogSetFunc != (ffi.Fun{}) {
		return
	}

	if fn, err := lib.Prep("ggml_log_set", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypePointer); err == nil {
		ggmlLogSetFunc = fn
	}
}

// installLogCallback registers a process-wide log handler with the C
// library. Called once from Load. Safe to call again; the closure is reused.
func installLogCallback() error {
	if setLogCallbackFunc == (ffi.Fun{}) {
		return nil
	}
	if logClosure == nil {
		closure, code, callback, err := prepareLogClosure("sd_log_cb_t", &logCifCallback, logTrampoline)
		if err != nil {
			return err
		}
		logClosure = closure
		logCallbackCode = code
		logCallbackFun = callback
	}

	if ggmlLogSetFunc != (ffi.Fun{}) {
		if err := installGGMLLogCallback(); err != nil {
			return err
		}
	}

	var nilData unsafe.Pointer
	setLogCallbackFunc.Call(nil, unsafe.Pointer(&logCallbackCode), unsafe.Pointer(&nilData))
	if ggmlLogSetFunc != (ffi.Fun{}) {
		ggmlLogSetFunc.Call(nil, unsafe.Pointer(&ggmlLogCode), unsafe.Pointer(&nilData))
	}
	runtime.KeepAlive(logCallbackFun)
	runtime.KeepAlive(ggmlLogFun)

	return nil
}

func installGGMLLogCallback() error {
	if ggmlLogClosure != nil {
		return nil
	}

	closure, code, callback, err := prepareLogClosure("ggml_log_callback", &ggmlLogCif, ggmlLogTrampoline)
	if err != nil {
		return err
	}
	ggmlLogClosure = closure
	ggmlLogCode = code
	ggmlLogFun = callback

	return nil
}

func prepareLogClosure(name string, cif *ffi.Cif, callback ffi.Callback) (*ffi.Closure, unsafe.Pointer, uintptr, error) {
	// Both native callbacks have the signature:
	// void (*)(int32_t level, const char* text, void* user_data).
	if status := ffi.PrepCif(cif, ffi.DefaultAbi, 3,
		&ffi.TypeVoid,
		&ffi.TypeSint32,
		&ffi.TypePointer,
		&ffi.TypePointer,
	); status != ffi.OK {
		return nil, nil, 0, fmt.Errorf("PrepCif %s: %v", name, status)
	}

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

// logTrampoline is the C-callable function libffi invokes for every log line.
// args is &[3]unsafe.Pointer{ &level, &textPtr, &dataPtr }.
func logTrampoline(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	argsArr := (*[3]unsafe.Pointer)(unsafe.Pointer(args))
	level := LogLevel(*(*int32)(argsArr[0]))
	textPtr := *(**byte)(argsArr[1])

	dispatchLog(level, strings.TrimRight(utils.BytePtrToString(textPtr), "\n"))
	return 0
}

func ggmlLogTrampoline(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	argsArr := (*[3]unsafe.Pointer)(unsafe.Pointer(args))
	rawLevel := *(*int32)(argsArr[0])
	textPtr := *(**byte)(argsArr[1])

	logMu.Lock()
	ggmlLastLevel = mapGGMLLogLevel(rawLevel, ggmlLastLevel)
	level := ggmlLastLevel
	logMu.Unlock()

	dispatchLog(level, strings.TrimRight(utils.BytePtrToString(textPtr), "\n"))
	return 0
}

func mapGGMLLogLevel(rawLevel int32, previous LogLevel) LogLevel {
	switch rawLevel {
	case 1: // GGML_LOG_LEVEL_DEBUG
		return LogDebug
	case 2: // GGML_LOG_LEVEL_INFO
		return LogInfo
	case 3: // GGML_LOG_LEVEL_WARN
		return LogWarn
	case 4: // GGML_LOG_LEVEL_ERROR
		return LogError
	case 5: // GGML_LOG_LEVEL_CONT
		return previous
	default: // GGML_LOG_LEVEL_NONE or an unknown future value.
		return LogDebug
	}
}

func dispatchLog(level LogLevel, text string) {
	logMu.Lock()
	switch level {
	case LogError:
		logLastError = text
	case LogWarn:
		logLastWarn = text
	}
	handler := logUserHandler
	logMu.Unlock()

	if handler != nil {
		handler(level, text)
		return
	}

	if level == LogWarn || level == LogError {
		fmt.Fprintf(LogWriter, "[sd %s] %s\n", levelTag(level), text)
	}
}

func levelTag(l LogLevel) string {
	switch l {
	case LogDebug:
		return "debug"
	case LogInfo:
		return "info"
	case LogWarn:
		return "warn"
	case LogError:
		return "error"
	default:
		return fmt.Sprintf("level=%d", int32(l))
	}
}

// SetLogCallback installs a Go callback that receives every log line emitted
// by stable-diffusion.cpp and, when supported by the loaded library, GGML.
// Pass nil to restore the default warn/error-to-stderr handler. Safe to call
// from any goroutine.
func SetLogCallback(cb LogCallback) {
	logMu.Lock()
	logUserHandler = cb
	logMu.Unlock()
}

// LastError returns the most recent LogError line seen by the package, or
// "" if none has been seen. Useful for appending to errors returned by
// FFI calls that report failure as a NULL return.
func LastError() string {
	logMu.Lock()
	defer logMu.Unlock()
	return logLastError
}

// LastWarning returns the most recent LogWarn line seen by the package.
func LastWarning() string {
	logMu.Lock()
	defer logMu.Unlock()
	return logLastWarn
}

// clearLastLog resets LastError / LastWarning. Called immediately before an
// FFI call whose failure mode we want to attribute to a fresh log line.
func clearLastLog() {
	logMu.Lock()
	logLastError = ""
	logLastWarn = ""
	logMu.Unlock()
}
