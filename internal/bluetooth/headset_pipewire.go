package bluetooth

import (
	"encoding/json"
	"fmt"
	"strings"
)

// headsetProfile is one entry of a Bluez device's EnumProfile or Profile param.
type headsetProfile struct {
	Index     int         `json:"index"`
	Name      string      `json:"name"`
	Priority  int         `json:"priority"`
	Available interface{} `json:"available"`
}

// headsetGraph is the PipeWire view of one Bluetooth device.
type headsetGraph struct {
	DeviceID int
	Current  headsetProfile
	Profiles []headsetProfile
	// Source and Sink are node.name values; Source exists only in HFP.
	Source string
	Sink   string
}

type pipeWireGraphObject struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Info struct {
		Props  map[string]interface{}     `json:"props"`
		Params map[string]json.RawMessage `json:"params"`
	} `json:"info"`
}

func isHeadsetProfile(name string) bool {
	return strings.HasPrefix(name, "headset-head-unit")
}

func profileAvailable(profile headsetProfile) bool {
	switch value := profile.Available.(type) {
	case string:
		return value != "no"
	case bool:
		return value
	default:
		return true
	}
}

// bestHeadsetProfile picks the available HFP/HSP profile with the highest
// priority; PipeWire ranks mSBC above CVSD.
func (g headsetGraph) bestHeadsetProfile() (headsetProfile, bool) {
	var best headsetProfile
	found := false
	for _, profile := range g.Profiles {
		if !isHeadsetProfile(profile.Name) || !profileAvailable(profile) {
			continue
		}
		if !found || profile.Priority > best.Priority {
			best, found = profile, true
		}
	}
	return best, found
}

func (g headsetGraph) profileByName(name string) (headsetProfile, bool) {
	for _, profile := range g.Profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return headsetProfile{}, false
}

// parseHeadsetGraph finds the Bluez device of address and its audio nodes in
// pw-dump output. found is false when PipeWire does not know the device.
func parseHeadsetGraph(raw []byte, address string) (headsetGraph, bool, error) {
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return headsetGraph{}, false, err
	}
	var objects []pipeWireGraphObject
	if err := json.Unmarshal(raw, &objects); err != nil {
		return headsetGraph{}, false, fmt.Errorf("parse PipeWire objects: %w", err)
	}
	graph := headsetGraph{DeviceID: -1}
	for _, object := range objects {
		props := object.Info.Props
		if !bluezAddressMatches(props, normalized) {
			continue
		}
		switch object.Type {
		case "PipeWire:Interface:Device":
			graph.DeviceID = object.ID
			graph.Profiles = decodeHeadsetProfiles(object.Info.Params["EnumProfile"])
			if current := decodeHeadsetProfiles(object.Info.Params["Profile"]); len(current) > 0 {
				graph.Current = current[0]
			}
		case "PipeWire:Interface:Node":
			switch stringProperty(props, "media.class") {
			case "Audio/Source":
				graph.Source = stringProperty(props, "node.name")
			case "Audio/Sink":
				graph.Sink = stringProperty(props, "node.name")
			}
		}
	}
	return graph, graph.DeviceID >= 0, nil
}

// bluezAddressMatches prefers the explicit address properties and falls back
// to node/device names such as bluez_input.AA_BB_CC_DD_EE_FF.0.
func bluezAddressMatches(props map[string]interface{}, normalized string) bool {
	for _, key := range []string{"api.bluez5.address", "bluez5.address"} {
		if value := stringProperty(props, key); value != "" {
			return compactAddress(value) == compactAddress(normalized)
		}
	}
	name := strings.ToUpper(firstNonEmpty(stringProperty(props, "node.name"), stringProperty(props, "device.name")))
	return name != "" && strings.Contains(name, strings.ReplaceAll(normalized, ":", "_"))
}

func decodeHeadsetProfiles(raw json.RawMessage) []headsetProfile {
	var profiles []headsetProfile
	if len(raw) == 0 || json.Unmarshal(raw, &profiles) != nil {
		return nil
	}
	return profiles
}
