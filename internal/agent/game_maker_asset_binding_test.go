package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

func TestGameMakerDescriptionResolvesAcceptedPackOnly(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			root := t.TempDir()
			s, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			previous := gamemaker.DefaultService()
			gamemaker.SetDefaultService(s)
			defer gamemaker.SetDefaultService(previous)
			s.SetSkillStatus(nil, true)
			project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Binding", Description: "Use planned artwork", Dimension: dimension})
			if err != nil {
				t.Fatal(err)
			}
			pack, asset := "jump-and-run", "coin"
			if dimension == "3d" {
				pack, asset = gamemaker.ModelPackID, "fps-rifle"
			}
			s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				bound := gamemaker.WithJobContext(ctx, run.Job.ID)
				describe := func(packID, assetID string) string {
					out, _ := dispatchGameMaker(bound, ToolCall{Action: "game_maker_asset", Params: map[string]any{"operation": "describe_asset", "asset_id": assetID, "pack_id": packID}}, nil)
					return out
				}
				if run.Stage == "planning" {
					if out := describe("", asset); !strings.Contains(out, "requires pack_id") {
						return fmt.Errorf("missing binding has no actionable error: %s", out)
					}
					base, role := "platformer", "item"
					if dimension == "3d" {
						base, role = "fps", "weapon"
					}
					return s.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Use exact assets","features":["custom gameplay"],"assets":[{"role":%q,"pack_id":%q,"asset_id":%q}]}`, base, role, pack, asset)))
				}
				out := describe("", asset)
				if !strings.Contains(out, `"status":"ok"`) || !strings.Contains(out, `"pack_id":"`+pack+`"`) {
					return fmt.Errorf("accepted binding not resolved: %s", out)
				}
				if out := describe("missing-pack", asset); !strings.Contains(out, `"status":"error"`) {
					return fmt.Errorf("explicit invalid pack was replaced: %s", out)
				}
				if out := describe("", "not-in-the-plan"); !strings.Contains(out, "requires pack_id") {
					return fmt.Errorf("unknown asset was guessed: %s", out)
				}
				return fmt.Errorf("verified binding")
			}))
			job, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(15 * time.Second); ; {
				done, err := s.GetJob(context.Background(), job.ID)
				if err == nil && done.Status == "failed" {
					if done.Error != "verified binding" {
						t.Fatal(done.Error)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("binding check did not finish")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}
