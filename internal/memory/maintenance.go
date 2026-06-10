package memory

import (
	"database/sql"
	"fmt"
	"log/slog"
)

const (
	inventoryDeviceNodePrefix = "dev_"
	inventorySyncSource       = "inventory_sync"
	inventoryDeviceSyncLimit  = 10000
)

// SyncExternalSources pulls container and SSH device data from Inventory DB
// into the Knowledge Graph (using simple nodes/edges).
func (kg *KnowledgeGraph) SyncExternalSources(inventoryDB *sql.DB, logger *slog.Logger) error {
	logger.Info("Starting sync of external sources into knowledge graph...")
	if inventoryDB == nil {
		return fmt.Errorf("inventory database is nil")
	}

	rows, err := inventoryDB.Query(`SELECT id, name, type, ip_address, description, tags FROM devices`)
	if err != nil {
		return fmt.Errorf("query inventory devices: %w", err)
	}
	defer rows.Close()

	expected := make(map[string]struct{})
	var synced int
	for rows.Next() {
		var id, name, devType, ip, desc, tags string
		if err := rows.Scan(&id, &name, &devType, &ip, &desc, &tags); err != nil {
			logger.Warn("Failed to scan device row", "error", err)
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
		if err := kg.AddNode(nodeID, name, props); err != nil {
			logger.Warn("Failed to sync inventory device to knowledge graph", "id", nodeID, "error", err)
		} else {
			synced++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate inventory devices: %w", err)
	}

	removed := kg.removeStaleInventoryDeviceNodes(expected, logger)

	logger.Info("External sources sync complete", "devices_synced", synced, "stale_devices_removed", removed)
	return nil
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
