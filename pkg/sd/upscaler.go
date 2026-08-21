package sd

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// UpscalerContext is an opaque native upscaler context.
type UpscalerContext uintptr

// ADetailerContext is an opaque native ADetailer context.
type ADetailerContext uintptr

// cADetailerParams mirrors sd_adetailer_params_t. Size: 24 bytes.
type cADetailerParams struct {
	Prompt         uintptr
	NegativePrompt uintptr
	ExtraArgs      uintptr
}

var (
	newUpscalerCtxFunc   ffi.Fun
	freeUpscalerCtxFunc  ffi.Fun
	upscaleFunc          ffi.Fun
	getUpscaleFactorFunc ffi.Fun

	newADetailerCtxFunc  ffi.Fun
	freeADetailerCtxFunc ffi.Fun
	adetailImageFunc     ffi.Fun
)

// loadUpscalerFuncs optionally loads the upscaler and ADetailer APIs so
// libraries that predate them remain loadable.
func loadUpscalerFuncs(lib ffi.Lib) {
	newUpscalerCtxFunc = prepOptional(lib, "new_upscaler_ctx", &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeUint8, &ffi.TypeSint32, &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer)
	freeUpscalerCtxFunc = prepOptional(lib, "free_upscaler_ctx", &ffi.TypeVoid, &ffi.TypePointer)
	upscaleFunc = prepOptional(lib, "upscale", &ffi.TypeUint8, &ffi.TypePointer, &ffiTypeImage, &ffi.TypeUint32, &ffi.TypePointer, &ffi.TypePointer)
	getUpscaleFactorFunc = prepOptional(lib, "get_upscale_factor", &ffi.TypeSint32, &ffi.TypePointer)

	newADetailerCtxFunc = prepOptional(lib, "new_adetailer_ctx", &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer)
	freeADetailerCtxFunc = prepOptional(lib, "free_adetailer_ctx", &ffi.TypeVoid, &ffi.TypePointer)
	adetailImageFunc = prepOptional(lib, "adetail_image", &ffi.TypeUint8, &ffi.TypePointer, &ffi.TypePointer, &ffiTypeImage, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
}

// NewUpscalerContext loads an ESRGAN model and returns an upscaler context.
func NewUpscalerContext(esrganPath string, direct bool, nThreads int32, tileSize int32, backend string, paramsBackend string) (UpscalerContext, error) {
	if newUpscalerCtxFunc == (ffi.Fun{}) {
		return 0, unsupported("new_upscaler_ctx")
	}

	var refs cStringRefs
	path, err := refs.add(esrganPath)
	if err != nil {
		return 0, err
	}
	backendPtr, err := refs.add(backend)
	if err != nil {
		return 0, err
	}
	paramsBackendPtr, err := refs.add(paramsBackend)
	if err != nil {
		return 0, err
	}
	directValue := boolToU8(direct)
	var ctx UpscalerContext
	newUpscalerCtxFunc.Call(unsafe.Pointer(&ctx), unsafe.Pointer(&path), unsafe.Pointer(&directValue), unsafe.Pointer(&nThreads), unsafe.Pointer(&tileSize), unsafe.Pointer(&backendPtr), unsafe.Pointer(&paramsBackendPtr))
	runtime.KeepAlive(refs.keep)
	if ctx == 0 {
		return 0, errors.New("new_upscaler_ctx returned NULL")
	}
	return ctx, nil
}

// FreeUpscalerContext releases an upscaler context. A nil context is ignored.
func FreeUpscalerContext(ctx UpscalerContext) {
	if ctx == 0 || freeUpscalerCtxFunc == (ffi.Fun{}) {
		return
	}
	freeUpscalerCtxFunc.Call(nil, unsafe.Pointer(&ctx))
}

// GetUpscaleFactor returns the model's native upscale factor.
func GetUpscaleFactor(ctx UpscalerContext) (int32, error) {
	if ctx == 0 {
		return 0, errors.New("GetUpscaleFactor: nil context")
	}
	if getUpscaleFactorFunc == (ffi.Fun{}) {
		return 0, unsupported("get_upscale_factor")
	}
	var factor int32
	getUpscaleFactorFunc.Call(unsafe.Pointer(&factor), unsafe.Pointer(&ctx))
	return factor, nil
}

// Upscale upscales an image and returns every native result as a Go-owned copy.
func Upscale(ctx UpscalerContext, input *SDImage, factor uint32) ([]*SDImage, error) {
	if ctx == 0 {
		return nil, errors.New("Upscale: nil context")
	}
	if upscaleFunc == (ffi.Fun{}) {
		return nil, unsupported("upscale")
	}
	if input == nil {
		return nil, errors.New("Upscale: nil input image")
	}
	var raw cImage
	if err := bindCImage(&raw, input, "input"); err != nil {
		return nil, err
	}
	images, err := callImageArray(upscaleFunc, unsafe.Pointer(&ctx), unsafe.Pointer(&raw), unsafe.Pointer(&factor))
	runtime.KeepAlive(input)
	return images, err
}

// NewADetailerContext loads a detector model and returns an ADetailer context.
func NewADetailerContext(detectorPath string, nThreads int32, backend string, paramsBackend string) (ADetailerContext, error) {
	if newADetailerCtxFunc == (ffi.Fun{}) {
		return 0, unsupported("new_adetailer_ctx")
	}
	var refs cStringRefs
	path, err := refs.add(detectorPath)
	if err != nil {
		return 0, err
	}
	backendPtr, err := refs.add(backend)
	if err != nil {
		return 0, err
	}
	paramsBackendPtr, err := refs.add(paramsBackend)
	if err != nil {
		return 0, err
	}
	var ctx ADetailerContext
	newADetailerCtxFunc.Call(unsafe.Pointer(&ctx), unsafe.Pointer(&path), unsafe.Pointer(&nThreads), unsafe.Pointer(&backendPtr), unsafe.Pointer(&paramsBackendPtr))
	runtime.KeepAlive(refs.keep)
	if ctx == 0 {
		return 0, errors.New("new_adetailer_ctx returned NULL")
	}
	return ctx, nil
}

// FreeADetailerContext releases an ADetailer context. A nil context is ignored.
func FreeADetailerContext(ctx ADetailerContext) {
	if ctx == 0 || freeADetailerCtxFunc == (ffi.Fun{}) {
		return
	}
	freeADetailerCtxFunc.Call(nil, unsafe.Pointer(&ctx))
}

// ADetailImage detects and refines image regions using the supplied inpaint parameters.
func ADetailImage(adetailerCtx ADetailerContext, ctx Context, input *SDImage, params ADetailerParams, inpaintParams ImgGenParams) ([]*SDImage, error) {
	if adetailerCtx == 0 {
		return nil, errors.New("ADetailImage: nil ADetailer context")
	}
	if ctx == 0 {
		return nil, errors.New("ADetailImage: nil context")
	}
	if adetailImageFunc == (ffi.Fun{}) {
		return nil, unsupported("adetail_image")
	}
	if input == nil {
		return nil, errors.New("ADetailImage: nil input image")
	}
	var rawImage cImage
	if err := bindCImage(&rawImage, input, "input"); err != nil {
		return nil, err
	}
	inpaint, err := marshalImgGenParams(inpaintParams)
	if err != nil {
		return nil, err
	}
	var refs cStringRefs
	rawParams := cADetailerParams{}
	for _, item := range []struct {
		dst   *uintptr
		value string
	}{{&rawParams.Prompt, params.Prompt}, {&rawParams.NegativePrompt, params.NegativePrompt}, {&rawParams.ExtraArgs, params.ExtraArgs}} {
		ptr, err := refs.add(item.value)
		if err != nil {
			return nil, err
		}
		*item.dst = ptr
	}
	images, err := callImageArray(adetailImageFunc, unsafe.Pointer(&adetailerCtx), unsafe.Pointer(&ctx), unsafe.Pointer(&rawImage), unsafe.Pointer(&rawParams), unsafe.Pointer(&inpaint.raw))
	runtime.KeepAlive(input)
	runtime.KeepAlive(inpaint)
	runtime.KeepAlive(inpaintParams)
	runtime.KeepAlive(refs.keep)
	return images, err
}

func callImageArray(fn ffi.Fun, args ...any) ([]*SDImage, error) {
	var resultPtr *cImage
	var resultCount int32
	resultPtrPtr := &resultPtr
	resultCountPtr := &resultCount
	args = append(args, unsafe.Pointer(&resultPtrPtr), unsafe.Pointer(&resultCountPtr))
	var result ffi.Arg
	fn.Call(&result, args...)
	if byte(result) == 0 || resultPtr == nil || resultCount <= 0 {
		if resultPtr != nil && resultCount > 0 {
			freeSDImagesFunc.Call(nil, unsafe.Pointer(&resultPtr), unsafe.Pointer(&resultCount))
		}
		return nil, errors.New("native image operation failed")
	}
	rawImages := unsafe.Slice(resultPtr, int(resultCount))
	images := make([]*SDImage, resultCount)
	for i := range rawImages {
		images[i] = sdImageFromC(&rawImages[i])
	}
	freeSDImagesFunc.Call(nil, unsafe.Pointer(&resultPtr), unsafe.Pointer(&resultCount))
	return images, nil
}
