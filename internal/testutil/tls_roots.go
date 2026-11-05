package testutil

import (
	"context"
	"crypto/x509"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const tlsFixtureEnv = "AURAGO_TEST_TLS_FIXTURE"

var fallbackRootsOnce sync.Once

// RunTLSFixtureSubprocess returns true after running this test in an isolated process.
// Call it at the start of a default-client TLS test and return when it reports true.
func RunTLSFixtureSubprocess(t *testing.T) bool {
	t.Helper()
	if os.Getenv(tlsFixtureEnv) == t.Name() {
		return false
	}
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	parts := strings.Split(t.Name(), "/")
	for i, part := range parts {
		parts[i] = "^" + regexp.QuoteMeta(part) + "$"
	}
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run="+strings.Join(parts, "/"), "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), tlsFixtureEnv+"="+t.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated TLS fixture failed: %v\n%s", err, out)
	}
	t.Logf("isolated TLS fixture:\n%s", out)
	return true
}

// TrustHTTPSTestServer trusts httptest's certificate for clients using system roots.
// Only RunTLSFixtureSubprocess children may change the process-wide root pool.
func TrustHTTPSTestServer(t testing.TB, server *httptest.Server) {
	t.Helper()
	fixture := os.Getenv(tlsFixtureEnv)
	if fixture == "" || t.Name() != fixture && !strings.HasPrefix(t.Name(), fixture+"/") {
		t.Fatal("call RunTLSFixtureSubprocess at the start of this TLS fixture")
	}
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",x509usefallbackroots=1")
	// shortcut: httptest shares one built-in certificate; inject RootCAs for custom certificates.
	fallbackRootsOnce.Do(func() {
		pool := x509.NewCertPool()
		pool.AddCert(server.Certificate())
		x509.SetFallbackRoots(pool)
	})
}
