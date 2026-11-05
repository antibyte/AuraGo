package testutil

import (
	"crypto/x509"
	"net/http"
	"testing"
)

func TestTLSFixtureTrustStaysInSubprocess(t *testing.T) {
	server := NewHTTPSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	verify := func() error {
		_, err := server.Certificate().Verify(x509.VerifyOptions{DNSName: "127.0.0.1"})
		return err
	}
	if verify() == nil {
		t.Fatal("fixture certificate was already trusted before setup")
	}
	if RunTLSFixtureSubprocess(t) {
		if verify() == nil {
			t.Fatal("child fixture changed the parent process root pool")
		}
		return
	}
	TrustHTTPSTestServer(t, server)
	if err := verify(); err != nil {
		t.Fatalf("isolated fixture certificate was not trusted: %v", err)
	}
}
