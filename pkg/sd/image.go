package sd

import (
	"fmt"
	imgpkg "image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoadImage decodes a PNG or JPEG file from disk into an SDImage with 3
// channels (RGB). The decoder is selected from the filename extension
// (.png / .jpg / .jpeg, case-insensitive). The alpha channel of the
// source image, if any, is discarded.
func LoadImage(filename string) (SDImage, error) {
	switch ext := strings.ToLower(filepath.Ext(filename)); ext {
	case ".png":
		return LoadPNG(filename)
	case ".jpg", ".jpeg":
		return LoadJPEG(filename)
	default:
		return SDImage{}, fmt.Errorf("LoadImage: unsupported extension %q (want .png, .jpg, .jpeg)", ext)
	}
}

// LoadPNG decodes a PNG file from disk into an SDImage with 3 channels (RGB).
// The alpha channel of the source PNG, if any, is discarded.
func LoadPNG(filename string) (SDImage, error) {
	return loadImageWith(filename, "LoadPNG", png.Decode)
}

// LoadJPEG decodes a JPEG file from disk into an SDImage with 3 channels (RGB).
func LoadJPEG(filename string) (SDImage, error) {
	return loadImageWith(filename, "LoadJPEG", jpeg.Decode)
}

// loadImageWith opens filename, runs decode on it, and converts the
// resulting image.Image to a 3-channel RGB SDImage. fnName labels error
// messages so callers see "LoadPNG: ..." vs "LoadJPEG: ..." consistently.
func loadImageWith(filename string, fnName string, decode func(io.Reader) (imgpkg.Image, error)) (SDImage, error) {
	f, err := os.Open(filename)
	if err != nil {
		return SDImage{}, fmt.Errorf("%s: %w", fnName, err)
	}
	defer f.Close()

	img, err := decode(f)
	if err != nil {
		return SDImage{}, fmt.Errorf("%s: decode %s: %w", fnName, filename, err)
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

	return out, nil
}

// SDImage is a Go-side representation of a generated image. Data holds raw
// pixel bytes in row-major order with Channel bytes per pixel (typically 3
// for RGB output from stable-diffusion.cpp).
//
// Generated pixels are copied out of the C heap and remain valid after
// FreeContext. Copying an SDImage value shares its pixel backing array.
// A zero SDImage represents an absent optional image input.
type SDImage struct {
	Width   uint32
	Height  uint32
	Channel uint32
	Data    []byte
	// GenerationInfo is the Go-owned DGL program executed by the native
	// result, not JSON. GenerationMetadata infers image parameters from it.
	// Loading or saving PNG/JPEG pixels does not restore or embed this record.
	GenerationInfo string
}

// SavePNG writes the image to filename as a PNG. Grayscale (1-channel) and
// RGB (3-channel) inputs are encoded as 8-bit RGBA.
func (img SDImage) SavePNG(filename string) error {
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
