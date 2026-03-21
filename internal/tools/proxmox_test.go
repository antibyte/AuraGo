package tools

import (
	"sync"
	"testing"
)

func TestGetProxmoxClient_CacheIsolatedByInsecureFlag(t *testing.T) {
	proxmoxClientCache = sync.Map{}

	secureClient := getProxmoxClient(ProxmoxConfig{Insecure: false})
	insecureClient := getProxmoxClient(ProxmoxConfig{Insecure: true})

	if secureClient == insecureClient {
		t.Fatal("expected secure and insecure clients to be different instances")
	}
}

func TestProxmoxRequest_RejectsNonHTTPSURL(t *testing.T) {
	_, _, err := proxmoxRequest(ProxmoxConfig{
		URL:     "http://example.com:8006",
		TokenID: "user@pam!token",
		Secret:  "secret",
	}, "GET", "/nodes", "")
	if err == nil {
		t.Fatal("expected non-https URL to be rejected")
	}
}

func TestProxmoxRequest_RejectsHTTPSWithoutHost(t *testing.T) {
	_, _, err := proxmoxRequest(ProxmoxConfig{
		URL:     "https://",
		TokenID: "user@pam!token",
		Secret:  "secret",
	}, "GET", "/nodes", "")
	if err == nil {
		t.Fatal("expected https URL without host to be rejected")
	}
}
