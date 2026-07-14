package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPrepareOutgoingWebhooksResolvesMaskedHeadersCaseInsensitively(t *testing.T) {
	t.Parallel()

	existing := []OutgoingWebhook{{
		ID:            "hook_1",
		URL:           "https://example.test/hook",
		SecretHeaders: map[string]string{"Authorization": "Bearer existing"},
	}}
	incoming := []OutgoingWebhook{{
		ID:      "hook_1",
		Method:  "POST",
		URL:     OutgoingWebhookMaskedValue,
		Headers: map[string]string{"authorization": OutgoingWebhookMaskedValue},
	}}

	prepared, err := PrepareOutgoingWebhooks(incoming, existing)
	if err != nil {
		t.Fatalf("PrepareOutgoingWebhooks() error = %v", err)
	}
	if got := prepared[0].SecretHeaders["authorization"]; got != "Bearer existing" {
		t.Fatalf("resolved authorization header = %q, want existing secret", got)
	}
}

type orderedOutgoingWebhookVault struct {
	mu         sync.Mutex
	data       map[string]string
	aWriteSeen chan struct{}
	releaseA   chan struct{}
	blockAOnce sync.Once
}

func (v *orderedOutgoingWebhookVault) ReadSecret(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.data[key], nil
}

func (v *orderedOutgoingWebhookVault) WriteSecret(key, value string) error {
	v.mu.Lock()
	v.data[key] = value
	v.mu.Unlock()

	if strings.Contains(value, "https://a.example/") {
		blocked := false
		v.blockAOnce.Do(func() {
			blocked = true
			close(v.aWriteSeen)
		})
		if blocked {
			<-v.releaseA
		}
	}
	return nil
}

func (v *orderedOutgoingWebhookVault) DeleteSecret(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.data, key)
	return nil
}

func TestPersistOutgoingWebhooksSerializesVaultAndConfigUpdates(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("webhooks:\n  outgoing:\n    - id: hook_1\n      name: Initial\n      method: POST\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	vault := &orderedOutgoingWebhookVault{
		data:       map[string]string{},
		aWriteSeen: make(chan struct{}),
		releaseA:   make(chan struct{}),
	}
	current := &Config{ConfigPath: configPath}
	current.Webhooks.Outgoing = []OutgoingWebhook{{ID: "hook_1", Name: "Initial", Method: "POST", URL: "https://initial.example/"}}

	type result struct {
		cfg *Config
		err error
	}
	resultA := make(chan result, 1)
	resultB := make(chan result, 1)
	go func() {
		cfg, err := PersistOutgoingWebhooks(configPath, current, []OutgoingWebhook{{ID: "hook_1", Name: "A", Method: "POST", URL: "https://a.example/"}}, vault)
		resultA <- result{cfg: cfg, err: err}
	}()
	<-vault.aWriteSeen
	go func() {
		cfg, err := PersistOutgoingWebhooks(configPath, current, []OutgoingWebhook{{ID: "hook_1", Name: "B", Method: "POST", URL: "https://b.example/"}}, vault)
		resultB <- result{cfg: cfg, err: err}
	}()

	var completedB *result
	select {
	case res := <-resultB:
		// The old implementation reaches and completes B while A is paused in its
		// vault write, producing A metadata with B secrets after A resumes.
		completedB = &res
	case <-time.After(300 * time.Millisecond):
		// A serialized implementation keeps B outside the transaction.
	}
	close(vault.releaseA)

	select {
	case res := <-resultA:
		if res.err != nil {
			t.Fatalf("persist A error = %v", res.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("persist A did not complete")
	}
	if completedB == nil {
		select {
		case res := <-resultB:
			completedB = &res
		case <-time.After(3 * time.Second):
			t.Fatal("persist B did not complete")
		}
	}
	if completedB.err != nil {
		t.Fatalf("persist B error = %v", completedB.err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	loaded.ApplyVaultSecrets(vault)
	if len(loaded.Webhooks.Outgoing) != 1 {
		t.Fatalf("outgoing hooks = %#v", loaded.Webhooks.Outgoing)
	}
	hook := loaded.Webhooks.Outgoing[0]
	wantURL := map[string]string{"A": "https://a.example/", "B": "https://b.example/"}[hook.Name]
	if hook.URL != wantURL {
		t.Fatalf("persisted hook combines metadata %q with URL %q, want %q", hook.Name, hook.URL, wantURL)
	}
}

type failingOutgoingDeleteVault struct {
	data    map[string]string
	failKey string
}

func (v *failingOutgoingDeleteVault) ReadSecret(key string) (string, error) {
	return v.data[key], nil
}

func (v *failingOutgoingDeleteVault) WriteSecret(key, value string) error {
	v.data[key] = value
	return nil
}

func (v *failingOutgoingDeleteVault) DeleteSecret(key string) error {
	if key == v.failKey {
		return errors.New("delete failed")
	}
	delete(v.data, key)
	return nil
}

func TestPersistOutgoingWebhooksRollsBackWhenRemovedBundleCannotBeDeleted(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("# keep\nwebhooks:\n  outgoing:\n    - id: hook_1\n      name: Old\n      method: POST\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	key := OutgoingWebhookSecretsVaultKey("hook_1")
	bundle, err := json.Marshal(OutgoingWebhookSecrets{URL: "https://old.example/hook"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	vault := &failingOutgoingDeleteVault{data: map[string]string{key: string(bundle)}, failKey: key}
	current := &Config{ConfigPath: configPath}
	current.Webhooks.Outgoing = []OutgoingWebhook{{ID: "hook_1", Name: "Old", Method: "POST", URL: "https://old.example/hook"}}

	if _, err := PersistOutgoingWebhooks(configPath, current, nil, vault); err == nil {
		t.Fatal("PersistOutgoingWebhooks() error = nil, want delete failure")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "id: hook_1") || !strings.Contains(string(data), "# keep") {
		t.Fatalf("config was not rolled back after delete failure:\n%s", data)
	}
	if got := vault.data[key]; got != string(bundle) {
		t.Fatalf("vault bundle after rollback = %q, want %q", got, bundle)
	}
}

func TestPersistOutgoingWebhooksRestoresEarlierRemovedBundlesWhenLaterDeleteFails(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("webhooks:\n  outgoing:\n    - id: hook_1\n      name: One\n      method: POST\n    - id: hook_2\n      name: Two\n      method: POST\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	key1 := OutgoingWebhookSecretsVaultKey("hook_1")
	key2 := OutgoingWebhookSecretsVaultKey("hook_2")
	bundle1 := `{"url":"https://one.example/hook"}`
	bundle2 := `{"url":"https://two.example/hook"}`
	vault := &failingOutgoingDeleteVault{
		data:    map[string]string{key1: bundle1, key2: bundle2},
		failKey: key2,
	}
	current := &Config{ConfigPath: configPath}
	current.Webhooks.Outgoing = []OutgoingWebhook{
		{ID: "hook_1", Name: "One", Method: "POST", URL: "https://one.example/hook"},
		{ID: "hook_2", Name: "Two", Method: "POST", URL: "https://two.example/hook"},
	}

	if _, err := PersistOutgoingWebhooks(configPath, current, nil, vault); err == nil {
		t.Fatal("PersistOutgoingWebhooks() error = nil, want second delete failure")
	}
	if got := vault.data[key1]; got != bundle1 {
		t.Fatalf("first removed bundle was not restored: got %q, want %q", got, bundle1)
	}
	if got := vault.data[key2]; got != bundle2 {
		t.Fatalf("failed bundle changed: got %q, want %q", got, bundle2)
	}
}
