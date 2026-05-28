// sd-encode is the smallest possible "images to video" example: read a
// collection of PNG and/or JPEG frames from a directory and mux them into
// a single Motion-JPEG AVI. No model is loaded; this is a pure-Go encoder
// built on top of pkg/sd's SaveAVI helper.
//
//	go run ./examples/sd-encode -i frames/ -o out.avi -fps 24 -secs 1
//
// Files inside -i are sorted lexicographically; name them with a
// zero-padded index (frame_0001.jpg, frame_0002.jpg, ...) to control order.
//
// Frame sizing:
//
// AVI requires every frame to share the same width and height. The first
// image sets the target dimensions unless -w/-h are supplied. When a later
// image has different dimensions, -fit controls what happens:
//
//	letterbox  scale to fit inside target, preserve aspect ratio, fill the
//	           remaining margins with black (default; no distortion or
//	           cropping)
//	crop       scale to cover the target, preserve aspect ratio, center-crop
//	           the overflow (no distortion or bars, loses edge pixels)
//	stretch    scale to exactly target WxH, ignoring aspect ratio
//	           (fastest, distorts if aspect ratios differ)
//	skip       drop any frame whose dimensions do not exactly match target
//
// Transitions:
//
// -trans selects how to move from one image to the next:
//
//	crossfade  blend pixels of A and B for -xfade seconds (default)
//	fadeblack  A fades to black, then black fades to B
//	fadewhite  A fades to white, then white fades to B
//	wipe       B reveals over A from left to right
//	cut        abrupt change (no transition frames)
//	kenburns   ignore -trans transitions; instead apply a slow zoom-in to
//	           each held image so the slideshow has motion
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/image/draw"

	"github.com/ardanlabs/malina/pkg/sd"
)

const (
	fitLetterbox = "letterbox"
	fitCrop      = "crop"
	fitStretch   = "stretch"
	fitSkip      = "skip"

	transCrossfade = "crossfade"
	transFadeBlack = "fadeblack"
	transFadeWhite = "fadewhite"
	transWipe      = "wipe"
	transCut       = "cut"
	transKenBurns  = "kenburns"
)

func main() {
	var (
		inputDir = flag.String("i", "", "directory containing the source PNG/JPEG frames")
		outPath  = flag.String("o", "output.avi", "output AVI file path")
		fps      = flag.Int("fps", 24, "frames per second")
		quality  = flag.Int("quality", 90, "JPEG quality (1-100)")
		forceW   = flag.Int("w", 0, "force output width in pixels (0 = use first frame's width)")
		forceH   = flag.Int("h", 0, "force output height in pixels (0 = use first frame's height)")
		fitMode  = flag.String("fit", fitLetterbox,
			"how to handle frames whose dimensions differ from the target: letterbox|crop|stretch|skip")
		holdSecs = flag.Float64("secs", 2,
			"hold each image for this many seconds (excluding transition time)")
		transMode = flag.String("trans", transCrossfade,
			"transition between images: crossfade|fadeblack|fadewhite|wipe|cut|kenburns")
		xfadeSecs = flag.Float64("xfade", 0.5,
			"transition duration in seconds (ignored for cut and kenburns)")
	)
	flag.Parse()

	if *inputDir == "" {
		log.Fatal("missing -i <frames directory>")
	}
	switch *fitMode {
	case fitLetterbox, fitCrop, fitStretch, fitSkip:
	default:
		log.Fatalf("invalid -fit %q (want letterbox|crop|stretch|skip)", *fitMode)
	}
	switch *transMode {
	case transCrossfade, transFadeBlack, transFadeWhite, transWipe, transCut, transKenBurns:
	default:
		log.Fatalf("invalid -trans %q (want crossfade|fadeblack|fadewhite|wipe|cut|kenburns)", *transMode)
	}

	paths, err := listImages(*inputDir)
	if err != nil {
		log.Fatal(err)
	}
	if len(paths) == 0 {
		log.Fatalf("no .png/.jpg/.jpeg files found in %s", *inputDir)
	}

	// Load every source image once. Originals are kept (not the resized
	// versions) so Ken Burns can zoom into the full source pixels.
	srcs := make([]image.Image, 0, len(paths))
	skipped := 0
	var targetW, targetH int
	for i, p := range paths {
		img, err := decodeImage(p)
		if err != nil {
			log.Fatalf("load %s: %v", p, err)
		}

		if i == 0 {
			targetW = *forceW
			if targetW == 0 {
				targetW = img.Bounds().Dx()
			}
			targetH = *forceH
			if targetH == 0 {
				targetH = img.Bounds().Dy()
			}
		}

		if *fitMode == fitSkip {
			if img.Bounds().Dx() != targetW || img.Bounds().Dy() != targetH {
				fmt.Printf("skipping %s (%dx%d != %dx%d)\n",
					filepath.Base(p), img.Bounds().Dx(), img.Bounds().Dy(), targetW, targetH)
				skipped++
				continue
			}
		}
		srcs = append(srcs, img)
	}

	if len(srcs) == 0 {
		log.Fatalf("no frames left to encode (skipped %d)", skipped)
	}

	holdFrames := round(*holdSecs * float64(*fps))
	if holdFrames < 1 {
		holdFrames = 1
	}
	transFrames := round(*xfadeSecs * float64(*fps))
	if transFrames < 0 {
		transFrames = 0
	}

	var frames []*sd.SDImage
	if *transMode == transKenBurns {
		frames = buildKenBurns(srcs, targetW, targetH, holdFrames)
	} else {
		// Pre-render each source to a target-sized RGBA according to -fit
		// so transitions operate on identically-sized pixel buffers.
		fitted := make([]*image.RGBA, len(srcs))
		for i, s := range srcs {
			sw, sh := s.Bounds().Dx(), s.Bounds().Dy()
			if sw == targetW && sh == targetH {
				if rgba, ok := s.(*image.RGBA); ok {
					fitted[i] = rgba
					continue
				}
			}
			if sw != targetW || sh != targetH {
				fmt.Printf("%s %s from %dx%d -> %dx%d\n",
					*fitMode, filepath.Base(paths[i]), sw, sh, targetW, targetH)
			}
			fitted[i] = fitFrame(s, targetW, targetH, *fitMode)
		}
		frames = buildTransitions(fitted, holdFrames, transFrames, *transMode)
	}

	if err := sd.SaveAVI(*outPath, frames, *fps, *quality); err != nil {
		log.Fatalf("save AVI: %v", err)
	}

	fmt.Printf("wrote %s (%d frames, %dx%d @ %d fps; %d skipped; trans=%s)\n",
		*outPath, len(frames), targetW, targetH, *fps, skipped, *transMode)
}

// =============================================================================
// File listing + decoding

// listImages returns the sorted absolute paths of all PNG and JPEG files
// directly inside dir (non-recursive).
func listImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".png", ".jpg", ".jpeg":
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// decodeImage reads a PNG or JPEG file from disk and returns the decoded
// image. Format is detected automatically via the registered decoders.
func decodeImage(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// =============================================================================
// Fitting (resize / letterbox / crop / stretch)

// fitFrame returns a new W x H RGBA image containing src rendered according
// to mode. mode is assumed to be a valid resize mode (caller validates).
func fitFrame(src image.Image, w, h int, mode string) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	switch mode {
	case fitStretch:
		draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)

	case fitLetterbox:
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
		inner := fitRect(src.Bounds().Dx(), src.Bounds().Dy(), w, h)
		draw.BiLinear.Scale(dst, inner, src, src.Bounds(), draw.Src, nil)

	case fitCrop:
		coverW, coverH := coverSize(src.Bounds().Dx(), src.Bounds().Dy(), w, h)
		tmp := image.NewRGBA(image.Rect(0, 0, coverW, coverH))
		draw.BiLinear.Scale(tmp, tmp.Bounds(), src, src.Bounds(), draw.Src, nil)
		off := image.Point{X: (coverW - w) / 2, Y: (coverH - h) / 2}
		draw.Draw(dst, dst.Bounds(), tmp, off, draw.Src)

	default:
		// fitSkip and unknown modes: just stretch as a safe fallback.
		draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	}

	return dst
}

// fitRect returns the largest sub-rectangle of a (dstW x dstH) target that
// preserves the (srcW x srcH) aspect ratio, centered in the target. This
// is the "contain" / letterbox rectangle.
func fitRect(srcW, srcH, dstW, dstH int) image.Rectangle {
	if srcW <= 0 || srcH <= 0 {
		return image.Rect(0, 0, dstW, dstH)
	}
	if srcW*dstH > dstW*srcH {
		w := dstW
		h := srcH * dstW / srcW
		y := (dstH - h) / 2
		return image.Rect(0, y, w, y+h)
	}
	h := dstH
	w := srcW * dstH / srcH
	x := (dstW - w) / 2
	return image.Rect(x, 0, x+w, h)
}

// coverSize returns the smallest (W, H) that fully covers a (dstW x dstH)
// box while preserving the (srcW x srcH) aspect ratio.
func coverSize(srcW, srcH, dstW, dstH int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return dstW, dstH
	}
	if srcW*dstH > dstW*srcH {
		return srcW * dstH / srcH, dstH
	}
	return dstW, srcH * dstW / srcW
}

// =============================================================================
// Transitions between fitted frames

// buildTransitions returns the output frame sequence for the given
// already-fitted frames, inserting transFrames intermediate frames between
// each pair according to mode (other than cut which inserts none).
func buildTransitions(fitted []*image.RGBA, holdFrames, transFrames int, mode string) []*sd.SDImage {
	if mode == transCut {
		transFrames = 0
	}

	out := make([]*sd.SDImage, 0,
		len(fitted)*holdFrames+(len(fitted)-1)*transFrames)

	for i, a := range fitted {
		aSD := rgbaToSDImage(a)
		for k := 0; k < holdFrames; k++ {
			out = append(out, aSD)
		}
		if i < len(fitted)-1 && transFrames > 0 {
			b := fitted[i+1]
			for k := 1; k <= transFrames; k++ {
				t := float64(k) / float64(transFrames+1)
				out = append(out, transitionFrame(a, b, t, mode))
			}
		}
	}
	return out
}

// transitionFrame returns a new RGBA frame interpolated between a and b
// at progress t (0 < t < 1) using the chosen transition mode. a and b
// must have identical dimensions.
func transitionFrame(a, b *image.RGBA, t float64, mode string) *sd.SDImage {
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	switch mode {
	case transCrossfade:
		blend(dst.Pix, a.Pix, b.Pix, t)

	case transFadeBlack:
		if t < 0.5 {
			fadeToColor(dst.Pix, a.Pix, color.RGBA{0, 0, 0, 255}, 2*t)
		} else {
			fadeFromColor(dst.Pix, b.Pix, color.RGBA{0, 0, 0, 255}, 2*(t-0.5))
		}

	case transFadeWhite:
		if t < 0.5 {
			fadeToColor(dst.Pix, a.Pix, color.RGBA{255, 255, 255, 255}, 2*t)
		} else {
			fadeFromColor(dst.Pix, b.Pix, color.RGBA{255, 255, 255, 255}, 2*(t-0.5))
		}

	case transWipe:
		split := int(t*float64(w) + 0.5)
		for y := 0; y < h; y++ {
			rowStart := y * dst.Stride
			copy(dst.Pix[rowStart:rowStart+split*4], b.Pix[rowStart:rowStart+split*4])
			copy(dst.Pix[rowStart+split*4:rowStart+w*4], a.Pix[rowStart+split*4:rowStart+w*4])
		}
	}

	return rgbaToSDImage(dst)
}

// blend writes (1-t)*a + t*b into dst for every RGBA byte (alpha included
// but always 255 for our inputs).
func blend(dst, a, b []byte, t float64) {
	wa := uint32((1 - t) * 256)
	wb := uint32(t * 256)
	for i := 0; i < len(dst); i++ {
		dst[i] = byte((uint32(a[i])*wa + uint32(b[i])*wb) >> 8)
	}
}

// fadeToColor writes ((1-t)*src + t*c) into dst pixel-by-pixel.
func fadeToColor(dst, src []byte, c color.RGBA, t float64) {
	ws := uint32((1 - t) * 256)
	wc := uint32(t * 256)
	cb := [4]byte{c.R, c.G, c.B, c.A}
	for i := 0; i < len(dst); i++ {
		dst[i] = byte((uint32(src[i])*ws + uint32(cb[i&3])*wc) >> 8)
	}
}

// fadeFromColor writes ((1-t)*c + t*src) into dst pixel-by-pixel.
func fadeFromColor(dst, src []byte, c color.RGBA, t float64) {
	fadeToColor(dst, src, c, 1-t)
}

// =============================================================================
// Ken Burns (per-image slow zoom)

// buildKenBurns emits holdFrames per source image, each frame extracting a
// progressively-shrinking sub-rectangle (matching target aspect) of the
// source and scaling it to (targetW, targetH). Cuts between images are
// abrupt (no inter-image transition).
func buildKenBurns(srcs []image.Image, targetW, targetH, holdFrames int) []*sd.SDImage {
	out := make([]*sd.SDImage, 0, len(srcs)*holdFrames)

	const startZoom = 1.00
	const endZoom = 0.70

	for idx, src := range srcs {
		sb := src.Bounds()
		sw, sh := sb.Dx(), sb.Dy()

		// Largest sub-rectangle of src that matches the target aspect ratio
		// (so the zoom never produces letterbox bars).
		startInner := fitRect(targetW, targetH, sw, sh)

		// Alternate the pan direction per image so the slideshow feels varied.
		dirX := 1
		dirY := 1
		if idx%2 == 1 {
			dirX = -1
		}
		if idx%4 >= 2 {
			dirY = -1
		}

		for k := 0; k < holdFrames; k++ {
			var t float64
			if holdFrames > 1 {
				t = float64(k) / float64(holdFrames-1)
			}
			zoom := startZoom + (endZoom-startZoom)*t

			zw := int(float64(startInner.Dx()) * zoom)
			zh := int(float64(startInner.Dy()) * zoom)

			// Center of zoom rectangle pans across the image by up to half
			// the available slack in the chosen direction.
			slackX := (startInner.Dx() - zw) / 2
			slackY := (startInner.Dy() - zh) / 2
			cx := startInner.Min.X + startInner.Dx()/2 + int(float64(dirX*slackX)*t)
			cy := startInner.Min.Y + startInner.Dy()/2 + int(float64(dirY*slackY)*t)

			sub := image.Rect(cx-zw/2, cy-zh/2, cx-zw/2+zw, cy-zh/2+zh)
			// Clamp to source bounds.
			if sub.Min.X < sb.Min.X {
				sub = sub.Add(image.Point{X: sb.Min.X - sub.Min.X})
			}
			if sub.Min.Y < sb.Min.Y {
				sub = sub.Add(image.Point{Y: sb.Min.Y - sub.Min.Y})
			}
			if sub.Max.X > sb.Max.X {
				sub = sub.Sub(image.Point{X: sub.Max.X - sb.Max.X})
			}
			if sub.Max.Y > sb.Max.Y {
				sub = sub.Sub(image.Point{Y: sub.Max.Y - sb.Max.Y})
			}

			dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
			draw.BiLinear.Scale(dst, dst.Bounds(), src, sub, draw.Src, nil)
			out = append(out, rgbaToSDImage(dst))
		}
	}
	return out
}

// =============================================================================
// Helpers

// rgbaToSDImage builds an SDImage (3-channel RGB) from an RGBA image.
// Alpha is discarded.
func rgbaToSDImage(rgba *image.RGBA) *sd.SDImage {
	w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()
	out := &sd.SDImage{
		Width:   uint32(w),
		Height:  uint32(h),
		Channel: 3,
		Data:    make([]byte, w*h*3),
	}
	src := rgba.Pix
	dst := out.Data
	for i, j := 0, 0; i < len(src); i, j = i+4, j+3 {
		dst[j+0] = src[i+0]
		dst[j+1] = src[i+1]
		dst[j+2] = src[i+2]
	}
	return out
}

func round(f float64) int {
	if f >= 0 {
		return int(f + 0.5)
	}
	return int(f - 0.5)
}
