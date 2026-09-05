//go:build linux

package meshcore

import (
	"context"
	"fmt"
	"github.com/godbus/dbus/v5"
	"io"
	"strings"
	"sync"
	"time"
)

const bleRX = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"
const bleTX = "6e400003-b5a3-f393-e0a9-e50e24dcca9e"

type bleLink struct {
	conn           *dbus.Conn
	device, rx, tx dbus.ObjectPath
	signals        chan *dbus.Signal
	done           chan struct{}
	once           sync.Once
	mtu            uint16
}

func openBLE(parent context.Context, address string) (frameLink, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("meshcore_bluez_unavailable")
	}
	l := &bleLink{conn: conn, signals: make(chan *dbus.Signal, 128), done: make(chan struct{})}
	ok := false
	defer func() {
		if !ok {
			l.Close()
		}
	}()
	get := func() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
		var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
		err := conn.Object("org.bluez", "/").CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
		return objects, err
	}
	objects, err := get()
	if err != nil {
		return nil, err
	}
	for path, ifs := range objects {
		d := ifs["org.bluez.Device1"]
		if strings.EqualFold(fmt.Sprint(d["Address"].Value()), address) {
			if d["Paired"].Value() != true {
				return nil, fmt.Errorf("meshcore_pairing_required")
			}
			l.device = path
			break
		}
	}
	if l.device == "" {
		return nil, fmt.Errorf("meshcore_device_not_found")
	}
	if err := conn.Object("org.bluez", l.device).CallWithContext(ctx, "org.bluez.Device1.Connect", 0).Err; err != nil {
		return nil, fmt.Errorf("meshcore_ble_connect_failed")
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		objects, err = get()
		if err != nil {
			return nil, err
		}
		for path, ifs := range objects {
			if !strings.HasPrefix(string(path), string(l.device)+"/") {
				continue
			}
			ch := ifs["org.bluez.GattCharacteristic1"]
			uuid, _ := ch["UUID"].Value().(string)
			switch strings.ToLower(uuid) {
			case bleRX:
				l.rx = path
				if mtu, yes := ch["MTU"].Value().(uint16); yes {
					l.mtu = mtu
				}
			case bleTX:
				l.tx = path
			}
		}
		if l.rx != "" && l.tx != "" {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("meshcore_gatt_unavailable")
		case <-tick.C:
		}
	}
	if l.mtu < 176 {
		return nil, fmt.Errorf("meshcore_ble_mtu_too_small")
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged"), dbus.WithMatchSender("org.bluez")); err != nil {
		return nil, err
	}
	conn.Signal(l.signals)
	if err := conn.Object("org.bluez", l.tx).CallWithContext(ctx, "org.bluez.GattCharacteristic1.StartNotify", 0).Err; err != nil {
		return nil, fmt.Errorf("meshcore_notify_failed")
	}
	ok = true
	return l, nil
}
func (l *bleLink) ReadFrame() ([]byte, error) {
	for {
		select {
		case <-l.done:
			return nil, io.EOF
		case s, ok := <-l.signals:
			if !ok || s == nil {
				return nil, io.EOF
			}
			if len(s.Body) != 3 {
				continue
			}
			props, ok := s.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			if s.Path == l.device && props["Connected"].Value() == false {
				return nil, io.EOF
			}
			if s.Path != l.tx {
				continue
			}
			if b, ok := props["Value"].Value().([]byte); ok {
				// ESP32 can truncate a notification at MTU-3. Never execute ambiguous text.
				if !validBLENotification(b, l.mtu) {
					return nil, fmt.Errorf("meshcore_ble_frame_truncated")
				}
				return append([]byte(nil), b...), nil
			}
		}
	}
}
func (l *bleLink) WriteFrame(b []byte) error {
	if len(b) > int(l.mtu)-3 || len(b) > maxFrame {
		return fmt.Errorf("meshcore_ble_mtu_too_small")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.conn.Object("org.bluez", l.rx).CallWithContext(ctx, "org.bluez.GattCharacteristic1.WriteValue", 0, b, map[string]dbus.Variant{"type": dbus.MakeVariant("request")}).Err
}
func (l *bleLink) Close() error {
	l.once.Do(func() {
		close(l.done)
		if l.tx != "" {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			l.conn.Object("org.bluez", l.tx).CallWithContext(ctx, "org.bluez.GattCharacteristic1.StopNotify", 0)
			cancel()
		}
		l.conn.RemoveSignal(l.signals)
		l.conn.Close()
	})
	return nil
}
