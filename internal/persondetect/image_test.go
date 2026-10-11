package persondetect

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"testing"
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

func TestPreprocessRejectsOversizedFrames(t *testing.T) {
	data := encodeTestJPEG(t, 16, 16, func(int, int) color.RGBA { return color.RGBA{R: 10, G: 20, B: 30, A: 255} })
	// Rewrite the baseline SOF0 header to claim a 65535x65535 frame: the size
	// check must reject it from the header alone, before any pixel buffer exists.
	marker := bytes.Index(data, []byte{0xFF, 0xC0})
	if marker < 0 {
		t.Fatal("no SOF0 marker in the test JPEG")
	}
	huge := append([]byte(nil), data...)
	copy(huge[marker+5:], []byte{0xFF, 0xFF, 0xFF, 0xFF}) // height, width
	if _, _, _, err := Preprocess(huge, 64); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("oversized JPEG error = %v", err)
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
