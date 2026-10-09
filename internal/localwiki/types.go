// Package localwiki manages the offline Wikipedia edition: Kiwix catalog,
// download, verification, publication, update checks and the shared Library
// handle used by the agent tool, the admin API and the desktop app.
package localwiki

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Variant is a Kiwix flavour offered by Local Wikipedia.
type Variant string

const (
	VariantNoPic Variant = "nopic"
	VariantMaxi  Variant = "maxi"
)

// Edition describes one Kiwix Wikipedia ZIM edition.
type Edition struct {
	Language     string    `json:"language"`
	Variant      Variant   `json:"variant"`
	Date         string    `json:"date"`
	Name         string    `json:"name"`
	FileName     string    `json:"file_name"`
	Size         int64     `json:"size"`
	SHA256       string    `json:"sha256"`
	UUID         string    `json:"uuid"`
	ArticleCount int       `json:"article_count"`
	InstalledAt  time.Time `json:"installed_at"`
}

// Deps are the manager's replaceable dependencies.
type Deps struct {
	Logger         *slog.Logger
	HTTPClient     *http.Client                // nil → default with timeouts, HTTPS-only redirects
	FreeDiskBytes  func(string) (int64, error) // nil → fileutil.FreeDiskBytes
	Now            func() time.Time            // nil → time.Now
	CatalogBaseURL string                      // "" → https://opds.library.kiwix.org
	// IsSensitivePath refuses storage directories AuraGo must never write to.
	// nil accepts every absolute directory.
	IsSensitivePath func(string) bool
}

// Settings is the resolved configuration the manager acts on.
type Settings struct {
	Enabled        bool
	AgentAccess    bool
	Language       string // resolved (system language applied), e.g. "de"
	SystemLanguage string // resolved code an empty local_wikipedia.language stands for
	Variant        Variant
	DataDir        string // resolved absolute directory
	DataDirLocked  bool   // true when AuraGo runs in Docker
	UpdateCheck    bool
}

// InstallRequest is the body of POST /api/local-wikipedia/install.
type InstallRequest struct {
	ReplaceMode         string `json:"replace_mode"` // "keep_old" (default) | "delete_old_first"
	ConfirmUnknownSpace bool   `json:"confirm_unknown_space"`
}

const (
	ReplaceKeepOld        = "keep_old"
	ReplaceDeleteOldFirst = "delete_old_first"
)

// Preflight errors returned synchronously by the manager.
var (
	ErrBusy               = errors.New("localwiki: operation in progress")
	ErrDisabled           = errors.New("localwiki: integration disabled")
	ErrUnknownFreeSpace   = errors.New("localwiki: free space unknown")
	ErrDataDirInvalid     = errors.New("localwiki: storage directory invalid")
	ErrCatalogUnreachable = errors.New("localwiki: catalog unreachable")
	ErrAlreadyInstalled   = errors.New("localwiki: edition already installed")
	ErrNoOperation        = errors.New("localwiki: no operation in progress")
	ErrUnknownLanguage    = errors.New("localwiki: language not offered")
)

// Background failures recorded in the status.
var (
	errDownloadFailed   = errors.New("download_failed")
	errChecksumMismatch = errors.New("checksum_mismatch")
	errZIMUnreadable    = errors.New("zim_unreadable")
)

// InsufficientSpaceError reports a download that does not fit with the reserve.
type InsufficientSpaceError struct {
	Required, Available int64
	CanDeleteOld        bool
}

func (e *InsufficientSpaceError) Error() string {
	return fmt.Sprintf("localwiki: insufficient disk space: %d bytes required, %d available", e.Required, e.Available)
}

// States reported in Status.State.
const (
	StateNotInstalled = "not_installed"
	StateDownloading  = "downloading"
	StateVerifying    = "verifying"
	StateReady        = "ready"
	StateInterrupted  = "interrupted"
	StateError        = "error"
)

// Stable error codes for the status and the admin API.
const (
	CodeInsufficientDiskSpace = "insufficient_disk_space"
	CodeFreeSpaceUnknown      = "free_space_unknown"
	CodeChecksumMismatch      = "checksum_mismatch"
	CodeDownloadFailed        = "download_failed"
	CodeCatalogUnreachable    = "catalog_unreachable"
	CodeZIMUnreadable         = "zim_unreadable"
	CodeFulltextUnsupported   = "fulltext_unsupported"
	CodeBusy                  = "busy"
	CodeDisabled              = "disabled"
	CodeDataDirInvalid        = "data_dir_invalid"
	CodeAlreadyInstalled      = "already_installed"
	CodeNoOperation           = "no_operation"
	CodeUnknownLanguage       = "unknown_language"
	CodeInternal              = "localwiki_error"
)

// ErrorCode maps an error returned by the manager to its stable code.
func ErrorCode(err error) string {
	var space *InsufficientSpaceError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &space):
		return CodeInsufficientDiskSpace
	case errors.Is(err, ErrBusy):
		return CodeBusy
	case errors.Is(err, ErrDisabled):
		return CodeDisabled
	case errors.Is(err, ErrUnknownFreeSpace):
		return CodeFreeSpaceUnknown
	case errors.Is(err, ErrDataDirInvalid):
		return CodeDataDirInvalid
	case errors.Is(err, ErrCatalogUnreachable):
		return CodeCatalogUnreachable
	case errors.Is(err, ErrAlreadyInstalled):
		return CodeAlreadyInstalled
	case errors.Is(err, ErrNoOperation):
		return CodeNoOperation
	case errors.Is(err, ErrUnknownLanguage):
		return CodeUnknownLanguage
	case errors.Is(err, errChecksumMismatch):
		return CodeChecksumMismatch
	case errors.Is(err, errZIMUnreadable):
		return CodeZIMUnreadable
	case errors.Is(err, errDownloadFailed):
		return CodeDownloadFailed
	default:
		return CodeInternal
	}
}

// Recommendation is the English hint shown next to an error code; the config UI
// shows its own translation for every known code.
func Recommendation(code string) string {
	switch code {
	case "":
		return ""
	case CodeInsufficientDiskSpace:
		return "Free up disk space or choose another storage directory, then resume."
	case CodeFreeSpaceUnknown:
		return "Confirm the download explicitly; AuraGo cannot measure the free space."
	case CodeChecksumMismatch:
		return "The damaged download was removed. Install again."
	case CodeDownloadFailed:
		return "Check the internet connection and resume the download."
	case CodeCatalogUnreachable:
		return "Check the internet connection and retry. The installed edition keeps working."
	case CodeZIMUnreadable:
		return "Delete the edition and install it again."
	case CodeFulltextUnsupported:
		return "Search uses article titles only for this edition."
	case CodeBusy:
		return "Wait until the running operation has finished."
	case CodeDisabled:
		return "Enable Local Wikipedia and save the configuration."
	case CodeDataDirInvalid:
		return "Choose an absolute, writable directory outside system locations."
	case CodeAlreadyInstalled:
		return "This edition is already installed."
	case CodeNoOperation:
		return "No download is running."
	case CodeUnknownLanguage:
		return "Choose one of the offered languages."
	default:
		return "Check the AuraGo log for details."
	}
}

// operationRecommendation is the hint for a code that ended an install, resume
// or update. It differs from Recommendation where the code means something
// else for a download than for the installed edition.
func operationRecommendation(code string) string {
	if code == CodeZIMUnreadable {
		return "The downloaded file could not be read and was removed. Install again."
	}
	return Recommendation(code)
}

// Status is the JSON body of GET /api/local-wikipedia/status. FreeBytes is -1
// when the free space cannot be measured.
type Status struct {
	State                     string         `json:"state"` // not_installed|downloading|verifying|ready|interrupted|error
	Progress                  float64        `json:"progress"`
	BytesDone                 int64          `json:"bytes_done"`
	BytesTotal                int64          `json:"bytes_total"`
	RateBytesPerSec           int64          `json:"rate"`
	ETASeconds                int64          `json:"eta_seconds"`
	Edition                   *Edition       `json:"edition,omitempty"`
	Selection                 Selection      `json:"selection"`
	SelectionMatchesInstalled bool           `json:"selection_matches_installed"`
	UpdateAvailable           *UpdateInfo    `json:"update_available,omitempty"`
	Fulltext                  bool           `json:"fulltext"`
	FreeBytes                 int64          `json:"free_bytes"`
	RequiredBytes             int64          `json:"required_bytes"`
	DataDir                   string         `json:"data_dir"`
	DataDirLocked             bool           `json:"data_dir_locked"`
	OperationInProgress       bool           `json:"operation_in_progress"`
	ErrorCode                 string         `json:"error_code,omitempty"`
	Recommendation            string         `json:"recommendation,omitempty"`
	SystemLanguage            string         `json:"system_language"`
	Languages                 []LanguageInfo `json:"languages"`
}

type Selection struct {
	Language string  `json:"language"`
	Variant  Variant `json:"variant"`
}

type UpdateInfo struct {
	Date string `json:"date"`
	Size int64  `json:"size"`
}

type CatalogInfo struct {
	Language string                     `json:"language"`
	Fulltext bool                       `json:"fulltext"`
	Variants map[Variant]CatalogEdition `json:"variants"`
}

type CatalogEdition struct {
	Name         string `json:"name"`
	Date         string `json:"date"`
	Size         int64  `json:"size"` // approximate from OPDS until .meta4 is fetched at install
	ArticleCount int    `json:"article_count"`
	Meta4URL     string `json:"meta4_url"`
}

// LanguageInfo is one entry of the language dropdown.
type LanguageInfo struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Fulltext bool   `json:"fulltext"`
}
