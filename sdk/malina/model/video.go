package model

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/icza/mjpeg"
)

// SaveAVI writes standard Go images as a Motion-JPEG AVI file.
func SaveAVI(filename string, frames []image.Image, fps, quality int) (err error) {
	if len(frames) == 0 {
		return errors.New("save AVI: no frames")
	}
	if fps <= 0 {
		return errors.New("save AVI: fps must be positive")
	}
	if quality < 1 || quality > 100 {
		return errors.New("save AVI: quality must be between 1 and 100")
	}
	if frames[0] == nil || frames[0].Bounds().Dx() <= 0 || frames[0].Bounds().Dy() <= 0 {
		return errors.New("save AVI: frame 0 has invalid dimensions")
	}

	width, height := frames[0].Bounds().Dx(), frames[0].Bounds().Dy()
	writer, err := mjpeg.New(filename, int32(width), int32(height), int32(fps))
	if err != nil {
		return fmt.Errorf("save AVI: creating writer: %w", err)
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("save AVI: closing writer: %w", closeErr))
		}
	}()

	for i, frame := range frames {
		if frame == nil || frame.Bounds().Dx() != width || frame.Bounds().Dy() != height {
			return fmt.Errorf("save AVI: frame %d dimensions do not match %dx%d", i, width, height)
		}
		var encoded bytes.Buffer
		if encodeErr := jpeg.Encode(&encoded, frame, &jpeg.Options{Quality: quality}); encodeErr != nil {
			return fmt.Errorf("save AVI: encoding frame %d: %w", i, encodeErr)
		}
		if addErr := writer.AddFrame(encoded.Bytes()); addErr != nil {
			return fmt.Errorf("save AVI: adding frame %d: %w", i, addErr)
		}
	}

	return nil
}
