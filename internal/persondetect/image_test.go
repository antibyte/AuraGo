package persondetect

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"strings"
	"testing"

	"golang.org/x/image/draw"
)

func encodeTestJPEG(t testing.TB, width, height int, fill func(x, y int) color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, fill(x, y))
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// cmykJPEGBase64 is a 32x16 Adobe CMYK JPEG written by PIL (the Go encoder
// cannot write CMYK): the left half is CMYK(0,255,255,0) = red, the right half
// CMYK(255,255,0,0) = blue. A real-world source of such files is a camera
// proxy or a desktop screenshot tool that saves print-profile JPEGs.
const cmykJPEGBase64 = "/9j/7gAOQWRvYmUAZAAAAAAA/9sAQwACAQEBAQECAQEBAgICAgIEAwICAgIFBAQDBAYFBgYGBQYGBgcJCAYHCQcGBggLCAkKCgoKCgYICwwLCgwJCgoK" +
	"/8AAFAgAEAAgBEMRAE0RAFkRAEsRAP/EAB8AAAEFAQEBAQEBAAAAAAAAAAABAgMEBQYHCAkKC//EALUQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+v/aAA4EQwBNAFkASwAAPwD9/K/n/r+f+v38oooor+AOiv7/ACiiiiiv7/KK/gDooooor+AOiv7/ACiiiiiv/9k="

func TestPreprocessProducesCHWRGB(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	// The colour edge at x=16 is aligned to JPEG blocks and chroma MCUs.
	data := encodeTestJPEG(t, 32, 16, func(x, _ int) color.RGBA {
		if x < 16 {
			return red
		}
		return blue
	})
	pixels, width, height, err := Preprocess(data, 8)
	if err != nil {
		t.Fatal(err)
	}
	if width != 32 || height != 16 || len(pixels) != 3*8*8 {
		t.Fatalf("Preprocess = %d values, %dx%d", len(pixels), width, height)
	}
	const plane = 8 * 8
	at := func(channel, x, y int) float32 { return pixels[channel*plane+y*8+x] }
	if at(0, 0, 4) < 0.9 || at(1, 0, 4) > 0.1 || at(2, 0, 4) > 0.1 {
		t.Fatalf("left pixel RGB = %v %v %v", at(0, 0, 4), at(1, 0, 4), at(2, 0, 4))
	}
	if at(2, 7, 4) < 0.9 || at(0, 7, 4) > 0.1 || at(1, 7, 4) > 0.1 {
		t.Fatalf("right pixel RGB = %v %v %v", at(0, 7, 4), at(1, 7, 4), at(2, 7, 4))
	}
	for _, value := range pixels {
		if value < 0 || value > 1 {
			t.Fatalf("value %v out of [0,1]", value)
		}
	}
}

func TestPreprocessReferenceImage(t *testing.T) {
	pixels, width, height, err := Preprocess(ReferenceImage(), 640)
	if err != nil {
		t.Fatal(err)
	}
	if width != 800 || height != 533 || len(pixels) != 3*640*640 {
		t.Fatalf("Preprocess(reference) = %d values, %dx%d", len(pixels), width, height)
	}
	var sum float64
	for _, value := range pixels {
		if value < 0 || value > 1 {
			t.Fatalf("value %v out of [0,1]", value)
		}
		sum += float64(value)
	}
	if mean := sum / float64(len(pixels)); mean < 0.1 || mean > 0.9 {
		t.Fatalf("mean pixel value %.3f is implausible", mean)
	}
}

func TestPreprocessRejectsBadInput(t *testing.T) {
	if _, _, _, err := Preprocess(nil, 640); err == nil {
		t.Fatal("empty image accepted")
	}
	if _, _, _, err := Preprocess([]byte("not a jpeg"), 640); err == nil || !strings.Contains(err.Error(), "decode JPEG") {
		t.Fatalf("garbage image error = %v", err)
	}
	gray := encodeTestJPEG(t, 4, 4, func(int, int) color.RGBA { return color.RGBA{R: 128, G: 128, B: 128, A: 255} })
	if _, _, _, err := Preprocess(gray, 0); err == nil {
		t.Fatal("zero input size accepted")
	}
	if _, _, _, err := Preprocess(gray, -640); err == nil {
		t.Fatal("negative input size accepted")
	}
}

func TestPreprocessRejectsCorruptAndForeignImages(t *testing.T) {
	valid := ReferenceImage()
	// A frame that stops half way (dropped connection) must be an error, never a
	// half-grey frame that silently yields "no persons".
	if _, _, _, err := Preprocess(valid[:len(valid)/2], 640); err == nil || !strings.Contains(err.Error(), "decode JPEG") {
		t.Fatalf("truncated JPEG error = %v", err)
	}
	if _, _, _, err := Preprocess(valid[:2], 640); err == nil {
		t.Fatal("JPEG with only the SOI marker accepted")
	}
	// The worker only speaks JPEG: other formats are rejected, not decoded.
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Preprocess(pngData.Bytes(), 640); err == nil {
		t.Fatal("PNG accepted as JPEG")
	}
	// Garbage after a valid header must not panic either.
	corrupt := append([]byte(nil), valid...)
	for i := 400; i < len(corrupt) && i < 4000; i++ {
		corrupt[i] = byte(i * 31)
	}
	if _, _, _, err := Preprocess(corrupt, 64); err != nil && !strings.Contains(err.Error(), "decode JPEG") {
		t.Fatalf("corrupt JPEG error = %v", err)
	}
}

// withFrameSize rewrites the baseline SOF0 header of a Go-encoded JPEG to claim
// another frame size. The entropy data no longer matches, so decoding it would
// fail or allocate: only the header-based size check may look at such a file.
func withFrameSize(t testing.TB, data []byte, width, height int) []byte {
	t.Helper()
	marker := bytes.Index(data, []byte{0xFF, 0xC0})
	if marker < 0 {
		t.Fatal("no SOF0 marker in the test JPEG")
	}
	patched := append([]byte(nil), data...)
	copy(patched[marker+5:], []byte{byte(height >> 8), byte(height), byte(width >> 8), byte(width)})
	return patched
}

func TestPreprocessRejectsOversizedFrames(t *testing.T) {
	data := encodeTestJPEG(t, 16, 16, func(int, int) color.RGBA { return color.RGBA{R: 10, G: 20, B: 30, A: 255} })
	// The size check must reject these from the header alone, before any
	// decoder or scaler buffer exists. "not supported" (not "decode JPEG") shows
	// that Decode was never reached.
	for _, size := range [][2]int{
		{65535, 65535},
		{610, 65535}, // measured 1.39 GB in the scaler before the side limit
		{611, 65450},
		{8193, 16},
		{16, 8193},
		{8193, 8193},
		{4097, 4096}, // one column over the pixel cap
	} {
		huge := withFrameSize(t, data, size[0], size[1])
		if config, err := jpeg.DecodeConfig(bytes.NewReader(huge)); err != nil || config.Width != size[0] || config.Height != size[1] {
			t.Fatalf("patched header %v reads back as %+v, %v", size, config, err)
		}
		if _, _, _, err := Preprocess(huge, 64); err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("oversized JPEG %dx%d error = %v", size[0], size[1], err)
		}
	}
}

func TestValidateFrameSize(t *testing.T) {
	// The limits themselves are part of the memory bound: x/image/draw's scaler
	// needs destination width x source height x 32 B, the progressive decoder
	// allocates its coefficient buffers before it reads scans. 1<<24 is
	// 16,777,216 px.
	if maxFrameSide != 8192 || maxFramePixels != 1<<24 {
		t.Fatalf("limits changed: side %d, pixels %d", maxFrameSide, maxFramePixels)
	}
	for _, tc := range []struct {
		width, height int
		ok            bool
	}{
		{1, 1, true},
		{640, 480, true},
		{1280, 720, true},
		{3840, 2160, true},
		{4096, 4096, true}, // exactly maxFramePixels
		{8192, 2048, true}, // longest side and pixel cap both exactly at the limit
		{2048, 8192, true},
		{8192, 1, true},
		{1, 8192, true},
		{8193, 16, false},
		{16, 8193, false},
		{8193, 1, false},
		{1, 8193, false},
		{4097, 4096, false},
		{8192, 2049, false},
		{2049, 8192, false},
		{610, 65535, false},
		{65535, 65535, false},
		{0, 16, false},
		{16, 0, false},
		{-1, 16, false},
		{16, -1, false},
		{math.MaxInt, math.MaxInt, false},
		{math.MaxInt, 1, false},
	} {
		err := validateFrameSize(tc.width, tc.height)
		if tc.ok && err != nil {
			t.Errorf("%dx%d rejected: %v", tc.width, tc.height, err)
		}
		if !tc.ok && (err == nil || !strings.Contains(err.Error(), "not supported")) {
			t.Errorf("%dx%d error = %v", tc.width, tc.height, err)
		}
	}
	// The message states the exact limits, not rounded units.
	if err := validateFrameSize(4097, 4096); err == nil || !strings.Contains(err.Error(), "8192 px per side") || !strings.Contains(err.Error(), "16777216 px in total") {
		t.Errorf("error does not state the exact limits: %v", err)
	}
}

// realisticTestFrame is a camera-like frame: the reference photo scaled to the
// requested size with some sensor noise, so JPEG entropy coding has real work.
func realisticTestFrame(tb testing.TB, width, height int) []byte {
	tb.Helper()
	reference, err := jpeg.Decode(bytes.NewReader(ReferenceImage()))
	if err != nil {
		tb.Fatal(err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(frame, frame.Bounds(), reference, reference.Bounds(), draw.Src, nil)
	noise := rand.New(rand.NewSource(1))
	for i := 0; i < len(frame.Pix); i += 4 {
		for channel := 0; channel < 3; channel++ {
			value := int(frame.Pix[i+channel]) + noise.Intn(9) - 4
			frame.Pix[i+channel] = uint8(min(255, max(0, value)))
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, frame, &jpeg.Options{Quality: 85}); err != nil {
		tb.Fatal(err)
	}
	return buffer.Bytes()
}

// BenchmarkPreprocess is the baseline for the per-frame cost of the worker's
// hot path: a 1280x720 camera frame decoded and resized to the 640x640 model
// input. Not part of the normal test run; use
// go test ./internal/persondetect/ -run '^$' -bench BenchmarkPreprocess -benchmem
func BenchmarkPreprocess(b *testing.B) {
	data := realisticTestFrame(b, 1280, 720)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := Preprocess(data, 640); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRealisticTestFrameIsUsable(t *testing.T) {
	data := realisticTestFrame(t, 1280, 720)
	pixels, width, height, err := Preprocess(data, 64)
	if err != nil || width != 1280 || height != 720 || len(pixels) != 3*64*64 {
		t.Fatalf("Preprocess(realistic frame) = %d values, %dx%d, %v", len(pixels), width, height, err)
	}
}

func TestPreprocessGrayscaleJPEG(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			gray.SetGray(x, y, color.Gray{Y: uint8(x * 8)})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, gray, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	if config, err := jpeg.DecodeConfig(bytes.NewReader(buffer.Bytes())); err != nil || config.ColorModel != color.GrayModel {
		t.Fatalf("test JPEG is not grayscale: %+v, %v", config, err)
	}
	pixels, width, height, err := Preprocess(buffer.Bytes(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if width != 32 || height != 16 || len(pixels) != 3*8*8 {
		t.Fatalf("Preprocess(gray) = %d values, %dx%d", len(pixels), width, height)
	}
	const plane = 8 * 8
	for index := 0; index < plane; index++ {
		r, g, b := pixels[index], pixels[plane+index], pixels[2*plane+index]
		if r != g || g != b {
			t.Fatalf("grayscale pixel %d is not neutral: %v %v %v", index, r, g, b)
		}
		if r < 0 || r > 1 {
			t.Fatalf("value %v out of [0,1]", r)
		}
	}
	if left, right := pixels[4*8], pixels[4*8+7]; left > 0.1 || right < 0.8 {
		t.Fatalf("grayscale ramp lost: left %v, right %v", left, right)
	}
}

func TestPreprocessCMYKJPEG(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(cmykJPEGBase64)
	if err != nil {
		t.Fatal(err)
	}
	if config, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil || config.ColorModel != color.CMYKModel {
		t.Fatalf("test JPEG is not CMYK: %+v, %v", config, err)
	}
	pixels, width, height, err := Preprocess(data, 8)
	if err != nil {
		t.Fatal(err)
	}
	if width != 32 || height != 16 || len(pixels) != 3*8*8 {
		t.Fatalf("Preprocess(CMYK) = %d values, %dx%d", len(pixels), width, height)
	}
	const plane = 8 * 8
	at := func(channel, x, y int) float32 { return pixels[channel*plane+y*8+x] }
	if at(0, 0, 4) < 0.8 || at(1, 0, 4) > 0.2 || at(2, 0, 4) > 0.2 {
		t.Fatalf("CMYK red pixel RGB = %v %v %v", at(0, 0, 4), at(1, 0, 4), at(2, 0, 4))
	}
	if at(2, 7, 4) < 0.8 || at(0, 7, 4) > 0.2 || at(1, 7, 4) > 0.2 {
		t.Fatalf("CMYK blue pixel RGB = %v %v %v", at(0, 7, 4), at(1, 7, 4), at(2, 7, 4))
	}
	for _, value := range pixels {
		if value < 0 || value > 1 || value != value {
			t.Fatalf("value %v out of [0,1]", value)
		}
	}
}

func logit(p float64) float32 {
	return float32(math.Log(p / (1 - p)))
}

func TestDecodePersonsThresholdsSortsAndClamps(t *testing.T) {
	const queries, classes = 4, 2
	logits := make([]float32, queries*classes)
	boxes := make([]float32, queries*4)
	set := func(query int, person float64, cx, cy, w, h float32) {
		logits[query*classes] = logit(person)
		logits[query*classes+1] = logit(0.99) // another class: must not matter
		copy(boxes[query*4:], []float32{cx, cy, w, h})
	}
	set(0, 0.62, 0.5, 0.5, 0.2, 0.4)
	set(1, 0.05, 0.5, 0.5, 0.2, 0.2)
	set(2, 0.88, 0.05, 0.9, 0.2, 0.4) // x1 -0.05 and y2 1.1 are clamped
	set(3, 0.7, 1.5, 0.5, 0.2, 0.2)   // entirely right of the frame: degenerate after clamping
	got := DecodePersons(logits, boxes, queries, classes, 0, 0.5)
	if len(got) != 2 {
		t.Fatalf("DecodePersons = %+v", got)
	}
	if math.Abs(got[0].Score-0.88) > 1e-4 || math.Abs(got[1].Score-0.62) > 1e-4 {
		t.Fatalf("scores = %v, %v", got[0].Score, got[1].Score)
	}
	first := got[0]
	if first.X1 != 0 || math.Abs(first.X2-0.15) > 1e-6 || math.Abs(first.Y1-0.7) > 1e-6 || first.Y2 != 1 {
		t.Fatalf("clamped box = %+v", first)
	}
	second := got[1]
	if math.Abs(second.X1-0.4) > 1e-6 || math.Abs(second.Y1-0.3) > 1e-6 || math.Abs(second.X2-0.6) > 1e-6 || math.Abs(second.Y2-0.7) > 1e-6 {
		t.Fatalf("box = %+v", second)
	}
}

func TestDecodePersonsUsesPersonClassColumn(t *testing.T) {
	// Class 1 is the person class here; class 0 scores high but must be ignored.
	logits := []float32{logit(0.99), logit(0.2), logit(0.99), logit(0.9)}
	boxes := []float32{0.5, 0.5, 0.2, 0.2, 0.5, 0.5, 0.4, 0.4}
	got := DecodePersons(logits, boxes, 2, 2, 1, 0.5)
	if len(got) != 1 || math.Abs(got[0].Score-0.9) > 1e-4 || math.Abs(got[0].X2-got[0].X1-0.4) > 1e-6 {
		t.Fatalf("DecodePersons(personClass 1) = %+v", got)
	}
}

func TestDecodePersonsKeepsEqualScoresInQueryOrder(t *testing.T) {
	logits := []float32{logit(0.8), logit(0.8), logit(0.8)}
	boxes := []float32{0.2, 0.5, 0.1, 0.2, 0.5, 0.5, 0.1, 0.2, 0.8, 0.5, 0.1, 0.2}
	got := DecodePersons(logits, boxes, 3, 1, 0, 0.5)
	if len(got) != 3 || !(got[0].X1 < got[1].X1 && got[1].X1 < got[2].X1) {
		t.Fatalf("equal scores reordered: %+v", got)
	}
}

func TestDecodePersonsReturnsEmptyListForNoDetections(t *testing.T) {
	got := DecodePersons([]float32{logit(0.1)}, []float32{0.5, 0.5, 0.2, 0.2}, 1, 1, 0, 0.5)
	if got == nil || len(got) != 0 {
		t.Fatalf("valid input without detections = %#v, want empty non-nil list", got)
	}
}

func TestDecodePersonsHandlesBadInput(t *testing.T) {
	if got := DecodePersons(nil, nil, 300, 80, 0, 0.5); got != nil {
		t.Fatalf("short input decoded: %+v", got)
	}
	if got := DecodePersons(make([]float32, 2), make([]float32, 4), 1, 2, 2, 0.5); got != nil {
		t.Fatalf("person class out of range decoded: %+v", got)
	}
	nan := []float32{float32(math.NaN()), 0}
	if got := DecodePersons(nan, []float32{0.5, 0.5, 0.2, 0.2}, 1, 2, 0, 0.1); len(got) != 0 {
		t.Fatalf("NaN score decoded: %+v", got)
	}
}

func TestDecodePersonsRejectsLengthMismatchWithoutPanicking(t *testing.T) {
	logits := make([]float32, 4*3)
	boxes := make([]float32, 4*4)
	for i := range logits {
		logits[i] = 10
	}
	for i := 0; i < len(boxes); i += 4 {
		copy(boxes[i:], []float32{0.5, 0.5, 0.2, 0.2})
	}
	if got := DecodePersons(logits, boxes, 4, 3, 0, 0.5); len(got) != 4 {
		t.Fatalf("matching lengths decoded %d boxes", len(got))
	}
	if got := DecodePersons(logits[:11], boxes, 4, 3, 0, 0.5); got != nil {
		t.Fatalf("one logit missing decoded: %+v", got)
	}
	if got := DecodePersons(logits, boxes[:15], 4, 3, 0, 0.5); got != nil {
		t.Fatalf("one box value missing decoded: %+v", got)
	}
	if got := DecodePersons(logits, boxes, 5, 3, 0, 0.5); got != nil {
		t.Fatalf("more queries than data decoded: %+v", got)
	}
	// Extra trailing data is ignored, not mis-decoded as more queries.
	if got := DecodePersons(append(logits, 10, 10, 10), append(boxes, 0.5, 0.5, 0.2, 0.2), 4, 3, 0, 0.5); len(got) != 4 {
		t.Fatalf("trailing data decoded %d boxes", len(got))
	}
	for _, args := range [][3]int{{0, 3, 0}, {-1, 3, 0}, {4, 0, 0}, {4, -3, 0}, {4, 3, -1}, {math.MaxInt, 3, 0}, {4, math.MaxInt, 0}, {math.MaxInt, math.MaxInt, 0}} {
		if got := DecodePersons(logits, boxes, args[0], args[1], args[2], 0.5); got != nil {
			t.Fatalf("DecodePersons(queries %d, classes %d, personClass %d) = %+v", args[0], args[1], args[2], got)
		}
	}
}

func TestDecodePersonsDropsNonFiniteValues(t *testing.T) {
	inf, nan := float32(math.Inf(1)), float32(math.NaN())
	const queries = 8
	logits := make([]float32, queries)
	boxes := make([]float32, queries*4)
	for query := range logits {
		logits[query] = 5
		copy(boxes[query*4:], []float32{0.5, 0.5, 0.2, 0.2})
	}
	logits[0] = inf  // sigmoid(+Inf) = 1 would pass every threshold
	logits[1] = -inf // score 0
	logits[2] = nan
	boxes[3*4] = nan
	boxes[4*4+1] = inf
	boxes[5*4+2] = -inf
	boxes[6*4+3] = nan
	// Query 7 is valid and has its own box, so the test proves that non-finite
	// inputs are the only reason for the other drops.
	copy(boxes[7*4:], []float32{0.7, 0.5, 0.2, 0.2})
	for _, minScore := range []float64{0.5, 0, -1} {
		got := DecodePersons(logits, boxes, queries, 1, 0, minScore)
		if len(got) != 1 {
			t.Fatalf("minScore %v: DecodePersons = %+v, want only query 7", minScore, got)
		}
		if math.Abs(got[0].X1-0.6) > 1e-6 {
			t.Fatalf("minScore %v: surviving box %+v is not query 7", minScore, got[0])
		}
		for _, box := range got {
			for _, value := range []float64{box.X1, box.Y1, box.X2, box.Y2, box.Score} {
				if !finite(value) {
					t.Fatalf("non-finite value in %+v", box)
				}
			}
		}
	}
	if got := DecodePersons(logits, boxes, queries, 1, 0, math.NaN()); len(got) != 0 {
		t.Fatalf("NaN threshold decoded %+v", got)
	}
}

func TestDecodePersonsDropsDegenerateBoxes(t *testing.T) {
	const queries = 8
	logits := make([]float32, queries)
	for query := range logits {
		logits[query] = 5
	}
	boxes := []float32{
		0.5, 0.5, 0, 0.2, // zero width
		0.5, 0.5, 0.2, 0, // zero height
		0.5, 0.5, -0.2, 0.2, // negative width
		0.5, 0.5, 0.2, -0.2, // negative height
		0.5, 0.5, -0.2, -0.2, // both negative
		-0.5, 0.5, 0.2, 0.2, // fully left of the frame
		0.5, 1.5, 0.2, 0.2, // fully below the frame
		0.5, 0.5, 1e30, 1e30, // huge but valid: covers the frame
	}
	got := DecodePersons(logits, boxes, queries, 1, 0, 0.5)
	if len(got) != 1 {
		t.Fatalf("DecodePersons = %+v, want only the frame-covering box", got)
	}
	if got[0].X1 != 0 || got[0].Y1 != 0 || got[0].X2 != 1 || got[0].Y2 != 1 {
		t.Fatalf("huge box not clamped to the frame: %+v", got[0])
	}
	// A box that only touches the frame edge has no area and is dropped as well.
	edge := DecodePersons([]float32{5}, []float32{1.1, 0.5, 0.2, 0.2}, 1, 1, 0, 0.5)
	if len(edge) != 0 {
		t.Fatalf("zero-area edge box decoded: %+v", edge)
	}
}

func TestIoU(t *testing.T) {
	a := Box{X1: 0, Y1: 0, X2: 0.5, Y2: 0.5}
	if v := IoU(a, a); math.Abs(v-1) > 1e-12 {
		t.Fatalf("IoU(a, a) = %v", v)
	}
	if v := IoU(a, Box{X1: 0.6, Y1: 0.6, X2: 0.9, Y2: 0.9}); v != 0 {
		t.Fatalf("disjoint IoU = %v", v)
	}
	b := Box{X1: 0.25, Y1: 0, X2: 0.75, Y2: 0.5}
	if v := IoU(a, b); math.Abs(v-1.0/3) > 1e-12 {
		t.Fatalf("half-overlap IoU = %v", v)
	}
	if v := IoU(Box{}, Box{}); v != 0 {
		t.Fatalf("empty IoU = %v", v)
	}
}

func TestIoUIsSafeForDegenerateBoxes(t *testing.T) {
	a := Box{X1: 0, Y1: 0, X2: 0.5, Y2: 0.5}
	nan, inf := math.NaN(), math.Inf(1)
	inverted := Box{X1: 0.5, Y1: 0.5, X2: 0, Y2: 0}
	for name, other := range map[string]Box{
		"NaN":      {X1: nan, Y1: 0, X2: 0.5, Y2: 0.5},
		"Inf":      {X1: 0, Y1: 0, X2: inf, Y2: inf},
		"inverted": inverted,
	} {
		for _, value := range []float64{IoU(a, other), IoU(other, a), IoU(other, other)} {
			if value != value || value < 0 || value > 1 {
				t.Fatalf("IoU with %s box = %v", name, value)
			}
		}
	}
}

func TestSameDetections(t *testing.T) {
	a := []Box{
		{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.5, Score: 0.9},
		{X1: 0.6, Y1: 0.2, X2: 0.7, Y2: 0.6, Score: 0.6},
	}
	if ok, reason := SameDetections(a, []Box{a[1], a[0]}, 0.9, 0.02); !ok || reason != "min IoU 1.0000, max score diff 0.0000" {
		t.Fatalf("reordered = %v, %q", ok, reason)
	}
	if ok, reason := SameDetections(a, a[:1], 0.9, 0.02); ok || reason != "count 2 vs 1" {
		t.Fatalf("count mismatch = %v, %q", ok, reason)
	}
	moved := []Box{a[0], {X1: 0.62, Y1: 0.2, X2: 0.72, Y2: 0.6, Score: 0.6}}
	if ok, reason := SameDetections(a, moved, 0.9, 0.02); ok || !strings.HasPrefix(reason, "min IoU 0.6") {
		t.Fatalf("moved box = %v, %q", ok, reason)
	}
	rescored := []Box{a[0], {X1: 0.6, Y1: 0.2, X2: 0.7, Y2: 0.6, Score: 0.65}}
	if ok, _ := SameDetections(a, rescored, 0.9, 0.02); ok {
		t.Fatal("score difference 0.05 accepted")
	}
	if ok, _ := SameDetections(nil, nil, 0.9, 0.02); !ok {
		t.Fatal("two empty sets differ")
	}
}

func TestSameDetectionsEmptySets(t *testing.T) {
	for name, pair := range map[string][2][]Box{
		"nil/nil":     {nil, nil},
		"empty/empty": {{}, {}},
		"nil/empty":   {nil, {}},
	} {
		ok, reason := SameDetections(pair[0], pair[1], 0.9, 0.02)
		if !ok || reason != "no detections in either set" {
			t.Fatalf("%s = %v, %q", name, ok, reason)
		}
	}
	one := []Box{{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.5, Score: 0.9}}
	if ok, reason := SameDetections(nil, one, 0.9, 0.02); ok || reason != "count 0 vs 1" {
		t.Fatalf("empty vs one = %v, %q", ok, reason)
	}
	if ok, reason := SameDetections(one, nil, 0.9, 0.02); ok || reason != "count 1 vs 0" {
		t.Fatalf("one vs empty = %v, %q", ok, reason)
	}
}

func TestSameDetectionsMatchesEachBoxOnlyOnce(t *testing.T) {
	box := Box{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.5, Score: 0.9}
	other := Box{X1: 0.6, Y1: 0.2, X2: 0.7, Y2: 0.6, Score: 0.9}
	// Two identical boxes on one side may not both claim the single match of the other side.
	ok, reason := SameDetections([]Box{box, box}, []Box{box, other}, 0.9, 0.02)
	if ok || !strings.HasPrefix(reason, "min IoU 0.0000") {
		t.Fatalf("double match = %v, %q", ok, reason)
	}
	// Identical boxes with identical scores on both sides match one to one.
	if ok, reason := SameDetections([]Box{box, box}, []Box{box, box}, 0.9, 0.02); !ok {
		t.Fatalf("duplicate boxes = %v, %q", ok, reason)
	}
}

func TestSameDetectionsToleratesNonFiniteBoxes(t *testing.T) {
	good := []Box{{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.5, Score: 0.9}}
	bad := []Box{{X1: math.NaN(), Y1: 0.1, X2: 0.3, Y2: 0.5, Score: 0.9}}
	if ok, reason := SameDetections(good, bad, 0.9, 0.02); ok {
		t.Fatalf("NaN box accepted: %q", reason)
	}
	badScore := []Box{{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.5, Score: math.NaN()}}
	if ok, reason := SameDetections(good, badScore, 0.9, 0.02); ok {
		t.Fatalf("NaN score accepted: %q", reason)
	}
}

// stripe is a tall box of width 0.2 whose left edge is at x. Two stripes with
// the left edges x and x+s have IoU (0.2-s)/(0.2+s), so IoU >= 0.9 needs s <=
// 0.0105.
func stripe(x, score float64) Box {
	return Box{X1: x, Y1: 0.2, X2: x + 0.2, Y2: 0.8, Score: score}
}

// The comparison parameters of the WebGPU self-test: boxes are decoded at
// threshold-margin = 0.28 and boxes with score >= threshold+margin = 0.32 are
// confident. The variables keep the sums at run time exactly like the
// implementation computes them.
var (
	confThreshold = 0.3
	confMaxDiff   = 0.02
	confMinIoU    = 0.9
)

func sameConfident(a, b []Box) (bool, string) {
	return SameConfidentDetections(a, b, confThreshold, confMinIoU, confMaxDiff)
}

// sameConfidentBothWays checks that the verdict does not depend on the
// argument order and returns it with both reasons.
func sameConfidentBothWays(t *testing.T, a, b []Box) (bool, string, string) {
	t.Helper()
	forward, forwardReason := sameConfident(a, b)
	backward, backwardReason := sameConfident(b, a)
	if forward != backward {
		t.Fatalf("verdict depends on argument order: (a,b) = %v %q, (b,a) = %v %q", forward, forwardReason, backward, backwardReason)
	}
	return forward, forwardReason, backwardReason
}

func TestSameConfidentDetectionsAcceptsNearDuplicateSwap(t *testing.T) {
	// Two near-identical boxes (IoU 0.956) whose scores are swapped between the
	// sets: only the crossed pairing is valid. Greedy best-IoU matching pairs the
	// first box with its geometric twin and fails.
	a := []Box{stripe(0.4000, 0.80), stripe(0.4045, 0.55)}
	b := []Box{stripe(0.4000, 0.55), stripe(0.4045, 0.80)}
	if iou := IoU(a[0], a[1]); iou < 0.95 || iou > 0.96 {
		t.Fatalf("test boxes have IoU %v", iou)
	}
	if ok, reason := SameDetections(a, b, confMinIoU, confMaxDiff); ok {
		t.Fatalf("greedy SameDetections unexpectedly accepts the swap: %q", reason)
	}
	ok, forward, backward := sameConfidentBothWays(t, a, b)
	if !ok {
		t.Fatalf("swap rejected: %q / %q", forward, backward)
	}
	for _, reason := range []string{forward, backward} {
		if !strings.HasPrefix(reason, "matched 2 confident detections (min IoU 0.95") || !strings.HasSuffix(reason, "max score diff 0.0000)") {
			t.Fatalf("reason = %q", reason)
		}
	}
}

func TestSameConfidentDetectionsUsesAugmentingPaths(t *testing.T) {
	// a1 prefers b1 (IoU 1) but can also use b2; a2 can only use b1. A matching
	// that keeps a1-b1 strands a2; the augmenting path re-routes a1 to b2.
	a := []Box{stripe(0.400, 0.8), stripe(0.408, 0.8)}
	b := []Box{stripe(0.400, 0.8), stripe(0.390, 0.8)}
	if IoU(a[0], b[0]) != 1 || IoU(a[0], b[1]) < confMinIoU || IoU(a[1], b[0]) < confMinIoU || IoU(a[1], b[1]) >= confMinIoU {
		t.Fatalf("test geometry is off: %v %v %v %v", IoU(a[0], b[0]), IoU(a[0], b[1]), IoU(a[1], b[0]), IoU(a[1], b[1]))
	}
	if ok, _ := SameDetections(a, b, confMinIoU, confMaxDiff); ok {
		t.Fatal("greedy SameDetections unexpectedly accepts the chain")
	}
	ok, forward, backward := sameConfidentBothWays(t, a, b)
	if !ok {
		t.Fatalf("chain rejected: %q / %q", forward, backward)
	}
	// The statistics cover the pairs of the final matchings; the weakest pair is the one forced onto b2.
	for _, reason := range []string{forward, backward} {
		if !strings.HasPrefix(reason, "matched 2 confident detections (min IoU 0.90") {
			t.Fatalf("reason = %q", reason)
		}
	}
}

func TestSameConfidentDetectionsIgnoresTheThresholdBand(t *testing.T) {
	for name, pair := range map[string][2][]Box{
		"band vs band":      {{stripe(0.4, 0.305)}, {stripe(0.4, 0.295)}},
		"band vs nothing":   {{stripe(0.4, 0.305)}, nil},
		"edge of the band":  {{stripe(0.4, 0.319)}, {stripe(0.4, 0.281)}},
		"band box far away": {{stripe(0.1, 0.31)}, {stripe(0.7, 0.29)}},
	} {
		ok, forward, backward := sameConfidentBothWays(t, pair[0], pair[1])
		if !ok || forward != "no confident detections in either set" || backward != forward {
			t.Errorf("%s = %v, %q, %q", name, ok, forward, backward)
		}
	}
	// An unmatched band box next to a matched confident box is fine.
	a := []Box{stripe(0.4, 0.80), stripe(0.1, 0.31)}
	b := []Box{stripe(0.4, 0.79)}
	if ok, forward, backward := sameConfidentBothWays(t, a, b); !ok || !strings.HasPrefix(forward, "matched 1 confident detection (") || !strings.HasPrefix(backward, "matched 1 confident detection (") {
		t.Fatalf("band box next to a match = %v, %q, %q", ok, forward, backward)
	}
	// A confident box may be matched by a band box of the other set: the
	// partner is not required to be confident itself.
	ok, forward, backward := sameConfidentBothWays(t, []Box{stripe(0.4, 0.322)}, []Box{stripe(0.4, 0.305)})
	if !ok || !strings.HasPrefix(forward, "matched 1 confident detection in a and 0 in b (") || !strings.HasPrefix(backward, "matched 0 confident detections in a and 1 in b (") {
		t.Fatalf("confident vs band partner = %v, %q, %q", ok, forward, backward)
	}
	// ... but only within the score margin.
	if ok, reason, _ := sameConfidentBothWays(t, []Box{stripe(0.4, 0.34)}, []Box{stripe(0.4, 0.31)}); ok {
		t.Fatalf("confident 0.34 matched by 0.31: %q", reason)
	}
}

func TestSameConfidentDetectionsConfidenceCutoffIsThresholdPlusMargin(t *testing.T) {
	cutoff := confThreshold + confMaxDiff
	if ok, reason := sameConfident([]Box{stripe(0.4, cutoff)}, nil); ok || !strings.Contains(reason, "set a") {
		t.Fatalf("box exactly at the cutoff is not required: %v %q", ok, reason)
	}
	if ok, reason := sameConfident([]Box{stripe(0.4, math.Nextafter(cutoff, 0))}, nil); !ok || reason != "no confident detections in either set" {
		t.Fatalf("box just below the cutoff is required: %v %q", ok, reason)
	}
}

func TestSameConfidentDetectionsEmptySets(t *testing.T) {
	for name, pair := range map[string][2][]Box{
		"nil/nil":     {nil, nil},
		"empty/empty": {{}, {}},
		"nil/empty":   {nil, {}},
	} {
		ok, reason := sameConfident(pair[0], pair[1])
		if !ok || reason != "no confident detections in either set" {
			t.Errorf("%s = %v, %q", name, ok, reason)
		}
	}
}

func TestSameConfidentDetectionsNamesTheUnmatchedBox(t *testing.T) {
	p, q := stripe(0.10, 0.90), stripe(0.60, 0.6)
	ok, reason := sameConfident([]Box{p, q}, []Box{stripe(0.10, 0.90)})
	if ok || !strings.Contains(reason, "set a") || !strings.Contains(reason, "score 0.6000") || !strings.Contains(reason, "no match in set b") {
		t.Fatalf("missing confident box (a to b) = %v, %q", ok, reason)
	}
	ok, reason = sameConfident([]Box{stripe(0.10, 0.90)}, []Box{p, q})
	if ok || !strings.Contains(reason, "set b") || !strings.Contains(reason, "score 0.6000") || !strings.Contains(reason, "no match in set a") {
		t.Fatalf("missing confident box (b to a) = %v, %q", ok, reason)
	}
	ok, reason = sameConfident([]Box{p}, nil)
	if ok || !strings.Contains(reason, "set a") || !strings.Contains(reason, "set b is empty") {
		t.Fatalf("confident box against an empty set = %v, %q", ok, reason)
	}
	// The first unmatched confident box in input order is the one named.
	ok, reason = sameConfident([]Box{stripe(0.10, 0.9), stripe(0.4, 0.8), stripe(0.7, 0.7)}, []Box{stripe(0.10, 0.9)})
	if ok || !strings.Contains(reason, "score 0.8000") {
		t.Fatalf("first unmatched confident box = %v, %q", ok, reason)
	}
}

func TestSameConfidentDetectionsRejectsScoreAndBoxMismatch(t *testing.T) {
	ok, reason := sameConfident([]Box{stripe(0.4, 0.80)}, []Box{stripe(0.4, 0.75)})
	if ok || !strings.Contains(reason, "set a") || !strings.Contains(reason, "score 0.8000") || !strings.Contains(reason, "score diff 0.0500") {
		t.Fatalf("score diff 0.05 = %v, %q", ok, reason)
	}
	// Exactly the margin is fine (0.75 and 0.5 are exact in binary).
	if ok, reason := SameConfidentDetections([]Box{stripe(0.4, 0.75)}, []Box{stripe(0.4, 0.5)}, 0.3, 0.9, 0.25); !ok {
		t.Fatalf("score diff exactly at the margin rejected: %q", reason)
	}
	ok, reason = sameConfident([]Box{stripe(0.40, 0.8)}, []Box{stripe(0.43, 0.8)})
	if ok || !strings.Contains(reason, "best candidate IoU 0.7") {
		t.Fatalf("shifted box = %v, %q", ok, reason)
	}
}

func TestSameConfidentDetectionsNeedsDistinctMatches(t *testing.T) {
	one := []Box{stripe(0.4, 0.8)}
	two := []Box{stripe(0.4, 0.8), stripe(0.4, 0.8)}
	if ok, reason, _ := sameConfidentBothWays(t, two, one); ok || !strings.Contains(reason, "no match") {
		t.Fatalf("two confident boxes sharing one partner = %v, %q", ok, reason)
	}
	if ok, reason, _ := sameConfidentBothWays(t, two, two); !ok || !strings.HasPrefix(reason, "matched 2 confident detections (") {
		t.Fatalf("duplicates on both sides = %v, %q", ok, reason)
	}
	// The band box of the other set is no partner for the second confident box (score gap).
	if ok, _, _ := sameConfidentBothWays(t, two, []Box{stripe(0.4, 0.8), stripe(0.4, 0.31)}); ok {
		t.Fatal("0.8 + 0.8 accepted against 0.8 + 0.31")
	}
}

func TestSameConfidentDetectionsRejectsNonFiniteInput(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	good := []Box{stripe(0.4, 0.8)}
	for name, bad := range map[string]Box{
		"NaN score":   stripe(0.4, nan),
		"Inf score":   stripe(0.4, inf),
		"NaN X1":      {X1: nan, Y1: 0.2, X2: 0.6, Y2: 0.8, Score: 0.8},
		"Inf Y2":      {X1: 0.4, Y1: 0.2, X2: 0.6, Y2: inf, Score: 0.8},
		"-Inf X2":     {X1: 0.4, Y1: 0.2, X2: -inf, Y2: 0.8, Score: 0.8},
		"NaN in band": {X1: 0.4, Y1: nan, X2: 0.6, Y2: 0.8, Score: 0.3},
	} {
		for _, pair := range [][2][]Box{{good, {bad}}, {{bad}, good}, {{bad}, {bad}}, {{bad}, nil}} {
			if ok, reason := sameConfident(pair[0], pair[1]); ok || !strings.Contains(reason, "non-finite") {
				t.Errorf("%s: sets %v / %v = %v, %q", name, pair[0], pair[1], ok, reason)
			}
		}
	}
	for name, params := range map[string][3]float64{
		"NaN threshold":    {nan, 0.9, 0.02},
		"NaN minIoU":       {0.3, nan, 0.02},
		"NaN margin":       {0.3, 0.9, nan},
		"Inf threshold":    {inf, 0.9, 0.02},
		"-Inf threshold":   {-inf, 0.9, 0.02},
		"Inf margin":       {0.3, 0.9, inf},
		"negative diff":    {0.3, 0.9, -0.02},
		"Inf minIoU":       {0.3, inf, 0.02},
		"-Inf minIoU":      {0.3, -inf, 0.02},
		"zero minIoU":      {0.3, 0, 0.02},
		"negative minIoU":  {0.3, -0.5, 0.02},
		"minIoU above one": {0.3, 1.0000001, 0.02},
		"minIoU of 1.5":    {0.3, 1.5, 0.02},
		"overflowing cut":  {math.MaxFloat64, 0.9, math.MaxFloat64},
	} {
		// Vacuous truth must never leak through bad parameters: even two empty sets fail.
		for _, sets := range [][2][]Box{{good, good}, {nil, nil}} {
			if ok, reason := SameConfidentDetections(sets[0], sets[1], params[0], params[1], params[2]); ok || !strings.Contains(reason, "invalid") {
				t.Errorf("%s: %v / %v = %v, %q", name, sets[0], sets[1], ok, reason)
			}
		}
	}
}

func TestSameConfidentDetectionsAcceptsMinIoUInRange(t *testing.T) {
	box := []Box{stripe(0.4, 0.8)}
	// minIoU 1 is the strictest valid value: identical boxes still match exactly.
	for _, minIoU := range []float64{1e-9, 0.5, 0.9, 1} {
		if ok, reason := SameConfidentDetections(box, box, confThreshold, minIoU, confMaxDiff); !ok || !strings.HasPrefix(reason, "matched 1 confident detection (") {
			t.Errorf("minIoU %v rejected identical boxes: %q", minIoU, reason)
		}
	}
	if ok, _ := SameConfidentDetections(box, []Box{stripe(0.401, 0.8)}, confThreshold, 1, confMaxDiff); ok {
		t.Error("minIoU 1 accepted shifted boxes")
	}
}

func TestSameConfidentDetectionsDoesNotModifyInput(t *testing.T) {
	a := []Box{stripe(0.4045, 0.55), stripe(0.4000, 0.80)}
	b := []Box{stripe(0.4000, 0.55), stripe(0.4045, 0.80)}
	wantA, wantB := append([]Box(nil), a...), append([]Box(nil), b...)
	sameConfident(a, b)
	sameConfident(b, a)
	for i := range a {
		if a[i] != wantA[i] || b[i] != wantB[i] {
			t.Fatalf("input reordered or changed: a=%v b=%v", a, b)
		}
	}
}

// coversByBruteForce is an independent oracle: can every confident box of
// from be assigned a distinct box of to under the edge rule?
func coversByBruteForce(from, to []Box) bool {
	var confident []Box
	for _, box := range from {
		if box.Score >= confThreshold+confMaxDiff {
			confident = append(confident, box)
		}
	}
	used := make([]bool, len(to))
	var assign func(index int) bool
	assign = func(index int) bool {
		if index == len(confident) {
			return true
		}
		for j, other := range to {
			if used[j] || IoU(confident[index], other) < confMinIoU || math.Abs(confident[index].Score-other.Score) > confMaxDiff {
				continue
			}
			used[j] = true
			if assign(index + 1) {
				return true
			}
			used[j] = false
		}
		return false
	}
	return assign(0)
}

func TestSameConfidentDetectionsMatchesBruteForce(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	scores := []float64{0.281, 0.295, 0.305, 0.319, 0.321, 0.335, 0.34, 0.5, 0.55, 0.8}
	randomSet := func() []Box {
		set := make([]Box, random.Intn(6))
		for i := range set {
			// Few positions at 0.004 spacing: many pairs hover around IoU 0.9, and the
			// score list has pairs just inside and just outside the 0.02 margin.
			set[i] = stripe(0.4+0.004*float64(random.Intn(6)), scores[random.Intn(len(scores))])
		}
		return set
	}
	accepted := 0
	for iteration := 0; iteration < 5000; iteration++ {
		a, b := randomSet(), randomSet()
		want := coversByBruteForce(a, b) && coversByBruteForce(b, a)
		got, reason := sameConfident(a, b)
		if got != want {
			t.Fatalf("a=%v b=%v: got %v (%q), brute force %v", a, b, got, reason, want)
		}
		if reverse, _ := sameConfident(b, a); reverse != got {
			t.Fatalf("a=%v b=%v: verdict depends on argument order", a, b)
		}
		if got {
			accepted++
		}
	}
	if accepted < 500 || accepted > 4500 {
		t.Fatalf("random sets are degenerate: %d of 5000 accepted", accepted)
	}
}
