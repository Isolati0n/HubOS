package main

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// textLike draws fine dark strokes on white, the kind of picture where lossy coding hurts most.
func textLike(w, h int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(245)
			if x%6 < 2 || y%10 == 3 {
				v = 20
			}
			im.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return im
}

func TestIdentical(t *testing.T) {
	a := textLike(64, 48)
	s, p, err := Compare(a, a)
	if err != nil || s != 1 || !math.IsInf(p, 1) {
		t.Fatalf("identical: ssim=%v psnr=%v err=%v", s, p, err)
	}
}

func TestKnownPSNR(t *testing.T) {
	// every value differs by exactly 1: MSE is 1, PSNR is 10*log10(255^2) = 48.1308 dB
	a := image.NewRGBA(image.Rect(0, 0, 16, 16))
	b := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := range a.Pix {
		a.Pix[i] = 100
		b.Pix[i] = 100
	}
	for i := 0; i < len(b.Pix); i += 4 {
		b.Pix[i], b.Pix[i+1], b.Pix[i+2] = 101, 101, 101
		a.Pix[i+3], b.Pix[i+3] = 255, 255
	}
	_, p, err := Compare(a, b)
	if err != nil || math.Abs(p-48.1308) > 0.001 {
		t.Fatalf("psnr=%v err=%v", p, err)
	}
}

func TestBlurIsWorse(t *testing.T) {
	a := textLike(96, 64)
	b := image.NewRGBA(a.Bounds())
	// horizontal 3-tap blur
	for y := 0; y < 64; y++ {
		for x := 0; x < 96; x++ {
			var s int
			for d := -1; d <= 1; d++ {
				xx := x + d
				if xx < 0 {
					xx = 0
				}
				if xx > 95 {
					xx = 95
				}
				s += int(a.RGBAAt(xx, y).R)
			}
			v := uint8(s / 3)
			b.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	s, p, err := Compare(a, b)
	if err != nil || s >= 0.99 || s <= 0 || p <= 5 || p >= 40 {
		t.Fatalf("blur: ssim=%v psnr=%v err=%v", s, p, err)
	}
}

func TestInvertedIsLow(t *testing.T) {
	a := textLike(64, 48)
	b := image.NewRGBA(a.Bounds())
	for i := 0; i < len(a.Pix); i += 4 {
		b.Pix[i], b.Pix[i+1], b.Pix[i+2], b.Pix[i+3] = 255-a.Pix[i], 255-a.Pix[i+1], 255-a.Pix[i+2], 255
	}
	s, _, err := Compare(a, b)
	if err != nil || s > 0.2 {
		t.Fatalf("inverted: ssim=%v err=%v", s, err)
	}
}

func TestSizeMismatch(t *testing.T) {
	_, _, err := Compare(textLike(32, 32), textLike(32, 33))
	if err != ErrSize {
		t.Fatalf("want ErrSize, got %v", err)
	}
}
