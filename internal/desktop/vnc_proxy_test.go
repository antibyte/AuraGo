package desktop

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

func TestHandleVNCProxyMissingDeviceID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &security.Vault{}
	handler := HandleVNCProxy(db, vault, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/vnc", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing device_id") {
		t.Fatalf("expected missing device_id error, got %q", rec.Body.String())
	}
}

func TestHandleVNCProxyProtocolMismatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "test-ssh", Name: "test-ssh", Type: "server", Protocol: "ssh", IPAddress: "192.168.1.1", Port: 22, Username: "user", Description: "desc", Tags: []string{}})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	vault := &security.Vault{}
	handler := HandleVNCProxy(db, vault, testLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=test-ssh"
	header := http.Header{"Origin": []string{server.URL}}
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected error message, got read error: %v", err)
	}
	status := decodeVNCStatusMessage(t, msg)
	if status.Code != "protocol_mismatch" {
		t.Fatalf("expected protocol_mismatch code, got %#v", status)
	}
}

func TestHandleVNCProxyDeviceNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &security.Vault{}
	handler := HandleVNCProxy(db, vault, testLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=nonexistent"
	header := http.Header{"Origin": []string{server.URL}}
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected error message, got read error: %v", err)
	}
	status := decodeVNCStatusMessage(t, msg)
	if status.Code != "device_not_found" {
		t.Fatalf("expected device_not_found code, got %#v", status)
	}
}

func TestHandleVNCProxyNoVNCDeviceErrorUsesRFBSecurityFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	handler := HandleVNCProxy(db, &security.Vault{}, testLogger)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=nonexistent"
	dialer := websocket.Dialer{Subprotocols: []string{"binary"}}
	ws, _, err := dialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	version := readVNCBinaryMessage(t, ws)
	if string(version) != "RFB 003.008\n" {
		t.Fatalf("expected fake RFB version, got %q", string(version))
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, version); err != nil {
		t.Fatalf("write browser RFB version: %v", err)
	}
	status := readVNCSecurityFailure(t, ws)
	if status.Code != "device_not_found" {
		t.Fatalf("expected device_not_found code in RFB security failure, got %#v", status)
	}
}

func TestHandleVNCProxyDialFailureSendsErrorCode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	host, port := unusedLocalTCPAddress(t)
	err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "offline-vnc", Name: "offline-vnc", Type: "server", Protocol: "vnc", IPAddress: host, Port: port, Tags: []string{}})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	handler := HandleVNCProxy(db, &security.Vault{}, testLogger)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=offline-vnc"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	status := readVNCStatusMessage(t, ws)
	if status.Code != "dial_failed" {
		t.Fatalf("expected dial_failed code, got %#v", status)
	}
}

func TestHandleVNCProxyAuthFailureSendsErrorCode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	host, port, closeServer := startFakeVNCServer(t, func(conn net.Conn) error {
		return fakeRFBCredentialServer(t, conn, "test-pass")
	})
	defer closeServer()

	err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "auth-vnc", Name: "auth-vnc", Type: "server", Protocol: "vnc", IPAddress: host, Port: port, Tags: []string{}})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	handler := HandleVNCProxy(db, &security.Vault{}, testLogger)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=auth-vnc"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	connected := readVNCStatusMessage(t, ws)
	if connected.Type != "connected" {
		t.Fatalf("expected connected status before RFB handshake, got %#v", connected)
	}
	mt, version, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read RFB version from proxy: %v", err)
	}
	if mt != websocket.BinaryMessage || string(version) != "RFB 003.008\n" {
		t.Fatalf("unexpected RFB version frame: type=%d data=%q", mt, string(version))
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("RFB 003.008\n")); err != nil {
		t.Fatalf("write browser RFB version: %v", err)
	}
	status := readVNCStatusMessage(t, ws)
	if status.Code != "auth_failed" {
		t.Fatalf("expected auth_failed code, got %#v", status)
	}
}

func TestHandleVNCProxyNoVNCAuthFailureUsesRFBSecurityFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	host, port, closeServer := startFakeVNCServer(t, func(conn net.Conn) error {
		return fakeRFBCredentialServer(t, conn, "test-pass")
	})
	defer closeServer()

	err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "auth-vnc", Name: "auth-vnc", Type: "server", Protocol: "vnc", IPAddress: host, Port: port, Tags: []string{}})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	handler := HandleVNCProxy(db, &security.Vault{}, testLogger)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=auth-vnc"
	dialer := websocket.Dialer{Subprotocols: []string{"binary"}}
	ws, _, err := dialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	version := readVNCBinaryMessage(t, ws)
	if string(version) != "RFB 003.008\n" {
		t.Fatalf("unexpected RFB version frame: %q", string(version))
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, version); err != nil {
		t.Fatalf("write browser RFB version: %v", err)
	}
	status := readVNCSecurityFailure(t, ws)
	if status.Code != "auth_failed" {
		t.Fatalf("expected auth_failed code in RFB security failure, got %#v", status)
	}
}

func TestHandleVNCProxyInvalidRFBHandshakeSendsInitErrorCode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	host, port, closeServer := startFakeVNCServer(t, func(conn net.Conn) error {
		_, err := conn.Write([]byte("RFB BROKEN!\n"))
		return err
	})
	defer closeServer()

	err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "broken-vnc", Name: "broken-vnc", Type: "server", Protocol: "vnc", IPAddress: host, Port: port, Tags: []string{}})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	handler := HandleVNCProxy(db, &security.Vault{}, testLogger)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=broken-vnc"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	connected := readVNCStatusMessage(t, ws)
	if connected.Type != "connected" {
		t.Fatalf("expected connected status before RFB handshake, got %#v", connected)
	}
	status := readVNCStatusMessage(t, ws)
	if status.Code != "init_failed" {
		t.Fatalf("expected init_failed code for invalid RFB handshake, got %#v", status)
	}
}

func TestResolveVNCAccessDefaults(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	device := inventory.DeviceRecord{
		Name:      "vnc-test",
		Type:      "server",
		Protocol:  "vnc",
		IPAddress: "192.168.1.50",
		Port:      0,
	}

	// No credential, no vault
	host, port, password, err := resolveVNCAccess(device, db, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "192.168.1.50" {
		t.Fatalf("expected host 192.168.1.50, got %s", host)
	}
	if port != 5900 {
		t.Fatalf("expected port 5900, got %d", port)
	}
	if password != "" {
		t.Fatalf("expected empty password, got %q", password)
	}
}

func readVNCStatusMessage(t *testing.T, ws *websocket.Conn) sshStatusMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	if err := ws.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set websocket read deadline: %v", err)
	}
	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read VNC status message: %v", err)
		}
		if mt != websocket.TextMessage {
			continue
		}
		return decodeVNCStatusMessage(t, msg)
	}
}

func readVNCBinaryMessage(t *testing.T, ws *websocket.Conn) []byte {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	if err := ws.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set websocket read deadline: %v", err)
	}
	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read VNC binary message: %v", err)
		}
		if mt != websocket.BinaryMessage {
			continue
		}
		return msg
	}
}

func readVNCSecurityFailure(t *testing.T, ws *websocket.Conn) sshStatusMessage {
	t.Helper()
	msg := readVNCBinaryMessage(t, ws)
	if len(msg) < 5 {
		t.Fatalf("RFB security failure frame too short: %q", string(msg))
	}
	if msg[0] != 0 {
		t.Fatalf("expected RFB security type count 0, got %d in %q", msg[0], string(msg))
	}
	reasonLen := int(binary.BigEndian.Uint32(msg[1:5]))
	if len(msg) != 5+reasonLen {
		t.Fatalf("expected RFB security failure reason length %d, got frame length %d", reasonLen, len(msg))
	}
	return decodeVNCStatusMessage(t, msg[5:])
}

func decodeVNCStatusMessage(t *testing.T, msg []byte) sshStatusMessage {
	t.Helper()
	var status sshStatusMessage
	if err := json.Unmarshal(msg, &status); err != nil {
		t.Fatalf("decode VNC status %q: %v", string(msg), err)
	}
	return status
}

func unusedLocalTCPAddress(t *testing.T) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for unused tcp address: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	host := addr.IP.String()
	port := addr.Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close temporary listener: %v", err)
	}
	return host, port
}

func startFakeVNCServer(t *testing.T, handler func(net.Conn) error) (string, int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake VNC server: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = handler(conn)
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func TestVNCDialAddressSupportsIPv6(t *testing.T) {
	got := vncDialAddress("2001:db8::10", 5901)
	if got != "[2001:db8::10]:5901" {
		t.Fatalf("vncDialAddress() = %q, want [2001:db8::10]:5901", got)
	}
}

func TestPerformRFBHandshakeNoAuth(t *testing.T) {
	err := runRFBHandshakeTest(t, "", fakeRFBNoAuthServer, fakeNoAuthBrowser)
	if err != nil {
		t.Fatalf("performRFBHandshake no-auth: %v", err)
	}
}

func TestPerformRFBHandshakePasswordAuth(t *testing.T) {
	err := runRFBHandshakeTest(t, "test-pass", func(t *testing.T, conn net.Conn) error {
		return fakeRFBCredentialServer(t, conn, "test-pass")
	}, fakeNoAuthBrowser)
	if err != nil {
		t.Fatalf("performRFBHandshake password auth: %v", err)
	}
}

func TestPerformRFBHandshakeRejectsWrongPassword(t *testing.T) {
	err := runRFBHandshakeTest(t, "wrong", func(t *testing.T, conn net.Conn) error {
		return fakeRFBCredentialServer(t, conn, "test-pass")
	}, fakeNoAuthBrowser)
	if err == nil {
		t.Fatal("performRFBHandshake succeeded with wrong password")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("expected authentication failure, got %v", err)
	}
}

func runRFBHandshakeTest(t *testing.T, password string, serverFn func(*testing.T, net.Conn) error, browserFn func(*testing.T, net.Conn) error) error {
	t.Helper()
	browserClient, browserProxy := net.Pipe()
	serverProxy, serverPeer := net.Pipe()
	deadline := time.Now().Add(2 * time.Second)
	for _, conn := range []net.Conn{browserClient, browserProxy, serverProxy, serverPeer} {
		if err := conn.SetDeadline(deadline); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
	}
	defer browserClient.Close()
	defer browserProxy.Close()
	defer serverProxy.Close()
	defer serverPeer.Close()

	proxyErr := make(chan error, 1)
	serverErr := make(chan error, 1)
	browserErr := make(chan error, 1)
	go func() { proxyErr <- performRFBHandshake(browserProxy, serverProxy, password) }()
	go func() { serverErr <- serverFn(t, serverPeer) }()
	go func() { browserErr <- browserFn(t, browserClient) }()

	var err error
	select {
	case err = <-proxyErr:
	case <-time.After(3 * time.Second):
		t.Fatal("performRFBHandshake timed out")
	}
	_ = browserClient.Close()
	_ = serverPeer.Close()
	if serverErr := <-serverErr; serverErr != nil && err == nil {
		t.Fatalf("fake server failed: %v", serverErr)
	}
	if browserErr := <-browserErr; browserErr != nil && err == nil {
		t.Fatalf("fake browser failed: %v", browserErr)
	}
	return err
}

func fakeNoAuthBrowser(t *testing.T, conn net.Conn) error {
	t.Helper()
	version, err := readRFBTestBytes(conn, 12)
	if err != nil {
		return err
	}
	if string(version) != "RFB 003.008\n" {
		return errUnexpectedRFB("browser version", version)
	}
	if _, err := conn.Write(version); err != nil {
		return err
	}
	securityTypes, err := readRFBTestBytes(conn, 2)
	if err != nil {
		return err
	}
	if !bytes.Equal(securityTypes, []byte{1, 1}) {
		return errUnexpectedRFB("browser security types", securityTypes)
	}
	if _, err := conn.Write([]byte{1}); err != nil {
		return err
	}
	result, err := readRFBTestBytes(conn, 4)
	if err != nil {
		return err
	}
	if !bytes.Equal(result, []byte{0, 0, 0, 0}) {
		return errUnexpectedRFB("browser security result", result)
	}
	_, err = conn.Write([]byte{1})
	return err
}

func fakeRFBNoAuthServer(t *testing.T, conn net.Conn) error {
	t.Helper()
	version := []byte("RFB 003.008\n")
	if _, err := conn.Write(version); err != nil {
		return err
	}
	clientVersion, err := readRFBTestBytes(conn, 12)
	if err != nil {
		return err
	}
	if !bytes.Equal(clientVersion, version) {
		return errUnexpectedRFB("server client version", clientVersion)
	}
	if _, err := conn.Write([]byte{1, 1}); err != nil {
		return err
	}
	choice, err := readRFBTestBytes(conn, 1)
	if err != nil {
		return err
	}
	if !bytes.Equal(choice, []byte{1}) {
		return errUnexpectedRFB("server no-auth choice", choice)
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	clientInit, err := readRFBTestBytes(conn, 1)
	if err != nil {
		return err
	}
	if !bytes.Equal(clientInit, []byte{1}) {
		return errUnexpectedRFB("server client init", clientInit)
	}
	return nil
}

func fakeRFBCredentialServer(t *testing.T, conn net.Conn, expectedCredential string) error {
	t.Helper()
	version := []byte("RFB 003.008\n")
	if _, err := conn.Write(version); err != nil {
		return err
	}
	if _, err := readRFBTestBytes(conn, 12); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{1, 2}); err != nil {
		return err
	}
	choice, err := readRFBTestBytes(conn, 1)
	if err != nil {
		return err
	}
	if !bytes.Equal(choice, []byte{2}) {
		return errUnexpectedRFB("server password choice", choice)
	}
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	if _, err := conn.Write(challenge); err != nil {
		return err
	}
	response, err := readRFBTestBytes(conn, 16)
	if err != nil {
		return err
	}
	want, err := vncPasswordResponse(expectedCredential, challenge)
	if err != nil {
		return err
	}
	if !bytes.Equal(response, want) {
		_, _ = conn.Write([]byte{0, 0, 0, 1, 0, 0, 0, 0})
		return nil
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	clientInit, err := readRFBTestBytes(conn, 1)
	if err != nil {
		return err
	}
	if !bytes.Equal(clientInit, []byte{1}) {
		return errUnexpectedRFB("server password client init", clientInit)
	}
	return nil
}

func readRFBTestBytes(conn net.Conn, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(conn, buf)
	return buf, err
}

func errUnexpectedRFB(label string, got []byte) error {
	return fmt.Errorf("%s = %v", label, got)
}
