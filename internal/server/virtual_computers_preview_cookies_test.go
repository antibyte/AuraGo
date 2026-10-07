package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestVirtualComputerLegacyPreviewPreservesGuestResponseCookies(t *testing.T) {
	guestCookie := "guest_session=login; Path=/app; HttpOnly; SameSite=Lax; Priority=High"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := http.Header{"Set-Cookie": {sessionCookieName + "=overwrite; Path=/", "__Host-aurago-preview=overwrite; Secure; Path=/", "aurago_custom=overwrite; Path=/", guestCookie}}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, headers)
			if err == nil {
				defer conn.Close()
				_ = conn.WriteMessage(websocket.TextMessage, []byte("guest ready"))
			}
			return
		}
		w.Header()["Set-Cookie"] = headers["Set-Cookie"]
		_, _ = w.Write([]byte("guest ready"))
	}))
	defer upstream.Close()
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	token, _, err := s.TokenManager.Create("preview cookie test", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	server := httptest.NewServer(mux)
	defer server.Close()
	url := server.URL + "/api/virtual-computers/machines/vm-1/web/8080/app/"
	for _, socket := range []bool{false, true} {
		var response *http.Response
		if socket {
			conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(url, "http"), http.Header{"Authorization": {"Bearer " + token}})
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.Close()
			response = resp
		} else {
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			response, err = http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
		}
		cookies := response.Header.Values("Set-Cookie")
		if len(cookies) != 1 || cookies[0] != guestCookie {
			t.Fatalf("unexpected response cookies (websocket=%v): %v", socket, cookies)
		}
	}
}
