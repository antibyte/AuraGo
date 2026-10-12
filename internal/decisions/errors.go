package decisions

import (
	"context"
	"errors"
)

const (
	CodeServiceDisabled          = "service_disabled"
	CodeServiceNotReady          = "service_not_ready"
	CodeCPUConsentRequired       = "cpu_consent_required"
	CodeHardwareUnavailable      = "hardware_unavailable"
	CodeModelNotInstalled        = "model_not_installed"
	CodeModelIntegrityFailed     = "model_integrity_failed"
	CodeModelInstallFailed       = "model_install_failed"
	CodeModelInstallBusy         = "model_install_busy"
	CodeModelDownloadFailed      = "model_download_failed"
	CodeRuntimeAttestationFailed = "runtime_attestation_failed"
	CodeRuntimeUnavailable       = "runtime_unavailable"
	CodeRuntimeArtifactMissing   = "runtime_artifact_missing"
	CodeRuntimeStartFailed       = "runtime_start_failed"
	CodeDockerUnavailable        = "docker_unavailable"
	CodeDeploymentDenied         = "deployment_denied"
	CodeInvalidRequest           = "invalid_request"
	CodeCandidateNotScorable     = "candidate_not_single_token"
	CodeInputTokenBudgetExceeded = "input_token_budget_exceeded"
	CodeInvalidRuntimeResponse   = "invalid_runtime_response"
	CodeInferenceFailed          = "inference_failed"
	CodeCancelled                = "cancelled"
	CodeQueueFull                = "queue_full"
	CodeLifecycleBusy            = "lifecycle_busy"
	CodeRuntimeBusy              = "runtime_busy"
)

// Error carries a stable, safe-to-publish code and an optional wrapped cause.
// Error() intentionally exposes only the code; callers can log causes only
// after applying their own sensitive-data policy.
type Error struct {
	Code  string
	Cause error
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	return err.Code
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// NewError creates an error with a stable public classification.
func NewError(code string, cause error) error {
	if code == "" {
		code = CodeInferenceFailed
	}
	return &Error{Code: code, Cause: cause}
}

// ErrorCode returns an error's stable public code, or an appropriate generic
// code for unclassified cancellation and failures.
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var decisionsError *Error
	if errors.As(err, &decisionsError) && decisionsError.Code != "" {
		return decisionsError.Code
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return CodeCancelled
	}
	return CodeInferenceFailed
}

// IsNonRetryable reports whether repeating the same request without changing
// configuration or input is known not to help.
func IsNonRetryable(err error) bool {
	switch ErrorCode(err) {
	case CodeServiceDisabled, CodeCPUConsentRequired, CodeModelNotInstalled,
		CodeModelIntegrityFailed, CodeRuntimeArtifactMissing, CodeRuntimeStartFailed,
		CodeRuntimeUnavailable, CodeRuntimeAttestationFailed, CodeHardwareUnavailable,
		CodeDockerUnavailable, CodeInvalidRuntimeResponse, CodeInferenceFailed,
		CodeDeploymentDenied, CodeInvalidRequest, CodeCandidateNotScorable,
		CodeInputTokenBudgetExceeded:
		return true
	default:
		return false
	}
}

// WrapError preserves the original cause for errors.Is/As while keeping Error
// output limited to the stable code.
func WrapError(code string, cause error) error {
	return NewError(code, cause)
}
