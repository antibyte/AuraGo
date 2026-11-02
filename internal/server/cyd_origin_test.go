package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

func TestCYDWebSocketOriginAndBearer(t *testing.T) {
	s, token := testCYDServer(t)
	ts := httptest.NewServer(handleCYDWebSocket(s))
	defer ts.Close()
	for _, tc := range []struct {
		name, origin  string
		authenticated bool
		status        int
	}{
		{"native", "", true, 101}, {"same origin", ts.URL, true, 101},
		{"foreign", "https://foreign.example", true, 403}, {"missing bearer", "", false, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{}
			if tc.origin != "" {
				headers.Set("Origin", tc.origin)
			}
			if tc.authenticated {
				headers.Set("Authorization", "Bearer "+token)
			}
			conn, response, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):], headers)
			if conn != nil {
				defer conn.Close()
			}
			if response == nil || response.StatusCode != tc.status {
				t.Fatalf("response=%v error=%v", response, err)
			}
			if tc.status == 101 && err != nil {
				t.Fatal(err)
			}
		})
	}
}
