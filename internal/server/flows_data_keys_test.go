package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/vaultprompt"
)

func TestFlowDataKeysCreateOnceAndNeverCreateOnRead(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	keys := flowDataKeys{s: &Server{Vault: vault}}
	if _, err := keys.ScenarioKey(context.Background(), false); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("missing key read = %v", err)
	}
	if _, err := vault.ReadSecret(flowScenarioVaultKey); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("read created a key: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan []byte, 2)
	errorsCh := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, err := keys.ScenarioKey(context.Background(), true)
			results <- key
			errorsCh <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	first, second := <-results, <-results
	if len(first) != 32 || string(first) != string(second) {
		t.Fatal("concurrent callers did not receive the same key")
	}
	clear(first)
	clear(second)
	if _, err := keys.CheckpointKey(context.Background(), false); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("scenario creation also created a checkpoint key: %v", err)
	}
	if err := vault.WriteSecret(flowScenarioVaultKey, "not-valid-base64"); err != nil {
		t.Fatal(err)
	}
	if _, err := keys.ScenarioKey(context.Background(), true); err == nil {
		t.Fatal("malformed existing key was replaced or accepted")
	}
	other, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(t.TempDir(), "agent.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := other.WriteAgentSecret(flowScenarioVaultKey, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatal(err)
	}
	if _, err := (flowDataKeys{s: &Server{Vault: other}}).ScenarioKey(context.Background(), true); err == nil {
		t.Fatal("agent-readable existing key was accepted")
	}
}

func TestFlowDataKeysAreReservedAndBackedUp(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Vault: vault}
	keys := flowDataKeys{s: s}
	checkpoint, err := keys.CheckpointKey(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(checkpoint)
	scenario, err := keys.ScenarioKey(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(scenario)
	for _, name := range []string{flowCheckpointVaultKey, flowScenarioVaultKey} {
		if tools.IsPythonAccessibleSecret(name) {
			t.Fatal("flow data key is available to Python")
		}
		if _, err := vault.ReadSecretForAgent(name); err == nil {
			t.Fatal("flow data key is agent-readable")
		}
		if _, err := vaultprompt.NormalizeVaultKey(name); err == nil {
			t.Fatal("flow data key is available to the secret prompt")
		}
		post := httptest.NewRecorder()
		handleSetVaultSecret(s, post, httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":"`+name+`","value":"replacement"}`)))
		if post.Code != http.StatusForbidden {
			t.Fatalf("set reserved key = %d", post.Code)
		}
		remove := httptest.NewRecorder()
		handleDeleteVaultSecret(s, remove, httptest.NewRequest(http.MethodDelete, "/api/vault/secrets?key="+url.QueryEscape(name), nil))
		if remove.Code != http.StatusForbidden {
			t.Fatalf("delete reserved key = %d", remove.Code)
		}
	}
	list := httptest.NewRecorder()
	handleListVaultSecrets(s, list, httptest.NewRequest(http.MethodGet, "/api/vault/secrets", nil))
	if strings.Contains(list.Body.String(), flowCheckpointVaultKey) || strings.Contains(list.Body.String(), flowScenarioVaultKey) {
		t.Fatal("flow data key names appeared in the generic Vault list")
	}
	blob, err := exportVaultSecrets(vault, "backup-password")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "restored.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importVaultSecrets(restored, blob, "backup-password"); err != nil {
		t.Fatal(err)
	}
	restoredKeys := flowDataKeys{s: &Server{Vault: restored}}
	for _, tc := range []struct {
		read func(context.Context, bool) ([]byte, error)
		want []byte
	}{{restoredKeys.CheckpointKey, checkpoint}, {restoredKeys.ScenarioKey, scenario}} {
		got, err := tc.read(context.Background(), false)
		if err != nil || string(got) != string(tc.want) {
			t.Fatal("backup did not preserve flow data key", err)
		}
		clear(got)
	}
}
