package meshcore

import (
	"context"
	"fmt"
	"go.bug.st/serial"
)

func SerialPorts() ([]string, error) { return serial.GetPortsList() }

func validBLENotification(b []byte, mtu uint16) bool {
	// Exact MTU-3 notifications may have been silently truncated by ESP32 firmware.
	return mtu >= maxFrame && len(b) > 0 && len(b) < int(mtu)-3 && len(b) <= maxFrame
}
func openLink(ctx context.Context, cfg Config, docker bool) (frameLink, error) {
	if cfg.Transport == "ble" {
		if docker {
			return nil, fmt.Errorf("meshcore_ble_docker_unavailable")
		}
		return openBLE(ctx, cfg.Address)
	}
	p, err := serial.Open(cfg.Port, &serial.Mode{BaudRate: 115200, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit, InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false}})
	if err != nil {
		return nil, fmt.Errorf("meshcore_serial_unavailable")
	}
	return &serialLink{p}, nil
}
