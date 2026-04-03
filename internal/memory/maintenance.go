package memory

import (
	"database/sql"
	"fmt"
	"log/slog"
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

	var inserted int
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
			"source":      "inventory_sync",
		}

		nodeID := "dev_" + id
		if err := kg.AddNode(nodeID, name, props); err != nil {
			logger.Warn("Failed to sync inventory device to knowledge graph", "id", nodeID, "error", err)
		} else {
			inserted++
		}
	}

	logger.Info("External sources sync complete", "devices_synced", inserted)
	return nil
}
