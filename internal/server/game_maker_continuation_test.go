package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	openai "github.com/sashabaranov/go-openai"
)

func TestGameMakerRequestViewBoundsToolFreeProse(t *testing.T) {
	var history []openai.ChatCompletionMessage
	for i := range 100 {
		history = append(history, openai.ChatCompletionMessage{Role: "assistant", Content: fmt.Sprint(i)})
	}
	view := gameMakerRequestView(history)
	if len(view) != 8 || view[0].Content != "92" || view[7].Content != "99" || len(history) != 100 {
		t.Fatalf("unbounded or destructive request view: %+v", view)
	}
}

func TestGameMakerCompactHistoryKeepsDurableToolRounds(t *testing.T) {
	root := t.TempDir()
	svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.SetSkillStatus(nil, true)
	project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "History", Description: "Keep the forest", Dimension: "2d"})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		var original []openai.ChatCompletionMessage
		for i := 0; i < 9; i++ {
			id := fmt.Sprint(i)
			original = append(original, openai.ChatCompletionMessage{Role: "user", Content: "old phase " + id}, openai.ChatCompletionMessage{Role: "assistant", ReasoningContent: "private " + id, ToolCalls: []openai.ToolCall{{ID: id, Type: "function", Function: openai.FunctionCall{Name: "game_maker_file", Arguments: `{"operation":"read"}`}}}}, openai.ChatCompletionMessage{Role: "tool", ToolCallID: id, Content: "old source " + id})
		}
		data, _ := json.Marshal(original)
		if err := svc.SaveAgentConversation(ctx, run.Job.ID, "stepfun", "step-5-preview", data); err != nil {
			return err
		}
		cfg := &config.Config{}
		cfg.LLM.Provider = "stepfun"
		cfg.LLM.Model = "step-5-preview"
		runner := &gameMakerAgentRunner{service: svc}
		view, checkpoint, err := runner.gameConversation(ctx, cfg, run, true)
		if err != nil {
			return err
		}
		if len(view) != 8 || view[0].ToolCalls[0].ID != "5" {
			t.Errorf("request view must contain four complete rounds: %d", len(view))
		}
		current := openai.ChatCompletionMessage{Role: "user", Content: "current phase"}
		first := openai.ChatCompletionMessage{Role: "assistant", Content: "completed new work", ReasoningContent: "new private reasoning"}
		if err := checkpoint(append(append(view, current), first)); err != nil {
			return err
		}
		// A later fitted request has discarded an earlier phase message.
		last := openai.ChatCompletionMessage{Role: "assistant", Content: "final work"}
		if err := checkpoint([]openai.ChatCompletionMessage{current, last}); err != nil {
			return err
		}
		if err := checkpoint([]openai.ChatCompletionMessage{current, last}); err != nil {
			return err
		}
		saved, err := svc.LoadAgentConversation(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		var durable []openai.ChatCompletionMessage
		if err := json.Unmarshal(saved.Messages, &durable); err != nil {
			return err
		}
		if len(durable) != len(original)+3 || durable[1].ReasoningContent != "private 0" || durable[len(original)+1].ReasoningContent != "new private reasoning" {
			t.Errorf("archive was trimmed or duplicated: %d", len(durable))
		}
		return errors.New("verified history")
	}))
	job, err := svc.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
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
			if done.Error != "verified history" {
				t.Fatal(done.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
}
