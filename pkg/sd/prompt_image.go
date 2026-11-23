package sd

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// PromptImageCallback resolves an <image:source> marker. Return SDImage{} for an
// unresolved source. Pixels are copied before returning to native code. The
// callback must not panic and must not mutate its context or registration.
type PromptImageCallback func(source string) SDImage

type promptImageState struct {
	handler PromptImageCallback
	tensor  uintptr
}

var promptImageMu sync.Mutex
var promptImageHandlers = make(map[Context]*promptImageState)
var setPromptImageFunc ffi.Fun
var promptImageCif ffi.Cif
var promptImageClosure *ffi.Closure
var promptImageCode unsafe.Pointer
var promptImageFun uintptr

func loadPromptImageFunc(lib ffi.Lib) error {
	setPromptImageFunc = prepOptional(lib, "sd_set_prompt_image_callback", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
	if setPromptImageFunc == (ffi.Fun{}) {
		return unsupported("sd_set_prompt_image_callback")
	}
	if promptImageClosure != nil {
		return nil
	}
	if status := ffi.PrepCif(&promptImageCif, ffi.DefaultAbi, 3, &ffi.TypeUint8, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer); status != ffi.OK {
		return fmt.Errorf("PrepCif sd_prompt_image_cb_t: %v", status)
	}
	var err error
	promptImageClosure, promptImageCode, promptImageFun, err = allocateCallbackClosure("sd_prompt_image_cb_t", &promptImageCif, promptImageTrampoline)
	return err
}

// SetPromptImageCallback installs a context-local resolver for models supporting
// interleaved image prompts (such as MOSS-Image). Register/clear only while idle.
// Nil clears registration and releases the last native image snapshot.
func SetPromptImageCallback(ctx Context, callback PromptImageCallback) error {
	if ctx == 0 {
		return errors.New("SetPromptImageCallback: nil context")
	}
	if setPromptImageFunc == (ffi.Fun{}) {
		return unsupported("sd_set_prompt_image_callback")
	}
	promptImageMu.Lock()
	defer promptImageMu.Unlock()
	var code unsafe.Pointer
	if callback != nil {
		code = promptImageCode
	}
	data := uintptr(ctx)
	setPromptImageFunc.Call(nil, unsafe.Pointer(&ctx), unsafe.Pointer(&code), unsafe.Pointer(&data))
	if previous := promptImageHandlers[ctx]; previous != nil {
		freeTensor(previous.tensor)
	}
	delete(promptImageHandlers, ctx)
	if callback != nil {
		promptImageHandlers[ctx] = &promptImageState{handler: callback}
	}
	runtime.KeepAlive(promptImageFun)
	return nil
}

func clearPromptImage(ctx Context) {
	promptImageMu.Lock()
	defer promptImageMu.Unlock()
	if state := promptImageHandlers[ctx]; state != nil {
		freeTensor(state.tensor)
		delete(promptImageHandlers, ctx)
	}
}

func promptImageTrampoline(_ *ffi.Cif, ret unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
	a := (*[3]unsafe.Pointer)(unsafe.Pointer(args))
	source := cString(*(**byte)(a[0]))
	ctx := Context(*(*uintptr)(a[1]))
	out := *(**uintptr)(a[2])
	*(*uint8)(ret) = 0
	if out == nil {
		return 0
	}
	*out = 0
	promptImageMu.Lock()
	state := promptImageHandlers[ctx]
	promptImageMu.Unlock()
	if state == nil {
		return 0
	}
	freeTensor(state.tensor)
	state.tensor = 0
	image := state.handler(source)
	tensor, err := imageTensor(image)
	if err != nil {
		return 0
	}
	state.tensor = tensor
	*out = tensor
	*(*uint8)(ret) = 1
	return 0
}
