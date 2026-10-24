package infra

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// sourceColumns is the column list scanSource reads, in order.
const sourceColumns = `kind, instance, source_key, name, state, fetched_at, removed_at, observed_json, ignored_at`

func CandidateID(identity SourceIdentity) string {
	return "infra_" + base64.RawURLEncoding.EncodeToString([]byte(identity.Kind+"\x00"+identity.Instance+"\x00"+identity.Key))
}

func (s *Store) GetSource(ctx context.Context, identity SourceIdentity) (Source, error) {
	if err := validateIdentity(identity); err != nil {
		return Source{}, err
	}
	return loadSource(ctx, s.db, identity)
}

func (s *Store) GetCandidate(ctx context.Context, id string) (Candidate, error) {
	identity, err := parseCandidateID(id)
	if err != nil {
		return Candidate{}, err
	}
	source, err := loadSource(ctx, s.db, identity)
	if err != nil {
		return Candidate{}, err
	}
	if source.State == SourceRemoved {
		return Candidate{}, ErrSourceRemoved
	}
	var linked bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_sources
		WHERE kind=? AND instance=? AND source_key=? AND node_id IS NOT NULL)`, identity.Kind, identity.Instance, identity.Key).Scan(&linked); err != nil {
		return Candidate{}, fmt.Errorf("check infra candidate link: %w", err)
	}
	if linked {
		return Candidate{}, ErrSourceLinked
	}
	observed := source.Observed
	name := strings.TrimSpace(observed.Name)
	if name == "" {
		name = source.Name
	}
	profile := Profile{
		Role: observed.Role, Vendor: observed.Vendor, Model: observed.Model, Serial: observed.Serial,
		OS: observed.OS, Firmware: observed.Firmware, CPU: observed.CPU, RAM: observed.RAM,
		Storage: observed.Storage, RackPosition: observed.RackPosition, IPAddress: observed.IPAddress,
		CIDR: observed.CIDR, VLAN: observed.VLAN, MACAddress: observed.MACAddress, Tags: cloneStrings(observed.Tags),
		Interfaces: append([]NetworkInterface{}, observed.Interfaces...), Links: []Link{}, AppBindings: []AppLink{}, Custom: map[string]string{},
	}
	node := Node{
		Kind: observed.Kind, Name: name, Icon: observed.Icon, Profile: profile,
		Placements: Placements{}, Sources: []Source{}, AvailableActions: []string{}, AppLinks: []AppLink{},
	}
	candidate := Candidate{ID: id, Source: source, Node: node, Suggestions: []string{}, Ignored: source.Ignored}
	if identity.Kind == "registry" {
		candidate.RegistryID = identity.Key
		node.RegistryID = identity.Key
		candidate.Node = node
	}
	return candidate, nil
}

func parseCandidateID(id string) (SourceIdentity, error) {
	if !strings.HasPrefix(id, "infra_") || len(id) > 2048 {
		return SourceIdentity{}, fmt.Errorf("%w: invalid candidate id", ErrValidation)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "infra_"))
	if err != nil {
		return SourceIdentity{}, fmt.Errorf("%w: invalid candidate id", ErrValidation)
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 {
		return SourceIdentity{}, fmt.Errorf("%w: invalid candidate identity", ErrValidation)
	}
	identity := SourceIdentity{Kind: parts[0], Instance: parts[1], Key: parts[2]}
	if err := validateIdentity(identity); err != nil {
		return SourceIdentity{}, err
	}
	return identity, nil
}

// ListSources lists observations, optionally of one kind and instance, without
// ignored sources. ListSourcesIncludingIgnored also returns those.
func (s *Store) ListSources(ctx context.Context, kind, instance string) ([]Source, error) {
	return s.listSources(ctx, kind, instance, false)
}

// ListSourcesIncludingIgnored lists observations including ignored sources.
func (s *Store) ListSourcesIncludingIgnored(ctx context.Context, kind, instance string) ([]Source, error) {
	return s.listSources(ctx, kind, instance, true)
}

func (s *Store) listSources(ctx context.Context, kind, instance string, includeIgnored bool) ([]Source, error) {
	query := `SELECT ` + sourceColumns + ` FROM infra_sources WHERE 1=1`
	args := make([]any, 0, 2)
	if kind != "" {
		query += ` AND kind=?`
		args = append(args, kind)
	}
	if instance != "" {
		query += ` AND instance=?`
		args = append(args, instance)
	}
	if !includeIgnored {
		query += ` AND ignored_at=''`
	}
	query += ` ORDER BY kind, instance, source_key`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list infra sources: %w", err)
	}
	defer rows.Close()
	sources := make([]Source, 0)
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra sources: %w", err)
	}
	return sources, nil
}

// ObserveSources records one provider enumeration. Incomplete results stale unseen
// sources but never claim they were removed; complete results confirm removals.
// A complete enumeration may legitimately be empty (a source with no resources),
// which removes every previously known source of that instance.
func (s *Store) ObserveSources(ctx context.Context, kind, instance string, observations []SourceObservation, complete bool) error {
	if err := validateIdentity(SourceIdentity{Kind: kind, Instance: instance, Key: "validation"}); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(observations))
	for i := range observations {
		observation := &observations[i]
		if observation.Identity.Kind != kind || observation.Identity.Instance != instance {
			return fmt.Errorf("%w: source observation does not match its enumeration", ErrValidation)
		}
		if err := validateIdentity(observation.Identity); err != nil {
			return err
		}
		if _, exists := seen[observation.Identity.Key]; exists {
			return fmt.Errorf("%w: duplicate source identity", ErrValidation)
		}
		seen[observation.Identity.Key] = struct{}{}
		if err := validateObserved(kind, observation.Observed); err != nil {
			return fmt.Errorf("source %q: %w", observation.Identity.Key, err)
		}
		if err := validateText("source name", observation.Name, 256, false); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin infra source observation: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	nowText := formatTime(now)
	if complete {
		if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET state='removed', removed_at=?
			WHERE kind=? AND instance=? AND state<>'removed'`, nowText, kind, instance); err != nil {
			return fmt.Errorf("mark missing infra sources removed: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET state='stale'
			WHERE kind=? AND instance=? AND state='current'`, kind, instance); err != nil {
			return fmt.Errorf("mark incomplete infra sources stale: %w", err)
		}
	}
	state := SourceCurrent
	for _, observation := range observations {
		data, err := json.Marshal(observation.Observed)
		if err != nil {
			return fmt.Errorf("encode infra source observation: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO infra_sources
			(kind, instance, source_key, name, state, fetched_at, removed_at, observed_json)
			VALUES(?,?,?,?,?,?, '', ?)
			ON CONFLICT(kind, instance, source_key) DO UPDATE SET
			name=excluded.name, state=excluded.state, fetched_at=excluded.fetched_at,
			removed_at='', observed_json=excluded.observed_json`,
			kind, instance, observation.Identity.Key, observation.Name, state, nowText, string(data)); err != nil {
			return fmt.Errorf("record infra source observation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra source observation: %w", err)
	}
	_, err = s.PruneSources(ctx, now.Add(-RemovedSourceRetention))
	if err != nil {
		return err
	}
	return nil
}

// UpsertObservation records one source observation without invalidating related
// sources. Use it for single-item updates (for example after a source edit);
// ObserveSources stays the path for provider enumerations.
func (s *Store) UpsertObservation(ctx context.Context, observation SourceObservation) error {
	if err := validateIdentity(observation.Identity); err != nil {
		return err
	}
	if err := validateObserved(observation.Identity.Kind, observation.Observed); err != nil {
		return fmt.Errorf("source %q: %w", observation.Identity.Key, err)
	}
	if err := validateText("source name", observation.Name, 256, false); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin infra source upsert: %w", err)
	}
	defer tx.Rollback()
	if err := upsertSourceTx(ctx, tx, observation.Identity, observation.Name, observation.Observed, SourceCurrent, time.Now().UTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra source upsert: %w", err)
	}
	return nil
}

func (s *Store) MarkSourcesStale(ctx context.Context, kind, instance string) error {
	if err := validateText("source kind", kind, 32, true); err != nil {
		return err
	}
	if err := validateText("source instance", instance, 256, true); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE infra_sources SET state='stale'
		WHERE kind=? AND instance=? AND state='current'`, kind, instance); err != nil {
		return fmt.Errorf("mark infra source observations stale: %w", err)
	}
	return nil
}

// LinkSource links an observed source to an existing node.
func (s *Store) LinkSource(ctx context.Context, nodeID string, identity SourceIdentity) (Node, error) {
	return s.LinkSourceWithLayout(ctx, nodeID, identity, ImportLayout{})
}

// LinkSourceWithLayout links a source and, in the same transaction and version
// step, applies an optional parent and view placement to the node.
func (s *Store) LinkSourceWithLayout(ctx context.Context, nodeID string, identity SourceIdentity, layout ImportLayout) (Node, error) {
	if err := validateIdentity(identity); err != nil {
		return Node{}, err
	}
	if err := validateText("node id", nodeID, 128, true); err != nil {
		return Node{}, err
	}
	if layout.Placement != nil && layout.Placement.View != ViewAtlas && layout.Placement.View != ViewTopology {
		return Node{}, fmt.Errorf("%w: unsupported placement view", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin infra source link: %w", err)
	}
	defer tx.Rollback()
	source, err := loadSource(ctx, tx, identity)
	if err != nil {
		return Node{}, err
	}
	if source.State == SourceRemoved {
		return Node{}, ErrSourceRemoved
	}
	node, err := loadNode(ctx, tx, nodeID)
	if err != nil {
		return Node{}, err
	}
	var linkedID sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT node_id FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`, identity.Kind, identity.Instance, identity.Key).Scan(&linkedID); err != nil {
		return Node{}, fmt.Errorf("check infra source link: %w", err)
	}
	if linkedID.Valid {
		if linkedID.String != nodeID {
			return Node{}, ErrSourceLinked
		}
		if err := tx.Commit(); err != nil {
			return Node{}, fmt.Errorf("commit existing infra source link: %w", err)
		}
		return s.GetNode(ctx, nodeID)
	}
	if identity.Kind == "registry" {
		if !registryLinkableKind(node.Kind) {
			return Node{}, fmt.Errorf("%w: registry sources can link only to devices, VMs, or containers", ErrValidation)
		}
		if node.RegistryID != "" && node.RegistryID != identity.Key {
			return Node{}, fmt.Errorf("%w: node already has a different registry device", ErrValidation)
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id=?)`, identity.Key).Scan(&exists); err != nil {
			return Node{}, fmt.Errorf("check registry device: %w", err)
		}
		if !exists {
			return Node{}, ErrSourceNotFound
		}
		// The unique registry index owns the one-node-per-device rule. Report a
		// claim by another node as a link conflict instead of a raw SQL error.
		var claimedBy string
		switch err := tx.QueryRowContext(ctx, `SELECT id FROM infra_nodes WHERE registry_id=?`, identity.Key).Scan(&claimedBy); {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return Node{}, fmt.Errorf("check registry link ownership: %w", err)
		case claimedBy != nodeID:
			return Node{}, ErrSourceLinked
		}
		node.RegistryID = identity.Key
	}
	if layout.ParentID != nil {
		parentID := strings.TrimSpace(*layout.ParentID)
		if err := validateParent(ctx, tx, node.ID, parentID); err != nil {
			return Node{}, err
		}
		node.ParentID = parentID
	}
	if layout.Placement != nil {
		placement := layout.Placement.AsPlacement()
		if err := validatePlacement(node.Kind, placement); err != nil {
			return Node{}, err
		}
		node.Placements[layout.Placement.View] = placement
	}
	placementsJSON, err := json.Marshal(node.Placements)
	if err != nil {
		return Node{}, fmt.Errorf("encode infra node placements: %w", err)
	}
	previous := node.Version
	node.Version++
	res, err := tx.ExecContext(ctx, `UPDATE infra_nodes SET registry_id=?, parent_id=?, placements_json=?, version=?, updated_at=? WHERE id=? AND version=?`,
		node.RegistryID, nullIfEmpty(node.ParentID), string(placementsJSON), node.Version, formatTime(time.Now().UTC()), node.ID, previous)
	if err != nil {
		return Node{}, fmt.Errorf("advance infra node version for source link: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return Node{}, fmt.Errorf("count linked infra source nodes: %w", err)
		}
		return Node{}, ErrConflict
	}
	res, err = tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=?, ignored_at='' WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`, nodeID, identity.Kind, identity.Instance, identity.Key)
	if err != nil {
		return Node{}, fmt.Errorf("set infra source link: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return Node{}, fmt.Errorf("count linked infra sources: %w", err)
		}
		return Node{}, ErrSourceLinked
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit infra source link: %w", err)
	}
	return s.GetNode(ctx, nodeID)
}

func registryLinkableKind(kind NodeKind) bool {
	return kind == NodeDevice || kind == NodeVM || kind == NodeContainer
}

func loadSource(ctx context.Context, q queryer, identity SourceIdentity) (Source, error) {
	row := q.QueryRowContext(ctx, `SELECT `+sourceColumns+`
		FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`, identity.Kind, identity.Instance, identity.Key)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrSourceNotFound
	}
	return source, err
}

type sourceScanner interface {
	Scan(...any) error
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func scanSource(row sourceScanner) (Source, error) {
	var source Source
	var fetchedAt, removedAt, observedJSON, ignoredAt string
	if err := row.Scan(&source.Kind, &source.Instance, &source.Key, &source.Name, &source.State, &fetchedAt, &removedAt, &observedJSON, &ignoredAt); err != nil {
		return Source{}, fmt.Errorf("read infra source: %w", err)
	}
	source.Ignored = ignoredAt != ""
	if fetchedAt != "" {
		t, err := time.Parse(timeLayout, fetchedAt)
		if err != nil {
			return Source{}, fmt.Errorf("parse infra source timestamp: %w", err)
		}
		source.FetchedAt = &t
	}
	if removedAt != "" {
		t, err := time.Parse(timeLayout, removedAt)
		if err != nil {
			return Source{}, fmt.Errorf("parse infra source removal timestamp: %w", err)
		}
		source.RemovedAt = &t
	}
	if err := json.Unmarshal([]byte(observedJSON), &source.Observed); err != nil {
		return Source{}, fmt.Errorf("decode infra source observation: %w", err)
	}
	normalizeObserved(&source.Observed)
	return source, nil
}

func validateIdentity(identity SourceIdentity) error {
	if err := validateText("source kind", identity.Kind, 32, true); err != nil {
		return err
	}
	if err := validateText("source instance", identity.Instance, 256, true); err != nil {
		return err
	}
	if err := validateText("source key", identity.Key, 512, true); err != nil {
		return err
	}
	switch identity.Kind {
	case "registry", "docker", "proxmox", "fritzbox":
	default:
		return fmt.Errorf("%w: unsupported source kind", ErrValidation)
	}
	if identity.Kind == "fritzbox" {
		if _, err := net.ParseMAC(identity.Key); err == nil {
			return fmt.Errorf("%w: Fritz!Box source key must be opaque", ErrValidation)
		}
	}
	return nil
}

func validateObserved(sourceKind string, observed SourceObserved) error {
	if !validNodeKind(observed.Kind) {
		return fmt.Errorf("%w: invalid observed node kind", ErrValidation)
	}
	if err := validateText("observed name", observed.Name, 200, true); err != nil {
		return err
	}
	if err := validateIcon(observed.Icon); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"role": observed.Role, "vendor": observed.Vendor, "model": observed.Model, "serial": observed.Serial,
		"os": observed.OS, "firmware": observed.Firmware, "cpu": observed.CPU, "ram": observed.RAM,
		"storage": observed.Storage, "rack position": observed.RackPosition,
	} {
		if err := validateText(name, value, 256, false); err != nil {
			return err
		}
	}
	if err := validateIP("observed ip address", observed.IPAddress); err != nil {
		return err
	}
	if err := validateSegmentFields(observed.CIDR, observed.VLAN); err != nil {
		return err
	}
	if err := validateText("source host", observed.Host, 256, false); err != nil {
		return err
	}
	switch observed.Power {
	case "", PowerRunning, PowerStopped, PowerPaused, PowerUnknown:
	default:
		return fmt.Errorf("%w: invalid observed power", ErrValidation)
	}
	if observed.MACAddress != "" {
		if _, err := net.ParseMAC(observed.MACAddress); err != nil {
			return fmt.Errorf("%w: invalid observed MAC address", ErrValidation)
		}
	}
	if sourceKind == "fritzbox" && observed.MACAddress != "" {
		return fmt.Errorf("%w: Fritz!Box MAC addresses must stay in the source adapter", ErrValidation)
	}
	if err := validateTags(observed.Tags); err != nil {
		return err
	}
	if err := validateInterfaces(observed.Interfaces, sourceKind == "fritzbox"); err != nil {
		return err
	}
	return nil
}

func normalizeObserved(observed *SourceObserved) {
	if observed.Tags == nil {
		observed.Tags = []string{}
	}
	if observed.Interfaces == nil {
		observed.Interfaces = []NetworkInterface{}
	}
}

func cloneStrings(values []string) []string {
	return append([]string{}, values...)
}

func upsertSourceTx(ctx context.Context, tx *sql.Tx, identity SourceIdentity, name string, observed SourceObserved, state SourceState, fetchedAt time.Time) error {
	if err := validateIdentity(identity); err != nil {
		return err
	}
	if err := validateText("source name", name, 256, false); err != nil {
		return err
	}
	if err := validateObserved(identity.Kind, observed); err != nil {
		return err
	}
	data, err := json.Marshal(observed)
	if err != nil {
		return fmt.Errorf("encode infra source observation: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO infra_sources
		(kind, instance, source_key, name, state, fetched_at, removed_at, observed_json)
		VALUES(?,?,?,?,?,?, '', ?)
		ON CONFLICT(kind, instance, source_key) DO UPDATE SET
		name=excluded.name, state=excluded.state, fetched_at=excluded.fetched_at,
		removed_at='', observed_json=excluded.observed_json`, identity.Kind, identity.Instance, identity.Key,
		name, state, formatTime(fetchedAt), string(data))
	if err != nil {
		return fmt.Errorf("upsert infra source: %w", err)
	}
	return nil
}
