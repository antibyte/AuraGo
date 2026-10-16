package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/prompts"
)

// Fixed bilingual acceptance tasks. The baseline has no inserted insights; this
// measures deterministic retrieval/conflict behavior, not live model quality.
func TestLearningBilingualTaskEvaluation(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	project := "evaluation-project"
	storeAgentLearning(t, dc, "Keep config.yaml server.port 443", memory.LearningScope{Project: project, Resource: "config.yaml"}, &memory.LearningCondition{Target: "config.yaml", Field: "server.port", Value: "443"})
	storeAgentLearning(t, dc, "Keep TypeScript types across main.ts and library.ts", memory.LearningScope{Project: project}, nil)
	storeAgentLearning(t, dc, "Private project uses config.yaml server.port 80", memory.LearningScope{Project: "foreign", Resource: "config.yaml"}, nil)
	tasks := []struct {
		language, query, resource, scope string
		want                             int
		value                            int
		conflict                         bool
	}{
		{"de", "Ändere config.yaml server.port auf 80", "config.yaml", project, 1, 80, true},
		{"en", "Change config.yaml server.port to 80", "config.yaml", project, 1, 80, true},
		{"de", "Behalte config.yaml server.port 443", "config.yaml", project, 1, 443, false},
		{"en", "Keep config.yaml server.port 443", "config.yaml", project, 1, 443, false},
		{"de", "Prüfe TypeScript in main.ts und library.ts", "main.ts", project, 1, 0, false},
		{"en", "Review TypeScript in main.ts and library.ts", "main.ts", project, 1, 0, false},
		{"de", "Ändere config.yaml", "config.yaml", "other", 0, 80, false},
		{"en", "Change config.yaml", "config.yaml", "other", 0, 80, false},
		{"de", "Gartentipps", "", project, 0, 0, false},
		{"en", "Gardening advice", "", project, 0, 0, false},
	}
	totalTokens, warnings := 0, 0
	for _, task := range tasks {
		t.Run(task.language+"/"+task.query, func(t *testing.T) {
			run := s.runCfg
			run.LearningScope = memory.LearningScope{Project: task.scope, Resource: task.resource}
			run.AllowProjectLearnings = true
			content := learningPromptContext(t.Context(), run, task.query, "gpt-4o")
			if strings.Count(content, "[learning:") != task.want || strings.Contains(content, "Private project") {
				t.Fatalf("irrelevant retrieval: %s", content)
			}
			tokens := prompts.CountTokensForModel(isolateAgentPromptExternalData(content), "gpt-4o")
			if content == "" {
				tokens = 0
			}
			if tokens > 600 {
				t.Fatal("context limit", tokens)
			}
			totalTokens += tokens
			if task.value != 0 {
				dc.LearningScope = run.LearningScope
				call := ToolCall{Action: "yaml_editor", Params: map[string]any{"operation": "set", "file_path": "config.yaml", "json_path": "server.port", "set_value": task.value}}
				blocked, _ := checkLearningAction(t.Context(), call, dc)
				if (blocked != "") != task.conflict {
					t.Fatal("false or missed warning", blocked)
				}
				if blocked != "" {
					warnings++
				}
			}
		})
	}
	t.Logf("fixed DE/EN tasks=%d; baseline inserted tokens=0; insights inserted tokens=%d; expected conflict warnings=%d; false positives=0", len(tasks), totalTokens, warnings)
}
