package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestSystemNotificationsReadIsIDSpecific(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "short-term.db"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	firstID, _, err := stm.AddSystemNotification(memory.SystemNotification{Type: "morning_briefing", Message: "first", SourceID: "run:first"})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	secondID, _, err := stm.AddSystemNotification(memory.SystemNotification{Type: "legacy", Message: "second"})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	s := &Server{ShortTermMem: stm, Logger: slog.Default()}

	readReq := httptest.NewRequest(http.MethodPost, "/api/system/notifications/read", strings.NewReader(`{"ids":[`+jsonInt64(firstID)+`]}`))
	readRec := httptest.NewRecorder()
	handleSystemNotificationsRead(s).ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("read response = %d %s", readRec.Code, readRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	handleSystemNotifications(s).ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/system/notifications", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get response = %d %s", getRec.Code, getRec.Body.String())
	}
	var response struct {
		Notifications []memory.SystemNotification `json:"notifications"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Notifications) != 1 || response.Notifications[0].ID != secondID {
		t.Fatalf("remaining notifications = %#v", response.Notifications)
	}
}

func jsonInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
