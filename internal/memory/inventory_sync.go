package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const (
	inventoryDeviceNodePrefix = "dev_"
	inventorySyncSource       = "inventory_sync"
	inventoryDeviceSyncLimit  = 10000
)

// SyncExternalSources pulls container and SSH device data from Inventory DB
// into the Knowledge Graph (using simple nodes/edges).
func (kg *KnowledgeGraph) SyncExternalSources(inventoryDB *sql.DB, logger *slog.Logger) error {
	if logger != nil {
		logger.Info("Starting sync of external sources into knowledge graph...")
	}
	if inventoryDB == nil {
		return fmt.Errorf("inventory database is nil")
	}

	rows, err := inventoryDB.Query(`SELECT id, name, type, ip_address, description, tags FROM devices`)
	if err != nil {
		return fmt.Errorf("query inventory devices: %w", err)
	}
	defer rows.Close()

	expected := make(map[string]struct{})
	type inventoryDevice struct {
		nodeID string
		name   string
		props  map[string]string
	}
	devices := make([]inventoryDevice, 0)
	for rows.Next() {
		var id, name, devType, ip, desc, tags string
		if err := rows.Scan(&id, &name, &devType, &ip, &desc, &tags); err != nil {
			if logger != nil {
				logger.Warn("Failed to scan device row", "error", err)
			}
			continue
		}

		props := map[string]string{
			"type":        "device",
			"device_type": devType,
			"ip":          ip,
			"description": desc,
			"tags":        tags,
			"source":      inventorySyncSource,
		}

		nodeID := inventoryDeviceNodePrefix + id
		expected[nodeID] = struct{}{}
		devices = append(devices, inventoryDevice{nodeID: nodeID, name: name, props: props})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate inventory devices: %w", err)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin inventory KG sync transaction: %w", err)
	}
	defer tx.Rollback()

	upsertedNodes := make([]Node, 0, len(devices))
	for _, device := range devices {
		node, err := kg.upsertKnowledgeGraphNodeTx(tx, device.nodeID, device.name, device.props)
		if err != nil {
			return fmt.Errorf("sync inventory device %s to knowledge graph: %w", device.nodeID, err)
		}
		upsertedNodes = append(upsertedNodes, node)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit inventory KG sync transaction: %w", err)
	}
	for _, node := range upsertedNodes {
		kg.upsertSemanticNodeIndex(node)
	}

	removed := kg.removeStaleInventoryDeviceNodes(expected, logger)

	if logger != nil {
		logger.Info("External sources sync complete", "devices_synced", len(upsertedNodes), "stale_devices_removed", removed)
	}
	return nil
}

func (kg *KnowledgeGraph) upsertKnowledgeGraphNodeTx(tx *sql.Tx, id, label string, properties map[string]string) (Node, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Node{}, fmt.Errorf("node id is required")
	}
	isProtected := strings.EqualFold(strings.TrimSpace(properties["protected"]), "true")
	properties = sanitizeKnowledgeGraphNodeProperties(properties, isProtected)
	label = strings.TrimSpace(label)

	existingLabel, existingProps, existingProtected, _, err := loadKnowledgeGraphNode(tx, id)
	if err != nil {
		return Node{}, fmt.Errorf("load existing node %s: %w", id, err)
	}

	finalLabel := mergeKnowledgeGraphLabel(existingLabel, label)
	finalProps := mergeKnowledgeGraphPropertiesOverwrite(existingProps, properties)
	isProtectedFinal := existingProtected
	if finalProps["protected"] == "true" {
		isProtectedFinal = 1
	}
	finalProps = sanitizeKnowledgeGraphNodeProperties(finalProps, isProtectedFinal != 0)

	propsJSON, err := json.Marshal(finalProps)
	if err != nil {
		return Node{}, fmt.Errorf("marshal node properties: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO kg_nodes (id, label, properties, protected, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			label = excluded.label,
			properties = excluded.properties,
			protected = excluded.protected,
			updated_at = CURRENT_TIMESTAMP
	`, id, finalLabel, string(propsJSON), isProtectedFinal); err != nil {
		return Node{}, fmt.Errorf("upsert node %s: %w", id, err)
	}
	return Node{ID: id, Label: finalLabel, Properties: finalProps, Protected: isProtectedFinal != 0}, nil
}

func (kg *KnowledgeGraph) removeStaleInventoryDeviceNodes(expected map[string]struct{}, logger *slog.Logger) int {
	if kg == nil {
		return 0
	}
	nodes, err := kg.ListNodesByIDPrefix(inventoryDeviceNodePrefix, inventoryDeviceSyncLimit)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to list inventory KG nodes for stale cleanup", "error", err)
		}
		return 0
	}

	removed := 0
	for _, node := range nodes {
		if node.Properties["source"] != inventorySyncSource {
			continue
		}
		if _, ok := expected[node.ID]; ok {
			continue
		}
		if node.Protected {
			if logger != nil {
				logger.Debug("Skipping protected stale inventory device node", "node_id", node.ID)
			}
			continue
		}
		if err := kg.DeleteNode(node.ID); err != nil {
			if logger != nil {
				logger.Warn("Failed to delete stale inventory device node", "node_id", node.ID, "error", err)
			}
			continue
		}
		removed++
	}
	return removed
}
