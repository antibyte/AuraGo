package flows

import (
	"bytes"
	"errors"
	"testing"
)

func TestCheckpointEncryptionBindsRunAndGeneration(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	input := []byte(`{"state":"private"}`)
	a, err := sealCheckpoint(key, "run_a", 1, input)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sealCheckpoint(key, "run_a", 1, input)
	if err != nil || bytes.Equal(a, b) || bytes.Contains(a, input) {
		t.Fatalf("nonce or secrecy failure: %v", err)
	}
	plain, err := openCheckpoint(key, "run_a", 1, a)
	if err != nil || !bytes.Equal(plain, input) {
		t.Fatalf("round trip: %q %v", plain, err)
	}
	tampered := bytes.Clone(a)
	tampered[len(tampered)-1] ^= 1
	for _, tc := range []struct {
		key        []byte
		run        string
		generation uint64
		data       []byte
	}{
		{key, "run_b", 1, a}, {key, "run_a", 2, a}, {bytes.Repeat([]byte{2}, 32), "run_a", 1, a},
		{key, "run_a", 1, a[:len(a)-1]}, {key, "run_a", 1, tampered}, {key[:31], "run_a", 1, a},
	} {
		if _, err := openCheckpoint(tc.key, tc.run, tc.generation, tc.data); !errors.Is(err, errInvalidCheckpoint) {
			t.Fatalf("accepted invalid checkpoint: %v", err)
		}
	}
	if _, err := sealCheckpoint(key, "run_a", 1, make([]byte, maxCheckpointBytes+1)); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("oversize: %v", err)
	}
}
