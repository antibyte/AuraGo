package releaseverify

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func fixtureKey(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func fixtureSignature(t *testing.T, key *ecdsa.PrivateKey, data []byte) []byte {
	t.Helper()
	digest := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return []byte(base64.StdEncoding.EncodeToString(sig))
}

func TestVerifyDetachedCosignSignature(t *testing.T) {
	key, pub := fixtureKey(t)
	_, wrongKey := fixtureKey(t)
	data := []byte("fixture manifest\n")
	sig := fixtureSignature(t, key, data)
	for _, tc := range []struct {
		name           string
		data, sig, key []byte
		valid          bool
	}{
		{"valid", data, sig, pub, true},
		{"Cosign newline", data, append(append([]byte{}, sig...), '\n'), pub, true},
		{"changed manifest", []byte("changed\n"), sig, pub, false},
		{"wrong signer", data, sig, wrongKey, false},
		{"invalid encoding", data, []byte("invalid!"), pub, false},
		{"truncated ASN1", data, []byte("AAAA"), pub, false},
		{"empty signature", data, nil, pub, false},
		{"extra public key", data, sig, append(append([]byte{}, pub...), pub...), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Verify(tc.data, tc.sig, tc.key); (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
}

func TestPublishedCosignManifest(t *testing.T) {
	data, err := ReadFile("testdata/SHA256SUMS", ManifestLimit)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := ReadFile("testdata/SHA256SUMS.sig", 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(data, sig, PublicKey); err != nil {
		t.Fatalf("published v2026.10.25 signature: %v", err)
	}
	if Verify(append(data, 'x'), sig, PublicKey) == nil {
		t.Fatal("tampered published manifest accepted")
	}
}

func TestRootSignedKeyRotation(t *testing.T) {
	root, rootPub := fixtureKey(t)
	successor, nextPub := fixtureKey(t)
	data := []byte("next release\n")
	sig := fixtureSignature(t, successor, data)
	ring, err := json.Marshal(struct {
		Version int      `json:"version"`
		Keys    []string `json:"keys"`
	}{1, []string{string(nextPub)}})
	if err != nil {
		t.Fatal(err)
	}
	rootSig := fixtureSignature(t, root, ring)
	if err := verifyKeyring(data, sig, ring, rootSig, rootPub); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		ring, proof []byte
	}{
		{"self signed", ring, fixtureSignature(t, successor, ring)},
		{"changed keyring", append(append([]byte{}, ring...), 'x'), rootSig},
		{"unsupported version", []byte(`{"version":2,"keys":[]}`), nil},
		{"unknown field", []byte(`{"version":1,"keys":[],"ignored":true}`), nil},
		{"trailing JSON", append(append([]byte{}, ring...), []byte(`{}`)...), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof := tc.proof
			if proof == nil {
				proof = fixtureSignature(t, root, tc.ring)
			}
			if verifyKeyring(data, sig, tc.ring, proof, rootPub) == nil {
				t.Fatal("unauthorized keyring accepted")
			}
		})
	}
}

func TestBoundedVerificationInputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input")
	if err := os.WriteFile(path, []byte("1234"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path, 3); err == nil {
		t.Fatal("oversized input accepted")
	}
	if _, err := ReadFile(dir, 10); err == nil {
		t.Fatal("directory accepted")
	}
	link := filepath.Join(dir, "link")
	if os.Symlink(path, link) == nil {
		if _, err := ReadFile(link, 4); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}
