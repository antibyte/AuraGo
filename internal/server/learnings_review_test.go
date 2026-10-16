package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/memory"
)

func TestLearningHTTPGlobalFilterPrecedesResultLimit(t *testing.T) {
	s, h, token := learningHTTPFixture(t)
	for i := 0; i < 1002; i++ {
		scope := memory.LearningScope{}
		if i > 0 {
			scope.Project = "project"
		}
		quote := fmt.Sprintf("Decision %d", i)
		hash := sha256.Sum256([]byte(quote))
		_, err := s.ShortTermMem.ObserveLearning(t.Context(), memory.LearningDraft{Statement: quote, Kind: "decision", Confidence: .99}, scope, memory.LearningSource{Reference: "user:current", Session: "human", Kind: "user", Quote: quote, Hash: hex.EncodeToString(hash[:])}, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	w := learningHTTP(t, h, token, "GET", "/api/learnings?project=global", "")
	var response struct {
		Learnings []memory.Learning `json:"learnings"`
		Limited   bool              `json:"limited"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(response.Learnings) != 1 || response.Learnings[0].Scope.Project != "" || response.Limited {
		t.Fatalf("global decision hidden by unrelated entries: %s", w.Body.String())
	}
	w = learningHTTP(t, h, token, "GET", "/api/learnings?query=Decision+0", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Learnings) != 1 {
		t.Fatalf("older matching decision hidden by scan limit: %s", w.Body.String())
	}
}

func TestLearningRunRejectsReplacedWorkspaceRegistration(t *testing.T) {
	s, _, _ := learningHTTPFixture(t)
	run := agent.RunConfig{Config: s.Cfg}
	s.configureRunKnowledge(&run)
	if run.LearningProjectValid == nil || !run.LearningProjectValid(run.CodeProject) {
		t.Fatal("authorized project unavailable")
	}
	s.Cfg.Directories.WorkspaceDir = t.TempDir()
	if run.LearningProjectValid(run.CodeProject) {
		t.Fatal("captured project survived registration replacement")
	}
}
