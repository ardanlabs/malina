package sd

import (
	"bytes"
	"errors"
	"fmt"
	imgpkg "image"
	"image/jpeg"

	"github.com/icza/mjpeg"
)

// SaveAVI writes frames as a Motion-JPEG track inside a single-track AVI
// container at the given fps. Every frame must share the same Width, Height
// and Channel count as frames[0]. quality is the JPEG encoder quality (1-100);
// 90 is a good default.
//
// The produced .avi plays in QuickTime, VLC, mpv and Windows Media Player.
// MJPEG is intra-frame only (every frame is a standalone JPEG), so files are
// larger than equivalent H.264 output but the muxer stays pure-Go.
func SaveAVI(filename string, frames []*SDImage, fps, quality int) error {
	if len(frames) == 0 {
		return errors.New("SaveAVI: no frames")
	}
	if fps <= 0 {
		return fmt.Errorf("SaveAVI: fps must be > 0, got %d", fps)
	}
	if quality < 1 || quality > 100 {
		return fmt.Errorf("SaveAVI: quality must be 1..100, got %d", quality)
	}

	width := int32(frames[0].Width)
	height := int32(frames[0].Height)

	aw, err := mjpeg.New(filename, width, height, int32(fps))
	if err != nil {
		return fmt.Errorf("SaveAVI: create writer: %w", err)
	}

	for i, f := range frames {
		if int32(f.Width) != width || int32(f.Height) != height {
			_ = aw.Close()
			return fmt.Errorf("SaveAVI: frame %d size %dx%d does not match frame 0 size %dx%d",
				i, f.Width, f.Height, width, height)
		}

		jpegBytes, err := f.encodeJPEG(quality)
		if err != nil {
			_ = aw.Close()
			return fmt.Errorf("SaveAVI: frame %d: %w", i, err)
		}

		if err := aw.AddFrame(jpegBytes); err != nil {
			_ = aw.Close()
			return fmt.Errorf("SaveAVI: add frame %d: %w", i, err)
		}
	}

	if err := aw.Close(); err != nil {
		return fmt.Errorf("SaveAVI: close writer: %w", err)
	}

	return nil
}

// encodeJPEG converts the SDImage's raw pixel buffer into a JPEG byte slice
// at the requested quality. Supports 1-channel (gray), 3-channel (RGB) and
// 4-channel (RGBA, alpha discarded) inputs.
func (img *SDImage) encodeJPEG(quality int) ([]byte, error) {
	rect := imgpkg.Rect(0, 0, int(img.Width), int(img.Height))

	var src imgpkg.Image
	switch img.Channel {
	case 3:
		rgba := imgpkg.NewRGBA(rect)
		for i, j := 0, 0; i < len(img.Data); i, j = i+3, j+4 {
			rgba.Pix[j+0] = img.Data[i+0]
			rgba.Pix[j+1] = img.Data[i+1]
			rgba.Pix[j+2] = img.Data[i+2]
			rgba.Pix[j+3] = 255
		}
		src = rgba
	case 4:
		rgba := imgpkg.NewRGBA(rect)
		copy(rgba.Pix, img.Data)
		src = rgba
	case 1:
		gray := imgpkg.NewGray(rect)
		copy(gray.Pix, img.Data)
		src = gray
	default:
		return nil, fmt.Errorf("encodeJPEG: unsupported channel count %d", img.Channel)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encodeJPEG: %w", err)
	}
	return buf.Bytes(), nil
}
