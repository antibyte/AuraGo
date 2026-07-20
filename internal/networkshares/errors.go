package networkshares

import (
	"errors"
	"fmt"
)

const (
	ErrorDisabled         = "NETWORK_SHARES_DISABLED"
	ErrorUnavailable      = "SHARE_PROTOCOL_UNAVAILABLE"
	ErrorOutsideRoot      = "SHARE_OUTSIDE_ALLOWED_ROOT"
	ErrorReadOnly         = "SHARE_READ_ONLY"
	ErrorPermissionDenied = "SHARE_PERMISSION_DENIED"
	ErrorNotManaged       = "SHARE_NOT_MANAGED"
	ErrorConflict         = "SHARE_CONFLICT"
	ErrorDrift            = "SHARE_DRIFT"
	ErrorApplyFailed      = "SHARE_APPLY_FAILED"
	ErrorInvalidArgument  = "SHARE_INVALID_ARGUMENT"
	ErrorNotFound         = "SHARE_NOT_FOUND"
)

// CodedError provides stable API and agent-tool error identifiers.
type CodedError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *CodedError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *CodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func codedError(code, message string, err error) error {
	return &CodedError{Code: code, Message: message, Err: err}
}

// ErrorCode extracts a stable network-share error code.
func ErrorCode(err error) string {
	var coded *CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func wrapApplyError(action string, err error) error {
	if err == nil {
		return nil
	}
	return codedError(ErrorApplyFailed, fmt.Sprintf("Could not %s the network share.", action), err)
}
