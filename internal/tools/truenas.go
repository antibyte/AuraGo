package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/truenas"
)

const defaultTrueNASRequestTimeout = 60 * time.Second

func truenasRequestContext(cfg config.TrueNASConfig) (context.Context, context.CancelFunc) {
	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultTrueNASRequestTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

// TrueNASHealth returns system health information.
func TrueNASHealth(cfg config.TrueNASConfig, logger *slog.Logger) string {
	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	// Get system info
	info, err := client.Health(ctx)
	if err != nil {
		return errJSON("Failed to get system info: %v", err)
	}

	// Get pools
	pools, err := client.ListPools(ctx)
	if err != nil {
		logger.Error("Failed to list pools", "error", err)
	}

	// Get alerts
	alerts, err := client.ListAlerts(ctx)
	if err != nil {
		logger.Error("Failed to list alerts", "error", err)
	}

	// Determine overall status
	status := "healthy"
	for _, p := range pools {
		if p.Status != "ONLINE" {
			status = "degraded"
			break
		}
	}

	for _, a := range alerts {
		if a.Level == "CRITICAL" || a.Level == "ERROR" {
			status = "alert"
			break
		}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"health": map[string]interface{}{
			"status":      status,
			"version":     info.Version,
			"hostname":    info.Hostname,
			"uptime":      info.Uptime,
			"model":       info.Model,
			"cores":       info.Cores,
			"memory_gb":   info.PhysMem / (1024 * 1024 * 1024),
			"pools":       pools,
			"pool_count":  len(pools),
			"alerts":      alerts,
			"alert_count": len(alerts),
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	})
	return string(result)
}

// TrueNASPoolList returns all ZFS pools.
func TrueNASPoolList(cfg config.TrueNASConfig, db *sql.DB, logger *slog.Logger) string {
	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	pools, err := client.ListPools(ctx)
	if err != nil {
		return errJSON("Failed to list pools: %v", err)
	}

	// Enrich with usage percentage
	type PoolInfo struct {
		truenas.Pool
		UsagePercent float64 `json:"usage_percent"`
	}

	var poolInfos []PoolInfo
	for _, p := range pools {
		usage := float64(0)
		if p.Size.Total > 0 {
			usage = float64(p.Size.Allocated) / float64(p.Size.Total) * 100
		}
		poolInfos = append(poolInfos, PoolInfo{
			Pool:         p,
			UsagePercent: usage,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "pools": poolInfos})
	return string(result)
}

// TrueNASPoolScrub starts a scrub operation on a pool.
func TrueNASPoolScrub(cfg config.TrueNASConfig, poolID int64, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("Pool scrubbing is disabled (readonly mode)")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	if err := client.ScrubPool(ctx, poolID); err != nil {
		return errJSON("Failed to start scrub: %v", err)
	}

	return okJSON("message", "Scrub started successfully")
}

// TrueNASDatasetList returns all datasets, optionally filtered by pool.
func TrueNASDatasetList(cfg config.TrueNASConfig, pool string, logger *slog.Logger) string {
	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()
	var datasets []truenas.Dataset

	if pool != "" {
		datasets, err = client.ListDatasetsByPool(ctx, pool)
	} else {
		datasets, err = client.ListDatasets(ctx)
	}

	if err != nil {
		return errJSON("Failed to list datasets: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "datasets": datasets})
	return string(result)
}

// TrueNASDatasetCreate creates a new dataset.
func TrueNASDatasetCreate(cfg config.TrueNASConfig, name, compression string, quotaGB int64, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("Dataset creation is disabled (readonly mode)")
	}

	// Security: validate dataset name
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return errJSON("Invalid dataset name: path traversal detected")
	}

	// Validate name format (must include pool)
	if !strings.Contains(name, "/") {
		return errJSON("Invalid dataset name: must include pool prefix (e.g., 'tank/share')")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	req := truenas.CreateDatasetRequest{
		Name:        name,
		Type:        "FILESYSTEM",
		Compression: compression,
	}

	if quotaGB > 0 {
		req.Quota = quotaGB * 1024 * 1024 * 1024 // Convert GB to bytes
	}

	// Apply defaults from config
	if compression == "" {
		compression = cfg.DefaultShares.Compression
		if compression == "" {
			compression = "lz4"
		}
		req.Compression = compression
	}

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	dataset, err := client.CreateDataset(ctx, req)
	if err != nil {
		return errJSON("Failed to create dataset: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "dataset": dataset})
	return string(result)
}

// TrueNASDatasetDelete deletes a dataset.
func TrueNASDatasetDelete(cfg config.TrueNASConfig, name string, recursive bool, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("Dataset deletion is disabled (readonly mode)")
	}
	if !cfg.AllowDestructive {
		return errJSON("Dataset deletion is disabled (allow_destructive: false)")
	}

	// Security validation
	if strings.Contains(name, "..") {
		return errJSON("Invalid dataset name: path traversal detected")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	if err := client.DeleteDataset(ctx, name, recursive); err != nil {
		return errJSON("Failed to delete dataset: %v", err)
	}

	return okJSON("message", fmt.Sprintf("Dataset %s deleted successfully", name))
}

// TrueNASSnapshotList returns snapshots, optionally filtered by dataset.
func TrueNASSnapshotList(cfg config.TrueNASConfig, dataset string, logger *slog.Logger) string {
	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	snapshots, err := client.ListSnapshots(ctx, dataset)
	if err != nil {
		return errJSON("Failed to list snapshots: %v", err)
	}

	// Add age information
	type SnapshotInfo struct {
		truenas.Snapshot
		Age      string `json:"age"`
		AgeHours int    `json:"age_hours"`
		IsManual bool   `json:"is_manual"`
	}

	var infos []SnapshotInfo
	for _, s := range snapshots {
		age := s.Age()
		infos = append(infos, SnapshotInfo{
			Snapshot: s,
			Age:      formatDuration(age),
			AgeHours: int(age.Hours()),
			IsManual: s.IsManual(),
		})
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "snapshots": infos})
	return string(result)
}

// TrueNASSnapshotCreate creates a new snapshot.
func TrueNASSnapshotCreate(cfg config.TrueNASConfig, dataset, name string, recursive bool, retentionDays int, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("Snapshot creation is disabled (readonly mode)")
	}

	if strings.Contains(dataset, "..") {
		return errJSON("Invalid dataset name: path traversal detected")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	// Auto-generate name if not provided
	if name == "" {
		name = time.Now().Format("aura-20060102-150405")
	}

	// Apply default retention if not specified
	if retentionDays == 0 && cfg.SnapshotRetention.Enabled {
		retentionDays = cfg.SnapshotRetention.DefaultDays
	}

	req := truenas.CreateSnapshotRequest{
		Dataset:   dataset,
		Name:      name,
		Recursive: recursive,
		Retention: retentionDays,
	}

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	snapshot, err := client.CreateSnapshot(ctx, req)
	if err != nil {
		return errJSON("Failed to create snapshot: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "snapshot": snapshot})
	return string(result)
}

// TrueNASSnapshotDelete deletes a snapshot.
func TrueNASSnapshotDelete(cfg config.TrueNASConfig, name string, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("Snapshot deletion is disabled (readonly mode)")
	}
	if !cfg.AllowDestructive {
		return errJSON("Snapshot deletion is disabled (allow_destructive: false)")
	}

	if strings.Contains(name, "..") {
		return errJSON("Invalid snapshot name: path traversal detected")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	if err := client.DeleteSnapshot(ctx, name); err != nil {
		return errJSON("Failed to delete snapshot: %v", err)
	}

	return okJSON("message", fmt.Sprintf("Snapshot %s deleted successfully", name))
}

// TrueNASSnapshotRollback rolls back to a snapshot.
func TrueNASSnapshotRollback(cfg config.TrueNASConfig, name string, force bool, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("Snapshot rollback is disabled (readonly mode)")
	}
	if !cfg.AllowDestructive {
		return errJSON("Snapshot rollback is disabled (allow_destructive: false)")
	}

	if strings.Contains(name, "..") {
		return errJSON("Invalid snapshot name: path traversal detected")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	if err := client.RollbackSnapshot(ctx, name, force); err != nil {
		return errJSON("Failed to rollback snapshot: %v", err)
	}

	return okJSON("message", fmt.Sprintf("Rolled back to snapshot %s", name))
}

// TrueNASSMBList returns all SMB shares.
func TrueNASSMBList(cfg config.TrueNASConfig, logger *slog.Logger) string {
	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	shares, err := client.ListSMBShares(ctx)
	if err != nil {
		return errJSON("Failed to list SMB shares: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "shares": shares})
	return string(result)
}

// TrueNASSMBCreate creates an SMB share.
func TrueNASSMBCreate(cfg config.TrueNASConfig, name, path string, guestOK, timemachine bool, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("SMB share creation is disabled (readonly mode)")
	}

	if strings.Contains(path, "..") {
		return errJSON("Invalid path: path traversal detected")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	req := truenas.CreateSMBShareRequest{
		Name:        name,
		Path:        path,
		Enabled:     true,
		GuestOK:     guestOK,
		Timemachine: timemachine,
		Browseable:  true,
		ShadowCopy:  true,
		RecycleBin:  true,
	}

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	share, err := client.CreateSMBShare(ctx, req)
	if err != nil {
		return errJSON("Failed to create SMB share: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "share": share})
	return string(result)
}

// TrueNASSMBDelete deletes an SMB share.
func TrueNASSMBDelete(cfg config.TrueNASConfig, shareID int64, logger *slog.Logger) string {
	if cfg.ReadOnly {
		return errJSON("SMB share deletion is disabled (readonly mode)")
	}

	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	if err := client.DeleteSMBShare(ctx, shareID); err != nil {
		return errJSON("Failed to delete SMB share: %v", err)
	}

	return okJSON("message", "SMB share deleted successfully")
}

// TrueNASFSSpace returns space usage information.
func TrueNASFSSpace(cfg config.TrueNASConfig, dataset string, logger *slog.Logger) string {
	client, err := truenas.NewClient(cfg, nil)
	if err != nil {
		return errJSON("TrueNAS connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := truenasRequestContext(cfg)
	defer cancel()

	datasets, err := client.ListDatasets(ctx)
	if err != nil {
		return errJSON("Failed to list datasets: %v", err)
	}

	type SpaceInfo struct {
		Name         string  `json:"name"`
		Used         int64   `json:"used_bytes"`
		Available    int64   `json:"available_bytes"`
		Total        int64   `json:"total_bytes"`
		UsedGB       float64 `json:"used_gb"`
		AvailableGB  float64 `json:"available_gb"`
		UsagePercent float64 `json:"usage_percent"`
	}

	var spaces []SpaceInfo
	for _, d := range datasets {
		if dataset != "" && d.Name != dataset && !strings.HasPrefix(d.Name, dataset+"/") {
			continue
		}

		total := d.Used.Parsed + d.Available.Parsed
		usage := float64(0)
		if total > 0 {
			usage = float64(d.Used.Parsed) / float64(total) * 100
		}

		spaces = append(spaces, SpaceInfo{
			Name:         d.Name,
			Used:         d.Used.Parsed,
			Available:    d.Available.Parsed,
			Total:        total,
			UsedGB:       float64(d.Used.Parsed) / (1024 * 1024 * 1024),
			AvailableGB:  float64(d.Available.Parsed) / (1024 * 1024 * 1024),
			UsagePercent: usage,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{"status": "ok", "space": spaces})
	return string(result)
}

// DispatchTrueNASTool routes TrueNAS tool calls.
func DispatchTrueNASTool(name string, params map[string]string, cfg *config.Config, db *sql.DB, logger *slog.Logger) string {
	if !cfg.TrueNAS.Enabled {
		return errJSON("TrueNAS integration is disabled")
	}

	switch name {
	case "truenas_health":
		return TrueNASHealth(cfg.TrueNAS, logger)

	case "truenas_pool_list":
		return TrueNASPoolList(cfg.TrueNAS, db, logger)

	case "truenas_pool_scrub":
		poolID := getInt64(params, "pool_id")
		return TrueNASPoolScrub(cfg.TrueNAS, poolID, logger)

	case "truenas_dataset_list":
		pool := getString(params, "pool", "")
		return TrueNASDatasetList(cfg.TrueNAS, pool, logger)

	case "truenas_dataset_create":
		name := getString(params, "name")
		compression := getString(params, "compression", "lz4")
		quotaGB := getInt64(params, "quota_gb", 0)
		return TrueNASDatasetCreate(cfg.TrueNAS, name, compression, quotaGB, logger)

	case "truenas_dataset_delete":
		name := getString(params, "name")
		recursive := getBool(params, "recursive", false)
		return TrueNASDatasetDelete(cfg.TrueNAS, name, recursive, logger)

	case "truenas_snapshot_list":
		dataset := getString(params, "dataset", "")
		return TrueNASSnapshotList(cfg.TrueNAS, dataset, logger)

	case "truenas_snapshot_create":
		dataset := getString(params, "dataset")
		name := getString(params, "name", "")
		recursive := getBool(params, "recursive", false)
		retention := getInt(params, "retention_days", 0)
		return TrueNASSnapshotCreate(cfg.TrueNAS, dataset, name, recursive, retention, logger)

	case "truenas_snapshot_delete":
		name := getString(params, "name")
		return TrueNASSnapshotDelete(cfg.TrueNAS, name, logger)

	case "truenas_snapshot_rollback":
		name := getString(params, "name")
		force := getBool(params, "force", false)
		return TrueNASSnapshotRollback(cfg.TrueNAS, name, force, logger)

	case "truenas_smb_list":
		return TrueNASSMBList(cfg.TrueNAS, logger)

	case "truenas_smb_create":
		name := getString(params, "name")
		path := getString(params, "path")
		guestOK := getBool(params, "guest_ok", false)
		timemachine := getBool(params, "timemachine", false)
		return TrueNASSMBCreate(cfg.TrueNAS, name, path, guestOK, timemachine, logger)

	case "truenas_smb_delete":
		shareID := getInt64(params, "share_id")
		return TrueNASSMBDelete(cfg.TrueNAS, shareID, logger)

	case "truenas_fs_space":
		dataset := getString(params, "dataset", "")
		return TrueNASFSSpace(cfg.TrueNAS, dataset, logger)

	default:
		return errJSON("Unknown TrueNAS tool: %s", name)
	}
}

// Helper functions
func formatDuration(d time.Duration) string {
	if d.Hours() < 1 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d.Hours() < 24 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	return fmt.Sprintf("%dy", months/12)
}

func getString(params map[string]string, key string, defaultValue ...string) string {
	if v, ok := params[key]; ok {
		return v
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func getInt64(params map[string]string, key string, defaultValue ...int64) int64 {
	if v, ok := params[key]; ok {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

func getInt(params map[string]string, key string, defaultValue ...int) int {
	if v, ok := params[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

func getBool(params map[string]string, key string, defaultValue ...bool) bool {
	if v, ok := params[key]; ok {
		return v == "true" || v == "1" || v == "yes"
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return false
}
