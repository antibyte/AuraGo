package webhooks

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var promptPlaceholderPattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// ValidatePromptTemplate verifies that a webhook prompt template only uses the allowed placeholder set.
func ValidatePromptTemplate(tmpl string) error {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = DefaultPromptTemplate
	}

	matches := promptPlaceholderPattern.FindAllStringSubmatchIndex(tmpl, -1)
	for _, match := range matches {
		token := strings.TrimSpace(tmpl[match[2]:match[3]])
		if err := validatePromptToken(token); err != nil {
			return err
		}
	}

	cleaned := promptPlaceholderPattern.ReplaceAllString(tmpl, "")
	if strings.Contains(cleaned, "{{") || strings.Contains(cleaned, "}}") {
		return fmt.Errorf("invalid prompt template: malformed placeholder syntax")
	}

	return nil
}

func renderPromptTemplate(tmpl string, data PromptData) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = DefaultPromptTemplate
	}
	if err := ValidatePromptTemplate(tmpl); err != nil {
		return "", err
	}

	var renderErr error
	rendered := promptPlaceholderPattern.ReplaceAllStringFunc(tmpl, func(raw string) string {
		if renderErr != nil {
			return raw
		}
		match := promptPlaceholderPattern.FindStringSubmatch(raw)
		token := strings.TrimSpace(match[1])
		value, err := resolvePromptToken(token, data)
		if err != nil {
			renderErr = err
			return raw
		}
		return value
	})
	if renderErr != nil {
		return "", renderErr
	}
	return rendered, nil
}

func validatePromptToken(token string) error {
	switch token {
	case "webhook_name", "slug", "payload", "timestamp":
		return nil
	}
	if strings.HasPrefix(token, "field.") {
		key := strings.TrimSpace(strings.TrimPrefix(token, "field."))
		if key == "" {
			return fmt.Errorf("invalid prompt template: field placeholder requires a name")
		}
		return nil
	}
	if strings.HasPrefix(token, "header.") {
		key := strings.TrimSpace(strings.TrimPrefix(token, "header."))
		if key == "" {
			return fmt.Errorf("invalid prompt template: header placeholder requires a name")
		}
		return nil
	}
	return fmt.Errorf("invalid prompt template: unsupported placeholder %q", token)
}

func resolvePromptToken(token string, data PromptData) (string, error) {
	switch token {
	case "webhook_name":
		return data.WebhookName, nil
	case "slug":
		return data.Slug, nil
	case "payload":
		return data.Payload, nil
	case "timestamp":
		return data.Timestamp, nil
	}
	if strings.HasPrefix(token, "field.") {
		key := strings.TrimSpace(strings.TrimPrefix(token, "field."))
		value, ok := data.Fields[key]
		if !ok {
			return "", fmt.Errorf("invalid prompt template: unknown field placeholder %q", key)
		}
		return fmt.Sprint(value), nil
	}
	if strings.HasPrefix(token, "header.") {
		key := strings.TrimSpace(strings.TrimPrefix(token, "header."))
		if value, ok := data.Headers[key]; ok {
			return value, nil
		}
		if value, ok := data.Headers[http.CanonicalHeaderKey(key)]; ok {
			return value, nil
		}
		return "", fmt.Errorf("invalid prompt template: unknown header placeholder %q", key)
	}
	return "", fmt.Errorf("invalid prompt template: unsupported placeholder %q", token)
}
