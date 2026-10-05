//go:build !remote_minimal

package remote

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// legacyHMACData must reproduce what agents built before the canonical form
// sign and verify. The expected string and HMAC were computed outside Go (a
// Python script concatenating the fields the way the old hmacData did), so the
// check is not circular.
func TestLegacyHMACDataMatchesPreUpgradeForm(t *testing.T) {
	msg := &RemoteMessage{
		Type:      MsgAuthResponse,
		DeviceID:  "dev-1",
		MessageID: "1700000000000000000",
		Sequence:  12,
		Nonce:     "0123456789abcdef0123456789abcdef",
		Timestamp: "2026-10-05T12:00:00Z",
		Payload:   []byte(`{"status":"rejected"}`),
	}
	const wantData = `auth_responsedev-11700000000000000000120123456789abcdef0123456789abcdef2026-10-05T12:00:00Z{"status":"rejected"}`
	const wantHMAC = "54aa7b2003314f25e2e4b0bcbf0e1f5c89b0f87ac0440d5d8920fa3bbecc5b28"
	if got := string(legacyHMACData(msg)); got != wantData {
		t.Fatalf("legacyHMACData = %q\nwant              %q", got, wantData)
	}
	key, _ := hex.DecodeString(strings.Repeat("ab", 32))
	mac := hmac.New(sha256.New, key)
	mac.Write(legacyHMACData(msg))
	if got := hex.EncodeToString(mac.Sum(nil)); got != wantHMAC {
		t.Fatalf("legacy HMAC = %s, want %s", got, wantHMAC)
	}
}

// A legacy refusal carries only status, message and a well-formed request
// nonce: old agents apply bootstrap settings before checking the status, and
// a malformed nonce would put requester-chosen bytes under the signature.
func TestLegacyRefusalCarriesOnlyFixedFieldsAndWellFormedNonces(t *testing.T) {
	key := strings.Repeat("ab", 32)
	valid := "0123456789abcdef0123456789abcdef"
	for name, nonce := range map[string]string{
		"empty":     "",
		"short":     "xyz",
		"uppercase": strings.ToUpper("abcdef" + valid[6:]),
		"33 chars":  valid + "0",
		"valid":     valid,
	} {
		refusal, err := legacyRefusal(nonce, key, "fixed reason")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var payload map[string]any
		if err := json.Unmarshal(refusal.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		wantNonce := ""
		if name == "valid" {
			wantNonce = valid
		}
		if got, _ := payload["request_nonce"].(string); got != wantNonce {
			t.Fatalf("%s: request_nonce = %q, want %q", name, got, wantNonce)
		}
		delete(payload, "request_nonce")
		if len(payload) != 2 || payload["status"] != "rejected" || payload["message"] != "fixed reason" {
			t.Fatalf("%s: legacy refusal payload = %s", name, refusal.Payload)
		}
		if refusal.Version != 0 || !verifiesLegacy(*refusal, key) {
			t.Fatalf("%s: refusal must be signed in the old form", name)
		}
	}
	if _, err := legacyRefusal(valid, "not-a-key", "fixed reason"); err == nil {
		t.Fatal("an unusable key must not produce a refusal")
	}
}

// If the enrollment row cannot be committed, the MAC key already written to the
// vault is deleted again, so no key outlives its row.
func TestIssueEnrollmentCommitFailureDeletesVaultKey(t *testing.T) {
	hub, db, vault := newEnrollmentTestHub(t)
	var rolledBack *sql.Tx
	_, err := hub.issueEnrollment("fresh-admin-token", "test", time.Now().Add(time.Hour).UTC().Format(time.RFC3339), func(tx *sql.Tx) error {
		// Ending the transaction here makes the later Commit fail after the
		// MAC key has been written.
		rolledBack = tx
		return tx.Rollback()
	})
	if rolledBack == nil || !errors.Is(err, sql.ErrTxDone) || !strings.Contains(err.Error(), "commit enrollment") {
		t.Fatalf("issueEnrollment must fail at the commit: %v", err)
	}
	keys, err := vault.ListKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if strings.HasPrefix(key, enrollmentAuthKeyPrefix) {
			t.Fatalf("vault key %s outlived its uncommitted enrollment", key)
		}
	}
	assertNoEnrollmentRows(t, db)
}
