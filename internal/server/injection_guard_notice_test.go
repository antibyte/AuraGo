package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/injectionguard"
	"aurago/internal/memory"
)

func TestInjectionGuardNoticeEligibility(t *testing.T) {
	for _, available := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			status := injectionguard.Status{Available: available, Enabled: enabled}
			if got := optionalInjectionGuardNoticeAvailable(status); got != (available && !enabled) {
				t.Fatalf("wrong notice eligibility for %+v", status)
			}
		}
	}
}

func TestInjectionGuardNoticeIsDurablePassiveAndReadByID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath, modelRoot := filepath.Join(t.TempDir(), "memory.db"), t.TempDir()
	stm, err := memory.NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	manager := injectionguard.NewManager(context.Background(), modelRoot, "")
	t.Cleanup(manager.Close)
	if !manager.Status().Available {
		t.Skip("classifier is unavailable on this platform")
	}
	s := &Server{ShortTermMem: stm, InjectionGuard: manager, Logger: logger}
	get := func() []memory.SystemNotification {
		t.Helper()
		w := httptest.NewRecorder()
		handleSystemNotifications(s)(w, httptest.NewRequest(http.MethodGet, "/api/system/notifications", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("notifications: %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Notifications []memory.SystemNotification `json:"notifications"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Notifications
	}
	manager.Configure(true, "quarantine")
	queueOptionalInjectionGuardNotice(s)
	if len(get()) != 0 {
		t.Fatal("enabled classifier generated a notice")
	}
	manager.Configure(false, "quarantine")
	queueOptionalInjectionGuardNotice(s)
	queueOptionalInjectionGuardNotice(s)
	notes := get()
	if len(notes) != 1 || notes[0].SourceID != horizonOptionalNoticeSourceID || notes[0].Type != "security_advisory" || notes[0].Data["code"] != "horizon_optional" {
		t.Fatalf("notice was duplicated or lost its type: %+v", notes)
	}
	status := manager.Status()
	if notes[0].Data["download_bytes"] != float64(status.SizeBytes) || notes[0].Data["runtime_bytes"] != float64(status.RuntimeSizeBytes) {
		t.Fatal("notice does not use pinned asset sizes")
	}
	// A second browser may read the same outstanding notice, but GET never consumes it.
	if again := get(); len(again) != 1 || again[0].ID != notes[0].ID {
		t.Fatal("GET consumed or duplicated the notice")
	}
	manager.Configure(true, "quarantine")
	if len(get()) != 0 {
		t.Fatal("pending optional notice is visible while the model is enabled")
	}
	unread, err := stm.GetUnreadSystemNotifications()
	if err != nil || len(unread) != 1 {
		t.Fatal("hidden notice was acknowledged without display", err)
	}
	manager.Configure(false, "quarantine")
	otherID, _, err := stm.AddSystemNotification(memory.SystemNotification{Type: "legacy", Message: "Other system notice"})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleSystemNotificationsRead(s)(w, httptest.NewRequest(http.MethodPost, "/api/system/notifications/read", strings.NewReader(`{"ids":[`+jsonInt64(notes[0].ID)+`]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("acknowledgement: %d", w.Code)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	stm, err = memory.NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	s.ShortTermMem = stm
	queueOptionalInjectionGuardNotice(s)
	if remaining := get(); len(remaining) != 1 || remaining[0].ID != otherID {
		t.Fatalf("restart/browser change replayed a read notice or consumed another: %+v", remaining)
	}
	if entries, err := os.ReadDir(modelRoot); err != nil || len(entries) != 0 {
		t.Fatal("notice created classifier files", err)
	}
	if after := manager.Status(); after.Enabled || after.Ready || after.State != "not_installed" {
		t.Fatalf("notice changed classifier state: %+v", after)
	}
	// A failed advisory write must not prevent server startup.
	_ = stm.Close()
	queueOptionalInjectionGuardNotice(s)
}
