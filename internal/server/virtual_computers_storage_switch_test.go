package server

import (
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

func TestStorageSwitchTokenRoundTrip(t *testing.T) {
	token, err := virtualComputersIssueStorageSwitchToken("abc123", "session-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := virtualComputersConsumeStorageSwitchToken(token, "abc123", "session-1"); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if err := virtualComputersConsumeStorageSwitchToken(token, "abc123", "session-1"); err == nil {
		t.Fatal("token must be single-use")
	}
}

func TestStorageSwitchTokenRejectsWrongHash(t *testing.T) {
	token, err := virtualComputersIssueStorageSwitchToken("want-hash", "sess", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := virtualComputersConsumeStorageSwitchToken(token, "other-hash", "sess"); err == nil {
		t.Fatal("expected hash mismatch error")
	}
}

func TestStorageSwitchGateAllowsUnchangedIdentity(t *testing.T) {
	cfg := config.Config{}
	cfg.VirtualComputers.Storage.Mode = virtualcomputers.StorageModeManagedGarage
	msg, code, ok := virtualComputersEnforceStorageSwitchGate(nil, nil, cfg, cfg)
	if !ok || msg != "" || code != 0 {
		t.Fatalf("unchanged identity should pass: ok=%v code=%d msg=%q", ok, code, msg)
	}
}

func TestStorageSwitchContextRejectsMismatchedTargetHash(t *testing.T) {
	cfg := config.Config{}
	cfg.VirtualComputers.Storage.Mode = virtualcomputers.StorageModeManagedGarage
	s := &Server{Cfg: &cfg}

	_, _, _, err := virtualComputersStorageSwitchContext(s, virtualComputersStorageSwitchRequest{
		Mode:       virtualcomputers.StorageModeExternalS3,
		Endpoint:   "minio.internal:9000",
		Bucket:     "volumes",
		TargetHash: "not-the-computed-target-hash",
	})
	if err == nil {
		t.Fatal("expected target hash mismatch")
	}
}

func TestStorageSwitchContextAcceptsComputedTargetHash(t *testing.T) {
	cfg := config.Config{}
	cfg.VirtualComputers.Storage.Mode = virtualcomputers.StorageModeManagedGarage
	s := &Server{Cfg: &cfg}
	proposed := cfg.VirtualComputers
	proposed.Storage.Mode = virtualcomputers.StorageModeExternalS3
	proposed.Storage.Endpoint = "minio.internal:9000"
	proposed.Storage.Bucket = "volumes"
	wantHash := virtualcomputers.StorageIdentityFromConfig(proposed).Hash()

	_, target, _, err := virtualComputersStorageSwitchContext(s, virtualComputersStorageSwitchRequest{
		Mode:       virtualcomputers.StorageModeExternalS3,
		Endpoint:   "minio.internal:9000",
		Bucket:     "volumes",
		TargetHash: wantHash,
	})
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if got := target.Hash(); got != wantHash {
		t.Fatalf("target hash = %q, want %q", got, wantHash)
	}
}

func TestStorageSwitchTokenExpiryCleanup(t *testing.T) {
	virtualComputersStorageSwitchTokens.Lock()
	virtualComputersStorageSwitchTokens.byToken["old"] = virtualComputersStorageSwitchToken{
		Token: "old", TargetHash: "h", ExpiresAt: time.Now().Add(-time.Minute),
	}
	virtualComputersStorageSwitchTokens.Unlock()
	_, err := virtualComputersIssueStorageSwitchToken("h2", "s", false)
	if err != nil {
		t.Fatal(err)
	}
	virtualComputersStorageSwitchTokens.Lock()
	_, stillThere := virtualComputersStorageSwitchTokens.byToken["old"]
	virtualComputersStorageSwitchTokens.Unlock()
	if stillThere {
		t.Fatal("expired token should be cleaned on issue")
	}
}
