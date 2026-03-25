package truenas

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Dataset represents a ZFS dataset or zvol.
type Dataset struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`       // FILESYSTEM, VOLUME
	Name       string `json:"name"`       // e.g., "tank/share" or "tank/share@snapshot"
	Pool       string `json:"pool"`
	Mountpoint string `json:"mountpoint"`
	Encrypted  bool   `json:"encrypted"`
	EncryptionRoot string `json:"encryption_root,omitempty"`
	KeyLoaded  bool   `json:"key_loaded"`

	// Capacity
	Available DatasetValue `json:"available"`
	Used      DatasetValue `json:"used"`
	UsedByDataset DatasetValue `json:"usedbydataset"`
	UsedBySnapshots DatasetValue `json:"usedbysnapshots"`
	UsedByRefreservation DatasetValue `json:"usedbyrefreservation"`
	UsedByChildren DatasetValue `json:"usedbychildren"`
	LogicalUsed DatasetValue `json:"logicalused,omitempty"`

	// Properties
	Compression DatasetStringValue `json:"compression"`
	Quota       DatasetValue `json:"quota"`
	Refquota    DatasetValue `json:"refquota"`
	Reservation DatasetValue `json:"reservation"`
	Refreservation DatasetValue `json:"refreservation"`
	Copies      int          `json:"copies"`
	Readonly    DatasetStringValue `json:"readonly"`
	Exec        DatasetStringValue `json:"exec"`
	Atime       DatasetStringValue `json:"atime"`
	Snapdir     DatasetStringValue `json:"snapdir"`
	Aclmode     DatasetStringValue `json:"aclmode,omitempty"`
	Acltype     DatasetStringValue `json:"acltype,omitempty"`
	Xattr       DatasetStringValue `json:"xattr"`
	Casesensitive DatasetStringValue `json:"casesensitivity,omitempty"`
	Deduplication DatasetStringValue `json:"deduplication,omitempty"`
	Checksum    DatasetStringValue `json:"checksum,omitempty"`
	Sync        DatasetStringValue `json:"sync,omitempty"`
	ShareType   string       `json:"share_type,omitempty"`
	Comments    DatasetStringValue `json:"comments,omitempty"`
	SpecialSmallBlockSize DatasetValue `json:"special_small_block_size,omitempty"`
	
	// Creation time
	Creation struct {
		Timestamp int64 `json:"$date"`
	} `json:"creation"`
}

// DatasetValue wraps a numeric ZFS property.
type DatasetValue struct {
	Parsed  int64  `json:"parsed"`
	Raw     string `json:"rawvalue"`
	Value   string `json:"value"`
}

// DatasetStringValue wraps a string ZFS property.
type DatasetStringValue struct {
	Parsed  string `json:"parsed"`
	Raw     string `json:"rawvalue"`
	Value   string `json:"value"`
}

// IsFilesystem returns true if the dataset is a filesystem.
func (d *Dataset) IsFilesystem() bool {
	return d.Type == "FILESYSTEM"
}

// IsVolume returns true if the dataset is a zvol.
func (d *Dataset) IsVolume() bool {
	return d.Type == "VOLUME"
}

// IsSnapshot returns true if the dataset name contains @.
func (d *Dataset) IsSnapshot() bool {
	return strings.Contains(d.Name, "@")
}

// Parent returns the parent dataset name.
func (d *Dataset) Parent() string {
	parts := strings.Split(d.Name, "/")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

// ListDatasets returns all datasets.
func (c *Client) ListDatasets(ctx context.Context) ([]Dataset, error) {
	var datasets []Dataset
	if err := c.Get(ctx, "/pool/dataset", &datasets); err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	return datasets, nil
}

// ListDatasetsByPool returns datasets filtered by pool.
func (c *Client) ListDatasetsByPool(ctx context.Context, poolName string) ([]Dataset, error) {
	all, err := c.ListDatasets(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []Dataset
	for _, d := range all {
		if d.Pool == poolName || strings.HasPrefix(d.Name, poolName+"/") {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

// GetDataset returns a dataset by name.
func (c *Client) GetDataset(ctx context.Context, name string) (*Dataset, error) {
	// URL encode the dataset name as it may contain special characters
	encoded := url.PathEscape(name)
	var dataset Dataset
	if err := c.Get(ctx, fmt.Sprintf("/pool/dataset/id/%s", encoded), &dataset); err != nil {
		return nil, fmt.Errorf("get dataset %s: %w", name, err)
	}
	return &dataset, nil
}

// CreateDatasetRequest contains parameters for creating a dataset.
type CreateDatasetRequest struct {
	Name        string                 `json:"name"`                  // Full path: "pool/dataset"
	Type        string                 `json:"type"`                  // FILESYSTEM (default), VOLUME
	Comments    string                 `json:"comments,omitempty"`
	Compression string                 `json:"compression,omitempty"` // lz4, gzip, zle, off
	Atime       string                 `json:"atime,omitempty"`       // on, off
	Exec        string                 `json:"exec,omitempty"`        // on, off
	Readonly    string                 `json:"readonly,omitempty"`    // on, off
	Quota       int64                  `json:"quota,omitempty"`       // in bytes
	Refquota    int64                  `json:"refquota,omitempty"`    // in bytes
	Reservation int64                  `json:"reservation,omitempty"` // in bytes
	Refreservation int64               `json:"refreservation,omitempty"` // in bytes
	Copies      int                    `json:"copies,omitempty"`      // 1, 2, 3
	Snapdir     string                 `json:"snapdir,omitempty"`     // hidden, visible
	SharesMB    map[string]interface{} `json:"sharesmb,omitempty"`    // SMB share options
	SharesNFS   []NFSShareConfig       `json:"sharenfs,omitempty"`    // NFS share options
	
	// SCALE-specific
	Aclmode     string                 `json:"aclmode,omitempty"`     // passthrough, restricted, discard
	Acltype     string                 `json:"acltype,omitempty"`     // nfsv4, posix, off
	Xattr       string                 `json:"xattr,omitempty"`       // on, off, sa
	Casesensitive string               `json:"casesensitivity,omitempty"` // sensitive, insensitive, mixed
}

// NFSShareConfig contains NFS share options.
type NFSShareConfig struct {
	Enabled bool     `json:"enabled"`
	Hosts   []string `json:"hosts,omitempty"`
	Networks []string `json:"networks,omitempty"`
}

// CreateDataset creates a new dataset.
func (c *Client) CreateDataset(ctx context.Context, req CreateDatasetRequest) (*Dataset, error) {
	// Validate name contains pool prefix
	if !strings.Contains(req.Name, "/") {
		return nil, fmt.Errorf("dataset name must include pool prefix (e.g., 'tank/share')")
	}

	var dataset Dataset
	if err := c.Post(ctx, "/pool/dataset", req, &dataset); err != nil {
		return nil, fmt.Errorf("create dataset %s: %w", req.Name, err)
	}
	return &dataset, nil
}

// UpdateDataset updates dataset properties.
func (c *Client) UpdateDataset(ctx context.Context, name string, updates map[string]interface{}) error {
	encoded := url.PathEscape(name)
	if err := c.Put(ctx, fmt.Sprintf("/pool/dataset/id/%s", encoded), updates); err != nil {
		return fmt.Errorf("update dataset %s: %w", name, err)
	}
	return nil
}

// UpdateDatasetQuota updates the quota of a dataset.
func (c *Client) UpdateDatasetQuota(ctx context.Context, name string, quota int64) error {
	return c.UpdateDataset(ctx, name, map[string]interface{}{
		"quota": quota,
	})
}

// UpdateDatasetCompression updates the compression algorithm.
func (c *Client) UpdateDatasetCompression(ctx context.Context, name, compression string) error {
	return c.UpdateDataset(ctx, name, map[string]interface{}{
		"compression": compression,
	})
}

// UpdateDatasetReadonly sets the readonly property.
func (c *Client) UpdateDatasetReadonly(ctx context.Context, name string, readonly bool) {
	value := "off"
	if readonly {
		value = "on"
	}
	c.UpdateDataset(ctx, name, map[string]interface{}{
		"readonly": value,
	})
}

// DeleteDataset deletes a dataset.
func (c *Client) DeleteDataset(ctx context.Context, name string, recursive bool) error {
	encoded := url.PathEscape(name)
	endpoint := fmt.Sprintf("/pool/dataset/id/%s", encoded)
	if recursive {
		endpoint += "?recursive=true"
	}

	if err := c.Delete(ctx, endpoint); err != nil {
		return fmt.Errorf("delete dataset %s: %w", name, err)
	}
	return nil
}

// GetDatasetPermissions returns ACL/permissions for a dataset.
func (c *Client) GetDatasetPermissions(ctx context.Context, name string) (*DatasetPermissions, error) {
	encoded := url.PathEscape(name)
	var perms DatasetPermissions
	if err := c.Get(ctx, fmt.Sprintf("/pool/dataset/id/%s/permission", encoded), &perms); err != nil {
		return nil, fmt.Errorf("get permissions for %s: %w", name, err)
	}
	return &perms, nil
}

// DatasetPermissions represents dataset ACL/permissions.
type DatasetPermissions struct {
	Path  string     `json:"path"`
	ACL   []ACLItem  `json:"acl"`
	UID   int        `json:"uid"`
	GID   int        `json:"gid"`
	Mode  string     `json:"mode"`
}

// ACLItem represents an ACL entry.
type ACLItem struct {
	Tag        string `json:"tag"`        // owner@, group@, everyone@, USER, GROUP
	ID         int    `json:"id,omitempty"`
	Type       string `json:"type"`       // ALLOW, DENY
	Perms      ACLPermissions `json:"perms"`
	Flags      ACLFlags       `json:"flags"`
}

// ACLPermissions contains permission bits.
type ACLPermissions struct {
	ReadData      bool `json:"READ_DATA"`
	WriteData     bool `json:"WRITE_DATA"`
	Execute       bool `json:"EXECUTE"`
	AppendData    bool `json:"APPEND_DATA"`
	DeleteChild   bool `json:"DELETE_CHILD"`
	Delete        bool `json:"DELETE"`
	ReadAttributes bool `json:"READ_ATTRIBUTES"`
	WriteAttributes bool `json:"WRITE_ATTRIBUTES"`
	ReadXattr     bool `json:"READ_NAMED_ATTRS"`
	WriteXattr    bool `json:"WRITE_NAMED_ATTRS"`
	ReadACL       bool `json:"READ_ACL"`
	WriteACL      bool `json:"WRITE_ACL"`
	WriteOwner    bool `json:"WRITE_OWNER"`
	Synchronize   bool `json:"SYNCHRONIZE"`
	Basic         int  `json:"BASIC,omitempty"` // 0=NONE, 1=READ, 2=MODIFY, 3=FULL_CONTROL
}

// ACLFlags contains inheritance flags.
type ACLFlags struct {
	FileInherit      bool `json:"FILE_INHERIT"`
	DirectoryInherit bool `json:"DIRECTORY_INHERIT"`
	NoPropagate      bool `json:"NO_PROPAGATE_INHERIT"`
	InheritOnly      bool `json:"INHERIT_ONLY"`
	Inherited        bool `json:"INHERITED"`
}

// SetDatasetPermissions sets permissions for a dataset.
func (c *Client) SetDatasetPermissions(ctx context.Context, name string, perms DatasetPermissions) error {
	encoded := url.PathEscape(name)
	if err := c.Post(ctx, fmt.Sprintf("/pool/dataset/id/%s/permission", encoded), perms, nil); err != nil {
		return fmt.Errorf("set permissions for %s: %w", name, err)
	}
	return nil
}

// GetDatasetUsers returns users with quota on a dataset.
func (c *Client) GetDatasetUsers(ctx context.Context, name string) ([]DatasetUser, error) {
	encoded := url.PathEscape(name)
	var users []DatasetUser
	if err := c.Get(ctx, fmt.Sprintf("/pool/dataset/id/%s/userquotas", encoded), &users); err != nil {
		return nil, fmt.Errorf("get users for %s: %w", name, err)
	}
	return users, nil
}

// DatasetUser represents a user quota entry.
type DatasetUser struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	UID      int    `json:"uid"`
	Used     int64  `json:"used"`
	Quota    int64  `json:"quota"`
	ObjUsed  int64  `json:"obj_used"`
	ObjQuota int64  `json:"obj_quota"`
}

// GetDatasetGroups returns groups with quota on a dataset.
func (c *Client) GetDatasetGroups(ctx context.Context, name string) ([]DatasetGroup, error) {
	encoded := url.PathEscape(name)
	var groups []DatasetGroup
	if err := c.Get(ctx, fmt.Sprintf("/pool/dataset/id/%s/groupquotas", encoded), &groups); err != nil {
		return nil, fmt.Errorf("get groups for %s: %w", name, err)
	}
	return groups, nil
}

// DatasetGroup represents a group quota entry.
type DatasetGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	GID      int    `json:"gid"`
	Used     int64  `json:"used"`
	Quota    int64  `json:"quota"`
	ObjUsed  int64  `json:"obj_used"`
	ObjQuota int64  `json:"obj_quota"`
}

// PromoteDataset promotes a cloned dataset.
func (c *Client) PromoteDataset(ctx context.Context, name string) error {
	encoded := url.PathEscape(name)
	if err := c.Post(ctx, fmt.Sprintf("/pool/dataset/id/%s/promote", encoded), nil, nil); err != nil {
		return fmt.Errorf("promote dataset %s: %w", name, err)
	}
	return nil
}
