package meshcentral

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	urlStr := "https://mesh.example.com"
	username := "admin"
	password := "secret"
	loginToken := "token123"
	insecure := true

	client, err := NewClient(urlStr, username, password, loginToken, insecure)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client.url != urlStr {
		t.Errorf("expected url %s, got %s", urlStr, client.url)
	}
	if client.username != username {
		t.Errorf("expected username %s, got %s", username, client.username)
	}
	if client.password != password {
		t.Errorf("expected password %s, got %s", password, client.password)
	}
	if client.loginToken != loginToken {
		t.Errorf("expected loginToken %s, got %s", loginToken, client.loginToken)
	}
	if client.insecure != insecure {
		t.Errorf("expected insecure %v, got %v", insecure, client.insecure)
	}
	if client.pendingReqs == nil {
		t.Errorf("expected pendingReqs to be initialized")
	}
}

func TestNewClientValidation(t *testing.T) {
	// Test empty URL
	_, err := NewClient("", "admin", "pass", "", false)
	if err == nil {
		t.Error("expected error for empty URL")
	}

	// Test invalid URL scheme
	_, err = NewClient("ftp://invalid", "admin", "pass", "", false)
	if err == nil {
		t.Error("expected error for invalid URL scheme")
	}

	// Test valid URL with http
	_, err = NewClient("http://mesh.example.com", "admin", "pass", "", false)
	if err != nil {
		t.Errorf("unexpected error for valid http URL: %v", err)
	}

	// Test valid URL with https
	_, err = NewClient("https://mesh.example.com", "admin", "pass", "", false)
	if err != nil {
		t.Errorf("unexpected error for valid https URL: %v", err)
	}
}
