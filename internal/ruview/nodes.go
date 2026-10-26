package ruview

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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

// NodeList.LiveState says why the live list is, or is not, part of Nodes.
// Available is kept for compatibility and is true exactly for liveStateOK.
const (
	liveStateOK            = "ok"             // upstream list was read and valid
	liveStateNotApplicable = "not_applicable" // the configured source has no node list
	liveStateNotRunning    = "not_running"    // managed runtime is not running in this process; upstream was not dialed
	liveStateUnreachable   = "unreachable"    // the upstream request failed
	liveStateInvalid       = "invalid"        // the upstream reply was not a valid node list
)

type NodeList struct {
	Nodes     []Node `json:"nodes"`
	Available bool   `json:"available"`
	LiveState string `json:"live_state"`
	Scope     string `json:"scope"`
}

// Nodes retains known board identities separately from upstream's transient telemetry.
func (m *Manager) Nodes(parent context.Context) (NodeList, error) {
	c := m.config()
	if !c.RuView.Enabled {
		return NodeList{}, fail(CodeDisabled, "RuView is disabled", nil)
	}
	ctx, done := m.bound(parent)
	defer done()
	name, err := nodeFile(c)
	if err != nil {
		return NodeList{}, fail(CodeInvalidConfig, err.Error(), err)
	}
	result := NodeList{Nodes: []Node{}, Scope: filepath.Base(name), LiveState: liveStateNotApplicable}
	if c.RuView.Source == "esp32" {
		m.mu.Lock()
		running := m.runningLocked()
		m.mu.Unlock()
		if c.RuView.Mode != "external" && !running {
			// Nothing of ours is listening; dialing would only report a refused connection as a fault.
			result.LiveState = liveStateNotRunning
		} else {
			result.LiveState = liveStateUnreachable
			data, err := m.request(ctx, c, "GET", "/api/v1/nodes", nil)
			switch {
			case err == nil:
				result.LiveState = liveStateInvalid
				var live struct {
					Nodes *[]Node `json:"nodes"`
				}
				if json.Unmarshal(data, &live) == nil && live.Nodes != nil && validNodes(*live.Nodes) {
					result.Nodes, result.Available, result.LiveState = *live.Nodes, true, liveStateOK
				}
			case errors.Is(err, errInvalidReply):
				// The service answered 2xx, but not with JSON it could be asked to parse.
				result.LiveState = liveStateInvalid
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
	if !c.RuView.Enabled {
		return fail(CodeDisabled, "RuView is disabled", nil)
	}
	if c.RuView.ReadOnly {
		return fail(CodeReadOnly, "RuView is read-only", nil)
	}
	if id < 1 || id > 255 {
		return fail(CodeInvalidNodeID, "RuView node ID must be between 1 and 255", nil)
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

// deviceListError types a failure of the saved device list. The OS errors carry
// the file's full path, so the sentence is fixed and the cause reaches only the
// scrubbed Detail: the agent tool forwards err.Error() to the model. A revoked
// context stays cancelled/timeout.
func deviceListError(ctx context.Context, msg string, cause error) error {
	switch {
	case errors.Is(cause, context.DeadlineExceeded) || errors.Is(cause, context.Canceled):
		return ctxError(cause)
	case ctx.Err() != nil:
		return ctxError(ctx.Err())
	}
	return fail(CodeUnknown, msg, cause)
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
			return nil, fail(CodeUnknown, "RuView saved device list is invalid", nil)
		}
		for i, id := range known {
			if id < 1 || id > 255 || slices.Contains(known[:i], id) {
				return nil, fail(CodeUnknown, "RuView saved node ID is invalid", nil)
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, deviceListError(ctx, "RuView device list could not be read", err)
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
			return nil, deviceListError(ctx, "RuView device directory could not be created", err)
		}
		data, err := json.Marshal(known)
		if err != nil {
			return nil, deviceListError(ctx, "RuView devices could not be encoded", err)
		}
		ctx = fileutil.WithPublicationGate(ctx, func(commit func() error) error {
			// The rename runs with mu held, so a Configure cannot slip in between
			// the check and the publication. It is one local file rename.
			m.mu.Lock()
			defer m.mu.Unlock()
			if !reflect.DeepEqual(c.RuView, m.config().RuView) {
				return context.Canceled
			}
			return commit()
		})
		if err := fileutil.WriteFileContext(ctx, name, data, 0600); err != nil {
			return nil, deviceListError(ctx, "RuView device list could not be saved", err)
		}
	}
	return known, ctx.Err()
}
