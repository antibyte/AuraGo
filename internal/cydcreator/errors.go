// Package cydcreator is the domain of the CYD Creator desktop app: board
// profiles, themes, the widget catalog, the project model with validation and
// layout patch operations, partition tables, the device config blob and the
// build diagnostics parser. The 1a files are pure (stdlib only); later plans add
// the generator, storage and the service to this package. It must never import
// internal/server, internal/agent or internal/tools.
package cydcreator

import (
	"errors"
	"sort"
)

// Error codes shared by every CYD Creator layer (overview "Error codes"). Later
// plans use these constants and must not redeclare them.
const (
	CodeDisabled            = "CYD_DISABLED"
	CodeDockerUnavailable   = "CYD_DOCKER_UNAVAILABLE"
	CodeImageMissing        = "CYD_IMAGE_MISSING"
	CodeImagePullFailed     = "CYD_IMAGE_PULL_FAILED"
	CodeBadRequest          = "CYD_BAD_REQUEST"
	CodeNotFound            = "CYD_NOT_FOUND"
	CodeReadOnly            = "CYD_READONLY"
	CodeBusy                = "CYD_BUSY"
	CodeProjectLocked       = "CYD_PROJECT_LOCKED"
	CodeProjectConflict     = "CYD_PROJECT_CONFLICT"
	CodeInvalidLayout       = "CYD_INVALID_LAYOUT"
	CodeFileReadOnly        = "CYD_FILE_READONLY"
	CodeFileConflict        = "CYD_FILE_CONFLICT"
	CodeFileLimit           = "CYD_FILE_LIMIT"
	CodeAssetTooLarge       = "CYD_ASSET_TOO_LARGE"
	CodeGeneratorError      = "CYD_GENERATOR_ERROR"
	CodeBuildFailed         = "CYD_BUILD_FAILED"
	CodeBuildTimeout        = "CYD_BUILD_TIMEOUT"
	CodeBuildCancelled      = "CYD_BUILD_CANCELLED"
	CodeBuildInterrupted    = "CYD_BUILD_INTERRUPTED"
	CodeFlashBudgetExceeded = "CYD_FLASH_BUDGET_EXCEEDED"
	CodeJobNotFound         = "CYD_JOB_NOT_FOUND"
	CodeJobState            = "CYD_JOB_STATE"
	CodeDeviceNotFound      = "CYD_DEVICE_NOT_FOUND"
	CodeUnauthorized        = "CYD_UNAUTHORIZED"
	CodeRateLimited         = "CYD_RATE_LIMITED"
	CodeOTADisabled         = "CYD_OTA_DISABLED"
	CodeImportInvalid       = "CYD_IMPORT_INVALID"
	CodeTemplateTooNew      = "CYD_TEMPLATE_TOO_NEW"
	CodeFlashWrongChip      = "CYD_FLASH_WRONG_CHIP"
	CodeFlashPortBusy       = "CYD_FLASH_PORT_BUSY"
	CodeFlashDisconnected   = "CYD_FLASH_DISCONNECTED"
	CodeSerialUnsupported   = "CYD_SERIAL_UNSUPPORTED"
)

var allErrorCodes = []string{
	CodeDisabled, CodeDockerUnavailable, CodeImageMissing, CodeImagePullFailed, CodeBadRequest,
	CodeNotFound, CodeReadOnly, CodeBusy, CodeProjectLocked, CodeProjectConflict, CodeInvalidLayout,
	CodeFileReadOnly, CodeFileConflict, CodeFileLimit, CodeAssetTooLarge, CodeGeneratorError,
	CodeBuildFailed, CodeBuildTimeout, CodeBuildCancelled, CodeBuildInterrupted, CodeFlashBudgetExceeded,
	CodeJobNotFound, CodeJobState, CodeDeviceNotFound, CodeUnauthorized, CodeRateLimited, CodeOTADisabled,
	CodeImportInvalid, CodeTemplateTooNew, CodeFlashWrongChip, CodeFlashPortBusy, CodeFlashDisconnected,
	CodeSerialUnsupported,
}

// ErrorCodes returns every CYD_* code, sorted. The UI i18n tests use it to
// require a cyd_creator.error.<CODE> key per code.
func ErrorCodes() []string {
	out := append([]string(nil), allErrorCodes...)
	sort.Strings(out)
	return out
}

// Error is the typed error every layer returns; Code is one of the CYD_* codes.
// Message is English (logs, agent); the UI translates by Code.
type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"error"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// NewError builds a typed error. details may be nil.
func NewError(code, message string, details map[string]any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}

// ErrorCode returns the CYD_* code of err, or "" when err does not wrap *Error.
func ErrorCode(err error) string {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e.Code
	}
	return ""
}
