package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"runtime"
	"strings"
)

// RuViewConfig separates sensing, operator writes and agent reads.
// API credentials belong exclusively to the Vault.
type RuViewConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	ReadOnly     bool     `yaml:"readonly" json:"readonly"`
	AgentAccess  bool     `yaml:"agent_access" json:"agent_access"`
	Mode         string   `yaml:"mode" json:"mode"`
	Source       string   `yaml:"source" json:"source"`
	URL          string   `yaml:"url" json:"url"`
	AllowPrivate bool     `yaml:"allow_private" json:"allow_private"`
	HTTPPort     int      `yaml:"http_port" json:"http_port"`
	ReceiverIP   string   `yaml:"receiver_ip" json:"receiver_ip"`
	UDPEnabled   bool     `yaml:"udp_enabled" json:"udp_enabled"`
	UDPBind      string   `yaml:"udp_bind" json:"udp_bind"`
	UDPPort      int      `yaml:"udp_port" json:"udp_port"`
	UDPAllow     []string `yaml:"udp_allow" json:"udp_allow"`
	StorageMB    int64    `yaml:"storage_mb" json:"storage_mb"`
}

func DefaultRuViewConfig() RuViewConfig {
	return RuViewConfig{ReadOnly: true, Mode: "docker", Source: "esp32", HTTPPort: 3022, UDPPort: 5005, StorageMB: 1024, UDPAllow: []string{}}
}

func ValidateRuViewConfig(c RuViewConfig, inDocker bool) error {
	if c.Mode != "docker" && c.Mode != "external" && c.Mode != "native" {
		return fmt.Errorf("RuView mode must be docker, external or native")
	}
	if c.Source != "esp32" && c.Source != "wifi" && c.Source != "simulated" {
		return fmt.Errorf("RuView source must be esp32, wifi or simulated")
	}
	if c.HTTPPort < 1024 || c.HTTPPort > 65534 || c.UDPPort < 1024 || c.UDPPort > 65535 {
		return fmt.Errorf("RuView HTTP port must be 1024..65534; UDP port must be 1024..65535")
	}
	if c.StorageMB < 64 || c.StorageMB > 1024*1024 {
		return fmt.Errorf("RuView storage budget must be 64..1048576 MiB")
	}
	if c.ReceiverIP != "" && !RuViewPrivateIPv4(c.ReceiverIP) {
		return fmt.Errorf("RuView receiver requires a private IPv4 address")
	}
	if c.Enabled && c.Source == "wifi" && c.Mode == "docker" {
		return fmt.Errorf("host Wi-Fi requires native or external RuView")
	}
	if c.Enabled && c.Mode == "native" && (inDocker || (runtime.GOOS != "windows" && runtime.GOOS != "darwin")) {
		return fmt.Errorf("native RuView requires a Windows or macOS host; use external mode from Docker/Linux")
	}
	if c.Mode == "external" && (c.Enabled || c.URL != "") {
		u, err := url.Parse(c.URL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(c.URL, "\\\r\n") {
			return fmt.Errorf("RuView URL must be an HTTP(S) origin without credentials, path, query or fragment")
		}
		if ip, err := netip.ParseAddr(u.Hostname()); err == nil && (ip.IsPrivate() || ip.IsLoopback()) && !c.AllowPrivate {
			return fmt.Errorf("private RuView access requires allow_private")
		}
	}
	if c.UDPEnabled {
		if !RuViewPrivateIPv4(c.UDPBind) || len(c.UDPAllow) == 0 || len(c.UDPAllow) > 64 {
			return fmt.Errorf("RuView UDP requires a concrete private IPv4 bind address and 1..64 allowed sources")
		}
	}
	for _, value := range c.UDPAllow {
		if RuViewPrivateIPv4(value) {
			continue
		}
		p, err := netip.ParsePrefix(value)
		if err != nil || !p.Addr().Is4() || !p.Addr().IsPrivate() || p != p.Masked() {
			return fmt.Errorf("RuView UDP sources must be private IPv4 addresses or canonical private CIDRs")
		}
		valid := false
		for _, private := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
			outer := netip.MustParsePrefix(private)
			if p.Bits() >= outer.Bits() && outer.Contains(p.Addr()) {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("RuView UDP CIDR exceeds private address space")
		}
	}
	return nil
}

func RuViewPrivateIPv4(value string) bool {
	ip, err := netip.ParseAddr(value)
	return err == nil && ip.Is4() && ip.IsPrivate()
}
