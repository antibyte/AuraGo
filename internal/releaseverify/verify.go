// Package releaseverify verifies detached, keyed Cosign release signatures offline.
package releaseverify

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

//go:embed release.pub
var PublicKey []byte

const ManifestLimit = 4 << 20

// ReadFile bounds untrusted inputs before parsing or hashing them.
func ReadFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("input is not a bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("input changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("input exceeds size limit")
	}
	return data, nil
}

func parseKey(data []byte) (*ecdsa.PublicKey, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("expected one PEM public key")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("expected an ECDSA public key")
	}
	return key, nil
}

// Verify accepts the base64-encoded ASN.1 ECDSA signature emitted by Cosign sign-blob.
func Verify(data, signature, publicKey []byte) error {
	key, err := parseKey(publicKey)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(signature)))
	if err != nil || len(sig) > 256 {
		return errors.New("invalid detached signature encoding")
	}
	digest := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(key, digest[:], sig) {
		return errors.New("release signature verification failed")
	}
	return nil
}

// VerifyKeyring permits release-key rotation only when the embedded root signed
// the complete replacement keyring. The root remains available across skipped versions.
func VerifyKeyring(data, signature, keyring, keyringSignature []byte) error {
	return verifyKeyring(data, signature, keyring, keyringSignature, PublicKey)
}

func verifyKeyring(data, signature, keyring, keyringSignature, root []byte) error {
	if err := Verify(keyring, keyringSignature, root); err != nil {
		return fmt.Errorf("keyring: %w", err)
	}
	var ring struct {
		Version int      `json:"version"`
		Keys    []string `json:"keys"`
	}
	decoder := json.NewDecoder(bytes.NewReader(keyring))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ring); err != nil {
		return fmt.Errorf("decode keyring: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF || ring.Version != 1 || len(ring.Keys) == 0 || len(ring.Keys) > 16 {
		return errors.New("invalid release keyring")
	}
	for _, key := range ring.Keys {
		if _, err := parseKey([]byte(key)); err != nil {
			return fmt.Errorf("keyring public key: %w", err)
		}
	}
	for _, key := range ring.Keys {
		if Verify(data, signature, []byte(key)) == nil {
			return nil
		}
	}
	return errors.New("release is not signed by an authorized key")
}
