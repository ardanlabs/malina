package sd

import (
	"fmt"
	"io"
	"os"
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

	// Persistent state for the C-callable trampoline. libffi requires
	// these to outlive the FFI call, so they are package-scoped.
	logCifCallback  ffi.Cif
	logClosure      *ffi.Closure
	logCallbackCode unsafe.Pointer
	logCallbackFun  uintptr

	logMu          sync.Mutex
	logUserHandler LogCallback
	logLastError   string
	logLastWarn    string
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
	return nil
}

// installLogCallback registers a process-wide log handler with the C
// library. Called once from Load. Safe to call again; the closure is reused.
func installLogCallback() error {
	if setLogCallbackFunc == (ffi.Fun{}) {
		return nil
	}
	if logClosure != nil {
		// Already installed for this process.
		return nil
	}

	// Describe sd_log_cb_t: void (*)(sd_log_level_t, const char*, void*).
	if status := ffi.PrepCif(&logCifCallback, ffi.DefaultAbi, 3,
		&ffi.TypeVoid,
		&ffi.TypeSint32,
		&ffi.TypePointer,
		&ffi.TypePointer,
	); status != ffi.OK {
		return fmt.Errorf("PrepCif sd_log_cb_t: %v", status)
	}

	logClosure = ffi.ClosureAlloc(unsafe.Sizeof(ffi.Closure{}), &logCallbackCode)
	if logClosure == nil {
		return fmt.Errorf("ffi.ClosureAlloc returned nil")
	}

	logCallbackFun = ffi.NewCallback(logTrampoline)
	if status := ffi.PrepClosureLoc(logClosure, &logCifCallback, logCallbackFun, nil, logCallbackCode); status != ffi.OK {
		return fmt.Errorf("PrepClosureLoc sd_log_cb_t: %v", status)
	}

	var nilData unsafe.Pointer
	setLogCallbackFunc.Call(nil, unsafe.Pointer(&logCallbackCode), unsafe.Pointer(&nilData))
	return nil
}

// logTrampoline is the C-callable function libffi invokes for every log line.
// args is &[3]unsafe.Pointer{ &level, &textPtr, &dataPtr }.
func logTrampoline(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	argsArr := (*[3]unsafe.Pointer)(unsafe.Pointer(args))
	level := LogLevel(*(*int32)(argsArr[0]))
	textPtr := *(**byte)(argsArr[1])

	text := strings.TrimRight(utils.BytePtrToString(textPtr), "\n")

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
		return 0
	}

	if level == LogWarn || level == LogError {
		fmt.Fprintf(LogWriter, "[sd %s] %s\n", levelTag(level), text)
	}
	return 0
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
// by stable-diffusion.cpp. Pass nil to restore the default warn/error-to-
// stderr handler. Safe to call from any goroutine.
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
