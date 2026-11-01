package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/personabooster"
)

func TestPersonalityAppraisalBoosterCue(t *testing.T) {
	var appraisal PersonalityAppraisal
	if err := json.Unmarshal([]byte(`{"signal":"none","target":"unknown","cue":"deep_topic","confidence":0.7,"reference":"current_user_message"}`), &appraisal); err != nil {
		t.Fatal(err)
	}
	if cue, ok := appraisal.BoosterCue(); !ok || cue != personabooster.CueDeepTopic {
		t.Fatalf("cue = %q, ok = %v", cue, ok)
	}
	for _, rejected := range []PersonalityAppraisal{{Cue: "deep_topic", Confidence: 0.49}, {Cue: "none", Confidence: 1}, {Cue: "tantrum", Confidence: 1}} {
		if _, ok := rejected.BoosterCue(); ok {
			t.Errorf("accepted %+v", rejected)
		}
	}
	var missing *PersonalityAppraisal
	if _, ok := missing.BoosterCue(); ok {
		t.Error("nil appraisal produced a cue")
	}
}

func TestAnalyzeMoodV2RequestsAndKeepsAppraisalCue(t *testing.T) {
	stm := newTestAnalysisDB(t)
	mock := &mockPersonalityAnalysisClient{
		response: `{"user_sentiment":"annoyed","agent_appropriate_response_mood":"focused","relationship_delta":0,"trait_deltas":{},"appraisal":{"signal":"none","target":"unknown","cue":"criticism","confidence":0.8,"reference":"current_user_message"}}`,
	}
	analysis := &MoodAnalysisContext{CurrentUserMessage: "Das war schwach."}
	if _, _, _, _, err := stm.AnalyzeMoodV2(context.Background(), mock, "test-model", "history", "", PersonalityMeta{}, false, analysis); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mock.request.Messages[0].Content, AppraisalCueRule) {
		t.Fatal("AnalyzeMoodV2 does not request the cue")
	}
	if analysis.Appraisal == nil || analysis.Appraisal.Cue != "criticism" {
		t.Fatalf("appraisal = %+v", analysis.Appraisal)
	}
}

func TestAnalyzeMoodV2WithEmotionRequestsAndKeepsAppraisalCue(t *testing.T) {
	stm := newTestAnalysisDB(t)
	mock := &mockPersonalityAnalysisClient{
		response: `{"appraisal":{"signal":"none","target":"unknown","cue":"deep_topic","confidence":0.9,"reference":"current_user_message"},"mood_analysis":{"user_sentiment":"curious","agent_appropriate_response_mood":"focused","relationship_delta":0.02,"trait_deltas":{"curiosity":0.05},"user_profile_updates":[]},"emotion_state":{"description":"I feel calm and ready to help.","primary_mood":"focused","secondary_mood":"steady","valence":0.2,"arousal":0.3,"confidence":0.8,"cause":"the request is clear","recommended_response_style":"calm_and_precise"}}`,
	}
	_, _, _, _, emotion, _, _, _, err := stm.AnalyzeMoodV2WithEmotion(context.Background(), mock, "test-model", "history", "user statements", PersonalityMeta{}, false, EmotionInput{UserMessage: "Was ist Bewusstsein?"}, "English")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mock.request.Messages[0].Content, AppraisalCueRule) {
		t.Fatal("AnalyzeMoodV2WithEmotion does not request the cue")
	}
	if emotion == nil || emotion.Appraisal == nil || emotion.Appraisal.Cue != "deep_topic" {
		t.Fatalf("emotion appraisal = %+v", emotion)
	}
}
