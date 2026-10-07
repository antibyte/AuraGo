package desktop

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"
)

func TestSynthStudioProjectCanBeConditionallySavedAndReopened(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	path := "Documents/Synth Studio/Neon.aurasynth"
	want := `{"version":1,"name":"München 🎹","tempo":120,"ppq":480,"beatsPerBar":4,"loop":{"enabled":false,"start":0,"end":1920},"tracks":[]}`

	if _, err := svc.WriteFileBytesConditional(ctx, path, []byte(want), SourceUser, CheckNoteVersion("", true)); err != nil {
		t.Fatalf("create Synth Studio project: %v", err)
	}
	got, _, err := svc.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("reopen Synth Studio project: %v", err)
	}
	if !utf8.ValidString(got) {
		t.Fatal("reopened project is not valid UTF-8")
	}
	if got != want {
		t.Fatalf("reopened content differs\n got: %s\nwant: %s", got, want)
	}

	version := NoteVersion([]byte(got))
	updated := `{"version":1,"name":"München · edited 🎹","tempo":120,"ppq":480,"beatsPerBar":4,"loop":{"enabled":false,"start":0,"end":1920},"tracks":[]}`
	if _, err := svc.WriteFileBytesConditional(ctx, path, []byte(updated), SourceUser, CheckNoteVersion(version, false)); err != nil {
		t.Fatalf("update with observed version: %v", err)
	}
	if _, err := svc.WriteFileBytesConditional(ctx, path, []byte(want), SourceUser, CheckNoteVersion(version, false)); !errors.Is(err, ErrNoteConflict) {
		t.Fatalf("stale version should reject update, got %v", err)
	}
	got, _, err = svc.ReadFile(ctx, path)
	if err != nil || got != updated {
		t.Fatalf("stale update changed saved project: content=%s err=%v", got, err)
	}
}
