package model

import (
	"image"
	"testing"
)

func TestSaveAVI(t *testing.T) {
	frames := []image.Image{image.NewRGBA(image.Rect(0, 0, 4, 4)), image.NewGray(image.Rect(2, 3, 6, 7))}
	path := t.TempDir() + "/test.avi"
	if err := SaveAVI(path, frames, 24, 90); err != nil {
		t.Fatalf("SaveAVI: %v", err)
	}
}

func TestSaveAVIErrors(t *testing.T) {
	valid := image.NewRGBA(image.Rect(0, 0, 4, 4))
	tests := []struct {
		name    string
		frames  []image.Image
		fps     int
		quality int
	}{
		{name: "no frames", fps: 1, quality: 90},
		{name: "fps", frames: []image.Image{valid}, quality: 90},
		{name: "quality", frames: []image.Image{valid}, fps: 1},
		{name: "nil", frames: []image.Image{nil}, fps: 1, quality: 90},
		{name: "mismatch", frames: []image.Image{valid, image.NewRGBA(image.Rect(0, 0, 2, 2))}, fps: 1, quality: 90},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SaveAVI(t.TempDir()+"/test.avi", tt.frames, tt.fps, tt.quality); err == nil {
				t.Fatal("SaveAVI: got nil error, want error")
			}
		})
	}
}
