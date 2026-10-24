package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SetSourceHost assigns (NodeID) or clears ("") the device that runs one Docker
// engine instance. Version 0 creates the mapping; any other version must match
// the stored one. The instance must have been observed and only devices can
// host an engine. InferredNodeID is a runtime projection and is ignored.
func (s *Store) SetSourceHost(ctx context.Context, host SourceHost) (SourceHost, error) {
	host.NodeID = strings.TrimSpace(host.NodeID)
	host.InferredNodeID = ""
	if host.Kind != "docker" {
		return SourceHost{}, fmt.Errorf("%w: only Docker engines can be assigned a host", ErrValidation)
	}
	if err := validateText("source instance", host.Instance, 256, true); err != nil {
		return SourceHost{}, err
	}
	if err := validateText("host node id", host.NodeID, 128, false); err != nil {
		return SourceHost{}, err
	}
	if host.Version < 0 {
		return SourceHost{}, fmt.Errorf("%w: source host version must not be negative", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SourceHost{}, fmt.Errorf("begin infra source host update: %w", err)
	}
	defer tx.Rollback()
	var observed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_sources WHERE kind=? AND instance=?)`,
		host.Kind, host.Instance).Scan(&observed); err != nil {
		return SourceHost{}, fmt.Errorf("check infra source host instance: %w", err)
	}
	if !observed {
		return SourceHost{}, ErrSourceNotFound
	}
	if host.NodeID != "" {
		var kind NodeKind
		err := tx.QueryRowContext(ctx, `SELECT kind FROM infra_nodes WHERE id=?`, host.NodeID).Scan(&kind)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return SourceHost{}, fmt.Errorf("%w: host node does not exist", ErrValidation)
		case err != nil:
			return SourceHost{}, fmt.Errorf("read infra source host node: %w", err)
		case kind != NodeDevice:
			return SourceHost{}, fmt.Errorf("%w: only devices can run a Docker engine", ErrValidation)
		}
	}
	now := formatTime(time.Now().UTC())
	var res sql.Result
	if host.Version == 0 {
		res, err = tx.ExecContext(ctx, `INSERT INTO infra_source_hosts(kind, instance, node_id, version, updated_at)
			VALUES(?,?,?,1,?) ON CONFLICT(kind, instance) DO NOTHING`, host.Kind, host.Instance, nullIfEmpty(host.NodeID), now)
	} else {
		res, err = tx.ExecContext(ctx, `UPDATE infra_source_hosts SET node_id=?, version=version+1, updated_at=?
			WHERE kind=? AND instance=? AND version=?`, nullIfEmpty(host.NodeID), now, host.Kind, host.Instance, host.Version)
	}
	if err != nil {
		return SourceHost{}, fmt.Errorf("save infra source host: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return SourceHost{}, fmt.Errorf("count saved infra source hosts: %w", err)
	}
	if changed != 1 {
		return SourceHost{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return SourceHost{}, fmt.Errorf("commit infra source host update: %w", err)
	}
	host.Version++
	return host, nil
}

func (s *Store) listSourceHosts(ctx context.Context) ([]SourceHost, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, instance, COALESCE(node_id, ''), version FROM infra_source_hosts ORDER BY kind, instance`)
	if err != nil {
		return nil, fmt.Errorf("list infra source hosts: %w", err)
	}
	defer rows.Close()
	hosts := make([]SourceHost, 0)
	for rows.Next() {
		var host SourceHost
		if err := rows.Scan(&host.Kind, &host.Instance, &host.NodeID, &host.Version); err != nil {
			return nil, fmt.Errorf("read infra source host: %w", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra source hosts: %w", err)
	}
	return hosts, nil
}

// snapshotSourceHosts returns every stored mapping plus an unassigned entry
// (version 0) for each Docker instance seen among the snapshot's sources.
func (s *Store) snapshotSourceHosts(ctx context.Context, sources []Source) ([]SourceHost, error) {
	hosts, err := s.listSourceHosts(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		known[host.Kind+"\x00"+host.Instance] = struct{}{}
	}
	for _, source := range sources {
		key := source.Kind + "\x00" + source.Instance
		if _, exists := known[key]; exists || source.Kind != "docker" {
			continue
		}
		known[key] = struct{}{}
		hosts = append(hosts, SourceHost{Kind: source.Kind, Instance: source.Instance})
	}
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Kind != hosts[j].Kind {
			return hosts[i].Kind < hosts[j].Kind
		}
		return hosts[i].Instance < hosts[j].Instance
	})
	return hosts, nil
}
