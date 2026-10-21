package agent

import (
	"aurago/internal/dbutil"
	"aurago/internal/memory"
	"context"
	"errors"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersonalityProfileCredentialLabelsRejectOpaqueValues(t *testing.T) {
	for _, test := range []struct {
		key, value string
		wantStored bool
	}{
		{"password", "orchid-candle-seven", false},
		{"api_key", "cloud-sparrow-eleven", false},
		{"birthday", "May 12", true},
		{"theme", "dark", true},
	} {
		t.Run(test.key, func(t *testing.T) {
			stm, _ := maintenanceRegressionStores(t)
			_, applyErr := stm.ApplyPersonalityObservation(memory.PersonalityObservation{Input: memory.NewHumanInput(test.value, "profile-test"), ProfileUpdates: []memory.ProfileUpdate{{Category: "prefs", Key: test.key, Value: test.value}}})
			if applyErr != nil {
				t.Fatal(applyErr)
			}
			entries, err := stm.GetProfileEntries("prefs")
			wantCount := 0
			if test.wantStored {
				wantCount = 1
			}
			if err != nil || len(entries) != wantCount {
				t.Fatalf("stored=%d wantStored=%v err=%v", len(entries), test.wantStored, err)
			}
			if test.wantStored && (entries[0].Key != test.key || entries[0].Value != strings.ToLower(test.value)) {
				t.Fatal("ordinary personal profile entry changed")
			}
		})
	}
}

func TestTypedVectorOutagesDeferWithoutSpendingRetries(t *testing.T) {
	for _, outage := range []error{memory.ErrVectorDBDisabled, memory.ErrVectorDBNotReady, memory.ErrVectorDBClosed} {
		t.Run(outage.Error(), func(t *testing.T) {
			_, logger := maintenanceRegressionStores(t)
			path := filepath.Join(t.TempDir(), "typed.db")
			stm, err := memory.NewSQLiteMemory(path, logger)
			if err != nil {
				t.Fatal(err)
			}
			defer stm.Close()
			db, err := dbutil.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: "fact", Content: "The user prefers complete reports."}, nil); err != nil {
				t.Fatal(err)
			}
			wrapped := fmt.Errorf("embedding storage: %w", outage)
			if succeeded, failed := retryPendingMemoryWrites(t.Context(), logger, stm, &fakeVectorDB{storeErr: wrapped}); succeeded != 0 || failed != 1 {
				t.Fatalf("retry=%d/%d", succeeded, failed)
			}
			due, err := stm.GetDuePendingMemoryWrites(time.Now().Add(time.Hour), 10)
			if err != nil || len(due) != 1 || due[0].Attempts != 0 || !due[0].NextAttemptAt.After(time.Now()) {
				t.Fatalf("outage consumed retries or omitted backoff: %+v %v", due, err)
			}
			for range 2 {
				if _, err := stm.InsertMessage("typed-session", "user", "The backup target is the NAS.", false, false); err != nil {
					t.Fatal(err)
				}
			}
			if err := stm.DeleteOldMessages("typed-session", 1); err != nil {
				t.Fatal(err)
			}
			claims, err := stm.ClaimConsolidationCandidates(10, 3)
			if err != nil || len(claims) != 1 {
				t.Fatalf("claims=%+v %v", claims, err)
			}
			if err := finishConsolidationError(t.Context(), stm, []int64{claims[0].ID}, wrapped); err != nil {
				t.Fatal(err)
			}
			var status string
			var retries int
			var later bool
			if err := db.QueryRow(`SELECT consolidation_status,consolidation_retries,next_retry_at>CURRENT_TIMESTAMP FROM archived_messages WHERE id=?`, claims[0].ID).Scan(&status, &retries, &later); err != nil || status != "failed" || retries != 0 || !later {
				t.Fatalf("claim=%s retries=%d future=%v err=%v", status, retries, later, err)
			}
		})
	}
}

type cancelAnalysisVector struct {
	fakeVectorDB
	cancel context.CancelFunc
	calls  int
}

func (v *cancelAnalysisVector) DeleteDocumentIfContentMatches(string, string) (bool, error) {
	return false, nil
}

var _ memory.ContextOwnershipAwareVectorDB = (*cancelAnalysisVector)(nil)

func (v *cancelAnalysisVector) StoreDocumentOwned(string, string, memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	return memory.VectorStoreResult{}, errors.New("unexpected context-free write")
}
func (v *cancelAnalysisVector) StoreDocumentOwnedContext(ctx context.Context, _, _ string, _ memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	v.calls++
	v.cancel()
	return memory.VectorStoreResult{}, ctx.Err()
}

func TestCancelledAnalysisWriteNeverEnqueuesDurableRetry(t *testing.T) {
	for _, kind := range []string{"fact", "preference", "correction"} {
		t.Run(kind, func(t *testing.T) {
			stm, logger := maintenanceRegressionStores(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			vector := &cancelAnalysisVector{cancel: cancel}
			fact := extractedFact{Content: "The user prefers complete German explanations.", Category: "communication", Confidence: .99}
			result := memoryAnalysisResult{}
			switch kind {
			case "fact":
				result.Facts = []extractedFact{fact}
			case "preference":
				result.Preferences = []extractedFact{fact}
			case "correction":
				result.Corrections = []extractedFact{fact}
			}
			if count := applyMemoryAnalysisResultWithContext(ctx, enabledMemoryTestConfig(), logger, stm, vector, "cancel-session", result); count != 0 || vector.calls != 1 {
				t.Fatalf("write result=%d calls=%d", count, vector.calls)
			}
			if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 0 {
				t.Fatalf("cancelled payload queued: %d %v", count, err)
			}
		})
	}
}

func TestTurnExtractionPreservesFullSourceAcrossRetryAndDefersOversize(t *testing.T) {
	source := strings.Repeat("<& Complete source 生日. ", 300) + "FINAL_USER_SOURCE"
	assistant := strings.Repeat("Assistant context. ", 300) + "FINAL_ASSISTANT_SOURCE"
	valid := `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{}}`
	client := &helperRecoveryClient{responses: []openai.ChatCompletionResponse{helperRecoveryResponse(`{broken`, openai.FinishReasonLength), helperRecoveryResponse(valid, openai.FinishReasonStop)}}
	manager := &helperLLMManager{client: client, model: "test-model", contextWindow: 64000}
	if _, err := manager.AnalyzeTurn(t.Context(), source, assistant, nil, nil, nil, newHelperTurnBudget(t)); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 2 || client.requests[0].Messages[1].Content != client.requests[1].Messages[1].Content || !strings.Contains(client.requests[1].Messages[1].Content, "FINAL_USER_SOURCE") || !strings.Contains(client.requests[1].Messages[1].Content, "FINAL_ASSISTANT_SOURCE") {
		t.Fatal("turn source silently clipped on first request/retry")
	}
	oversizeClient := &helperRecoveryClient{responses: []openai.ChatCompletionResponse{helperRecoveryResponse(valid, openai.FinishReasonStop)}}
	manager = &helperLLMManager{client: oversizeClient, model: "test-model", contextWindow: 512}
	if _, err := manager.AnalyzeTurn(t.Context(), source, assistant, nil, nil, nil, newHelperTurnBudget(t)); err == nil || len(oversizeClient.requests) != 0 {
		t.Fatal("oversize source was truncated and partially analyzed")
	}
}

func TestBatchedPersonalityDeletionCancelsInFlightAndFencesAllState(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	_, logger := maintenanceRegressionStores(t)
	cfg := enabledMemoryTestConfig()
	cfg.Personality.Engine = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	synth := memory.NewEmotionSynthesizer(nil, "", 60, 100, "English", logger)
	basis, err := stm.GetPersonalitySnapshotAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ctx, release, err := stm.RegisterSessionRun(t.Context(), "batched-purge")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	complete := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-complete
		result := personalityV2AnalysisResult{Basis: &basis, Mood: memory.MoodCreative, TraitDeltas: map[string]float64{memory.TraitEmpathy: .1}, ProfileUpdates: []memory.ProfileUpdate{{Category: "tech", Key: "language", Value: "go"}}, SynthesizedEmotion: &memory.EmotionState{Description: "Feeling inspired.", PrimaryMood: memory.MoodCreative, Valence: .8, Arousal: .8, Confidence: .8}}
		applyPersonalityV2AnalysisResult("batched-purge", cfg, logger, stm, synth, nil, "human source", memory.EmotionTriggerConversation, "", 0, true, 0, 1, result, ctx)
		applyPersonalityV2AnalysisResult("batched-purge", cfg, logger, stm, synth, nil, "human source", memory.EmotionTriggerConversation, "", 0, true, 0, 1, result)
	}()
	if err := stm.PurgeChatSession("batched-purge"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("deletion did not cancel analysis")
	}
	close(complete)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("late analysis did not finish")
	}
	after, err := stm.GetPersonalitySnapshotAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if after.Traits[memory.TraitEmpathy] != basis.Traits[memory.TraitEmpathy] || synth.GetLastEmotion() != nil {
		t.Fatal("late batch changed shared personality")
	}
	entries, err := stm.GetProfileEntries("tech")
	if err != nil || len(entries) != 0 {
		t.Fatalf("late profile retained: %d %v", len(entries), err)
	}
	history, err := stm.GetEmotionHistory(24)
	if err != nil || len(history) != 0 {
		t.Fatalf("late narration retained: %d %v", len(history), err)
	}
}
