package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// planBoundStarterContent returns the real plan-bound src/main.ts of one base,
// exactly as installGameTemplate writes it.
func planBoundStarterContent(t *testing.T, base string) []byte {
	t.Helper()
	plan := GamePlan{Template: base, Objective: "Collect glowing seeds", Width: 960, Height: 540, Perspective: "side"}
	files, err := gameTemplateSources(plan)
	if err != nil {
		t.Fatalf("%s template sources: %v", base, err)
	}
	content, ok := files["main.ts"]
	if !ok || len(content) == 0 {
		t.Fatalf("%s template has no main.ts", base)
	}
	return content
}

// Every 2D plan-bound starter exposes a small, deterministic, copyable anchor
// list: bounded snippets that exist exactly once in the real template text.
func TestAdaptModeProvidesBoundedTargets(t *testing.T) {
	required := map[string]string{
		"stage list":         "configureLevels level array",
		"player spawn":       "spawn constants",
		"planned asset role": "asset-role bindings",
	}
	// At least one tuning anchor: lives, enemy spawn or the stage data block.
	tuningPurposes := []string{"life and run tuning", "enemy or wave spawn", "stage layout and enemy tuning"}
	for _, base := range []string{"shooter", "platformer", "topdown", "blocks", "board"} {
		base := base
		t.Run(base, func(t *testing.T) {
			content := planBoundStarterContent(t, base)
			digest := sourceHash(string(content))
			lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
			targets := AdaptTargets(content)
			if len(targets) == 0 || len(targets) > adaptTargetLimit {
				t.Fatalf("%s anchors = %d, want 1..%d", base, len(targets), adaptTargetLimit)
			}
			seen := map[string]bool{}
			usedLines := map[int]bool{}
			for _, target := range targets {
				if target.Path != "src/main.ts" || target.SHA256 != digest {
					t.Fatalf("%s target lost path/digest: %+v", base, target)
				}
				if target.Snippet == "" || len(target.Snippet) > adaptSnippetMaxBytes {
					t.Fatalf("%s snippet is %d bytes", base, len(target.Snippet))
				}
				if target.StartLine < 1 || target.EndLine < target.StartLine || target.EndLine > len(lines) {
					t.Fatalf("%s line range %d-%d is invalid", base, target.StartLine, target.EndLine)
				}
				if lines[target.StartLine-1] != target.Snippet {
					t.Fatalf("%s snippet does not match line %d", base, target.StartLine)
				}
				if strings.Count(string(content), target.Snippet) != 1 {
					t.Fatalf("%s snippet is not a unique replaceable block: %q", base, target.Snippet)
				}
				if usedLines[target.StartLine] || seen[target.Purpose] {
					t.Fatalf("%s duplicate anchor line/purpose: %+v", base, target)
				}
				usedLines[target.StartLine], seen[target.Purpose] = true, true
			}
			for prefix, want := range required {
				found := false
				for purpose := range seen {
					if strings.HasPrefix(purpose, prefix) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s anchors lack the %s (%s): %+v", base, prefix, want, seen)
				}
			}
			tuned := false
			for purpose := range seen {
				for _, prefix := range tuningPurposes {
					if strings.HasPrefix(purpose, prefix) {
						tuned = true
					}
				}
			}
			if !tuned {
				t.Errorf("%s anchors lack a tuning anchor: %+v", base, seen)
			}
			// Deterministic: the same bytes always yield the same anchors.
			first, _ := json.Marshal(targets)
			again, _ := json.Marshal(AdaptTargets(content))
			if string(first) != string(again) {
				t.Fatalf("%s anchors are not deterministic", base)
			}
			// The bounded JSON form carries the same entries.
			if payload := AdaptTargetsJSON(content); len(payload) != len(targets) || payload[0]["sha256"] != digest {
				t.Fatalf("%s bounded JSON form = %+v", base, payload)
			}
		})
	}
	// The blank base is never adapted and has no stage list to anchor.
	minimal := planBoundStarterContent(t, "minimal")
	if targets := AdaptTargets(minimal); len(targets) > 2 {
		t.Fatalf("minimal starter anchors = %d", len(targets))
	}
	if got := AdaptTargets(nil); got != nil {
		t.Fatalf("empty content produced %d anchors", len(got))
	}
}

// newAdaptJob starts a job whose accepted plan installs the given base and runs
// mutate during building. Every fixture ends with its own sentinel error.
func newAdaptJob(t *testing.T, s *Service, base string, small bool, mutate func(ctx context.Context, run JobRun) error) Job {
	t.Helper()
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			design := `{"base":"` + base + `","objective":"Collect glowing seeds","features":["Collect seeds through distinct stages"],"stages":["Meadow","Ridge"]}`
			return s.SetDesignJSON(ctx, run.Job.ID, []byte(design))
		}
		if run.Stage != "building" {
			return nil
		}
		return mutate(ctx, run)
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Collect glowing seeds", SmallModel: small})
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	return job
}

// A full-content write of the installed entry is refused in adapt mode with the
// stable code and the allowed anchors; new files and exact replacements stay
// available. The refusal is a tool error, never a job failure by itself.
func TestAdaptModeRejectsFullWrite(t *testing.T) {
	s := newTestService(t)
	var rejection *ModeRequiresReplaceError
	job := newAdaptJob(t, s, "platformer", true, func(ctx context.Context, run JobRun) error {
		if run.Job.BuildingMode != BuildingModeAdapt {
			return errors.New("building run did not receive the adapt mode: " + run.Job.BuildingMode)
		}
		content, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		source, err := s.ReadJobSource(ctx, run.Job.ID, "src/main.ts", SourceGenerationMaxBytes)
		if err != nil {
			return err
		}
		_, err = s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", content+"\n// rewritten\n", source.SHA256)
		var rejected *ModeRequiresReplaceError
		if !errors.As(err, &rejected) {
			return errors.New("full write was not refused: " + errorText(err))
		}
		if !ModeRequiresReplaceMatches(err) || rejected.Path != "src/main.ts" || len(rejected.Targets) == 0 {
			return errors.New("refusal lost its code or anchors")
		}
		rejection = rejected
		// No repair pass and no file change: the refusal wrote nothing.
		if unchanged, readErr := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); readErr != nil || unchanged != content {
			return errors.New("refused write changed the entry source")
		}
		// A new file under src/ is still writable.
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/level_data.ts", "export const levelTitles = ['Meadow', 'Ridge'];\n"); err != nil {
			return errors.New("new source file was refused: " + errorText(err))
		}
		// Exact replacements against the supplied anchors are the intended edit.
		// A build may have injected its diagnostic prelude, so re-read the digest.
		source, err = s.ReadJobSource(ctx, run.Job.ID, "src/main.ts", SourceGenerationMaxBytes)
		if err != nil {
			return err
		}
		replace := rejected.Targets[0]
		if _, err := s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", replace.Snippet, strings.Replace(replace.Snippet, "Meadow trail", "Seed meadow", 1), source.SHA256); err != nil {
			if !strings.Contains(replace.Snippet, "Meadow trail") {
				return errors.New("replace fixture lost its platformer stage title")
			}
			return errors.New("anchor replacement was refused: " + errorText(err))
		}
		return errors.New("adapt write guard verified")
	})
	if finished := waitJob(t, s, job.ID); finished.Error != "adapt write guard verified" {
		t.Fatalf("job = %+v", finished)
	}
	if rejection == nil {
		t.Fatal("adapt write guard never ran")
	}
	if !strings.Contains(rejection.Error(), CodeModeRequiresReplace) {
		t.Fatalf("refusal message lacks the stable code: %s", rejection.Error())
	}
	for _, target := range rejection.Targets {
		if len(target.Snippet) > adaptSnippetMaxBytes {
			t.Fatalf("anchor snippet is %d bytes", len(target.Snippet))
		}
	}
	stored, err := s.GetJob(context.Background(), job.ID)
	if err != nil || stored.BuildingMode != BuildingModeAdapt || !stored.SmallModel {
		t.Fatalf("stored job = %+v, %v", stored, err)
	}
}

// The blank base is never in adapt mode, so a new minimal game may write its
// entry source as before.
func TestAdaptModeKeepsMinimalBaseWritable(t *testing.T) {
	s := newTestService(t)
	job := newAdaptJob(t, s, "minimal", true, func(ctx context.Context, run JobRun) error {
		if run.Job.BuildingMode != BuildingModeRewrite {
			return errors.New("minimal base entered adapt mode: " + run.Job.BuildingMode)
		}
		source, err := s.ReadJobSource(ctx, run.Job.ID, "src/main.ts", SourceGenerationMaxBytes)
		if err != nil {
			return err
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", source.Content+"\n// custom rule\n", source.SHA256); err != nil {
			return errors.New("minimal base refused a write: " + errorText(err))
		}
		return errors.New("minimal base stayed writable")
	})
	if finished := waitJob(t, s, job.ID); finished.Error != "minimal base stayed writable" {
		t.Fatalf("job = %+v", finished)
	}
	stored, err := s.GetJob(context.Background(), job.ID)
	if err != nil || stored.BuildingMode != BuildingModeRewrite {
		t.Fatalf("stored job = %+v, %v", stored, err)
	}
}

// The recorded small-model decision lives on the job row and in every payload
// that already carries the job.
func TestSmallModelDecisionIsStoredOnTheJob(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		return errors.New("decision recorded")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Collect seeds", SmallModel: true})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Before a plan exists the adapt decision cannot be known.
	if !stored.SmallModel || stored.BuildingMode != BuildingModeRewrite {
		t.Fatalf("stored decision = small=%v mode=%q", stored.SmallModel, stored.BuildingMode)
	}
	encoded, _ := json.Marshal(stored)
	if !strings.Contains(string(encoded), `"small_model":true`) || !strings.Contains(string(encoded), `"building_mode":"`+BuildingModeRewrite+`"`) {
		t.Fatalf("job JSON lacks the decision: %s", encoded)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "decision recorded" {
		t.Fatalf("job = %+v", finished)
	}
	events, err := s.EventsAfter(context.Background(), project.ID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	terminal := false
	for _, event := range events {
		if event.Type != "job_status" || event.Payload["status"] != "failed" {
			continue
		}
		terminal = true
		if event.Payload["small_model"] != true || event.Payload["building_mode"] != BuildingModeRewrite {
			t.Fatalf("terminal job_status lost the decision: %+v", event.Payload)
		}
	}
	if !terminal {
		t.Fatalf("no terminal job_status in %d events", len(events))
	}
	latest, ok, err := s.LatestJob(context.Background(), project.ID)
	if err != nil || !ok || !latest.SmallModel || latest.BuildingMode != BuildingModeRewrite {
		t.Fatalf("latest job = %+v, %v", latest, err)
	}

	// An ordinary job keeps the rewrite workflow and omits the small_model field.
	ordinary, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Collect seeds"})
	if err != nil {
		t.Fatal(err)
	}
	if terminalJob := waitJob(t, s, ordinary.ID); terminalJob.Error != "decision recorded" {
		t.Fatalf("ordinary job = %+v", terminalJob)
	}
	ordinaryJSON, _ := json.Marshal(storedJob(ordinary, s))
	if strings.Contains(string(ordinaryJSON), `"small_model"`) {
		t.Fatalf("ordinary job claims a small model: %s", ordinaryJSON)
	}
	if InitialBuildingMode("adopt") != "" || InitialBuildingMode("create") != BuildingModeRewrite {
		t.Fatalf("initial modes = %q/%q", InitialBuildingMode("adopt"), InitialBuildingMode("create"))
	}
}

// An adapt edit of the installed starter compiles through the real build path
// without touching src/common.ts. Browser startup, gameplay checks and
// publication are owned by the existing suites; this fixture proves the source
// edit and the compiler gate, nothing more.
func TestAdaptOutputValidates(t *testing.T) {
	s := newTestService(t)
	job := newAdaptJob(t, s, "platformer", true, func(ctx context.Context, run JobRun) error {
		if run.Job.BuildingMode != BuildingModeAdapt {
			return errors.New("building run did not receive the adapt mode: " + run.Job.BuildingMode)
		}
		commonBefore, err := s.ReadJobFile(ctx, run.Job.ID, "src/common.ts")
		if err != nil {
			return err
		}
		source, err := s.ReadJobSource(ctx, run.Job.ID, "src/main.ts", SourceGenerationMaxBytes)
		if err != nil {
			return err
		}
		var levelAnchor, tuningAnchor *EditTarget
		targets := AdaptTargets([]byte(source.Content))
		for i := range targets {
			switch {
			case strings.HasPrefix(targets[i].Purpose, "stage list"):
				levelAnchor = &targets[i]
			case strings.HasPrefix(targets[i].Purpose, "life and run tuning"):
				tuningAnchor = &targets[i]
			}
		}
		if levelAnchor == nil {
			return errors.New("starter has no level anchor to adapt")
		}
		levels := "this.configureLevels([{id:'seed-meadow',title:'Seed meadow'},{id:'glass-ridge',title:'Glass ridge'}]);"
		if _, err := s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", levelAnchor.Snippet, levels, source.SHA256); err != nil {
			return errors.New("level replacement failed: " + errorText(err))
		}
		if tuningAnchor != nil {
			source, err = s.ReadJobSource(ctx, run.Job.ID, "src/main.ts", SourceGenerationMaxBytes)
			if err != nil {
				return err
			}
			tuned := strings.Replace(tuningAnchor.Snippet, "lives=3", "lives=4", 1)
			if tuned != tuningAnchor.Snippet && strings.Contains(source.Content, tuningAnchor.Snippet) {
				if _, err := s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", tuningAnchor.Snippet, tuned, source.SHA256); err != nil {
					return errors.New("tuning replacement failed: " + errorText(err))
				}
			}
		}
		current, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		if !strings.Contains(current, "Seed meadow") || !strings.Contains(current, "extends GameScene") {
			return errors.New("adapt edit lost the starter or its new stage titles")
		}
		commonAfter, err := s.ReadJobFile(ctx, run.Job.ID, "src/common.ts")
		if err != nil {
			return err
		}
		if commonAfter != commonBefore {
			return errors.New("adapt edit changed the installed common.ts")
		}
		// The real compiler gate, unchanged: a build, not a browser validation.
		if result := s.BuildJob(ctx, run.Job.ID); !result.OK {
			return errors.New("adapt output did not compile: " + diagnosticsText(result.Diagnostics))
		}
		return errors.New("adapt output compiled against the unchanged helper")
	})
	if terminal := waitJob(t, s, job.ID); terminal.Error != "adapt output compiled against the unchanged helper" {
		t.Fatalf("job = %+v", terminal)
	}
}

func storedJob(job Job, s *Service) Job {
	stored, err := s.GetJob(context.Background(), job.ID)
	if err != nil {
		return job
	}
	return stored
}

func errorText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
