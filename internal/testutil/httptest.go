package testutil

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewHTTPServer starts an httptest server on IPv4 loopback. Some Windows
// agent environments have broken IPv6 loopback providers, which makes
// httptest.NewServer panic before tests can run.
func NewHTTPServer(t testing.TB, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

// NewHTTPSServer starts a TLS httptest server on IPv4 loopback, with the same
// skip behavior as NewHTTPServer when the environment cannot open sockets.
func NewHTTPSServer(t testing.TB, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.StartTLS()
	return server
}
