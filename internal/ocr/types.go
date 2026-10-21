// Package ocr owns the optional Indra service and its transient, owner-bound jobs.
package ocr

import (
	"context"
	"errors"
	"time"
)

const (
	Version               = "1.1.2"
	ModelRevision         = "bd02167fdf2005a19962cc15188f8496a2c87d3b"
	MaxFiles              = 20
	MaxFileBytes    int64 = 32 << 20
	WorkerTempBytes int64 = 64 << 20
	MaxStorageBytes int64 = (1 << 30) - WorkerTempBytes
	MaxResultBytes  int64 = 16 << 20
	MaxPages              = 100
	FileTimeout           = 300 * time.Second
	Retention             = 24 * time.Hour
	OwnerLabel            = "ocr"
	ContainerName         = "aurago-ocr"
)

var (
	ErrDisabled = errors.New("ocr_disabled")
	ErrNotReady = errors.New("ocr_not_ready")
	ErrNotFound = errors.New("ocr_not_found")
	ErrLimit    = errors.New("ocr_limit_exceeded")
	ErrInvalid  = errors.New("ocr_invalid_input")
	ErrBusy     = errors.New("ocr_busy")
)

type Page struct {
	Index    int     `json:"index"`
	Text     string  `json:"text"`
	Markdown string  `json:"markdown"`
	Source   string  `json:"source"`
	Lines    []Line  `json:"lines,omitempty"`
	Blocks   []Block `json:"blocks,omitempty"`
	Note     string  `json:"note,omitempty"`
	Preview  string  `json:"preview,omitempty"`
}
type Line struct {
	Box        []float64 `json:"box"`
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
}
type Block struct {
	Label string    `json:"label"`
	BBox  []float64 `json:"bbox"`
	Text  string    `json:"text"`
}
type File struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MIME      string `json:"mime_type"`
	Size      int64  `json:"size_bytes"`
	State     string `json:"state"`
	PageCount int    `json:"page_count"`
	Pages     []Page `json:"pages,omitempty"`
	Error     string `json:"error,omitempty"`
	path      string
	digest    string
}
type Job struct {
	ID         string     `json:"id"`
	State      string     `json:"state"`
	Mode       string     `json:"mode"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Files      []File     `json:"files"`
	owner      string
	cancel     context.CancelFunc
}
type Status struct {
	State         string `json:"state"`
	Ready         bool   `json:"ready"`
	Installed     bool   `json:"installed"`
	Code          string `json:"code,omitempty"`
	Version       string `json:"version"`
	ModelRevision string `json:"model_revision"`
	Queued        int    `json:"queued"`
}
type Policy struct {
	InstanceKey    string
	Enabled        bool
	DockerEnabled  bool
	DockerReadOnly bool
	DockerHost     string
	InDocker       bool
}

// Runtime keeps Engine/lifecycle operations separate from the job coordinator.
type Runtime interface {
	Start(context.Context, bool) error
	Stop(context.Context) error
	Convert(context.Context, string, string, string, func(Page) error) error
}

func Supported(name string) bool { _, ok := extensionMIME(name); return ok }
