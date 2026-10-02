//go:build linux

package bluetooth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
)

const dbusDaemonInterface = "org.freedesktop.DBus"

// dbusBus is the live session's own system-bus connection. It only watches
// signals and performs the calls the Manager asks for.
type dbusBus struct {
	conn *dbus.Conn
}

func platformBusDialer(_ *slog.Logger) busDialer {
	return func() (bluezBus, error) {
		conn, err := dbus.ConnectSystemBus()
		if err != nil {
			return nil, fmt.Errorf("connect to the system D-Bus: %w", err)
		}
		return &dbusBus{conn: conn}, nil
	}
}

func (b *dbusBus) Watch(ctx context.Context, emit func(objectEvent)) error {
	signals := make(chan *dbus.Signal, 256)
	b.conn.Signal(signals)
	defer b.conn.RemoveSignal(signals)
	for _, match := range [][]dbus.MatchOption{
		{dbus.WithMatchSender(bluezService), dbus.WithMatchInterface(objectManager)},
		{dbus.WithMatchSender(bluezService), dbus.WithMatchInterface(propertiesInterface), dbus.WithMatchMember("PropertiesChanged"), dbus.WithMatchPathNamespace("/org/bluez")},
		{dbus.WithMatchSender(dbusDaemonInterface), dbus.WithMatchInterface(dbusDaemonInterface), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, bluezService)},
	} {
		if err := b.conn.AddMatchSignalContext(ctx, match...); err != nil {
			return fmt.Errorf("subscribe to BlueZ signals: %w", err)
		}
	}
	if err := b.reset(ctx, emit); err != nil {
		emit(objectEvent{Kind: objectsLost, Reason: "BlueZ is not running."})
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case signal, ok := <-signals:
			if !ok {
				return errors.New("D-Bus signal channel closed")
			}
			if signal != nil {
				b.dispatch(ctx, signal, emit)
			}
		}
	}
}

func (b *dbusBus) reset(ctx context.Context, emit func(objectEvent)) error {
	var objects managedObjects
	call := b.conn.Object(bluezService, "/").CallWithContext(ctx, objectManager+".GetManagedObjects", 0)
	if call.Err != nil {
		return call.Err
	}
	if err := call.Store(&objects); err != nil {
		return err
	}
	emit(objectEvent{Kind: objectsReset, Objects: plainObjects(objects)})
	return nil
}

func (b *dbusBus) dispatch(ctx context.Context, signal *dbus.Signal, emit func(objectEvent)) {
	switch signal.Name {
	case objectManager + ".InterfacesAdded":
		var path dbus.ObjectPath
		var interfaces map[string]map[string]dbus.Variant
		if dbus.Store(signal.Body, &path, &interfaces) == nil {
			emit(objectEvent{Kind: objectAdded, Path: string(path), Interfaces: plainInterfaces(interfaces)})
		}
	case objectManager + ".InterfacesRemoved":
		var path dbus.ObjectPath
		var names []string
		if dbus.Store(signal.Body, &path, &names) == nil {
			emit(objectEvent{Kind: objectRemoved, Path: string(path), RemovedInterfaces: names})
		}
	case propertiesInterface + ".PropertiesChanged":
		var iface string
		var changed map[string]dbus.Variant
		var invalidated []string
		if dbus.Store(signal.Body, &iface, &changed, &invalidated) == nil {
			emit(objectEvent{Kind: objectChanged, Path: string(signal.Path), Interface: iface, Changed: plainProperties(changed), Invalidated: invalidated})
		}
	case dbusDaemonInterface + ".NameOwnerChanged":
		var name, oldOwner, newOwner string
		if dbus.Store(signal.Body, &name, &oldOwner, &newOwner) != nil || name != bluezService {
			return
		}
		if newOwner == "" {
			emit(objectEvent{Kind: objectsLost, Reason: "BlueZ stopped."})
			return
		}
		if err := b.reset(ctx, emit); err != nil {
			emit(objectEvent{Kind: objectsLost, Reason: "BlueZ is not running."})
		}
	}
}

func (b *dbusBus) Call(ctx context.Context, path, method string) error {
	return asBusError(b.conn.Object(bluezService, dbus.ObjectPath(path)).CallWithContext(ctx, method, 0).Err)
}

func (b *dbusBus) RemoveDevice(ctx context.Context, adapterPath, devicePath string) error {
	return asBusError(b.conn.Object(bluezService, dbus.ObjectPath(adapterPath)).
		CallWithContext(ctx, bluezAdapterInterface+".RemoveDevice", 0, dbus.ObjectPath(devicePath)).Err)
}

func (b *dbusBus) SetProperty(ctx context.Context, path, iface, name string, value interface{}) error {
	return asBusError(b.conn.Object(bluezService, dbus.ObjectPath(path)).
		CallWithContext(ctx, propertiesInterface+".Set", 0, iface, name, dbus.MakeVariant(value)).Err)
}

func (b *dbusBus) Close() error {
	return b.conn.Close()
}

func plainValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case dbus.Variant:
		return plainValue(typed.Value())
	case dbus.ObjectPath:
		return string(typed)
	case []dbus.ObjectPath:
		out := make([]string, len(typed))
		for i, path := range typed {
			out[i] = string(path)
		}
		return out
	}
	return value
}

func plainProperties(properties map[string]dbus.Variant) map[string]interface{} {
	out := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		out[key] = plainValue(value.Value())
	}
	return out
}

func plainInterfaces(interfaces map[string]map[string]dbus.Variant) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{}, len(interfaces))
	for name, properties := range interfaces {
		out[name] = plainProperties(properties)
	}
	return out
}

func plainObjects(objects managedObjects) map[string]map[string]map[string]interface{} {
	out := make(map[string]map[string]map[string]interface{}, len(objects))
	for path, interfaces := range objects {
		out[string(path)] = plainInterfaces(interfaces)
	}
	return out
}

// asBusError converts godbus error replies into the platform-neutral busError.
func asBusError(err error) error {
	if err == nil {
		return nil
	}
	var value dbus.Error
	if errors.As(err, &value) {
		return &busError{Name: value.Name, Message: dbusErrorMessage(value.Body)}
	}
	var pointer *dbus.Error
	if errors.As(err, &pointer) && pointer != nil {
		return &busError{Name: pointer.Name, Message: dbusErrorMessage(pointer.Body)}
	}
	return err
}

func dbusErrorMessage(body []interface{}) string {
	if len(body) > 0 {
		if text, ok := body[0].(string); ok {
			return text
		}
	}
	return ""
}
