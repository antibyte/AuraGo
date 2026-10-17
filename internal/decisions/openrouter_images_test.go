package decisions

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"net/http"
	"testing"
)

func decisionTestImage(t *testing.T, w, h int, alpha bool) Image {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, w, h))
	random := rand.New(rand.NewPCG(1, 2))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if alpha {
				a = uint8(random.Uint32())
			}
			picture.SetNRGBA(x, y, color.NRGBA{uint8(random.Uint32()), uint8(random.Uint32()), uint8(random.Uint32()), a})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, picture); err != nil {
		t.Fatal(err)
	}
	return Image{Data: output.Bytes(), MIME: "image/png"}
}

func TestDecisionImageShippingPreservesOriginalAndTransparency(t *testing.T) {
	for _, alpha := range []bool{false, true} {
		t.Run(map[bool]string{false: "opaque", true: "transparent"}[alpha], func(t *testing.T) {
			source := decisionTestImage(t, 500, 420, alpha)
			original := bytes.Clone(source.Data)
			got, adjustment, err := prepareDecisionImage(context.Background(), source, decisionMediaLimits("unknown"), 64<<10, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Data) > 64<<10 || !bytes.Equal(original, source.Data) || adjustment.OriginalWidth != 500 || adjustment.Width > 500 || adjustment.Height > 420 {
				t.Fatalf("invalid adjustment: %+v", adjustment)
			}
			if alpha && got.MIME != "image/png" {
				t.Fatal("transparency discarded")
			}
			decoded, _, err := image.Decode(bytes.NewReader(got.Data))
			if err != nil {
				t.Fatal(err)
			}
			if alpha && decoded.(interface{ Opaque() bool }).Opaque() {
				t.Fatal("image became opaque")
			}
		})
	}
	source := decisionTestImage(t, 20, 20, false)
	got, _, err := prepareDecisionImage(context.Background(), source, decisionMediaLimits("unknown"), len(source.Data), 0)
	if err != nil || !bytes.Equal(got.Data, source.Data) {
		t.Fatal("fitting image was re-encoded")
	}
}

func TestDecisionImageOrientationAndInvalidInputs(t *testing.T) {
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 40, 20)), nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(exif)+2))
	segment = append(segment, exif...)
	source := Image{Data: append(append(bytes.Clone(jpegBytes.Bytes()[:2]), segment...), jpegBytes.Bytes()[2:]...), MIME: "image/jpeg"}
	got, adjustment, err := prepareDecisionImage(context.Background(), source, decisionMediaLimits("unknown"), 10000, 0)
	if err != nil || adjustment.Width != 20 || adjustment.Height != 40 || jpegOrientation(got.Data) != 1 {
		t.Fatalf("orientation: %+v %v", adjustment, err)
	}
	for _, input := range []Image{{Data: []byte("broken"), MIME: "image/png"}, {Data: source.Data[:len(source.Data)/2], MIME: "image/jpeg"}} {
		if _, _, err := prepareDecisionImage(context.Background(), input, decisionMediaLimits("unknown"), 10000, 0); ErrorCode(err) != "image_invalid" {
			t.Fatalf("invalid image accepted: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := prepareDecisionImage(ctx, source, decisionMediaLimits("unknown"), 10000, 0); ErrorCode(err) != CodeCancelled {
		t.Fatalf("cancel: %v", err)
	}
	if _, _, err := prepareDecisionImage(context.Background(), source, decisionMediaLimits("typesafe/jev-1.14"), 10000, 0); ErrorCode(err) != "image_minimum_unmet" {
		t.Fatalf("undersize Jev: %v", err)
	}
	for orientation := 1; orientation <= 8; orientation++ {
		input := image.NewNRGBA(image.Rect(0, 0, 3, 2))
		input.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
		rotated := orientDecisionImage(input, orientation)
		expected := [][2]int{{0, 0}, {0, 0}, {2, 0}, {2, 1}, {0, 1}, {0, 0}, {1, 0}, {1, 2}, {0, 2}}[orientation]
		r, _, _, _ := rotated.At(expected[0], expected[1]).RGBA()
		if r != 65535 {
			t.Fatalf("orientation %d lost corner", orientation)
		}
	}
}

func TestDecisionMediaProfilesAndRequestBudgets(t *testing.T) {
	for _, tc := range []struct {
		id           string
		count, bytes int
	}{{"unknown", 4, 3 << 20}, {"cloudflare/clef", 4, 3 << 20}, {"liquid/d1-ultra", 8, 4500000}, {"typesafe/jev-1.14", 8, 16 << 20}, {"inception/mercury-decide", 8, 10000000}, {"openai/gpt-6-luna-decisions", 8, 3 << 20}} {
		limits := decisionMediaLimits(tc.id)
		if limits.MaxImages != tc.count || limits.MaxRequestBytes != tc.bytes {
			t.Fatal(tc.id, limits)
		}
	}
	questions := map[string]openRouterQuestion{"q": {Type: "noul"}}
	source := decisionTestImage(t, 1200, 800, false)
	request := Request{State: "inspect", Images: []Image{source, source}}
	model := DecisionModel{ID: "liquid/d1", ContextLength: 1800}
	body, adjustments, err := prepareOpenRouterRequest(context.Background(), model, request, questions)
	if err != nil || len(adjustments) != 2 || len(body) > 4500000 {
		t.Fatalf("request budget: %d %+v %v", len(body), adjustments, err)
	}
	var patches int64
	for _, a := range adjustments {
		patches += imagePatches(a.Width, a.Height)
	}
	if patches > 10000 || patches*3/2 > 1800 {
		t.Fatal("Liquid patch/context limit exceeded")
	}
	var wire openRouterDecisionRequest
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.State.([]any)) != 3 {
		t.Fatal("image was removed")
	}
	request.Images = append(request.Images, source, source, source)
	if _, _, err := prepareOpenRouterRequest(context.Background(), DecisionModel{ID: "unknown", ContextLength: 65536}, request, questions); ErrorCode(err) != "image_limit_exceeded" {
		t.Fatal("image count not bounded")
	}
}

func TestDecisionImagesKeepUnequalOriginalsWhenCombinedBudgetFits(t *testing.T) {
	var solid bytes.Buffer
	if err := png.Encode(&solid, image.NewRGBA(image.Rect(0, 0, 1600, 800))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		model string
		large Image
	}{
		{"unknown", decisionTestImage(t, 40, 40, false)},
		{"liquid/d1", Image{Data: solid.Bytes(), MIME: "image/png"}},
	} {
		t.Run(tc.model, func(t *testing.T) {
			request := Request{State: "inspect", Images: []Image{tc.large, decisionTestImage(t, 4, 4, false)}}
			_, adjustments, err := prepareOpenRouterRequest(context.Background(), DecisionModel{ID: tc.model, ContextLength: 2600}, request, map[string]openRouterQuestion{"q": {Type: "noul"}})
			if err != nil || len(adjustments) != 0 {
				t.Fatalf("fitting originals were changed: %+v %v", adjustments, err)
			}
		})
	}
}

func TestOpenRouterImageAdjustmentsReachResponseAfterOnePost(t *testing.T) {
	source := decisionTestImage(t, 500, 420, false)
	original := bytes.Clone(source.Data)
	posts := 0
	client := openRouterTestClient(t, "fixture-key", func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			return openRouterHTTPResponse(http.StatusOK, `{"data":[{"id":"vision/decision","context_length":8192,"architecture":{"input_modalities":["text","image"],"output_modalities":["decisions"]}}]}`, nil), nil
		}
		posts++
		if request.ContentLength > 3<<20 {
			t.Fatal("wire request exceeds limit")
		}
		return openRouterHTTPResponse(http.StatusOK, `{"answers":{"pick":{"choice":"a","probabilities":{"a":0.8,"b":0.2},"type":"choice"}},"model":"vision/decision-v1","usage":{"cost":0.01,"input_tokens":10,"output_tokens":1}}`, nil), nil
	})
	result, err := client.Evaluate(context.Background(), "vision/decision", Request{State: "inspect", Images: []Image{source}, Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}}})
	if err != nil || posts != 1 || len(result.ImageAdjustments) != 1 || result.Usage.CostUSD == nil || *result.Usage.CostUSD != .01 || !bytes.Equal(source.Data, original) {
		t.Fatalf("image metadata/cost/original: %+v posts=%d err=%v", result, posts, err)
	}
	adjustment := result.ImageAdjustments[0]
	if adjustment.Index != 0 || adjustment.OriginalBytes != len(original) || adjustment.Bytes >= len(original) || adjustment.Width > 500 || adjustment.Height > 420 {
		t.Fatalf("invalid adjustment: %+v", adjustment)
	}
}
