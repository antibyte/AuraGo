package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
	"aurago/internal/memory"
)

func helperYesNo(p float64) decisions.Answer {
	return decisions.Answer{Noul: &p, Probabilities: map[string]float64{"yes": p, "no": 1 - p}}
}

func TestHelperDecisionsConsumersPreserveTaskDeadline(t *testing.T) {
	for _, duration := range []time.Duration{2 * time.Second, 100 * time.Millisecond} {
		for _, mode := range []string{"rag", "turn"} {
			t.Run(mode+"/"+duration.String(), func(t *testing.T) {
				var taskDeadline time.Time
				f := &fakeConsumers{available: true, evaluate: func(ctx context.Context, _ decisions.Request) (decisions.Response, error) {
					if deadline, ok := ctx.Deadline(); !ok || !deadline.Equal(taskDeadline) {
						t.Fatal("consumer replaced the task deadline; the qualified snapshot must own its scoring timeout")
					}
					if mode == "rag" {
						response := rerankScoreResponse(9, 2, 5)
						response.Answers["sufficient"] = helperYesNo(.95)
						return response, nil
					}
					response := appraisalResponse("praise", .95, "agent", .95)
					response.Answers["cue"] = appraisalAnswer(helperCueOptions, "praise", .95)
					return response, nil
				}}
				run := RunConfig{DecisionConsumers: f}
				if mode == "turn" {
					run = newAppraisalFixture(t, "Danke", f).run
				}
				ctx, cancel := context.WithTimeout(t.Context(), duration)
				defer cancel()
				taskDeadline, _ = ctx.Deadline()
				if mode == "rag" {
					if _, used := tryHelperRAG(ctx, run, nil, rerankFixture(), "NAS backup"); !used {
						t.Fatal("RAG result was discarded")
					}
				} else if plan := prepareHelperTurn(ctx, run, "Danke", true, nil, false); plan.Appraisal == nil {
					t.Fatal("turn result was discarded")
				}
			})
		}
	}
}

func TestHelperDecisionsMaskIgnoresUnrequestedLearningsBeforeDecode(t *testing.T) {
	for _, value := range []string{`"invalid"`, `{"invalid":true}`} {
		raw := `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[],"learnings":` + value + `},"activity_digest":{"intent":"check"}}`
		client := &mockChatClient{response: raw}
		manager := &helperLLMManager{client: client, model: "helper-model"}
		plan := helperTurnPlan{Selective: true, Memory: true, Activity: true}
		got, err := manager.analyzeTurn(t.Context(), "Check the NAS", "Checked", nil, nil, nil, nil, false, plan)
		if err != nil || client.calls != 1 || len(got.MemoryAnalysis.Learnings) != 0 || got.ActivityDigest.Intent != "check" {
			t.Fatalf("masked analysis: %+v, calls=%d, error=%v", got, client.calls, err)
		}
		plan.Learnings = true
		if _, err := plan.parse(raw, nil); err == nil {
			t.Fatal("malformed requested learnings were accepted")
		}
	}
}

func TestHelperDecisionsRAGSufficiencyAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name          string
		score, yes    float64
		partial, want bool
	}{
		{"good", 9, .95, false, true}, {"weak", 7, .95, false, false}, {"uncertain", 9, .84, false, false}, {"missing", 9, .95, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeConsumers{available: true, evaluate: func(_ context.Context, r decisions.Request) (decisions.Response, error) {
				response := rerankScoreResponse(tc.score, 2, 5)
				if !tc.partial {
					response.Answers["sufficient"] = helperYesNo(tc.yes)
				}
				return response, nil
			}}
			out, ok := tryHelperRAG(t.Context(), RunConfig{DecisionConsumers: f}, nil, rerankFixture(), "NAS backup")
			if ok != tc.want || f.calls != 1 {
				t.Fatalf("ok=%v calls=%d", ok, f.calls)
			}
			if ok && out[0].docID != "1" {
				t.Fatal("wrong score blending")
			}
		})
	}
}

func TestHelperDecisionsRAGPreservesValidatedHelperCache(t *testing.T) {
	for _, cached := range []bool{true, false} {
		t.Run(map[bool]string{true: "valid_cache", false: "incomplete_cache"}[cached], func(t *testing.T) {
			candidates := rerankFixture()
			client := &mockChatClient{response: `{"search_query":"NAS backup","search_terms":[],"candidate_scores":[{"memory_id":"1","score":9},{"memory_id":"2","score":2},{"memory_id":"3","score":5}]}`}
			manager := &helperLLMManager{client: client, model: "helper-model"}
			if _, err := manager.AnalyzeRAG(t.Context(), "NAS backup", candidates); err != nil {
				t.Fatal(err)
			}
			if !cached {
				key := manager.helperCacheKey("rag_batch", manager.model, helperRAGPrompt("NAS backup", candidates, 1))
				manager.setCachedResponse(key, `{"search_query":"NAS backup","search_terms":[],"candidate_scores":[]}`)
			}
			f := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
				response := rerankScoreResponse(9, 2, 5)
				response.Answers["sufficient"] = helperYesNo(.95)
				return response, nil
			}}
			_, used := tryHelperRAG(t.Context(), RunConfig{DecisionConsumers: f}, manager, candidates, "NAS backup")
			if used == cached || f.calls != map[bool]int{true: 0, false: 1}[cached] {
				t.Fatalf("cached=%v used=%v decisions_calls=%d", cached, used, f.calls)
			}
			if cached {
				if _, err := manager.AnalyzeRAG(t.Context(), "NAS backup", candidates); err != nil {
					t.Fatal(err)
				}
				if stat := manager.SnapshotStats()["rag_batch"]; stat.Requests != 2 || stat.CacheHits != 1 {
					t.Fatalf("cache accounting: %+v", stat)
				}
			}
			if client.calls != 1 {
				t.Fatalf("helper calls=%d", client.calls)
			}
		})
	}
}

func TestHelperDecisionsTurnPlanRetainsActivityAndLearnings(t *testing.T) {
	response := appraisalResponse("praise", .95, "agent", .95)
	response.Answers["cue"] = appraisalAnswer(helperCueOptions, "praise", .95)
	f := &fakeConsumers{available: true, evaluate: func(_ context.Context, r decisions.Request) (decisions.Response, error) {
		if len(r.Questions) != 3 {
			t.Fatalf("unexpected questions: %d", len(r.Questions))
		}
		return response, nil
	}}
	fxt := newAppraisalFixture(t, "Danke", f)
	plan := prepareHelperTurn(t.Context(), fxt.run, "Danke", true, nil, true)
	if plan.Memory || !plan.Activity || !plan.Learnings || plan.Appraisal == nil {
		t.Fatalf("plan=%+v", plan)
	}
	prompt, err := plan.prompt(&helperTurnPersonalityInput{EmotionEnabled: true, InnerVoiceEnabled: true})
	if err != nil || !strings.Contains(prompt, `"learnings"`) || !strings.Contains(prompt, `"inner_voice"`) {
		t.Fatalf("prompt: %v", err)
	}
	raw := `{"memory_analysis":{"facts":"unrequested invalid value","learnings":[]},"activity_digest":{"intent":"thanks"},"personality_analysis":{"appraisal":"unrequested invalid value"}}`
	got, err := plan.parse(raw, &helperTurnPersonalityInput{})
	if err != nil || len(got.MemoryAnalysis.Facts) != 0 || got.PersonalityAnalysis.Appraisal.Signal != "praise" {
		t.Fatalf("masked result: %+v %v", got, err)
	}
	if got.ActivityDigest.Intent != "thanks" {
		t.Fatal("activity was lost")
	}
}

func TestHelperDecisionsProbeShapesUseProductionBuilders(t *testing.T) {
	for _, id := range consumers.HelperProfiles {
		r := HelperQualificationRequest(id)
		if !consumers.HelperRequestFits(id, r) || decisions.ValidateRequest(r) != nil {
			t.Fatalf("invalid profile %s", id)
		}
	}
	if got := len(HelperQualificationRequest(consumers.HelperTurn).Questions); got != 4 {
		t.Fatalf("turn questions=%d", got)
	}
}

func TestHelperDecisionsEmptyPlanAvoidsGeneration(t *testing.T) {
	avoided := 0
	plan := helperTurnPlan{Selective: true, recordUse: func(_, n int) { avoided += n }}
	var manager *helperLLMManager
	got, err := manager.analyzeTurn(t.Context(), "human", "reply", nil, nil, nil, nil, false, plan)
	if err != nil || avoided != 1 || got.MemoryAnalysis.Facts == nil {
		t.Fatalf("empty plan: %+v %v avoided=%d", got, err, avoided)
	}
}

func TestHelperDecisionsUnqualifiedTurnKeepsOriginalBatch(t *testing.T) {
	f := newAppraisalFixture(t, "Danke", &fakeConsumers{available: false})
	p := prepareHelperTurn(t.Context(), f.run, "Danke", true, nil, true)
	if !p.Memory || !p.Activity || !p.Learnings || p.Selective || p.Appraisal != nil {
		t.Fatalf("unqualified path changed: %+v", p)
	}
}

func TestHelperDecisionsMaskDropsUnrequestedNarration(t *testing.T) {
	p := helperTurnPlan{Selective: true, Activity: true}
	raw := `{"activity_digest":{"intent":"check"},"personality_analysis":{"mood_analysis":{"agent_appropriate_response_mood":"focused"},"emotion_state":"unrequested invalid value","inner_voice":false}}`
	got, err := p.parse(raw, &helperTurnPersonalityInput{})
	if err != nil || got.PersonalityAnalysis.MoodAnalysis.AgentMood != "focused" || got.PersonalityAnalysis.InnerVoice != nil {
		t.Fatalf("masked narration: %+v %v", got, err)
	}
}

func TestHelperDecisionsAcceptedAppraisalDoesNotDuplicateObservation(t *testing.T) {
	f := newAppraisalFixture(t, "Ich finde dich wirklich klasse heute", nil)
	f.run.helperAppraisal = &memory.PersonalityAppraisal{Reference: "current_user_message", Signal: "praise", Target: "agent", Cue: "praise", Confidence: .95}
	before := f.affinity(t)
	applyAcceptedHelperAppraisal(t.Context(), f.run, f.basis, memory.DefaultPersonalityMeta(), rerankLogger())
	first := f.affinity(t)
	if first <= before {
		t.Fatal("accepted appraisal was not applied")
	}
	applyAcceptedHelperAppraisal(t.Context(), f.run, f.basis, memory.DefaultPersonalityMeta(), rerankLogger())
	if first != f.affinity(t) {
		t.Fatal("replayed appraisal changed relationship twice")
	}
	if _, err := json.Marshal(f.run.helperAppraisal); err != nil {
		t.Fatal(err)
	}
}

func TestHelperDecisionsAcceptedAppraisalSurvivesBlockedTextGeneration(t *testing.T) {
	f := newAppraisalFixture(t, "Ich finde dich wirklich klasse heute", nil)
	f.run.Config.Budget.Enabled = true
	f.run.SideEffects = NewAsyncTaskGroup(t.Context())
	defer f.run.SideEffects.Shutdown(time.Second)
	f.run.helperAppraisal = &memory.PersonalityAppraisal{Reference: "current_user_message", Signal: "praise", Target: "agent", Cue: "praise", Confidence: .95}
	before := f.affinity(t)
	launchAsyncPersonalityV2Analysis("default", f.run.Config, rerankLogger(), nil, f.run, f.stm, nil, nil, "", "", "", 0, "", "", memory.DefaultPersonalityMeta(), false, 0, 0, 0, false, false, f.basis)
	done := make(chan struct{})
	go func() { f.run.SideEffects.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("fallback did not finish")
	}
	if f.affinity(t) <= before {
		t.Fatal("text budget failure discarded accepted appraisal")
	}
}

func TestHelperDecisionsMemoryGatePreservesHumanData(t *testing.T) {
	for _, message := range []string{"Mein NAS nutzt ZFS.", "Ich bevorzuge Go.", "Nein, ich nutze Podman statt Docker.", "Erinnere mich morgen an den Backup-Test."} {
		t.Run(message, func(t *testing.T) {
			embedder := &fakeSalienceEmbedder{messages: map[string][]float32{message: {1, 0}}}
			resetSalienceCacheForTest(t)
			t.Cleanup(func() { resetSalienceCacheForTest(t) })
			if err := warmSaliencePrototypes(t.Context(), embedder); err != nil {
				t.Fatal(err)
			}
			f := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
				t.Fatal("salient human data must not be suppressed")
				return decisions.Response{}, nil
			}}
			fixture := newAppraisalFixture(t, message, f)
			plan := prepareHelperTurn(t.Context(), fixture.run, message, false, embedder, false)
			if !plan.Memory {
				t.Fatal("human data extraction was suppressed")
			}
		})
	}
}
