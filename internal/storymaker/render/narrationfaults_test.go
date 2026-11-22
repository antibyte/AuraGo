package render

import (
	"context"
	"io"
	"testing"

	"aurago/internal/storymaker"
)

// markPieceFaults rewrites the envelope of every seeded piece with the given faults.
func (fx *runFixture) markPieceFaults(t *testing.T, events []any, faults ...string) {
	t.Helper()
	ctx := context.Background()
	for _, e := range events {
		hash := e.(map[string]any)["ref"].(string)
		rc, _, err := fx.projects.OpenAudio(ctx, fx.projectID, "narration", hash+".env.json")
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		env, err := storymaker.ParseEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		env.Faults = faults
		if err := fx.projects.WriteEnvelope(ctx, fx.projectID, hash, env); err != nil {
			t.Fatal(err)
		}
	}
}

func countWarnings(job Job, code string) int {
	n := 0
	for _, w := range job.Warnings {
		if w.Code == code {
			n++
		}
	}
	return n
}

func TestRunWarnsOnceForEachNarrationFault(t *testing.T) {
	testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	fx.markPieceFaults(t, events, "clipped", "dropout")
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if countWarnings(job, "narration_dropout") != 1 || countWarnings(job, "narration_clipped") != 1 {
		t.Errorf("warnings = %+v", job.Warnings)
	}
}

func TestRunNarrationWithoutFaultsHasNoFaultWarning(t *testing.T) {
	testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded || len(job.Warnings) != 0 {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}
