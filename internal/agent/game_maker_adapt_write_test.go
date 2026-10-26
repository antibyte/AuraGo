package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

// A refused adapt-mode write reaches the model as a readable answer with the
// stable code and the allowed anchors, not as an opaque failure.
func TestGameMakerAdaptWriteAnswerListsTargets(t *testing.T) {
	root := t.TempDir()
	s, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(s)
	defer gamemaker.SetDefaultService(previous)
	s.SetSkillStatus(nil, true)
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Adapt", Dimension: "2d", Description: "Adapt the starter"})
	if err != nil {
		t.Fatal(err)
	}
	var answer string
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"platformer","objective":"Collect glowing seeds","features":["Collect seeds through distinct stages"],"stages":["Meadow","Ridge"]}`))
		}
		if run.Stage != "building" {
			return nil
		}
		bound := gamemaker.WithJobContext(ctx, run.Job.ID)
		source, err := s.ReadJobSource(ctx, run.Job.ID, "src/main.ts", gamemaker.SourceGenerationMaxBytes)
		if err != nil {
			return err
		}
		out, handled := dispatchGameMaker(bound, ToolCall{Action: "game_maker_file", Params: map[string]any{
			"operation": "write", "path": "src/main.ts", "content": source.Content + "\n// rewrite\n", "expected_sha256": source.SHA256,
		}}, nil)
		if !handled {
			return errors.New("adapt write was not dispatched")
		}
		answer = out
		if !strings.Contains(out, gamemaker.CodeModeRequiresReplace) || !strings.Contains(out, `"edit_targets":[`) {
			return fmt.Errorf("adapt answer lacks the code or anchors: %s", out)
		}
		if strings.Contains(out, "// rewrite") {
			return errors.New("adapt answer echoed the refused content")
		}
		// The refused write changed nothing and the round can continue.
		if current, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); err != nil || current != source.Content {
			return errors.New("refused adapt write changed the entry source")
		}
		return errors.New("adapt answer verified")
	}))
	job, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Collect glowing seeds", SmallModel: true})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		done, err := s.GetJob(context.Background(), job.ID)
		if err == nil && done.Status == "failed" {
			if done.Error != "adapt answer verified" {
				t.Fatal(done.Error)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("adapt job did not terminate: %+v", done)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if answer == "" {
		t.Fatal("adapt write never reached dispatch")
	}
}
