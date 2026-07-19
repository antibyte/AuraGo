//go:build linux

package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	bluezService          = "org.bluez"
	bluezAdapterInterface = "org.bluez.Adapter1"
	bluezDeviceInterface  = "org.bluez.Device1"
	propertiesInterface   = "org.freedesktop.DBus.Properties"
	objectManager         = "org.freedesktop.DBus.ObjectManager"
	agentManager          = "org.bluez.AgentManager1"
)

type bluezAdapter struct {
	logger  *slog.Logger
	connect func() (bluezConnection, error)
}

type managedObjects map[dbus.ObjectPath]map[string]map[string]dbus.Variant

type bluezConnection interface {
	Object(string, dbus.ObjectPath) dbus.BusObject
	Export(interface{}, dbus.ObjectPath, string) error
	Close() error
}

func platformSupported() bool {
	return true
}

func newPlatformAdapter(logger *slog.Logger) platformAdapter {
	return &bluezAdapter{logger: logger, connect: systemBus}
}

func systemBus() (bluezConnection, error) {
	// Use a private connection because every operation owns and closes its
	// connection. Closing godbus.SystemBus's shared singleton would disrupt
	// concurrent Bluetooth operations.
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect to the system D-Bus: %w", err)
	}
	return conn, nil
}

func getManagedObjects(ctx context.Context, conn bluezConnection) (managedObjects, error) {
	var objects managedObjects
	call := conn.Object(bluezService, dbus.ObjectPath("/")).CallWithContext(ctx, objectManager+".GetManagedObjects", 0)
	if call.Err != nil {
		return nil, fmt.Errorf("query BlueZ objects: %w", call.Err)
	}
	if err := call.Store(&objects); err != nil {
		return nil, fmt.Errorf("decode BlueZ objects: %w", err)
	}
	return objects, nil
}

func (a *bluezAdapter) Probe(ctx context.Context) (AdapterStatus, error) {
	conn, err := a.connect()
	if err != nil {
		return AdapterStatus{}, err
	}
	defer conn.Close()
	objects, err := getManagedObjects(ctx, conn)
	if err != nil {
		return AdapterStatus{}, fmt.Errorf("BlueZ service is not reachable: %w", err)
	}
	var unpowered *AdapterStatus
	for path, interfaces := range objects {
		properties, ok := interfaces[bluezAdapterInterface]
		if !ok {
			continue
		}
		status := AdapterStatus{
			Path:    string(path),
			Address: variantString(properties, "Address"),
			Name:    firstNonEmpty(variantString(properties, "Alias"), variantString(properties, "Name"), string(path)),
			Powered: variantBool(properties, "Powered"),
		}
		if status.Powered {
			return status, nil
		}
		copyStatus := status
		unpowered = &copyStatus
	}
	if unpowered != nil {
		return *unpowered, fmt.Errorf("BlueZ is running, but no Bluetooth adapter is powered on")
	}
	return AdapterStatus{}, fmt.Errorf("BlueZ is running, but no Bluetooth adapter was found")
}

func (a *bluezAdapter) List(ctx context.Context) ([]Device, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	objects, err := getManagedObjects(ctx, conn)
	if err != nil {
		return nil, err
	}
	return devicesFromManagedObjects(objects), nil
}

func (a *bluezAdapter) Discover(ctx context.Context, timeout time.Duration) ([]Device, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	objects, err := getManagedObjects(ctx, conn)
	if err != nil {
		return nil, err
	}
	adapterPath, err := poweredAdapterPath(objects)
	if err != nil {
		return nil, err
	}
	adapter := conn.Object(bluezService, adapterPath)
	if call := adapter.CallWithContext(ctx, bluezAdapterInterface+".StartDiscovery", 0); call.Err != nil {
		return nil, fmt.Errorf("start BlueZ discovery: %w", call.Err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if call := adapter.CallWithContext(stopCtx, bluezAdapterInterface+".StopDiscovery", 0); call.Err != nil {
			a.logger.Warn("[Bluetooth] Failed to stop BlueZ discovery", "error", call.Err)
		}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	objects, err = getManagedObjects(ctx, conn)
	if err != nil {
		return nil, err
	}
	return devicesFromManagedObjects(objects), nil
}

func (a *bluezAdapter) Pair(ctx context.Context, address, pin string) error {
	conn, err := a.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	objects, err := getManagedObjects(ctx, conn)
	if err != nil {
		return err
	}
	devicePath, properties, err := devicePathByAddress(objects, address)
	if err != nil {
		return err
	}
	if variantBool(properties, "Paired") {
		return nil
	}

	agentPath := dbus.ObjectPath(fmt.Sprintf("/com/aurago/bluetooth/agent_%d", time.Now().UnixNano()))
	agent := &pairingAgent{pin: pin}
	if err := conn.Export(agent, agentPath, "org.bluez.Agent1"); err != nil {
		return fmt.Errorf("export BlueZ pairing agent: %w", err)
	}
	capability := "NoInputNoOutput"
	if pin != "" {
		capability = "KeyboardOnly"
	}
	managerObject := conn.Object(bluezService, dbus.ObjectPath("/org/bluez"))
	if call := managerObject.CallWithContext(ctx, agentManager+".RegisterAgent", 0, agentPath, capability); call.Err != nil {
		_ = conn.Export(nil, agentPath, "org.bluez.Agent1")
		return fmt.Errorf("register BlueZ pairing agent: %w", call.Err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if call := managerObject.CallWithContext(cleanupCtx, agentManager+".UnregisterAgent", 0, agentPath); call.Err != nil {
			a.logger.Warn("[Bluetooth] Failed to unregister pairing agent", "error", call.Err)
		}
		_ = conn.Export(nil, agentPath, "org.bluez.Agent1")
	}()

	call := conn.Object(bluezService, devicePath).CallWithContext(ctx, bluezDeviceInterface+".Pair", 0)
	if call.Err != nil {
		message := strings.ToLower(call.Err.Error())
		if strings.Contains(message, "rejected") ||
			strings.Contains(message, "authentication") ||
			strings.Contains(message, "confirmation") {
			return codedError(ErrorPairingInteractionRequired, "The device requires an interactive pairing confirmation that AuraGo cannot approve automatically.", call.Err)
		}
		return fmt.Errorf("pair Bluetooth device: %w", call.Err)
	}
	if call := conn.Object(bluezService, devicePath).CallWithContext(ctx, propertiesInterface+".Set", 0, bluezDeviceInterface, "Trusted", dbus.MakeVariant(true)); call.Err != nil {
		a.logger.Warn("[Bluetooth] Paired device could not be marked trusted", "address", address, "error", call.Err)
	}
	return nil
}

func (a *bluezAdapter) Connect(ctx context.Context, address string) error {
	return a.deviceCall(ctx, address, "Connect")
}

func (a *bluezAdapter) Disconnect(ctx context.Context, address string) error {
	return a.deviceCall(ctx, address, "Disconnect")
}

func (a *bluezAdapter) deviceCall(ctx context.Context, address, operation string) error {
	conn, err := a.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	objects, err := getManagedObjects(ctx, conn)
	if err != nil {
		return err
	}
	devicePath, _, err := devicePathByAddress(objects, address)
	if err != nil {
		return err
	}
	if call := conn.Object(bluezService, devicePath).CallWithContext(ctx, bluezDeviceInterface+"."+operation, 0); call.Err != nil {
		return call.Err
	}
	return nil
}

func poweredAdapterPath(objects managedObjects) (dbus.ObjectPath, error) {
	for path, interfaces := range objects {
		if properties, ok := interfaces[bluezAdapterInterface]; ok && variantBool(properties, "Powered") {
			return path, nil
		}
	}
	return "", fmt.Errorf("no powered Bluetooth adapter is available")
}

func devicePathByAddress(objects managedObjects, address string) (dbus.ObjectPath, map[string]dbus.Variant, error) {
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return "", nil, err
	}
	for path, interfaces := range objects {
		properties, ok := interfaces[bluezDeviceInterface]
		if !ok {
			continue
		}
		if strings.EqualFold(variantString(properties, "Address"), normalized) {
			return path, properties, nil
		}
	}
	return "", nil, codedError(ErrorDeviceNotFound, fmt.Sprintf("Bluetooth device %q was not found by BlueZ.", normalized), nil)
}

func devicesFromManagedObjects(objects managedObjects) []Device {
	devices := make([]Device, 0)
	for _, interfaces := range objects {
		properties, ok := interfaces[bluezDeviceInterface]
		if !ok {
			continue
		}
		device := Device{
			Address:   strings.ToUpper(variantString(properties, "Address")),
			Name:      variantString(properties, "Name"),
			Alias:     variantString(properties, "Alias"),
			Paired:    variantBool(properties, "Paired"),
			Connected: variantBool(properties, "Connected"),
			Trusted:   variantBool(properties, "Trusted"),
			UUIDs:     variantStrings(properties, "UUIDs"),
		}
		if rssi, ok := variantInt16(properties, "RSSI"); ok {
			device.RSSI = &rssi
		}
		device.Audio = isAudioDevice(device)
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool {
		left := strings.ToLower(firstNonEmpty(devices[i].Alias, devices[i].Name, devices[i].Address))
		right := strings.ToLower(firstNonEmpty(devices[j].Alias, devices[j].Name, devices[j].Address))
		if left == right {
			return devices[i].Address < devices[j].Address
		}
		return left < right
	})
	return devices
}

func variantString(properties map[string]dbus.Variant, key string) string {
	if value, ok := properties[key]; ok {
		if text, ok := value.Value().(string); ok {
			return text
		}
	}
	return ""
}

func variantBool(properties map[string]dbus.Variant, key string) bool {
	if value, ok := properties[key]; ok {
		if boolean, ok := value.Value().(bool); ok {
			return boolean
		}
	}
	return false
}

func variantStrings(properties map[string]dbus.Variant, key string) []string {
	if value, ok := properties[key]; ok {
		if stringsValue, ok := value.Value().([]string); ok {
			return append([]string(nil), stringsValue...)
		}
	}
	return nil
}

func variantInt16(properties map[string]dbus.Variant, key string) (int16, bool) {
	if value, ok := properties[key]; ok {
		switch typed := value.Value().(type) {
		case int16:
			return typed, true
		case int32:
			return int16(typed), true
		}
	}
	return 0, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type pairingAgent struct {
	pin string
}

func (a *pairingAgent) Release() *dbus.Error {
	return nil
}

func (a *pairingAgent) RequestPinCode(dbus.ObjectPath) (string, *dbus.Error) {
	if a.pin == "" {
		return "", pairingInteractionDBusError()
	}
	return a.pin, nil
}

func (a *pairingAgent) DisplayPinCode(dbus.ObjectPath, string) *dbus.Error {
	return nil
}

func (a *pairingAgent) RequestPasskey(dbus.ObjectPath) (uint32, *dbus.Error) {
	if a.pin == "" {
		return 0, pairingInteractionDBusError()
	}
	value, err := strconv.ParseUint(a.pin, 10, 32)
	if err != nil {
		return 0, dbus.MakeFailedError(err)
	}
	return uint32(value), nil
}

func (a *pairingAgent) DisplayPasskey(dbus.ObjectPath, uint32, uint16) *dbus.Error {
	return nil
}

func (a *pairingAgent) RequestConfirmation(dbus.ObjectPath, uint32) *dbus.Error {
	return pairingInteractionDBusError()
}

func (a *pairingAgent) RequestAuthorization(dbus.ObjectPath) *dbus.Error {
	return nil
}

func (a *pairingAgent) AuthorizeService(dbus.ObjectPath, string) *dbus.Error {
	return nil
}

func (a *pairingAgent) Cancel() *dbus.Error {
	return nil
}

func pairingInteractionDBusError() *dbus.Error {
	return &dbus.Error{
		Name: "org.bluez.Error.Rejected",
		Body: []interface{}{"PAIRING_INTERACTION_REQUIRED"},
	}
}
