// Package fritzbox – system service calls.
// Provides: device info, uptime, firmware version, update check, system log, reboot.
package fritzbox

import (
	"fmt"
	"strconv"
	"strings"
)

// SystemInfo holds general device information from TR-064 DeviceInfo.
type SystemInfo struct {
	ModelName       string
	SoftwareVersion string
	HardwareVersion string
	Uptime          int // uptime in seconds
	Serial          string
	OEM             string
}

// GetSystemInfo retrieves general device info via TR-064 DeviceInfo:GetInfo.
func (c *Client) GetSystemInfo() (*SystemInfo, error) {
	res, err := c.SOAP(svcDeviceInfo, ctlDeviceInfo, "GetInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox system: GetInfo: %w", err)
	}
	uptime, _ := strconv.Atoi(res["NewUpTime"])
	return &SystemInfo{
		ModelName:       res["NewModelName"],
		SoftwareVersion: res["NewSoftwareVersion"],
		HardwareVersion: res["NewHardwareVersion"],
		Uptime:          uptime,
		Serial:          res["NewSerialNumber"],
		OEM:             res["NewOEM"],
	}, nil
}

// GetSystemLog retrieves the Fritz!Box system log as a list of lines.
// Each line contains timestamp and message.
func (c *Client) GetSystemLog() ([]string, error) {
	res, err := c.SOAP(svcDeviceInfo, ctlDeviceInfo, "GetLog", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox system: GetLog: %w", err)
	}
	raw := res["NewLog"]
	if raw == "" {
		return nil, nil
	}
	lines := strings.Split(raw, "\n")
	// Trim whitespace-only lines.
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// GetSystemSecurityPort fetches the HTTPS port used for the Fritz!Box UI.
func (c *Client) GetSystemSecurityPort() (string, error) {
	res, err := c.SOAP(svcDeviceConfig, ctlDeviceConfig, "GetSecurityPort", nil)
	if err != nil {
		return "", fmt.Errorf("fritzbox system: GetSecurityPort: %w", err)
	}
	return res["NewSecurityPort"], nil
}

// Reboot triggers a soft reboot of the Fritz!Box.
// Requires System.ReadOnly == false in config.
func (c *Client) Reboot() error {
	if c.SystemReadOnly() {
		return fmt.Errorf("fritzbox system: reboot is blocked (readonly mode)")
	}
	_, err := c.SOAP(svcDeviceConfig, ctlDeviceConfig, "Reboot", nil)
	if err != nil {
		return fmt.Errorf("fritzbox system: Reboot: %w", err)
	}
	return nil
}
