package ruview

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// Stable error codes. The strings are a contract with the UI model and the
// translations; never rename one. HTTP statuses live in errorHTTP below.
const (
	CodeDisabled             = "disabled"
	CodeReadOnly             = "readonly"
	CodeExternalRuntime      = "external_runtime"
	CodeDockerReadOnly       = "docker_readonly" // Docker integration off or read-only
	CodeAlreadyRunning       = "already_running"
	CodeNotInstalled         = "not_installed"
	CodeBusy                 = "busy"      // another start/install holds the lifecycle slot
	CodeCancelled            = "cancelled" // generation cancelled by Stop/Configure/shutdown
	CodeContainerConflict    = "container_conflict"
	CodePlatformUnsupported  = "platform_unsupported"
	CodeInvalidNodeID        = "invalid_node_id"
	CodeInvalidConfig        = "invalid_config"
	CodeUnreachable          = "unreachable"        // transport failure
	CodeUpstreamAuth         = "upstream_auth"      // upstream 401/403
	CodeUpstreamError        = "upstream_error"     // other non-2xx
	CodeDockerUnavailable    = "docker_unavailable" // Engine transport failure
	CodeImageDenied          = "image_denied"
	CodeImagePullFailed      = "image_pull_failed"
	CodeContractUnsupported  = "contract_unsupported" // invalid JSON / status != ready
	CodeDownloadFailed       = "download_failed"
	CodeStartFailed          = "start_failed" // any other start failure (dir, vault, placement, create/start)
	CodeStopFailed           = "stop_failed"
	CodeProcessExited        = "process_exited"        // status-only in practice
	CodeSourceMismatch       = "source_mismatch"       // status-only
	CodeUnexpectedSimulation = "unexpected_simulation" // status-only
	CodeTimeout              = "timeout"
	CodeStorageUnreadable    = "storage_unreadable"
	CodeStorageFull          = "storage_full"
	CodeArtifactMissing      = "artifact_missing" // no verified digest/package in this release
	CodeUnknown              = "unknown"          // fallback for untyped errors
)

// errorHTTP is the status internal/server writes for each code (design §8.4).
var errorHTTP = map[string]int{
	CodeDisabled:             403,
	CodeReadOnly:             403,
	CodeExternalRuntime:      403,
	CodeDockerReadOnly:       403,
	CodeAlreadyRunning:       409,
	CodeNotInstalled:         409,
	CodeBusy:                 409,
	CodeCancelled:            409,
	CodeContainerConflict:    409,
	CodePlatformUnsupported:  409,
	CodeInvalidNodeID:        400,
	CodeInvalidConfig:        400,
	CodeUnreachable:          502,
	CodeUpstreamAuth:         502,
	CodeUpstreamError:        502,
	CodeDockerUnavailable:    502,
	CodeImageDenied:          502,
	CodeImagePullFailed:      502,
	CodeContractUnsupported:  502,
	CodeDownloadFailed:       502,
	CodeStartFailed:          502,
	CodeStopFailed:           502,
	CodeProcessExited:        502,
	CodeSourceMismatch:       502,
	CodeUnexpectedSimulation: 502,
	CodeTimeout:              504,
	CodeStorageUnreadable:    503,
	CodeStorageFull:          507,
	CodeArtifactMissing:      500,
	CodeUnknown:              500,
}

// codeText is the short English sentence used when an Error carries no message
// of its own. The UI never shows it: it translates the code instead.
var codeText = map[string]string{
	CodeDisabled:             "RuView is disabled",
	CodeReadOnly:             "RuView is read-only",
	CodeExternalRuntime:      "external RuView is managed by its operator",
	CodeDockerReadOnly:       "managed RuView requires writable Docker integration",
	CodeAlreadyRunning:       "RuView is already started",
	CodeNotInstalled:         "RuView native package is not installed",
	CodeBusy:                 "RuView is busy with another start or installation",
	CodeCancelled:            "RuView action was cancelled",
	CodeContainerConflict:    "a container named aurago-ruview belongs to another workload",
	CodePlatformUnsupported:  "no verified native RuView package for this platform in this release",
	CodeInvalidNodeID:        "RuView node ID must be between 1 and 255",
	CodeInvalidConfig:        "RuView configuration is invalid",
	CodeUnreachable:          "RuView service could not be reached",
	CodeUpstreamAuth:         "RuView rejected the access token",
	CodeUpstreamError:        "RuView answered with an error",
	CodeDockerUnavailable:    "Docker could not be reached",
	CodeImageDenied:          "the registry denied the RuView image",
	CodeImagePullFailed:      "the RuView image could not be downloaded",
	CodeContractUnsupported:  "RuView status contract is unsupported",
	CodeDownloadFailed:       "RuView download failed",
	CodeStartFailed:          "RuView could not be started",
	CodeStopFailed:           "RuView stop was not confirmed",
	CodeProcessExited:        "RuView stopped unexpectedly",
	CodeSourceMismatch:       "RuView uses a different signal source than configured",
	CodeUnexpectedSimulation: "RuView sends simulated data although real sensing is configured",
	CodeTimeout:              "RuView did not answer in time",
	CodeStorageUnreadable:    "RuView storage could not be checked",
	CodeStorageFull:          "RuView storage budget reached",
	CodeArtifactMissing:      "no verified RuView package in this release",
	CodeUnknown:              "RuView failed unexpectedly",
}

// maxDetailBytes bounds Error.Detail and Status.ErrorDetail; maxLegacyBytes
// bounds the legacy sentence (Status.Error, the "error" response field).
const (
	maxDetailBytes = 240
	maxLegacyBytes = 600
)

// Error is a RuView failure with a stable code. Detail is scrubbed: no token,
// URL or file path, at most 240 bytes. HTTP is the status internal/server writes.
type Error struct {
	Code   string
	Detail string
	HTTP   int
	msg    string // legacy English sentence: Status.Error, the "error" response field; redacted like Detail (scrubDetail)
	cause  error  // errors.Is/As only; never serialized
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.msg != "" {
		return e.msg
	}
	text := codeText[e.Code]
	if text == "" {
		text = e.Code
	}
	if e.Detail != "" {
		text += ": " + e.Detail
	}
	return text
}

func (e *Error) Unwrap() error { return e.cause }

// HTTPStatus returns the status for a code; unknown codes map to 500.
func HTTPStatus(code string) int {
	if status, ok := errorHTTP[code]; ok {
		return status
	}
	return 500
}

// ErrorCodes lists every Code* constant, sorted.
func ErrorCodes() []string {
	codes := make([]string, 0, len(errorHTTP))
	for code := range errorHTTP {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// knownCodeOr returns code when it is a declared Code*, else fallback.
func knownCodeOr(code, fallback string) string {
	if _, ok := errorHTTP[code]; ok {
		return code
	}
	return fallback
}

// AsError returns the typed form of err: nil for nil, the first *Error in the
// chain (with HTTP filled in), cancelled/timeout for context errors, and
// unknown with a scrubbed detail for anything else.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var typed *Error
	if errors.As(err, &typed) && typed != nil {
		if typed.HTTP != 0 {
			return typed
		}
		filled := *typed
		filled.HTTP = HTTPStatus(filled.Code)
		return &filled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ctxError(context.DeadlineExceeded)
	}
	if errors.Is(err, context.Canceled) {
		return ctxError(context.Canceled)
	}
	return &Error{Code: CodeUnknown, Detail: scrubDetail(err.Error(), maxDetailBytes), HTTP: HTTPStatus(CodeUnknown), cause: err}
}

// fail builds a typed error. msg is the legacy sentence; it is redacted here
// exactly like Detail (scrubDetail), so no caller can leak a path, URL, address
// or secret through the "error" field. The cause only feeds Detail and
// errors.Is/As; the server log gets it raw through rawText.
func fail(code, msg string, cause error) *Error {
	e := &Error{Code: code, HTTP: HTTPStatus(code), msg: scrubDetail(msg, maxLegacyBytes), cause: cause}
	if cause != nil {
		e.Detail = scrubDetail(cause.Error(), maxDetailBytes)
	}
	return e
}

// ctxError types a context failure: a deadline is a timeout, anything else a cancellation.
func ctxError(err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: CodeTimeout, HTTP: HTTPStatus(CodeTimeout), msg: codeText[CodeTimeout], cause: err}
	}
	if err == nil {
		err = context.Canceled
	}
	return &Error{Code: CodeCancelled, HTTP: HTTPStatus(CodeCancelled), msg: codeText[CodeCancelled], cause: err}
}

var (
	detailURL = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S+`)
	// Paths may contain spaces ("C:\Program Files", "/home/Andi Jakob"), so a
	// path runs to the next ": " (the usual "path: reason" shape), quote or
	// closing parenthesis, or to the end of the text.
	detailWinPath  = regexp.MustCompile(`(?i)(?:\b[a-z]:[\\/]|\\\\)[^"']*?(:\s|["']|$)`)
	detailUnixPath = regexp.MustCompile(`(^|[\s"'(=])/[^\s"')][^"']*?(:\s|["')]|$)`)
	detailIPv4Port = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}:\d{1,5}\b`)
	detailIPv6Port = regexp.MustCompile(`\[[0-9a-fA-F:.]+\]:\d{1,5}`)
)

// scrubDetail makes a failure description safe for the UI: secrets, URLs,
// file paths and network addresses are removed, control characters become
// spaces, whitespace is collapsed and the result is cut to limit bytes at a
// rune boundary.
func scrubDetail(text string, limit int) string {
	text = security.Scrub(text)
	text = detailURL.ReplaceAllString(text, "[url]")
	text = detailWinPath.ReplaceAllString(text, "[path]${1}")
	text = detailUnixPath.ReplaceAllString(text, "${1}[path]${2}")
	text = detailIPv4Port.ReplaceAllString(text, "[addr]")
	text = detailIPv6Port.ReplaceAllString(text, "[addr]")
	text = strings.Join(strings.Fields(dockerutil.SanitizeOneLine(text, 0)), " ")
	if limit > 0 && len(text) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut]
	}
	return strings.TrimSpace(text)
}

// legacyText is the scrubbed one-line form of err used for Status.Error, the
// "error" response field and logs that leave the process: no secret, URL, path
// or address. The raw text only goes to the server log (rawText).
func legacyText(err error) string {
	if err == nil {
		return ""
	}
	return scrubDetail(err.Error(), maxLegacyBytes)
}

// rawText is the unredacted text of err, with the cause chain, for the server
// log. Registered secrets are still scrubbed. Never send it to a client.
func rawText(err error) string {
	if err == nil {
		return ""
	}
	var typed *Error
	if errors.As(err, &typed) && typed != nil && typed.cause != nil {
		return security.Scrub(typed.Error() + " (" + typed.cause.Error() + ")")
	}
	return security.Scrub(err.Error())
}
