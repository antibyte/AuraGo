package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/memory"
)

// memoryPromptDate is the date line the extractors resolve "until Friday" against.
// Day granularity keeps the helper response cache valid for the whole day.
func memoryPromptDate(now time.Time) string {
	return now.Format("2006-01-02 (Monday)")
}

// memoryFactExpiry turns the analyzer's expires_on day into the end of that
// local day. keep is false when the stated day is already over: the item is
// stale before it is stored. Unparseable or implausibly distant days are
// ignored, so the item is kept without an expiry.
func memoryFactExpiry(expiresOn string, now time.Time) (expiry time.Time, keep bool) {
	expiresOn = strings.TrimSpace(expiresOn)
	if expiresOn == "" {
		return time.Time{}, true
	}
	day, err := time.ParseInLocation("2006-01-02", expiresOn, now.Location())
	if err != nil {
		return time.Time{}, true
	}
	end := day.AddDate(0, 0, 1).Add(-time.Second)
	if end.Before(now) {
		return time.Time{}, false
	}
	if end.After(now.AddDate(1, 0, 1)) {
		return time.Time{}, true
	}
	return end, true
}

// recordMemoryExpiry also clears an older expiry when the restated fact has none.
func recordMemoryExpiry(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ids []string, expiry time.Time) {
	if stm == nil || len(ids) == 0 {
		return
	}
	if err := stm.SetMemoryExpiry(ctx, ids, expiry); err != nil && logger != nil {
		logger.Warn("[Memory Analysis] Failed to record memory expiry", "error", err)
	}
}
