package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"aurago/internal/security"
)

const defaultPromptPayloadRunes = 4000

// PayloadError describes a payload validation failure and its HTTP status.
type PayloadError struct {
	StatusCode int
	Message    string
}

func (e *PayloadError) Error() string {
	return e.Message
}

// PreparedPayload separates the original webhook body from its LLM-safe representation.
type PreparedPayload struct {
	RawPayload    []byte
	RawPreview    string
	PromptPayload string
	Fields        map[string]interface{}
}

// PreparePayload validates, extracts and isolates an incoming webhook payload.
func PreparePayload(body []byte, contentType string, mappings []FieldMapping, maxPromptRunes int) (PreparedPayload, error) {
	if maxPromptRunes <= 0 {
		maxPromptRunes = defaultPromptPayloadRunes
	}
	prepared := PreparedPayload{
		RawPayload: append([]byte(nil), body...),
		RawPreview: truncateRunes(string(body), 500),
	}

	isJSON := isJSONContentType(contentType)
	if len(mappings) > 0 && !isJSON {
		return PreparedPayload{}, &PayloadError{StatusCode: http.StatusUnsupportedMediaType, Message: "field mappings require a JSON content type"}
	}

	if isJSON {
		if !json.Valid(body) {
			return PreparedPayload{}, &PayloadError{StatusCode: http.StatusBadRequest, Message: "invalid JSON payload"}
		}
		if len(mappings) > 0 {
			trimmed := bytes.TrimSpace(body)
			if len(trimmed) == 0 || trimmed[0] != '{' {
				return PreparedPayload{}, &PayloadError{StatusCode: http.StatusBadRequest, Message: "field mappings require a JSON object"}
			}
			var object map[string]interface{}
			if err := json.Unmarshal(trimmed, &object); err != nil {
				return PreparedPayload{}, &PayloadError{StatusCode: http.StatusBadRequest, Message: "field mappings require a JSON object"}
			}
			prepared.Fields = extractMappedFields(object, mappings)
		}

		var compact bytes.Buffer
		if err := json.Compact(&compact, body); err != nil {
			return PreparedPayload{}, &PayloadError{StatusCode: http.StatusBadRequest, Message: "invalid JSON payload"}
		}
		safeJSON := strings.NewReplacer("&", `\u0026`, "<", `\u003c`, ">", `\u003e`).Replace(compact.String())
		safeJSON = truncateJSONEnvelope(safeJSON, maxPromptRunes)
		prepared.PromptPayload = "<external_data>\n" + safeJSON + "\n</external_data>"
		return prepared, nil
	}

	text := truncateRunes(string(body), maxPromptRunes)
	prepared.PromptPayload = security.IsolateExternalData(text)
	if prepared.PromptPayload == "" {
		prepared.PromptPayload = "<external_data>\n\n</external_data>"
	}
	return prepared, nil
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func truncateJSONEnvelope(payload string, maxRunes int) string {
	if utf8.RuneCountInString(payload) <= maxRunes {
		return payload
	}
	type truncationEnvelope struct {
		Truncated     bool   `json:"truncated"`
		OriginalRunes int    `json:"original_runes"`
		PayloadPrefix string `json:"payload_prefix"`
	}
	runes := []rune(payload)
	for prefixLength := min(len(runes), maxRunes); prefixLength >= 0; prefixLength-- {
		encoded, err := json.Marshal(truncationEnvelope{
			Truncated:     true,
			OriginalRunes: len(runes),
			PayloadPrefix: string(runes[:prefixLength]),
		})
		if err == nil && utf8.RuneCount(encoded) <= maxRunes {
			return string(encoded)
		}
	}
	return `{"truncated":true}`
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	const marker = "... (truncated)"
	markerRunes := []rune(marker)
	if maxRunes <= len(markerRunes) {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-len(markerRunes)]) + marker
}

func extractMappedFields(raw map[string]interface{}, mappings []FieldMapping) map[string]interface{} {
	result := make(map[string]interface{}, len(mappings))
	for _, mapping := range mappings {
		alias := mapping.Alias
		if alias == "" {
			alias = strings.ReplaceAll(mapping.Source, ".", "_")
		}
		result[alias] = getNestedValue(raw, mapping.Source)
	}
	return result
}

func validateSignatureConfiguration(format WebhookFormat) error {
	header := strings.TrimSpace(format.SignatureHeader)
	algorithm := strings.ToLower(strings.TrimSpace(format.SignatureAlgo))
	secret := strings.TrimSpace(format.SignatureSecret)
	if header == "" && algorithm == "" && secret == "" {
		return nil
	}
	if header == "" || algorithm == "" || secret == "" {
		return fmt.Errorf("signature header, algorithm and secret must be configured together")
	}
	switch algorithm {
	case "sha256", "sha1", "plain":
		return nil
	default:
		return fmt.Errorf("unsupported signature algorithm %q", format.SignatureAlgo)
	}
}

// ValidateSignatureConfiguration verifies that signature settings are complete and supported.
func ValidateSignatureConfiguration(format WebhookFormat) error {
	return validateSignatureConfiguration(format)
}
