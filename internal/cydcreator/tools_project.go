package cydcreator

import (
	"context"
	"fmt"
	"os"
	"slices"
)

// Agent tool names (overview "Agent tools"). They exist only in CYD Creator
// job runs; the normal chat never offers them.
const (
	ToolReadProject  = "cyd_creator_read_project"
	ToolUpdateLayout = "cyd_creator_update_layout"
	ToolReadFile     = "cyd_creator_read_file"
	ToolWriteFile    = "cyd_creator_write_file"
	ToolDeleteFile   = "cyd_creator_delete_file"
	ToolAPIReference = "cyd_creator_api_reference"
	ToolBuild        = "cyd_creator_build"
	ToolSubmitPlan   = "cyd_creator_submit_plan"
	ToolFinish       = "cyd_creator_finish"
)

// ToolNames lists every agent tool in schema order.
func ToolNames() []string {
	return []string{ToolReadProject, ToolAPIReference, ToolSubmitPlan, ToolUpdateLayout, ToolReadFile, ToolWriteFile,
		ToolDeleteFile, ToolBuild, ToolFinish}
}

// PhaseTools returns the tools an agent phase may use, in schema order. The
// agent schemas and the service checks share this table.
func PhaseTools(phase string) []string {
	switch phase {
	case PhasePlanning:
		return []string{ToolReadProject, ToolAPIReference, ToolSubmitPlan}
	case PhaseLayout:
		return []string{ToolReadProject, ToolAPIReference, ToolUpdateLayout}
	case PhaseCoding, PhaseRepairing:
		return []string{ToolReadProject, ToolAPIReference, ToolUpdateLayout, ToolReadFile, ToolWriteFile, ToolDeleteFile,
			ToolBuild, ToolFinish}
	}
	return nil
}

// jobTool returns the running job when tool is allowed in its current phase.
func (s *Service) jobTool(jobID, tool string) (*jobRun, error) {
	run := s.activeRun()
	if run == nil || run.jobID != jobID || jobID == "" {
		return nil, NewError(CodeJobState, "this job is not running", map[string]any{"job_id": jobID})
	}
	if run.ctx.Err() != nil {
		// Cancelled or out of time: the working copy is about to become the
		// draft and must not change any more.
		return nil, NewError(CodeJobState, "this job is ending", map[string]any{"job_id": jobID})
	}
	phase := run.currentPhase()
	if !slices.Contains(PhaseTools(phase), tool) {
		return nil, NewError(CodeJobState, fmt.Sprintf("%s is not available in the %s phase", tool, phase),
			map[string]any{"phase": phase, "tool": tool})
	}
	return run, nil
}

// BoardInfo is the board summary the agent sees.
type BoardInfo struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Status       string   `json:"status"`
	Chip         string   `json:"chip"`
	FlashMB      int      `json:"flash_mb"`
	PSRAM        bool     `json:"psram"`
	ScreenW      int      `json:"screen_w"`
	ScreenH      int      `json:"screen_h"`
	Display      string   `json:"display"`
	Touch        string   `json:"touch"`
	Features     []string `json:"features"` // rgb_led light_sensor speaker sd_card
	AppSlotBytes int      `json:"app_slot_bytes"`
}

func boardInfo(b Board, orientation string) BoardInfo {
	w, h := b.ScreenSize(orientation)
	info := BoardInfo{ID: b.ID, Label: b.Label, Status: b.Status, Chip: b.Chip, FlashMB: b.FlashMB, PSRAM: b.PSRAM,
		ScreenW: w, ScreenH: h, Display: b.Display.Driver, Touch: b.Touch.Driver, Features: []string{}}
	if b.Extras.RGBLED != nil {
		info.Features = append(info.Features, "rgb_led")
	}
	if b.Extras.LDR != nil {
		info.Features = append(info.Features, "light_sensor")
	}
	if b.Extras.Speaker != nil {
		info.Features = append(info.Features, "speaker")
	}
	if b.Extras.SDCS != nil {
		info.Features = append(info.Features, "sd_card")
	}
	info.AppSlotBytes, _ = AppSlotSize(b.FlashMB)
	return info
}

// JobProjectView is the cyd_creator_read_project answer: the working copy.
type JobProjectView struct {
	Phase       string       `json:"phase"`
	Layout      Project      `json:"layout"`
	Issues      []Issue      `json:"issues"`
	Files       []FileInfo   `json:"files"`
	Board       BoardInfo    `json:"board"`
	UserChanges *UserChanges `json:"user_changes"`
	Plan        *JobPlan     `json:"plan"`
}

// ReadProjectForJob returns the job's working copy: layout and issues, the
// editable files, board capabilities and the user's changes since the last
// job (settings are part of the layout).
func (s *Service) ReadProjectForJob(ctx context.Context, jobID string) (JobProjectView, error) {
	run, err := s.jobTool(jobID, ToolReadProject)
	if err != nil {
		return JobProjectView{}, err
	}
	root, err := os.OpenRoot(run.stage)
	if err != nil {
		return JobProjectView{}, s.internalError("read project", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return JobProjectView{}, s.internalError("read project", err)
	}
	files, _, err := listAppFiles(root)
	if err != nil {
		return JobProjectView{}, s.internalError("read project", err)
	}
	board, err := BoardByID(p.Board)
	if err != nil {
		return JobProjectView{}, err
	}
	// run.plan is set at launch and by SubmitPlan (run.job belongs to the
	// orchestrator goroutine and is not read here).
	run.mu.Lock()
	plan := run.plan
	run.mu.Unlock()
	return JobProjectView{Phase: run.currentPhase(), Layout: p, Issues: validateLayout(p), Files: nonNil(files),
		Board: boardInfo(board, p.Orientation), UserChanges: run.userChanges, Plan: plan}, nil
}
