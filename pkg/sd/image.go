package sd

import (
	"fmt"
	imgpkg "image"
	"image/png"
	"os"
	"unsafe"
)

// LoadPNG decodes a PNG file from disk into an SDImage with 3 channels (RGB).
// The alpha channel of the source PNG, if any, is discarded.
func LoadPNG(filename string) (*SDImage, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("LoadPNG: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("LoadPNG: decode %s: %w", filename, err)
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	out := SDImage{
		Width:   uint32(w),
		Height:  uint32(h),
		Channel: 3,
		Data:    make([]byte, w*h*3),
	}

	for y := range h {
		for x := range w {
			r, g, blue, _ := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			i := (y*w + x) * 3
			out.Data[i+0] = byte(r >> 8)
			out.Data[i+1] = byte(g >> 8)
			out.Data[i+2] = byte(blue >> 8)
		}
	}

	return &out, nil
}

// cImage mirrors struct sd_image_t.
//
// Data is held as *byte (rather than uintptr) so the unsafe.Slice in
// sdImageFromC does not trigger a "possible misuse of unsafe.Pointer" vet
// warning. The Go GC does not track the C-heap memory it points at; we copy
// out into a Go slice before returning to the caller.
//
// Total size: 24 bytes on darwin/arm64 (12 bytes of fields + 4 pad + 8-byte
// pointer).
type cImage struct {
	Width   uint32 // 0..4
	Height  uint32 // 4..8
	Channel uint32 // 8..12
	_       [4]byte
	Data    *byte // 16..24  (uint8 *)
}

// SDImage is a Go-side representation of a generated image. Data holds raw
// pixel bytes in row-major order with Channel bytes per pixel (typically 3
// for RGB output from stable-diffusion.cpp).
//
// SDImage owns its Data slice (it is copied out of the C heap by
// GenerateImage), so callers may use it after FreeContext / FreeImage are
// called.
type SDImage struct {
	Width   uint32
	Height  uint32
	Channel uint32
	Data    []byte
}

// SavePNG writes the image to filename as a PNG. Grayscale (1-channel) and
// RGB (3-channel) inputs are encoded as 8-bit RGBA.
func (img *SDImage) SavePNG(filename string) error {
	rect := imgpkg.Rect(0, 0, int(img.Width), int(img.Height))
	rgba := imgpkg.NewRGBA(rect)

	channels := int(img.Channel)
	src := img.Data
	dst := rgba.Pix

	switch channels {
	case 3:
		for i, j := 0, 0; i < len(src); i, j = i+3, j+4 {
			dst[j+0] = src[i+0]
			dst[j+1] = src[i+1]
			dst[j+2] = src[i+2]
			dst[j+3] = 255
		}
	case 4:
		copy(dst, src)
	case 1:
		for i, j := 0, 0; i < len(src); i, j = i+1, j+4 {
			v := src[i]
			dst[j+0] = v
			dst[j+1] = v
			dst[j+2] = v
			dst[j+3] = 255
		}
	default:
		return fmt.Errorf("SavePNG: unsupported channel count %d", channels)
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("SavePNG: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, rgba); err != nil {
		return fmt.Errorf("SavePNG: %w", err)
	}
	return nil
}

// bindCImage populates dst so the C library can read src's pixel buffer
// in place. The caller MUST keep src reachable across the FFI call (via
// runtime.KeepAlive) so the Go GC does not collect the backing array.
// field names the destination so validation errors are self-describing.
func bindCImage(dst *cImage, src *SDImage, field string) error {
	if src.Width == 0 || src.Height == 0 {
		return fmt.Errorf("%s: zero dimension (%dx%d)", field, src.Width, src.Height)
	}
	if src.Channel != 3 {
		return fmt.Errorf("%s: channel count %d, want 3 (RGB)", field, src.Channel)
	}
	want := int(src.Width) * int(src.Height) * int(src.Channel)
	if len(src.Data) != want {
		return fmt.Errorf("%s: data length %d, want %d for %dx%dx%d", field, len(src.Data), want, src.Width, src.Height, src.Channel)
	}

	dst.Width = src.Width
	dst.Height = src.Height
	dst.Channel = src.Channel
	dst.Data = &src.Data[0]
	return nil
}

// sdImageFromC copies the raw pixel buffer out of the C heap into a Go slice
// so it survives independently of the C-side allocation.
func sdImageFromC(c *cImage) *SDImage {
	if c == nil {
		return nil
	}
	size := int(c.Width) * int(c.Height) * int(c.Channel)
	out := &SDImage{
		Width:   c.Width,
		Height:  c.Height,
		Channel: c.Channel,
	}
	if size == 0 || c.Data == nil {
		return out
	}
	src := unsafe.Slice(c.Data, size)
	out.Data = make([]byte, size)
	copy(out.Data, src)
	return out
}
