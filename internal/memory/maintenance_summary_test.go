package memory

import (
	"reflect"
	"testing"
	"time"
)

func TestMaintenanceSummaryWindowAndInsertOnly(t *testing.T) {
	stm := newTestJournalDB(t)
	start := time.Date(2026, 9, 22, 4, 0, 0, 0, time.FixedZone("local", 2*60*60))
	for offset := -9; offset <= 0; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		if _, err := stm.InsertJournalEntry(JournalEntry{Date: date, EntryType: "system_event", Title: date, Content: "Journal content"}); err != nil {
			t.Fatal(err)
		}
	}
	assertDates := func(want []string) {
		t.Helper()
		got, err := stm.MissingMaintenanceSummaryDates(start)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("dates = %v, %v; want %v", got, err, want)
		}
	}
	assertDates([]string{"2026-09-21", "2026-09-15", "2026-09-16"})
	for _, date := range []string{"2026-09-21", "2026-09-15", "2026-09-16"} {
		inserted, err := stm.InsertMaintenanceDailySummary(DailySummary{Date: date, Summary: "original", Sentiment: "neutral"})
		if err != nil || !inserted {
			t.Fatalf("insert = %v, %v", inserted, err)
		}
	}
	inserted, err := stm.InsertMaintenanceDailySummary(DailySummary{Date: "2026-09-21", Summary: "replacement"})
	if err != nil || inserted {
		t.Fatalf("duplicate insert = %v, %v", inserted, err)
	}
	summary, err := stm.GetDailySummary("2026-09-21")
	if err != nil || summary.Summary != "original" {
		t.Fatalf("existing summary changed: %+v, %v", summary, err)
	}
	assertDates([]string{"2026-09-17", "2026-09-18", "2026-09-19"})
}

func TestMaintenanceSummarySkipsDaysWithoutJournal(t *testing.T) {
	stm := newTestJournalDB(t)
	start := time.Date(2026, 3, 30, 4, 0, 0, 0, time.UTC)
	if _, err := stm.InsertJournalEntry(JournalEntry{Date: "2026-03-24", EntryType: "system_event", Title: "older", Content: "only older content"}); err != nil {
		t.Fatal(err)
	}
	dates, err := stm.MissingMaintenanceSummaryDates(start)
	if err != nil || !reflect.DeepEqual(dates, []string{"2026-03-24"}) {
		t.Fatalf("dates = %v, %v", dates, err)
	}
}
