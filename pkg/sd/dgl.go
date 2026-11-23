package sd

import (
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// DGLTensor is a Go-owned flat output tensor. Type uses sd_tensor_type_t values
// (1 U8, 2 I32, 3 I64, 4 F32, 5 F64); Shape is channel-first native
// order, with the first dimension contiguous. Data is the exact native bytes.
type DGLTensor struct {
	Type  int32
	Shape []int64
	Data  []byte
}

// Images copies a U8 [C,W,H] image or [C,W,H,T] video into Go-owned images.
func (t DGLTensor) Images() ([]SDImage, error) {
	if t.Type != 1 || (len(t.Shape) != 3 && len(t.Shape) != 4) {
		return nil, errors.New("DGL tensor: expected U8 image or video")
	}
	count := int64(1)
	if len(t.Shape) == 4 {
		count = t.Shape[3]
	}
	size := uint64(1)
	for _, n := range t.Shape {
		if n <= 0 || n > 1<<32-1 || size > uint64(len(t.Data))/uint64(n) {
			return nil, errors.New("DGL tensor: invalid media shape or storage")
		}
		size *= uint64(n)
	}
	if size != uint64(len(t.Data)) {
		return nil, errors.New("DGL tensor: media storage size mismatch")
	}
	frameSize := len(t.Data) / int(count)
	images := make([]SDImage, int(count))
	for i := range images {
		images[i] = SDImage{Channel: uint32(t.Shape[0]), Width: uint32(t.Shape[1]), Height: uint32(t.Shape[2]), Data: slices.Clone(t.Data[i*frameSize : (i+1)*frameSize])}
	}
	return images, nil
}

// cDGLInput mirrors sd_dgl_tensor_input_t.
type cDGLInput struct{ Name, Tensor uintptr }

var ctxDGLFunc, dglRunFunc, dglFreeFunc, dglCountFunc, dglNameFunc, dglTensorFunc, generationMetadataFunc ffi.Fun

func loadDGLFuncs(lib ffi.Lib) error {
	for _, s := range []struct {
		name   string
		dst    *ffi.Fun
		result *ffi.Type
		args   []*ffi.Type
	}{
		{"sd_ctx_get_dgl_ctx", &ctxDGLFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer}},
		{"sd_dgl_run", &dglRunFunc, &ffi.TypeSint32, []*ffi.Type{&ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeUint64, &ffi.TypePointer}},
		{"sd_dgl_result_free", &dglFreeFunc, &ffi.TypeVoid, []*ffi.Type{&ffi.TypePointer}},
		{"sd_dgl_result_count", &dglCountFunc, &ffi.TypeUint64, []*ffi.Type{&ffi.TypePointer}},
		{"sd_dgl_result_name", &dglNameFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"sd_dgl_result_tensor", &dglTensorFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer, &ffi.TypePointer}},
		{"sd_get_generation_metadata", &generationMetadataFunc, &ffi.TypePointer, []*ffi.Type{&ffi.TypePointer}},
	} {
		fn, err := lib.Prep(s.name, s.result, s.args...)
		if err != nil {
			return loadError(s.name, err)
		}
		*s.dst = fn
	}
	return nil
}

// GenerationMetadata returns Go-owned JSON inferred from executed DGL source.
// It does not execute source or read files. Unrecognized flows return {}.
// The inferred JSON is best-effort; GenerationInfo retains the exact program.
func GenerationMetadata(source string) (string, error) {
	var refs cStringRefs
	p, err := refs.addPointer(source)
	if err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if generationMetadataFunc == (ffi.Fun{}) {
		return "", unsupported("sd_get_generation_metadata")
	}
	var out *byte
	generationMetadataFunc.Call(unsafe.Pointer(&out), unsafe.Pointer(&p))
	runtime.KeepAlive(refs)
	if out == nil {
		return "", fmt.Errorf("generation metadata: %s", nativeDiagnostic())
	}
	return cString(out), nil
}

// RunDGL executes a trusted program using ctx's component cache and execution
// settings. Do not run it concurrently with generation or other context calls.
// Inputs bind RGB images to named main parameters. All flattened output tensors
// (including video frames and scalar values) are copied before native result
// release. Structured/opaque/bytes values are not returned by this flat API.
// Programs can read and write files with the process's permissions; this is not
// a sandbox. Use explicit DGL for ADetailer, AnimateDiff and custom preprocessing.
func RunDGL(ctx Context, source string, inputs map[string]SDImage) (map[string]DGLTensor, error) {
	if ctx == 0 {
		return nil, errors.New("RunDGL: nil context")
	}
	var refs cStringRefs
	p, err := refs.addPointer(source)
	if err != nil {
		return nil, err
	}
	bindings := make([]cDGLInput, 0, len(inputs))
	defer func() {
		for _, b := range bindings {
			freeTensor(b.Tensor)
		}
	}()
	for _, name := range slices.Sorted(maps.Keys(inputs)) {
		n, err := refs.add(name)
		if err != nil {
			return nil, err
		}
		t, err := imageTensor(inputs[name])
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, cDGLInput{Name: n, Tensor: t})
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if ctxDGLFunc == (ffi.Fun{}) {
		return nil, unsupported("sd_ctx_get_dgl_ctx")
	}
	var dgl uintptr
	ctxDGLFunc.Call(unsafe.Pointer(&dgl), unsafe.Pointer(&ctx))
	var result uintptr
	out, data, count := &result, unsafe.SliceData(bindings), uint64(len(bindings))
	err = statusCall(dglRunFunc, unsafe.Pointer(&dgl), unsafe.Pointer(&p), unsafe.Pointer(&data), unsafe.Pointer(&count), unsafe.Pointer(&out))
	runtime.KeepAlive(refs)
	runtime.KeepAlive(bindings)
	defer dglFreeFunc.Call(nil, unsafe.Pointer(&result))
	if err != nil {
		return nil, err
	}
	var total ffi.Arg
	dglCountFunc.Call(&total, unsafe.Pointer(&result))
	outputs := make(map[string]DGLTensor, int(total))
	for i := range uint64(total) {
		var name *byte
		dglNameFunc.Call(unsafe.Pointer(&name), unsafe.Pointer(&result), unsafe.Pointer(&i))
		if name == nil {
			return nil, fmt.Errorf("DGL output name: %s", nativeDiagnostic())
		}
		key := cString(name)
		var tensor uintptr
		dglTensorFunc.Call(unsafe.Pointer(&tensor), unsafe.Pointer(&result), unsafe.Pointer(&name))
		if tensor == 0 {
			return nil, fmt.Errorf("DGL output tensor: %s", nativeDiagnostic())
		}
		var kind, rank ffi.Arg
		tensorTypeFunc.Call(&kind, unsafe.Pointer(&tensor))
		tensorRankFunc.Call(&rank, unsafe.Pointer(&tensor))
		shape := make([]int64, int(rank))
		for axis := range shape {
			index := uint64(axis)
			tensorDimFunc.Call(unsafe.Pointer(&shape[axis]), unsafe.Pointer(&tensor), unsafe.Pointer(&index))
		}
		bytes, err := tensorBytes(tensor)
		if err != nil {
			return nil, err
		}
		outputs[key] = DGLTensor{Type: int32(kind), Shape: shape, Data: bytes}
	}
	return outputs, nil
}
