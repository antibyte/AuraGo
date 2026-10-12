package server

import (
	"bytes"
	"encoding/base64"
	"errors"
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

	auto := false
	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openrouter"
	cfg.Providers = []config.ProviderEntry{{
		ID:   "main",
		Type: "openrouter",
		Capabilities: config.ProviderCapabilities{
			Auto:       &auto,
			Multimodal: true,
		},
	}}

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

func TestPromoteUploadedImagesToMultiContentAllowsNestedAgoDeskAttachmentPaths(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "attachments", "agodesk", "sess-1", "att-1")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "diagram.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	auto := false
	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openrouter"
	cfg.Providers = []config.ProviderEntry{{
		ID:   "main",
		Type: "openrouter",
		Capabilities: config.ProviderCapabilities{
			Auto:       &auto,
			Multimodal: true,
		},
	}}

	agentPath := "agent_workspace/workdir/attachments/agodesk/sess-1/att-1/diagram.png"
	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Please inspect this attached image.\n" + agentPath,
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)

	if out.Content != "" {
		t.Fatalf("expected Content to be empty, got %q", out.Content)
	}
	if len(out.MultiContent) != 2 {
		t.Fatalf("expected 2 parts (text + image), got %d", len(out.MultiContent))
	}
	if strings.Contains(out.MultiContent[0].Text, agentPath) {
		t.Fatalf("expected nested attachment path to be stripped, got %q", out.MultiContent[0].Text)
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

func TestPromoteUploadedImagesToMultiContent_UsesProviderMultimodalCapability(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	autoFalse := false
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "custom"
	cfg.LLM.Model = "manual-vision-model"
	cfg.LLM.Multimodal = false
	cfg.Providers = []config.ProviderEntry{{
		ID:      "main",
		Type:    "custom",
		BaseURL: "https://example.test/v1",
		Model:   "manual-vision-model",
		Capabilities: config.ProviderCapabilities{
			Auto:          &autoFalse,
			ToolCalling:   true,
			Multimodal:    true,
			DetectedModel: "manual-vision-model",
			Source:        "manual",
		},
	}}

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if len(out.MultiContent) != 2 {
		t.Fatalf("expected MultiContent parts (text + image), got %d", len(out.MultiContent))
	}
	if out.MultiContent[1].Type != openai.ChatMessagePartTypeImageURL {
		t.Fatalf("expected second part type image_url, got %q", out.MultiContent[1].Type)
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

func TestAgnesMainProviderRejectsLocalAndInlineImageInputs(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "agnes"
	cfg.LLM.Model = "agnes-2.0-flash"
	cfg.LLM.Multimodal = true

	local := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Inspect agent_workspace/workdir/attachments/image.png",
	}
	if err := validateCurrentMainProviderImageInput(cfg, local); err == nil {
		t.Fatal("expected Agnes local attachment rejection")
	}
	out := promoteUploadedImagesToMultiContent(cfg, local, t.TempDir(), nil)
	if len(out.MultiContent) != 0 || out.Content != local.Content {
		t.Fatalf("Agnes local attachment was promoted: %+v", out)
	}

	inline := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{{
			Type:     openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64,AA=="},
		}},
	}
	if err := validateMainProviderImageParts(cfg, inline); err == nil {
		t.Fatal("expected Agnes data URL rejection")
	}
}

func TestAgnesMainProviderAllowsOnlyPublicImageURLs(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "agnes"

	publicMessage := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{{
			Type:     openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{URL: "https://8.8.8.8/image.png?signature=keep-me"},
		}},
	}
	if err := validateMainProviderImageParts(cfg, publicMessage); err != nil {
		t.Fatalf("public Agnes image URL rejected: %v", err)
	}

	privateMessage := publicMessage
	privateMessage.MultiContent = []openai.ChatMessagePart{{
		Type:     openai.ChatMessagePartTypeImageURL,
		ImageURL: &openai.ChatMessageImageURL{URL: "http://192.168.1.2/image.png"},
	}}
	if err := validateMainProviderImageParts(cfg, privateMessage); err == nil {
		t.Fatal("expected private Agnes image URL rejection")
	}
}

func kimiInlineConfig() *config.Config {
	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.Model = "kimi-k2.6"
	return cfg
}

func TestPromoteRequestImagesSkipsOlderUserAttachments(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	path := "agent_workspace/workdir/attachments/img.png"
	older := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "first " + path}
	latest := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "second " + path}
	out := promoteRequestImages(kimiInlineConfig(), []openai.ChatCompletionMessage{older, latest}, dir, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if out[0].Content != older.Content || len(out[0].MultiContent) != 0 {
		t.Fatalf("older user turn was changed: %+v", out[0])
	}
	if len(out[1].MultiContent) < 2 {
		t.Fatalf("expected the latest user turn to gain an image part, got %+v", out[1])
	}
}

func TestPromoteUploadedImagesKeepsUnsentPaths(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "ok.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, (10<<20)+1)
	copy(big, []byte{0x89, 0x50, 0x4e, 0x47})
	if err := os.WriteFile(filepath.Join(attachDir, "huge.png"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "see agent_workspace/workdir/attachments/ok.png and agent_workspace/workdir/attachments/huge.png",
	}
	out := promoteUploadedImagesToMultiContent(kimiInlineConfig(), in, dir, nil)
	var images int
	for _, part := range out.MultiContent {
		if part.Type == openai.ChatMessagePartTypeImageURL {
			images++
		}
	}
	if images != 1 {
		t.Fatalf("expected one image part, got %d (%+v)", images, out.MultiContent)
	}
	joined := out.Content
	for _, part := range out.MultiContent {
		if part.Type == openai.ChatMessagePartTypeText {
			joined += "\n" + part.Text
		}
	}
	if strings.Contains(joined, "attachments/ok.png") {
		t.Fatalf("sent path still present: %q", joined)
	}
	if !strings.Contains(joined, "Image huge.png was not sent: larger than 10 MiB.") {
		t.Fatalf("missing skip line, got %q", joined)
	}
}

func TestFallbackVisionUsesTheUserQuestion(t *testing.T) {
	dir := t.TempDir()
	attachDir := filepath.Join(dir, "attachments")
	if err := os.MkdirAll(attachDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachDir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	var gotPrompt string
	orig := analyzeImageForFallback
	analyzeImageForFallback = func(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
		gotPrompt = prompt
		return "stub-analysis", 1, 1, nil
	}
	defer func() { analyzeImageForFallback = orig }()

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama"
	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "How many red cars? agent_workspace/workdir/attachments/img.png",
	}
	promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if !strings.Contains(gotPrompt, "How many red cars?") {
		t.Fatalf("expected the user question in the fallback prompt, got %q", gotPrompt)
	}
	if strings.Contains(gotPrompt, "Describe this image in detail") {
		t.Fatalf("expected the user question instead of the fixed describe sentence, got %q", gotPrompt)
	}
}

func TestFallbackVisionReportsWhenAnalysisFails(t *testing.T) {
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
		return "", 0, 0, errors.New("vision unavailable")
	}
	defer func() { analyzeImageForFallback = orig }()

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama"
	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "What is this? agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if len(out.MultiContent) != 0 {
		t.Fatalf("expected no MultiContent when analysis fails, got %+v", out.MultiContent)
	}
	if !strings.Contains(out.Content, "The attached image could not be analyzed.") {
		t.Fatalf("expected analysis failure note, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "What is this?") {
		t.Fatalf("expected the user text to remain, got %q", out.Content)
	}
}

func TestLegacyOpenAIFlagDoesNotInlineWithoutImageCapability(t *testing.T) {
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
		return "", 0, 0, errors.New("vision unavailable")
	}
	defer func() { analyzeImageForFallback = orig }()

	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "text-only-unknown"
	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "look at agent_workspace/workdir/attachments/img.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	if len(out.MultiContent) != 0 {
		t.Fatalf("expected text content without image parts, got %+v", out)
	}
	if strings.TrimSpace(out.Content) == "" {
		t.Fatal("expected content to stay text")
	}
	if strings.Contains(out.Content, "data:image") {
		t.Fatalf("expected no inlined image data, got %q", out.Content)
	}
}

func TestPromoteUploadedImagesDirectoryReadUsesShortReason(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "attachments", "folder.png"), 0o755); err != nil {
		t.Fatal(err)
	}

	auto := false
	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openrouter"
	cfg.Providers = []config.ProviderEntry{{
		ID:   "main",
		Type: "openrouter",
		Capabilities: config.ProviderCapabilities{
			Auto:       &auto,
			Multimodal: true,
		},
	}}

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Look at agent_workspace/workdir/attachments/folder.png",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, dir, nil)
	joined := out.Content
	for _, part := range out.MultiContent {
		if part.Type == openai.ChatMessagePartTypeText {
			joined += "\n" + part.Text
		}
	}
	if !strings.Contains(joined, "Image folder.png was not sent: could not be read.") {
		t.Fatalf("expected short read reason, got %q", joined)
	}
	if strings.Contains(joined, dir) {
		t.Fatalf("message contains the absolute workspace path: %q", joined)
	}
}

func TestFallbackVisionLeavesNonImageAttachmentsUnchanged(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Multimodal = true
	cfg.LLM.ProviderType = "ollama"
	orig := analyzeImageForFallback
	analyzeImageForFallback = func(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
		t.Fatalf("non-image attachment was sent for analysis: %s", filePath)
		return "", 0, 0, nil
	}
	defer func() { analyzeImageForFallback = orig }()

	in := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Read agent_workspace/workdir/attachments/notes.pdf",
	}
	out := promoteUploadedImagesToMultiContent(cfg, in, t.TempDir(), nil)
	if out.Content != in.Content || len(out.MultiContent) != 0 {
		t.Fatalf("expected the pdf-only message to stay unchanged, got %+v", out)
	}
	if strings.Contains(out.Content, "could not be analyzed") {
		t.Fatalf("non-image attachment was reported as a failed image analysis: %q", out.Content)
	}
}
