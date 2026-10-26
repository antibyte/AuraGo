package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// maxIgnoreBatch bounds one ignore or restore request.
const maxIgnoreBatch = 1000

// SetSourcesIgnored hides sources from discoveries (ignored) or restores them.
// The batch is atomic and idempotent; repeating an ignore keeps the first
// ignore time. A linked source cannot be ignored (ErrSourceLinked) and an
// unknown identity is ErrSourceNotFound.
func (s *Store) SetSourcesIgnored(ctx context.Context, identities []SourceIdentity, ignored bool) ([]Source, error) {
	if len(identities) == 0 || len(identities) > maxIgnoreBatch {
		return nil, fmt.Errorf("%w: ignore between 1 and %d sources", ErrValidation, maxIgnoreBatch)
	}
	seen := make(map[SourceIdentity]struct{}, len(identities))
	for _, identity := range identities {
		if err := validateIdentity(identity); err != nil {
			return nil, err
		}
		if _, exists := seen[identity]; exists {
			return nil, fmt.Errorf("%w: duplicate source in ignore request", ErrValidation)
		}
		seen[identity] = struct{}{}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin infra source ignore: %w", err)
	}
	defer tx.Rollback()
	now := formatTime(time.Now().UTC())
	for _, identity := range identities {
		if err := setSourceIgnoredTx(ctx, tx, identity, ignored, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit infra source ignore: %w", err)
	}
	sources := make([]Source, 0, len(identities))
	for _, identity := range identities {
		source, err := loadSource(ctx, s.db, identity)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func setSourceIgnoredTx(ctx context.Context, tx *sql.Tx, identity SourceIdentity, ignored bool, now string) error {
	var linkedID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT node_id FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`,
		identity.Kind, identity.Instance, identity.Key).Scan(&linkedID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSourceNotFound
	}
	if err != nil {
		return fmt.Errorf("read infra source for ignore: %w", err)
	}
	switch {
	case !ignored:
		_, err = tx.ExecContext(ctx, `UPDATE infra_sources SET ignored_at='' WHERE kind=? AND instance=? AND source_key=?`,
			identity.Kind, identity.Instance, identity.Key)
	case linkedID.Valid:
		return ErrSourceLinked
	default:
		_, err = tx.ExecContext(ctx, `UPDATE infra_sources SET ignored_at=? WHERE kind=? AND instance=? AND source_key=? AND ignored_at=''`,
			now, identity.Kind, identity.Instance, identity.Key)
	}
	if err != nil {
		return fmt.Errorf("record infra source ignore: %w", err)
	}
	return nil
}
