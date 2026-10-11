package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Capabilities is the GET /capabilities answer (overview "JSON shapes").
// Enabled and ReadOnly are set by the HTTP layer from the configuration.
type Capabilities struct {
	Enabled         bool             `json:"enabled"`
	ReadOnly        bool             `json:"readonly"`
	TemplateVersion string           `json:"template_version"`
	Toolchain       ToolchainStatus  `json:"toolchain"`
	ActiveJob       *ActiveJob       `json:"active_job"`
	BuildQueue      int              `json:"build_queue"`
	Boards          []Board          `json:"boards"`
	Themes          []Theme          `json:"themes"`
	Widgets         []WidgetSpec     `json:"widgets"`
	Fonts           FontCatalog      `json:"fonts"`
	Symbols         []string         `json:"symbols"`
	Libraries       []Library        `json:"libraries"`
	Languages       []string         `json:"languages"`
	OTA             OTACapability    `json:"ota"`
	Limits          CapabilityLimits `json:"limits"`
}

// ActiveJob is the job that holds the global agent slot.
type ActiveJob struct {
	JobID     string `json:"job_id"`
	ProjectID string `json:"project_id"`
	Phase     string `json:"phase"`
	Status    string `json:"status"`
}

// FontCatalog lists the font families and sizes of the designer.
type FontCatalog struct {
	Families []string `json:"families"`
	Sizes    []int    `json:"sizes"`
}

// OTACapability tells the UI whether OTA is on and which URL devices use.
type OTACapability struct {
	Enabled   bool   `json:"enabled"`
	PublicURL string `json:"public_url"`
	Insecure  bool   `json:"insecure"`
}

// CapabilityLimits are the project limits the UI enforces up front.
type CapabilityLimits struct {
	MaxScreens          int   `json:"max_screens"`
	MaxWidgetsPerScreen int   `json:"max_widgets_per_screen"`
	MaxAppFiles         int   `json:"max_app_files"`
	MaxAppFileBytes     int   `json:"max_app_file_bytes"`
	AssetBudgetBytes    int64 `json:"asset_budget_bytes"`
}

// Capabilities assembles the static catalogs and the live toolchain, queue
// and job state.
func (s *Service) Capabilities(ctx context.Context) Capabilities {
	boards, err := Boards()
	if err != nil {
		s.logger.Error("CYD Creator board profiles are broken", "error", err)
	}
	themes, err := Themes()
	if err != nil {
		s.logger.Error("CYD Creator themes are broken", "error", err)
	}
	toolchain := s.Toolchain(ctx)
	limits := s.Limits()
	publicURL := strings.TrimSpace(limits.OTA.PublicURL)
	active, _ := s.activeJob(ctx) // logged; the answer stays usable without it
	caps := Capabilities{
		Enabled:         true,
		TemplateVersion: TemplateVersion,
		Toolchain:       toolchain,
		ActiveJob:       active,
		BuildQueue:      s.activeBuilds(),
		Boards:          nonNil(boards),
		Themes:          nonNil(themes),
		Widgets:         WidgetCatalog(),
		Fonts:           FontCatalog{Families: append([]string(nil), FontFamilies...), Sizes: append([]int(nil), FontSizes...)},
		Symbols:         append([]string(nil), Symbols...),
		Libraries:       Libraries(),
		Languages:       append([]string(nil), Languages...),
		OTA: OTACapability{Enabled: limits.OTA.Enabled, PublicURL: publicURL,
			Insecure: strings.HasPrefix(strings.ToLower(publicURL), "http://")},
		Limits: CapabilityLimits{MaxScreens: MaxScreens, MaxWidgetsPerScreen: MaxWidgetsPerScreen, MaxAppFiles: MaxAppFiles,
			MaxAppFileBytes: MaxAppFileBytes, AssetBudgetBytes: AssetBudget4MB},
	}
	return caps
}

// Toolchain returns the builder state. Pulling is already true while a pull
// that PullToolchain started is still being set up.
func (s *Service) Toolchain(ctx context.Context) ToolchainStatus {
	st := ToolchainStatus{}
	if s.builder != nil {
		st = s.builder.Status(ctx)
	}
	if s.pulling.Load() {
		st.Pulling = true
	}
	return st
}

// activeJob returns the newest job that is queued, running or waiting for
// confirmation (plan 3 writes cc_jobs), or nil. A failed query is logged and
// returned, never read as "no job": StartJob treats it as a busy slot, while
// Capabilities shows no active job.
func (s *Service) activeJob(ctx context.Context) (*ActiveJob, error) {
	var j ActiveJob
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, phase, status FROM cc_jobs
		WHERE status IN ('queued','running','awaiting_confirmation') ORDER BY created_at DESC LIMIT 1`).
		Scan(&j.JobID, &j.ProjectID, &j.Phase, &j.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		s.logger.Warn("CYD Creator active job could not be read", "error", err)
		return nil, fmt.Errorf("read the active cyd creator job: %w", err)
	}
	return &j, nil
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
