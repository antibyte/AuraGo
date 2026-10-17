package decisions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"mime"

	"golang.org/x/image/draw"
)

// ImageAdjustment describes only the shipping copy; no paths or contents escape.
type ImageAdjustment struct {
	Index          int    `json:"index"`
	OriginalWidth  int    `json:"original_width"`
	OriginalHeight int    `json:"original_height"`
	OriginalBytes  int    `json:"original_bytes"`
	OriginalFormat string `json:"original_format"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Bytes          int    `json:"bytes"`
	Format         string `json:"format"`
}

func prepareOpenRouterRequest(ctx context.Context, model DecisionModel, request Request, questions map[string]openRouterQuestion) ([]byte, []ImageAdjustment, error) {
	limits := decisionMediaLimits(model.ResolvedID())
	if len(request.Images) > limits.MaxImages {
		return nil, nil, NewError("image_limit_exceeded", nil)
	}
	text, err := openRouterTextState(request.State)
	if err != nil {
		return nil, nil, NewError(CodeInvalidRequest, err)
	}
	wire := openRouterDecisionRequest{Model: model.ResolvedID(), State: text, Questions: questions}
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, nil, NewError(CodeInvalidRequest, err)
	}
	textTokens := int64(len(body)) + 512
	if len(body) > limits.MaxRequestBytes {
		return nil, nil, NewError(CodeInvalidRequest, nil)
	}
	remainingTokens := min(int64(model.ContextLength), openRouterMaxContextTokens) - textTokens
	if remainingTokens < 0 || (isOpenRouterClefModel(model.ResolvedID()) && textTokens > 2000) {
		return nil, nil, NewError(CodeInputTokenBudgetExceeded, nil)
	}
	if len(request.Images) == 0 {
		return body, nil, nil
	}
	// Reserve JSON/data-URL overhead before sharing the remaining byte budget.
	remainingBytes := limits.MaxRequestBytes - len(body) - 2 - len(request.Images)*100
	remainingPatches, remainingPixels := limits.MaxPatches, limits.MaxProcessedBytes/4
	originalSizes := make([]image.Config, len(request.Images))
	var originalBytes, originalPatches, originalPixels, originalTokens int64
	for index, source := range request.Images {
		size, _, err := image.DecodeConfig(bytes.NewReader(source.Data))
		if err != nil {
			return nil, nil, NewError("image_invalid", err)
		}
		originalSizes[index] = size
		originalBytes += int64(base64.StdEncoding.EncodedLen(len(source.Data)))
		patches := imagePatches(size.Width, size.Height)
		originalPatches += patches
		originalPixels += int64(size.Width) * int64(size.Height)
		originalTokens += (patches*3 + 1) / 2 * int64(len(questions))
	}
	parts := []any{text}
	var adjustments []ImageAdjustment
	for index, source := range request.Images {
		if err := ctx.Err(); err != nil {
			return nil, nil, NewError(CodeCancelled, err)
		}
		count := int64(len(request.Images) - index)
		// Share scarce budgets, but do not shrink unequal images that already fit together.
		byteShare, tokenShare := remainingBytes/int(count), remainingTokens/count
		if originalBytes <= int64(remainingBytes) {
			byteShare = remainingBytes
		}
		byteLimit := min(limits.MaxImageBytes, byteShare/4*3)
		patchLimit := int64(0)
		if limits.MaxPatches > 0 {
			patchLimit = remainingPatches / count
			if originalPatches <= remainingPatches {
				patchLimit = remainingPatches
			}
			if originalTokens <= remainingTokens {
				tokenShare = remainingTokens
			}
			patchLimit = min(patchLimit, tokenShare*2/(3*int64(len(questions))))
			if patchLimit < 1 {
				return nil, nil, NewError(CodeInputTokenBudgetExceeded, nil)
			}
		} else {
			if originalBytes/4 <= remainingTokens {
				tokenShare = remainingTokens
			}
			byteLimit = min(byteLimit, int(tokenShare)*3)
		}
		imageLimits := limits
		if limits.MaxProcessedBytes > 0 {
			pixelShare := remainingPixels / count
			if originalPixels <= remainingPixels {
				pixelShare = remainingPixels
			}
			imageLimits.MaxPixels = min(limits.MaxPixels, pixelShare)
		}
		prepared, adjustment, err := prepareDecisionImage(ctx, source, imageLimits, byteLimit, patchLimit)
		if err != nil {
			return nil, nil, fmt.Errorf("image %d: %w", index, err)
		}
		encoded := base64.StdEncoding.EncodeToString(prepared.Data)
		parts = append(parts, openRouterImagePart{Type: "image_url", ImageURL: openRouterImageURL{URL: "data:" + prepared.MIME + ";base64," + encoded}})
		remainingBytes -= len(encoded)
		if limits.MaxPatches > 0 {
			patches := imagePatches(adjustment.Width, adjustment.Height)
			remainingPatches -= patches
			remainingTokens -= (patches*3 + 1) / 2 * int64(len(questions))
		} else {
			remainingTokens -= int64((len(encoded) + 3) / 4)
		}
		remainingPixels -= int64(adjustment.Width) * int64(adjustment.Height)
		originalBytes -= int64(base64.StdEncoding.EncodedLen(len(source.Data)))
		size := originalSizes[index]
		patches := imagePatches(size.Width, size.Height)
		originalPatches -= patches
		originalPixels -= int64(size.Width) * int64(size.Height)
		originalTokens -= (patches*3 + 1) / 2 * int64(len(questions))
		if !bytes.Equal(source.Data, prepared.Data) {
			adjustment.Index = index
			adjustments = append(adjustments, adjustment)
		}
	}
	wire.State = parts
	body, err = json.Marshal(wire)
	if err != nil {
		return nil, nil, NewError(CodeInvalidRequest, err)
	}
	if len(body) > limits.MaxRequestBytes {
		return nil, nil, NewError("image_limit_exceeded", nil)
	}
	if remainingTokens < 0 {
		return nil, nil, NewError(CodeInputTokenBudgetExceeded, nil)
	}
	return body, adjustments, nil
}

func imagePatches(width, height int) int64 { return int64((width+31)/32) * int64((height+31)/32) }

func prepareDecisionImage(ctx context.Context, source Image, limits DecisionMediaLimits, byteLimit int, patchLimit int64) (Image, ImageAdjustment, error) {
	var adjustment ImageAdjustment
	if byteLimit <= 0 {
		return Image{}, adjustment, NewError("image_limit_exceeded", nil)
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(source.Data))
	mediaType, _, mimeErr := mime.ParseMediaType(source.MIME)
	if err != nil || mimeErr != nil || (format != "png" && format != "jpeg") || mediaType != "image/"+format || configuration.Width <= 0 || configuration.Height <= 0 || int64(configuration.Width)*int64(configuration.Height) > 32<<20 {
		return Image{}, adjustment, NewError("image_invalid", nil)
	}
	source.MIME = mediaType
	if format == "png" && animatedDecisionPNG(source.Data) {
		return Image{}, adjustment, NewError("image_invalid", nil)
	}
	orientation := jpegOrientation(source.Data)
	w, h := configuration.Width, configuration.Height
	adjustment = ImageAdjustment{OriginalWidth: w, OriginalHeight: h, OriginalBytes: len(source.Data), OriginalFormat: format}
	if orientation >= 5 {
		w, h = h, w
	}
	scale := 1.0
	if limits.MaxSide > 0 {
		scale = min(scale, float64(limits.MaxSide)/float64(max(w, h)))
	}
	if limits.MaxPixels > 0 {
		scale = min(scale, math.Sqrt(float64(limits.MaxPixels)/(float64(w)*float64(h))))
	}
	if patchLimit > 0 {
		scale = min(scale, math.Sqrt(float64(patchLimit*1024)/(float64(w)*float64(h))))
	}
	w, h = max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	for patchLimit > 0 && imagePatches(w, h) > patchLimit {
		w, h = max(1, int(float64(w)*.98)), max(1, int(float64(h)*.98))
	}
	if !decisionImageMinimum(w, h, limits) {
		return Image{}, adjustment, NewError("image_minimum_unmet", nil)
	}
	decoded, _, err := image.Decode(bytes.NewReader(source.Data))
	if err != nil {
		return Image{}, adjustment, NewError("image_invalid", nil)
	}
	if err := ctx.Err(); err != nil {
		return Image{}, adjustment, NewError(CodeCancelled, err)
	}
	if orientation == 1 && w == configuration.Width && h == configuration.Height && len(source.Data) <= byteLimit {
		adjustment.Width, adjustment.Height, adjustment.Bytes, adjustment.Format = w, h, len(source.Data), format
		return source, adjustment, nil
	}
	decoded = orientDecisionImage(decoded, orientation)
	opaque := false
	if candidate, ok := decoded.(interface{ Opaque() bool }); ok {
		opaque = candidate.Opaque()
	}
	for step := 0; step <= 6; step++ {
		if err := ctx.Err(); err != nil {
			return Image{}, adjustment, NewError(CodeCancelled, err)
		}
		if !decisionImageMinimum(w, h, limits) {
			return Image{}, adjustment, NewError("image_minimum_unmet", nil)
		}
		working := decoded
		if decoded.Bounds().Dx() != w || decoded.Bounds().Dy() != h {
			resized := image.NewNRGBA(image.Rect(0, 0, w, h))
			draw.CatmullRom.Scale(resized, resized.Bounds(), decoded, decoded.Bounds(), draw.Src, nil)
			working = resized
		}
		for _, quality := range []int{85, 70} {
			buffer := imageEncodeBuffer{ctx: ctx, limit: byteLimit}
			mime, encodedFormat := "image/png", "png"
			if opaque {
				mime, encodedFormat = "image/jpeg", "jpeg"
				err = jpeg.Encode(&buffer, working, &jpeg.Options{Quality: quality})
			} else {
				err = png.Encode(&buffer, working)
			}
			if err == nil {
				adjustment.Width, adjustment.Height, adjustment.Bytes, adjustment.Format = w, h, buffer.buffer.Len(), encodedFormat
				return Image{Data: buffer.buffer.Bytes(), MIME: mime}, adjustment, nil
			}
			if ctx.Err() != nil {
				return Image{}, adjustment, NewError(CodeCancelled, ctx.Err())
			}
			if !errors.Is(err, errImageEncodingLimit) {
				return Image{}, adjustment, NewError("image_invalid", err)
			}
			if !opaque {
				break
			}
		}
		w, h = max(1, int(float64(w)*.8)), max(1, int(float64(h)*.8))
	}
	return Image{}, adjustment, NewError("image_limit_exceeded", nil)
}

func decisionImageMinimum(w, h int, limits DecisionMediaLimits) bool {
	return w >= max(1, limits.MinSide) && h >= max(1, limits.MinSide) && int64(w)*int64(h) >= limits.MinPixels
}

var errImageEncodingLimit = errors.New("encoded image exceeds shipping budget")

type imageEncodeBuffer struct {
	buffer bytes.Buffer
	ctx    context.Context
	limit  int
}

func (buffer *imageEncodeBuffer) Write(data []byte) (int, error) {
	if err := buffer.ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) > buffer.limit-buffer.buffer.Len() {
		return 0, errImageEncodingLimit
	}
	return buffer.buffer.Write(data)
}
