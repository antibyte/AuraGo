package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	openai "github.com/sashabaranov/go-openai"
)

func TestGameMakerPreviousRequestsCompactsWithoutChangingSequence(t *testing.T) {
	original := "Keep the blue forest, the winding river, and every existing animal. " + strings.Repeat("Preserve the established world details. ", 20)
	correction := "Change the forest from blue to red; keep the river and all other requirements. " + strings.Repeat("Do not remove established world details. ", 20)
	current := "Add a wind trail around the river. " + strings.Repeat("Keep all previous requirements. ", 20)
	additive := "Add one new enemy to each forest path. " + strings.Repeat("Keep the existing enemies. ", 20)

	requests := []string{original, correction, original, current, additive, additive, correction}
	encoded := gameMakerPreviousRequests(requests)
	compact, ok := encoded.(gameMakerPreviousRequestContext)
	if !ok {
		t.Fatalf("expected compact encoding for repeated long prompts, got %T", encoded)
	}
	got, ok := expandGameMakerPreviousRequests(encoded)
	if !ok || !reflect.DeepEqual(got, requests) {
		t.Fatalf("compacted history changed chronology or multiplicity: got %#v, want %#v", got, requests)
	}
	if len(compact.Sequence) != len(requests) {
		t.Fatalf("sequence has %d entries for %d requests", len(compact.Sequence), len(requests))
	}
	originalJSON, _ := json.Marshal(requests)
	compactJSON, _ := json.Marshal(compact)
	if len(compactJSON) >= len(originalJSON) {
		t.Fatalf("compact form is not smaller: %d >= %d", len(compactJSON), len(originalJSON))
	}
}

func TestGameMakerPreviousRequestsKeepsArrayWhenCompactionDoesNotSaveBytes(t *testing.T) {
	longA := strings.Repeat("First distinct correction. ", 8)
	longB := strings.Repeat("Second distinct correction. ", 8)
	cases := [][]string{
		{longA},
		{longA, longB},
		{"retry", "retry", "retry", "retry", "retry"},
	}
	for _, requests := range cases {
		encoded := gameMakerPreviousRequests(requests)
		legacy, ok := encoded.([]string)
		if !ok || !reflect.DeepEqual(legacy, requests) {
			t.Errorf("expected unchanged legacy array for %#v, got %#v", requests, encoded)
		}
	}
}

func TestGameMakerPreviousRequestsStaySeparateAcrossProviderSwitch(t *testing.T) {
	root := t.TempDir()
	svc, err := gamemaker.NewService(gamemaker.Options{
		DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"),
		Enabled: true, AllowCreate: true, AllowEdit: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.SetSkillStatus(nil, true)

	description := "Build a blue forest adventure. " + strings.Repeat("Keep every original gameplay requirement. ", 20)
	project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{
		Name: "Request context", Description: description, Dimension: "2d",
	})
	if err != nil {
		t.Fatal(err)
	}
	verificationErr := errors.New("provider-switched continuation verified")
	svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		stored := []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: description},
			{Role: openai.ChatMessageRoleAssistant, Content: "Earlier work", ReasoningContent: "private reasoning from the old provider"},
			{Role: openai.ChatMessageRoleUser, Content: "Earlier correction"},
			{Role: openai.ChatMessageRoleAssistant, Content: "More earlier work"},
		}
		data, _ := json.Marshal(stored)
		if err := svc.SaveAgentConversation(ctx, run.Job.ID, "old-provider", "old-model", data); err != nil {
			return err
		}

		cfg := &config.Config{}
		cfg.LLM.Provider, cfg.LLM.Model = "new-provider", "new-model"
		runner := &gameMakerAgentRunner{service: svc}
		view, _, err := runner.gameConversation(ctx, cfg, run, true)
		if err != nil {
			return err
		}
		foundHistoricalReasoning := false
		for _, message := range view {
			if message.Role == openai.ChatMessageRoleUser {
				return fmt.Errorf("restored request history unexpectedly contains a user message")
			}
			if strings.Contains(message.Content, "private reasoning from the old provider") && message.ReasoningContent == "" {
				foundHistoricalReasoning = true
			}
		}
		if !foundHistoricalReasoning {
			return fmt.Errorf("provider change did not retain reasoning as historical text")
		}

		current := "Add a wind trail. " + strings.Repeat("Keep existing gameplay. ", 20)
		correction := "Change the forest from blue to red. " + strings.Repeat("Keep all other requirements. ", 20)
		requests := []string{description, correction, description, current, current}
		encoded := gameMakerPreviousRequests(requests)
		got, ok := expandGameMakerPreviousRequests(encoded)
		if !ok || !reflect.DeepEqual(got, requests) {
			return fmt.Errorf("provider-switched request context changed prompts: got %#v, want %#v", got, requests)
		}
		return verificationErr
	}))

	job, err := svc.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Add a wind trail"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		done, err := svc.GetJob(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if done.Status == "failed" {
			if done.Error != verificationErr.Error() {
				t.Fatal(done.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
}

func expandGameMakerPreviousRequests(value any) ([]string, bool) {
	switch encoded := value.(type) {
	case []string:
		return append([]string(nil), encoded...), true
	case gameMakerPreviousRequestContext:
		requests := make([]string, len(encoded.Sequence))
		for i, index := range encoded.Sequence {
			if index < 0 || index >= len(encoded.Texts) {
				return nil, false
			}
			requests[i] = encoded.Texts[index]
		}
		return requests, true
	default:
		return nil, false
	}
}
