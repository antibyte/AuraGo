package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/providerutil"
	"aurago/internal/security"
)

var visionHTTPClient = &http.Client{Timeout: 60 * time.Second}
var validatePublicImageURL = security.ValidatePublicHTTPURL

const DefaultVisionModel = "google/gemini-2.0-flash-001"
const VisionPublicURLRequiredMessage = "Agnes AI vision requires a publicly reachable HTTP(S) image URL; local files, uploads, and base64 images are not supported"

var ErrVisionPublicURLRequired = errors.New(VisionPublicURLRequiredMessage)

// AnalyzeImageWithPrompt sends an image file to the configured Vision LLM for analysis.
// The prompt parameter controls what the model should focus on.
func AnalyzeImageWithPrompt(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
	return AnalyzeImageWithPromptContext(context.Background(), filePath, prompt, cfg)
}

// AnalyzeImageWithPromptContext sends an image file to the configured Vision LLM
// and binds the provider request to ctx.
func AnalyzeImageWithPromptContext(ctx context.Context, filePath, prompt string, cfg *config.Config) (string, int, int, error) {
	return analyzeLocalImageWithPromptContext(ctx, filePath, prompt, cfg, true)
}

// AnalyzeTrustedImageFileWithPrompt analyzes an application-managed local file
// such as a temporary Telegram/Discord download. Agent-provided paths must use
// AnalyzeImageWithPrompt so workspace boundary checks remain enforced.
func AnalyzeTrustedImageFileWithPrompt(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
	return analyzeLocalImageWithPromptContext(context.Background(), filePath, prompt, cfg, false)
}

func analyzeLocalImageWithPromptContext(ctx context.Context, filePath, prompt string, cfg *config.Config, resolveWorkspacePath bool) (string, int, int, error) {
	if cfg == nil {
		return "", 0, 0, fmt.Errorf("vision configuration is not available")
	}
	if VisionRequiresPublicImageURL(cfg) {
		return "", 0, 0, ErrVisionPublicURLRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolvedPath := filePath
	if resolveWorkspacePath {
		var err error
		resolvedPath, err = resolveToolInputPath(filePath, cfg)
		if err != nil {
			return "", 0, 0, fmt.Errorf("invalid image file path: %w", err)
		}
	}

	// Read and base64-encode the image
	visionInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to stat image file: %w", err)
	}
	const maxVisionBytes = 50 * 1024 * 1024 // 50 MB
	if visionInfo.Size() > maxVisionBytes {
		return "", 0, 0, fmt.Errorf("image file too large (%d bytes, max 50 MB)", visionInfo.Size())
	}
	imageData, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to read image file: %w", err)
	}

	mimeType := "image/jpeg"
	lower := strings.ToLower(resolvedPath)
	switch {
	case strings.HasSuffix(lower, ".png"):
		mimeType = "image/png"
	case strings.HasSuffix(lower, ".gif"):
		mimeType = "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		mimeType = "image/webp"
	case strings.HasSuffix(lower, ".bmp"):
		mimeType = "image/bmp"
	}

	encodedImage := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, encodedImage)
	return analyzeImageReferenceWithPromptContext(ctx, dataURL, prompt, cfg)
}

// AnalyzeImageURLWithPrompt analyzes a publicly reachable image URL using the
// configured Vision LLM.
func AnalyzeImageURLWithPrompt(imageURL, prompt string, cfg *config.Config) (string, int, int, error) {
	return AnalyzeImageURLWithPromptContext(context.Background(), imageURL, prompt, cfg)
}

// AnalyzeImageURLWithPromptContext validates and analyzes a publicly reachable
// image URL without downloading or logging the signed URL in AuraGo.
func AnalyzeImageURLWithPromptContext(ctx context.Context, imageURL, prompt string, cfg *config.Config) (string, int, int, error) {
	if cfg == nil {
		return "", 0, 0, fmt.Errorf("vision configuration is not available")
	}
	imageURL = strings.TrimSpace(imageURL)
	if err := validatePublicImageURL(imageURL); err != nil {
		return "", 0, 0, fmt.Errorf("image_url must be publicly reachable via HTTP(S): %w", err)
	}
	return analyzeImageReferenceWithPromptContext(ctx, imageURL, prompt, cfg)
}

// VisionRequiresPublicImageURL reports whether the resolved Vision provider can
// consume public image URLs but cannot consume inline base64/local file data.
func VisionRequiresPublicImageURL(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	providerType := strings.TrimSpace(cfg.Vision.ProviderType)
	if providerType == "" {
		providerType = strings.TrimSpace(cfg.LLM.ProviderType)
	}
	return strings.EqualFold(providerType, "agnes")
}

func analyzeImageReferenceWithPromptContext(ctx context.Context, imageReference, prompt string, cfg *config.Config) (string, int, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	prompt, _ = PrepareVisionPrompt(prompt)

	// Resolved by config.ResolveProviders (falls back to main LLM)
	apiKey := strings.TrimSpace(cfg.Vision.APIKey)
	baseURL := providerutil.NormalizeBaseURL(strings.TrimSpace(cfg.Vision.BaseURL))

	model := strings.TrimSpace(cfg.Vision.Model)
	if model == "" {
		model = DefaultVisionModel
	}

	if apiKey == "" {
		apiKey = strings.TrimSpace(cfg.LLM.APIKey)
	}
	if apiKey == "" {
		return "", 0, 0, fmt.Errorf("vision API key is not configured — set vision.provider or vision.api_key in config, or ensure the main LLM API key is set as fallback")
	}
	if baseURL == "" {
		baseURL = providerutil.NormalizeBaseURL(strings.TrimSpace(cfg.LLM.BaseURL))
	}
	if baseURL == "" {
		return "", 0, 0, fmt.Errorf("vision base URL is not configured")
	}

	// OpenAI-compatible vision payload
	type ImageURL struct {
		URL string `json:"url"`
	}
	type ContentPart struct {
		Type     string    `json:"type"`
		Text     string    `json:"text,omitempty"`
		ImageURL *ImageURL `json:"image_url,omitempty"`
	}
	type Message struct {
		Role    string        `json:"role"`
		Content []ContentPart `json:"content"`
	}
	type RequestBody struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	payload := RequestBody{
		Model: model,
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: prompt},
					{Type: "image_url", ImageURL: &ImageURL{URL: imageReference}},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to marshal vision payload: %w", err)
	}

	reqURL := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create vision request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/antibyte/AuraGo")
	req.Header.Set("X-Title", "AuraGo")

	resp, err := visionHTTPClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("vision request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return "", 0, 0, fmt.Errorf("vision API error (status %d) and failed to read response body: %w", resp.StatusCode, err)
		}
		return "", 0, 0, fmt.Errorf("vision API error (status %d, provider=%s, base_url=%s, auth_present=%t): %s", resp.StatusCode, strings.TrimSpace(cfg.Vision.ProviderType), baseURL, apiKey != "", string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, 0, fmt.Errorf("failed to decode vision response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("no analysis received in vision response")
	}

	analysis := result.Choices[0].Message.Content
	if normalized, handled := NormalizeVisionAnalysis(prompt, analysis); handled {
		analysis = normalized
	}
	return analysis, result.Usage.PromptTokens, result.Usage.CompletionTokens, nil
}

const visionCountSchemaMarker = "AURAGO_STRUCTURED_OBJECT_COUNT_V1"

// PrepareVisionPrompt requests a machine-checkable object count when the user
// asks for quantities. Non-count prompts are returned unchanged.
func PrepareVisionPrompt(prompt string) (string, bool) {
	prompt = strings.TrimSpace(prompt)
	if !visionPromptRequestsObjectCount(prompt) {
		return prompt, false
	}
	if strings.Contains(prompt, visionCountSchemaMarker) {
		return prompt, true
	}
	return prompt + `

` + visionCountSchemaMarker + `
Return ONLY one JSON object with this exact shape:
{"confirmed_count":0,"possible_additional_count":0,"other_vehicles":[],"items":[{"index":1,"type":"car","confidence":0.0,"confirmed":true}],"uncertainty":""}
confirmed_count must equal the number of items where confirmed=true. Put ambiguous candidates only in possible_additional_count and uncertainty. List other vehicle types separately and never add them to confirmed_count.`, true
}

type visionCountItem struct {
	Index      int     `json:"index"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Confirmed  *bool   `json:"confirmed,omitempty"`
}

type visionCountResult struct {
	ConfirmedCount          int               `json:"confirmed_count"`
	PossibleAdditionalCount int               `json:"possible_additional_count"`
	OtherVehicles           []string          `json:"other_vehicles"`
	Items                   []visionCountItem `json:"items,omitempty"`
	Uncertainty             string            `json:"uncertainty"`
	Consistent              bool              `json:"consistent"`
}

// NormalizeVisionAnalysis validates count/list consistency locally. When the
// provider contradicts itself, the item list is discarded and only its stated
// confirmed count plus an explicit uncertainty signal is retained.
func NormalizeVisionAnalysis(prompt, raw string) (string, bool) {
	if !visionPromptRequestsObjectCount(prompt) && !strings.Contains(prompt, visionCountSchemaMarker) {
		return raw, false
	}
	result, ok := parseVisionCountResult(raw)
	if !ok {
		fallback := map[string]interface{}{
			"status":                    "uncertain",
			"confirmed_count":           nil,
			"possible_additional_count": nil,
			"other_vehicles":            []string{},
			"items":                     []visionCountItem{},
			"uncertainty":               "Vision provider did not return a valid structured count; do not infer a total from prose.",
			"consistent":                false,
		}
		encoded, _ := json.Marshal(fallback)
		return string(encoded), true
	}
	confirmedItems := 0
	for _, item := range result.Items {
		if item.Confirmed == nil || *item.Confirmed {
			confirmedItems++
		}
	}
	result.Consistent = confirmedItems == result.ConfirmedCount
	if !result.Consistent {
		conflict := "The provider's item list conflicted with confirmed_count; only confirmed_count is retained."
		if strings.TrimSpace(result.Uncertainty) == "" {
			result.Uncertainty = conflict
		} else {
			result.Uncertainty = strings.TrimSpace(result.Uncertainty) + " " + conflict
		}
		encoded, err := json.Marshal(map[string]interface{}{
			"confirmed_count": result.ConfirmedCount,
			"uncertainty":     result.Uncertainty,
			"consistent":      false,
		})
		if err != nil {
			return raw, false
		}
		return string(encoded), true
	}
	if result.OtherVehicles == nil {
		result.OtherVehicles = []string{}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return raw, false
	}
	return string(encoded), true
}

func visionPromptRequestsObjectCount(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	for _, marker := range []string{"wie viele", "anzahl", "zähle", "zähl", "how many", "count ", "number of"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func parseVisionCountResult(raw string) (visionCountResult, bool) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return visionCountResult{}, false
	}
	jsonText := trimmed[start : end+1]
	var wrapper struct {
		Analysis json.RawMessage `json:"analysis"`
	}
	if json.Unmarshal([]byte(jsonText), &wrapper) == nil && len(wrapper.Analysis) > 0 {
		var nested string
		if json.Unmarshal(wrapper.Analysis, &nested) == nil {
			return parseVisionCountResult(nested)
		}
		var result visionCountResult
		if json.Unmarshal(wrapper.Analysis, &result) == nil {
			return result, true
		}
	}
	var result visionCountResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return visionCountResult{}, false
	}
	var presence map[string]json.RawMessage
	if json.Unmarshal([]byte(jsonText), &presence) != nil || presence["confirmed_count"] == nil {
		return visionCountResult{}, false
	}
	return result, true
}
