package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckHomeAssistantConnectionUsesReadOnlyAPIEndpoint(t *testing.T) {
	const token = "home-assistant-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/" {
			t.Fatalf("request = %s %s, want GET /api/", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		fmt.Fprint(w, `{"message":"API running."}`)
	}))
	defer server.Close()

	if err := CheckHomeAssistantConnection(context.Background(), HAConfig{URL: server.URL, AccessToken: token}); err != nil {
		t.Fatalf("CheckHomeAssistantConnection() error = %v", err)
	}
}

func TestCheckProxmoxConnectionUsesVersionEndpoint(t *testing.T) {
	const tokenID = "user@pam!test"
	const secret = "proxmox-test-secret"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api2/json/version" {
			t.Fatalf("request = %s %s, want GET /api2/json/version", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "PVEAPIToken="+tokenID+"="+secret {
			t.Fatalf("Authorization = %q, want PVE API token", got)
		}
		fmt.Fprint(w, `{"data":{"version":"8.4"}}`)
	}))
	defer server.Close()

	if err := CheckProxmoxConnection(context.Background(), ProxmoxConfig{
		URL:      server.URL,
		TokenID:  tokenID,
		Secret:   secret,
		Insecure: true,
	}); err != nil {
		t.Fatalf("CheckProxmoxConnection() error = %v", err)
	}
}

func TestCheckS3ConnectionUsesHeadBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || !strings.HasPrefix(r.URL.Path, "/aurago-test-bucket") {
			t.Fatalf("request = %s %s, want HEAD bucket path", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := CheckS3Connection(context.Background(), S3Config{
		Endpoint:     server.URL,
		Region:       "us-east-1",
		Bucket:       "aurago-test-bucket",
		AccessKey:    "s3-test-access",
		SecretKey:    "s3-test-secret",
		UsePathStyle: true,
		Insecure:     true,
	}); err != nil {
		t.Fatalf("CheckS3Connection() error = %v", err)
	}
}

func TestCheckAnsibleConnectionUsesStatusEndpoint(t *testing.T) {
	const token = "ansible-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/status" {
			t.Fatalf("request = %s %s, want GET /status", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	if err := CheckAnsibleConnection(context.Background(), AnsibleConfig{URL: server.URL, Token: token}); err != nil {
		t.Fatalf("CheckAnsibleConnection() error = %v", err)
	}
}

func TestCheckProxmoxConnectionRejectsHTTP(t *testing.T) {
	err := CheckProxmoxConnection(context.Background(), ProxmoxConfig{
		URL:     "http://proxmox.example.test",
		TokenID: "user@pam!test",
		Secret:  "proxmox-test-secret",
	})
	if err == nil || !strings.Contains(err.Error(), "HTTPS is required") {
		t.Fatalf("CheckProxmoxConnection() error = %v, want HTTPS validation error", err)
	}
}

func TestCheckAnsibleConnectionHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := CheckAnsibleConnection(ctx, AnsibleConfig{URL: server.URL, Token: "ansible-test-token"})
	if err == nil {
		t.Fatal("CheckAnsibleConnection() error = nil, want canceled request error")
	}
}
