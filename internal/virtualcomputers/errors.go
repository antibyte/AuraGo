package virtualcomputers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type ClassifiedError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func ClassifyError(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{}
	}
	classified := ClassifiedError{Code: "upstream_error", Message: err.Error(), HTTPStatus: http.StatusBadGateway}
	var restErr RESTError
	if !errors.As(err, &restErr) {
		lower := strings.ToLower(classified.Message)
		if strings.Contains(lower, " is required") || strings.Contains(lower, "must be") || strings.Contains(lower, "not supported") || strings.Contains(lower, "supports at most") {
			classified.Code, classified.HTTPStatus = "invalid_argument", http.StatusBadRequest
		}
		return classified
	}
	message := restErr.upstreamMessage()
	if message != "" {
		classified.Message = message
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "no vsock device") || strings.Contains(lower, "rfb"):
		classified.Code, classified.HTTPStatus = "capability_unavailable", http.StatusConflict
	case strings.Contains(lower, "connected computer"):
		classified.Code, classified.HTTPStatus = "machine_not_connected", http.StatusConflict
	case strings.Contains(lower, "busy"):
		classified.Code, classified.HTTPStatus = "machine_busy", http.StatusConflict
	case restErr.StatusCode == http.StatusNotFound && strings.Contains(restErr.Path, "/v1/volumes") && !looksLikeJSON(restErr.Body):
		classified.Code, classified.HTTPStatus = "storage_unavailable", http.StatusServiceUnavailable
	case restErr.StatusCode == http.StatusNotFound:
		classified.Code, classified.HTTPStatus = "not_found", http.StatusNotFound
	case restErr.StatusCode == http.StatusTooManyRequests:
		classified.Code, classified.HTTPStatus = "rate_limited", http.StatusTooManyRequests
	case restErr.StatusCode == http.StatusBadRequest:
		classified.Code, classified.HTTPStatus = "invalid_argument", http.StatusBadRequest
	}
	return classified
}

func (e RESTError) upstreamMessage() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return http.StatusText(e.StatusCode)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(body), &payload) == nil && strings.TrimSpace(payload.Error) != "" {
		return strings.TrimSpace(payload.Error)
	}
	return body
}

func looksLikeJSON(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}
