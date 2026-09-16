package server

import (
	"os"
	"strings"
	"testing"
)

// Keep historical comparisons pinned unless the operator explicitly selects a
// different configured model. This never changes the provider configuration.
func gameMakerEvaluationModels() map[string]string {
	expected := func(key, fallback string) string {
		if value := strings.ToLower(strings.TrimSpace(os.Getenv(key))); value != "" {
			return value
		}
		return fallback
	}
	return map[string]string{
		"agnesai": expected("GAMEMAKER_EVAL_AGNES_MODEL", "agnes-2.5-flash"),
		"stepfun": expected("GAMEMAKER_EVAL_STEPFUN_MODEL", "step-3.7-flash"),
	}
}

func TestGameMakerEvaluationModels(t *testing.T) {
	for _, tc := range []struct {
		name, agnes, stepfun, wantAgnes, wantStepFun string
	}{
		{"historical defaults", "", "", "agnes-2.5-flash", "step-3.7-flash"},
		{"current world comparison", "agnes-3.0-flash", "", "agnes-3.0-flash", "step-3.7-flash"},
		{"explicit selections normalized", " Agnes-3.0-Flash ", " Step-3.7-Flash ", "agnes-3.0-flash", "step-3.7-flash"},
		{"blank uses pinned default", " \t ", " \n ", "agnes-2.5-flash", "step-3.7-flash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GAMEMAKER_EVAL_AGNES_MODEL", tc.agnes)
			t.Setenv("GAMEMAKER_EVAL_STEPFUN_MODEL", tc.stepfun)
			got := gameMakerEvaluationModels()
			if got["agnesai"] != tc.wantAgnes || got["stepfun"] != tc.wantStepFun {
				t.Fatalf("unexpected evaluation model selection: %v", got)
			}
		})
	}
}
