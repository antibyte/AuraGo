package pixelmatte

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buffer.Bytes()
}

func closeTo(got, want, tolerance float32) bool {
	return float32(math.Abs(float64(got-want))) <= tolerance
}

func TestTensorFromImageNormalisesLikeRembg(t *testing.T) {
	// 2x2 image: the largest channel value is 200, so every channel is first
	// divided by 200, then mean/std are applied. Alpha is ignored (straight
	// colour): the transparent pixel keeps its colour.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 0, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 50, B: 20, A: 255})
	img.SetNRGBA(0, 1, color.NRGBA{R: 10, G: 20, B: 30, A: 0})
	img.SetNRGBA(1, 1, color.NRGBA{R: 40, G: 80, B: 160, A: 128})

	for _, kind := range []Kind{KindStandard, KindPrecise} {
		spec := models[kind]
		pixels := tensorFromImage(img, spec, 2)
		if len(pixels) != 3*2*2 {
			t.Fatalf("%s: tensor holds %d values, want 12", kind, len(pixels))
		}
		want := [4][3]float32{{200, 100, 0}, {0, 50, 20}, {10, 20, 30}, {40, 80, 160}}
		for pixel := 0; pixel < 4; pixel++ {
			for c := 0; c < 3; c++ {
				expected := (want[pixel][c]/200 - spec.Mean[c]) / spec.Std[c]
				got := pixels[c*4+pixel] // NCHW: plane c, pixel index
				if !closeTo(got, expected, 0.02) {
					t.Errorf("%s: pixel %d channel %d = %v, want %v", kind, pixel, c, got, expected)
				}
			}
		}
	}
}

func TestTensorFromImageBlackImageStaysFinite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	spec := models[KindStandard]
	for i, v := range tensorFromImage(img, spec, 4) {
		if want := -spec.Mean[i/16] / spec.Std[i/16]; !closeTo(v, want, 1e-4) {
			t.Fatalf("value %d = %v, want %v", i, v, want)
		}
	}
}

func TestPreprocessProducesModelInput(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for x := 0; x < 3; x++ {
		src.Set(x, 0, color.RGBA{R: uint8(80 * x), G: 40, B: 255, A: 255})
		src.Set(x, 1, color.RGBA{R: 255, G: uint8(80 * x), B: 40, A: 255})
	}
	pixels, width, height, err := Preprocess(encodePNG(t, src), models[KindPrecise])
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}
	if width != 3 || height != 2 || len(pixels) != 3*InputSize*InputSize {
		t.Fatalf("got %dx%d with %d values, want 3x2 with %d", width, height, len(pixels), 3*InputSize*InputSize)
	}
}

func TestPreprocessRejectsBadInput(t *testing.T) {
	var gif bytes.Buffer
	gif.WriteString("GIF89a")
	huge := encodePNG(t, image.NewGray(image.Rect(0, 0, 1, 1)))
	// Rewrite the IHDR size to 8192x8192 (64 Mi pixels) and fix its CRC: the
	// header check must reject the image before any pixel is decoded.
	copy(huge[16:24], []byte{0, 0, 0x20, 0, 0, 0, 0x20, 0})
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if config, _, err := image.DecodeConfig(bytes.NewReader(huge)); err != nil || config.Width != 8192 || config.Height != 8192 {
		t.Fatalf("test image header = %+v, %v", config, err)
	}
	cases := map[string][]byte{
		"empty":       nil,
		"not image":   []byte("hello"),
		"gif header":  gif.Bytes(),
		"too many px": huge,
	}
	for name, data := range cases {
		if _, _, _, err := Preprocess(data, models[KindStandard]); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestMatteFromOutputNormalisesAndResizes(t *testing.T) {
	spec := models[KindStandard]

	// Min-max: [0 1 2 4] -> [0 64 128 255] (63.75 and 127.5 round up).
	matte, err := matteFromOutput([]float32{0, 1, 2, 4}, []int64{1, 1, 2, 2}, spec, 2, 2, 2)
	if err != nil {
		t.Fatalf("matteFromOutput: %v", err)
	}
	if want := []uint8{0, 64, 128, 255}; !bytes.Equal(matte.Pix, want) {
		t.Fatalf("matte = %v, want %v", matte.Pix, want)
	}

	// Only the first channel is used.
	matte, err = matteFromOutput([]float32{0, 1, 2, 4, 9, 9, 9, 9}, []int64{1, 2, 2, 2}, spec, 2, 2, 2)
	if err != nil || !bytes.Equal(matte.Pix, []uint8{0, 64, 128, 255}) {
		t.Fatalf("two channels: %v, %v", matte, err)
	}

	// A constant plane has no foreground.
	matte, err = matteFromOutput([]float32{3, 3, 3, 3}, []int64{1, 1, 2, 2}, spec, 2, 2, 2)
	if err != nil || !bytes.Equal(matte.Pix, []uint8{0, 0, 0, 0}) {
		t.Fatalf("constant plane: %v, %v", matte, err)
	}

	// Resize back: 2x2 -> 6x4 keeps the corners and stays monotonic across.
	matte, err = matteFromOutput([]float32{0, 0, 4, 4}, []int64{1, 1, 2, 2}, spec, 6, 4, 2)
	if err != nil {
		t.Fatalf("matteFromOutput: %v", err)
	}
	if b := matte.Bounds(); b.Dx() != 6 || b.Dy() != 4 {
		t.Fatalf("matte is %v, want 6x4", b)
	}
	if matte.GrayAt(0, 0).Y != 0 || matte.GrayAt(5, 3).Y != 255 {
		t.Fatalf("corners = %d, %d; want 0, 255", matte.GrayAt(0, 0).Y, matte.GrayAt(5, 3).Y)
	}
	for y := 1; y < 4; y++ {
		if matte.GrayAt(2, y).Y < matte.GrayAt(2, y-1).Y {
			t.Fatalf("column 2 is not monotonic at row %d", y)
		}
	}
}

func TestMatteFromOutputSigmoidForBiRefNet(t *testing.T) {
	spec := models[KindPrecise]
	// Logits -8, 0, 8 and 0: sigmoid first (0.0003, 0.5, 0.9997, 0.5), then
	// min-max -> 0, ~0.5, 1, ~0.5.
	matte, err := matteFromOutput([]float32{-8, 0, 8, 0}, []int64{1, 1, 2, 2}, spec, 2, 2, 2)
	if err != nil {
		t.Fatalf("matteFromOutput: %v", err)
	}
	if matte.Pix[0] != 0 || matte.Pix[2] != 255 || matte.Pix[1] != matte.Pix[3] || matte.Pix[1] < 126 || matte.Pix[1] > 129 {
		t.Fatalf("matte = %v, want [0 ~127 255 ~127]", matte.Pix)
	}
}

func TestMatteFromOutputRejectsBadTensors(t *testing.T) {
	spec := models[KindStandard]
	plane := []float32{0, 1, 2, 3}
	cases := map[string]struct {
		data  []float32
		shape []int64
	}{
		"wrong spatial size": {plane, []int64{1, 1, 2, 4}},
		"rank 2":             {plane, []int64{2, 2}},
		"batch of two":       {append(plane, plane...), []int64{2, 1, 2, 2}},
		"length mismatch":    {plane[:3], []int64{1, 1, 2, 2}},
		"NaN":                {[]float32{0, float32(math.NaN()), 1, 2}, []int64{1, 1, 2, 2}},
		"Inf":                {[]float32{0, float32(math.Inf(1)), 1, 2}, []int64{1, 1, 2, 2}},
	}
	for name, c := range cases {
		if _, err := matteFromOutput(c.data, c.shape, spec, 2, 2, 2); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := matteFromOutput(plane, []int64{1, 1, 2, 2}, spec, 0, 2, 2); err == nil {
		t.Error("zero target width: expected an error")
	}
}

func TestPostprocessAtFullSizeAndEncode(t *testing.T) {
	output := make([]float32, InputSize*InputSize)
	for y := 0; y < InputSize; y++ {
		for x := 0; x < InputSize; x++ {
			output[y*InputSize+x] = float32(x) // left to right ramp
		}
	}
	for _, size := range [][2]int{{200, 100}, {1024, 1024}, {2048, 700}} {
		matte, err := Postprocess(output, []int64{1, 1, InputSize, InputSize}, models[KindStandard], size[0], size[1])
		if err != nil {
			t.Fatalf("Postprocess %v: %v", size, err)
		}
		encoded, err := encodeMatte(matte)
		if err != nil {
			t.Fatalf("encodeMatte: %v", err)
		}
		decoded, err := png.Decode(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("decode matte: %v", err)
		}
		gray, ok := decoded.(*image.Gray)
		if !ok || gray.Bounds().Dx() != size[0] || gray.Bounds().Dy() != size[1] {
			t.Fatalf("%v: decoded %T %v, want *image.Gray of that size", size, decoded, decoded.Bounds())
		}
		left, right := gray.GrayAt(0, 0).Y, gray.GrayAt(size[0]-1, 0).Y
		if left > 5 || right < 250 {
			t.Fatalf("%v: ramp ends = %d, %d", size, left, right)
		}
	}
}
