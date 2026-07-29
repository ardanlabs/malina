// Image tests cover native image conversion and pure-Go codecs.
package sd

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBindCImageValidation locks down the input validation bindCImage
// performs before handing pixel pointers off to stable-diffusion.cpp.
// These checks run entirely in Go and need neither the shared library
// nor a model, so they always execute as part of `go test ./sdk/malina/sd/...`.
func TestBindCImageValidation(t *testing.T) {
	cases := []struct {
		name    string
		img     SDImage
		wantSub string
	}{
		{
			name:    "zero width",
			img:     SDImage{Width: 0, Height: 8, Channel: 3, Data: make([]byte, 0)},
			wantSub: "zero dimension",
		},
		{
			name:    "zero height",
			img:     SDImage{Width: 8, Height: 0, Channel: 3, Data: make([]byte, 0)},
			wantSub: "zero dimension",
		},
		{
			name:    "wrong channel count",
			img:     SDImage{Width: 8, Height: 8, Channel: 4, Data: make([]byte, 8*8*4)},
			wantSub: "channel count 4",
		},
		{
			name:    "short data buffer",
			img:     SDImage{Width: 8, Height: 8, Channel: 3, Data: make([]byte, 10)},
			wantSub: "data length 10",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var dst cImage
			err := bindCImage(&dst, &c.img, "InitImage")
			if err == nil {
				t.Fatalf("bindCImage: expected error containing %q, got nil", c.wantSub)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("bindCImage error: got %q, want substring %q", err.Error(), c.wantSub)
			}
		})
	}
}

// TestLoadImageDispatch verifies LoadImage routes by extension: a PNG
// fixture round-trips through LoadPNG, a JPEG fixture round-trips
// through LoadJPEG, and an unknown extension returns a clear error.
// Encoding JPEG inline (rather than committing a sample to disk) keeps
// the test self-contained and avoids dragging binary fixtures into the
// repo.
func TestLoadImageDispatch(t *testing.T) {
	dir := t.TempDir()

	// PNG fixture: synthesize an SDImage and round-trip through SavePNG.
	src := SDImage{
		Width: 4, Height: 4, Channel: 3,
		Data: make([]byte, 4*4*3),
	}
	for i := range src.Data {
		src.Data[i] = byte(i * 7)
	}
	pngPath := filepath.Join(dir, "fixture.png")
	if err := src.SavePNG(pngPath); err != nil {
		t.Fatalf("SavePNG: %v", err)
	}

	// JPEG fixture: encode the same grid via image/jpeg directly.
	jpegPath := filepath.Join(dir, "fixture.jpg")
	rgba := image.NewRGBA(image.Rect(0, 0, int(src.Width), int(src.Height)))
	for i, j := 0, 0; i < len(src.Data); i, j = i+3, j+4 {
		rgba.Pix[j+0] = src.Data[i+0]
		rgba.Pix[j+1] = src.Data[i+1]
		rgba.Pix[j+2] = src.Data[i+2]
		rgba.Pix[j+3] = 255
	}
	f, err := os.Create(jpegPath)
	if err != nil {
		t.Fatalf("Create jpeg: %v", err)
	}
	if err := jpeg.Encode(f, rgba, &jpeg.Options{Quality: 95}); err != nil {
		f.Close()
		t.Fatalf("jpeg.Encode: %v", err)
	}
	f.Close()

	cases := []struct {
		name string
		path string
	}{
		{"png", pngPath},
		{"jpg", jpegPath},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := LoadImage(c.path)
			if err != nil {
				t.Fatalf("LoadImage(%s): %v", c.path, err)
			}
			if got.Width != src.Width || got.Height != src.Height || got.Channel != src.Channel {
				t.Errorf("dims: got %dx%dx%d, want %dx%dx%d",
					got.Width, got.Height, got.Channel,
					src.Width, src.Height, src.Channel)
			}
		})
	}

	t.Run("unknown extension", func(t *testing.T) {
		_, err := LoadImage(filepath.Join(dir, "fixture.bmp"))
		if err == nil {
			t.Fatal("LoadImage(.bmp): expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported extension") {
			t.Errorf("error: got %q, want substring %q", err.Error(), "unsupported extension")
		}
	})
}

// TestBindCImageOK verifies the happy path: dimensions are copied
// through, Data points at the first byte of the source slice, and the
// destination cImage is otherwise zero-initialized.
func TestBindCImageOK(t *testing.T) {
	src := SDImage{
		Width:   16,
		Height:  8,
		Channel: 3,
		Data:    make([]byte, 16*8*3),
	}
	src.Data[0] = 0x42

	var dst cImage
	if err := bindCImage(&dst, &src, "InitImage"); err != nil {
		t.Fatalf("bindCImage: %v", err)
	}

	if dst.Width != src.Width || dst.Height != src.Height || dst.Channel != src.Channel {
		t.Errorf("dims: got %dx%dx%d, want %dx%dx%d", dst.Width, dst.Height, dst.Channel, src.Width, src.Height, src.Channel)
	}
	if dst.Data == nil {
		t.Fatal("dst.Data is nil; want pointer to src.Data[0]")
	}
	if *dst.Data != 0x42 {
		t.Errorf("dst.Data points at %#x, want %#x", *dst.Data, 0x42)
	}
}
