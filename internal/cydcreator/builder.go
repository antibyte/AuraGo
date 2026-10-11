package cydcreator

import (
	"context"
	"time"
)

// Builder runs PlatformIO builds in the toolchain container. The server
// implements it (internal/server/cyd_creator_builder.go); tests use fakes.
// The service calls Build for one build at a time.
type Builder interface {
	// Status reports Docker and image state. It must be cheap (one or two
	// Engine requests) because Capabilities calls it on every poll.
	Status(ctx context.Context) ToolchainStatus
	// Pull pulls the toolchain image and blocks until it finished; progress is
	// visible through Status while it runs. Failures are *Error
	// CYD_IMAGE_PULL_FAILED or CYD_DOCKER_UNAVAILABLE.
	Pull(ctx context.Context) error
	// Build uploads req.Tree, runs cyd-build and returns the exit code, the
	// parsed result.json, the artefacts and the full log. onLog receives every
	// output line while the build runs. A cancelled or expired ctx ends the
	// build (the error is ctx.Err()); a missing image is *Error
	// CYD_IMAGE_MISSING, an unreachable Engine *Error CYD_DOCKER_UNAVAILABLE.
	Build(ctx context.Context, req BuildRequest, onLog func(line string)) (BuildOutput, error)
	// Addr2Line decodes firmware addresses against an ELF image.
	Addr2Line(ctx context.Context, req Addr2LineRequest) ([]Frame, error)
}

// ToolchainStatus is the builder state shown in Capabilities.
type ToolchainStatus struct {
	DockerAvailable bool    `json:"docker_available"`
	Image           string  `json:"image"`
	Present         bool    `json:"present"`
	Pulling         bool    `json:"pulling"`
	Progress        float64 `json:"progress"` // 0..1
	Error           string  `json:"error,omitempty"`
}

// BuildRequest is one build for the container. Tree is the complete
// PlatformIO tree from Generate (project-relative path → content).
type BuildRequest struct {
	BuildID, Env, CacheKey string
	Tree                   map[string][]byte
	Timeout                time.Duration
}

// BuildOutput is what the container produced. Files holds merged.bin,
// firmware.bin and firmware.elf when they exist, plus "fonts/<name>" for every
// entry of Result.Fonts; Log is capped at MaxBuildLogBytes.
type BuildOutput struct {
	ExitCode int
	Result   BuildResultFile
	Files    map[string][]byte
	Log      string
}

// MaxBuildLogBytes caps a stored build log (2 MiB).
const MaxBuildLogBytes = 2 << 20

// Artefact names written by cyd-build into /work/<build_id>/out/.
const (
	ArtifactMergedBin   = "merged.bin"
	ArtifactFirmwareBin = "firmware.bin"
	ArtifactFirmwareELF = "firmware.elf"
	ArtifactResultJSON  = "result.json"
	ArtifactBuildLog    = "build.log"
)

// ArtifactFontsDir holds the converted LVGL fonts of a succeeded build and the
// fonts.json they were made from (out/fonts/ in the container,
// <artifacts>/<build_id>/fonts/ on disk). They exist only inside the build
// container; the service keeps them (not served) so the ZIP export (plan 5)
// builds outside it. BuildOutput.Files keys them "fonts/<name>".
const ArtifactFontsDir = "fonts"

// cyd-build exit codes (overview "1b/1c contracts").
const (
	BuildExitOK            = 0
	BuildExitCompileError  = 1
	BuildExitToolingError  = 2
	BuildExitFlashExceeded = 3
)

// BuildResultFile mirrors result.json. Fonts lists out/fonts/ (the converted
// fonts and fonts.json; succeeded builds only).
type BuildResultFile struct {
	OK         bool                           `json:"ok"`
	Exit       int                            `json:"exit"`
	Env        string                         `json:"env"`
	Chip       string                         `json:"chip"`
	DurationMS int64                          `json:"duration_ms"`
	Warnings   int                            `json:"warnings"`
	Flash      BuildResultUsage               `json:"flash"`
	RAM        BuildResultUsage               `json:"ram"`
	Files      map[string]BuildResultArtifact `json:"files"`
	Fonts      map[string]BuildResultArtifact `json:"fonts,omitempty"`
}

// BuildResultUsage is a used/total pair in bytes.
type BuildResultUsage struct {
	Used  int64 `json:"used"`
	Total int64 `json:"total"`
}

// BuildResultArtifact is the size and SHA-256 cyd-build reports for one file.
type BuildResultArtifact struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Addr2LineRequest decodes Addresses ("0x400d1234") against ELF for Chip.
type Addr2LineRequest struct {
	BuildID, Chip string
	ELF           []byte
	Addresses     []string
}

// Frame is one decoded address.
type Frame struct {
	Address  string `json:"address"`
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// Build states and origins (overview "Build" JSON).
const (
	BuildQueued      = "queued"
	BuildRunning     = "running"
	BuildSucceeded   = "succeeded"
	BuildFailed      = "failed"
	BuildCancelled   = "cancelled"
	BuildInterrupted = "interrupted"

	BuildOriginUser  = "user"
	BuildOriginAgent = "agent"
)

// Build is one build record.
type Build struct {
	ID             string         `json:"id"`
	ProjectID      string         `json:"project_id"`
	JobID          string         `json:"job_id"`
	Origin         string         `json:"origin"`
	Status         string         `json:"status"`
	Board          string         `json:"board"`
	ProjectVersion int64          `json:"project_version"`
	ErrorCode      string         `json:"error_code"`
	Diagnostics    []Diagnostic   `json:"diagnostics"`
	Usage          BuildUsage     `json:"usage"`
	Artifacts      []ArtifactInfo `json:"artifacts"`
	CreatedAt      string         `json:"created_at"`
	StartedAt      string         `json:"started_at"`
	FinishedAt     string         `json:"finished_at"`
}

// BuildUsage is the flash/RAM usage reported by result.json.
type BuildUsage struct {
	FlashUsed  int64 `json:"flash_used"`
	FlashTotal int64 `json:"flash_total"`
	RAMUsed    int64 `json:"ram_used"`
	RAMTotal   int64 `json:"ram_total"`
	DurationMS int64 `json:"duration_ms"`
	Warnings   int   `json:"warnings"`
}

// ArtifactInfo describes one downloadable artefact.
type ArtifactInfo struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// BuildOptions select who builds what. Origin is OriginUser (default) or
// JobOrigin(id) (an agent build); Dir is the tree to build ("" = project
// directory, plan 3 passes the job's staging copy).
type BuildOptions struct {
	Origin string
	Dir    string
}
