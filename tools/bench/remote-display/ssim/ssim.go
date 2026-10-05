package main

import (
	"errors"
	"image"
	"math"
)

// The numbers follow the usual definitions:
//
//   - PSNR is 10*log10(255^2 / MSE) over all red, green and blue values. Identical pictures give +Inf (printed as "inf").
//   - SSIM is the mean structural similarity of the luma (0.299 R + 0.587 G + 0.114 B) over 8x8 windows moved in steps of 4
//     pixels, with the constants C1 = (0.01*255)^2 and C2 = (0.03*255)^2 from Wang et al. 2004, equal weight in each window
//     (no Gaussian weighting). It is the common "fast" form; absolute values differ a little from other tools, so only compare
//     numbers made by this program.
const (
	win  = 8
	step = 4
	c1   = (0.01 * 255) * (0.01 * 255)
	c2   = (0.03 * 255) * (0.03 * 255)
)

// ErrSize is returned when the two pictures do not have the same width and height.
var ErrSize = errors.New("pictures differ in size")

// luma returns the luma plane (0..255) and the raw RGB (8 bit) of a picture, row by row.
func planes(im image.Image) (y []float64, rgb []uint8, w, h int) {
	b := im.Bounds()
	w, h = b.Dx(), b.Dy()
	y = make([]float64, w*h)
	rgb = make([]uint8, w*h*3)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			r, g, bl, _ := im.At(b.Min.X+i, b.Min.Y+j).RGBA()
			r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(bl>>8)
			k := j*w + i
			rgb[3*k], rgb[3*k+1], rgb[3*k+2] = r8, g8, b8
			y[k] = 0.299*float64(r8) + 0.587*float64(g8) + 0.114*float64(b8)
		}
	}
	return
}

// Compare returns SSIM (0..1) and PSNR (dB) of b against the reference a.
func Compare(a, b image.Image) (ssim, psnr float64, err error) {
	ya, ra, w, h := planes(a)
	yb, rb, w2, h2 := planes(b)
	if w != w2 || h != h2 {
		return 0, 0, ErrSize
	}
	var se float64
	for i := range ra {
		d := float64(ra[i]) - float64(rb[i])
		se += d * d
	}
	if se == 0 {
		psnr = math.Inf(1)
	} else {
		psnr = 10 * math.Log10(255*255/(se/float64(len(ra))))
	}
	if w < win || h < win {
		return 0, psnr, errors.New("picture smaller than 8x8")
	}
	var sum float64
	var n int
	for y0 := 0; y0+win <= h; y0 += step {
		for x0 := 0; x0+win <= w; x0 += step {
			var sa, sb, saa, sbb, sab float64
			for j := 0; j < win; j++ {
				row := (y0+j)*w + x0
				for i := 0; i < win; i++ {
					p, q := ya[row+i], yb[row+i]
					sa += p
					sb += q
					saa += p * p
					sbb += q * q
					sab += p * q
				}
			}
			const nn = win * win
			ma, mb := sa/nn, sb/nn
			va, vb := saa/nn-ma*ma, sbb/nn-mb*mb
			cov := sab/nn - ma*mb
			sum += ((2*ma*mb + c1) * (2*cov + c2)) / ((ma*ma + mb*mb + c1) * (va + vb + c2))
			n++
		}
	}
	return sum / float64(n), psnr, nil
}
