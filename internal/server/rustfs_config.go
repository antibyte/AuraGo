package server

import (
	"aurago/internal/config"
	"aurago/internal/rustfs"
	"context"
	"errors"
	"fmt"
	"time"
)

// Called under CfgSaveMu before writing; Docker/IAM work must not hold CfgMu.
func (s *Server) prepareRustFSConfigSave(ctx context.Context, next *config.Config) (func(bool, error), error) {
	if err := config.NormalizeRustFSConfig(next); err != nil {
		return nil, err
	}
	if s.RustFS == nil {
		return func(bool, error) {}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.RustFS.PrepareReconfigure(ctx, next); err != nil {
		s.RustFS.Configure(s.ConfigSnapshot())
		return nil, fmt.Errorf("RustFS configuration change refused: %s", rustfs.ErrorCode(err))
	}
	finished := false
	return func(committed bool, saveErr error) {
		if finished {
			return
		}
		finished = true
		var writeErr *providerWriteError
		if errors.As(saveErr, &writeErr) && writeErr.code == "provider_rollback_failed" {
			// Unknown persistence must not restore superseded S3 authority.
			return
		}
		current := s.ConfigSnapshot()
		if committed {
			// A reload failure must not restore grants superseded by a durable save.
			current = next
		}
		s.RustFS.Configure(current)
	}, nil
}
