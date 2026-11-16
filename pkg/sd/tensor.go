package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

var tensorCreateFunc, tensorFreeFunc, tensorTypeFunc, tensorRankFunc, tensorDimFunc, tensorSizeFunc, tensorDataFunc ffi.Fun

func loadTensorFuncs(lib ffi.Lib) error {
	for _, s := range []struct {
		name   string
		dst    *ffi.Fun
		result *ffi.Type
		args   []*ffi.Type
	}{
		{"sd_tensor_create", &tensorCreateFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypeSint32, &ffi.TypeUint64, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeUint64}},
		{"sd_tensor_free", &tensorFreeFunc, &ffi.TypeVoid, []*ffi.Type{&ffi.TypePointer}},
		{"sd_tensor_type", &tensorTypeFunc, &ffi.TypeSint32, []*ffi.Type{&ffi.TypePointer}},
		{"sd_tensor_rank", &tensorRankFunc, &ffi.TypeUint64, []*ffi.Type{&ffi.TypePointer}},
		{"sd_tensor_dim", &tensorDimFunc, &ffi.TypeSint64, []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"sd_tensor_byte_size", &tensorSizeFunc, &ffi.TypeUint64, []*ffi.Type{&ffi.TypePointer}},
		{"sd_tensor_data", &tensorDataFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer}},
	} {
		fn, err := lib.Prep(s.name, s.result, s.args...)
		if err != nil {
			return loadError(s.name, err)
		}
		*s.dst = fn
	}
	return nil
}

func createTensor(kind int32, shape []int64, data unsafe.Pointer, size uint64) (uintptr, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rank, dims := uint64(len(shape)), unsafe.SliceData(shape)
	var t uintptr
	tensorCreateFunc.Call(unsafe.Pointer(&t), unsafe.Pointer(&kind), unsafe.Pointer(&rank), unsafe.Pointer(&dims), unsafe.Pointer(&data), unsafe.Pointer(&size))
	runtime.KeepAlive(shape)
	if t == 0 {
		return 0, fmt.Errorf("creating tensor: %s", nativeDiagnostic())
	}
	return t, nil
}

func freeTensor(t uintptr) {
	if t != 0 {
		tensorFreeFunc.Call(nil, unsafe.Pointer(&t))
	}
}

func validateImage(image *SDImage) error {
	if image == nil {
		return errors.New("nil image")
	}
	if image.Width == 0 || image.Height == 0 {
		return errors.New("image: zero dimension")
	}
	if image.Channel != 3 {
		return fmt.Errorf("image: channel count %d, want 3 (RGB)", image.Channel)
	}
	want := uint64(image.Width) * uint64(image.Height) * 3
	if want != uint64(len(image.Data)) {
		return fmt.Errorf("image: data length %d, want %d", len(image.Data), want)
	}
	return nil
}

func imageTensor(image *SDImage) (uintptr, error) {
	if err := validateImage(image); err != nil {
		return 0, err
	}
	t, err := createTensor(1, []int64{int64(image.Channel), int64(image.Width), int64(image.Height)}, unsafe.Pointer(unsafe.SliceData(image.Data)), uint64(len(image.Data)))
	runtime.KeepAlive(image)
	return t, err
}

func framesTensor(frames []*SDImage) (uintptr, error) {
	if len(frames) == 0 {
		return 0, errors.New("video: no frames")
	}
	var data []byte
	for _, f := range frames {
		if err := validateImage(f); err != nil {
			return 0, err
		}
		if f.Width != frames[0].Width || f.Height != frames[0].Height {
			return 0, errors.New("video: frame dimensions differ")
		}
		data = append(data, f.Data...)
	}
	f := frames[0]
	t, err := createTensor(1, []int64{3, int64(f.Width), int64(f.Height), int64(len(frames))}, unsafe.Pointer(unsafe.SliceData(data)), uint64(len(data)))
	runtime.KeepAlive(data)
	return t, err
}

func tensorShape(t uintptr, kind int32, rank int) ([]int64, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var typ, dimensions ffi.Arg
	tensorTypeFunc.Call(&typ, unsafe.Pointer(&t))
	if typ == 0 {
		return nil, fmt.Errorf("reading tensor type: %s", nativeDiagnostic())
	}
	tensorRankFunc.Call(&dimensions, unsafe.Pointer(&t))
	if int32(typ) != kind || int(dimensions) != rank {
		return nil, fmt.Errorf("tensor: got type %d rank %d, want type %d rank %d", typ, dimensions, kind, rank)
	}
	shape := make([]int64, rank)
	for i := range shape {
		axis := uint64(i)
		tensorDimFunc.Call(unsafe.Pointer(&shape[i]), unsafe.Pointer(&t), unsafe.Pointer(&axis))
		if shape[i] <= 0 {
			return nil, errors.New("tensor: nonpositive media dimension")
		}
	}
	return shape, nil
}

func tensorBytes(t uintptr) ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var size ffi.Arg
	tensorSizeFunc.Call(&size, unsafe.Pointer(&t))
	if t == 0 {
		return nil, fmt.Errorf("reading tensor size: %s", nativeDiagnostic())
	}
	var data *byte
	tensorDataFunc.Call(unsafe.Pointer(&data), unsafe.Pointer(&t))
	if data == nil && size != 0 {
		return nil, fmt.Errorf("reading tensor data: %s", nativeDiagnostic())
	}
	if uint64(size) > uint64(^uint(0)>>1) {
		return nil, errors.New("tensor: storage exceeds Go slice size")
	}
	return append([]byte(nil), unsafe.Slice(data, int(size))...), nil
}

func imageFromTensor(t uintptr) (*SDImage, error) {
	shape, err := tensorShape(t, 1, 3)
	if err != nil {
		return nil, err
	}
	for _, n := range shape {
		if n > 1<<32-1 {
			return nil, errors.New("image: dimension overflow")
		}
	}
	data, err := tensorBytes(t)
	if err != nil {
		return nil, err
	}
	out := SDImage{Channel: uint32(shape[0]), Width: uint32(shape[1]), Height: uint32(shape[2]), Data: data}
	return &out, nil
}
