//go:build linux

package bluetooth

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestPlainValueUnwrapsVariantsAndPaths(t *testing.T) {
	if got := plainValue(dbus.MakeVariant(dbus.ObjectPath("/org/bluez/hci0"))); got != "/org/bluez/hci0" {
		t.Fatalf("object path = %#v", got)
	}
	properties := plainProperties(map[string]dbus.Variant{"Powered": dbus.MakeVariant(true), "RSSI": dbus.MakeVariant(int16(-50))})
	if properties["Powered"] != true || properties["RSSI"] != int16(-50) {
		t.Fatalf("properties = %#v", properties)
	}
}

func TestAsBusErrorKeepsTheBlueZName(t *testing.T) {
	err := asBusError(dbus.Error{Name: "org.bluez.Error.InProgress", Body: []interface{}{"Operation already in progress"}})
	if ErrorCode(mapBlueZError("connect", err)) != ErrorOperationBusy {
		t.Fatalf("mapped = %v", mapBlueZError("connect", err))
	}
}
