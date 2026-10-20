package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

type providerWriteError struct {
	status     int
	code       string
	message    string
	references []providerReferencePayload
}

func (e *providerWriteError) Error() string { return e.message }

func providerInvalid(message string) error {
	return &providerWriteError{status: http.StatusBadRequest, code: "provider_invalid", message: message}
}

func writeProviderError(w http.ResponseWriter, err error) {
	var detail *providerWriteError
	if !errors.As(err, &detail) {
		detail = &providerWriteError{status: http.StatusInternalServerError, code: "provider_save_failed", message: "Provider settings could not be saved"}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(detail.status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": detail.message, "code": detail.code, "references": detail.references})
}

// providerETag is an opaque edit revision, never a hash of credentials. Config
// publication rotates it even when only another feature changed its references.
func (s *Server) providerETag() string {
	if current := s.providerRevision.Load(); current != nil {
		return *current
	}
	version := `"` + rand.Text() + `"`
	if s.providerRevision.CompareAndSwap(nil, &version) {
		return version
	}
	return *s.providerRevision.Load()
}

func (s *Server) bumpProviderRevision() {
	version := `"` + rand.Text() + `"`
	s.providerRevision.Store(&version)
}

func (s *Server) checkProviderRevision(expected string) error {
	if strings.TrimSpace(expected) == "" {
		return &providerWriteError{status: http.StatusPreconditionRequired, code: "provider_revision_required", message: "Read providers first and send their ETag in If-Match"}
	}
	if expected != s.providerETag() {
		return &providerWriteError{status: http.StatusPreconditionFailed, code: "provider_revision_conflict", message: "Provider settings changed; reload before saving"}
	}
	return nil
}

type providerMutation func(current *config.Config) ([]config.ProviderEntry, []vaultMutation, error)

func validateNewProviderReferences(current, candidate *config.Config) error {
	previous := map[string]string{}
	for _, ref := range current.ProviderReferences() {
		previous[ref.Path] = ref.ProviderID
	}
	var conflicts []providerReferencePayload
	for _, ref := range candidate.MissingProviderReferences() {
		if previous[ref.Path] != ref.ProviderID {
			conflicts = append(conflicts, providerReferencePayload{Path: ref.Path, Role: ref.Role})
		}
	}
	if len(conflicts) > 0 {
		return &providerWriteError{status: http.StatusConflict, code: "provider_reference_conflict", message: "A selected provider no longer exists; reload the provider list", references: conflicts}
	}
	return nil
}

func validateProviderReferencePatch(current *config.Config, patch map[string]interface{}) error {
	candidate := current.Clone()
	data, err := json.Marshal(patch)
	if err != nil {
		return providerInvalid("Invalid provider reference patch")
	}
	// Other field validation belongs to the normal config path.
	if yaml.Unmarshal(data, candidate) != nil {
		return nil
	}
	return validateNewProviderReferences(current, candidate)
}

// Missing fields remain editable drafts. Provided fields still obey the full
// gateway syntax contract; runtime clients require the complete real entry.
func validateProviderDraftGateway(p config.ProviderEntry) error {
	if p.AccountID == "" {
		p.AccountID = "draft"
	}
	if p.GatewayID == "" {
		p.GatewayID = "draft"
	}
	if p.Model == "" {
		p.Model = "draft/model"
	}
	return config.ValidateGatewayProvider(p)
}

func rejectManagedProviderVaultWrite(s *Server, w http.ResponseWriter, key string) bool {
	managed := strings.HasPrefix(key, "provider_") && (strings.HasSuffix(key, "_api_key") || strings.HasSuffix(key, "_oauth_client_secret"))
	s.CfgMu.RLock()
	if s.Cfg != nil {
		for _, p := range s.Cfg.Providers {
			if key == "oauth_"+p.ID {
				managed = true
				break
			}
		}
	}
	s.CfgMu.RUnlock()
	if managed {
		writeProviderError(w, &providerWriteError{status: http.StatusBadRequest, code: "provider_use_api_providers", message: "Use the versioned provider settings or the provider OAuth workflow to change these credentials"})
	}
	return managed
}

// saveProviderConfig is called with CfgSaveMu held, after all candidate validation.
// It returns a resolved snapshot without publishing a partially saved state.
func saveProviderConfig(s *Server, path string, before, out []byte, mutations []vaultMutation) (*config.Config, error) {
	if s.Vault == nil {
		for _, mutation := range mutations {
			if !mutation.delete && mutation.value != "" {
				return nil, providerInvalid("A configured Vault is required to save credentials")
			}
		}
	}
	if err := config.WriteFileAtomic(path, out, 0o600); err != nil {
		return nil, fmt.Errorf("write provider config: %w", err)
	}
	snapshots, err := applyVaultMutations(s.Vault, mutations)
	if err == nil {
		var cfg *config.Config
		cfg, err = config.Load(path)
		if err == nil {
			cfg.ConfigPath = path
			cfg.ApplyVaultSecrets(s.Vault)
			cfg.ResolveProviders()
			cfg.ApplyOAuthTokens(s.Vault)
			return cfg, nil
		}
	}
	rollback := errors.Join(config.WriteFileAtomic(path, before, 0o600), restoreVaultSecrets(s.Vault, snapshots))
	if rollback != nil {
		if s.Logger != nil {
			s.Logger.Error("[Providers] Save rollback failed", "error", rollback)
		}
		return nil, &providerWriteError{status: http.StatusInternalServerError, code: "provider_rollback_failed", message: "Provider save failed and rollback was incomplete; inspect the server log"}
	}
	return nil, fmt.Errorf("save provider config: %w", err)
}

// prepareSetupProviders applies the same allowlist and secret semantics as PUT.
// The setup handler already holds the common save lock and guards first use.
func prepareSetupProviders(s *Server, patch map[string]interface{}) ([]vaultMutation, error) {
	raw, exists := patch["providers"]
	if !exists {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, providerInvalid("Invalid providers")
	}
	var incoming []providerJSON
	if err := json.Unmarshal(data, &incoming); err != nil {
		return nil, providerInvalid("Invalid providers")
	}
	entries, mutations, err := providerEntriesFromJSON(s, s.Cfg.Clone(), incoming)
	if err != nil {
		return nil, err
	}
	clean := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		clean = append(clean, buildProviderYAMLEntry(entry))
	}
	patch["providers"] = clean
	return mutations, nil
}

func validateProviderEntries(current, candidate *config.Config) error {
	existing := make(map[string]bool, len(current.Providers))
	for _, p := range current.Providers {
		existing[p.ID] = true
	}
	seen := map[string]bool{}
	seenFold := map[string]bool{}
	for _, p := range candidate.Providers {
		if err := validateProviderIDForSave(p.ID, existing); err != nil {
			return providerInvalid(err.Error())
		}
		if seen[p.ID] {
			return providerInvalid("Duplicate provider ID: " + p.ID)
		}
		if seenFold[strings.ToLower(p.ID)] {
			return providerInvalid("Duplicate provider ID: " + p.ID)
		}
		seenFold[strings.ToLower(p.ID)] = true
		seen[p.ID] = true
		for _, raw := range []string{p.BaseURL, p.OAuthAuthURL, p.OAuthTokenURL} {
			if raw == "" {
				continue
			}
			u, err := url.Parse(raw)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
				return providerInvalid("Provider URLs must be absolute HTTP(S) URLs without embedded credentials")
			}
		}
		for _, price := range p.Models {
			if strings.TrimSpace(price.Name) == "" || price.InputPerMillion < 0 || price.OutputPerMillion < 0 || math.IsInf(price.InputPerMillion, 0) || math.IsNaN(price.InputPerMillion) || math.IsInf(price.OutputPerMillion, 0) || math.IsNaN(price.OutputPerMillion) {
				return providerInvalid("Model prices require a name and finite nonnegative rates")
			}
		}
		if p.ContextWindow < 0 || p.MaxOutputTokens < 0 {
			return providerInvalid("context_window and max_output_tokens must be zero or positive")
		}
		if strings.TrimSpace(p.APIKey) == maskedKey || strings.TrimSpace(p.OAuthClientSecret) == maskedKey {
			return providerInvalid("A masked credential cannot be stored as a secret")
		}
		if err := config.ValidateProviderEntryUsage(p); err != nil {
			return providerInvalid(err.Error())
		}
		if err := validateProviderDraftGateway(p); err != nil {
			return providerInvalid(err.Error())
		}
	}
	for _, old := range current.Providers {
		if !seen[old.ID] {
			if refs := providerReferences(candidate, old.ID); len(refs) != 0 {
				return &providerWriteError{status: http.StatusConflict, code: "provider_in_use", message: "Provider is still referenced: " + old.ID, references: refs}
			}
		}
	}
	if err := config.ValidateDecisionsProvider(candidate); err != nil {
		return providerInvalid(err.Error())
	}
	return validateNewProviderReferences(current, candidate)
}

func readProviderSecret(s *Server, key, fallback string) (string, error) {
	if s.Vault == nil {
		return fallback, nil
	}
	value, err := s.Vault.ReadSecret(key)
	if errors.Is(err, security.ErrSecretNotFound) {
		return fallback, nil
	}
	if err != nil {
		return "", fmt.Errorf("read provider credential: %w", err)
	}
	return value, nil
}

func applyProviderSecret(op, value, existing string) (string, error) {
	value = strings.TrimSpace(value)
	switch op {
	case "keep":
		if value == maskedKey && (strings.TrimSpace(existing) == "" || strings.TrimSpace(existing) == maskedKey) {
			return "", providerInvalid("A masked credential requires an existing valid secret")
		}
		if strings.TrimSpace(existing) == maskedKey {
			return "", providerInvalid("Replace or remove the invalid masked credential")
		}
		return existing, nil
	case "clear":
		return "", nil
	case "set":
		if value == maskedKey || strings.HasPrefix(value, copyFromPrefix) {
			return "", providerInvalid("A placeholder cannot be stored as a credential")
		}
		return value, nil
	default:
		return "", providerInvalid("Unsupported credential operation")
	}
}

func providerEntriesFromJSON(s *Server, current *config.Config, incoming []providerJSON) ([]config.ProviderEntry, []vaultMutation, error) {
	keys, secrets, old := map[string]string{}, map[string]string{}, map[string]config.ProviderEntry{}
	for _, p := range current.Providers {
		old[p.ID] = p
		var err error
		keys[p.ID], err = readProviderSecret(s, "provider_"+p.ID+"_api_key", p.APIKey)
		if err != nil {
			return nil, nil, err
		}
		secrets[p.ID], err = readProviderSecret(s, "provider_"+p.ID+"_oauth_client_secret", p.OAuthClientSecret)
		if err != nil {
			return nil, nil, err
		}
	}
	entries := make([]config.ProviderEntry, 0, len(incoming))
	var mutations []vaultMutation
	for _, p := range incoming {
		if _, exists := old[p.ID]; !exists {
			p.ID = strings.TrimSpace(p.ID)
		}
		p.Usage = config.NormalizeProviderUsage(p.Usage)
		if p.Usage == "" {
			p.Usage = old[p.ID].Usage
		}
		auth := normalizeProviderAuthType(p.AuthType)
		value := p.APIKey
		if strings.HasPrefix(value, copyFromPrefix) {
			value = keys[strings.TrimPrefix(value, copyFromPrefix)]
			if value == "" || strings.TrimSpace(value) == maskedKey {
				return nil, nil, providerInvalid("The selected provider has no valid API key to copy")
			}
		}
		keyOp, secretOp := "set", "set"
		if strings.TrimSpace(value) == maskedKey {
			keyOp = "keep"
		}
		if strings.TrimSpace(p.OAuthClientSecret) == maskedKey {
			secretOp = "keep"
		}
		key, err := applyProviderSecret(keyOp, value, keys[p.ID])
		if err != nil {
			return nil, nil, err
		}
		secret, err := applyProviderSecret(secretOp, p.OAuthClientSecret, secrets[p.ID])
		if err != nil {
			return nil, nil, err
		}
		entry := config.ProviderEntry{ID: p.ID, Name: p.Name, Usage: p.Usage, Type: p.Type, BaseURL: p.BaseURL,
			APIKey: key, Model: p.Model, ContextWindow: p.ContextWindow, MaxOutputTokens: p.MaxOutputTokens,
			AccountID: p.AccountID, GatewayID: p.GatewayID, AuthType: auth, OAuthAuthURL: p.OAuthAuthURL,
			OAuthTokenURL: p.OAuthTokenURL, OAuthClientID: p.OAuthClientID, OAuthClientSecret: secret,
			OAuthScopes: p.OAuthScopes, Models: p.Models, Capabilities: providerCapabilitiesFromJSON(p.Capabilities)}
		entries = append(entries, entry)
		for _, credential := range []struct{ name, value string }{{"api_key", key}, {"oauth_client_secret", secret}} {
			mutations = append(mutations, vaultMutation{key: "provider_" + p.ID + "_" + credential.name, value: credential.value, delete: credential.value == ""})
		}
	}
	return entries, mutations, nil
}
