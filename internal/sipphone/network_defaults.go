package sipphone

import (
	"context"
	"net"
	"strings"
	"time"

	"aurago/internal/config"
)

type sipLocalAddress struct {
	ip      net.IP
	network *net.IPNet
}

// ApplyProviderNetworkDefaults fills addresses that a local PBX must be able
// to route back to. It only derives addresses for private/local peers and
// therefore never guesses a NAT address for a public SIP provider.
func ApplyProviderNetworkDefaults(ctx context.Context, cfg *config.SIPConfig, browserPeer string) {
	if cfg == nil {
		return
	}
	applyProviderNetworkDefaults(ctx, cfg, browserPeer, sipLocalAddresses())
}

func applyProviderNetworkDefaults(ctx context.Context, cfg *config.SIPConfig, browserPeer string, locals []sipLocalAddress) {
	if len(locals) == 0 {
		return
	}
	if cfg.AdvertisedSignalingHost == "" || cfg.Media.AdvertisedHost == "" {
		if host := localSIPHostForRegistrar(ctx, cfg.Registrar, locals); host != "" {
			if cfg.AdvertisedSignalingHost == "" {
				cfg.AdvertisedSignalingHost = host
			}
			if cfg.Media.AdvertisedHost == "" {
				cfg.Media.AdvertisedHost = host
			}
		}
	}
	if cfg.BrowserMedia.AdvertisedIP == "" {
		if peer := parseSIPPeerIP(browserPeer); isLocalSIPPeer(peer) {
			cfg.BrowserMedia.AdvertisedIP = selectLocalSIPHost([]net.IP{peer}, locals)
		}
	}
}

func localSIPHostForRegistrar(ctx context.Context, registrar string, locals []sipLocalAddress) string {
	host := sipRegistrarHost(registrar)
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isLocalSIPPeer(ip) {
			return ""
		}
		return selectLocalSIPHost([]net.IP{ip}, locals)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	resolved, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return ""
	}
	peers := make([]net.IP, 0, len(resolved))
	for _, address := range resolved {
		if isLocalSIPPeer(address.IP) {
			peers = append(peers, address.IP)
		}
	}
	return selectLocalSIPHost(peers, locals)
}

func sipRegistrarHost(registrar string) string {
	registrar = strings.TrimSpace(registrar)
	if host, _, err := net.SplitHostPort(registrar); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(registrar, "[]")
}

func parseSIPPeerIP(address string) net.IP {
	address = strings.TrimSpace(address)
	if host, _, err := net.SplitHostPort(address); err == nil {
		address = host
	}
	return net.ParseIP(strings.Trim(address, "[]"))
}

func sipLocalAddresses() []sipLocalAddress {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	result := make([]sipLocalAddress, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, network, err := net.ParseCIDR(address.String())
			if err != nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() {
				continue
			}
			result = append(result, sipLocalAddress{ip: ip, network: network})
		}
	}
	return result
}

func selectLocalSIPHost(peers []net.IP, locals []sipLocalAddress) string {
	bestScore := -1
	var best net.IP
	for _, peer := range peers {
		for _, local := range locals {
			if peer == nil || local.ip == nil || (peer.To4() == nil) != (local.ip.To4() == nil) {
				continue
			}
			score := -1
			if local.network != nil && local.network.Contains(peer) {
				ones, _ := local.network.Mask.Size()
				score = 1000 + ones
			} else if isSIPCGNAT(peer) && isSIPCGNAT(local.ip) {
				// Tailscale peers normally use separate /32 routes but remain
				// mutually reachable inside 100.64.0.0/10.
				score = 900
			}
			if score > bestScore {
				bestScore = score
				best = local.ip
			}
		}
	}
	if best == nil {
		return ""
	}
	return best.String()
}

func isLocalSIPPeer(ip net.IP) bool {
	return ip != nil &&
		(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || isSIPCGNAT(ip))
}

func isSIPCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
}
