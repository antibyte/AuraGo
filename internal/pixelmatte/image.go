package pixelmatte

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
)

// MaxPixels bounds the decoded input image (32 Mi pixels, like the Pixel
// handlers); MaxImageBytes bounds the encoded input.
const (
	MaxPixels     = 32 * 1024 * 1024
	MaxImageBytes = 32 << 20
)

// Preprocess decodes a PNG or JPEG and returns the model input, NCHW float32
// [1,3,InputSize,InputSize], plus the original size. Alpha is dropped like
// PIL's convert("RGB") does (the straight colour is used), the image is
// resized to InputSize x InputSize ignoring the aspect ratio (bilinear),
// divided by the largest channel value of the resized image (at least 1e-6,
// 8-bit units) and normalised with the model's mean and standard deviation.
func Preprocess(data []byte, spec ModelSpec) (pixels []float32, width, height int, err error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := img.Bounds()
	return tensorFromImage(img, spec, InputSize), bounds.Dx(), bounds.Dy(), nil
}

// decodeImage decodes a PNG or JPEG after checking its header: format, size
// limits. The PNG decoder is registered by the png import above.
func decodeImage(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no image data")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, fmt.Errorf("a valid PNG or JPEG image is required")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > MaxPixels {
		return nil, fmt.Errorf("image dimensions exceed the limit (%d pixels)", MaxPixels)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid image data")
	}
	return img, nil
}

// tensorFromImage is Preprocess after decoding, with a configurable model
// input size.
func tensorFromImage(img image.Image, spec ModelSpec, size int) []float32 {
	switch src := img.(type) {
	case *image.NRGBA:
		// The decoder's buffer is ours: make it opaque in place instead of
		// copying up to 128 MB. The straight colour of translucent pixels is
		// kept, which is what PIL's convert("RGB") does.
		for i := 3; i < len(src.Pix); i += 4 {
			src.Pix[i] = 0xff
		}
	case *image.NRGBA64:
		for i := 0; i+7 < len(src.Pix); i += 8 {
			src.Pix[i+6], src.Pix[i+7] = 0xff, 0xff
		}
	}
	resized := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), xdraw.Src, nil)

	// Premultiplied pixels (a source that was not made opaque above) are
	// converted back to straight colour so a half-transparent edge is not
	// darkened twice.
	pix := resized.Pix
	var maxValue float32
	for i := 0; i+3 < len(pix); i += 4 {
		alpha := uint32(pix[i+3])
		if alpha != 0xff {
			for c := 0; c < 3; c++ {
				if alpha == 0 {
					pix[i+c] = 0
				} else {
					pix[i+c] = uint8(min(uint32(pix[i+c])*0xff/alpha, 0xff))
				}
			}
		}
		for c := 0; c < 3; c++ {
			maxValue = max(maxValue, float32(pix[i+c]))
		}
	}
	maxValue = max(maxValue, 1e-6)

	plane := size * size
	tensor := make([]float32, 3*plane)
	for i := 0; i < plane; i++ {
		for c := 0; c < 3; c++ {
			tensor[c*plane+i] = (float32(pix[i*4+c])/maxValue - spec.Mean[c]) / spec.Std[c]
		}
	}
	return tensor
}

// Postprocess turns the first output plane of the model into an 8-bit
// grayscale matte of the original size. output is the tensor data with the
// given shape, [1,1,S,S] (or [1,S,S]) with S = InputSize; only the first
// channel is used. The plane gets a sigmoid when the model needs one, is
// min-max normalised to 0..1 (a constant plane becomes 0) and resized back
// bilinearly.
func Postprocess(output []float32, shape []int64, spec ModelSpec, width, height int) (*image.Gray, error) {
	return matteFromOutput(output, shape, spec, width, height, InputSize)
}

func matteFromOutput(output []float32, shape []int64, spec ModelSpec, width, height, size int) (*image.Gray, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid target size %dx%d", width, height)
	}
	if len(shape) < 3 || len(shape) > 4 || shape[len(shape)-1] != int64(size) || shape[len(shape)-2] != int64(size) {
		return nil, fmt.Errorf("unexpected model output shape %v (want [1 1 %d %d])", shape, size, size)
	}
	total := int64(1)
	for _, dim := range shape {
		if dim <= 0 {
			return nil, fmt.Errorf("unexpected model output shape %v", shape)
		}
		total *= dim
	}
	if shape[0] != 1 || int64(len(output)) != total {
		return nil, fmt.Errorf("model output holds %d values for shape %v", len(output), shape)
	}
	plane := output[:size*size]

	values := make([]float32, len(plane))
	low, high := float32(math.Inf(1)), float32(math.Inf(-1))
	for i, v := range plane {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("model output is not finite")
		}
		if spec.Sigmoid {
			v = float32(1 / (1 + math.Exp(-float64(v))))
		}
		values[i] = v
		low, high = min(low, v), max(high, v)
	}
	small := image.NewGray(image.Rect(0, 0, size, size))
	if span := high - low; span > 0 {
		for i, v := range values {
			small.Pix[i] = uint8((v-low)/span*255 + 0.5)
		}
	}
	return resizeGray(small, width, height), nil
}

// resizeGray resizes a matte bilinearly. The resize runs on RGBA buffers: the
// scaler has fast paths for them and a generic path, an order of magnitude
// slower, for an *image.Gray destination.
func resizeGray(src *image.Gray, width, height int) *image.Gray {
	bounds := src.Bounds()
	if bounds.Dx() == width && bounds.Dy() == height {
		return src
	}
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.BiLinear.Scale(scaled, scaled.Bounds(), src, bounds, xdraw.Src, nil)
	out := image.NewGray(scaled.Bounds())
	for i := range out.Pix {
		out.Pix[i] = scaled.Pix[i*4]
	}
	return out
}

// encodeMatte writes a matte as a PNG, optimised for speed: a matte is mostly
// flat regions and large ones are written inside a request.
func encodeMatte(matte *image.Gray) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buffer, matte); err != nil {
		return nil, fmt.Errorf("encode matte: %w", err)
	}
	return buffer.Bytes(), nil
}
