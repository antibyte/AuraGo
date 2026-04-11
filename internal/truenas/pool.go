package truenas

import (
	"context"
	"fmt"
	"net/http"
)

// Pool represents a ZFS storage pool.
type Pool struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	GUID     string   `json:"guid"`
	Path     string   `json:"path"`
	Status   string   `json:"status"` // ONLINE, DEGRADED, FAULTED, etc.
	Healthy  bool     `json:"healthy"`
	Warning  bool     `json:"warning"`
	Scan     Scan     `json:"scan"`
	Topology Topology `json:"topology"`
	Size     PoolSize `json:"size"`
}

// Scan represents pool scrub/resilver status.
type Scan struct {
	Function       string  `json:"function"` // SCRUB, RESILVER
	State          string  `json:"state"`    // SCANNING, FINISHED, CANCELED
	StartTime      string  `json:"start_time"`
	EndTime        string  `json:"end_time"`
	Percentage     float64 `json:"percentage"`
	BytesToProcess int64   `json:"bytes_to_process"`
	BytesProcessed int64   `json:"bytes_processed"`
	Errors         int     `json:"errors"`
}

// Topology represents pool device topology.
type Topology struct {
	Data    []VDev `json:"data"`
	Cache   []VDev `json:"cache,omitempty"`
	Log     []VDev `json:"log,omitempty"`
	Spare   []VDev `json:"spare,omitempty"`
	Special []VDev `json:"special,omitempty"` // Metadata vdevs (SCALE)
	Dedup   []VDev `json:"dedup,omitempty"`   // Dedup vdevs (SCALE 23.10+)
}

// VDev represents a virtual device.
type VDev struct {
	Type     string    `json:"type"` // RAIDZ1, RAIDZ2, MIRROR, STRIPE, etc.
	Name     string    `json:"name"`
	Path     string    `json:"path,omitempty"`
	Status   string    `json:"status"` // ONLINE, DEGRADED, UNAVAIL, etc.
	GUID     string    `json:"guid"`
	Children []VDev    `json:"children,omitempty"`
	Disk     string    `json:"disk,omitempty"` // Underlying disk name
	Stats    VDevStats `json:"stats,omitempty"`
}

// VDevStats contains device statistics.
type VDevStats struct {
	ReadErrors     int64 `json:"read_errors"`
	WriteErrors    int64 `json:"write_errors"`
	ChecksumErrors int64 `json:"checksum_errors"`
}

// PoolSize represents pool capacity.
type PoolSize struct {
	Allocated int64 `json:"allocated"`
	Free      int64 `json:"free"`
	Total     int64 `json:"total"`
}

// Usage returns the percentage of used space.
func (p *Pool) Usage() float64 {
	if p.Size.Total == 0 {
		return 0
	}
	return float64(p.Size.Allocated) / float64(p.Size.Total) * 100
}

// ListPools returns all ZFS pools.
func (c *Client) ListPools(ctx context.Context) ([]Pool, error) {
	var pools []Pool
	if err := c.Get(ctx, "/pool", &pools); err != nil {
		return nil, fmt.Errorf("list pools: %w", err)
	}
	return pools, nil
}

// GetPool returns a pool by ID.
func (c *Client) GetPool(ctx context.Context, id int64) (*Pool, error) {
	var pool Pool
	if err := c.Get(ctx, fmt.Sprintf("/pool/id/%d", id), &pool); err != nil {
		return nil, fmt.Errorf("get pool %d: %w", id, err)
	}
	return &pool, nil
}

// GetPoolByName returns a pool by name.
func (c *Client) GetPoolByName(ctx context.Context, name string) (*Pool, error) {
	pools, err := c.ListPools(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range pools {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("pool %q not found", name)
}

// ScrubPool starts a scrub on a pool.
func (c *Client) ScrubPool(ctx context.Context, poolID int64) error {
	body := map[string]interface{}{
		"id": poolID,
	}
	if err := c.Post(ctx, "/pool/scrub", body, nil); err != nil {
		return fmt.Errorf("start scrub on pool %d: %w", poolID, err)
	}
	return nil
}

// ResilverPool starts a resilver on a pool.
func (c *Client) ResilverPool(ctx context.Context, poolID int64) error {
	body := map[string]interface{}{
		"id": poolID,
	}
	if err := c.Post(ctx, "/pool/resilver", body, nil); err != nil {
		return fmt.Errorf("start resilver on pool %d: %w", poolID, err)
	}
	return nil
}

// UpgradePool upgrades a pool to the latest ZFS version.
func (c *Client) UpgradePool(ctx context.Context, poolID int64) error {
	if err := c.Post(ctx, fmt.Sprintf("/pool/id/%d/upgrade", poolID), nil, nil); err != nil {
		return fmt.Errorf("upgrade pool %d: %w", poolID, err)
	}
	return nil
}

// GetPoolDisks returns disks in a pool.
func (c *Client) GetPoolDisks(ctx context.Context, poolID int64) ([]Disk, error) {
	var disks []Disk
	if err := c.Get(ctx, fmt.Sprintf("/pool/id/%d/get_disks", poolID), &disks); err != nil {
		return nil, fmt.Errorf("get pool %d disks: %w", poolID, err)
	}
	return disks, nil
}

// Disk represents a physical disk.
type Disk struct {
	Name         string `json:"name"`
	Serial       string `json:"serial"`
	Model        string `json:"model"`
	Size         int64  `json:"size"`
	Type         string `json:"type"` // HDD, SSD, NVME
	Pool         string `json:"pool,omitempty"`
	Description  string `json:"description"`
	TransferMode string `json:"transfermode"`
	HMSEnabled   bool   `json:"hmr_enabled"`
	RotationRate int    `json:"rotationrate"` // 0 for SSDs
}

// IsSSD returns true if the disk is an SSD.
func (d *Disk) IsSSD() bool {
	return d.Type == "SSD" || d.Type == "NVME" || d.RotationRate == 0
}

// CreatePoolRequest contains parameters for creating a new pool.
type CreatePoolRequest struct {
	Name       string       `json:"name"`
	Encryption bool         `json:"encryption,omitempty"`
	Topology   PoolTopology `json:"topology"`
	Options    PoolOptions  `json:"options,omitempty"`
}

// PoolTopology defines the pool layout.
type PoolTopology struct {
	Data  []PoolVDev `json:"data"`
	Cache []PoolVDev `json:"cache,omitempty"`
	Log   []PoolVDev `json:"log,omitempty"`
	Spare []PoolVDev `json:"spare,omitempty"`
}

// PoolVDev represents a vdev configuration.
type PoolVDev struct {
	Type  string   `json:"type"` // RAIDZ1, RAIDZ2, RAIDZ3, MIRROR, STRIPE
	Disks []string `json:"disks"`
}

// PoolOptions contains optional pool settings.
type PoolOptions struct {
	Force bool `json:"force,omitempty"`
}

// CreatePool creates a new ZFS pool.
func (c *Client) CreatePool(ctx context.Context, req CreatePoolRequest) (*Pool, error) {
	var pool Pool
	if err := c.Post(ctx, "/pool", req, &pool); err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &pool, nil
}

// UpdatePool updates pool properties.
func (c *Client) UpdatePool(ctx context.Context, poolID int64, autotrim bool) error {
	body := map[string]interface{}{
		"autotrim": autotrim,
	}
	if err := c.Put(ctx, fmt.Sprintf("/pool/id/%d", poolID), body); err != nil {
		return fmt.Errorf("update pool %d: %w", poolID, err)
	}
	return nil
}

// DeletePool destroys a pool (DANGEROUS).
func (c *Client) DeletePool(ctx context.Context, poolID int64, confirm bool) error {
	if !confirm {
		return fmt.Errorf("pool deletion requires explicit confirmation")
	}

	// TrueNAS requires a specific confirmation body for pool deletion
	body := map[string]interface{}{
		"cascade": true,
		"confirm": true,
	}

	resp, err := c.request(ctx, http.MethodDelete, fmt.Sprintf("/pool/id/%d", poolID), body)
	if err != nil {
		return fmt.Errorf("delete pool %d: %w", poolID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete pool %d: API returned %d", poolID, resp.StatusCode)
	}

	return nil
}

// Import pool (for already created pools)
func (c *Client) FindPoolsToImport(ctx context.Context) ([]ImportablePool, error) {
	var pools []ImportablePool
	if err := c.Get(ctx, "/pool/import_find", &pools); err != nil {
		return nil, fmt.Errorf("find importable pools: %w", err)
	}
	return pools, nil
}

// ImportablePool represents a pool that can be imported.
type ImportablePool struct {
	Name     string   `json:"name"`
	GUID     string   `json:"guid"`
	Status   string   `json:"status"`
	Topology Topology `json:"topology"`
}

// ImportPool imports an existing pool.
func (c *Client) ImportPool(ctx context.Context, guidOrName string, enableEncryption bool) (*Pool, error) {
	body := map[string]interface{}{
		"guid":              guidOrName,
		"enable_encryption": enableEncryption,
	}

	var pool Pool
	if err := c.Post(ctx, "/pool/import_pool", body, &pool); err != nil {
		return nil, fmt.Errorf("import pool %s: %w", guidOrName, err)
	}
	return &pool, nil
}
