package sd

import (
	"fmt"
	"maps"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"unicode"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

// Options explicitly sets native keys after compatibility fields. Values accept
// bool, signed integers, float32/64, string, []float32, []int32, []Lora,
// []Embedding, SDImage, []SDImage, []Audio and []RefVideo. Nil or a zero SDImage
// unsets a key. Zero-valued optional image fields are omitted.
// Keys, types and applicability are validated by the native API, not ignored.
type Options map[string]any

// Status identifies a native operation outcome.
type Status int32

const (
	StatusOK Status = iota
	StatusInvalidArgument
	StatusUnknownKey
	StatusTypeMismatch
	StatusOutOfRange
	StatusUnsupported
	StatusOutOfMemory
	StatusLoadFailed
	StatusGenerationFailed
	StatusBusy
	StatusCancelled
)

// NativeError carries the native status and thread-local diagnostic.
type NativeError struct {
	Status  Status
	Message string
}

func (e *NativeError) Error() string {
	return fmt.Sprintf("stable-diffusion status %d: %s", e.Status, e.Message)
}

type nativeOptions struct{ ptr uintptr }

var optionConstructors map[string]ffi.Fun
var optionSetters map[string]ffi.Fun
var optionFreeFunc, lastErrorFunc ffi.Fun

func loadOptionFuncs(lib ffi.Lib) error {
	optionConstructors = make(map[string]ffi.Fun)
	optionSetters = make(map[string]ffi.Fun)
	for _, purpose := range []string{"ctx", "img_gen", "vid_gen", "upscaler", "upscale"} {
		name := "sd_" + purpose + "_opt_create"
		fn, err := lib.Prep(name, &ffi.TypePointer)
		if err != nil {
			return loadError(name, err)
		}
		optionConstructors[purpose] = fn
	}
	for _, item := range []struct {
		name string
		args []*ffi.Type
	}{
		{"bool", []*ffi.Type{&ffi.TypeUint8}}, {"int", []*ffi.Type{&ffi.TypeSint64}}, {"float", []*ffi.Type{&ffi.TypeDouble}},
		{"string", []*ffi.Type{&ffi.TypePointer}}, {"tensor", []*ffi.Type{&ffi.TypePointer}},
		{"tensors", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}}, {"floats", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"ints", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}}, {"loras", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"embeddings", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}}, {"audios", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}},
		{"videos", []*ffi.Type{&ffi.TypePointer, &ffi.TypeUint64}}, {"unset", nil},
	} {
		name := "sd_opt_set_" + item.name
		if item.name == "unset" {
			name = "sd_opt_unset"
		}
		args := append([]*ffi.Type{&ffi.TypePointer, &ffi.TypePointer}, item.args...)
		fn, err := lib.Prep(name, &ffi.TypeSint32, args...)
		if err != nil {
			return loadError(name, err)
		}
		optionSetters[item.name] = fn
	}
	var err error
	if optionFreeFunc, err = lib.Prep("sd_opt_free", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return err
	}
	if lastErrorFunc, err = lib.Prep("sd_get_last_error", &ffi.TypePointer); err != nil {
		return err
	}
	return loadTensorFuncs(lib)
}

func nativeDiagnostic() string {
	var p *byte
	lastErrorFunc.Call(unsafe.Pointer(&p))
	return cString(p)
}

// statusCall pins the failing call and diagnostic read to the same OS thread.
func statusCall(fn ffi.Fun, args ...any) error {
	if fn == (ffi.Fun{}) {
		return unsupported("native operation")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var status ffi.Arg
	fn.Call(&status, args...)
	if Status(int32(status)) != StatusOK {
		return &NativeError{Status: Status(int32(status)), Message: nativeDiagnostic()}
	}
	return nil
}

func newOptions(purpose string) (*nativeOptions, error) {
	fn := optionConstructors[purpose]
	if fn == (ffi.Fun{}) {
		return nil, unsupported("sd_" + purpose + "_opt_create")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	o := nativeOptions{}
	fn.Call(unsafe.Pointer(&o.ptr))
	if o.ptr == 0 {
		return nil, &NativeError{Status: StatusOutOfMemory, Message: nativeDiagnostic()}
	}
	return &o, nil
}

func (o *nativeOptions) free() { optionFreeFunc.Call(nil, unsafe.Pointer(&o.ptr)) }

func (o *nativeOptions) apply(values Options) error {
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if err := o.set(key, values[key]); err != nil {
			return fmt.Errorf("setting %s: %w", key, err)
		}
	}
	return nil
}

func (o *nativeOptions) set(key string, value any) error {
	k, err := utils.BytePtrFromString(key)
	if err != nil {
		return err
	}
	args := []any{unsafe.Pointer(&o.ptr), unsafe.Pointer(&k)}
	kind := ""
	var refs cStringRefs
	var tensors []uintptr
	defer func() {
		for _, t := range tensors {
			freeTensor(t)
		}
	}()
	switch v := value.(type) {
	case nil:
		kind = "unset"
	case bool:
		kind = "bool"
		n := boolToU8(v)
		args = append(args, unsafe.Pointer(&n))
	case string:
		kind = "string"
		p, err := utils.BytePtrFromString(v)
		if err != nil {
			return err
		}
		refs.keep = append(refs.keep, p)
		args = append(args, unsafe.Pointer(&p))
	case float32:
		kind = "float"
		n := float64(v)
		args = append(args, unsafe.Pointer(&n))
	case float64:
		kind = "float"
		args = append(args, unsafe.Pointer(&v))
	case SDImage:
		if reflect.ValueOf(v).IsZero() {
			kind = "unset"
			break
		}
		t, err := imageTensor(v)
		if err != nil {
			return err
		}
		tensors = append(tensors, t)
		kind = "tensor"
		args = append(args, unsafe.Pointer(&t))
	case []SDImage:
		if key == "control_frames" || key == "frames" {
			t, err := framesTensor(v)
			if err != nil {
				return err
			}
			tensors = append(tensors, t)
			kind = "tensor"
			args = append(args, unsafe.Pointer(&t))
		} else {
			for _, image := range v {
				t, err := imageTensor(image)
				if err != nil {
					return err
				}
				tensors = append(tensors, t)
			}
			kind = "tensors"
			args = appendArray(args, tensors)
		}
	case []float32:
		kind = "floats"
		args = appendArray(args, v)
	case []int32:
		kind = "ints"
		args = appendArray(args, v)
	case []Lora:
		items := make([]cLora, len(v))
		for i, l := range v {
			p, err := refs.add(l.Path)
			if err != nil {
				return err
			}
			items[i] = cLora{IsHighNoise: boolToU8(l.IsHighNoise), Multiplier: l.Multiplier, Path: p}
		}
		kind = "loras"
		args = appendArray(args, items)
	case []Embedding:
		items := make([]cEmbedding, len(v))
		for i, e := range v {
			n, err := refs.add(e.Name)
			if err != nil {
				return err
			}
			p, err := refs.add(e.Path)
			if err != nil {
				return err
			}
			items[i] = cEmbedding{Name: n, Path: p}
		}
		kind = "embeddings"
		args = appendArray(args, items)
	case []Audio:
		items := make([]cAudio, len(v))
		for i := range v {
			a, err := audioToC(v[i], key)
			if err != nil {
				return err
			}
			items[i] = a
			tensors = append(tensors, a.Data)
		}
		kind = "audios"
		args = appendArray(args, items)
	case []RefVideo:
		items := make([]cVideo, len(v))
		for i, video := range v {
			t, err := framesTensor(video.Frames)
			if err != nil {
				return err
			}
			tensors = append(tensors, t)
			items[i] = cVideo{Data: t, FPS: video.FPS}
			if !reflect.ValueOf(video.Audio).IsZero() {
				a, err := audioToC(video.Audio, key)
				if err != nil {
					return err
				}
				tensors = append(tensors, a.Data)
				items[i].Audio = a
			}
		}
		kind = "videos"
		args = appendArray(args, items)
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Int64 {
			kind = "int"
			n := rv.Int()
			args = append(args, unsafe.Pointer(&n))
		} else {
			return fmt.Errorf("option %s: unsupported Go value %T", key, value)
		}
	}
	err = statusCall(optionSetters[kind], args...)
	runtime.KeepAlive(value)
	runtime.KeepAlive(refs)
	runtime.KeepAlive(k)
	return err
}

func appendArray[T any](args []any, values []T) []any {
	p := unsafe.SliceData(values)
	n := uint64(len(values))
	return append(args, unsafe.Pointer(&p), unsafe.Pointer(&n))
}

func (o *nativeOptions) fields(value, defaults any, prefix string) error {
	v, d := reflect.ValueOf(value), reflect.ValueOf(defaults)
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Name == "Options" {
			continue
		}
		x, def := v.Field(i), d.Field(i)
		if reflect.DeepEqual(x.Interface(), def.Interface()) {
			continue
		}
		key := f.Tag.Get("sd")
		if key == "" {
			key = optionKey(f.Name)
		}
		if prefix != "" {
			key = prefix + "." + key
		}
		if x.Kind() == reflect.Struct && x.Type() != reflect.TypeFor[SDImage]() {
			if err := o.fields(x.Interface(), def.Interface(), key); err != nil {
				return err
			}
			continue
		}
		val := x.Interface()
		if text, ok, err := enumOption(val); ok {
			if err != nil {
				return err
			}
			val = text
		}
		if err := o.set(key, val); err != nil {
			return fmt.Errorf("setting %s: %w", key, err)
		}
	}
	if f := v.FieldByName("Options"); f.IsValid() {
		return o.apply(f.Interface().(Options))
	}
	return nil
}

// optionKey handles Go initialisms and the few upstream names that differ.
func optionKey(name string) string {
	switch name {
	case "ForceSDXLVAEConvScale":
		return "force_sdxl_vae_conv_scale"
	case "T5XXLPath":
		return "t5xxl_path"
	case "TAESDPath":
		return "taesd_path"
	case "TaePreviewOnly":
		return "tae_preview_only"
	case "TextCFG":
		return "txt_cfg"
	case "ImageCFG":
		return "img_cfg"
	case "CustomSigmas":
		return "sigmas"
	case "ExtraArgs":
		return "args"
	case "TileSizeX":
		return "tile_size_w"
	case "TileSizeY":
		return "tile_size_h"
	case "RelativeSizeX":
		return "rel_size_w"
	case "RelativeSizeY":
		return "rel_size_h"
	case "SpectrumLambda":
		return "spectrum_lam"
	}
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1]))) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func cString(p *byte) string {
	if p == nil {
		return ""
	}
	return utils.BytePtrToString(p)
}
