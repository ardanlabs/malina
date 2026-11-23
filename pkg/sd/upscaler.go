package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// UpscalerContext is an opaque native image upscaler.
type UpscalerContext uintptr

// ADetailerContext is an opaque native detector.
type ADetailerContext uintptr

var newUpscalerCtxFunc, freeUpscalerCtxFunc, upscaleFunc, getUpscaleFactorFunc, getUpscalerScaleFunc ffi.Fun
var newADetailerCtxFunc, freeADetailerCtxFunc, adetailImageFunc ffi.Fun

func loadUpscalerFuncs(lib ffi.Lib) {
	newUpscalerCtxFunc = prepOptional(lib, "sd_upscaler_create", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer)
	freeUpscalerCtxFunc = prepOptional(lib, "sd_upscaler_free", &ffi.TypeVoid, &ffi.TypePointer)
	upscaleFunc = prepOptional(lib, "sd_upscale", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
	getUpscaleFactorFunc = prepOptional(lib, "sd_upscaler_get_scale", &ffi.TypeSint32, &ffi.TypePointer)
	getUpscalerScaleFunc = prepOptional(lib, "sd_upscaler_probe_scale", &ffi.TypeSint32, &ffi.TypePointer)
	newADetailerCtxFunc = prepOptional(lib, "sd_adetailer_create", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer)
	freeADetailerCtxFunc = prepOptional(lib, "sd_adetailer_free", &ffi.TypeVoid, &ffi.TypePointer)
	adetailImageFunc = prepOptional(lib, "sd_adetail", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
}

// NewUpscalerContext loads an RGB upscaler. Scale comes from the model.
func NewUpscalerContext(path string, direct bool, nThreads, tileSize int32, backend, paramsBackend string) (UpscalerContext, error) {
	values := Options{"model_path": path, "conv_direct": direct}
	if nThreads > 0 {
		values["n_threads"] = nThreads
	}
	if tileSize != 0 {
		values["tile_size"] = tileSize
	}
	if backend != "" {
		values["backend"] = backend
	}
	if paramsBackend != "" {
		values["params_backend"] = paramsBackend
	}
	ctx, err := createPostprocessor("upscaler", newUpscalerCtxFunc, values)
	return UpscalerContext(ctx), err
}

func createPostprocessor(purpose string, fn ffi.Fun, values Options) (uintptr, error) {
	if fn == (ffi.Fun{}) {
		return 0, unsupported(purpose)
	}
	o, err := newOptions(purpose)
	if err != nil {
		return 0, err
	}
	defer o.free()
	if err := o.apply(values); err != nil {
		return 0, err
	}
	var handle uintptr
	out := &handle
	if err := statusCall(fn, unsafe.Pointer(&o.ptr), unsafe.Pointer(&out)); err != nil {
		return 0, err
	}
	return handle, nil
}

// FreeUpscalerContext releases an upscaler. Zero is ignored.
func FreeUpscalerContext(ctx UpscalerContext) {
	if ctx != 0 {
		freeUpscalerCtxFunc.Call(nil, unsafe.Pointer(&ctx))
	}
}

// GetUpscaleFactor returns the loaded model's actual scale.
func GetUpscaleFactor(ctx UpscalerContext) (int32, error) {
	if ctx == 0 {
		return 0, errors.New("GetUpscaleFactor: nil context")
	}
	if getUpscaleFactorFunc == (ffi.Fun{}) {
		return 0, unsupported("sd_upscaler_get_scale")
	}
	var result ffi.Arg
	getUpscaleFactorFunc.Call(&result, unsafe.Pointer(&ctx))
	return int32(result), nil
}

// GetUpscalerModelScale reads scale metadata without loading model weights.
func GetUpscalerModelScale(path string) (int32, error) {
	if getUpscalerScaleFunc == (ffi.Fun{}) {
		return 0, unsupported("sd_upscaler_probe_scale")
	}
	var refs cStringRefs
	p, err := refs.add(path)
	if err != nil {
		return 0, err
	}
	var result ffi.Arg
	getUpscalerScaleFunc.Call(&result, unsafe.Pointer(&p))
	runtime.KeepAlive(refs)
	return int32(result), nil
}

// Upscale returns Go-owned pixels and metadata. The legacy factor argument must
// equal the model's actual scale; it is no longer silently ignored upstream.
// Use UpscaleImage when the model should determine the scale automatically.
func Upscale(ctx UpscalerContext, image SDImage, factor uint32) ([]SDImage, error) {
	actual, err := GetUpscaleFactor(ctx)
	if err != nil {
		return nil, err
	}
	if factor != uint32(actual) {
		return nil, fmt.Errorf("upscale: requested factor %d differs from model scale %d", factor, actual)
	}
	return UpscaleImage(ctx, image)
}

// UpscaleImage upscales using the loaded model's native scale.
func UpscaleImage(ctx UpscalerContext, image SDImage) ([]SDImage, error) {
	if ctx == 0 {
		return nil, errors.New("UpscaleImage: nil context")
	}
	if upscaleFunc == (ffi.Fun{}) {
		return nil, unsupported("sd_upscale")
	}
	o, err := newOptions("upscale")
	if err != nil {
		return nil, err
	}
	defer o.free()
	if err := validateImage(image); err != nil {
		return nil, err
	}
	if err := o.set("image", image); err != nil {
		return nil, err
	}
	return callImageArray(upscaleFunc, unsafe.Pointer(&ctx), unsafe.Pointer(&o.ptr))
}

// NewADetailerContext loads a detector model.
// Deprecated: removed upstream. Use RefineFaces with FaceRefinementParams.
func NewADetailerContext(path string, nThreads int32, backend, paramsBackend string) (ADetailerContext, error) {
	values := Options{"model_path": path}
	if nThreads > 0 {
		values["n_threads"] = nThreads
	}
	if backend != "" {
		values["backend"] = backend
	}
	if paramsBackend != "" {
		values["params_backend"] = paramsBackend
	}
	ctx, err := createPostprocessor("adetailer", newADetailerCtxFunc, values)
	return ADetailerContext(ctx), err
}

// FreeADetailerContext releases a detector. Zero is ignored.
func FreeADetailerContext(ctx ADetailerContext) {
	if ctx != 0 {
		freeADetailerCtxFunc.Call(nil, unsafe.Pointer(&ctx))
	}
}

// ADetailImage refines detected regions. Image-only inpaint settings unsupported
// by ADetailer produce a native applicability error instead of being ignored.
// Deprecated: removed upstream. Use RefineFaces with FaceRefinementParams.
func ADetailImage(detector ADetailerContext, ctx Context, image SDImage, params ADetailerParams, inpaint ImgGenParams) ([]SDImage, error) {
	if detector == 0 || ctx == 0 {
		return nil, errors.New("ADetailImage: nil context")
	}
	if adetailImageFunc == (ffi.Fun{}) {
		return nil, unsupported("sd_adetail")
	}
	if err := validateImage(image); err != nil {
		return nil, err
	}
	o, err := newOptions("adetail")
	if err != nil {
		return nil, err
	}
	defer o.free()
	if err := o.fields(inpaint, ImgGenParamsInit(), ""); err != nil {
		return nil, err
	}
	if err := o.apply(Options{"image": image, "adetail.prompt": params.Prompt, "adetail.negative_prompt": params.NegativePrompt, "adetail.args": params.ExtraArgs}); err != nil {
		return nil, err
	}
	return callImageArray(adetailImageFunc, unsafe.Pointer(&detector), unsafe.Pointer(&ctx), unsafe.Pointer(&o.ptr))
}
