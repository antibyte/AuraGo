package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestListMessagesMarksContinuationPrompt(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	ctx := context.Background()
	for _, m := range []struct{ role, content string }{
		{"user", ContinuationPrompt},
		{"user", "Make the ball faster"},
		{"assistant", ContinuationPrompt},
	} {
		if _, err := service.appendMessage(ctx, project.ID, "", m.role, m.content); err != nil {
			t.Fatal(err)
		}
	}
	messages, err := service.ListMessages(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	var flags []bool
	for _, m := range messages {
		flags = append(flags, m.Continuation)
	}
	if len(flags) != 3 || !flags[0] || flags[1] || flags[2] {
		t.Fatalf("continuation flags = %v, want [true false false]", flags)
	}
	raw, _ := json.Marshal(messages[1])
	if string(raw) != "" && containsKey(raw, "continuation") {
		t.Fatalf("continuation must be omitted when false: %s", raw)
	}
}

func containsKey(raw []byte, key string) bool {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	_, ok := m[key]
	return ok
}

func TestRevisionSummaryKind(t *testing.T) {
	for summary, want := range map[string]string{
		ContinuationPrompt:      "continuation",
		adoptPublicationSummary: "adopt",
		"Added a second level":  "",
		"":                      "",
	} {
		if got := revisionSummaryKind(summary); got != want {
			t.Errorf("revisionSummaryKind(%q) = %q, want %q", summary, got, want)
		}
	}
}

func TestJobErrorCodeKeepsMessageAndCode(t *testing.T) {
	inner := fmt.Errorf("agent stopped without finish; draft retained")
	err := fmt.Errorf("wrapped: %w", &JobError{Code: "unfinished", Err: inner})
	if got := jobErrorCode(err); got != "unfinished" {
		t.Fatalf("code = %q", got)
	}
	if got := (&JobError{Code: "unfinished", Err: inner}).Error(); got != inner.Error() {
		t.Fatalf("message changed: %q", got)
	}
	if got := jobErrorCode(errors.New("boom")); got != "internal" {
		t.Fatalf("plain error code = %q", got)
	}
}
