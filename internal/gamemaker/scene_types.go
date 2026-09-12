package gamemaker

import "encoding/json"

const (
	SceneSchemaVersion = 1
	SceneFilePath      = "src/scene.json"
	// SceneMaxItems bounds every ID-keyed collection shared with the runtime.
	SceneMaxItems = 4096
)

// Vec3 is used for both dimensions. A 2D scene always stores z as zero.
type Vec3 [3]float64

type SceneBounds struct {
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
}

type Bounds = SceneBounds

type SceneLevel struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Active bool   `json:"active"`
}

type SceneNode struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Position   Vec3           `json:"position"`
	Size       Vec3           `json:"size,omitempty"`
	Pinned     bool           `json:"pinned,omitempty"`
	LevelID    string         `json:"level_id,omitempty"`
	RegionID   string         `json:"region_id,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

type SceneRegion struct {
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	Bounds    SceneBounds `json:"bounds"`
	Seed      int64       `json:"seed"`
	LevelID   string      `json:"level_id,omitempty"`
	Generator string      `json:"generator,omitempty"`
	Pinned    bool        `json:"pinned,omitempty"`
}

// ScenePlacement deliberately keeps asset identity and game behavior as
// separate fields. A visual asset never implicitly grants behavior.
type ScenePlacement struct {
	ID        string `json:"id"`
	NodeID    string `json:"node_id"`
	AssetID   string `json:"asset_id,omitempty"`
	AssetRole string `json:"asset_role"`
	Behavior  string `json:"behavior"`
	Position  Vec3   `json:"position"`
	Rotation  Vec3   `json:"rotation,omitempty"`
	Scale     Vec3   `json:"scale,omitempty"`
	LevelID   string `json:"level_id,omitempty"`
	RegionID  string `json:"region_id,omitempty"`
}

type SceneCollider struct {
	ID       string `json:"id"`
	NodeID   string `json:"node_id"`
	Shape    string `json:"shape"`
	Extents  Vec3   `json:"extents"`
	Offset   Vec3   `json:"offset,omitempty"`
	LevelID  string `json:"level_id,omitempty"`
	RegionID string `json:"region_id,omitempty"`
}

type SceneAttachment struct {
	ID       string `json:"id"`
	NodeID   string `json:"node_id"`
	AssetID  string `json:"asset_id,omitempty"`
	Socket   string `json:"socket"`
	Position Vec3   `json:"position,omitempty"`
	Rotation Vec3   `json:"rotation,omitempty"`
	LevelID  string `json:"level_id,omitempty"`
	RegionID string `json:"region_id,omitempty"`
}

type SceneZone struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Bounds     SceneBounds    `json:"bounds"`
	LevelID    string         `json:"level_id,omitempty"`
	RegionID   string         `json:"region_id,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

type SceneRoute struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Waypoints []Vec3 `json:"waypoints,omitempty"`
	Kind      string `json:"kind,omitempty"`
	LevelID   string `json:"level_id,omitempty"`
	RegionID  string `json:"region_id,omitempty"`
}

type Scene struct {
	SchemaVersion int               `json:"schema_version"`
	Dimension     string            `json:"dimension"`
	Seed          int64             `json:"seed"`
	Navigation    string            `json:"navigation,omitempty"`
	Levels        []SceneLevel      `json:"levels"`
	WorldBounds   SceneBounds       `json:"world_bounds"`
	CameraBounds  *SceneBounds      `json:"camera_bounds,omitempty"`
	Nodes         []SceneNode       `json:"nodes,omitempty"`
	Regions       []SceneRegion     `json:"regions,omitempty"`
	Placements    []ScenePlacement  `json:"placements,omitempty"`
	Colliders     []SceneCollider   `json:"colliders,omitempty"`
	Attachments   []SceneAttachment `json:"attachments,omitempty"`
	Zones         []SceneZone       `json:"zones,omitempty"`
	Routes        []SceneRoute      `json:"routes,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// SceneAsset describes only the accepted catalog facts needed by validation.
// A catalog entry without roles or sockets cannot authorize those references.
type SceneAsset struct {
	ID      string      `json:"id"`
	Roles   []string    `json:"roles,omitempty"`
	Sockets []string    `json:"sockets,omitempty"`
	Bounds  SceneBounds `json:"bounds,omitempty"`
}

type SceneAssetCatalog struct {
	Assets map[string]SceneAsset `json:"assets,omitempty"`
}

// AssetCatalog is kept as the short public name used by agent/runtime callers.
type AssetCatalog = SceneAssetCatalog

type SceneDiagnostic struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type SceneChanges struct {
	Added    []string       `json:"added,omitempty"`
	Modified []string       `json:"modified,omitempty"`
	Removed  []string       `json:"removed,omitempty"`
	Counts   map[string]int `json:"counts"`
}

type SceneResult struct {
	Changes *SceneChanges `json:"changes,omitempty"`
	Scene   Scene         `json:"scene"`
	Path    string        `json:"path"`
	// SHA256 remains the hash callers should use as the next conditional
	// precondition: the current file for inspect/dry-run and the written file
	// after a successful mutation.
	SHA256         string            `json:"sha256"`
	CurrentSHA256  string            `json:"current_sha256,omitempty"`
	ProposedSHA256 string            `json:"proposed_sha256,omitempty"`
	Exists         bool              `json:"exists"`
	Written        bool              `json:"written"`
	DryRun         bool              `json:"dry_run"`
	Diagnostics    []SceneDiagnostic `json:"diagnostics,omitempty"`
}

// ScenePatch uses ID-keyed upserts and removals so a local edit cannot replace
// unrelated scene content accidentally.
type ScenePatch struct {
	Replace             *Scene            `json:"replace,omitempty"`
	SchemaVersion       *int              `json:"schema_version,omitempty"`
	Dimension           *string           `json:"dimension,omitempty"`
	Seed                *int64            `json:"seed,omitempty"`
	Navigation          *string           `json:"navigation,omitempty"`
	WorldBounds         *SceneBounds      `json:"world_bounds,omitempty"`
	CameraBounds        *SceneBounds      `json:"camera_bounds,omitempty"`
	ClearCameraBounds   bool              `json:"clear_camera_bounds,omitempty"`
	Levels              *[]SceneLevel     `json:"levels,omitempty"`
	Nodes               []SceneNode       `json:"nodes,omitempty"`
	RemoveNodeIDs       []string          `json:"remove_node_ids,omitempty"`
	Regions             []SceneRegion     `json:"regions,omitempty"`
	RemoveRegionIDs     []string          `json:"remove_region_ids,omitempty"`
	Placements          []ScenePlacement  `json:"placements,omitempty"`
	RemovePlacementIDs  []string          `json:"remove_placement_ids,omitempty"`
	Colliders           []SceneCollider   `json:"colliders,omitempty"`
	RemoveColliderIDs   []string          `json:"remove_collider_ids,omitempty"`
	Attachments         []SceneAttachment `json:"attachments,omitempty"`
	RemoveAttachmentIDs []string          `json:"remove_attachment_ids,omitempty"`
	Zones               []SceneZone       `json:"zones,omitempty"`
	RemoveZoneIDs       []string          `json:"remove_zone_ids,omitempty"`
	Routes              []SceneRoute      `json:"routes,omitempty"`
	RemoveRouteIDs      []string          `json:"remove_route_ids,omitempty"`
}

type GenerateSceneRegionRequest struct {
	RegionID    string       `json:"region_id"`
	Kind        string       `json:"kind"`
	Bounds      SceneBounds  `json:"bounds"`
	Seed        int64        `json:"seed"`
	LevelID     string       `json:"level_id,omitempty"`
	Density     int          `json:"density,omitempty"`
	CellSize    float64      `json:"cell_size,omitempty"`
	MinDistance float64      `json:"min_distance,omitempty"`
	Clear       bool         `json:"clear,omitempty"`
	NodeKind    string       `json:"node_kind,omitempty"`
	AssetID     string       `json:"asset_id,omitempty"`
	AssetRole   string       `json:"asset_role,omitempty"`
	Behavior    string       `json:"behavior,omitempty"`
	ZoneKind    string       `json:"zone_kind,omitempty"`
	DryRun      bool         `json:"dry_run,omitempty"`
	Assets      AssetCatalog `json:"-"`
}

func (s Scene) MarshalJSON() ([]byte, error) {
	type plain Scene
	return json.Marshal(plain(s))
}
