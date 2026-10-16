package ruview

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

type Node struct {
	NodeID        int      `json:"node_id"`
	CSIStatus     string   `json:"csi_status"`
	CSILastSeenMS *int64   `json:"csi_last_seen_ms"`
	CSISequence   *uint32  `json:"csi_sequence"`
	RSSI          *float64 `json:"rssi_dbm"`
}

type NodeList struct {
	Nodes     []Node `json:"nodes"`
	Available bool   `json:"available"`
	Scope     string `json:"scope"`
}

// Nodes retains known board identities separately from upstream's transient telemetry.
func (m *Manager) Nodes(parent context.Context) (NodeList, error) {
	c := m.config()
	if !c.RuView.Enabled {
		return NodeList{}, fmt.Errorf("RuView is disabled")
	}
	ctx, done := m.bound(parent)
	defer done()
	name, err := nodeFile(c)
	if err != nil {
		return NodeList{}, err
	}
	result := NodeList{Nodes: []Node{}, Scope: filepath.Base(name)}
	if c.RuView.Source == "esp32" {
		data, err := m.request(ctx, c, "GET", "/api/v1/nodes", nil)
		if err == nil {
			var live struct {
				Nodes *[]Node `json:"nodes"`
			}
			if json.Unmarshal(data, &live) == nil && live.Nodes != nil && validNodes(*live.Nodes) {
				result.Nodes, result.Available = *live.Nodes, true
			}
		}
	}
	ids := make([]int, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		ids = append(ids, node.NodeID)
	}
	known, err := m.knownNodes(ctx, c, ids)
	if err != nil {
		return NodeList{}, err
	}
	for _, id := range known {
		if !slices.Contains(ids, id) {
			result.Nodes = append(result.Nodes, Node{NodeID: id, CSIStatus: "offline"})
		}
	}
	slices.SortFunc(result.Nodes, func(a, b Node) int { return a.NodeID - b.NodeID })
	return result, ctx.Err()
}

func validNodes(nodes []Node) bool {
	if len(nodes) > 255 {
		return false
	}
	seen := make(map[int]bool, len(nodes))
	for _, node := range nodes {
		if node.NodeID < 1 || node.NodeID > 255 || seen[node.NodeID] ||
			(node.CSIStatus != "active" && node.CSIStatus != "stale") ||
			(node.CSILastSeenMS != nil && *node.CSILastSeenMS < 0) {
			return false
		}
		seen[node.NodeID] = true
	}
	return true
}

// RememberNode records only the ID, after the operator confirms serial readback.
func (m *Manager) RememberNode(parent context.Context, id int) error {
	c := m.config()
	if !c.RuView.Enabled || c.RuView.ReadOnly {
		return fmt.Errorf("RuView is disabled or read-only")
	}
	if id < 1 || id > 255 {
		return fmt.Errorf("RuView node ID must be between 1 and 255")
	}
	ctx, done := m.bound(parent)
	defer done()
	_, err := m.knownNodes(ctx, c, []int{id})
	return err
}

func nodeFile(c *config.Config) (string, error) {
	u, err := target(c)
	if err != nil {
		return "", err
	}
	// A different service must not inherit another receiver's board identities.
	scope := c.RuView.Mode + "\n" + u.String()
	if c.RuView.Mode == "docker" {
		scope += "\n" + c.Docker.Host
	}
	return filepath.Join(c.Directories.DataDir, "ruview", fmt.Sprintf("nodes-%x.json", sha256.Sum256([]byte(scope)))), nil
}

func (m *Manager) knownNodes(ctx context.Context, c *config.Config, observed []int) ([]int, error) {
	name, err := nodeFile(c)
	if err != nil {
		return nil, err
	}
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var known []int
	f, err := os.Open(name)
	if err == nil {
		data, readErr := io.ReadAll(io.LimitReader(f, 4097))
		f.Close()
		if readErr != nil || len(data) > 4096 || json.Unmarshal(data, &known) != nil || len(known) > 255 {
			return nil, fmt.Errorf("RuView saved device list is invalid")
		}
		for i, id := range known {
			if id < 1 || id > 255 || slices.Contains(known[:i], id) {
				return nil, fmt.Errorf("RuView saved node ID is invalid")
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read RuView device list: %w", err)
	}
	changed := false
	for _, id := range observed {
		if !slices.Contains(known, id) {
			known = append(known, id)
			changed = true
		}
	}
	slices.Sort(known)
	if changed {
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return nil, fmt.Errorf("create RuView device directory: %w", err)
		}
		data, err := json.Marshal(known)
		if err != nil {
			return nil, fmt.Errorf("encode RuView devices: %w", err)
		}
		ctx = fileutil.WithPublicationGate(ctx, func(commit func() error) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			if !reflect.DeepEqual(c.RuView, m.config().RuView) {
				return context.Canceled
			}
			return commit()
		})
		if err := fileutil.WriteFileContext(ctx, name, data, 0600); err != nil {
			return nil, fmt.Errorf("save RuView device list: %w", err)
		}
	}
	return known, ctx.Err()
}
