package meshcentral

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	urlStr := "wss://mesh.example.com"
	username := "admin"
	password := "secret"
	loginToken := "token123"
	insecure := true

	client := NewClient(urlStr, username, password, loginToken, insecure)

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
