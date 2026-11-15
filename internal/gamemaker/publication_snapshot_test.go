package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationRequiresUnchangedBuiltFiles(t *testing.T) {
	for _, changed := range []string{"", "missing-fingerprint", "src/main.ts", "assets/art.svg", "vendor/player-ui.js", ".aurago/game-plan.json", "dist/game.js"} {
		t.Run(changed, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			job := Job{ID: "snapshot-job", ProjectID: project.ID}
			if _, err := s.db.Exec(`INSERT INTO gm_jobs(id,project_id,kind,prompt,status,phase,created_at) VALUES(?,?,'create','','building','building',CURRENT_TIMESTAMP)`, job.ID, project.ID); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(s.stagingDir, job.ID)
			if err := WriteScaffold(stage, project); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"src/main.ts", "assets/art.svg", "vendor/player-ui.js", ".aurago/game-plan.json", "dist/game.js"} {
				full := filepath.Join(stage, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("validated fixture"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			fingerprint, err := validationFingerprint(stage)
			if err != nil {
				t.Fatal(err)
			}
			check := &previewCheck{ID: "build", JobID: job.ID, GameplayReceived: true}
			s.activeJobID, s.previewCheck = job.ID, check
			result := BuildResult{OK: true, RuntimeStatus: "passed", GameplayStatus: "passed", check: check, fingerprint: fingerprint}
			if changed == "missing-fingerprint" {
				result.fingerprint = ""
			} else if changed != "" {
				if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(changed)), []byte("unvalidated replacement"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			_, err = s.publishValidated(context.Background(), stage, project, job, result)
			if changed == "" || changed == ".aurago/game-plan.json" {
				if err != nil {
					t.Fatalf("unchanged validated snapshot rejected: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "changed after build") {
					t.Fatalf("stale snapshot was not rejected: %v", err)
				}
				current, err := s.GetProject(context.Background(), project.ID)
				if err != nil || current.CurrentRevision != 0 {
					t.Fatalf("rejection changed the published revision: %+v, %v", current, err)
				}
				if _, err := os.Stat(stage); err != nil {
					t.Fatalf("rejection lost the working copy: %v", err)
				}
			}
		})
	}
}
