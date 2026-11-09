package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"

	"aurago/internal/config"
	"aurago/internal/desktopstore"
)

// storeRelayGOOS is the platform the Docker reachability rules apply to. Docker
// Desktop (Windows, macOS) forwards host.docker.internal to host loopback
// listeners; native Linux Docker does not, so a loopback-only AuraGo is
// unreachable there. Tests override it.
var storeRelayGOOS = runtime.GOOS

const (
	storeRelayDefaultPort = 8088
	storeRelayHostGateway = storeLLMRelayDockerHostAlias + ":host-gateway"
)

// storeLLMRelayProvider tells the Store how a container reaches the relay
// (desktopstore.LLMRelayProvider). Its errors are shown as Store operation
// errors: they name settings, never credentials or credentialed URLs.
type storeLLMRelayProvider struct {
	s *Server
}

var _ desktopstore.LLMRelayProvider = storeLLMRelayProvider{}

// RelayInfo answers with http://host.docker.internal:<port>. networkMode is
// ignored: DeepTutor has no shared-network mode, so it always uses the default
// bridge and the host-gateway alias, which is the Host the relay accepts when
// login is disabled (storeLLMRelayHostAllowed).
func (p storeLLMRelayProvider) RelayInfo(_ context.Context, _ string) (desktopstore.RelayInfo, error) {
	if p.s == nil {
		return desktopstore.RelayInfo{}, errors.New("the AuraGo server is not available")
	}
	cfg, client := p.s.chatRuntimeSnapshot()
	if cfg == nil || client == nil || strings.TrimSpace(cfg.LLM.Model) == "" {
		return desktopstore.RelayInfo{}, errors.New("no main LLM is configured in AuraGo; set up a provider and model under Config > Providers first")
	}
	listener, err := storeRelayListener(cfg)
	if err != nil {
		return desktopstore.RelayInfo{}, err
	}
	if listener.loopbackOnly && storeRelayGOOS == "linux" {
		if listener.dedicatedLoopback {
			return desktopstore.RelayInfo{}, errors.New("with HTTPS enabled AuraGo opens its plain-HTTP listener on 127.0.0.1 only, which Docker containers cannot reach on Linux; " +
				"turn HTTPS off (server.https.enabled: false) and set server.host to an address Docker containers can reach (for example 0.0.0.0), or run AuraGo on Docker Desktop")
		}
		return desktopstore.RelayInfo{}, errors.New("AuraGo only listens on a loopback address, which Docker containers cannot reach on Linux; " +
			"set server.host to an address Docker containers can reach (for example 0.0.0.0), restart AuraGo, and keep login enabled or the port firewalled")
	}
	info := desktopstore.RelayInfo{
		BaseURL:             fmt.Sprintf("http://%s:%d", storeLLMRelayDockerHostAlias, listener.port),
		ExtraHosts:          []string{storeRelayHostGateway},
		Vision:              cfg.LLM.Multimodal,
		EmbeddingsAvailable: storeRelayEmbeddingsAvailable(cfg),
	}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil && provider.ContextWindow > 0 {
		info.ContextWindow = provider.ContextWindow
	}
	return info, nil
}

// storeRelayListenerInfo describes the plain-HTTP listener a container can use.
type storeRelayListenerInfo struct {
	port         int
	loopbackOnly bool
	// dedicatedLoopback marks the 127.0.0.1 listener AuraGo opens next to its
	// TLS listener; server.host does not move it.
	dedicatedLoopback bool
}

// storeRelayListener finds the plain-HTTP port a container can use and whether
// that listener is bound to loopback only. The Server.Host default is
// 127.0.0.1 (config.Load), so a stock install is loopback-only.
func storeRelayListener(cfg *config.Config) (storeRelayListenerInfo, error) {
	if !cfg.Server.HTTPS.Enabled {
		port := cfg.Server.Port
		if port <= 0 {
			port = storeRelayDefaultPort
		}
		return storeRelayListenerInfo{port: port, loopbackOnly: storeRelayHostIsLoopback(cfg.Server.Host)}, nil
	}
	// With HTTPS enabled the main listener speaks TLS and the optional HTTP port
	// only redirects to it. The one plain-HTTP listener is the dedicated
	// 127.0.0.1 one on server.port, which exists only when that port differs
	// from both. A Cloudflare tunnel loopback port replaces it and is routed by
	// the tunnel's own expose target, so it never counts.
	port := DedicatedInternalLoopbackPort(cfg)
	if port <= 0 || cfg.CloudflareTunnel.LoopbackPort > 0 {
		return storeRelayListenerInfo{}, errors.New("it needs a plain-HTTP AuraGo listener that Docker containers can reach, " +
			"but AuraGo only serves HTTPS here (the HTTP port only redirects to HTTPS); " +
			"use a separate server.port for a plain-HTTP listener or turn HTTPS off (server.https.enabled: false) on a trusted network")
	}
	return storeRelayListenerInfo{port: port, loopbackOnly: true, dedicatedLoopback: true}, nil
}

// storeRelayHostIsLoopback reports whether a server.host value binds loopback
// only. An empty value counts as loopback because config.Load defaults it to
// 127.0.0.1.
func storeRelayHostIsLoopback(host string) bool {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
