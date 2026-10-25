package ruview

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"aurago/internal/security"
)

// payloadRead is one live GET plus the status observed around it.
// status_error is set only when that payload arrived and the status probe did not.
type payloadRead struct {
	Source      string          `json:"source"`
	ObservedAt  time.Time       `json:"observed_at"`
	Revision    string          `json:"revision"`
	Fresh       bool            `json:"fresh"`
	Simulated   bool            `json:"simulated"`
	DataState   string          `json:"data_state"`
	StatusError string          `json:"status_error,omitempty"`
	Data        json.RawMessage `json:"data"`
}

// presenceRead is the agent summary. It never carries a scope path, a file path, or Wi-Fi data.
// live_state is NodeList.LiveState: it tells an offline board from a live list that could not be read.
type presenceRead struct {
	Source      string         `json:"source"`
	ObservedAt  time.Time      `json:"observed_at"`
	Revision    string         `json:"revision"`
	Fresh       bool           `json:"fresh"`
	Simulated   bool           `json:"simulated"`
	DataState   string         `json:"data_state"`
	StatusError string         `json:"status_error,omitempty"`
	LiveState   string         `json:"live_state"`
	Nodes       []presenceNode `json:"nodes"`
}

type presenceNode struct {
	NodeID       int      `json:"node_id"`
	State        string   `json:"state"`
	Fresh        bool     `json:"fresh"`
	RSSIRetained bool     `json:"rssi_retained"`
	RSSI         *float64 `json:"rssi_dbm,omitempty"`
}

var statusErrorURL = regexp.MustCompile(`(?i)\b(?:https?|wss?)://\S+`)

func (m *Manager) readPresence(ctx context.Context) (presenceRead, error) {
	list, err := m.Nodes(ctx)
	if err != nil {
		return presenceRead{}, err
	}
	out := presenceRead{LiveState: list.LiveState, Nodes: make([]presenceNode, 0, len(list.Nodes))}
	m.attachStatus(ctx, &out.Source, &out.Fresh, &out.Simulated, &out.DataState, &out.StatusError)
	out.ObservedAt = time.Now().UTC()
	out.Revision = Revision
	for _, node := range list.Nodes {
		item := presenceNode{NodeID: node.NodeID, State: presenceState(node.CSIStatus)}
		item.Fresh = presenceFresh(out.Source, out.Simulated, item.State, node)
		if item.State == "live" && node.RSSI != nil {
			rssi := *node.RSSI
			item.RSSI = &rssi
		}
		out.Nodes = append(out.Nodes, item)
	}
	return out, nil
}

func (m *Manager) attachStatus(ctx context.Context, source *string, fresh, simulated *bool, dataState, statusError *string) {
	prior := m.Status()
	status, err := m.Test(ctx)
	if err != nil {
		*source = prior.Source
		*fresh = false
		*simulated = prior.Simulated
		*dataState = "disconnected"
		*statusError = readStatusError(err)
		return
	}
	*source = status.Source
	*fresh = status.Fresh
	*simulated = status.Simulated
	*dataState = status.DataState
}

func presenceState(status string) string {
	switch status {
	case "active":
		return "live"
	case "stale":
		return "stale"
	default:
		return "offline"
	}
}

// presenceFresh is true only for real ESP32 CSI that is active, sequenced, and at most 10 seconds old.
func presenceFresh(source string, simulated bool, state string, node Node) bool {
	if source != "esp32" || simulated || state != "live" || node.CSISequence == nil || node.CSILastSeenMS == nil {
		return false
	}
	age := *node.CSILastSeenMS
	return age >= 0 && age <= 10000
}

func readStatusError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.Join(strings.Fields(statusErrorURL.ReplaceAllString(security.Scrub(err.Error()), "")), " ")
	if text == "" {
		return "RuView status could not be read"
	}
	if len(text) > 160 {
		text = strings.TrimSpace(text[:160])
	}
	return text
}
