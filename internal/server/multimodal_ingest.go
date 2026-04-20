package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/image/draw"
)

var attachmentPathRe = regexp.MustCompile(`agent_workspace/workdir/attachments/([A-Za-z0-9._-]+)`)

var analyzeImageForFallback = tools.AnalyzeImageWithPrompt

func promoteUploadedImagesToMultiContent(cfg *config.Config, msg openai.ChatCompletionMessage, workspaceDir string, logger *slog.Logger) openai.ChatCompletionMessage {
	if cfg == nil || !cfg.LLM.Multimodal {
		return msg
	}
	if msg.Role != openai.ChatMessageRoleUser || strings.TrimSpace(msg.Content) == "" {
		return msg
	}
	if len(msg.MultiContent) > 0 {
		return msg
	}

	matches := attachmentPathRe.FindAllStringSubmatch(msg.Content, -1)
	if len(matches) == 0 {
		return msg
	}

	const maxInlineBytes = 10 << 20 // 10MB per image (provider limits vary; keep a conservative default)
	const maxDecodedPixels = 4_000_000
	const maxDim = 1600

	// Trust the explicit multimodal setting from config over provider heuristics.
	// If users enable llm.multimodal, uploaded images are sent directly to the
	// main model even when the provider type is not part of AuraGo's built-in
	// allowlist.
	if !mainProviderSupportsImageMultimodal(cfg) {
		return fallbackVisionAnalysis(cfg, msg, matches, logger)
	}

	var imageParts []openai.ChatMessagePart
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		filename := m[1]
		ext := strings.ToLower(filepath.Ext(filename))
		mime := imageMimeType(ext)
		if mime == "" {
			continue // not an image we can inline
		}
		fullPath := filepath.Join(workspaceDir, "attachments", filename)
		info, err := os.Stat(fullPath)
		if err != nil {
			if logger != nil {
				logger.Warn("Attachment referenced but missing on disk", "path", fullPath, "error", err)
			}
			continue
		}
		if info.Size() <= 0 || info.Size() > maxInlineBytes {
			if logger != nil {
				logger.Warn("Attachment too large to inline as multimodal input", "path", fullPath, "size", info.Size(), "max", maxInlineBytes)
			}
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if logger != nil {
				logger.Warn("Failed to read attachment for multimodal input", "path", fullPath, "error", err)
			}
			continue
		}
		dataURI, err := buildOptimizedImageDataURI(data, ext, mime, maxDecodedPixels, maxDim, logger)
		if err != nil {
			if logger != nil {
				logger.Warn("Failed to build optimized image data URI; skipping attachment", "path", fullPath, "error", err)
			}
			continue
		}
		imageParts = append(imageParts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL: dataURI,
			},
		})
	}

	if len(imageParts) == 0 {
		return msg
	}

	cleanText := stripAttachmentPaths(msg.Content)
	if strings.TrimSpace(cleanText) == "" {
		cleanText = "Please analyze the attached image(s)."
	}

	parts := make([]openai.ChatMessagePart, 0, 1+len(imageParts))
	parts = append(parts, openai.ChatMessagePart{Type: openai.ChatMessagePartTypeText, Text: cleanText})
	parts = append(parts, imageParts...)

	out := msg
	out.Content = ""
	out.MultiContent = parts
	return out
}

func mainProviderSupportsImageMultimodal(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.LLM.Multimodal {
		return true
	}
	pt := strings.ToLower(strings.TrimSpace(cfg.LLM.ProviderType))
	for _, extra := range cfg.LLM.MultimodalProviderTypesExtra {
		if strings.ToLower(strings.TrimSpace(extra)) == pt && pt != "" {
			return true
		}
	}

	switch pt {
	case "openai", "openrouter", "anthropic", "google", "custom", "workers-ai":
		return true
	default:
		return false
	}
}

func fallbackVisionAnalysis(cfg *config.Config, msg openai.ChatCompletionMessage, matches [][]string, logger *slog.Logger) openai.ChatCompletionMessage {
	if cfg == nil || analyzeImageForFallback == nil {
		return msg
	}
	cleanText := stripAttachmentPaths(msg.Content)
	if strings.TrimSpace(cleanText) == "" {
		cleanText = "Please analyze the attached image(s)."
	}
	prompt := "Describe this image in detail. What do you see? If there is text, transcribe it. If there are people, describe their actions."

	var b strings.Builder
	b.WriteString(cleanText)
	b.WriteString("\n\n[IMAGE ANALYSIS]\n")
	added := 0
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		filename := m[1]
		ext := strings.ToLower(filepath.Ext(filename))
		if imageMimeType(ext) == "" {
			continue
		}
		agentPath := "agent_workspace/workdir/attachments/" + filename
		analysis, _, _, err := analyzeImageForFallback(agentPath, prompt, cfg)
		if err != nil {
			if logger != nil {
				logger.Warn("Vision fallback analysis failed", "path", agentPath, "error", err)
			}
			continue
		}
		analysis = strings.TrimSpace(analysis)
		if analysis == "" {
			continue
		}
		added++
		b.WriteString("- ")
		b.WriteString(filename)
		b.WriteString(": ")
		b.WriteString(analysis)
		b.WriteString("\n")
	}
	if added == 0 {
		return msg
	}

	out := msg
	out.MultiContent = nil
	out.Content = strings.TrimSpace(b.String())
	return out
}

func imageMimeType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return ""
	}
}

func buildOptimizedImageDataURI(raw []byte, ext string, mime string, maxDecodedPixels int, maxDim int, logger *slog.Logger) (string, error) {
	// Fast path: if decode fails, fall back to raw bytes as-is.
	// This keeps the flow robust for edge cases while still enabling optimization for normal images.
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(raw)), nil
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return "", fmt.Errorf("invalid decoded bounds")
	}
	if w*h > maxDecodedPixels {
		return "", fmt.Errorf("decoded image too large: %dx%d (%d pixels)", w, h, w*h)
	}

	// Resize if needed.
	targetW, targetH := w, h
	if w > maxDim || h > maxDim {
		if w >= h {
			targetW = maxDim
			targetH = int(float64(h) * (float64(maxDim) / float64(w)))
		} else {
			targetH = maxDim
			targetW = int(float64(w) * (float64(maxDim) / float64(h)))
		}
		if targetW < 1 {
			targetW = 1
		}
		if targetH < 1 {
			targetH = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
		w, h = targetW, targetH
	}

	// Encode: prefer preserving PNG only when the original is PNG and it stays reasonably small.
	// Otherwise use JPEG to keep size down. If the image has alpha, composite onto white.
	hasAlpha := imageHasAlpha(img)
	var outBytes []byte

	if strings.EqualFold(format, "png") && !hasAlpha {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err == nil {
			outBytes = buf.Bytes()
			// If PNG is still large, we fall back to JPEG.
			if len(outBytes) <= 1_500_000 {
				return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(outBytes)), nil
			}
		}
	}

	if hasAlpha {
		img = flattenAlpha(img, color.White)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("jpeg encode: %w", err)
	}
	outBytes = buf.Bytes()
	if logger != nil && (w != 0 && h != 0) {
		_ = ext // reserved for future logging/detail
	}
	return fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(outBytes)), nil
}

func imageHasAlpha(img image.Image) bool {
	switch i := img.(type) {
	case *image.NRGBA:
		return true
	case *image.NRGBA64:
		return true
	case *image.RGBA:
		// RGBA may have alpha; scan a small sample grid to avoid O(w*h).
		b := i.Bounds()
		stepX := max(1, b.Dx()/32)
		stepY := max(1, b.Dy()/32)
		for y := b.Min.Y; y < b.Max.Y; y += stepY {
			for x := b.Min.X; x < b.Max.X; x += stepX {
				_, _, _, a := i.At(x, y).RGBA()
				if a != 0xffff {
					return true
				}
			}
		}
		return false
	default:
		// Unknown format: assume it might have alpha; be conservative.
		return true
	}
}

func flattenAlpha(src image.Image, bg color.Color) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	// Fill background.
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	// Draw source over it.
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stripAttachmentPaths(s string) string {
	out := attachmentPathRe.ReplaceAllString(s, "")
	// Remove empty leftover lines and collapse spacing a bit to avoid spamming the prompt.
	out = strings.ReplaceAll(out, "\r\n", "\n")
	lines := strings.Split(out, "\n")
	keep := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		keep = append(keep, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}
