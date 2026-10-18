package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	result := Snapshot{Nodes: []Node{}, Edges: []Edge{}, Sources: []Source{}}
	rows, err := s.db.QueryContext(ctx, `SELECT id, version, kind, name, icon, registry_id, parent_id, profile_json, placements_json
		FROM infra_nodes ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list infra nodes: %w", err)
	}
	byID := make(map[string]int)
	for rows.Next() {
		var node Node
		var profileJSON, placementsJSON string
		var parentID sql.NullString
		if err := rows.Scan(&node.ID, &node.Version, &node.Kind, &node.Name, &node.Icon, &node.RegistryID, &parentID, &profileJSON, &placementsJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, fmt.Errorf("read infra node: %w", err)
		}
		if parentID.Valid {
			node.ParentID = parentID.String
		}
		if err := decodeNodeJSON(&node, profileJSON, placementsJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, err
		}
		byID[node.ID] = len(result.Nodes)
		result.Nodes = append(result.Nodes, node)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return Snapshot{}, fmt.Errorf("read infra nodes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Snapshot{}, fmt.Errorf("close infra node list: %w", err)
	}
	// Unlinked removed observations are retained only for the retention window and
	// carry no graph information; they stay out of the editor snapshot.
	rows, err = s.db.QueryContext(ctx, `SELECT node_id, kind, instance, source_key, name, state, fetched_at, removed_at, observed_json
		FROM infra_sources WHERE node_id IS NOT NULL OR state<>'removed' ORDER BY kind, instance, source_key`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list infra sources: %w", err)
	}
	for rows.Next() {
		var nodeID sql.NullString
		var source Source
		var fetchedAt, removedAt, observedJSON string
		if err := rows.Scan(&nodeID, &source.Kind, &source.Instance, &source.Key, &source.Name, &source.State, &fetchedAt, &removedAt, &observedJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, fmt.Errorf("read infra source: %w", err)
		}
		if err := decodeSource(&source, fetchedAt, removedAt, observedJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, err
		}
		result.Sources = append(result.Sources, source)
		if nodeID.Valid {
			if i, exists := byID[nodeID.String]; exists {
				result.Nodes[i].Sources = append(result.Nodes[i].Sources, source)
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return Snapshot{}, fmt.Errorf("read infra sources: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Snapshot{}, fmt.Errorf("close infra source list: %w", err)
	}
	result.Edges, err = s.listEdges(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return result, nil
}

func decodeNodeJSON(node *Node, profileJSON, placementsJSON string) error {
	if err := json.Unmarshal([]byte(profileJSON), &node.Profile); err != nil {
		return fmt.Errorf("decode infra node profile: %w", err)
	}
	if err := json.Unmarshal([]byte(placementsJSON), &node.Placements); err != nil {
		return fmt.Errorf("decode infra node placements: %w", err)
	}
	normalizeProfile(&node.Profile)
	if node.Placements == nil {
		node.Placements = Placements{}
	}
	normalizeNodeRuntime(node)
	node.Sources = []Source{}
	return nil
}

func decodeSource(source *Source, fetchedAt, removedAt, observedJSON string) error {
	if fetchedAt != "" {
		t, err := time.Parse(timeLayout, fetchedAt)
		if err != nil {
			return fmt.Errorf("parse infra source timestamp: %w", err)
		}
		source.FetchedAt = &t
	}
	if removedAt != "" {
		t, err := time.Parse(timeLayout, removedAt)
		if err != nil {
			return fmt.Errorf("parse infra source removal timestamp: %w", err)
		}
		source.RemovedAt = &t
	}
	if err := json.Unmarshal([]byte(observedJSON), &source.Observed); err != nil {
		return fmt.Errorf("decode infra source observation: %w", err)
	}
	normalizeObserved(&source.Observed)
	return nil
}
