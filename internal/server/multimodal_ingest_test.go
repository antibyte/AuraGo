package server

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

func TestPromoteUploadedImagesToMultiContent(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "openrouter"

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Please analyze this.\nagent_workspace/workdir/attachments/img.png\nThanks.",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)

	if out.Content != "" {
		t.Fatalf("expected Content to be empty, got %q", out.Content)
	}
	if len(out.MultiContent) != 2 {
		t.Fatalf("expected 2 parts (text + image), got %d", len(out.MultiContent))
	}
	if out.MultiContent[0].Type != openai.ChatMessagePartTypeText {
		t.Fatalf("expected first part type text, got %q", out.MultiContent[0].Type)
	}
	if strings.Contains(out.MultiContent[0].Text, "agent_workspace/workdir/attachments/img.png") {
		t.Fatalf("expected attachment path to be stripped from text part, got %q", out.MultiContent[0].Text)
	}
	if out.MultiContent[1].Type != openai.ChatMessagePartTypeImageURL {
		t.Fatalf("expected second part type image_url, got %q", out.MultiContent[1].Type)
	}
	if out.MultiContent[1].ImageURL == nil || !strings.HasPrefix(out.MultiContent[1].ImageURL.URL, "data:image/png;base64,") {
		t.Fatalf("expected data URI image_url, got %+v", out.MultiContent[1].ImageURL)
	}
}

func TestPromoteUploadedImagesToMultiContent_Disabled(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.LLM.Multimodal = false

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if out.Content != in.Content {
		t.Fatalf("expected Content to be unchanged, got %q", out.Content)
	}
	if len(out.MultiContent) != 0 {
		t.Fatalf("expected MultiContent to remain empty, got %d parts", len(out.MultiContent))
	}
}

func TestBuildOptimizedImageDataURI_DownscalesAndEncodesJPEG(t *testing.T) {
	// Create a large image with alpha variation so the encoder takes the JPEG path
	// (PNG-with-alpha would be huge; we flatten and encode JPEG).
	w, h := 2500, 1800
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i+0] = byte(x % 256)
			img.Pix[i+1] = byte(y % 256)
			img.Pix[i+2] = byte((x + y) % 256)
			img.Pix[i+3] = byte((x*y + 13) % 256) // varying alpha
		}
	}

	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}

	uri, err := buildOptimizedImageDataURI(raw.Bytes(), ".png", "image/png", 20_000_000, 1600, nil)
	if err != nil {
		t.Fatalf("buildOptimizedImageDataURI: %v", err)
	}
	if !strings.HasPrefix(uri, "data:image/jpeg;base64,") {
		t.Fatalf("expected jpeg data URI, got %q", uri[:min(40, len(uri))])
	}
	encoded := strings.TrimPrefix(uri, "data:image/jpeg;base64,")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	gotImg, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	gb := gotImg.Bounds()
	if gb.Dx() > 1600 || gb.Dy() > 1600 {
		t.Fatalf("expected downscaled <=1600px, got %dx%d", gb.Dx(), gb.Dy())
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPromoteUploadedImagesToMultiContent_FallbackToVisionWhenProviderUnsupported(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	orig := analyzeImageForFallback
	analyzeImageForFallback = func(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
		return "stub-analysis", 1, 1, nil
	}
	defer func() { analyzeImageForFallback = orig }()

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama" // treated as unsupported for multimodal image parts

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)

	if len(out.MultiContent) != 0 {
		t.Fatalf("expected MultiContent to be empty for fallback path, got %d parts", len(out.MultiContent))
	}
	if !strings.Contains(out.Content, "stub-analysis") {
		t.Fatalf("expected fallback analysis to be injected, got %q", out.Content)
	}
	if strings.Contains(out.Content, "agent_workspace/workdir/attachments/img.png") {
		t.Fatalf("expected attachment path to be stripped in fallback text, got %q", out.Content)
	}
}

func TestPromoteUploadedImagesToMultiContent_ProviderExtraAllowlist(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.MultimodalProviderTypesExtra = []string{"ollama"}

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if out.Content != "" {
		t.Fatalf("expected Content to be empty, got %q", out.Content)
	}
	if len(out.MultiContent) != 2 {
		t.Fatalf("expected MultiContent parts (text + image), got %d", len(out.MultiContent))
	}
}

func TestPromoteUploadedImagesToMultiContent_KimiK26ModelAllowlist(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.Model = "kimi-k2.6"

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if out.Content != "" {
		t.Fatalf("expected Content to be empty, got %q", out.Content)
	}
	if len(out.MultiContent) != 2 {
		t.Fatalf("expected MultiContent parts (text + image), got %d", len(out.MultiContent))
	}
}
