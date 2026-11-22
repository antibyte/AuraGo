package server

import (
	"context"
	"slices"
	"testing"

	"aurago/internal/tools"
)

func TestStorymakerAgentJobRecordsTheNarrationFaults(t *testing.T) {
	for name, tc := range map[string]struct {
		wav  []byte
		want []string
	}{
		"dropout": {narrateJobDropoutWAV(), []string{"dropout"}},
		"clean":   {narrateJobWAV(), nil},
	} {
		t.Run(name, func(t *testing.T) {
			env := newSMScriptEnv(t, smScriptNativeModel, &smScriptClient{turns: smScriptHappyTurns()})
			env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
				return tc.wav, "wav", nil
			}
			job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
			// A fault never changes the outcome.
			smScriptRequireReady(t, job)
			if !slices.Equal(job.Agent.NarrationFaults, tc.want) {
				t.Errorf("narration_faults = %v, want %v", job.Agent.NarrationFaults, tc.want)
			}
		})
	}
}
