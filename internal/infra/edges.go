package infra

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/uid"
)

// CreateEdge adds a manual edge. A node cannot connect to itself, and one
// (from, to, kind) triple exists at most once (including provider edges).
func (s *Store) CreateEdge(ctx context.Context, edge Edge) (Edge, error) {
	if edge.ID != "" || edge.Version != 0 || edge.Source != nil {
		return Edge{}, fmt.Errorf("%w: new manual edges cannot choose identity, version, or source", ErrValidation)
	}
	if err := validateEdge(&edge); err != nil {
		return Edge{}, err
	}
	if edge.From == edge.To {
		return Edge{}, fmt.Errorf("%w: a connection needs two different nodes", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Edge{}, fmt.Errorf("begin infra edge creation: %w", err)
	}
	defer tx.Rollback()
	if err := validateEdgeEndpoints(ctx, tx, edge); err != nil {
		return Edge{}, err
	}
	var duplicate bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_edges WHERE from_id=? AND to_id=? AND kind=?)`,
		edge.From, edge.To, edge.Kind).Scan(&duplicate); err != nil {
		return Edge{}, fmt.Errorf("check duplicate infra edge: %w", err)
	}
	if duplicate {
		return Edge{}, ErrEdgeExists
	}
	edge.ID, edge.Version = uid.New(), 1
	if err := insertEdge(ctx, tx, edge); err != nil {
		return Edge{}, err
	}
	if err := tx.Commit(); err != nil {
		return Edge{}, fmt.Errorf("commit infra edge creation: %w", err)
	}
	return edge, nil
}

// UpdateEdge saves a manual edge under the same rules as CreateEdge: no
// self-loop, and no (from, to, kind) triple that another edge already has. An
// edge that keeps its triple stays editable even if an older duplicate of it
// exists, so legacy duplicates can still be relabelled or removed.
func (s *Store) UpdateEdge(ctx context.Context, edge Edge) (Edge, error) {
	if edge.ID == "" || edge.Version <= 0 || edge.Source != nil {
		return Edge{}, fmt.Errorf("%w: manual edge id and version are required and provider sources are read-only", ErrValidation)
	}
	if err := validateEdge(&edge); err != nil {
		return Edge{}, err
	}
	if edge.From == edge.To {
		return Edge{}, fmt.Errorf("%w: a connection needs two different nodes", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Edge{}, fmt.Errorf("begin infra edge update: %w", err)
	}
	defer tx.Rollback()
	current, err := loadEdge(ctx, tx, edge.ID)
	if err != nil {
		return Edge{}, err
	}
	if current.Source != nil {
		return Edge{}, ErrReadOnly
	}
	if current.Version != edge.Version {
		return Edge{}, ErrConflict
	}
	if err := validateEdgeEndpoints(ctx, tx, edge); err != nil {
		return Edge{}, err
	}
	if edge.From != current.From || edge.To != current.To || edge.Kind != current.Kind {
		var duplicate bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_edges WHERE from_id=? AND to_id=? AND kind=? AND id<>?)`,
			edge.From, edge.To, edge.Kind, edge.ID).Scan(&duplicate); err != nil {
			return Edge{}, fmt.Errorf("check duplicate infra edge: %w", err)
		}
		if duplicate {
			return Edge{}, ErrEdgeExists
		}
	}
	previous := edge.Version
	edge.Version++
	res, err := tx.ExecContext(ctx, `UPDATE infra_edges SET version=?, from_id=?, to_id=?, kind=?, label=?, from_port=?, to_port=?, speed=?, updated_at=?
		WHERE id=? AND version=? AND source_kind=''`, edge.Version, edge.From, edge.To, edge.Kind, edge.Label,
		edge.FromPort, edge.ToPort, edge.Speed, formatTime(time.Now().UTC()), edge.ID, previous)
	if err != nil {
		return Edge{}, fmt.Errorf("update infra edge: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return Edge{}, fmt.Errorf("count updated infra edges: %w", err)
		}
		return Edge{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return Edge{}, fmt.Errorf("commit infra edge update: %w", err)
	}
	return edge, nil
}

func (s *Store) DeleteEdge(ctx context.Context, id string, version int64) error {
	if id == "" || version <= 0 {
		return fmt.Errorf("%w: edge id and version are required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin infra edge removal: %w", err)
	}
	defer tx.Rollback()
	edge, err := loadEdge(ctx, tx, id)
	if err != nil {
		return err
	}
	if edge.Source != nil {
		return ErrReadOnly
	}
	if edge.Version != version {
		return ErrConflict
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM infra_edges WHERE id=? AND version=? AND source_kind=''`, id, version)
	if err != nil {
		return fmt.Errorf("delete infra edge: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return fmt.Errorf("count deleted infra edges: %w", err)
		}
		return ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra edge removal: %w", err)
	}
	return nil
}

// ReplaceProviderEdges replaces one provider-owned edge set after a complete read.
func (s *Store) ReplaceProviderEdges(ctx context.Context, identity SourceIdentity, edges []Edge) error {
	if err := validateIdentity(identity); err != nil {
		return err
	}
	if len(edges) > 5000 {
		return fmt.Errorf("%w: too many provider edges", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin provider infra edge refresh: %w", err)
	}
	defer tx.Rollback()
	if _, err := loadSource(ctx, tx, identity); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM infra_edges WHERE source_kind=? AND source_instance=? AND source_key=?`, identity.Kind, identity.Instance, identity.Key); err != nil {
		return fmt.Errorf("remove prior provider infra edges: %w", err)
	}
	for _, edge := range edges {
		if edge.Source != nil && *edge.Source != identity {
			return fmt.Errorf("%w: provider edge source mismatch", ErrValidation)
		}
		if err := validateEdge(&edge); err != nil {
			return err
		}
		if err := validateEdgeEndpoints(ctx, tx, edge); err != nil {
			return err
		}
		edge.ID = providerEdgeID(identity, edge)
		edge.Version = 1
		edge.Source = &identity
		if err := insertEdge(ctx, tx, edge); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider infra edge refresh: %w", err)
	}
	return nil
}

func (s *Store) listEdges(ctx context.Context) ([]Edge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, version, from_id, to_id, kind, label, from_port, to_port, speed,
		source_kind, source_instance, source_key FROM infra_edges ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list infra edges: %w", err)
	}
	defer rows.Close()
	edges := make([]Edge, 0)
	for rows.Next() {
		edge, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra edges: %w", err)
	}
	return edges, nil
}

func loadEdge(ctx context.Context, q nodeQueryer, id string) (Edge, error) {
	var edge Edge
	var sourceKind, sourceInstance, sourceKey string
	if err := q.QueryRowContext(ctx, `SELECT id, version, from_id, to_id, kind, label, from_port, to_port, speed,
		source_kind, source_instance, source_key FROM infra_edges WHERE id=?`, id).Scan(&edge.ID, &edge.Version,
		&edge.From, &edge.To, &edge.Kind, &edge.Label, &edge.FromPort, &edge.ToPort, &edge.Speed,
		&sourceKind, &sourceInstance, &sourceKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Edge{}, ErrNotFound
		}
		return Edge{}, fmt.Errorf("read infra edge: %w", err)
	}
	if sourceKind != "" {
		edge.Source = &SourceIdentity{Kind: sourceKind, Instance: sourceInstance, Key: sourceKey}
	}
	return edge, nil
}

func scanEdge(row interface{ Scan(...any) error }) (Edge, error) {
	var edge Edge
	var sourceKind, sourceInstance, sourceKey string
	if err := row.Scan(&edge.ID, &edge.Version, &edge.From, &edge.To, &edge.Kind, &edge.Label, &edge.FromPort, &edge.ToPort, &edge.Speed,
		&sourceKind, &sourceInstance, &sourceKey); err != nil {
		return Edge{}, fmt.Errorf("read infra edge: %w", err)
	}
	if sourceKind != "" {
		edge.Source = &SourceIdentity{Kind: sourceKind, Instance: sourceInstance, Key: sourceKey}
	}
	return edge, nil
}

func insertEdge(ctx context.Context, tx *sql.Tx, edge Edge) error {
	source := SourceIdentity{}
	if edge.Source != nil {
		source = *edge.Source
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO infra_edges
		(id, version, from_id, to_id, kind, label, from_port, to_port, speed, source_kind, source_instance, source_key, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, edge.ID, edge.Version, edge.From, edge.To, edge.Kind, edge.Label,
		edge.FromPort, edge.ToPort, edge.Speed, source.Kind, source.Instance, source.Key, formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("insert infra edge: %w", err)
	}
	return nil
}

func validateEdgeEndpoints(ctx context.Context, q nodeQueryer, edge Edge) error {
	for _, id := range []string{edge.From, edge.To} {
		var exists bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_nodes WHERE id=?)`, id).Scan(&exists); err != nil {
			return fmt.Errorf("check infra edge endpoint: %w", err)
		}
		if !exists {
			return fmt.Errorf("%w: edge endpoint %q does not exist", ErrValidation, id)
		}
	}
	return nil
}

func providerEdgeID(source SourceIdentity, edge Edge) string {
	identity := strings.Join([]string{source.Kind, source.Instance, source.Key, edge.From, edge.To, string(edge.Kind), edge.FromPort, edge.ToPort}, "\x00")
	hash := sha256.Sum256([]byte(identity))
	return "provider_" + hex.EncodeToString(hash[:16])
}
