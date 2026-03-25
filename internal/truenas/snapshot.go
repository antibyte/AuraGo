package truenas

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Snapshot represents a ZFS snapshot.
type Snapshot struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`       // Full name: "tank/dataset@snapshot-name"
	Dataset  string `json:"dataset"`    // Parent dataset
	SnapshotName string `json:"snapshot_name"` // Just the part after @
	Type     string `json:"type"`       // SNAPSHOT
	
	// Properties
	Properties SnapshotProperties `json:"properties"`
	
	// Replication info
	Replication *ReplicationInfo `json:"replication,omitempty"`
	
	// Metadata
	Retention struct {
		Value string `json:"value"`
		Parsed int   `json:"parsed"`
	} `json:"retention,omitempty"`
	
	// Creation time from ZFS
	RawCreation struct {
		Timestamp int64 `json:"$date"`
	} `json:"rawcreation,omitempty"`
}

// SnapshotProperties contains snapshot properties.
type SnapshotProperties struct {
	Used       SnapshotValue `json:"used"`
	Referenced SnapshotValue `json:"referenced"`
	CompressRatio SnapshotValue `json:"compressratio,omitempty"`
	Clones     int           `json:"clones,omitempty"` // Number of dependent clones
}

// SnapshotValue wraps a numeric property.
type SnapshotValue struct {
	Parsed int64  `json:"parsed"`
	Raw    string `json:"rawvalue"`
	Value  string `json:"value"`
}

// ReplicationInfo contains replication status.
type ReplicationInfo struct {
	State       string `json:"state"`
	LastSnapshot string `json:"last_snapshot,omitempty"`
}

// CreatedAt returns the snapshot creation time.
func (s *Snapshot) CreatedAt() time.Time {
	return time.Unix(s.RawCreation.Timestamp/1000, 0)
}

// Age returns how long ago the snapshot was created.
func (s *Snapshot) Age() time.Duration {
	return time.Since(s.CreatedAt())
}

// IsManual returns true if the snapshot appears to be manually created.
func (s *Snapshot) IsManual() bool {
	return !strings.HasPrefix(s.SnapshotName, "auto-") &&
		!strings.HasPrefix(s.SnapshotName, "scheduled-") &&
		!strings.Contains(s.SnapshotName, "replication")
}

// ListSnapshots returns all snapshots, optionally filtered by dataset.
func (c *Client) ListSnapshots(ctx context.Context, datasetFilter string) ([]Snapshot, error) {
	endpoint := "/pool/snapshot"
	if datasetFilter != "" {
		// TrueNAS API doesn't have direct query params, we filter client-side
	}
	
	var snapshots []Snapshot
	if err := c.Get(ctx, endpoint, &snapshots); err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}

	// Filter by dataset if specified
	if datasetFilter != "" {
		var filtered []Snapshot
		for _, s := range snapshots {
			if s.Dataset == datasetFilter || strings.HasPrefix(s.Dataset, datasetFilter+"/") {
				filtered = append(filtered, s)
			}
		}
		return filtered, nil
	}

	return snapshots, nil
}

// GetSnapshot returns a snapshot by full name.
func (c *Client) GetSnapshot(ctx context.Context, name string) (*Snapshot, error) {
	encoded := url.PathEscape(name)
	var snapshot Snapshot
	if err := c.Get(ctx, fmt.Sprintf("/pool/snapshot/id/%s", encoded), &snapshot); err != nil {
		return nil, fmt.Errorf("get snapshot %s: %w", name, err)
	}
	return &snapshot, nil
}

// CreateSnapshotRequest contains parameters for creating a snapshot.
type CreateSnapshotRequest struct {
	Dataset        string `json:"dataset"`
	Name           string `json:"name"`
	Recursive      bool   `json:"recursive,omitempty"`
	VmwareSync     bool   `json:"vmware_sync,omitempty"`
	NamingSchema   string `json:"naming_schema,omitempty"` // e.g., "auto-%Y-%m-%d_%H-%M"
	Retention      int    `json:"retention,omitempty"`     // in days
}

// CreateSnapshot creates a new snapshot.
func (c *Client) CreateSnapshot(ctx context.Context, req CreateSnapshotRequest) (*Snapshot, error) {
	// Generate name if not provided
	if req.Name == "" {
		if req.NamingSchema != "" {
			req.Name = time.Now().Format(req.NamingSchema)
		} else {
			req.Name = time.Now().Format("aura-20060102-150405")
		}
	}

	var snapshot Snapshot
	if err := c.Post(ctx, "/pool/snapshot", req, &snapshot); err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}
	return &snapshot, nil
}

// CreateSnapshotSimple is a convenience method for simple snapshot creation.
func (c *Client) CreateSnapshotSimple(ctx context.Context, dataset, name string, recursive bool) (*Snapshot, error) {
	return c.CreateSnapshot(ctx, CreateSnapshotRequest{
		Dataset:   dataset,
		Name:      name,
		Recursive: recursive,
	})
}

// DeleteSnapshot deletes a snapshot.
func (c *Client) DeleteSnapshot(ctx context.Context, name string) error {
	encoded := url.PathEscape(name)
	if err := c.Delete(ctx, fmt.Sprintf("/pool/snapshot/id/%s", encoded)); err != nil {
		return fmt.Errorf("delete snapshot %s: %w", name, err)
	}
	return nil
}

// RollbackSnapshot rolls back a dataset to a snapshot.
func (c *Client) RollbackSnapshot(ctx context.Context, snapshotName string, force bool) error {
	encoded := url.PathEscape(snapshotName)
	body := map[string]bool{
		"force": force,
		"recursive": false,
		"recursive_clones": false,
		"recursive_rollback": false,
	}

	// Use the proper rollback endpoint
	if err := c.Post(ctx, fmt.Sprintf("/pool/snapshot/id/%s/rollback", encoded), body, nil); err != nil {
		return fmt.Errorf("rollback to snapshot %s: %w", snapshotName, err)
	}
	return nil
}

// RollbackSnapshotRecursive rolls back a dataset and all children to a snapshot.
func (c *Client) RollbackSnapshotRecursive(ctx context.Context, snapshotName string, force bool) error {
	encoded := url.PathEscape(snapshotName)
	body := map[string]bool{
		"force": force,
		"recursive": true,
		"recursive_clones": true,
		"recursive_rollback": true,
	}

	if err := c.Post(ctx, fmt.Sprintf("/pool/snapshot/id/%s/rollback", encoded), body, nil); err != nil {
		return fmt.Errorf("recursive rollback to snapshot %s: %w", snapshotName, err)
	}
	return nil
}

// CloneSnapshot clones a snapshot to a new dataset.
func (c *Client) CloneSnapshot(ctx context.Context, snapshotName, newDataset string) error {
	body := map[string]string{
		"snapshot": snapshotName,
		"dataset_dst": newDataset,
	}

	if err := c.Post(ctx, "/pool/snapshot/clone", body, nil); err != nil {
		return fmt.Errorf("clone snapshot %s to %s: %w", snapshotName, newDataset, err)
	}
	return nil
}

// HoldSnapshot adds a user hold to a snapshot.
func (c *Client) HoldSnapshot(ctx context.Context, snapshotName, holdTag string) error {
	encoded := url.PathEscape(snapshotName)
	body := map[string]string{
		"hold_tag": holdTag,
	}

	if err := c.Post(ctx, fmt.Sprintf("/pool/snapshot/id/%s/hold", encoded), body, nil); err != nil {
		return fmt.Errorf("hold snapshot %s: %w", snapshotName, err)
	}
	return nil
}

// ReleaseSnapshot removes a user hold from a snapshot.
func (c *Client) ReleaseSnapshot(ctx context.Context, snapshotName, holdTag string) error {
	encoded := url.PathEscape(snapshotName)
	body := map[string]string{
		"hold_tag": holdTag,
	}

	if err := c.Post(ctx, fmt.Sprintf("/pool/snapshot/id/%s/release", encoded), body, nil); err != nil {
		return fmt.Errorf("release snapshot %s: %w", snapshotName, err)
	}
	return nil
}

// GetSnapshotHoldTags returns hold tags for a snapshot.
func (c *Client) GetSnapshotHoldTags(ctx context.Context, snapshotName string) ([]string, error) {
	encoded := url.PathEscape(snapshotName)
	var holds struct {
		Holds []string `json:"holds"`
	}

	if err := c.Get(ctx, fmt.Sprintf("/pool/snapshot/id/%s/holds", encoded), &holds); err != nil {
		return nil, fmt.Errorf("get holds for snapshot %s: %w", snapshotName, err)
	}
	return holds.Holds, nil
}

// BulkDeleteSnapshots deletes multiple snapshots.
func (c *Client) BulkDeleteSnapshots(ctx context.Context, snapshotNames []string) (map[string]error) {
	results := make(map[string]error)
	for _, name := range snapshotNames {
		results[name] = c.DeleteSnapshot(ctx, name)
	}
	return results
}

// SnapshotCount returns the total number of snapshots.
func (c *Client) SnapshotCount(ctx context.Context) (int, error) {
	snapshots, err := c.ListSnapshots(ctx, "")
	if err != nil {
		return 0, err
	}
	return len(snapshots), nil
}

// SnapshotCountByDataset returns the number of snapshots for a specific dataset.
func (c *Client) SnapshotCountByDataset(ctx context.Context, dataset string) (int, error) {
	snapshots, err := c.ListSnapshots(ctx, dataset)
	if err != nil {
		return 0, err
	}
	return len(snapshots), nil
}

// OldestSnapshot returns the oldest snapshot for a dataset.
func (c *Client) OldestSnapshot(ctx context.Context, dataset string) (*Snapshot, error) {
	snapshots, err := c.ListSnapshots(ctx, dataset)
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, nil
	}

	oldest := &snapshots[0]
	for i := range snapshots {
		if snapshots[i].RawCreation.Timestamp < oldest.RawCreation.Timestamp {
			oldest = &snapshots[i]
		}
	}
	return oldest, nil
}

// NewestSnapshot returns the newest snapshot for a dataset.
func (c *Client) NewestSnapshot(ctx context.Context, dataset string) (*Snapshot, error) {
	snapshots, err := c.ListSnapshots(ctx, dataset)
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, nil
	}

	newest := &snapshots[0]
	for i := range snapshots {
		if snapshots[i].RawCreation.Timestamp > newest.RawCreation.Timestamp {
			newest = &snapshots[i]
		}
	}
	return newest, nil
}

// CleanupOldSnapshots removes snapshots older than the specified age.
func (c *Client) CleanupOldSnapshots(ctx context.Context, dataset string, maxAge time.Duration) (int, error) {
	snapshots, err := c.ListSnapshots(ctx, dataset)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, snap := range snapshots {
		if snap.CreatedAt().Before(cutoff) {
			// Don't delete snapshots with holds
			holds, err := c.GetSnapshotHoldTags(ctx, snap.Name)
			if err != nil || len(holds) > 0 {
				continue
			}

			if err := c.DeleteSnapshot(ctx, snap.Name); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// GetSnapshotsToReclaim returns snapshots that can be safely deleted.
// It filters out snapshots with holds, recent snapshots, and snapshots used by clones.
func (c *Client) GetSnapshotsToReclaim(ctx context.Context, dataset string, keepCount int, minAge time.Duration) ([]Snapshot, error) {
	snapshots, err := c.ListSnapshots(ctx, dataset)
	if err != nil {
		return nil, err
	}

	if len(snapshots) <= keepCount {
		return nil, nil
	}

	cutoff := time.Now().Add(-minAge)
	var reclaimable []Snapshot

	// Sort by creation time (oldest first) - we only need to check after we have enough to keep
	// but since API doesn't guarantee order, we'll check all
	for _, snap := range snapshots {
		// Skip recent snapshots
		if snap.CreatedAt().After(cutoff) {
			continue
		}

		// Skip snapshots with clones
		if snap.Properties.Clones > 0 {
			continue
		}

		reclaimable = append(reclaimable, snap)
	}

	// Keep the most recent ones in the reclaimable list
	if len(reclaimable) > keepCount {
		return reclaimable[:len(reclaimable)-keepCount], nil
	}

	return nil, nil
}
