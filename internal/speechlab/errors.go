package speechlab

import (
	"encoding/json"
	"net/http"
)

// ResponseError carries an application-owned message and code, never an upstream body.
type ResponseError struct {
	StatusCode int
	Code       string
}

func (e *ResponseError) Error() string {
	switch e.Code {
	case "voice_not_active":
		return "The requested voice is no longer active. Refresh the Speech Lab stack."
	case "module_not_provisioned", "model_not_installed", "host_module_delivery_pending":
		return "The selected speech module is not installed or ready. Check the Browser Lab."
	case "speech_lab_invalid_voice":
		return "Select a voice supported by the selected TTS backend."
	case "speech_lab_voice_mode_unsupported":
		return "The TTS catalog declares an unsupported voice mode."
	case "speech_lab_invalid_stack":
		return "Select an available ASR and TTS backend."
	case "speech_lab_stack_busy":
		return "Speech Lab stack is busy. Try again after the current operation or call."
	case "stack_activation_failed", "speech_lab_conflict":
		return "Speech Lab could not apply this change. Refresh the stack and check the Browser Lab."
	default:
		return "Speech Lab request failed."
	}
}

func upstreamResponseError(status int, body []byte) error {
	result := &ResponseError{StatusCode: http.StatusBadGateway, Code: "speech_lab_unavailable"}
	if status == http.StatusServiceUnavailable {
		result.StatusCode = status
	}
	if status != http.StatusConflict {
		return result
	}
	result.StatusCode, result.Code = http.StatusConflict, "speech_lab_conflict"
	var wire struct {
		Code      string `json:"code"`
		ErrorCode string `json:"error_code"`
	}
	if json.Unmarshal(body, &wire) != nil {
		return result
	}
	code := wire.Code
	if code == "" {
		code = wire.ErrorCode
	}
	switch code {
	case "voice_not_active", "module_not_provisioned", "model_not_installed", "host_module_delivery_pending", "stack_activation_failed":
		result.Code = code
	}
	return result
}
