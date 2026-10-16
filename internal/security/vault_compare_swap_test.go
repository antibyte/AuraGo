package security

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestVaultWriteSecretIfAbsentIsAtomicAndAgentHidden(t *testing.T) {
	v, err := NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	const writers = 12
	start := make(chan struct{})
	results := make(chan error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results <- v.WriteSecretIfAbsent("first-login", "token-"+strconv.Itoa(i))
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, occupied := 0, 0
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrSecretExists):
			occupied++
		default:
			t.Fatalf("unexpected first-login write error: %v", err)
		}
	}
	if wins != 1 || occupied != writers-1 {
		t.Fatalf("first-login winners = %d, occupied = %d; want 1 and %d", wins, occupied, writers-1)
	}
	if _, err := v.ReadSecretForAgent("first-login"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("first-login secret became agent-readable: %v", err)
	}
	if err := v.WriteSecretIfAbsent("first-login", "stale-token"); !errors.Is(err, ErrSecretExists) {
		t.Fatalf("existing token was overwritten: %v", err)
	}
}

func TestVaultCompareSwapPreservesConcurrentLoginAndRevocation(t *testing.T) {
	v, err := NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.WriteAgentSecret("fixture", "old"); err != nil {
		t.Fatal(err)
	}
	if err := v.CompareAndSwapSecret("fixture", "old", "rotated"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadSecretForAgent("fixture"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatal("rotation granted agent access")
	}
	if err := v.CompareAndSwapSecret("fixture", "old", "stale"); !errors.Is(err, ErrSecretChanged) {
		t.Fatal("stale write accepted")
	}
	if got, err := v.ReadSecret("fixture"); err != nil || got != "rotated" {
		t.Fatal("latest value lost")
	}
	if err := v.DeleteSecret("fixture"); err != nil {
		t.Fatal(err)
	}
	if err := v.CompareAndSwapSecret("fixture", "rotated", "revived"); !errors.Is(err, ErrSecretChanged) {
		t.Fatal("revoked secret revived")
	}
}

func TestVaultCompareAndDeletePreservesConcurrentRotationAndRevocation(t *testing.T) {
	v, err := NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.WriteSecret("fixture", "old"); err != nil {
		t.Fatal(err)
	}
	if err := v.CompareAndSwapSecret("fixture", "old", "rotated"); err != nil {
		t.Fatal(err)
	}
	if err := v.CompareAndDeleteSecret("fixture", "old"); !errors.Is(err, ErrSecretChanged) {
		t.Fatalf("stale compare-and-delete error = %v, want ErrSecretChanged", err)
	}
	if got, err := v.ReadSecret("fixture"); err != nil || got != "rotated" {
		t.Fatalf("concurrent rotation was lost: got %q, err %v", got, err)
	}
	if err := v.CompareAndDeleteSecret("fixture", "rotated"); err != nil {
		t.Fatalf("delete current value: %v", err)
	}
	if _, err := v.ReadSecret("fixture"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("deleted secret still exists: %v", err)
	}
	if err := v.CompareAndDeleteSecret("fixture", "rotated"); !errors.Is(err, ErrSecretChanged) {
		t.Fatalf("revoked value was accepted: %v", err)
	}
}
