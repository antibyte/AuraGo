package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"aurago/internal/security"
)

const (
	flowCheckpointVaultKey = "flows_checkpoint_key_v1"
	flowScenarioVaultKey   = "flows_scenario_key_v1"
)

var (
	errFlowDataKeyMissing   = errors.New("flow data key is missing")
	errFlowDataKeyInvalid   = errors.New("flow data key is invalid")
	errFlowVaultUnavailable = errors.New("flow data vault is unavailable")
)

// flowDataKeys stores independent keys for durable checkpoints and private scenarios.
// Callers may request creation only after proving their encrypted store is empty.
type flowDataKeys struct{ s *Server }

func (k flowDataKeys) CheckpointKey(ctx context.Context, create bool) ([]byte, error) {
	return k.key(ctx, flowCheckpointVaultKey, create)
}

func (k flowDataKeys) ScenarioKey(ctx context.Context, create bool) ([]byte, error) {
	return k.key(ctx, flowScenarioVaultKey, create)
}

func (k flowDataKeys) key(ctx context.Context, name string, create bool) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if k.s == nil || k.s.Vault == nil {
		return nil, errFlowVaultUnavailable
	}
	value, err := k.s.Vault.ReadSecret(name)
	if errors.Is(err, security.ErrSecretNotFound) && create {
		var key [32]byte
		defer clear(key[:])
		if _, err := rand.Read(key[:]); err != nil {
			return nil, fmt.Errorf("generate flow data key: %w", err)
		}
		value = base64.StdEncoding.EncodeToString(key[:])
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if err = k.s.Vault.WriteSecretIfAbsent(name, value); err != nil && !errors.Is(err, security.ErrSecretExists) {
			return nil, fmt.Errorf("store flow data key: %w", err)
		}
		value, err = k.s.Vault.ReadSecret(name)
	}
	if err != nil {
		if errors.Is(err, security.ErrSecretNotFound) {
			return nil, fmt.Errorf("%w: %w", errFlowDataKeyMissing, err)
		}
		return nil, fmt.Errorf("read flow data key: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	readable, err := k.s.Vault.AgentCanReadSecret(name)
	if err != nil {
		return nil, fmt.Errorf("check flow data key access: %w", err)
	}
	if readable {
		return nil, errFlowDataKeyInvalid
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != value {
		clear(decoded)
		return nil, errFlowDataKeyInvalid
	}
	security.RegisterSensitive(value)
	return decoded, nil
}
