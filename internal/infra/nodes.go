package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"aurago/internal/inventory"
	"aurago/internal/uid"
)

func (s *Store) GetNode(ctx context.Context, id string) (Node, error) {
	if err := validateText("node id", id, 128, true); err != nil {
		return Node{}, err
	}
	node, err := loadNode(ctx, s.db, id)
	if err != nil {
		return Node{}, err
	}
	if err := loadNodeSources(ctx, s.db, &node); err != nil {
		return Node{}, err
	}
	return node, nil
}

func (s *Store) CreateNode(ctx context.Context, node Node) (Node, error) {
	if node.ID != "" || node.Version != 0 || node.RegistryID != "" || len(node.Sources) != 0 {
		return Node{}, fmt.Errorf("%w: new nodes cannot choose identity, version, registry link, or sources", ErrValidation)
	}
	if err := validateNode(&node); err != nil {
		return Node{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin infra node creation: %w", err)
	}
	defer tx.Rollback()
	node.ID, node.Version = uid.New(), 1
	if err := validateParent(ctx, tx, node.ID, node.ParentID); err != nil {
		return Node{}, err
	}
	if err := insertNode(ctx, tx, node); err != nil {
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit infra node creation: %w", err)
	}
	return node, nil
}

// CreateManualNode creates a device node and its non-executable inventory entry atomically.
func (s *Store) CreateManualNode(ctx context.Context, node Node) (Node, error) {
	if node.Kind != NodeDevice || node.ID != "" || node.Version != 0 || node.RegistryID != "" || len(node.Sources) != 0 {
		return Node{}, fmt.Errorf("%w: manual registry creation requires a new device node", ErrValidation)
	}
	if err := validateNode(&node); err != nil {
		return Node{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin manual infra device creation: %w", err)
	}
	defer tx.Rollback()
	node.ID, node.Version = uid.New(), 1
	registryID := uid.New()
	if err := validateParent(ctx, tx, node.ID, node.ParentID); err != nil {
		return Node{}, err
	}
	if err := insertRegistryDevice(ctx, tx, registryID, node); err != nil {
		return Node{}, err
	}
	node.RegistryID = registryID
	if err := insertNode(ctx, tx, node); err != nil {
		return Node{}, err
	}
	identity := SourceIdentity{Kind: "registry", Instance: "inventory", Key: registryID}
	if err := upsertSourceTx(ctx, tx, identity, node.Name, sourceObservedFromNode(node), SourceCurrent, time.Now().UTC()); err != nil {
		return Node{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=? WHERE kind=? AND instance=? AND source_key=?`,
		node.ID, identity.Kind, identity.Instance, identity.Key); err != nil {
		return Node{}, fmt.Errorf("link manual inventory source: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit manual infra device creation: %w", err)
	}
	fetchedAt := time.Now().UTC()
	node.Sources = []Source{{SourceIdentity: identity, Name: node.Name, State: SourceCurrent, FetchedAt: &fetchedAt, Observed: sourceObservedFromNode(node)}}
	return node, nil
}

func (s *Store) CreateImportedNode(ctx context.Context, node Node, identity SourceIdentity) (Node, error) {
	if node.ID != "" || node.Version != 0 || node.RegistryID != "" || len(node.Sources) != 0 {
		return Node{}, fmt.Errorf("%w: imported nodes cannot choose identity, version, registry link, or sources", ErrValidation)
	}
	if err := validateIdentity(identity); err != nil {
		return Node{}, err
	}
	if err := validateNode(&node); err != nil {
		return Node{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin infra source import: %w", err)
	}
	defer tx.Rollback()
	source, err := loadSource(ctx, tx, identity)
	if err != nil {
		return Node{}, err
	}
	if source.State == SourceRemoved {
		return Node{}, ErrSourceRemoved
	}
	if source.Observed.Kind != node.Kind {
		return Node{}, fmt.Errorf("%w: imported node kind does not match its source", ErrValidation)
	}
	var linkedID sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT node_id FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`, identity.Kind, identity.Instance, identity.Key).Scan(&linkedID); err != nil {
		return Node{}, fmt.Errorf("check infra source link: %w", err)
	}
	if linkedID.Valid {
		return Node{}, ErrSourceLinked
	}
	if identity.Kind == "registry" {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id=?)`, identity.Key).Scan(&exists); err != nil {
			return Node{}, fmt.Errorf("check registry device for source import: %w", err)
		}
		if !exists {
			return Node{}, ErrSourceNotFound
		}
		node.RegistryID = identity.Key
	}
	node.ID, node.Version = uid.New(), 1
	if err := validateParent(ctx, tx, node.ID, node.ParentID); err != nil {
		return Node{}, err
	}
	if err := insertNode(ctx, tx, node); err != nil {
		return Node{}, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=?, ignored_at='' WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`,
		node.ID, identity.Kind, identity.Instance, identity.Key)
	if err != nil {
		return Node{}, fmt.Errorf("link imported infra source: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return Node{}, fmt.Errorf("count imported infra source links: %w", err)
		}
		return Node{}, ErrSourceLinked
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit infra source import: %w", err)
	}
	node.Sources = []Source{source}
	return node, nil
}

func (s *Store) UpdateNode(ctx context.Context, node Node) (Node, error) {
	if err := validateText("node id", node.ID, 128, true); err != nil {
		return Node{}, err
	}
	if node.Version <= 0 {
		return Node{}, fmt.Errorf("%w: node id and current version are required", ErrValidation)
	}
	// Stored placements replace the caller's below. Dropping a size the new kind
	// cannot carry keeps a place-to-device kind change valid.
	node.Placements = placementsForKind(node.Kind, node.Placements)
	if err := validateNode(&node); err != nil {
		return Node{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin infra node update: %w", err)
	}
	defer tx.Rollback()
	previous, err := loadNode(ctx, tx, node.ID)
	if err != nil {
		return Node{}, err
	}
	if previous.Version != node.Version {
		return Node{}, ErrConflict
	}
	if err := loadNodeSources(ctx, tx, &previous); err != nil {
		return Node{}, err
	}
	if previous.RegistryID != "" && !registryLinkableKind(node.Kind) {
		return Node{}, fmt.Errorf("%w: registry-backed nodes must remain devices, VMs, or containers", ErrValidation)
	}
	if err := validateChildKind(ctx, tx, node.ID, previous.Kind, node.Kind); err != nil {
		return Node{}, err
	}
	if err := validateParent(ctx, tx, node.ID, node.ParentID); err != nil {
		return Node{}, err
	}
	node.RegistryID = previous.RegistryID
	node.Sources = previous.Sources
	node.Version++
	node.Placements = placementsForKind(node.Kind, previous.Placements)
	if err := updateNodeTx(ctx, tx, node, previous.Version); err != nil {
		return Node{}, err
	}
	// Only devices can run a Docker engine (SetSourceHost). A device that
	// becomes another kind drops its mappings so the projection falls back to
	// inference instead of placing guests on a node that cannot host them.
	if previous.Kind == NodeDevice && node.Kind != NodeDevice {
		if _, err := tx.ExecContext(ctx, `DELETE FROM infra_source_hosts WHERE node_id=?`, node.ID); err != nil {
			return Node{}, fmt.Errorf("drop infra source hosts of a node that is no longer a device: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit infra node update: %w", err)
	}
	return s.GetNode(ctx, node.ID)
}

// DeleteNode removes a node and unlinks its sources.
func (s *Store) DeleteNode(ctx context.Context, id string, version int64) error {
	_, err := s.deleteNode(ctx, id, version, false)
	return err
}

// DeleteNodeAndIgnoreSources removes a node and, in the same transaction,
// unlinks and ignores every source it was linked to. It returns those sources.
func (s *Store) DeleteNodeAndIgnoreSources(ctx context.Context, id string, version int64) ([]SourceIdentity, error) {
	return s.deleteNode(ctx, id, version, true)
}

func (s *Store) deleteNode(ctx context.Context, id string, version int64, ignore bool) ([]SourceIdentity, error) {
	if err := validateText("node id", id, 128, true); err != nil {
		return nil, err
	}
	if version <= 0 {
		return nil, fmt.Errorf("%w: node id and version are required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin infra node removal: %w", err)
	}
	defer tx.Rollback()
	var current int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM infra_nodes WHERE id=?`, id).Scan(&current); err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read infra node version: %w", err)
	}
	if current != version {
		return nil, ErrConflict
	}
	var children bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_nodes WHERE parent_id=?)`, id).Scan(&children); err != nil {
		return nil, fmt.Errorf("check infra node children: %w", err)
	}
	if children {
		return nil, ErrHasChildren
	}
	ignored := []SourceIdentity{}
	if ignore {
		ignored, err = linkedSourceIdentities(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=NULL, ignored_at=? WHERE node_id=?`, formatTime(time.Now().UTC()), id); err != nil {
			return nil, fmt.Errorf("unlink and ignore infra node sources: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=NULL WHERE node_id=?`, id); err != nil {
		return nil, fmt.Errorf("unlink infra node sources: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM infra_nodes WHERE id=? AND version=?`, id, version)
	if err != nil {
		return nil, fmt.Errorf("delete infra node: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("count deleted infra nodes: %w", err)
	}
	if n != 1 {
		return nil, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit infra node removal: %w", err)
	}
	return ignored, nil
}

func linkedSourceIdentities(ctx context.Context, tx *sql.Tx, nodeID string) ([]SourceIdentity, error) {
	rows, err := tx.QueryContext(ctx, `SELECT kind, instance, source_key FROM infra_sources WHERE node_id=? ORDER BY kind, instance, source_key`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list infra node sources: %w", err)
	}
	defer rows.Close()
	identities := []SourceIdentity{}
	for rows.Next() {
		var identity SourceIdentity
		if err := rows.Scan(&identity.Kind, &identity.Instance, &identity.Key); err != nil {
			return nil, fmt.Errorf("read infra node source: %w", err)
		}
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra node sources: %w", err)
	}
	return identities, nil
}

type nodeQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadNode(ctx context.Context, q nodeQueryer, id string) (Node, error) {
	var node Node
	var profileJSON, placementsJSON string
	var parentID sql.NullString
	if err := q.QueryRowContext(ctx, `SELECT id, version, kind, name, icon, registry_id, parent_id, profile_json, placements_json
		FROM infra_nodes WHERE id=?`, id).Scan(&node.ID, &node.Version, &node.Kind, &node.Name, &node.Icon, &node.RegistryID, &parentID, &profileJSON, &placementsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, ErrNotFound
		}
		return Node{}, fmt.Errorf("read infra node: %w", err)
	}
	if parentID.Valid {
		node.ParentID = parentID.String
	}
	if err := json.Unmarshal([]byte(profileJSON), &node.Profile); err != nil {
		return Node{}, fmt.Errorf("decode infra node profile: %w", err)
	}
	if err := json.Unmarshal([]byte(placementsJSON), &node.Placements); err != nil {
		return Node{}, fmt.Errorf("decode infra node placements: %w", err)
	}
	normalizeProfile(&node.Profile)
	if node.Placements == nil {
		node.Placements = Placements{}
	}
	normalizeNodeRuntime(&node)
	node.Sources = []Source{}
	return node, nil
}

func loadNodeSources(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, node *Node) error {
	rows, err := q.QueryContext(ctx, `SELECT `+sourceColumns+`
		FROM infra_sources WHERE node_id=? ORDER BY kind, instance, source_key`, node.ID)
	if err != nil {
		return fmt.Errorf("list sources for infra node: %w", err)
	}
	defer rows.Close()
	node.Sources = make([]Source, 0)
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return err
		}
		node.Sources = append(node.Sources, source)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read sources for infra node: %w", err)
	}
	return nil
}

func insertNode(ctx context.Context, tx *sql.Tx, node Node) error {
	profileJSON, err := json.Marshal(node.Profile)
	if err != nil {
		return fmt.Errorf("encode infra node profile: %w", err)
	}
	placementsJSON, err := json.Marshal(node.Placements)
	if err != nil {
		return fmt.Errorf("encode infra node placements: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO infra_nodes
		(id, version, kind, name, icon, registry_id, parent_id, profile_json, placements_json, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, node.ID, node.Version, node.Kind, node.Name, node.Icon, node.RegistryID,
		nullIfEmpty(node.ParentID), string(profileJSON), string(placementsJSON), formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("insert infra node: %w", err)
	}
	return nil
}

func updateNodeTx(ctx context.Context, tx *sql.Tx, node Node, expected int64) error {
	profileJSON, err := json.Marshal(node.Profile)
	if err != nil {
		return fmt.Errorf("encode infra node profile: %w", err)
	}
	placementsJSON, err := json.Marshal(node.Placements)
	if err != nil {
		return fmt.Errorf("encode infra node placements: %w", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE infra_nodes SET version=?, kind=?, name=?, icon=?, parent_id=?, profile_json=?, placements_json=?, updated_at=?
		WHERE id=? AND version=?`, node.Version, node.Kind, node.Name, node.Icon, nullIfEmpty(node.ParentID), string(profileJSON), string(placementsJSON),
		formatTime(time.Now().UTC()), node.ID, expected)
	if err != nil {
		return fmt.Errorf("update infra node: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated infra nodes: %w", err)
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// containerKind reports whether nodes of this kind may contain other nodes.
func containerKind(kind NodeKind) bool {
	return kind == NodeSite || kind == NodeRoom || kind == NodeRack || kind == NodeGroup
}

// validateChildKind protects the container hierarchy in both directions: a node
// that still has children may not become a kind that cannot contain nodes.
func validateChildKind(ctx context.Context, tx *sql.Tx, nodeID string, previous, next NodeKind) error {
	if previous == next || containerKind(next) {
		return nil
	}
	var children bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_nodes WHERE parent_id=?)`, nodeID).Scan(&children); err != nil {
		return fmt.Errorf("check infra node children for a kind change: %w", err)
	}
	if children {
		return ErrHasChildren
	}
	return nil
}

func validateParent(ctx context.Context, tx *sql.Tx, nodeID, parentID string) error {
	if parentID == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for current := parentID; current != ""; {
		if current == nodeID {
			return fmt.Errorf("%w: parent relationship would create a cycle", ErrValidation)
		}
		if _, exists := seen[current]; exists {
			return fmt.Errorf("%w: existing parent chain contains a cycle", ErrValidation)
		}
		seen[current] = struct{}{}
		var kind NodeKind
		var next sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT kind, parent_id FROM infra_nodes WHERE id=?`, current).Scan(&kind, &next); err == sql.ErrNoRows {
			return fmt.Errorf("%w: parent node does not exist", ErrValidation)
		} else if err != nil {
			return fmt.Errorf("read infra parent chain: %w", err)
		}
		if current == parentID && !containerKind(kind) {
			return fmt.Errorf("%w: only sites, rooms, racks, and groups can contain nodes", ErrValidation)
		}
		current = ""
		if next.Valid {
			current = next.String
		}
	}
	return nil
}

func insertRegistryDevice(ctx context.Context, tx *sql.Tx, registryID string, node Node) error {
	tagsJSON, err := json.Marshal(node.Profile.Tags)
	if err != nil {
		return fmt.Errorf("encode manual inventory device tags: %w", err)
	}
	deviceType := node.Profile.Role
	if deviceType == "" {
		deviceType = node.Icon
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO devices
		(id, name, type, protocol, ip_address, port, username, vault_secret_id, credential_id, description, tags, mac_address)
		VALUES(?,?,?,?,?,0,'','','',?,?,?)`, registryID, node.Name, deviceType, inventory.ProtocolNone,
		node.Profile.IPAddress, node.Profile.Notes, string(tagsJSON), node.Profile.MACAddress)
	if err != nil {
		return fmt.Errorf("insert manual inventory device: %w", err)
	}
	return nil
}

func sourceObservedFromNode(node Node) SourceObserved {
	return SourceObserved{
		Kind: node.Kind, Name: node.Name, Icon: node.Icon, Role: node.Profile.Role,
		Vendor: node.Profile.Vendor, Model: node.Profile.Model, Serial: node.Profile.Serial,
		OS: node.Profile.OS, Firmware: node.Profile.Firmware, CPU: node.Profile.CPU,
		RAM: node.Profile.RAM, Storage: node.Profile.Storage, RackPosition: node.Profile.RackPosition,
		IPAddress: node.Profile.IPAddress, CIDR: node.Profile.CIDR, VLAN: node.Profile.VLAN,
		MACAddress: node.Profile.MACAddress,
		Tags:       cloneStrings(node.Profile.Tags), Interfaces: append([]NetworkInterface{}, node.Profile.Interfaces...),
	}
}

func normalizeProfile(profile *Profile) {
	if profile.Tags == nil {
		profile.Tags = []string{}
	}
	if profile.Interfaces == nil {
		profile.Interfaces = []NetworkInterface{}
	}
	if profile.Links == nil {
		profile.Links = []Link{}
	}
	if profile.AppBindings == nil {
		profile.AppBindings = []AppLink{}
	}
	if profile.Custom == nil {
		profile.Custom = map[string]string{}
	}
}
