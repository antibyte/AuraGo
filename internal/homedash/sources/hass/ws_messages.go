package hass

import (
	"encoding/json"
	"time"
)

// wsMsg is the envelope of every Home Assistant websocket message the client reads.
type wsMsg struct {
	ID      int64           `json:"id,omitempty"`
	Type    string          `json:"type"`
	Success *bool           `json:"success,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
	Message string          `json:"message,omitempty"`
}

// HAState is one entity state as Home Assistant reports it.
type HAState struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
}

// StateChanged is the payload of a state_changed event.
type StateChanged struct {
	EntityID string   `json:"entity_id"`
	NewState *HAState `json:"new_state"`
	OldState *HAState `json:"old_state"`
}

// AreaEntry is one area registry row.
type AreaEntry struct {
	AreaID string `json:"area_id"`
	Name   string `json:"name"`
}

// DeviceEntry is one device registry row.
type DeviceEntry struct {
	ID     string `json:"id"`
	AreaID string `json:"area_id"`
}

// EntityEntry is one entity registry row.
type EntityEntry struct {
	EntityID   string `json:"entity_id"`
	DeviceID   string `json:"device_id"`
	AreaID     string `json:"area_id"`
	HiddenBy   string `json:"hidden_by"`
	DisabledBy string `json:"disabled_by"`
}

// Registries bundles the three registry listings.
type Registries struct {
	Areas    []AreaEntry
	Devices  []DeviceEntry
	Entities []EntityEntry
}
