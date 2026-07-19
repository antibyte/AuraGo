//go:build linux

package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakeBlueZConnection struct {
	mu      sync.Mutex
	objects managedObjects
	calls   []string
	exports []interface{}
}

func (f *fakeBlueZConnection) Object(_ string, path dbus.ObjectPath) dbus.BusObject {
	return &fakeBlueZObject{conn: f, path: path}
}

func (f *fakeBlueZConnection) Export(value interface{}, _ dbus.ObjectPath, _ string) error {
	f.mu.Lock()
	f.exports = append(f.exports, value)
	f.mu.Unlock()
	return nil
}

func (f *fakeBlueZConnection) Close() error {
	return nil
}

func (f *fakeBlueZConnection) record(method string) {
	f.mu.Lock()
	f.calls = append(f.calls, method)
	f.mu.Unlock()
}

func (f *fakeBlueZConnection) hasCall(method string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if call == method {
			return true
		}
	}
	return false
}

type fakeBlueZObject struct {
	conn *fakeBlueZConnection
	path dbus.ObjectPath
}

func (o *fakeBlueZObject) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	return o.CallWithContext(context.Background(), method, flags, args...)
}

func (o *fakeBlueZObject) CallWithContext(_ context.Context, method string, _ dbus.Flags, _ ...interface{}) *dbus.Call {
	o.conn.record(method)
	if method == objectManager+".GetManagedObjects" {
		return &dbus.Call{Body: []interface{}{o.conn.objects}}
	}
	return &dbus.Call{}
}

func (o *fakeBlueZObject) Go(method string, flags dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	call := o.Call(method, flags, args...)
	if ch != nil {
		ch <- call
	}
	return call
}

func (o *fakeBlueZObject) GoWithContext(ctx context.Context, method string, flags dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	call := o.CallWithContext(ctx, method, flags, args...)
	if ch != nil {
		ch <- call
	}
	return call
}

func (*fakeBlueZObject) AddMatchSignal(string, string, ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

func (*fakeBlueZObject) RemoveMatchSignal(string, string, ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

func (*fakeBlueZObject) GetProperty(string) (dbus.Variant, error) {
	return dbus.Variant{}, fmt.Errorf("not implemented")
}

func (*fakeBlueZObject) StoreProperty(string, interface{}) error {
	return fmt.Errorf("not implemented")
}

func (*fakeBlueZObject) SetProperty(string, interface{}) error {
	return fmt.Errorf("not implemented")
}

func (*fakeBlueZObject) Destination() string {
	return bluezService
}

func (o *fakeBlueZObject) Path() dbus.ObjectPath {
	return o.path
}

func fakeBlueZObjects(paired bool) managedObjects {
	return managedObjects{
		dbus.ObjectPath("/org/bluez/hci0"): {
			bluezAdapterInterface: {
				"Address": dbus.MakeVariant("00:11:22:33:44:55"),
				"Alias":   dbus.MakeVariant("Adapter"),
				"Powered": dbus.MakeVariant(true),
			},
		},
		dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF"): {
			bluezDeviceInterface: {
				"Address": dbus.MakeVariant("AA:BB:CC:DD:EE:FF"),
				"Alias":   dbus.MakeVariant("Speaker"),
				"Paired":  dbus.MakeVariant(paired),
				"UUIDs":   dbus.MakeVariant([]string{"0000110b-0000-1000-8000-00805f9b34fb"}),
			},
		},
	}
}

func newFakeBlueZAdapter(connection *fakeBlueZConnection) *bluezAdapter {
	return &bluezAdapter{
		logger: slog.Default(),
		connect: func() (bluezConnection, error) {
			return connection, nil
		},
	}
}

func TestBlueZProbeFindsPoweredAdapter(t *testing.T) {
	connection := &fakeBlueZConnection{objects: fakeBlueZObjects(false)}
	status, err := newFakeBlueZAdapter(connection).Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !status.Powered || status.Name != "Adapter" {
		t.Fatalf("adapter status = %+v", status)
	}
}

func TestBlueZDiscoverAlwaysStopsDiscovery(t *testing.T) {
	connection := &fakeBlueZConnection{objects: fakeBlueZObjects(false)}
	devices, err := newFakeBlueZAdapter(connection).Discover(context.Background(), time.Millisecond)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(devices) != 1 || devices[0].Address != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("devices = %+v", devices)
	}
	if !connection.hasCall(bluezAdapterInterface+".StartDiscovery") ||
		!connection.hasCall(bluezAdapterInterface+".StopDiscovery") {
		t.Fatalf("discovery calls = %v", connection.calls)
	}
}

func TestBlueZPairAlwaysCleansUpAgent(t *testing.T) {
	connection := &fakeBlueZConnection{objects: fakeBlueZObjects(false)}
	if err := newFakeBlueZAdapter(connection).Pair(context.Background(), "AA:BB:CC:DD:EE:FF", "1234"); err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if !connection.hasCall(agentManager+".RegisterAgent") ||
		!connection.hasCall(agentManager+".UnregisterAgent") {
		t.Fatalf("pairing calls = %v", connection.calls)
	}
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if len(connection.exports) != 2 || connection.exports[0] == nil || connection.exports[1] != nil {
		t.Fatalf("pairing exports = %#v", connection.exports)
	}
}
