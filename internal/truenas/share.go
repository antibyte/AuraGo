package truenas

import (
	"context"
	"fmt"
)

// SMBShare represents an SMB share configuration.
type SMBShare struct {
	ID         int64    `json:"id"`
	Path       string   `json:"path"`
	PathSuffix string   `json:"path_suffix,omitempty"`
	Name       string   `json:"name"`
	Purpose    string   `json:"purpose,omitempty"`
	Comment    string   `json:"comment,omitempty"`
	HostsAllow []string `json:"hostsallow,omitempty"`
	HostsDeny  []string `json:"hostsdeny,omitempty"`
	Enabled    bool     `json:"enabled"`

	// Access controls
	ACL []SMBACLEntry `json:"acl,omitempty"`

	// SMB options
	GuestOK          bool   `json:"guestok,omitempty"`
	Browseable       bool   `json:"browseable,omitempty"`
	ReadOnly         bool   `json:"ro,omitempty"`
	ShowHiddenFiles  bool   `json:"showhiddenfiles,omitempty"`
	HomeShare        bool   `json:"home_share,omitempty"`
	Timemachine      bool   `json:"timemachine,omitempty"`
	RecycleBin       bool   `json:"recyclebin,omitempty"`
	ShadowCopy       bool   `json:"shadowcopy,omitempty"`
	DurabilityHandle bool   `json:"durablehandle,omitempty"`
	StreamSupport    string `json:"streams,omitempty"` // native, ro, disabled

	// VSS/Aux
	VFSObjects []string `json:"vfsobjects,omitempty"`
	Auxiliary  string   `json:"auxsmbconf,omitempty"`
}

// ListSMBShares returns all SMB shares.
func (c *Client) ListSMBShares(ctx context.Context) ([]SMBShare, error) {
	var shares []SMBShare
	if err := c.Get(ctx, "/sharing/smb", &shares); err != nil {
		return nil, fmt.Errorf("list SMB shares: %w", err)
	}
	return shares, nil
}

// GetSMBShare returns an SMB share by ID.
func (c *Client) GetSMBShare(ctx context.Context, id int64) (*SMBShare, error) {
	var share SMBShare
	if err := c.Get(ctx, fmt.Sprintf("/sharing/smb/id/%d", id), &share); err != nil {
		return nil, fmt.Errorf("get SMB share %d: %w", id, err)
	}
	return &share, nil
}

// CreateSMBShareRequest contains parameters for creating an SMB share.
type CreateSMBShareRequest struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Purpose     string `json:"purpose,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Enabled     bool   `json:"enabled"`
	GuestOK     bool   `json:"guestok,omitempty"`
	Browseable  bool   `json:"browseable,omitempty"`
	ReadOnly    bool   `json:"ro,omitempty"`
	Timemachine bool   `json:"timemachine,omitempty"`
	HomeShare   bool   `json:"home_share,omitempty"`
	ShadowCopy  bool   `json:"shadowcopy,omitempty"`
	RecycleBin  bool   `json:"recyclebin,omitempty"`
	Auxiliary   string `json:"auxsmbconf,omitempty"`
}

// CreateSMBShare creates a new SMB share.
func (c *Client) CreateSMBShare(ctx context.Context, req CreateSMBShareRequest) (*SMBShare, error) {
	// Set defaults
	if !req.ReadOnly && !req.GuestOK && !req.Browseable {
		req.Browseable = true
		req.RecycleBin = true
		req.ShadowCopy = true
	}

	var share SMBShare
	if err := c.Post(ctx, "/sharing/smb", req, &share); err != nil {
		return nil, fmt.Errorf("create SMB share: %w", err)
	}
	return &share, nil
}

// UpdateSMBShare updates an existing SMB share.
func (c *Client) UpdateSMBShare(ctx context.Context, id int64, updates map[string]interface{}) error {
	if err := c.Put(ctx, fmt.Sprintf("/sharing/smb/id/%d", id), updates); err != nil {
		return fmt.Errorf("update SMB share %d: %w", id, err)
	}
	return nil
}

// DeleteSMBShare deletes an SMB share.
func (c *Client) DeleteSMBShare(ctx context.Context, id int64) error {
	if err := c.Delete(ctx, fmt.Sprintf("/sharing/smb/id/%d", id)); err != nil {
		return fmt.Errorf("delete SMB share %d: %w", id, err)
	}
	return nil
}

// NFSShare represents an NFS share configuration.
type NFSShare struct {
	ID      int64  `json:"id"`
	Path    string `json:"path"`
	Comment string `json:"comment,omitempty"`
	Enabled bool   `json:"enabled"`

	// Client access
	Networks []string `json:"networks,omitempty"`
	Hosts    []string `json:"hosts,omitempty"`

	// Options
	ReadOnly     bool     `json:"ro,omitempty"`
	Quiet        bool     `json:"quiet,omitempty"`
	MaprootUser  string   `json:"maproot_user,omitempty"`
	MaprootGroup string   `json:"maproot_group,omitempty"`
	MapallUser   string   `json:"mapall_user,omitempty"`
	MapallGroup  string   `json:"mapall_group,omitempty"`
	Security     []string `json:"security,omitempty"` // SYS, KRB5, KRB5I, KRB5P

	// Advanced
	Aliases []string `json:"aliases,omitempty"`
}

// ListNFSShares returns all NFS shares.
func (c *Client) ListNFSShares(ctx context.Context) ([]NFSShare, error) {
	var shares []NFSShare
	if err := c.Get(ctx, "/sharing/nfs", &shares); err != nil {
		return nil, fmt.Errorf("list NFS shares: %w", err)
	}
	return shares, nil
}

// GetNFSShare returns an NFS share by ID.
func (c *Client) GetNFSShare(ctx context.Context, id int64) (*NFSShare, error) {
	var share NFSShare
	if err := c.Get(ctx, fmt.Sprintf("/sharing/nfs/id/%d", id), &share); err != nil {
		return nil, fmt.Errorf("get NFS share %d: %w", id, err)
	}
	return &share, nil
}

// CreateNFSShareRequest contains parameters for creating an NFS share.
type CreateNFSShareRequest struct {
	Path         string   `json:"path"`
	Comment      string   `json:"comment,omitempty"`
	Enabled      bool     `json:"enabled"`
	Networks     []string `json:"networks,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	ReadOnly     bool     `json:"ro,omitempty"`
	MaprootUser  string   `json:"maproot_user,omitempty"`
	MaprootGroup string   `json:"maproot_group,omitempty"`
	Security     []string `json:"security,omitempty"`
}

// CreateNFSShare creates a new NFS share.
func (c *Client) CreateNFSShare(ctx context.Context, req CreateNFSShareRequest) (*NFSShare, error) {
	var share NFSShare
	if err := c.Post(ctx, "/sharing/nfs", req, &share); err != nil {
		return nil, fmt.Errorf("create NFS share: %w", err)
	}
	return &share, nil
}

// UpdateNFSShare updates an existing NFS share.
func (c *Client) UpdateNFSShare(ctx context.Context, id int64, updates map[string]interface{}) error {
	if err := c.Put(ctx, fmt.Sprintf("/sharing/nfs/id/%d", id), updates); err != nil {
		return fmt.Errorf("update NFS share %d: %w", id, err)
	}
	return nil
}

// DeleteNFSShare deletes an NFS share.
func (c *Client) DeleteNFSShare(ctx context.Context, id int64) error {
	if err := c.Delete(ctx, fmt.Sprintf("/sharing/nfs/id/%d", id)); err != nil {
		return fmt.Errorf("delete NFS share %d: %w", id, err)
	}
	return nil
}

// ShareInfo contains information about a dataset's shares.
type ShareInfo struct {
	SMBShares []SMBShare `json:"smb_shares"`
	NFSShares []NFSShare `json:"nfs_shares"`
	HasShares bool       `json:"has_shares"`
}

// GetSharesForPath returns all shares for a given path.
func (c *Client) GetSharesForPath(ctx context.Context, path string) (*ShareInfo, error) {
	smbShares, err := c.ListSMBShares(ctx)
	if err != nil {
		return nil, err
	}

	nfsShares, err := c.ListNFSShares(ctx)
	if err != nil {
		return nil, err
	}

	info := &ShareInfo{
		SMBShares: []SMBShare{},
		NFSShares: []NFSShare{},
	}

	for _, s := range smbShares {
		if s.Path == path {
			info.SMBShares = append(info.SMBShares, s)
		}
	}

	for _, s := range nfsShares {
		if s.Path == path {
			info.NFSShares = append(info.NFSShares, s)
		}
	}

	info.HasShares = len(info.SMBShares) > 0 || len(info.NFSShares) > 0
	return info, nil
}

// SMBACLEntry represents an SMB ACL entry.
type SMBACLEntry struct {
	Domain     string   `json:"domain,omitempty"`
	Name       string   `json:"name"`
	Permission string   `json:"permission"` // READ, CHANGE, FULL_CONTROL
	Aeftype    string   `json:"ae_type"`    // ALLOWED, DENIED
	Aewho      string   `json:"ae_who"`     // USER, GROUP, OWNER, EVERYONE
	Aeflags    []string `json:"ae_flags,omitempty"`
}

// DeleteSharesForPath removes all shares for a given path.
func (c *Client) DeleteSharesForPath(ctx context.Context, path string) (int, error) {
	info, err := c.GetSharesForPath(ctx, path)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, s := range info.SMBShares {
		if err := c.DeleteSMBShare(ctx, s.ID); err == nil {
			deleted++
		}
	}

	for _, s := range info.NFSShares {
		if err := c.DeleteNFSShare(ctx, s.ID); err == nil {
			deleted++
		}
	}

	return deleted, nil
}
