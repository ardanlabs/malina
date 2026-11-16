package sd

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

func TestPromptImageCallbackFFIRoundTrip(t *testing.T) {
	testSetup(t)
	// Intercept only native registration (a real context requires a model).
	// Invoke the registered Go closure using the exact native callback CIF;
	// all image tensors and allocators below are from the target library.
	var registration ffi.Cif
	if status := ffi.PrepCif(&registration, ffi.DefaultAbi, 3, &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer); status != ffi.OK {
		t.Fatal(status)
	}
	type installed struct{ code, data uintptr }
	registrations := make(map[Context]installed)
	closure, code, callback, err := allocateCallbackClosure("test prompt registration", &registration, func(_ *ffi.Cif, _ unsafe.Pointer, args *unsafe.Pointer, _ unsafe.Pointer) uintptr {
		a := (*[3]unsafe.Pointer)(unsafe.Pointer(args))
		ctx := Context(*(*uintptr)(a[0]))
		registrations[ctx] = installed{code: *(*uintptr)(a[1]), data: *(*uintptr)(a[2])}
		return 0
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ffi.ClosureFree(closure)
	previous := setPromptImageFunc
	setPromptImageFunc = ffi.Fun{Addr: uintptr(code), Cif: &registration}
	defer func() { setPromptImageFunc = previous }()
	image := makeGradient(7, 3, 12)
	if err := SetPromptImageCallback(101, func(source string) *SDImage {
		if source != "first" {
			return nil
		}
		return image
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetPromptImageCallback(202, func(string) *SDImage { return testMarshalImage(91) }); err != nil {
		t.Fatal(err)
	}
	defer clearPromptImage(101)
	defer clearPromptImage(202)
	resolve := func(ctx Context, source string) (uintptr, bool) {
		t.Helper()
		text, err := utils.BytePtrFromString(source)
		if err != nil {
			t.Fatal(err)
		}
		reg := registrations[ctx]
		fn := ffi.Fun{Addr: reg.code, Cif: &promptImageCif}
		var result uintptr
		out := &result
		var ok ffi.Arg
		fn.Call(&ok, unsafe.Pointer(&text), unsafe.Pointer(&reg.data), unsafe.Pointer(&out))
		return result, byte(ok) != 0
	}
	tensor, ok := resolve(101, "first")
	if !ok || tensor == 0 {
		t.Fatal("first callback did not supply a tensor")
	}
	image.Data[0] = 255
	got, err := imageFromTensor(tensor)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 7 || got.Height != 3 || got.Data[0] != 12 {
		t.Fatalf("snapshot: got %#v", got)
	}
	second, ok := resolve(202, "second")
	if !ok {
		t.Fatal("second callback unresolved")
	}
	other, err := imageFromTensor(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(other.Data, []byte{91, 92, 93}) {
		t.Fatalf("context-local callback: got %v", other.Data)
	}
	if tensor, ok := resolve(101, "missing"); ok || tensor != 0 {
		t.Fatal("missing source reported success")
	}
	if err := SetPromptImageCallback(101, nil); err != nil {
		t.Fatal(err)
	}
	if registrations[101].code != 0 {
		t.Fatal("nil callback did not clear native registration")
	}
	if err := SetPromptImageCallback(0, nil); err == nil {
		t.Fatal("nil context accepted")
	}
	_ = callback // The closure registry retains this callback for the test lifetime.
}
