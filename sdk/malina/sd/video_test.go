// Video tests cover the pure-Go Motion-JPEG encoder.
package sd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// makeGradient builds an SDImage filled with a deterministic RGB gradient so
// tests don't depend on the stable-diffusion library being loaded.
func makeGradient(w, h int, base byte) *SDImage {
	out := SDImage{
		Width:   uint32(w),
		Height:  uint32(h),
		Channel: 3,
		Data:    make([]byte, w*h*3),
	}
	for y := range h {
		for x := range w {
			i := (y*w + x) * 3
			out.Data[i+0] = byte(x) + base
			out.Data[i+1] = byte(y) + base
			out.Data[i+2] = base
		}
	}
	return &out
}

func TestSaveAVI(t *testing.T) {
	const (
		width  = 64
		height = 64
		frames = 8
		fps    = 8
	)

	imgs := make([]*SDImage, frames)
	for i := range imgs {
		imgs[i] = makeGradient(width, height, byte(i*16))
	}

	path := filepath.Join(t.TempDir(), "out.avi")
	if err := SaveAVI(path, imgs, fps, 90); err != nil {
		t.Fatalf("SaveAVI: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// AVI files begin with "RIFF<size>AVI ". This catches misuse of the
	// underlying writer (wrong header bytes, truncated output).
	if len(data) < 12 {
		t.Fatalf("file too small: %d bytes", len(data))
	}
	if got, want := string(data[0:4]), "RIFF"; got != want {
		t.Errorf("magic[0:4]: got %q, want %q", got, want)
	}
	if got, want := string(data[8:12]), "AVI "; got != want {
		t.Errorf("magic[8:12]: got %q, want %q", got, want)
	}
}

func TestSaveAVIErrors(t *testing.T) {
	tmp := t.TempDir()
	good := []*SDImage{makeGradient(8, 8, 0)}

	cases := []struct {
		name    string
		frames  []*SDImage
		fps     int
		quality int
		wantErr bool
	}{
		{"no frames", nil, 8, 90, true},
		{"zero fps", good, 0, 90, true},
		{"quality 0", good, 8, 0, true},
		{"quality 101", good, 8, 101, true},
		{"mismatched size", []*SDImage{makeGradient(8, 8, 0), makeGradient(16, 8, 0)}, 8, 90, true},
		{"ok", good, 8, 90, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(tmp, c.name+".avi")
			err := SaveAVI(path, c.frames, c.fps, c.quality)
			if (err != nil) != c.wantErr {
				t.Errorf("SaveAVI: got err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestLoadPNGRoundTrip(t *testing.T) {
	src := makeGradient(32, 32, 50)

	pngPath := filepath.Join(t.TempDir(), "in.png")
	if err := src.SavePNG(pngPath); err != nil {
		t.Fatalf("SavePNG: %v", err)
	}

	got, err := LoadPNG(pngPath)
	if err != nil {
		t.Fatalf("LoadPNG: %v", err)
	}

	if got.Width != src.Width || got.Height != src.Height || got.Channel != src.Channel {
		t.Fatalf("dims: got %dx%dx%d, want %dx%dx%d",
			got.Width, got.Height, got.Channel,
			src.Width, src.Height, src.Channel)
	}
	if !bytes.Equal(got.Data, src.Data) {
		t.Errorf("Data: pixel mismatch after PNG round-trip")
	}
}
