package ruview

import (
	"net"
	"net/url"
	"slices"
	"strings"

	"aurago/internal/config"
)

// BoardSetup contains non-secret provisioning defaults and unverified LAN suggestions.
// It is an operator-only read, separate from the agent's passive status response.
type BoardSetup struct {
	ReceiverIP string   `json:"receiver_ip"`
	UDPPort    int      `json:"udp_port"`
	UDPEnabled bool     `json:"udp_enabled"`
	Mode       string   `json:"mode"`
	Candidates []string `json:"candidates"`
}

// BoardSetup reads saved settings and local interfaces only. It never probes a
// receiver, resolves DNS, starts a runtime or changes the UDP publication policy.
func (m *Manager) BoardSetup() BoardSetup {
	return boardSetup(m.config(), localReceiverIPv4s)
}

func boardSetup(c *config.Config, localIPs func() []string) BoardSetup {
	r := c.RuView
	info := BoardSetup{ReceiverIP: r.ReceiverIP, UDPPort: r.UDPPort, UDPEnabled: r.UDPEnabled, Mode: r.Mode, Candidates: []string{}}
	add := func(ip string) {
		if config.RuViewPrivateIPv4(ip) && !slices.Contains(info.Candidates, ip) {
			info.Candidates = append(info.Candidates, ip)
		}
	}
	if r.Mode == "external" {
		if endpoint, err := url.Parse(r.URL); err == nil {
			add(endpoint.Hostname())
		}
		return info
	}
	add(r.UDPBind)
	if len(info.Candidates) > 0 {
		return info
	}
	if r.Mode == "docker" {
		host := strings.TrimSpace(c.Docker.Host)
		if host != "" && !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://") {
			// A remote Engine or socket proxy does not identify this machine as
			// the receiver. Its literal private address is only a suggestion.
			if endpoint, err := url.Parse(host); err == nil && !c.Runtime.IsDocker {
				add(endpoint.Hostname())
			}
			return info
		}
	}
	// Interfaces inside AuraGo's container are not the Docker host's LAN addresses.
	if !c.Runtime.IsDocker && (r.Mode == "docker" || r.Mode == "native") {
		for _, ip := range localIPs() {
			add(ip)
		}
	}
	return info
}

func localReceiverIPv4s() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || receiverVirtualInterface(iface.Name) {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && config.RuViewPrivateIPv4(ip.String()) {
				ips = append(ips, ip.String())
			}
		}
	}
	slices.Sort(ips)
	return slices.Compact(ips)
}

func receiverVirtualInterface(name string) bool {
	name = strings.ToLower(name)
	for _, prefix := range []string{"docker", "veth", "br-", "virbr", "tailscale", "tun", "tap", "wg", "zt", "utun", "vethernet"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
