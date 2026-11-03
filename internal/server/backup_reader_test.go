package server

import (
	"bytes"
	"testing"
)

func TestBackupEncryptionReadersRoundTripWithoutChangingInput(t *testing.T) {
	plain := bytes.Repeat([]byte("private fixture"), 1024)
	encrypted, err := encryptAGOReader(bytes.NewReader(plain), int64(len(plain)), "fixture password")
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(encrypted)
	decrypted, err := decryptAGOReader(bytes.NewReader(encrypted), int64(len(encrypted)), "fixture password")
	if err != nil || !bytes.Equal(decrypted, plain) {
		t.Fatalf("round trip failed: %v", err)
	}
	if !bytes.Equal(encrypted, original) {
		t.Fatal("decryption changed the source archive")
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err := decryptAGOReader(bytes.NewReader(encrypted), int64(len(encrypted)), "fixture password"); err == nil {
		t.Fatal("modified ciphertext was accepted")
	}
	if _, err := encryptAGOReader(bytes.NewReader(nil), agoMaxUploadSize+1, "fixture"); err == nil {
		t.Fatal("oversized allocation was accepted")
	}
	if _, err := encryptAGOReader(bytes.NewReader(nil), 100, "fixture"); err == nil {
		t.Fatal("truncated input was accepted")
	}
}
