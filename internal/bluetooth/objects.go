package bluetooth

import (
	"sort"
	"strings"
)

const (
	bluezService          = "org.bluez"
	bluezAdapterInterface = "org.bluez.Adapter1"
	bluezDeviceInterface  = "org.bluez.Device1"
	bluezBatteryInterface = "org.bluez.Battery1"
	propertiesInterface   = "org.freedesktop.DBus.Properties"
	objectManager         = "org.freedesktop.DBus.ObjectManager"
	agentManager          = "org.bluez.AgentManager1"
	powerStateOffBlocked  = "off-blocked"
)

type objectEventKind int

const (
	objectsReset  objectEventKind = iota // Objects replaces the whole tree
	objectAdded                          // Interfaces were added to Path
	objectRemoved                        // RemovedInterfaces were removed from Path
	objectChanged                        // properties of Interface on Path changed
	objectsLost                          // BlueZ is gone; Reason says why
)

// objectEvent is a platform-neutral BlueZ ObjectManager change. Property values
// are plain Go values: D-Bus variants are unwrapped and object paths are strings.
type objectEvent struct {
	Kind              objectEventKind
	Objects           map[string]map[string]map[string]interface{}
	Path              string
	Interfaces        map[string]map[string]interface{}
	RemovedInterfaces []string
	Interface         string
	Changed           map[string]interface{}
	Invalidated       []string
	Reason            string
}

// objectTree mirrors BlueZ's managed objects: path -> interface -> properties.
type objectTree map[string]map[string]map[string]interface{}

func (t objectTree) apply(event objectEvent) objectTree {
	switch event.Kind {
	case objectsReset:
		next := objectTree{}
		for path, interfaces := range event.Objects {
			next[path] = copyInterfaces(interfaces)
		}
		return next
	case objectsLost:
		return objectTree{}
	case objectAdded:
		if t == nil {
			t = objectTree{}
		}
		interfaces := t[event.Path]
		if interfaces == nil {
			interfaces = map[string]map[string]interface{}{}
			t[event.Path] = interfaces
		}
		for name, properties := range event.Interfaces {
			interfaces[name] = copyProperties(properties)
		}
	case objectRemoved:
		if interfaces := t[event.Path]; interfaces != nil {
			for _, name := range event.RemovedInterfaces {
				delete(interfaces, name)
			}
			if len(interfaces) == 0 {
				delete(t, event.Path)
			}
		}
	case objectChanged:
		properties := t[event.Path][event.Interface]
		if properties == nil {
			return t
		}
		for key, value := range event.Changed {
			properties[key] = value
		}
		for _, key := range event.Invalidated {
			delete(properties, key)
		}
	}
	return t
}

func copyInterfaces(interfaces map[string]map[string]interface{}) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{}, len(interfaces))
	for name, properties := range interfaces {
		out[name] = copyProperties(properties)
	}
	return out
}

func copyProperties(properties map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		out[key] = value
	}
	return out
}

// selectAdapter returns the first powered adapter by object path, otherwise the
// first adapter by object path, or "" when BlueZ reports none.
func (t objectTree) selectAdapter() (string, map[string]interface{}) {
	paths := make([]string, 0, len(t))
	for path, interfaces := range t {
		if _, ok := interfaces[bluezAdapterInterface]; ok {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return "", nil
	}
	sort.Strings(paths)
	for _, path := range paths {
		if propBool(t[path][bluezAdapterInterface], "Powered") {
			return path, t[path][bluezAdapterInterface]
		}
	}
	return paths[0], t[paths[0]][bluezAdapterInterface]
}

func adapterStatusFromProperties(path string, properties map[string]interface{}) AdapterStatus {
	return AdapterStatus{
		Path:                path,
		Address:             strings.ToUpper(propString(properties, "Address")),
		Name:                firstNonEmpty(propString(properties, "Alias"), propString(properties, "Name"), path),
		Powered:             propBool(properties, "Powered"),
		PowerState:          propString(properties, "PowerState"),
		Discoverable:        propBool(properties, "Discoverable"),
		DiscoverableTimeout: propUint32(properties, "DiscoverableTimeout"),
		Discovering:         propBool(properties, "Discovering"),
		Pairable:            propBool(properties, "Pairable"),
	}
}

// devicesOf lists the devices that belong to adapterPath.
func (t objectTree) devicesOf(adapterPath string) []Device {
	devices := make([]Device, 0)
	if adapterPath == "" {
		return devices
	}
	for path, interfaces := range t {
		properties, ok := interfaces[bluezDeviceInterface]
		if !ok {
			continue
		}
		owner := propString(properties, "Adapter")
		if owner != "" && owner != adapterPath {
			continue
		}
		if owner == "" && !strings.HasPrefix(path, adapterPath+"/") {
			continue
		}
		devices = append(devices, deviceFromProperties(properties, interfaces[bluezBatteryInterface]))
	}
	sortDevices(devices)
	return devices
}

func (t objectTree) devicePathByAddress(address string) string {
	for path, interfaces := range t {
		if properties, ok := interfaces[bluezDeviceInterface]; ok && strings.EqualFold(propString(properties, "Address"), address) {
			return path
		}
	}
	return ""
}

func deviceFromProperties(properties, battery map[string]interface{}) Device {
	device := Device{
		Address:   strings.ToUpper(propString(properties, "Address")),
		Name:      propString(properties, "Name"),
		Alias:     propString(properties, "Alias"),
		Paired:    propBool(properties, "Paired"),
		Connected: propBool(properties, "Connected"),
		Trusted:   propBool(properties, "Trusted"),
		UUIDs:     propStrings(properties, "UUIDs"),
		Type:      deviceType(propString(properties, "Icon"), propUint32(properties, "Class")),
	}
	if rssi, ok := propInt16(properties, "RSSI"); ok {
		device.RSSI = &rssi
	}
	if percentage, ok := propUint8(battery, "Percentage"); ok {
		value := int(percentage)
		device.Battery = &value
	}
	device.Audio = isAudioDevice(device)
	return device
}

// deviceType maps BlueZ's Icon hint, or the Class of Device as fallback, to the
// small set of device types the desktop app draws.
func deviceType(icon string, class uint32) string {
	switch icon {
	case "audio-headphones":
		return "headphones"
	case "audio-headset":
		return "headset"
	case "audio-card", "multimedia-player":
		return "speaker"
	case "input-keyboard":
		return "keyboard"
	case "input-mouse", "input-tablet":
		return "mouse"
	case "input-gaming":
		return "gamepad"
	case "phone":
		return "phone"
	case "computer":
		return "computer"
	case "video-display", "camera-video", "camera-photo":
		return "display"
	}
	switch (class >> 8) & 0x1f {
	case 1:
		return "computer"
	case 2:
		return "phone"
	case 4:
		return "speaker"
	case 5:
		if class&0x40 != 0 {
			return "keyboard"
		}
		if class&0x80 != 0 {
			return "mouse"
		}
	}
	return "other"
}

func sortDevices(devices []Device) {
	sort.Slice(devices, func(i, j int) bool {
		left := strings.ToLower(firstNonEmpty(devices[i].Alias, devices[i].Name, devices[i].Address))
		right := strings.ToLower(firstNonEmpty(devices[j].Alias, devices[j].Name, devices[j].Address))
		if left == right {
			return devices[i].Address < devices[j].Address
		}
		return left < right
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func propString(properties map[string]interface{}, key string) string {
	if value, ok := properties[key].(string); ok {
		return value
	}
	return ""
}

func propBool(properties map[string]interface{}, key string) bool {
	value, _ := properties[key].(bool)
	return value
}

func propStrings(properties map[string]interface{}, key string) []string {
	if value, ok := properties[key].([]string); ok {
		return append([]string(nil), value...)
	}
	return nil
}

func propInt16(properties map[string]interface{}, key string) (int16, bool) {
	switch value := properties[key].(type) {
	case int16:
		return value, true
	case int32:
		return int16(value), true
	case int:
		return int16(value), true
	}
	return 0, false
}

func propUint32(properties map[string]interface{}, key string) uint32 {
	switch value := properties[key].(type) {
	case uint32:
		return value
	case uint16:
		return uint32(value)
	case int:
		return uint32(value)
	}
	return 0
}

func propUint8(properties map[string]interface{}, key string) (uint8, bool) {
	switch value := properties[key].(type) {
	case uint8:
		return value, true
	case int:
		return uint8(value), true
	}
	return 0, false
}
