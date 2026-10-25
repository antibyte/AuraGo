package fritzbox

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"
)

const maxInfraHostEntries = 4096

// InfraHost contains only display-safe fields. The router MAC is represented
// by Key and is resolved inside this package when an authorized action runs.
type InfraHost struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address,omitempty"`
	Active    bool   `json:"active"`
	Interface string `json:"interface,omitempty"`
}

type InfraHostEditInfo struct {
	FriendlyName string `json:"friendly_name"`
	Writable     bool   `json:"writable"`
}

type infraHostRecord struct {
	entry HostEntry
	key   string
}

func (c *Client) infraHostsAllowed(write bool) error {
	if c == nil || !c.Cfg.FritzBox.Enabled || c.Cfg.FritzBox.Host == "" || !c.NetworkEnabled() || !c.Cfg.FritzBox.Network.SubFeatures.Hosts {
		return errors.New("fritzbox hosts are disabled")
	}
	if write && c.NetworkReadOnly() {
		return errors.New("fritzbox network is read-only")
	}
	return nil
}

func (c *Client) infraHostRecords() ([]infraHostRecord, error) {
	if err := c.infraHostsAllowed(false); err != nil {
		return nil, err
	}
	result, err := c.SOAP(svcHosts, ctlHosts, "GetHostNumberOfEntries", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox host count: %w", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(result["NewHostNumberOfEntries"]))
	if err != nil || count < 0 || count > maxInfraHostEntries {
		return nil, errors.New("fritzbox host count is invalid")
	}
	records := make([]infraHostRecord, 0, count)
	for i := 0; i < count; i++ {
		entry, err := c.SOAP(svcHosts, ctlHosts, "GetGenericHostEntry", map[string]string{"NewIndex": strconv.Itoa(i)})
		if err != nil {
			return nil, fmt.Errorf("fritzbox host list incomplete: %w", err)
		}
		mac, err := net.ParseMAC(strings.TrimSpace(entry["NewMACAddress"]))
		if err != nil || len(mac) != 6 {
			// Entries without a usable MAC (VPN/WireGuard peers, remote or
			// IPv6-only entries) cannot be keyed or acted on; skip them.
			continue
		}
		host := HostEntry{
			MACAddress: strings.ToUpper(mac.String()),
			IPAddress:  strings.TrimSpace(entry["NewIPAddress"]),
			Name:       strings.TrimSpace(entry["NewHostName"]),
			Active:     entry["NewActive"] == "1",
			Interface:  strings.TrimSpace(entry["NewInterfaceType"]),
		}
		records = append(records, infraHostRecord{entry: host, key: c.infraHostKey(host.MACAddress)})
	}
	if count > 0 && len(records) == 0 {
		// A firmware that stops reporting MACs must not make every Fritz!Box
		// host look removed to callers that diff successive observations.
		return nil, errors.New("fritzbox hosts carry no usable MAC address")
	}
	return records, nil
}

func (c *Client) infraHostKey(mac string) string {
	instance := strings.ToLower(strings.TrimSpace(c.Cfg.FritzBox.Host))
	sum := sha256.Sum256([]byte(instance + "\x00" + strings.ToUpper(strings.TrimSpace(mac))))
	return "fbh1_" + hex.EncodeToString(sum[:])
}

// InfraHosts enumerates all Fritz!Box Hosts entries that carry a usable MAC
// address. Entries without one (VPN peers, remote or IPv6-only entries) are
// skipped because they can be neither keyed nor acted on. A SOAP failure on the
// count or on any entry fails the whole observation, as does a table whose
// entries all lack a MAC, so callers cannot infer removals from a partial list.
func (c *Client) InfraHosts() ([]InfraHost, error) {
	records, err := c.infraHostRecords()
	if err != nil {
		return nil, err
	}
	result := make([]InfraHost, 0, len(records))
	for _, record := range records {
		result = append(result, InfraHost{
			Key: record.key, Name: record.entry.Name, IPAddress: record.entry.IPAddress,
			Active: record.entry.Active, Interface: record.entry.Interface,
		})
	}
	return result, nil
}

func (c *Client) infraHostByKey(key string) (infraHostRecord, error) {
	if !strings.HasPrefix(key, "fbh1_") || len(key) != len("fbh1_")+64 {
		return infraHostRecord{}, errors.New("unknown fritzbox host")
	}
	records, err := c.infraHostRecords()
	if err != nil {
		return infraHostRecord{}, err
	}
	for _, record := range records {
		if record.key == key {
			return record, nil
		}
	}
	return infraHostRecord{}, errors.New("unknown fritzbox host")
}

// InfraHostEdit reads the supported TR-064 friendly-name field and write grant.
func (c *Client) InfraHostEdit(key string) (InfraHostEditInfo, error) {
	record, err := c.infraHostByKey(key)
	if err != nil {
		return InfraHostEditInfo{}, err
	}
	result, err := c.SOAP(svcHosts, ctlHosts, "X_AVM-DE_GetSpecificHostEntryByIP", map[string]string{"NewIPAddress": record.entry.IPAddress})
	if err != nil {
		return InfraHostEditInfo{}, fmt.Errorf("read fritzbox host metadata: %w", err)
	}
	return InfraHostEditInfo{
		FriendlyName: strings.TrimSpace(result["NewX_AVM-DE_FriendlyName"]),
		Writable:     result["NewX_AVM-DE_FriendlyNameIsWriteable"] == "1",
	}, nil
}

// InfraSetHostFriendlyName performs the documented Hosts action and confirms
// the new value through a separate readback.
func (c *Client) InfraSetHostFriendlyName(key, friendlyName string) (string, error) {
	if err := c.infraHostsAllowed(true); err != nil {
		return "failed", err
	}
	friendlyName = strings.TrimSpace(friendlyName)
	if friendlyName == "" || len([]rune(friendlyName)) > 64 {
		return "failed", errors.New("friendly name must contain 1 to 64 characters")
	}
	for _, char := range friendlyName {
		if unicode.IsControl(char) {
			return "failed", errors.New("friendly name contains control characters")
		}
	}
	record, err := c.infraHostByKey(key)
	if err != nil {
		return "failed", err
	}
	info, err := c.InfraHostEdit(key)
	if err != nil {
		return "failed", err
	}
	if !info.Writable {
		return "failed", errors.New("fritzbox host friendly name is not writable")
	}
	_, err = c.SOAP(svcHosts, ctlHosts, "X_AVM-DE_SetFriendlyNameByMAC", map[string]string{
		"NewMACAddress":            record.entry.MACAddress,
		"NewX_AVM-DE_FriendlyName": friendlyName,
	})
	if err != nil {
		return "unknown", fmt.Errorf("fritzbox host write outcome unknown: %w", err)
	}
	readback, err := c.InfraHostEdit(key)
	if err != nil || readback.FriendlyName != friendlyName {
		return "unknown", errors.New("fritzbox host write readback failed")
	}
	return "completed", nil
}

// InfraWakeHost resolves the opaque key against a fresh full host list before
// sending the documented WoL action; the MAC never crosses the package API.
func (c *Client) InfraWakeHost(key string) (string, error) {
	if err := c.infraHostsAllowed(true); err != nil {
		return "failed", err
	}
	if !c.Cfg.FritzBox.Network.SubFeatures.WakeOnLAN {
		return "failed", errors.New("fritzbox wake-on-lan is disabled")
	}
	record, err := c.infraHostByKey(key)
	if err != nil {
		return "failed", err
	}
	_, err = c.SOAP(svcHosts, ctlHosts, "X_AVM-DE_WakeOnLANByMACAddress", map[string]string{"NewMACAddress": record.entry.MACAddress})
	if err != nil {
		return "unknown", fmt.Errorf("fritzbox wake outcome unknown: %w", err)
	}
	return "completed", nil
}
