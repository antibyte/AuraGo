package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"aurago/internal/retronet"

	"github.com/gorilla/websocket"
)

func TestRetroNetSizeFromQueryClampsToTheContract(t *testing.T) {
	for _, tc := range []struct {
		query      string
		cols, rows int
	}{
		{"", 80, 25},
		{"cols=132&rows=43", 132, 43},
		{"cols=10&rows=3", 20, 5},
		{"cols=1000&rows=999", 400, 200},
		{"cols=abc&rows=-4", 80, 25},
		{"cols=0&rows=0", 80, 25},
	} {
		query, err := url.ParseQuery(tc.query)
		if err != nil {
			t.Fatal(err)
		}
		if got := retroNetSizeFromQuery(query); got.Cols != tc.cols || got.Rows != tc.rows {
			t.Errorf("%q -> %+v, want %dx%d", tc.query, got, tc.cols, tc.rows)
		}
	}
}

func TestRetroNetClientMessagesDecodeOnlyKnownControls(t *testing.T) {
	resize, ok := decodeRetroNetClientMessage([]byte(`{"type":"resize","cols":1000,"rows":2}`))
	if !ok || resize.Resize == nil || resize.Resize.Cols != 400 || resize.Resize.Rows != 5 {
		t.Fatalf("resize = %+v, %v", resize, ok)
	}
	decision, ok := decodeRetroNetClientMessage([]byte(`{"type":"hostkey_decision","accept":false}`))
	if !ok || decision.HostKeyAccept == nil || *decision.HostKeyAccept {
		t.Fatalf("hostkey_decision = %+v, %v", decision, ok)
	}
	for _, ignored := range []string{
		`{"type":"hostkey_decision"}`,
		`{"type":"connect","host":"10.0.0.1","port":23}`,
		`{"type":"resize","cols":"wide"}`,
		`not json`,
	} {
		if event, ok := decodeRetroNetClientMessage([]byte(ignored)); ok {
			t.Fatalf("%s decoded to %+v", ignored, event)
		}
	}
}

// startRetroNetWSPair upgrades one browser connection on a test host and
// returns the browser side plus the server-side adapter.
func startRetroNetWSPair(t *testing.T) (*websocket.Conn, *retroNetWSClient) {
	t.Helper()
	clients := make(chan *retroNetWSClient, 1)
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := retroNetUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		clients <- newRetroNetWSClient(conn)
	}))
	t.Cleanup(host.Close)
	browser, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http"), http.Header{"Origin": []string{host.URL}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	client := <-clients
	t.Cleanup(client.closeSocket)
	return browser, client
}

func TestRetroNetWSClientAdaptsFramesBothWays(t *testing.T) {
	clients := make(chan *retroNetWSClient, 1)
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := retroNetUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		clients <- newRetroNetWSClient(conn)
	}))
	defer host.Close()
	wsURL := "ws" + strings.TrimPrefix(host.URL, "http")

	refused, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if refused != nil {
		refused.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("a handshake without Origin was not refused: %v %v", response, err)
	}

	browser, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{host.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	client := <-clients
	defer client.closeSocket()

	for _, frame := range []struct {
		kind    int
		payload string
	}{
		{websocket.BinaryMessage, "ATDT\r"},
		{websocket.TextMessage, `{"type":"resize","cols":120,"rows":40}`},
		{websocket.TextMessage, `{"type":"unknown"}`},
		{websocket.BinaryMessage, ""},
		{websocket.TextMessage, `{"type":"hostkey_decision","accept":true}`},
	} {
		if err := browser.WriteMessage(frame.kind, []byte(frame.payload)); err != nil {
			t.Fatal(err)
		}
	}
	next := func() retronet.ClientEvent {
		t.Helper()
		select {
		case event, ok := <-client.Events():
			if !ok {
				t.Fatal("events closed early")
			}
			return event
		case <-time.After(3 * time.Second):
			t.Fatal("no client event")
		}
		return retronet.ClientEvent{}
	}
	if event := next(); string(event.Data) != "ATDT\r" {
		t.Fatalf("first event = %+v, want keystrokes", event)
	}
	if event := next(); event.Resize == nil || *event.Resize != (retronet.Size{Cols: 120, Rows: 40}) {
		t.Fatalf("second event = %+v, want resize 120x40", event)
	}
	if event := next(); event.HostKeyAccept == nil || !*event.HostKeyAccept {
		t.Fatalf("third event = %+v, want hostkey accept", event)
	}

	if err := client.SendData([]byte("CONNECT")); err != nil {
		t.Fatal(err)
	}
	if err := client.SendControl(retronet.Control{Type: "result", Code: retronet.CodeBusy, Reason: retronet.ReasonLimit}); err != nil {
		t.Fatal(err)
	}
	_ = browser.SetReadDeadline(time.Now().Add(3 * time.Second))
	kind, payload, err := browser.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage || string(payload) != "CONNECT" {
		t.Fatalf("data frame = %d %q %v", kind, payload, err)
	}
	kind, payload, err = browser.ReadMessage()
	if err != nil || kind != websocket.TextMessage || string(payload) != `{"type":"result","code":"BUSY","reason":"limit"}` {
		t.Fatalf("control frame = %d %q %v", kind, payload, err)
	}

	if err := browser.WriteMessage(websocket.BinaryMessage, make([]byte, retroNetReadLimit+1)); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-client.Events():
		if ok {
			t.Fatal("an oversized frame was delivered")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("an oversized frame did not end the reader")
	}
}

// Hijacked connections keep their request context until the handler returns,
// so the closed Events channel is how Manager.Run learns the browser is gone.
func TestRetroNetWSClientClosesEventsWhenTheBrowserLeaves(t *testing.T) {
	for _, tc := range []struct {
		name  string
		leave func(*websocket.Conn)
	}{
		{"close frame", func(browser *websocket.Conn) {
			_ = browser.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
			_ = browser.Close()
		}},
		{"dropped connection", func(browser *websocket.Conn) { _ = browser.Close() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			browser, client := startRetroNetWSPair(t)
			tc.leave(browser)
			select {
			case _, ok := <-client.Events():
				if ok {
					t.Fatal("an event arrived after the browser left")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Events stayed open after the browser left")
			}
		})
	}
}

func TestRetroNetWSClientFailsWritesAfterClose(t *testing.T) {
	_, client := startRetroNetWSPair(t)
	client.closeSocket()
	client.closeSocket() // idempotent
	if err := client.SendData([]byte("late")); err == nil {
		t.Fatal("SendData succeeded on a closed socket")
	}
	if err := client.SendControl(retronet.Control{Type: "result"}); err == nil {
		t.Fatal("SendControl succeeded on a closed socket")
	}
	select {
	case _, ok := <-client.Events():
		if ok {
			t.Fatal("an event arrived after closeSocket")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("closeSocket did not close Events")
	}
}
