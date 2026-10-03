package gamemaker

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
)

// StarterGenerationReady identifies untouched, single-entry new games whose
// plan-bound helpers can be supplied directly to a source-generation request.
// It is a routing decision only; generated source still needs full validation.
func (s *Service) StarterGenerationReady(ctx context.Context, runJob Job) (bool, error) {
	// ResumeFrom belongs to the server-owned run snapshot, not the job table.
	if runJob.ResumeFrom != "" || runJob.BaseRevision != 0 {
		return false, nil
	}
	jobID := runJob.ID
	_, job, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return false, err
	}
	if job.Phase != "building" || job.BaseRevision != 0 {
		return false, nil
	}
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return false, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	plan, err := s.GetPlan(ctx, jobID)
	if err != nil || plan == nil {
		return false, err
	}
	// Scene/custom mechanics and blank or voxel bases may need edits outside
	// main.ts. Keep their normal tool-enabled workflow, as well as continuations.
	if plan.Scene != nil || len(plan.Mechanics) != 0 || !slices.Contains([]string{
		"platformer", "topdown", "shooter", "blocks", "board",
		"fps", "exploration", "transport", "flight", "space",
	}, plan.Template) {
		return false, nil
	}
	files, err := gameTemplateSources(*plan)
	if err != nil {
		return false, err
	}
	ready, err := s.unchangedManagedTemplateFiles(ctx, jobID, files, func(source string) string {
		return strings.ReplaceAll(source, "\r\n", "\n")
	})
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return ready, err
}
