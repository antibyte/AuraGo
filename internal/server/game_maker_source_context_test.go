package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

func TestGameMakerSourceContextDisclosesOmittedContent(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetSkillStatus(nil, true)
	service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return service.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Test context","features":["Read source safely"]}`))
		}
		const header = "// __AURAGO_GAME_DIAGNOSTICS__\n"
		for _, source := range []string{
			header + "// small",
			header + "//" + strings.Repeat("x", gameMakerCurrentSourceMaxBytes-len(header)-2),
			header + "//" + strings.Repeat("x", gameMakerCurrentSourceMaxBytes-len(header)-1),
			header + strings.Repeat("// unicode äöü\n", 2000),
			"// __AURAGO_GAME_DIAGNOSTICS__" + strings.Repeat("x", 25000),
		} {
			if err := service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", source); err != nil {
				return err
			}
			current, err := service.ReadJobSource(ctx, run.Job.ID, "src/main.ts", gamemaker.SourceGenerationMaxBytes)
			if err != nil {
				return err
			}
			sources, omitted := gameMakerCurrentSources(ctx, service, run.Job.ID)
			if len(source) <= gameMakerCurrentSourceMaxBytes {
				if len(sources) != 1 || len(omitted) != 0 || sources[0] != current {
					return fmt.Errorf("complete source was not supplied exactly at %d bytes", len(source))
				}
				continue
			}
			if len(sources) != 0 || len(omitted) != 1 {
				return fmt.Errorf("oversized source not disclosed: sources=%d omitted=%d", len(sources), len(omitted))
			}
			entry := omitted[0]
			if entry["bytes"] != len(source) || entry["total_lines"] != current.TotalLines || entry["sha256"] != current.SHA256 || entry["reason"] != "exceeds_context_limit" || entry["content"] != nil {
				return fmt.Errorf("omission metadata mismatch: %v", entry)
			}
			if read, ok := entry["suggested_read"].(map[string]any); ok {
				window, err := service.ReadJobFileRange(ctx, run.Job.ID, read["path"].(string), read["start_line"].(int), read["end_line"].(int))
				if err != nil || window.SHA256 != current.SHA256 || len(window.Content) > gameMakerCurrentSourceMaxBytes {
					return fmt.Errorf("suggested range cannot be read: %v", err)
				}
			} else if strings.Contains(source, "\n") {
				return errors.New("readable first lines have no suggested range")
			}
			encoded, _ := json.Marshal(entry)
			if len(encoded) > 1600 || strings.Contains(string(encoded), root) {
				return errors.New("omission leaked source or host path")
			}
		}
		return errors.New("source context verified")
	}))
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Source context", Description: "Read source safely", Dimension: "2d"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		done, err := service.GetJob(context.Background(), job.ID)
		if err == nil && done.Status == "failed" {
			if done.Error != "source context verified" {
				t.Fatal(done.Error)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("source context fixture did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestGameMakerWorkflowDefersFullValidationUntilFeaturesAreComplete(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		for _, phase := range []string{"building", "repair"} {
			profile, err := gameMakerPromptProfile(phase, dimension)
			if err != nil {
				t.Fatal(err)
			}
			text := profile.SystemPrompt()
			for _, want := range []string{"scope=startup for an early runtime check", "every accepted feature and stage before scope=full", "omitted_sources", "runtime.api_reference only when supplied"} {
				if !strings.Contains(text, want) {
					t.Errorf("%s/%s guidance lacks %q", dimension, phase, want)
				}
			}
			budget := gameMakerBudget(context.Background(), 40, gamemaker.JobRun{Stage: phase})
			if !strings.Contains(budget["guidance"].(string), "omitted_sources") {
				t.Errorf("%s budget still assumes all source is supplied", phase)
			}
		}
	}
}
