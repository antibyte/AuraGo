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
	signalingNeedsDefault := cfg.AdvertisedSignalingHost == "" ||
		!advertisedHostMatchesBindFamily(cfg.AdvertisedSignalingHost, cfg.BindHost)
	mediaNeedsDefault := cfg.Media.AdvertisedHost == "" ||
		!advertisedHostMatchesBindFamily(cfg.Media.AdvertisedHost, cfg.BindHost)
	if signalingNeedsDefault || mediaNeedsDefault {
		if host := localSIPHostForRegistrar(ctx, cfg.Registrar, cfg.BindHost, locals); host != "" {
			if signalingNeedsDefault {
				cfg.AdvertisedSignalingHost = host
			}
			if mediaNeedsDefault {
				cfg.Media.AdvertisedHost = host
			}
		}
	}
	browserNeedsDefault := cfg.BrowserMedia.AdvertisedIP == "" ||
		!advertisedHostMatchesBindFamily(cfg.BrowserMedia.AdvertisedIP, cfg.BrowserMedia.BindHost)
	if browserNeedsDefault {
		host := ""
		if peer := parseSIPPeerIP(browserPeer); isLocalSIPPeer(peer) {
			host = selectLocalSIPHostForBind([]net.IP{peer}, locals, cfg.BrowserMedia.BindHost)
		}
		// Embedded tsnet has no host interface address, so RemoteAddr can be a
		// tailnet peer that cannot be matched against net.Interfaces. For a
		// local PBX, the route-selected media address remains the best direct
		// host candidate and keeps SIP RTP and browser audio on one family.
		if host == "" {
			mediaIP := net.ParseIP(strings.Trim(strings.TrimSpace(cfg.Media.AdvertisedHost), "[]"))
			if isLocalSIPPeer(mediaIP) &&
				advertisedHostMatchesBindFamily(mediaIP.String(), cfg.BrowserMedia.BindHost) {
				host = mediaIP.String()
			}
		}
		if host != "" {
			cfg.BrowserMedia.AdvertisedIP = host
		}
	}
}

func localSIPHostForRegistrar(ctx context.Context, registrar, bindHost string, locals []sipLocalAddress) string {
	host := sipRegistrarHost(registrar)
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isLocalSIPPeer(ip) {
			return ""
		}
		return selectLocalSIPHostForBind([]net.IP{ip}, locals, bindHost)
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
	return selectLocalSIPHostForBind(peers, locals, bindHost)
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
	return selectLocalSIPHostFamily(peers, locals, 0)
}

func selectLocalSIPHostForBind(peers []net.IP, locals []sipLocalAddress, bindHost string) string {
	if family := sipIPFamily(net.ParseIP(strings.Trim(strings.TrimSpace(bindHost), "[]"))); family != 0 {
		if host := selectLocalSIPHostFamily(peers, locals, family); host != "" {
			return host
		}
	}
	return selectLocalSIPHost(peers, locals)
}

func selectLocalSIPHostFamily(peers []net.IP, locals []sipLocalAddress, family int) string {
	bestScore := -1
	var best net.IP
	for _, peer := range peers {
		for _, local := range locals {
			if peer == nil || local.ip == nil || (peer.To4() == nil) != (local.ip.To4() == nil) {
				continue
			}
			if family != 0 && sipIPFamily(peer) != family {
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

func advertisedHostMatchesBindFamily(advertisedHost, bindHost string) bool {
	advertisedIP := net.ParseIP(strings.Trim(strings.TrimSpace(advertisedHost), "[]"))
	bindIP := net.ParseIP(strings.Trim(strings.TrimSpace(bindHost), "[]"))
	if advertisedIP == nil || bindIP == nil {
		// Preserve explicit hostnames and configurations without an IP bind.
		return true
	}
	return sipIPFamily(advertisedIP) == sipIPFamily(bindIP)
}

func sipIPFamily(ip net.IP) int {
	if ip == nil {
		return 0
	}
	if ip.To4() != nil {
		return 4
	}
	return 6
}

func isLocalSIPPeer(ip net.IP) bool {
	return ip != nil &&
		(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || isSIPCGNAT(ip))
}

func isSIPCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
}
