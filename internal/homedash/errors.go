package homedash

// Error is every refusal HomeDash reports. Code is a fixed machine code; Message is English and
// never contains upstream text.
type Error struct {
	Code            string  `json:"code"`
	Message         string  `json:"message"`
	BlockedBy       string  `json:"blocked_by,omitempty"`
	RetryAfterMS    int64   `json:"retry_after_ms,omitempty"`
	CurrentRevision int64   `json:"current_revision,omitempty"`
	Issues          []Issue `json:"issues,omitempty"`
}

// Issue is one validation finding; Path is a JSON pointer into the document.
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Is matches by code, so errors.Is(err, ErrTooManyStreams) works for copies.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

func newError(code, msg string) *Error { return &Error{Code: code, Message: msg} }

const (
	CodeBadRequest           = "bad_request"
	CodeInvalidID            = "invalid_id"
	CodeInvalidArgs          = "invalid_args"
	CodeUnsupportedCommand   = "unsupported_command"
	CodePairingInvalid       = "pairing_invalid"
	CodeKioskUnpaired        = "kiosk_unpaired"
	CodeKioskRevoked         = "kiosk_revoked"
	CodeKioskDisabled        = "kiosk_disabled"
	CodeReadOnly             = "read_only"
	CodeForbiddenEntity      = "forbidden_entity"
	CodeViewOnly             = "view_only"
	CodeDashboardNotFound    = "dashboard_not_found"
	CodeEntityNotFound       = "entity_not_found"
	CodeStreamNotFound       = "stream_not_found"
	CodeRevisionNotFound     = "revision_not_found"
	CodeRevisionConflict     = "revision_conflict"
	CodeConfirmationRequired = "confirmation_required"
	CodeSchemaTooNew         = "schema_too_new"
	CodeSlugTaken            = "slug_taken"
	CodeInvalidDocument      = "invalid_document"
	CodeInvalidPatch         = "invalid_patch"
	CodeLimitExceeded        = "limit_exceeded"
	CodeRateLimited          = "rate_limited"
	CodeTooManyStreams       = "too_many_streams"
	CodeCommandFailed        = "command_failed"
	CodeSourceError          = "source_error"
	CodeHomeDashDisabled     = "homedash_disabled"
	CodeSourceUnavailable    = "source_unavailable"
	CodeHubClosed            = "hub_closed"
)

var (
	ErrHubClosed         = newError(CodeHubClosed, "HomeDash is shutting down")
	ErrTooManyStreams    = newError(CodeTooManyStreams, "too many open streams")
	ErrEntityNotFound    = newError(CodeEntityNotFound, "unknown entity")
	ErrDashboardNotFound = newError(CodeDashboardNotFound, "unknown dashboard")
)
