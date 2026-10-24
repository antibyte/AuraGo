// Package infra stores the editable infrastructure graph and safe source observations.
package infra

import "time"

type NodeKind string

const (
	NodeDevice    NodeKind = "device"
	NodeVM        NodeKind = "vm"
	NodeContainer NodeKind = "container"
	NodeService   NodeKind = "service"
	NodeNetwork   NodeKind = "network"
	NodeSite      NodeKind = "site"
	NodeRoom      NodeKind = "room"
	NodeRack      NodeKind = "rack"
	NodeGroup     NodeKind = "group"
)

type View string

const (
	ViewAtlas    View = "atlas"
	ViewTopology View = "topology"
)

type Node struct {
	ID                  string                       `json:"id"`
	Version             int64                        `json:"version"`
	Kind                NodeKind                     `json:"kind"`
	Name                string                       `json:"name"`
	Icon                string                       `json:"icon"`
	RegistryID          string                       `json:"registry_id,omitempty"`
	ParentID            string                       `json:"parent_id,omitempty"`
	Profile             Profile                      `json:"profile"`
	Placements          Placements                   `json:"placements"`
	Sources             []Source                     `json:"sources"`
	AvailableActions    []string                     `json:"available_actions"`
	AppLinks            []AppLink                    `json:"app_links"`
	Power               string                       `json:"power,omitempty"`
	ActionReasons       map[string]string            `json:"action_reasons,omitempty"`
	SourceActions       map[string][]string          `json:"source_actions,omitempty"`
	SourceActionReasons map[string]map[string]string `json:"source_action_reasons,omitempty"`
}

// AppLink names a local application target. Profile.AppBindings persists explicit
// user associations; Node.AppLinks is a permission-filtered runtime projection.
type AppLink struct {
	App      string `json:"app"`
	TargetID string `json:"target_id"`
}

type Profile struct {
	Role         string             `json:"role,omitempty"`
	Vendor       string             `json:"vendor,omitempty"`
	Model        string             `json:"model,omitempty"`
	Serial       string             `json:"serial,omitempty"`
	OS           string             `json:"os,omitempty"`
	Firmware     string             `json:"firmware,omitempty"`
	CPU          string             `json:"cpu,omitempty"`
	RAM          string             `json:"ram,omitempty"`
	Storage      string             `json:"storage,omitempty"`
	RackPosition string             `json:"rack_position,omitempty"`
	IPAddress    string             `json:"ip_address,omitempty"`
	CIDR         string             `json:"cidr,omitempty"`
	VLAN         int                `json:"vlan,omitempty"`
	MACAddress   string             `json:"mac_address,omitempty"`
	Tags         []string           `json:"tags"`
	Interfaces   []NetworkInterface `json:"interfaces"`
	Links        []Link             `json:"links"`
	AppBindings  []AppLink          `json:"app_bindings"`
	Notes        string             `json:"notes,omitempty"`
	Custom       map[string]string  `json:"custom"`
}

type NetworkInterface struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address,omitempty"`
	MACAddress string `json:"mac_address,omitempty"`
	VLAN       string `json:"vlan,omitempty"`
	Speed      string `json:"speed,omitempty"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Placements map[View]Placement

// Placement is one view position. W and H size a place region (sites, rooms,
// racks and groups only); zero means the client default size.
type Placement struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w,omitempty"`
	H      float64 `json:"h,omitempty"`
	Pinned bool    `json:"pinned"`
}

type SourceState string

const (
	SourceCurrent SourceState = "current"
	SourceStale   SourceState = "stale"
	SourceUnknown SourceState = "unknown"
	SourceRemoved SourceState = "removed"
)

const (
	PowerRunning = "running"
	PowerStopped = "stopped"
	PowerPaused  = "paused"
	PowerUnknown = "unknown"
)

type SourceIdentity struct {
	Kind     string `json:"kind"`
	Instance string `json:"instance"`
	Key      string `json:"key"`
}

type Source struct {
	SourceIdentity
	Name      string         `json:"name,omitempty"`
	State     SourceState    `json:"state"`
	FetchedAt *time.Time     `json:"fetched_at,omitempty"`
	RemovedAt *time.Time     `json:"removed_at,omitempty"`
	Observed  SourceObserved `json:"observed"`
	// Ignored hides the source from discoveries. An ignored source is never linked.
	Ignored bool `json:"ignored,omitempty"`
}

// SourceObserved contains only bounded, typed facts safe to retain from an adapter.
// Provider payloads, credentials, Fritz!Box MAC keys, and arbitrary JSON are excluded.
type SourceObserved struct {
	Kind         NodeKind           `json:"kind"`
	Name         string             `json:"name"`
	Icon         string             `json:"icon"`
	Role         string             `json:"role,omitempty"`
	Vendor       string             `json:"vendor,omitempty"`
	Model        string             `json:"model,omitempty"`
	Serial       string             `json:"serial,omitempty"`
	OS           string             `json:"os,omitempty"`
	Firmware     string             `json:"firmware,omitempty"`
	CPU          string             `json:"cpu,omitempty"`
	RAM          string             `json:"ram,omitempty"`
	Storage      string             `json:"storage,omitempty"`
	RackPosition string             `json:"rack_position,omitempty"`
	IPAddress    string             `json:"ip_address,omitempty"`
	CIDR         string             `json:"cidr,omitempty"`
	VLAN         int                `json:"vlan,omitempty"`
	Protected    bool               `json:"protected,omitempty"`
	Host         string             `json:"host,omitempty"`
	MACAddress   string             `json:"mac_address,omitempty"`
	Power        string             `json:"power,omitempty"`
	Tags         []string           `json:"tags"`
	Interfaces   []NetworkInterface `json:"interfaces"`
}

type Candidate struct {
	ID          string   `json:"id"`
	Source      Source   `json:"source"`
	Node        Node     `json:"node"`
	RegistryID  string   `json:"registry_id,omitempty"`
	Suggestions []string `json:"suggestions"`
	Ignored     bool     `json:"ignored,omitempty"`
}

type EdgeKind string

const (
	EdgeEthernet  EdgeKind = "ethernet"
	EdgeFiber     EdgeKind = "fiber"
	EdgeWiFi      EdgeKind = "wifi"
	EdgeMemberOf  EdgeKind = "member_of"
	EdgeRunsOn    EdgeKind = "runs_on"
	EdgeDependsOn EdgeKind = "depends_on"
	EdgePoweredBy EdgeKind = "powered_by"
)

type Edge struct {
	ID       string          `json:"id"`
	Version  int64           `json:"version"`
	From     string          `json:"from"`
	To       string          `json:"to"`
	Kind     EdgeKind        `json:"kind"`
	Label    string          `json:"label,omitempty"`
	FromPort string          `json:"from_port,omitempty"`
	ToPort   string          `json:"to_port,omitempty"`
	Speed    string          `json:"speed,omitempty"`
	Source   *SourceIdentity `json:"source,omitempty"`
}

// LayoutItem moves one node. ParentID nil keeps the parent and "" clears it;
// W/H nil keep the stored place size.
type LayoutItem struct {
	ID       string   `json:"id"`
	Version  int64    `json:"version"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Pinned   bool     `json:"pinned"`
	ParentID *string  `json:"parent_id,omitempty"`
	W        *float64 `json:"w,omitempty"`
	H        *float64 `json:"h,omitempty"`
}

type LayoutRequest struct {
	View  View         `json:"view"`
	Nodes []LayoutItem `json:"nodes"`
}

type Capabilities struct {
	Write      bool `json:"write"`
	SourceEdit bool `json:"source_edit"`
	Control    bool `json:"control"`
}

// SourceHost maps a provider engine instance (Docker only) to the Infra device
// that runs it. InferredNodeID is a runtime projection and is never stored.
type SourceHost struct {
	Kind           string `json:"kind"`
	Instance       string `json:"instance"`
	NodeID         string `json:"node_id"`
	Version        int64  `json:"version"`
	InferredNodeID string `json:"inferred_node_id,omitempty"`
}

type Snapshot struct {
	Nodes        []Node       `json:"nodes"`
	Edges        []Edge       `json:"edges"`
	Sources      []Source     `json:"sources"`
	SourceHosts  []SourceHost `json:"source_hosts"`
	Capabilities Capabilities `json:"capabilities"`
}

type SourceObservation struct {
	Identity SourceIdentity `json:"identity"`
	Name     string         `json:"name,omitempty"`
	Observed SourceObserved `json:"observed"`
}

type ActionRequest struct {
	RequestID   string `json:"request_id"`
	Fingerprint string `json:"fingerprint"`
	Action      string `json:"action"`
}

type ActionRecord struct {
	RequestID string `json:"request_id"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	UPID      string `json:"upid,omitempty"`
}
