package server

import (
	"testing"

	"aurago/internal/llm"
	"aurago/internal/storymaker"
)

func TestStorymakerAgentOutputTokens(t *testing.T) {
	limits := func(ctx int) llm.ModelLimits { return llm.ModelLimits{ContextWindow: ctx, MaxOutputTokens: 131072} }
	cases := []struct {
		name        string
		limits      llm.ModelLimits
		mode, phase string
		want        int
	}{
		{"free fill 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseFill, 32768},
		{"free check 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseCheck, 32768},
		{"free revise 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseRevise, 32768},
		{"free fill 256k", limits(262144), storymaker.ModeFree, storymaker.PhaseFill, 65536},
		{"free outline 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseOutline, 20480},
		{"catalog fill 200k", limits(204800), storymaker.ModeCatalog, storymaker.PhaseFill, 20480},
		{"catalog fill 256k", limits(262144), storymaker.ModeCatalog, storymaker.PhaseFill, 20480},
		{"free fill small", llm.ModelLimits{ContextWindow: 32768, MaxOutputTokens: 32768}, storymaker.ModeFree, storymaker.PhaseFill, 0},
	}
	for _, tc := range cases {
		if got := storymakerAgentOutputTokens(tc.limits, tc.mode, tc.phase); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
