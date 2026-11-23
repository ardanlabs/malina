package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

var resultFreeFunc, resultCountFunc, resultImageFunc, resultInfoFunc, resultAudioFunc, resultFPSFunc ffi.Fun

func loadResultFuncs(lib ffi.Lib) error {
	for _, s := range []struct {
		name   string
		dst    *ffi.Fun
		result *ffi.Type
		args   []*ffi.Type
	}{
		{"sd_result_free", &resultFreeFunc, &ffi.TypeVoid, []*ffi.Type{&ffi.TypePointer}},
		{"sd_result_get_image_count", &resultCountFunc, &ffi.TypeUint64, []*ffi.Type{&ffi.TypePointer}},
		{"sd_result_get_image", &resultImageFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"sd_result_get_generation_info", &resultInfoFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"sd_result_get_audio", &resultAudioFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer}},
		{"sd_result_get_fps", &resultFPSFunc, &ffi.TypeSint32, []*ffi.Type{&ffi.TypePointer}},
	} {
		fn, err := lib.Prep(s.name, s.result, s.args...)
		if err != nil {
			return loadError(s.name, err)
		}
		*s.dst = fn
	}
	return nil
}

func callResult(fn ffi.Fun, args ...any) (uintptr, error) {
	var result uintptr
	out := &result
	args = append(args, unsafe.Pointer(&out))
	if err := statusCall(fn, args...); err != nil {
		if result != 0 {
			resultFreeFunc.Call(nil, unsafe.Pointer(&result))
		}
		return 0, err
	}
	if result == 0 {
		return 0, errors.New("native operation: successful status without a result")
	}
	return result, nil
}

func resultImages(result uintptr) ([]SDImage, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var count ffi.Arg
	resultCountFunc.Call(&count, unsafe.Pointer(&result))
	if count == 0 {
		return nil, fmt.Errorf("reading native result: empty result: %s", nativeDiagnostic())
	}
	images := make([]SDImage, int(count))
	for i := range images {
		index := uint64(i)
		var tensor uintptr
		resultImageFunc.Call(unsafe.Pointer(&tensor), unsafe.Pointer(&result), unsafe.Pointer(&index))
		if tensor == 0 {
			return nil, fmt.Errorf("reading result image: %s", nativeDiagnostic())
		}
		image, err := imageFromTensor(tensor)
		if err != nil {
			return nil, err
		}
		var info *byte
		resultInfoFunc.Call(unsafe.Pointer(&info), unsafe.Pointer(&result), unsafe.Pointer(&index))
		if info == nil {
			return nil, fmt.Errorf("reading generation metadata: %s", nativeDiagnostic())
		}
		image.GenerationInfo = cString(info)
		images[i] = image
	}
	return images, nil
}

func callImageArray(fn ffi.Fun, args ...any) ([]SDImage, error) {
	result, err := callResult(fn, args...)
	if err != nil {
		return nil, err
	}
	defer resultFreeFunc.Call(nil, unsafe.Pointer(&result))
	return resultImages(result)
}
