package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"aurago/internal/config"
	"aurago/internal/desktopstore"
	"aurago/internal/i18n"
	"gopkg.in/yaml.v3"
)

func validateDeepTutorLogin(ctx context.Context, candidate, current *config.Config) error {
	if desktopstore.RequireDeepTutorLogin(candidate.Auth.Enabled, candidate.Auth.PasswordHash) == nil {
		return nil
	}
	for _, cfg := range []*config.Config{current, candidate} {
		if cfg == nil {
			continue
		}
		dir := cfg.Directories.DataDir
		if dir == "" {
			dir = "data"
		}
		needed, err := desktopstore.DeepTutorNeedsLogin(ctx, filepath.Join(dir, config.DesktopStoreDBFilename))
		if err != nil {
			return err
		}
		if needed {
			return desktopstore.ErrDeepTutorRequiresLogin
		}
	}
	return nil
}

// Call under CfgSaveMu, before any YAML or Vault mutation.
func (s *Server) validateDeepTutorPatch(patch map[string]interface{}) error {
	current := s.ConfigSnapshot()
	candidate := current.Clone()
	if current.ConfigPath != "" {
		data, err := os.ReadFile(current.ConfigPath)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(data, candidate); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(patch)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, candidate); err != nil {
		return err
	}
	if auth, ok := patch["auth"].(map[string]interface{}); ok {
		if hash, ok := auth["password_hash"].(string); ok {
			candidate.Auth.PasswordHash = hash
		}
	}
	return validateDeepTutorLogin(context.Background(), candidate, current)
}

func writeDeepTutorPolicyError(s *Server, w http.ResponseWriter, err error) bool {
	if !errors.Is(err, desktopstore.ErrDeepTutorRequiresLogin) {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "error", "code": "deeptutor_requires_login",
		"message": i18n.T(s.ConfigSnapshot().Server.UILanguage, "backend.deeptutor_requires_login"),
	})
	return true
}

// Serialize admission and its durable pending record with configuration saves.
func (p storeLLMRelayProvider) AdmitDeepTutor(ctx context.Context) (func(), error) {
	p.s.CfgSaveMu.Lock()
	cfg := p.s.ConfigSnapshot()
	if err := desktopstore.RequireDeepTutorLogin(cfg.Auth.Enabled, cfg.Auth.PasswordHash); err != nil {
		p.s.CfgSaveMu.Unlock()
		return nil, err
	}
	return p.s.CfgSaveMu.Unlock, nil
}
