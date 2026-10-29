package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMemoryFactExpiry(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 10, 29, 15, 0, 0, 0, loc)
	for _, test := range []struct {
		raw      string
		wantKeep bool
		want     time.Time
	}{
		{raw: "", wantKeep: true},
		{raw: "not a date", wantKeep: true},
		{raw: "2026-10-29", wantKeep: true, want: time.Date(2026, 10, 29, 23, 59, 59, 0, loc)},
		{raw: " 2026-11-02 ", wantKeep: true, want: time.Date(2026, 11, 2, 23, 59, 59, 0, loc)},
		{raw: "2026-10-28", wantKeep: false},
		{raw: "2028-01-01", wantKeep: true},
	} {
		got, keep := memoryFactExpiry(test.raw, now)
		if keep != test.wantKeep || !got.Equal(test.want) {
			t.Errorf("memoryFactExpiry(%q) = %v %v, want %v %v", test.raw, got, keep, test.want, test.wantKeep)
		}
	}
}

func TestMemoryAnalysisStoresFactExpiry(t *testing.T) {
	stm, logger := groundingTestStore(t)
	vdb := &fakeVectorDB{}
	tomorrow := time.Now().AddDate(0, 0, 1)
	result := memoryAnalysisResult{
		Facts: []extractedFact{{Content: "User is on vacation until tomorrow", Category: "personal", Confidence: 0.95, Quote: "I am on vacation until tomorrow", ExpiresOn: tomorrow.Format("2006-01-02")}},
		human: "I am on vacation until tomorrow, so keep answers short.",
	}
	if stored := applyMemoryAnalysisResult(enabledMemoryTestConfig(), logger, stm, vdb, "session-1", result); stored != 1 {
		t.Fatalf("stored=%d concepts=%v", stored, vdb.storedConcepts)
	}
	got, ok, err := stm.MemoryExpiry(context.Background(), "stored-doc")
	want := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 23, 59, 59, 0, time.Local)
	if err != nil || !ok || !got.Equal(want) {
		t.Fatalf("expiry=%v ok=%v err=%v, want %v", got, ok, err, want)
	}
}

func TestMemoryAnalysisDropsFactThatAlreadyExpired(t *testing.T) {
	stm, logger := groundingTestStore(t)
	vdb := &fakeVectorDB{}
	result := memoryAnalysisResult{
		Facts: []extractedFact{{Content: "User was on vacation until yesterday", Category: "personal", Confidence: 0.95, Quote: "I was on vacation until yesterday", ExpiresOn: time.Now().AddDate(0, 0, -1).Format("2006-01-02")}},
		human: "I was on vacation until yesterday, what did I miss?",
	}
	if stored := applyMemoryAnalysisResult(enabledMemoryTestConfig(), logger, stm, vdb, "session-1", result); stored != 0 || len(vdb.storedConcepts) != 0 {
		t.Fatalf("stored=%d concepts=%v", stored, vdb.storedConcepts)
	}
}

func TestMemoryAnalysisPromptsAskForExpiry(t *testing.T) {
	for _, prompt := range []string{memoryAnalysisPrompt, helperTurnBatchPrompt} {
		if !strings.Contains(prompt, "expires_on") {
			t.Fatalf("prompt does not ask for expires_on:\n%s", prompt)
		}
	}
}
