package testutil

import (
	"crypto/x509"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

var fallbackRootsOnce sync.Once

// TrustHTTPSTestServer trusts httptest's certificate for clients using system roots.
// Its environment changes require a serial test, just like testing.T.Setenv.
func TrustHTTPSTestServer(t testing.TB, server *httptest.Server) {
	t.Helper()
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",x509usefallbackroots=1")
	// shortcut: httptest shares one built-in certificate; inject RootCAs for custom certificates.
	fallbackRootsOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		pool.AddCert(server.Certificate())
		x509.SetFallbackRoots(pool)
	})
}
