package bluetooth

import (
	"fmt"
	"strings"
	"testing"
)

const (
	testHeadsetAddress = "AA:BB:CC:DD:EE:FF"
	handsfreeUUID      = "0000111e-0000-1000-8000-00805f9b34fb"
	headsetUUID        = "00001108-0000-1000-8000-00805f9b34fb"
	a2dpSinkUUID       = "0000110b-0000-1000-8000-00805f9b34fb"
)

func TestHasMicrophoneRecognizesHandsfreeAndHeadset(t *testing.T) {
	for _, tc := range []struct {
		uuids []string
		want  bool
	}{
		{[]string{a2dpSinkUUID}, false},
		{[]string{a2dpSinkUUID, handsfreeUUID}, true},
		{[]string{"00001108-0000-1000-8000-00805F9B34FB"}, true},
		// Handsfree Audio Gateway: the remote side is a phone, not a headset.
		{[]string{"0000111f-0000-1000-8000-00805f9b34fb"}, false},
	} {
		if got := hasMicrophone(Device{UUIDs: tc.uuids}); got != tc.want {
			t.Fatalf("hasMicrophone(%v) = %v, want %v", tc.uuids, got, tc.want)
		}
	}
	device := deviceFromProperties(map[string]interface{}{
		"Address": "aa:bb:cc:dd:ee:ff", "Paired": true,
		"UUIDs": []string{a2dpSinkUUID, headsetUUID},
	}, nil)
	if !device.Microphone || !device.Audio {
		t.Fatalf("device = %+v, want audio with microphone", device)
	}
}

// headsetDump renders a pw-dump snapshot of the test headset in one profile.
// Only the HFP profiles have a microphone (bluez_input) node.
func headsetDump(profile string) []byte {
	indexes := map[string]int{"off": 0, "a2dp-sink": 1, "headset-head-unit-cvsd": 2, "headset-head-unit": 3, "headset-head-unit-msbc": 4}
	nodes := ""
	switch {
	case strings.HasPrefix(profile, "headset-head-unit"):
		nodes = `,
	{"id": 80, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Source", "node.name": "bluez_input.AA_BB_CC_DD_EE_FF.0", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}}},
	{"id": 81, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "bluez_output.AA_BB_CC_DD_EE_FF.0", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}}}`
	case profile == "a2dp-sink":
		nodes = `,
	{"id": 82, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "bluez_output.AA_BB_CC_DD_EE_FF.1", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}}}`
	}
	return []byte(fmt.Sprintf(`[
	{"id": 50, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "alsa_output.pci"}}},
	{"id": 60, "type": "PipeWire:Interface:Device", "info": {"props": {"device.api": "bluez5", "api.bluez5.address": "11:22:33:44:55:66"}, "params": {"EnumProfile": [{"index": 2, "name": "headset-head-unit", "priority": 9, "available": "yes"}], "Profile": [{"index": 2, "name": "headset-head-unit"}]}}},
	{"id": 71, "type": "PipeWire:Interface:Device", "info": {"props": {"device.api": "bluez5", "device.name": "bluez_card.AA_BB_CC_DD_EE_FF", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}, "params": {
		"EnumProfile": [
			{"index": 0, "name": "off", "priority": 0, "available": "yes"},
			{"index": 1, "name": "a2dp-sink", "priority": 16, "available": "yes"},
			{"index": 2, "name": "headset-head-unit-cvsd", "priority": 1, "available": "yes"},
			{"index": 3, "name": "headset-head-unit", "priority": 2, "available": "yes"},
			{"index": 4, "name": "headset-head-unit-msbc", "priority": 3, "available": "no"}
		],
		"Profile": [{"index": %d, "name": %q}]
	}}}%s
]`, indexes[profile], profile, nodes))
}

const noMicrophoneDump = `[
	{"id": 71, "type": "PipeWire:Interface:Device", "info": {"props": {"device.api": "bluez5", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}, "params": {
		"EnumProfile": [{"index": 0, "name": "off", "priority": 0, "available": "yes"}, {"index": 1, "name": "a2dp-sink", "priority": 16, "available": "yes"}],
		"Profile": [{"index": 1, "name": "a2dp-sink"}]}}}
]`

func TestParseHeadsetGraphFindsDeviceProfilesAndNodes(t *testing.T) {
	graph, found, err := parseHeadsetGraph(headsetDump("headset-head-unit"), "aa:bb:cc:dd:ee:ff")
	if err != nil || !found {
		t.Fatalf("parseHeadsetGraph found=%v err=%v", found, err)
	}
	if graph.DeviceID != 71 || graph.Current.Name != "headset-head-unit" {
		t.Fatalf("graph = %+v", graph)
	}
	if graph.Source != "bluez_input.AA_BB_CC_DD_EE_FF.0" || graph.Sink != "bluez_output.AA_BB_CC_DD_EE_FF.0" {
		t.Fatalf("nodes = %q %q", graph.Source, graph.Sink)
	}
	// mSBC is unavailable here, so the next best HFP profile wins over CVSD.
	if best, ok := graph.bestHeadsetProfile(); !ok || best.Index != 3 {
		t.Fatalf("best profile = %+v ok=%v", best, ok)
	}
	if profile, ok := graph.profileByName("a2dp-sink"); !ok || profile.Index != 1 {
		t.Fatalf("a2dp-sink = %+v ok=%v", profile, ok)
	}

	a2dp, found, err := parseHeadsetGraph(headsetDump("a2dp-sink"), testHeadsetAddress)
	if err != nil || !found || a2dp.Source != "" || a2dp.Sink != "bluez_output.AA_BB_CC_DD_EE_FF.1" || isHeadsetProfile(a2dp.Current.Name) {
		t.Fatalf("a2dp graph = %+v found=%v err=%v", a2dp, found, err)
	}
	if _, found, _ := parseHeadsetGraph(headsetDump("a2dp-sink"), "AA:BB:CC:DD:EE:00"); found {
		t.Fatal("another address must not match")
	}
	if graph, _, _ := parseHeadsetGraph([]byte(noMicrophoneDump), testHeadsetAddress); func() bool { _, ok := graph.bestHeadsetProfile(); return ok }() {
		t.Fatal("a device without HFP profiles has no headset profile")
	}
	if _, _, err := parseHeadsetGraph([]byte("not json"), testHeadsetAddress); err == nil {
		t.Fatal("invalid JSON must fail")
	}
}
