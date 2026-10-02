package bluetooth

import "testing"

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
