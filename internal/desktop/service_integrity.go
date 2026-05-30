package desktop

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const desktopIntegrityVaultKey = "virtual_desktop.integrity.ed25519_private_key"

var (
	desktopIntegrityFallbackMu         sync.Mutex
	desktopIntegrityFallbackPrivateKey ed25519.PrivateKey
)

// IntegritySecretStore is the minimal vault interface needed for desktop asset signing.
type IntegritySecretStore interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
}

type desktopIntegrityEnvelope struct {
	Kind   string            `json:"kind"`
	ID     string            `json:"id"`
	Hashes map[string]string `json:"hashes"`
}

// SetIntegritySecretStore injects the secret store used for local Ed25519 signing.
func (s *Service) SetIntegritySecretStore(store IntegritySecretStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.integritySecrets = store
	s.integrityPrivateKey = nil
	s.integrityPublicKey = nil
}

func (s *Service) buildDesktopIntegrity(kind, id, baseRel string, rels []string) (*IntegrityData, error) {
	hashes := map[string]string{}
	for _, rel := range cleanIntegrityRelPaths(rels) {
		fullPath, err := s.ResolvePath(filepath.ToSlash(filepath.Join(baseRel, rel)))
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read integrity file %q: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		hashes[rel] = "sha256:" + hex.EncodeToString(sum[:])
	}
	signature, err := s.signDesktopIntegrity(kind, id, hashes)
	if err != nil {
		return nil, err
	}
	return &IntegrityData{
		Hashes: hashes,
		Signature: &IntegritySignature{
			Algorithm: "ed25519",
			Value:     signature,
		},
	}, nil
}

func cleanIntegrityRelPaths(rels []string) []string {
	seen := map[string]struct{}{}
	for _, rel := range rels {
		rel = cleanDesktopPathSlash(rel)
		if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			continue
		}
		seen[rel] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func (s *Service) signDesktopIntegrity(kind, id string, hashes map[string]string) (string, error) {
	privateKey, _, err := s.desktopIntegrityKeys()
	if err != nil {
		return "", err
	}
	payload, err := desktopIntegrityPayload(kind, id, hashes)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload)), nil
}

func (s *Service) verifyDesktopIntegrity(kind, id, baseRel string, integrity *IntegrityData) string {
	if integrity == nil || len(integrity.Hashes) == 0 {
		return ""
	}
	if integrity.Signature == nil || strings.TrimSpace(integrity.Signature.Value) == "" {
		return "integrity_signature_missing"
	}
	if strings.TrimSpace(strings.ToLower(integrity.Signature.Algorithm)) != "ed25519" {
		return "integrity_signature_algorithm"
	}
	_, publicKey, err := s.desktopIntegrityKeys()
	if err != nil {
		return "integrity_key_unavailable"
	}
	payload, err := desktopIntegrityPayload(kind, id, integrity.Hashes)
	if err != nil {
		return "integrity_signature_invalid"
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(integrity.Signature.Value))
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return "integrity_signature_invalid"
	}
	for rel, want := range integrity.Hashes {
		cleanRel := cleanDesktopPathSlash(rel)
		if cleanRel == "" || cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, "../") || strings.HasPrefix(cleanRel, "/") {
			return "integrity_path_invalid"
		}
		want = strings.TrimSpace(want)
		if !strings.HasPrefix(want, "sha256:") {
			return "integrity_hash_invalid"
		}
		fullPath, err := s.ResolvePath(filepath.ToSlash(filepath.Join(baseRel, cleanRel)))
		if err != nil {
			return "integrity_path_invalid"
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "integrity_file_missing"
			}
			return "integrity_file_unreadable"
		}
		sum := sha256.Sum256(data)
		got := "sha256:" + hex.EncodeToString(sum[:])
		if !strings.EqualFold(got, want) {
			return "integrity_hash_mismatch"
		}
	}
	return ""
}

func desktopIntegrityPayload(kind, id string, hashes map[string]string) ([]byte, error) {
	return json.Marshal(desktopIntegrityEnvelope{
		Kind:   strings.TrimSpace(strings.ToLower(kind)),
		ID:     strings.TrimSpace(strings.ToLower(id)),
		Hashes: hashes,
	})
}

func (s *Service) desktopIntegrityKeys() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	s.mu.Lock()
	if s.integrityPrivateKey != nil && s.integrityPublicKey != nil {
		privateKey := ed25519.PrivateKey(append([]byte(nil), s.integrityPrivateKey...))
		publicKey := ed25519.PublicKey(append([]byte(nil), s.integrityPublicKey...))
		s.mu.Unlock()
		return privateKey, publicKey, nil
	}
	store := s.integritySecrets
	s.mu.Unlock()

	var privateKey ed25519.PrivateKey
	if store != nil {
		secret, err := store.ReadSecret(desktopIntegrityVaultKey)
		if err == nil {
			decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(secret))
			if decodeErr != nil {
				return nil, nil, fmt.Errorf("decode desktop integrity key: %w", decodeErr)
			}
			privateKey = ed25519.PrivateKey(decoded)
		} else if !isIntegritySecretMissing(err) {
			return nil, nil, fmt.Errorf("read desktop integrity key: %w", err)
		}
	}
	if privateKey == nil {
		if store == nil {
			generatedPrivate, err := desktopIntegrityFallbackKey()
			if err != nil {
				return nil, nil, err
			}
			privateKey = generatedPrivate
		} else {
			_, generatedPrivate, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				return nil, nil, fmt.Errorf("generate desktop integrity key: %w", err)
			}
			privateKey = generatedPrivate
			if err := store.WriteSecret(desktopIntegrityVaultKey, base64.StdEncoding.EncodeToString(privateKey)); err != nil {
				return nil, nil, fmt.Errorf("write desktop integrity key: %w", err)
			}
		}
	}
	if l := len(privateKey); l != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("desktop integrity key has invalid length %d", l)
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("desktop integrity public key unavailable")
	}

	s.mu.Lock()
	s.integrityPrivateKey = append([]byte(nil), privateKey...)
	s.integrityPublicKey = append([]byte(nil), publicKey...)
	s.mu.Unlock()
	return privateKey, publicKey, nil
}

func desktopIntegrityFallbackKey() (ed25519.PrivateKey, error) {
	desktopIntegrityFallbackMu.Lock()
	defer desktopIntegrityFallbackMu.Unlock()
	if desktopIntegrityFallbackPrivateKey != nil {
		return append(ed25519.PrivateKey(nil), desktopIntegrityFallbackPrivateKey...), nil
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate desktop integrity key: %w", err)
	}
	desktopIntegrityFallbackPrivateKey = append(ed25519.PrivateKey(nil), privateKey...)
	return append(ed25519.PrivateKey(nil), privateKey...), nil
}

func isIntegritySecretMissing(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "secret not found")
}

// VerifyGeneratedAssetIntegrity checks whether a generated app/widget asset is still trusted.
func (s *Service) VerifyGeneratedAssetIntegrity(ctx context.Context, rawPath string) (bool, string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return false, "", err
	}
	clean := cleanDesktopPathSlash(rawPath)
	if strings.HasPrefix(clean, "Apps/") {
		rest := strings.TrimPrefix(clean, "Apps/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			return true, "", nil
		}
		app, ok, err := s.findApp(ctx, strings.ToLower(parts[0]))
		if err != nil || !ok || app.Builtin || app.Integrity == nil {
			return true, "", err
		}
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			if _, ok := app.Integrity.Hashes[cleanDesktopPathSlash(parts[1])]; !ok {
				return false, "integrity_file_untracked", nil
			}
		}
		app = s.validateGeneratedAppEntry(ctx, app)
		if app.Health == "broken" && strings.HasPrefix(app.HealthReason, "integrity_") {
			return false, app.HealthReason, nil
		}
		return true, "", nil
	}
	if strings.HasPrefix(clean, "Widgets/") {
		widgets, err := s.listWidgets(ctx)
		if err != nil {
			return false, "", err
		}
		for _, widget := range widgets {
			if widget.Builtin || widget.Integrity == nil {
				continue
			}
			widget = s.validateWidgetEntry(widget)
			if widget.EntryPath == clean && widget.Health == "broken" && strings.HasPrefix(widget.HealthReason, "integrity_") {
				return false, widget.HealthReason, nil
			}
		}
	}
	return true, "", nil
}
