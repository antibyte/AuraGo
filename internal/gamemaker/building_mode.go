package gamemaker

import (
	"context"
)

// Building modes decide how a job implements its accepted plan. They are
// recorded on the job so the Studio, the prompt profile and the tool guards all
// agree on one decision.
const (
	// BuildingModeRewrite is the ordinary workflow: the model writes the entry
	// source itself. Every existing (non-small) run keeps this mode, so its
	// prompts, tools and files stay byte-identical.
	BuildingModeRewrite = "rewrite"
	// BuildingModeAdapt edits the installed plan-bound starter through exact
	// replacements instead of rewriting src/main.ts.
	BuildingModeAdapt = "adapt"
)

// InitialBuildingMode is the mode a job starts with. The adapt decision needs the
// accepted plan, so a created job starts as rewrite and applyBuildingMode can
// still promote it once the base is known. Adoption publishes workspace edits
// without a model and keeps the empty mode.
func InitialBuildingMode(kind string) string {
	if kind == "adopt" {
		return ""
	}
	return BuildingModeRewrite
}

// buildingModeFor is the deterministic mode rule: adapt requires a new game
// created from a plan-bound starter on a small model. minimal, three and voxel
// bases keep the rewrite workflow (the blank base is never adapted).
func buildingModeFor(job Job, plan *GamePlan) string {
	if job.Kind == "adopt" {
		return ""
	}
	if job.Kind != "create" || job.BaseRevision != 0 || !job.SmallModel {
		return BuildingModeRewrite
	}
	if plan == nil {
		return BuildingModeRewrite
	}
	switch plan.Template {
	case "", "minimal", "three", "voxel":
		return BuildingModeRewrite
	}
	return BuildingModeAdapt
}

// applyBuildingMode records the decided mode on the job and on every later
// JobRun of the same run. Only a changed value reaches the database.
func (s *Service) applyBuildingMode(ctx context.Context, job *Job, plan *GamePlan) {
	if s == nil || job == nil {
		return
	}
	mode := buildingModeFor(*job, plan)
	if mode == job.BuildingMode {
		return
	}
	job.BuildingMode = mode
	_, _ = s.db.ExecContext(ctx, `UPDATE gm_jobs SET building_mode=? WHERE id=?`, mode, job.ID)
}

// jobBuildingMode reads the recorded mode of a job. A job row without a value
// keeps the ordinary rewrite workflow.
func (s *Service) jobBuildingMode(ctx context.Context, jobID string) string {
	var mode string
	if err := s.db.QueryRowContext(ctx, `SELECT building_mode FROM gm_jobs WHERE id=?`, jobID).Scan(&mode); err != nil {
		return ""
	}
	return mode
}
