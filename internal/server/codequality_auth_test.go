package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestDesktopChatAndLogsRejectReadAndWriteOnlyTokens(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	for name, handler := range map[string]http.HandlerFunc{
		"chat": handleDesktopChat(s), "tail": handleDesktopLogTail(s), "search": handleDesktopLogSearch(s), "download": handleDesktopLogDownload(s),
	} {
		for _, token := range []string{readToken, writeToken} {
			r := httptest.NewRequest(http.MethodPost, "/api/desktop/"+name, strings.NewReader(`{}`))
			r.Header.Set("Authorization", "Bearer "+token)
			if name != "chat" {
				r.Method = http.MethodGet
			}
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s status=%d, want forbidden", name, w.Code)
			}
		}
	}
}

func TestTokenSnapshotsRemainSafeDuringConcurrentSwap(t *testing.T) {
	s, token, _ := testDesktopPermissionServer(t)
	first := s.currentTokenManager()
	second, err := security.NewTokenManager(s.Vault, filepath.Join(t.TempDir(), "swap-tokens.bin"))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/desktop/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for i := 0; i < 200; i++ {
			s.replaceTokenManager(second)
			s.replaceTokenManager(first)
		}
	}()
	go func() {
		defer group.Done()
		for i := 0; i < 200; i++ {
			w := httptest.NewRecorder()
			requireDesktopPermission(s, w, r, desktopScopeRead)
			isDaemonAuthOK(s, r)
			go2RTCRequestIsAdmin(s, r)
			desktopRemoteTokenHasExactScope(s, token, desktopScopeRead)
		}
	}()
	group.Wait()
}

func TestSubThresholdLoginRecordsExpireAndStayBounded(t *testing.T) {
	resetLoginRecordsForStepUpTest(t)
	r := getLoginRecord("old-client")
	r.mu.Lock()
	r.count = 1
	r.lastSeen = time.Now().Add(-2 * time.Hour)
	r.mu.Unlock()
	loginMu.Lock()
	cleanupLoginRecordsLocked(time.Now())
	_, retained := loginRecords["old-client"]
	loginMu.Unlock()
	if retained {
		t.Fatal("subthreshold record never expired")
	}
	for i := 0; i < 5000; i++ {
		getLoginRecord(string(rune(i + 100)))
	}
	loginMu.Lock()
	count := len(loginRecords)
	loginMu.Unlock()
	if count > 4096 {
		t.Fatalf("login records unbounded: %d", count)
	}
}
