package desktop

import (
	"bytes"
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
	if !strings.Contains(string(msg), "expected vnc") {
		t.Fatalf("expected protocol mismatch error, got %q", string(msg))
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
	if !strings.Contains(string(msg), "not found") {
		t.Fatalf("expected device not found error, got %q", string(msg))
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
	err := runRFBHandshakeTest(t, "secret", func(t *testing.T, conn net.Conn) error {
		return fakeRFBPasswordServer(t, conn, "secret")
	}, fakeNoAuthBrowser)
	if err != nil {
		t.Fatalf("performRFBHandshake password auth: %v", err)
	}
}

func TestPerformRFBHandshakeRejectsWrongPassword(t *testing.T) {
	err := runRFBHandshakeTest(t, "wrong", func(t *testing.T, conn net.Conn) error {
		return fakeRFBPasswordServer(t, conn, "secret")
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

func fakeRFBPasswordServer(t *testing.T, conn net.Conn, expectedPassword string) error {
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
	want, err := vncPasswordResponse(expectedPassword, challenge)
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
