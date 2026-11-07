package testutil

import (
	"crypto/x509"
	"net/http"
	"os"
	"testing"
)

func TestTLSFixtureSubprocessFiltersSensitiveEnvironment(t *testing.T) {
	const sentinel = "tls-fixture-only-secret"
	if os.Getenv(tlsFixtureEnv) != t.Name() {
		t.Setenv("AURAGO_MASTER_KEY", sentinel)
	}
	if RunTLSFixtureSubprocess(t) {
		if os.Getenv("AURAGO_MASTER_KEY") != sentinel {
			t.Fatal("child fixture changed the parent environment")
		}
		return
	}
	if os.Getenv("AURAGO_MASTER_KEY") != "" {
		t.Fatal("child fixture inherited the parent secret")
	}
}

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
