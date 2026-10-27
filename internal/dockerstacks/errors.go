package dockerstacks

import (
	"errors"
	"fmt"
)

// Error codes of this package. The server maps them to HTTP statuses.
const (
	CodeStackNameInvalid   = "stack_name_invalid"     // 400
	CodeStackNotFound      = "stack_not_found"        // 404
	CodeStackExists        = "stack_exists"           // 409
	CodeRevisionConflict   = "revision_conflict"      // 409, *ConflictError
	CodeOperationBusy      = "operation_in_progress"  // 409, *BusyError
	CodeOperationNotFound  = "operation_not_found"    // 404
	CodeInvalidOperation   = "invalid_operation"      // 400
	CodeFileTooLarge       = "file_too_large"         // 413
	CodeHostNotPrepared    = "host_not_prepared"      // 409, fields: helpers (HelperStatus)
	CodeRunnerImageMissing = "runner_image_missing"   // 409, fields: command ("docker pull docker:29.9.2-cli@sha256:...")
	CodeAPITooOld          = "api_too_old"            // 409, fields: api_version
	CodeStacksDirInvalid   = "stacks_dir_unavailable" // 409, the lister or IO container could not use the directory
	CodeConvertFailed      = "convert_failed"         // 400
)

// Codes that only appear in Operation.Code (an operation that failed).
const (
	opCodeTimeout         = "timeout"          // the maximum duration passed; the runner was killed
	opCodeRunnerFailed    = "runner_failed"    // the runner exited with a non-zero code
	opCodeHostUnreachable = "host_unreachable" // the Engine did not answer
	opCodeEngineError     = "engine_error"     // the Engine refused a request
	opCodeInterrupted     = "interrupted"      // AuraGo restarted and found nothing to reattach to
)

// ErrStackExists is wrapped by the *Error that Create returns when the stack
// already has a compose file (CodeStackExists); test it with errors.Is.
var ErrStackExists = errors.New("stack already exists")

// Error is a coded failure that the server maps to its HTTP status by Code.
// Engine and transport errors are returned unwrapped (dockerhost.StatusError and
// net errors) so the server's engineFailure keeps classifying them.
type Error struct {
	Code    string
	Message string         // English, one line, no secrets
	Fields  map[string]any // extra JSON fields for the error envelope
	Err     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// newError builds an *Error with a formatted message.
func newError(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// withFields returns the error with extra envelope fields.
func (e *Error) withFields(fields map[string]any) *Error {
	e.Fields = fields
	return e
}

// ConflictError is a write whose base revision is no longer the revision on the host.
type ConflictError struct {
	File    string      // "compose" | "env"
	Current FileContent // what is on the host now
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("the %s file changed on the host since it was read", e.File)
}

// BusyError is a second operation on a stack that already has one running.
type BusyError struct {
	OperationID string
	Kind        string
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("another operation (%s) is already running on this stack", e.Kind)
}

// CodeOf returns the code of an *Error, *ConflictError or *BusyError in the
// chain, or "" when err carries none of them.
func CodeOf(err error) string {
	var conflict *ConflictError
	var busy *BusyError
	var coded *Error
	switch {
	case errors.As(err, &conflict):
		return CodeRevisionConflict
	case errors.As(err, &busy):
		return CodeOperationBusy
	case errors.As(err, &coded):
		return coded.Code
	}
	return ""
}
