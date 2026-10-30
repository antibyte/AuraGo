package server

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"aurago/internal/decisions"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// normalizeDecisionImage is the single validation path for every image that
// reaches Decisions: format sniffing, pixel limit, and GIF/WebP -> one PNG
// frame. runtimeRemaining bounds the resulting bytes.
func normalizeDecisionImage(data []byte, runtimeRemaining int64) (decisions.Image, error) {
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return decisions.Image{}, errDecisionImage
	}
	size, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || size.Width <= 0 || size.Height <= 0 || int64(size.Width)*int64(size.Height) > decisionImagePixels {
		return decisions.Image{}, errDecisionImage
	}
	if mime == "image/webp" || mime == "image/gif" {
		// GIF and WebP are reduced to one PNG frame because the runtime decoder is not the animated path.
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return decisions.Image{}, errDecisionImage
		}
		encoded := decisionImageBuffer{limit: min(decisionImageBytes, int(runtimeRemaining))}
		if err := png.Encode(&encoded, decoded); err != nil {
			return decisions.Image{}, errDecisionImage
		}
		data, mime = encoded.Bytes(), "image/png"
	}
	if int64(len(data)) > runtimeRemaining {
		return decisions.Image{}, errDecisionImage
	}
	return decisions.Image{Data: data, MIME: mime}, nil
}

var consumerImageSlot = make(chan struct{}, 1)

// prepareConsumerImage validates like the HTTP path and then bounds the longer
// side to maxSide (never enlarging). Photos become JPEG q85; images with alpha
// stay PNG. Pictures already small enough are returned unchanged.
func prepareConsumerImage(data []byte, maxSide int) (decisions.Image, error) {
	if len(data) > decisionImageBytes || maxSide <= 0 {
		return decisions.Image{}, errDecisionImage
	}
	// Full decodes of large pictures are memory heavy; one at a time.
	consumerImageSlot <- struct{}{}
	defer func() { <-consumerImageSlot }()
	normalized, err := normalizeDecisionImage(data, decisionImageBytes)
	if err != nil {
		return decisions.Image{}, err
	}
	size, _, err := image.DecodeConfig(bytes.NewReader(normalized.Data))
	if err != nil {
		return decisions.Image{}, errDecisionImage
	}
	if max(size.Width, size.Height) <= maxSide {
		return normalized, nil
	}
	source, _, err := image.Decode(bytes.NewReader(normalized.Data))
	if err != nil {
		return decisions.Image{}, errDecisionImage
	}
	if normalized.MIME == "image/jpeg" {
		source = decisions.OrientImage(source, normalized.Data)
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scale := float64(maxSide) / float64(max(width, height))
	width, height = max(1, int(float64(width)*scale+.5)), max(1, int(float64(height)*scale+.5))
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), source, bounds, draw.Src, nil)
	var out bytes.Buffer
	if scaled.Opaque() {
		if err := jpeg.Encode(&out, scaled, &jpeg.Options{Quality: 85}); err != nil {
			return decisions.Image{}, errDecisionImage
		}
		return decisions.Image{Data: out.Bytes(), MIME: "image/jpeg"}, nil
	}
	if err := png.Encode(&out, scaled); err != nil {
		return decisions.Image{}, errDecisionImage
	}
	return decisions.Image{Data: out.Bytes(), MIME: "image/png"}, nil
}
