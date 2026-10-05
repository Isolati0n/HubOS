// Command ssim compares two PNG pictures: the reference first, the picture to judge second.
//
//	ssim [-crop x,y,w,h] reference.png other.png
//
// It prints one line:  ssim=0.987654 psnr=35.21 width=1920 height=1080   (psnr=inf when the pictures are identical).
// Exit status 0 on success, 2 on a usage or file error, 3 when the sizes differ (the line then says so).
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
)

func load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func crop(im image.Image, x, y, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), im, image.Pt(x, y), draw.Src)
	return dst
}

func main() {
	cr := flag.String("crop", "", "compare only this part, taken from the same place in both: x,y,w,h")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: ssim [-crop x,y,w,h] reference.png other.png")
		os.Exit(2)
	}
	a, err := load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	b, err := load(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *cr != "" {
		var x, y, w, h int
		if _, err := fmt.Sscanf(*cr, "%d,%d,%d,%d", &x, &y, &w, &h); err != nil {
			fmt.Fprintln(os.Stderr, "bad -crop")
			os.Exit(2)
		}
		a, b = crop(a, x, y, w, h), crop(b, x, y, w, h)
	}
	s, p, err := Compare(a, b)
	if err == ErrSize {
		fmt.Printf("ssim=NA psnr=NA size_mismatch reference=%dx%d other=%dx%d\n", a.Bounds().Dx(), a.Bounds().Dy(), b.Bounds().Dx(), b.Bounds().Dy())
		os.Exit(3)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ps := fmt.Sprintf("%.2f", p)
	if math.IsInf(p, 1) {
		ps = "inf"
	}
	fmt.Printf("ssim=%.6f psnr=%s width=%d height=%d\n", s, ps, a.Bounds().Dx(), a.Bounds().Dy())
}
