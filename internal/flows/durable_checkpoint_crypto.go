package flows

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const maxCheckpointBytes = 40 << 20

var errInvalidCheckpoint = errors.New("invalid flow checkpoint")

// checkpointAAD binds ciphertext to its run and generation, including a wire version.
func checkpointAAD(runID string, generation uint64) []byte {
	const domain = "aurago.flows.checkpoint.v1\x00"
	aad := make([]byte, len(domain)+8+len(runID))
	copy(aad, domain)
	binary.BigEndian.PutUint64(aad[len(domain):], generation)
	copy(aad[len(domain)+8:], runID)
	return aad
}

func checkpointAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errInvalidCheckpoint
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errInvalidCheckpoint
	}
	return cipher.NewGCM(block)
}

// sealCheckpoint rejects over-budget payloads and generates a fresh random nonce.
func sealCheckpoint(key []byte, runID string, generation uint64, plain []byte) ([]byte, error) {
	if runID == "" || len(runID) > 128 || generation == 0 {
		return nil, errInvalidCheckpoint
	}
	aead, err := checkpointAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(plain) > maxCheckpointBytes-aead.NonceSize()-aead.Overhead() {
		return nil, errInvalidCheckpoint
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("create checkpoint nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plain, checkpointAAD(runID, generation)), nil
}

func openCheckpoint(key []byte, runID string, generation uint64, sealed []byte) ([]byte, error) {
	if runID == "" || len(runID) > 128 || generation == 0 {
		return nil, errInvalidCheckpoint
	}
	aead, err := checkpointAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() || len(sealed) > maxCheckpointBytes {
		return nil, errInvalidCheckpoint
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], checkpointAAD(runID, generation))
	if err != nil {
		return nil, errInvalidCheckpoint
	}
	return plain, nil
}
