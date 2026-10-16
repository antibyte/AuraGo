package decisions

import (
	"context"
	"fmt"
)

// Readiness requires a complete score response, never a confidence threshold.
func verifySetupScore(ctx context.Context, session runtimeSession, metadata runtimeMetadata, device runtimeDevice, deployment string, cpuAllowed bool) error {
	question := Question{ID: "light", Type: "noul", Instructions: "Is the light on?"}
	options := optionsForQuestion(question)
	candidates, err := candidatesForOptions(options)
	if err != nil {
		return NewError(CodeInvalidRequest, err)
	}
	prompt, err := buildPrompt("The light is on.", question, options, metadata.ImageMarker, 0)
	if err != nil {
		return NewError(CodeInvalidRequest, err)
	}
	score, err := session.score(ctx, runtimeScoreRequest{Prompt: prompt, Candidates: candidates, MaxInputTokens: maxEvaluationTokens})
	if ctx.Err() != nil {
		return NewError(CodeCancelled, ctx.Err())
	}
	if err != nil {
		return NewError(CodeInferenceFailed, err)
	}
	if err := validateRuntimeMetadata(score.Metadata, device, deployment, cpuAllowed); err != nil {
		return NewError(CodeRuntimeAttestationFailed, err)
	}
	if score.usage.PromptTokens < 0 || score.usage.ImageTokens != 0 || score.usage.ContextTokens != score.usage.PromptTokens || score.usage.ContextTokens > maxEvaluationTokens {
		return NewError(CodeInvalidRuntimeResponse, fmt.Errorf("invalid setup probe token accounting"))
	}
	if _, err := answerFromScores(question, options, candidates, score.Scores); err != nil {
		return NewError(CodeInvalidRuntimeResponse, err)
	}
	return nil
}
