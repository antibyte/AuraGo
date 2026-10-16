package goldenframe

import (
	"fmt"
	"image"
	"math"
)

// Score is the mean structural similarity per RGB channel; Min is the smallest of the three.
type Score struct{ R, G, B, Min float64 }

// SSIM thresholds per case group (spec §12). A plan may change one only with a documented reason next to the case.
const (
	ThresholdGeometry   = 0.98 // geometry, opacity, fit, keyframed transform
	ThresholdColour     = 0.97 // colour, LUT
	ThresholdTransition = 0.96
	ThresholdDetail     = 0.95 // blur, sharpen, vignette, key, mask
)

const (
	ssimWindow = 11
	ssimSigma  = 1.5
	ssimC1     = (0.01 * 255) * (0.01 * 255)
	ssimC2     = (0.03 * 255) * (0.03 * 255)
)

// ssimKernel is the normalized 11-tap Gaussian (sigma 1.5); the 2D window is its separable product.
var ssimKernel = func() [ssimWindow]float64 {
	var kernel [ssimWindow]float64
	sum := 0.0
	for i := range kernel {
		d := float64(i - ssimWindow/2)
		kernel[i] = math.Exp(-d * d / (2 * ssimSigma * ssimSigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}()

// SSIM is the mean structural similarity per RGB channel (11x11 Gaussian window, sigma 1.5, C1 = (0.01*255)^2,
// C2 = (0.03*255)^2, windows that fit completely); Min is the smallest channel. The images must have the same size.
func SSIM(want, got image.Image) (Score, error) {
	wb, gb := want.Bounds(), got.Bounds()
	if wb.Dx() != gb.Dx() || wb.Dy() != gb.Dy() {
		return Score{}, fmt.Errorf("image sizes differ: want %dx%d, got %dx%d", wb.Dx(), wb.Dy(), gb.Dx(), gb.Dy())
	}
	width, height := wb.Dx(), wb.Dy()
	if width < ssimWindow || height < ssimWindow {
		return Score{}, fmt.Errorf("images of %dx%d are smaller than the %dx%d SSIM window", width, height, ssimWindow, ssimWindow)
	}
	a, b := channels(want), channels(got)
	var mean [3]float64
	for c := range mean {
		mean[c] = ssimChannel(a[c], b[c], width, height)
	}
	return Score{R: mean[0], G: mean[1], B: mean[2], Min: math.Min(mean[0], math.Min(mean[1], mean[2]))}, nil
}

// channels returns the 8-bit R, G and B planes of img, row-major. The goldens are opaque, so straight and
// premultiplied colour agree.
func channels(img image.Image) [3][]float64 {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	var planes [3][]float64
	for c := range planes {
		planes[c] = make([]float64, width*height)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b uint8
			switch src := img.(type) {
			case *image.RGBA:
				i := src.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
				r, g, b = src.Pix[i], src.Pix[i+1], src.Pix[i+2]
			case *image.NRGBA:
				i := src.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
				r, g, b = src.Pix[i], src.Pix[i+1], src.Pix[i+2]
			default:
				cr, cg, cb, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				r, g, b = uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)
			}
			i := y*width + x
			planes[0][i], planes[1][i], planes[2][i] = float64(r), float64(g), float64(b)
		}
	}
	return planes
}

func ssimChannel(a, b []float64, width, height int) float64 {
	n := width * height
	aa, bb, ab := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		aa[i], bb[i], ab[i] = a[i]*a[i], b[i]*b[i], a[i]*b[i]
	}
	mu1, mu2 := blurValid(a, width, height), blurValid(b, width, height)
	s11, s22, s12 := blurValid(aa, width, height), blurValid(bb, width, height), blurValid(ab, width, height)
	sum := 0.0
	for i := range mu1 {
		m1, m2 := mu1[i], mu2[i]
		v1, v2, cov := s11[i]-m1*m1, s22[i]-m2*m2, s12[i]-m1*m2
		sum += (2*m1*m2 + ssimC1) * (2*cov + ssimC2) / ((m1*m1 + m2*m2 + ssimC1) * (v1 + v2 + ssimC2))
	}
	return sum / float64(len(mu1))
}

// blurValid filters plane with the separable Gaussian window wherever the window fits completely.
func blurValid(plane []float64, width, height int) []float64 {
	kernel := ssimKernel[:]
	ow, oh := width-ssimWindow+1, height-ssimWindow+1
	rows := make([]float64, ow*height)
	for y := 0; y < height; y++ {
		line := plane[y*width : (y+1)*width]
		for x := 0; x < ow; x++ {
			s := 0.0
			for k, w := range kernel {
				s += w * line[x+k]
			}
			rows[y*ow+x] = s
		}
	}
	out := make([]float64, ow*oh)
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			s := 0.0
			for k, w := range kernel {
				s += w * rows[(y+k)*ow+x]
			}
			out[y*ow+x] = s
		}
	}
	return out
}
