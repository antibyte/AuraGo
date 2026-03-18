// Package fritzbox – network service calls.
// Covers: WLAN info/toggle, guest WLAN, mesh topology, host list, WoL, port forwarding.
package fritzbox

import (
	"fmt"
)

// WLANInfo holds the status of a WLAN radio (2.4 GHz / 5 GHz / 60 GHz / Guest).
type WLANInfo struct {
	Index     int // 1–4
	SSID      string
	Channel   string
	Enabled   bool
	Frequency string // "2.4 GHz", "5 GHz", etc.
}

// HostEntry represents a connected or known network host.
type HostEntry struct {
	MACAddress string
	IPAddress  string
	Name       string
	Active     bool
	Interface  string // e.g., "802.11" or "Ethernet"
}

// PortForwardEntry holds a NAT/port-forward rule.
type PortForwardEntry struct {
	RemoteHost     string
	ExternalPort   string
	Protocol       string // "TCP" or "UDP"
	InternalPort   string
	InternalClient string
	Enabled        bool
	Description    string
}

// wlanService returns the service URN and control URL for a given WLAN index (1–4).
func wlanService(index int) (string, string, error) {
	switch index {
	case 1:
		return svcWLAN1, ctlWLAN1, nil
	case 2:
		return svcWLAN2, ctlWLAN2, nil
	case 3:
		return svcWLAN3, ctlWLAN3, nil
	case 4:
		return svcWLAN4, ctlWLAN4, nil
	default:
		return "", "", fmt.Errorf("fritzbox network: invalid WLAN index %d (must be 1–4)", index)
	}
}

// GetWLANInfo retrieves info for a WLAN interface by index (1=2.4 GHz, 2=5 GHz, 3=60 GHz/Guest, 4=Guest).
func (c *Client) GetWLANInfo(index int) (*WLANInfo, error) {
	svc, ctl, err := wlanService(index)
	if err != nil {
		return nil, err
	}
	res, err := c.SOAP(svc, ctl, "GetInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox network: WLAN%d GetInfo: %w", index, err)
	}
	return &WLANInfo{
		Index:   index,
		SSID:    res["NewSSID"],
		Channel: res["NewChannel"],
		Enabled: res["NewEnable"] == "1",
	}, nil
}

// SetWLANEnabled enables or disables a WLAN interface by index.
// Blocked when ReadOnly is true.
func (c *Client) SetWLANEnabled(index int, enabled bool) error {
	if c.NetworkReadOnly() {
		return fmt.Errorf("fritzbox network: WLAN toggle blocked (readonly mode)")
	}
	svc, ctl, err := wlanService(index)
	if err != nil {
		return err
	}
	val := "0"
	if enabled {
		val = "1"
	}
	_, err = c.SOAP(svc, ctl, "SetEnable", map[string]string{"NewEnable": val})
	if err != nil {
		return fmt.Errorf("fritzbox network: WLAN%d SetEnable: %w", index, err)
	}
	return nil
}

// GetHostList returns the list of all known hosts (active and inactive).
func (c *Client) GetHostList() ([]HostEntry, error) {
	// First get the total number of hosts.
	countRes, err := c.SOAP(svcHosts, ctlHosts, "GetHostNumberOfEntries", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox network: GetHostNumberOfEntries: %w", err)
	}
	count := 0
	fmt.Sscanf(countRes["NewHostNumberOfEntries"], "%d", &count)

	entries := make([]HostEntry, 0, count)
	for i := 0; i < count; i++ {
		res, err := c.SOAP(svcHosts, ctlHosts, "GetGenericHostEntry",
			map[string]string{"NewIndex": fmt.Sprintf("%d", i)})
		if err != nil {
			// Non-fatal: some models return fewer entries – stop on first error.
			break
		}
		entries = append(entries, HostEntry{
			MACAddress: res["NewMACAddress"],
			IPAddress:  res["NewIPAddress"],
			Name:       res["NewHostName"],
			Active:     res["NewActive"] == "1",
			Interface:  res["NewInterfaceType"],
		})
	}
	return entries, nil
}

// WakeOnLAN sends a WoL magic packet via TR-064.
// Blocked when ReadOnly is true.
func (c *Client) WakeOnLAN(mac string) error {
	if c.NetworkReadOnly() {
		return fmt.Errorf("fritzbox network: WoL blocked (readonly mode)")
	}
	_, err := c.SOAP(svcHosts, ctlHosts, "X_AVM-DE_WakeOnLANByMACAddress",
		map[string]string{"NewMACAddress": mac})
	if err != nil {
		return fmt.Errorf("fritzbox network: WakeOnLAN %s: %w", mac, err)
	}
	return nil
}

// GetPortForwardingList returns all port forwarding entries.
func (c *Client) GetPortForwardingList() ([]PortForwardEntry, error) {
	countRes, err := c.SOAP(svcWANIPConn, ctlWANIPConn, "GetPortMappingNumberOfEntries", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox network: GetPortMappingNumberOfEntries: %w", err)
	}
	count := 0
	fmt.Sscanf(countRes["NewPortMappingNumberOfEntries"], "%d", &count)

	entries := make([]PortForwardEntry, 0, count)
	for i := 0; i < count; i++ {
		res, err := c.SOAP(svcWANIPConn, ctlWANIPConn, "GetGenericPortMappingEntry",
			map[string]string{"NewPortMappingIndex": fmt.Sprintf("%d", i)})
		if err != nil {
			break
		}
		entries = append(entries, PortForwardEntry{
			RemoteHost:     res["NewRemoteHost"],
			ExternalPort:   res["NewExternalPort"],
			Protocol:       res["NewProtocol"],
			InternalPort:   res["NewInternalPort"],
			InternalClient: res["NewInternalClient"],
			Enabled:        res["NewEnabled"] == "1",
			Description:    res["NewPortMappingDescription"],
		})
	}
	return entries, nil
}

// AddPortForwarding creates a new port forwarding rule.
// Blocked when ReadOnly is true.
func (c *Client) AddPortForwarding(e PortForwardEntry) error {
	if c.NetworkReadOnly() {
		return fmt.Errorf("fritzbox network: port forwarding add blocked (readonly mode)")
	}
	enabled := "0"
	if e.Enabled {
		enabled = "1"
	}
	_, err := c.SOAP(svcWANIPConn, ctlWANIPConn, "AddPortMapping", map[string]string{
		"NewRemoteHost":             e.RemoteHost,
		"NewExternalPort":           e.ExternalPort,
		"NewProtocol":               e.Protocol,
		"NewInternalPort":           e.InternalPort,
		"NewInternalClient":         e.InternalClient,
		"NewEnabled":                enabled,
		"NewPortMappingDescription": e.Description,
		"NewLeaseDuration":          "0",
	})
	if err != nil {
		return fmt.Errorf("fritzbox network: AddPortMapping: %w", err)
	}
	return nil
}

// DeletePortForwarding removes a port forwarding rule by external port + protocol.
// Blocked when ReadOnly is true.
func (c *Client) DeletePortForwarding(remoteHost, externalPort, protocol string) error {
	if c.NetworkReadOnly() {
		return fmt.Errorf("fritzbox network: port forwarding delete blocked (readonly mode)")
	}
	_, err := c.SOAP(svcWANIPConn, ctlWANIPConn, "DeletePortMapping", map[string]string{
		"NewRemoteHost":   remoteHost,
		"NewExternalPort": externalPort,
		"NewProtocol":     protocol,
	})
	if err != nil {
		return fmt.Errorf("fritzbox network: DeletePortMapping: %w", err)
	}
	return nil
}
