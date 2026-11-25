package desktop

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/inventory"

	"github.com/gorilla/websocket"
)

var rdpTestX224Request = []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x08, 0x00, 0x03, 0x00, 0x00, 0x00}
var rdpTestX224Confirm = []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00, 0x02, 0x1f, 0x08, 0x00, 0x02, 0x00, 0x00, 0x00}

func TestRDCleanPathKnownVector(t *testing.T) {
	// Same bytes the Rust der crate produces for version 3390 + proxy_auth "ab".
	got, err := rdCleanPathPDU{Version: 3390, ProxyAuth: "ab"}.marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte{0x30, 0x0c, 0xa0, 0x04, 0x02, 0x02, 0x0d, 0x3e}) {
		t.Fatalf("unexpected prefix % x", got)
	}
	if !bytes.Equal(got, []byte{0x30, 0x0c, 0xa0, 0x04, 0x02, 0x02, 0x0d, 0x3e, 0xa3, 0x04, 0x0c, 0x02, 'a', 'b'}) {
		t.Fatalf("unexpected encoding % x", got)
	}
}

func TestRDCleanPathRoundTrip(t *testing.T) {
	for name, pdu := range map[string]rdCleanPathPDU{
		"request": {Version: 3390, Destination: "10.0.0.5:3389", ProxyAuth: "ticket", X224ConnectionPDU: rdpTestX224Request},
		"preconnection": {Version: 3390, ProxyAuth: "t", PreconnectionBlob: "blob",
			X224ConnectionPDU: rdpTestX224Request},
		"response": {Version: 3390, X224ConnectionPDU: rdpTestX224Confirm, ServerAddr: "10.0.0.5:3389",
			ServerCertChain: [][]byte{{1, 2, 3}, {4, 5}}},
		"error": {Version: 3390, Error: rdCleanPathError{ErrorCode: 103, HTTPStatusCode: 401, WSALastError: 10061, TLSAlertCode: 40}},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := pdu.marshal()
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseRDCleanPathPDU(data)
			if err != nil {
				t.Fatal(err)
			}
			if got.Version != pdu.Version || got.Destination != pdu.Destination || got.ProxyAuth != pdu.ProxyAuth ||
				got.PreconnectionBlob != pdu.PreconnectionBlob || !bytes.Equal(got.X224ConnectionPDU, pdu.X224ConnectionPDU) ||
				got.ServerAddr != pdu.ServerAddr || got.Error != pdu.Error || len(got.ServerCertChain) != len(pdu.ServerCertChain) {
				t.Fatalf("round trip mismatch: %+v vs %+v", got, pdu)
			}
			for i := range pdu.ServerCertChain {
				if !bytes.Equal(got.ServerCertChain[i], pdu.ServerCertChain[i]) {
					t.Fatalf("chain[%d] mismatch", i)
				}
			}
		})
	}
	if _, err := parseRDCleanPathPDU([]byte{0x30, 0x00}); err == nil {
		t.Fatal("expected error for PDU without version")
	}
	bad, _ := rdCleanPathPDU{Version: 1}.marshal()
	if _, err := parseRDCleanPathPDU(bad); err == nil {
		t.Fatal("expected error for wrong version")
	}
}

func TestRDCleanPathErrorPDU(t *testing.T) {
	data := buildRDCleanPathErrorPDU(2, 0, 0, 0, rdpTestX224Confirm)
	pdu, err := parseRDCleanPathPDU(data)
	if err != nil {
		t.Fatal(err)
	}
	if pdu.Error.ErrorCode != 2 || pdu.Error.HTTPStatusCode != 0 || !bytes.Equal(pdu.X224ConnectionPDU, rdpTestX224Confirm) {
		t.Fatalf("unexpected error PDU %+v", pdu)
	}
}

func TestRDPTicketStore(t *testing.T) {
	store := NewRDPTicketStore()
	now := time.Now()
	store.now = func() time.Time { return now }

	ticket, err := store.Issue("dev1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket) != 43 {
		t.Fatalf("expected 43 char base64url ticket, got %d", len(ticket))
	}
	if !store.Consume(ticket, "dev1") {
		t.Fatal("fresh ticket must be accepted")
	}
	if store.Consume(ticket, "dev1") {
		t.Fatal("ticket must be single use")
	}

	ticket, _ = store.Issue("dev1")
	if store.Consume(ticket, "dev2") {
		t.Fatal("ticket must be bound to its device")
	}
	if store.Consume(ticket, "dev1") {
		t.Fatal("a mismatched attempt must burn the ticket")
	}

	ticket, _ = store.Issue("dev1")
	now = now.Add(61 * time.Second)
	if store.Consume(ticket, "dev1") {
		t.Fatal("expired ticket must be rejected")
	}
	if store.Consume("", "dev1") || store.Consume("unknown", "dev1") {
		t.Fatal("unknown tickets must be rejected")
	}

	store.max = 3
	for i := 0; i < 3; i++ {
		if _, err := store.Issue("d"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Issue("d"); err == nil {
		t.Fatal("cap must be enforced")
	}
	now = now.Add(2 * time.Minute)
	if _, err := store.Issue("d"); err != nil {
		t.Fatalf("expired tickets must be swept: %v", err)
	}
}

func TestSplitRDPUsername(t *testing.T) {
	for _, tc := range []struct{ in, domain, user string }{
		{`CORP\alice`, "CORP", "alice"},
		{"alice@corp.local", "", "alice@corp.local"},
		{"alice", "", "alice"},
		{`\alice`, "", `\alice`},
	} {
		d, u := splitRDPUsername(tc.in)
		if d != tc.domain || u != tc.user {
			t.Errorf("%q -> %q,%q", tc.in, d, u)
		}
	}
}

func TestHandleRDPTicket(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	for _, d := range []inventory.DeviceRecord{
		{ID: "rdp1", Name: "rdp1", Type: "server", Protocol: "rdp", IPAddress: "127.0.0.1", Username: `CORP\bob`, Tags: []string{}},
		{ID: "ssh1", Name: "ssh1", Type: "server", Protocol: "ssh", IPAddress: "127.0.0.1", Port: 22, Username: "u", Tags: []string{}},
	} {
		if err := inventory.AddDevice(db, d); err != nil {
			t.Fatal(err)
		}
	}
	store := NewRDPTicketStore()
	handler := HandleRDPTicket(db, nil, testLogger, store, nil, nil)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/x?device_id=rdp1", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET must be rejected, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/x?device_id=rdp1", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected response %d %v", rec.Code, rec.Header())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["username"] != "bob" || resp["domain"] != "CORP" || resp["has_password"] != false || resp["password"] != "" {
		t.Fatalf("unexpected body %v", resp)
	}
	if !store.Consume(resp["ticket"].(string), "rdp1") {
		t.Fatal("issued ticket must be valid for the device")
	}

	for _, id := range []string{"ssh1", "missing", ""} {
		rec = httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/x?device_id="+id, nil))
		if rec.Code == http.StatusOK {
			t.Fatalf("device %q must be rejected", id)
		}
	}
}

// fakeRDPServer accepts one connection: reads the X.224 CR, answers a CC (or
// the given reply), then upgrades to TLS and echoes with an "echo:" prefix.
type fakeRDPServer struct {
	listener net.Listener
	leafDER  []byte
	gotCR    chan []byte
}

func newFakeRDPServer(t *testing.T, reply []byte) *fakeRDPServer {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fake-rdp"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeRDPServer{listener: ln, leafDER: der, gotCR: make(chan []byte, 1)}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn, reply, der, key)
		}
	}()
	return s
}

func (s *fakeRDPServer) serve(conn net.Conn, reply, der []byte, key *ecdsa.PrivateKey) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	cr, err := readTPKT(conn)
	if err != nil {
		return
	}
	select {
	case s.gotCR <- cr:
	default:
	}
	if _, err := conn.Write(reply); err != nil {
		return
	}
	if reply[11] == 0x03 {
		return
	}
	tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}})
	buf := make([]byte, 4096)
	for {
		n, err := tc.Read(buf)
		if err != nil {
			return
		}
		if string(buf[:n]) == "bye" {
			return
		}
		if _, err := tc.Write(append([]byte("echo:"), buf[:n]...)); err != nil {
			return
		}
	}
}

func rdpGatewayServer(t *testing.T, deviceIP string, port int) (*httptest.Server, *RDPTicketStore) {
	srv, store, _ := rdpGatewayServerDB(t, deviceIP, port)
	return srv, store
}

func rdpGatewayServerDB(t *testing.T, deviceIP string, port int) (*httptest.Server, *RDPTicketStore, *sql.DB) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	if err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "rdp1", Name: "rdp1", Type: "server", Protocol: "rdp", IPAddress: deviceIP, Port: port, Tags: []string{}}); err != nil {
		t.Fatal(err)
	}
	store := NewRDPTicketStore()
	srv := httptest.NewServer(HandleRDPProxy(db, testLogger, store))
	t.Cleanup(srv.Close)
	return srv, store, db
}

func dialRDPGateway(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := strings.Replace(srv.URL, "http", "ws", 1) + "?device_id=rdp1"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	return ws
}

func readRDPPDU(t *testing.T, ws *websocket.Conn) rdCleanPathPDU {
	t.Helper()
	mt, data, err := ws.ReadMessage()
	if err != nil || mt != websocket.BinaryMessage {
		t.Fatalf("read PDU: %v (type %d)", err, mt)
	}
	pdu, err := parseRDCleanPathPDU(data)
	if err != nil {
		t.Fatal(err)
	}
	return pdu
}

func TestHandleRDPProxyEndToEnd(t *testing.T) {
	fake := newFakeRDPServer(t, rdpTestX224Confirm)
	port := fake.listener.Addr().(*net.TCPAddr).Port
	srv, store := rdpGatewayServer(t, "127.0.0.1", port)
	ws := dialRDPGateway(t, srv)

	ticket, _ := store.Issue("rdp1")
	req, _ := rdCleanPathPDU{Version: 3390, Destination: "203.0.113.9:3389", ProxyAuth: ticket, X224ConnectionPDU: rdpTestX224Request}.marshal()
	if err := ws.WriteMessage(websocket.BinaryMessage, req); err != nil {
		t.Fatal(err)
	}
	resp := readRDPPDU(t, ws)
	if resp.Error.ErrorCode != 0 {
		t.Fatalf("unexpected error %+v", resp.Error)
	}
	if !bytes.Equal(resp.X224ConnectionPDU, rdpTestX224Confirm) {
		t.Fatalf("confirm mismatch % x", resp.X224ConnectionPDU)
	}
	if len(resp.ServerCertChain) != 1 || !bytes.Equal(resp.ServerCertChain[0], fake.leafDER) {
		t.Fatal("server cert chain must contain the leaf")
	}
	if resp.ServerAddr != fake.listener.Addr().String() {
		t.Fatalf("server_addr %q, want the inventory device", resp.ServerAddr)
	}
	select {
	case cr := <-fake.gotCR:
		if !bytes.Equal(cr, rdpTestX224Request) {
			t.Fatalf("server saw % x", cr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fake server never saw the connection request")
	}

	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	var got []byte
	for len(got) < len("echo:hello") {
		mt, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("relay read: %v", err)
		}
		if mt == websocket.BinaryMessage {
			got = append(got, data...)
		}
	}
	if string(got) != "echo:hello" {
		t.Fatalf("relay returned %q", got)
	}

	// The ticket was consumed: a second session with it must fail.
	ws2 := dialRDPGateway(t, srv)
	_ = ws2.WriteMessage(websocket.BinaryMessage, req)
	if pdu := readRDPPDU(t, ws2); pdu.Error.ErrorCode != 1 || pdu.Error.HTTPStatusCode != 401 {
		t.Fatalf("reused ticket must give 401, got %+v", pdu.Error)
	}
}

func TestHandleRDPProxyRejections(t *testing.T) {
	fake := newFakeRDPServer(t, rdpTestX224Confirm)
	port := fake.listener.Addr().(*net.TCPAddr).Port
	srv, store := rdpGatewayServer(t, "127.0.0.1", port)
	other, _ := store.Issue("other-device")
	good, _ := store.Issue("rdp1")

	for name, pdu := range map[string]rdCleanPathPDU{
		"wrong ticket":     {Version: 3390, ProxyAuth: "nope", X224ConnectionPDU: rdpTestX224Request},
		"other device":     {Version: 3390, ProxyAuth: other, X224ConnectionPDU: rdpTestX224Request},
		"no ticket":        {Version: 3390, X224ConnectionPDU: rdpTestX224Request},
		"preconnection":    {Version: 3390, ProxyAuth: good, PreconnectionBlob: "x", X224ConnectionPDU: rdpTestX224Request},
		"missing x224 pdu": {Version: 3390, ProxyAuth: good},
	} {
		t.Run(name, func(t *testing.T) {
			ws := dialRDPGateway(t, srv)
			data, _ := pdu.marshal()
			_ = ws.WriteMessage(websocket.BinaryMessage, data)
			got := readRDPPDU(t, ws)
			if got.Error.ErrorCode != 1 || got.Error.HTTPStatusCode < 400 {
				t.Fatalf("expected a general error PDU, got %+v", got.Error)
			}
		})
	}
	select {
	case <-fake.gotCR:
		t.Fatal("a rejected request must never reach the RDP server")
	default:
	}

	t.Run("garbage", func(t *testing.T) {
		ws := dialRDPGateway(t, srv)
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte("not asn1"))
		if got := readRDPPDU(t, ws); got.Error.ErrorCode != 1 {
			t.Fatalf("got %+v", got.Error)
		}
	})
}

func TestHandleRDPProxyNegotiationFailure(t *testing.T) {
	failure := []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00, 0x03, 0x00, 0x08, 0x00, 0x02, 0x00, 0x00, 0x00}
	fake := newFakeRDPServer(t, failure)
	srv, store := rdpGatewayServer(t, "127.0.0.1", fake.listener.Addr().(*net.TCPAddr).Port)
	ws := dialRDPGateway(t, srv)
	ticket, _ := store.Issue("rdp1")
	req, _ := rdCleanPathPDU{Version: 3390, ProxyAuth: ticket, X224ConnectionPDU: rdpTestX224Request}.marshal()
	_ = ws.WriteMessage(websocket.BinaryMessage, req)
	got := readRDPPDU(t, ws)
	if got.Error.ErrorCode != 2 || !bytes.Equal(got.X224ConnectionPDU, failure) {
		t.Fatalf("expected negotiation error with x224 bytes, got %+v", got)
	}
}

func TestHandleRDPProxyDialFailure(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	srv, store := rdpGatewayServer(t, "127.0.0.1", port)
	ws := dialRDPGateway(t, srv)
	ticket, _ := store.Issue("rdp1")
	req, _ := rdCleanPathPDU{Version: 3390, ProxyAuth: ticket, X224ConnectionPDU: rdpTestX224Request}.marshal()
	_ = ws.WriteMessage(websocket.BinaryMessage, req)
	if got := readRDPPDU(t, ws); got.Error.ErrorCode != 103 {
		t.Fatalf("expected 103, got %+v", got.Error)
	}
}

func TestHandleRDPProxyTLSFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := readTPKT(conn); err != nil {
			return
		}
		_, _ = conn.Write(rdpTestX224Confirm)
		_, _ = io.Copy(io.Discard, io.LimitReader(conn, 1)) // read client hello byte, then hang up
	}()
	srv, store := rdpGatewayServer(t, "127.0.0.1", ln.Addr().(*net.TCPAddr).Port)
	ws := dialRDPGateway(t, srv)
	ticket, _ := store.Issue("rdp1")
	req, _ := rdCleanPathPDU{Version: 3390, ProxyAuth: ticket, X224ConnectionPDU: rdpTestX224Request}.marshal()
	_ = ws.WriteMessage(websocket.BinaryMessage, req)
	if got := readRDPPDU(t, ws); got.Error.ErrorCode != 105 {
		t.Fatalf("expected 105, got %+v", got.Error)
	}
}

func TestParseRDPTSRequestLogonFailure(t *testing.T) {
	pubKeyAuth := bytes.Repeat([]byte{0xAB}, 48)
	var body []byte
	body = append(body, 0xa0, 0x03, 0x02, 0x01, 0x06)
	body = append(body, 0xa3, 0x32, 0x04, 0x30)
	body = append(body, pubKeyAuth...)
	body = append(body, 0xa4, 0x06, 0x02, 0x04, 0xc0, 0x00, 0x00, 0x6d)
	data := append([]byte{0x30, byte(len(body))}, body...)
	if len(data) != 67 {
		t.Fatalf("vector must be 67 bytes, got %d", len(data))
	}
	info, ok := parseRDPTSRequest(data)
	if !ok {
		t.Fatal("expected a TSRequest")
	}
	if info.Version != 6 || !info.HasPubKeyAuth || info.PubKeyAuthLen != 48 || !info.HasErrorCode || info.ErrorCode != 0xc000006d {
		t.Fatalf("unexpected info %+v", info)
	}
	if info.HasNegoTokens || info.HasAuthInfo || info.HasClientNonce {
		t.Fatalf("unexpected optional fields %+v", info)
	}
}

func TestParseRDPTSRequestRejectsGarbage(t *testing.T) {
	for _, data := range [][]byte{nil, {0x16, 0x03, 0x03}, {0x30, 0x05, 0x01}, {0x30, 0x00}, {0x30, 0x03, 0x02, 0x01, 0x06}} {
		if _, ok := parseRDPTSRequest(data); ok {
			t.Errorf("%x must not parse", data)
		}
	}
}

func TestRDPCredSSPDiagLogsShapeOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	diag := &rdpCredSSPDiag{logger: logger, deviceID: "d", direction: "server_to_browser", remaining: 4}
	secret := bytes.Repeat([]byte{0x5A}, 8)
	body := append([]byte{0xa0, 0x03, 0x02, 0x01, 0x06, 0xa2, 0x0a, 0x04, 0x08}, secret...)
	diag.observe(append([]byte{0x30, byte(len(body))}, body...))
	out := buf.String()
	if !strings.Contains(out, "direction=server_to_browser") || !strings.Contains(out, "version=6") || !strings.Contains(out, "auth_info=true") {
		t.Fatalf("unexpected log %q", out)
	}
	if strings.Contains(out, "5a5a") || strings.Contains(out, "ZZZZ") {
		t.Fatalf("log must not contain field bytes: %q", out)
	}
}

func TestRDPTicketRateLimitResponse(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "rdp1", Name: "rdp1", Type: "server", Protocol: "rdp", IPAddress: "127.0.0.1", Tags: []string{}}); err != nil {
		t.Fatal(err)
	}
	store := NewRDPTicketStore()
	now := time.Now()
	store.now = func() time.Time { return now }
	handler := HandleRDPTicket(db, nil, testLogger, store, nil, nil)
	for i := 0; i < rdpTicketRateLimit; i++ {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/x?device_id=rdp1", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("issue %d: %d", i, rec.Code)
		}
		now = now.Add(time.Second)
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/x?device_id=rdp1", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "50" {
		t.Fatalf("Retry-After = %q, want 50", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var resp struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		RetryAfter int    `json:"retry_after"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != "rdp_ticket_rate_limited" || resp.RetryAfter != 50 || resp.Error == "" {
		t.Fatalf("unexpected body %+v", resp)
	}

	store.max = 0
	now = now.Add(2 * time.Minute)
	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/x?device_id=rdp1", nil))
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"rdp_ticket_capacity"`) || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("capacity response wrong: %d %s %v", rec.Code, rec.Body.String(), rec.Header())
	}
}
