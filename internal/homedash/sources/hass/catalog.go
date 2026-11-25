package hass

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// SourceID is the HomeDash source id of Home Assistant; entity ids are "ha:<domain>.<object_id>".
const SourceID homedash.SourceID = "ha"

// Keys of the homedash.SourceSettings the server builds (internal/server/homedash_sources.go).
const (
	ParamURL            = "url"
	ParamToken          = "token"
	ParamReadOnly       = "readonly"
	ListAllowedServices = "allowed_services"
	ListBlockedServices = "blocked_services"
)

func domainOf(native string) (string, bool) {
	if !NativeIDValid(native) {
		return "", false
	}
	d, _, _ := strings.Cut(native, ".")
	return d, true
}

// iconHint maps a domain and device class to the fixed HomeDash icon list (decision D9). "" lets the
// frontend use the kind icon. Home Assistant's own mdi: icons are never forwarded.
func iconHint(domain, deviceClass string, numeric bool) string {
	switch domain {
	case "light":
		return "bulb"
	case "switch":
		if deviceClass == "outlet" {
			return "outlet"
		}
		return "power"
	case "input_boolean":
		return "power"
	case "scene":
		return "scene"
	case "script":
		return "script"
	case "button", "input_button":
		return "button"
	case "select", "input_select":
		return "list"
	case "number", "input_number":
		return "number"
	case "climate":
		return "thermometer"
	case "lock":
		return "lock"
	case "sensor":
		switch deviceClass {
		case "temperature":
			return "thermometer"
		case "humidity", "moisture":
			return "droplet"
		}
		if numeric {
			return "gauge"
		}
	}
	return ""
}

// registryIndex resolves areas and the hidden/disabled filter from the three registries.
type registryIndex struct {
	areaName map[string]string      // area id -> name
	device   map[string]string      // device id -> area id
	entity   map[string]EntityEntry // entity id -> registry entry
}

func indexRegistries(reg Registries) registryIndex {
	ix := registryIndex{areaName: map[string]string{}, device: map[string]string{}, entity: map[string]EntityEntry{}}
	for _, a := range reg.Areas {
		ix.areaName[a.AreaID] = a.Name
	}
	for _, d := range reg.Devices {
		ix.device[d.ID] = d.AreaID
	}
	for _, e := range reg.Entities {
		ix.entity[e.EntityID] = e
	}
	return ix
}

func (ix registryIndex) hidden(entityID string) bool {
	e, ok := ix.entity[entityID]
	return ok && (e.HiddenBy != "" || e.DisabledBy != "")
}

// area returns the entity registry area, else the device's area, and the area's display name (the
// id when the area registry has no name for it).
func (ix registryIndex) area(entityID string) (id, name string) {
	e := ix.entity[entityID]
	id = e.AreaID
	if id == "" && e.DeviceID != "" {
		id = ix.device[e.DeviceID]
	}
	if id == "" {
		return "", ""
	}
	if name = ix.areaName[id]; name == "" {
		name = id
	}
	return id, name
}

var (
	onOffDomains   = map[string]bool{"switch": true, "input_boolean": true, "light": true, "fan": true, "humidifier": true, "binary_sensor": true, "script": true, "automation": true}
	numericDomains = map[string]bool{"sensor": true, "number": true, "input_number": true}
)

// mapState turns a Home Assistant state into a HomeDash state (contract section 2): on/off domains
// carry a bool, numeric domains a finite number, others the state text; "unavailable" is
// available:false without a value, "unknown" is available without a value. UpdatedAt is
// last_updated (else last_changed) in UTC.
func mapState(domain string, st HAState) homedash.State {
	_, attrs := FilterAttributes(st.Attributes)
	at := st.LastUpdated
	if at.IsZero() {
		at = st.LastChanged
	}
	out := homedash.State{Text: st.State, Attributes: attrs, Available: true, UpdatedAt: at.UTC()}
	switch st.State {
	case "unavailable":
		out.Available = false
		return out
	case "unknown", "":
		return out
	}
	switch {
	case onOffDomains[domain] && (st.State == "on" || st.State == "off"):
		out.Value = st.State == "on"
	case numericDomains[domain]:
		if f, err := strconv.ParseFloat(st.State, 64); err == nil && finite(f) {
			out.Value = f
		} else {
			out.Value = st.State
		}
	default:
		out.Value = st.State
	}
	return out
}

// entityFor maps one state to a HomeDash entity; ok is false for invalid ids, excluded domains and
// registry-hidden or -disabled entities.
func entityFor(cfg tools.HAConfig, st HAState, ix registryIndex) (homedash.Entity, bool) {
	domain, ok := domainOf(st.EntityID)
	if !ok {
		return homedash.Entity{}, false
	}
	spec, ok := MapDomain(domain)
	if !ok || ix.hidden(st.EntityID) {
		return homedash.Entity{}, false
	}
	name, _ := FilterAttributes(st.Attributes)
	dc, _ := st.Attributes["device_class"].(string)
	unit, _ := st.Attributes["unit_of_measurement"].(string)
	caps := Capabilities(cfg, st.EntityID, st.State, st.Attributes)
	numeric := slices.ContainsFunc(caps, func(c homedash.CapabilitySpec) bool { return c.Name == homedash.CapNumeric })
	areaID, area := ix.area(st.EntityID)
	e := homedash.Entity{ID: homedash.MakeEntityID(SourceID, st.EntityID), Source: SourceID, Kind: spec.Kind, Name: name,
		Area: area, AreaID: areaID, DeviceClass: dc, Unit: unit, IconHint: iconHint(domain, dc, numeric), Capabilities: caps,
		ReadOnly: true, State: mapState(domain, st)}
	blocker := ""
	for _, c := range caps {
		if info, ok := homedash.LookupCapability(c.Name); ok && info.Command {
			if c.Allowed {
				e.ReadOnly = false
			} else if blocker == "" {
				blocker = c.BlockedBy
			}
		}
	}
	if e.ReadOnly {
		e.ReadOnlyReason = blocker // the first blocked_by when no command is allowed
	}
	if spec.Kind == homedash.KindCamera {
		e.Media = &homedash.MediaInfo{Snapshot: true}
	}
	return e, true
}

// buildCatalog maps every known state, sorted by entity id, at most homedash.MaxCatalogEntities.
func buildCatalog(cfg tools.HAConfig, states map[string]HAState, ix registryIndex) []homedash.Entity {
	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]homedash.Entity, 0, min(len(ids), homedash.MaxCatalogEntities))
	for _, id := range ids {
		if len(out) >= homedash.MaxCatalogEntities {
			break
		}
		if e, ok := entityFor(cfg, states[id], ix); ok {
			out = append(out, e)
		}
	}
	return out
}
