package server

import (
	"aurago/internal/desktopstore"
	"context"
	"errors"
	"net"
	"runtime"
	"strconv"
	"strings"
)

var storeRelayGOOS = runtime.GOOS

type storeLLMRelayProvider struct{ s *Server }

var _ desktopstore.LLMRelayProvider = storeLLMRelayProvider{}

func (p storeLLMRelayProvider) RelayInfo(ctx context.Context, network string) (desktopstore.RelayInfo, error) {
	if p.s == nil {
		return desktopstore.RelayInfo{}, errors.New("AuraGo server unavailable")
	}
	cfg := p.s.ConfigSnapshot()
	_, client := p.s.chatRuntimeSnapshot()
	if cfg == nil || client == nil || strings.TrimSpace(cfg.LLM.Model) == "" {
		return desktopstore.RelayInfo{}, errors.New("Configure the main AuraGo provider and model first")
	}
	if err := desktopstore.RequireDeepTutorLogin(cfg.Auth.Enabled, cfg.Auth.PasswordHash); err != nil {
		return desktopstore.RelayInfo{}, err
	}
	topology, err := storeRelayResolveTopology(ctx, cfg, network)
	if err != nil {
		return desktopstore.RelayInfo{}, err
	}
	if err := p.s.ensureStoreRelayListener(cfg, topology); err != nil {
		return desktopstore.RelayInfo{}, err
	}
	port := cfg.StoreLLMRelay.Port
	if port == 0 {
		port = storeRelayDefaultPort
	}
	info := desktopstore.RelayInfo{BaseURL: "http://" + net.JoinHostPort(topology.host, strconv.Itoa(port)), ExtraHosts: topology.extraHosts, DockerNetwork: topology.network, Vision: cfg.LLM.Multimodal, EmbeddingsAvailable: storeRelayEmbeddingsAvailable(cfg)}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil {
		info.ContextWindow = provider.ContextWindow
	}
	return info, nil
}
