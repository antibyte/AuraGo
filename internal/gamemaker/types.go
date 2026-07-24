package gamemaker

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	PhaserVersion = "4.2.1"
	ThreeVersion  = "0.185.1"
)

var (
	ErrDisabled       = errors.New("game maker is disabled")
	ErrReadOnly       = errors.New("game maker is read-only")
	ErrBusy           = errors.New("another game maker job is active")
	ErrNotFound       = errors.New("game maker record not found")
	ErrInvalidPath    = errors.New("invalid game maker path")
	ErrInvalidToken   = errors.New("invalid or expired preview token")
	ErrSkillsUnusable = errors.New("curated game maker skills are not verified")
)

// Options contains the resolved service configuration. Paths are runtime-only
// and are never serialized through the public API.
type Options struct {
	DBPath               string
	WorkspacePath        string
	Enabled              bool
	ReadOnly             bool
	AllowCreate          bool
	AllowEdit            bool
	AllowDelete          bool
	AllowMediaGeneration bool
	MaxProjects          int
	MaxFilesPerProject   int
	MaxFileBytes         int64
	MaxAssetBytes        int64
	MaxProjectBytes      int64
	JobTimeout           time.Duration
	Logger               *slog.Logger
}

// Policy contains the permissions that can be updated at runtime without
// reopening the database or moving the workspace.
type Policy struct {
	Enabled              bool
	ReadOnly             bool
	AllowCreate          bool
	AllowEdit            bool
	AllowDelete          bool
	AllowMediaGeneration bool
}

type Project struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	ProjectKey         string    `json:"project_key"`
	Dimension          string    `json:"dimension"`
	Description        string    `json:"description"`
	ProviderID         string    `json:"provider_id,omitempty"`
	Model              string    `json:"model,omitempty"`
	UseImageGeneration bool      `json:"use_image_generation"`
	UseMusicGeneration bool      `json:"use_music_generation"`
	Status             string    `json:"status"`
	CurrentRevision    int64     `json:"current_revision,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateProjectRequest struct {
	Name               string `json:"name"`
	Dimension          string `json:"dimension"`
	Description        string `json:"description"`
	ProviderID         string `json:"provider_id"`
	Model              string `json:"model"`
	UseImageGeneration bool   `json:"use_image_generation"`
	UseMusicGeneration bool   `json:"use_music_generation"`
}

type UpdateProjectRequest struct {
	Name string `json:"name"`
}

type Job struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	Kind           string     `json:"kind"`
	Prompt         string     `json:"prompt"`
	Status         string     `json:"status"`
	Phase          string     `json:"phase"`
	ProviderID     string     `json:"provider_id,omitempty"`
	Model          string     `json:"model,omitempty"`
	Error          string     `json:"error,omitempty"`
	BaseRevision   int64      `json:"base_revision,omitempty"`
	ResultRevision int64      `json:"result_revision,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

type StartJobRequest struct {
	Prompt          string `json:"prompt"`
	ProviderID      string `json:"provider_id"`
	Model           string `json:"model"`
	ImageGeneration *bool  `json:"image_generation,omitempty"`
	MusicGeneration *bool  `json:"music_generation,omitempty"`
}

type Event struct {
	ID        int64          `json:"id"`
	ProjectID string         `json:"project_id"`
	JobID     string         `json:"job_id,omitempty"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type Message struct {
	ID        int64     `json:"id"`
	ProjectID string    `json:"project_id"`
	JobID     string    `json:"job_id,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Revision struct {
	ID         int64     `json:"id"`
	ProjectID  string    `json:"project_id"`
	Number     int64     `json:"number"`
	Parent     int64     `json:"parent,omitempty"`
	Source     string    `json:"source"`
	Summary    string    `json:"summary"`
	FileCount  int       `json:"file_count"`
	TotalBytes int64     `json:"total_bytes"`
	CreatedAt  time.Time `json:"created_at"`
}

type Diagnostic struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

type BuildResult struct {
	OK          bool         `json:"ok"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type JobRun struct {
	Job     Job
	Project Project
}

// Runner is implemented by the server layer to execute the AuraGo agent with
// a tightly scoped tool and skill surface.
type Runner interface {
	RunGameMakerJob(ctx context.Context, run JobRun) error
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ActivePhase string `json:"active_phase,omitempty"`
	Source      string `json:"source"`
	Commit      string `json:"commit"`
	License     string `json:"license"`
}

type Capabilities struct {
	Enabled              bool        `json:"enabled"`
	ReadOnly             bool        `json:"readonly"`
	AllowCreate          bool        `json:"allow_create"`
	AllowEdit            bool        `json:"allow_edit"`
	AllowDelete          bool        `json:"allow_delete"`
	AllowMediaGeneration bool        `json:"allow_media_generation"`
	ImageGeneration      bool        `json:"image_generation"`
	MusicGeneration      bool        `json:"music_generation"`
	CodeStudio           bool        `json:"code_studio"`
	PhaserVersion        string      `json:"phaser_version"`
	ThreeVersion         string      `json:"three_version"`
	SkillsReady          bool        `json:"skills_ready"`
	Skills               []SkillInfo `json:"skills"`
	Providers            []Provider  `json:"providers"`
	DefaultProviderID    string      `json:"default_provider_id,omitempty"`
	DefaultModel         string      `json:"default_model,omitempty"`
}

type Provider struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Model string `json:"model"`
}

type PreviewGrant struct {
	Token     string    `json:"token"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}
