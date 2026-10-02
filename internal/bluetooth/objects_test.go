package bluetooth

import "testing"

func TestObjectTreeSelectsFirstPoweredAdapterDeterministically(t *testing.T) {
	tree := objectTree{
		"/org/bluez/hci1": {bluezAdapterInterface: {"Powered": true, "Alias": "second"}},
		"/org/bluez/hci0": {bluezAdapterInterface: {"Powered": false, "Alias": "first"}},
	}
	if path, _ := tree.selectAdapter(); path != "/org/bluez/hci1" {
		t.Fatalf("selected %q, want powered hci1", path)
	}
	tree["/org/bluez/hci1"][bluezAdapterInterface]["Powered"] = false
	if path, _ := tree.selectAdapter(); path != "/org/bluez/hci0" {
		t.Fatalf("selected %q, want first path hci0", path)
	}
}

func TestObjectTreeAppliesIncrementalEvents(t *testing.T) {
	tree := objectTree{}.apply(objectEvent{Kind: objectsReset, Objects: map[string]map[string]map[string]interface{}{
		"/org/bluez/hci0": {bluezAdapterInterface: {"Powered": true}},
	}})
	tree = tree.apply(objectEvent{Kind: objectAdded, Path: "/org/bluez/hci0/dev_1", Interfaces: map[string]map[string]interface{}{
		bluezDeviceInterface: {"Address": "aa:bb:cc:dd:ee:01", "Name": "Pad", "Adapter": "/org/bluez/hci0", "RSSI": int16(-40)},
	}})
	tree = tree.apply(objectEvent{Kind: objectChanged, Path: "/org/bluez/hci0/dev_1", Interface: bluezDeviceInterface,
		Changed: map[string]interface{}{"Connected": true}, Invalidated: []string{"RSSI"}})
	devices := tree.devicesOf("/org/bluez/hci0")
	if len(devices) != 1 || devices[0].Address != "AA:BB:CC:DD:EE:01" || !devices[0].Connected || devices[0].RSSI != nil {
		t.Fatalf("devices after change = %+v", devices)
	}
	tree = tree.apply(objectEvent{Kind: objectRemoved, Path: "/org/bluez/hci0/dev_1", RemovedInterfaces: []string{bluezDeviceInterface}})
	if devices := tree.devicesOf("/org/bluez/hci0"); len(devices) != 0 {
		t.Fatalf("device survived removal: %+v", devices)
	}
	if _, ok := tree["/org/bluez/hci0/dev_1"]; ok {
		t.Fatal("empty object path must be dropped")
	}
}

func TestObjectTreeIgnoresChangesForUnknownObjects(t *testing.T) {
	tree := objectTree{}.apply(objectEvent{Kind: objectChanged, Path: "/org/bluez/hci9", Interface: bluezAdapterInterface,
		Changed: map[string]interface{}{"Powered": true}})
	if len(tree) != 0 {
		t.Fatalf("unknown object created: %+v", tree)
	}
}

func TestDevicesBelongToTheSelectedAdapter(t *testing.T) {
	tree := objectTree{
		"/org/bluez/hci0":       {bluezAdapterInterface: {"Powered": true}},
		"/org/bluez/hci1":       {bluezAdapterInterface: {"Powered": true}},
		"/org/bluez/hci1/dev_2": {bluezDeviceInterface: {"Address": "AA:BB:CC:DD:EE:02", "Adapter": "/org/bluez/hci1"}},
	}
	if devices := tree.devicesOf("/org/bluez/hci0"); len(devices) != 0 {
		t.Fatalf("foreign adapter device leaked: %+v", devices)
	}
}

func TestDeviceMappingCarriesTypeBatteryAndAudio(t *testing.T) {
	device := deviceFromProperties(map[string]interface{}{
		"Address": "aa:bb:cc:dd:ee:03", "Alias": "Phones", "Icon": "audio-headphones", "Paired": true,
		"UUIDs": []string{"0000110b-0000-1000-8000-00805f9b34fb"},
	}, map[string]interface{}{"Percentage": byte(64)})
	if device.Type != "headphones" || device.Battery == nil || *device.Battery != 64 || !device.Audio {
		t.Fatalf("device = %+v", device)
	}
}

func TestDeviceTypeFallsBackToClass(t *testing.T) {
	for _, tc := range []struct {
		icon  string
		class uint32
		want  string
	}{
		{"input-keyboard", 0, "keyboard"},
		{"", 0x5a020c, "phone"},
		{"", 0x240404, "speaker"},
		{"", 0x002540, "keyboard"},
		{"", 0x002580, "mouse"},
		{"", 0, "other"},
	} {
		if got := deviceType(tc.icon, tc.class); got != tc.want {
			t.Errorf("deviceType(%q, %#x) = %q, want %q", tc.icon, tc.class, got, tc.want)
		}
	}
}
