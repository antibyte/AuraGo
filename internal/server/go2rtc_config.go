package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"

	"gopkg.in/yaml.v3"
)

type go2RTCSourceChange struct {
	ID     string
	Value  string
	Delete bool
}

type go2RTCConfigUpdateResult struct {
	Config       config.Go2RTCConfig
	Published    bool
	ReconcileErr error
}

// updateManagedGo2RTCConfig is the common config/Vault transaction used by the
// camera app. It publishes the non-secret YAML and vault changes as one unit,
// replaces the runtime config snapshot, and then reconciles the managed sidecar.
func updateManagedGo2RTCConfig(
	ctx context.Context,
	s *Server,
	mutate func(*config.Go2RTCConfig) error,
	sourceChanges []go2RTCSourceChange,
	forceStart bool,
) (go2RTCConfigUpdateResult, error) {
	if s == nil || s.Cfg == nil || s.Vault == nil || s.Go2RTC == nil {
		return go2RTCConfigUpdateResult{}, fmt.Errorf("go2rtc configuration service is unavailable")
	}

	type publication struct {
		oldCfg   config.Config
		newCfg   *config.Config
		recreate bool
	}
	published, err := func() (publication, error) {
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()

		s.CfgMu.RLock()
		oldCfg := *s.Cfg
		oldCfg.Go2RTC.Streams = append([]config.Go2RTCStreamConfig(nil), s.Cfg.Go2RTC.Streams...)
		s.CfgMu.RUnlock()
		if strings.TrimSpace(oldCfg.ConfigPath) == "" {
			return publication{}, fmt.Errorf("config path is not set")
		}
		nextCfg := oldCfg
		nextCfg.Go2RTC.Streams = append([]config.Go2RTCStreamConfig(nil), oldCfg.Go2RTC.Streams...)
		if err := mutate(&nextCfg.Go2RTC); err != nil {
			return publication{}, err
		}
		if oldCfg.Go2RTC.Enabled && !nextCfg.Go2RTC.Enabled && (!nextCfg.Docker.Enabled || nextCfg.Docker.ReadOnly) {
			return publication{}, fmt.Errorf("Docker mutations must remain enabled while the managed go2rtc container is disabled")
		}

		vaultTxn, err := stageGo2RTCSourceChanges(s, sourceChanges)
		if err != nil {
			return publication{}, err
		}
		defer func() {
			if rollbackErr := vaultTxn.Rollback(); rollbackErr != nil && s.Logger != nil {
				s.Logger.Error("[go2rtc] Failed to roll back camera source transaction", "error", rollbackErr)
			}
		}()
		if err := validateManagedDockerBackends(nextCfg, oldCfg.Runtime); err != nil {
			return publication{}, err
		}
		if err := validateGo2RTCSettings(nextCfg.Go2RTC, oldCfg.Runtime, s.Vault); err != nil {
			return publication{}, err
		}

		data, err := os.ReadFile(oldCfg.ConfigPath)
		if err != nil {
			return publication{}, fmt.Errorf("read config: %w", err)
		}
		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			return publication{}, fmt.Errorf("parse config: %w", err)
		}
		rawCfg = normalizeConfigYAMLMap(rawCfg)
		go2RTCData, err := yaml.Marshal(nextCfg.Go2RTC)
		if err != nil {
			return publication{}, fmt.Errorf("marshal go2rtc config: %w", err)
		}
		var go2RTCSection map[string]interface{}
		if err := yaml.Unmarshal(go2RTCData, &go2RTCSection); err != nil {
			return publication{}, fmt.Errorf("normalize go2rtc config: %w", err)
		}
		rawCfg["go2rtc"] = normalizeConfigYAMLMap(go2RTCSection)
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			return publication{}, fmt.Errorf("marshal config: %w", err)
		}
		candidate, err := os.CreateTemp(filepath.Dir(oldCfg.ConfigPath), ".aurago-config-candidate-*")
		if err != nil {
			return publication{}, fmt.Errorf("create validation config: %w", err)
		}
		candidatePath := candidate.Name()
		defer os.Remove(candidatePath)
		if err := candidate.Chmod(0o600); err != nil {
			_ = candidate.Close()
			return publication{}, fmt.Errorf("protect validation config: %w", err)
		}
		if _, err := candidate.Write(out); err != nil {
			_ = candidate.Close()
			return publication{}, fmt.Errorf("write validation config: %w", err)
		}
		if err := candidate.Sync(); err != nil {
			_ = candidate.Close()
			return publication{}, fmt.Errorf("sync validation config: %w", err)
		}
		if err := candidate.Close(); err != nil {
			return publication{}, fmt.Errorf("close validation config: %w", err)
		}
		reloaded, err := config.Load(candidatePath)
		if err != nil {
			return publication{}, fmt.Errorf("validate config: %w", err)
		}
		reloaded.ConfigPath = oldCfg.ConfigPath
		reloaded.ApplyVaultSecrets(s.Vault)
		reloaded.ResolveProviders()
		reloaded.ApplyOAuthTokens(s.Vault)
		reloaded.Runtime = oldCfg.Runtime
		perm := os.FileMode(0o600)
		if info, statErr := os.Stat(oldCfg.ConfigPath); statErr == nil && info.Mode().Perm() != 0 {
			perm = info.Mode().Perm()
		}
		if err := config.WriteFileAtomic(oldCfg.ConfigPath, out, perm); err != nil {
			return publication{}, fmt.Errorf("write config: %w", err)
		}
		vaultTxn.Commit()
		_, recreate := go2RTCRuntimeTransition(oldCfg, *reloaded)
		s.CfgMu.Lock()
		s.replaceConfigSnapshot(reloaded)
		s.CfgMu.Unlock()
		return publication{oldCfg: oldCfg, newCfg: reloaded, recreate: recreate}, nil
	}()
	if err != nil {
		return go2RTCConfigUpdateResult{}, err
	}
	result := go2RTCConfigUpdateResult{Config: published.newCfg.Go2RTC, Published: true}

	runtimeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := applyGo2RTCRuntimeTransition(runtimeCtx, s, &published.oldCfg, published.newCfg, published.recreate, forceStart); err != nil {
		// ReconfigureContainer may temporarily restore the old manager snapshot
		// while removing the old sidecar. Always leave the manager on the
		// published desired state and record lifecycle work for background retry.
		s.Go2RTC.Configure(published.newCfg)
		pendingStop := !published.newCfg.Go2RTC.Enabled
		pendingStart := published.newCfg.Go2RTC.Enabled && !published.recreate && (published.newCfg.Go2RTC.AutoStart || forceStart)
		s.Go2RTC.SetRuntimeTransitionPending(published.recreate, pendingStart, pendingStop)
		result.ReconcileErr = fmt.Errorf("go2rtc reconciliation failed: %w", err)
	} else {
		s.Go2RTC.SetRuntimeTransitionPending(false, false, false)
	}
	return result, nil
}

func applyGo2RTCRuntimeTransition(ctx context.Context, s *Server, oldCfg, newCfg *config.Config, recreate, forceStart bool) error {
	if s == nil || s.Go2RTC == nil || oldCfg == nil || newCfg == nil {
		return fmt.Errorf("go2rtc manager is unavailable")
	}
	switch {
	case !oldCfg.Go2RTC.Enabled && !newCfg.Go2RTC.Enabled:
		s.Go2RTC.Configure(newCfg)
		return nil
	case !newCfg.Go2RTC.Enabled:
		// Keep the old container identity until it is stopped to avoid orphaning it.
		s.Go2RTC.Configure(oldCfg)
		if err := s.Go2RTC.StopContainer(); err != nil {
			return err
		}
		s.Go2RTC.Configure(newCfg)
		return nil
	case recreate:
		return s.Go2RTC.ReconfigureContainer(ctx, oldCfg, newCfg)
	default:
		s.Go2RTC.Configure(newCfg)
		if newCfg.Go2RTC.AutoStart || forceStart {
			if err := s.Go2RTC.StartContainer(ctx); err != nil {
				return err
			}
		} else if _, err := s.Go2RTC.Test(ctx); err != nil {
			return err
		}
		_, err := s.Go2RTC.ReconcileStreams(ctx)
		return err
	}
}

func stageGo2RTCSourceChanges(s *Server, changes []go2RTCSourceChange) (*go2RTCSourceVaultTransaction, error) {
	txn := &go2RTCSourceVaultTransaction{vault: s.Vault, previous: make(map[string]go2RTCSourceVaultSnapshot)}
	for _, change := range changes {
		key := config.Go2RTCStreamSourceVaultKey(change.ID)
		if key == "" {
			return nil, fmt.Errorf("invalid go2rtc stream id")
		}
		if _, captured := txn.previous[key]; !captured {
			value, err := s.Vault.ReadSecret(key)
			if err == nil {
				txn.previous[key] = go2RTCSourceVaultSnapshot{value: value, exists: true}
			} else if strings.Contains(strings.ToLower(err.Error()), "secret not found") {
				txn.previous[key] = go2RTCSourceVaultSnapshot{}
			} else {
				return nil, fmt.Errorf("read previous source for stream %q: %w", change.ID, err)
			}
		}
		if change.Delete {
			if err := s.Vault.DeleteSecret(key); err != nil {
				_ = txn.Rollback()
				return nil, fmt.Errorf("delete source for stream %q: %w", change.ID, err)
			}
			continue
		}
		value := strings.TrimSpace(change.Value)
		if value == "" {
			_ = txn.Rollback()
			return nil, fmt.Errorf("stream %q requires a source", change.ID)
		}
		security.RegisterSensitive(value)
		if err := s.Vault.WriteSecret(key, value); err != nil {
			_ = txn.Rollback()
			return nil, fmt.Errorf("save source for stream %q: %w", change.ID, err)
		}
	}
	return txn, nil
}
