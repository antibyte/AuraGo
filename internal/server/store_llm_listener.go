package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktopstore"
	"aurago/internal/tools"
)

const storeRelayDefaultPort = 8099

type storeRelayTopology struct {
	bind, host, network string
	subnet              *net.IPNet
	extraHosts          []string
}

type storeRelayCall struct {
	app    string
	cancel context.CancelFunc
}
type storeRelayRuntime struct {
	sync.Mutex
	listener *http.Server
	address  string
	closed   bool
	next     uint64
	calls    map[uint64]storeRelayCall
}

var storeRelayEngineRequest = tools.DockerRequestContext
var storeRelayResolveTopology = resolveStoreRelayTopology
var storeRelayListen = net.Listen

func resolveStoreRelayTopology(ctx context.Context, cfg *config.Config, network string) (storeRelayTopology, error) {
	fail := errors.New("Relay network unavailable: select a local Docker bridge shared with AuraGo")
	if !printGuardEngineIsLocal(cfg.Docker.Host, cfg.Runtime.IsDocker) {
		return storeRelayTopology{}, fail
	}
	if !cfg.Runtime.IsDocker && storeRelayGOOS != "linux" {
		return storeRelayTopology{bind: "127.0.0.1", host: storeLLMRelayDockerHostAlias}, nil
	}
	docker := tools.DockerConfig{Host: cfg.Docker.Host}
	result := storeRelayTopology{}
	if cfg.Runtime.IsDocker {
		name, err := os.Hostname()
		if err != nil {
			return result, fail
		}
		data, status, err := storeRelayEngineRequest(ctx, docker, "GET", "/containers/"+url.PathEscape(name)+"/json", "")
		var own struct {
			Name            string
			NetworkSettings struct {
				Networks map[string]struct{ IPAddress string }
			}
		}
		if err != nil || status != 200 || json.Unmarshal(data, &own) != nil {
			return result, fail
		}
		if network == "" {
			network = cfg.StoreLLMRelay.DockerNetwork
		}
		if network == "" {
			for candidate := range own.NetworkSettings.Networks {
				if candidate == "bridge" || candidate == "host" || candidate == "none" {
					continue
				}
				if network != "" {
					return result, fail
				}
				network = candidate
			}
		}
		endpoint, ok := own.NetworkSettings.Networks[network]
		if !ok || network == "bridge" || network == "host" || network == "none" {
			return result, fail
		}
		result.bind, result.host, result.network = endpoint.IPAddress, strings.TrimPrefix(own.Name, "/"), network
	} else {
		if network == "" {
			network = cfg.StoreLLMRelay.DockerNetwork
		}
		if network == "" {
			network = "bridge"
		}
		if network != "bridge" {
			result.network = network
		}
	}
	data, status, err := storeRelayEngineRequest(ctx, docker, "GET", "/networks/"+url.PathEscape(network), "")
	var details struct {
		Driver   string
		Internal bool
		IPAM     struct {
			Config []struct{ Subnet, Gateway string }
		}
	}
	if err != nil || status != 200 || json.Unmarshal(data, &details) != nil || details.Driver != "bridge" || details.Internal {
		return result, fail
	}
	for _, ipam := range details.IPAM.Config {
		_, subnet, err := net.ParseCIDR(ipam.Subnet)
		if err != nil || subnet.IP.To4() == nil {
			continue
		}
		if !cfg.Runtime.IsDocker {
			result.bind, result.host = ipam.Gateway, ipam.Gateway
		}
		if subnet.Contains(net.ParseIP(result.bind)) {
			result.subnet = subnet
			break
		}
	}
	if result.subnet == nil || result.host == "" || !localStoreRelayIP(result.bind) {
		return storeRelayTopology{}, fail
	}
	return result, nil
}

func localStoreRelayIP(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addresses {
		if candidate, _, err := net.ParseCIDR(addr.String()); err == nil && candidate.Equal(ip) {
			return true
		}
	}
	return false
}

func (s *Server) ensureStoreRelayListener(cfg *config.Config, topology storeRelayTopology) error {
	port := cfg.StoreLLMRelay.Port
	if port == 0 {
		port = storeRelayDefaultPort
	}
	if port < 1 || port > 65535 {
		return errors.New("Relay port must be between 1 and 65535")
	}
	address := net.JoinHostPort(topology.bind, strconv.Itoa(port))
	s.storeRelay.Lock()
	defer s.storeRelay.Unlock()
	if s.storeRelay.closed || s.ConfigSnapshot() != cfg {
		return errors.New("Relay configuration changed; retry the connection test")
	}
	if s.storeRelay.listener != nil {
		if s.storeRelay.address == address {
			return nil
		}
		return errors.New("Relay already uses another Docker network; select the same network for both apps")
	}
	listener, err := storeRelayListen("tcp", address)
	if err != nil {
		return fmt.Errorf("Relay port %d unavailable; configure another internal relay port", port)
	}
	mux := s.storeRelayMux()
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 32 << 10, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || (topology.subnet == nil && !ip.IsLoopback()) || (topology.subnet != nil && !topology.subnet.Contains(ip)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})}
	s.storeRelay.listener, s.storeRelay.address = server, address
	go func() { _ = server.Serve(listener) }()
	return nil
}

func (s *Server) storeRelayMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, id := range []string{desktopstore.DeepTutorAppID, desktopstore.OpenWebUIAppID} {
		h := newStoreLLMRelay(s, s.storeRelayVerifier)
		h.appID, h.prefix = id, desktopstore.RelayPath(id)
		mux.Handle(h.prefix+"/", h)
	}
	return mux
}

func (s *Server) storeRelayContext(ctx context.Context, app string) (context.Context, func()) {
	s.storeRelay.Lock()
	defer s.storeRelay.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	if s.storeRelay.closed {
		cancel()
		return ctx, func() {}
	}
	if s.storeRelay.calls == nil {
		s.storeRelay.calls = map[uint64]storeRelayCall{}
	}
	s.storeRelay.next++
	id := s.storeRelay.next
	s.storeRelay.calls[id] = storeRelayCall{app, cancel}
	return ctx, func() { s.storeRelay.Lock(); delete(s.storeRelay.calls, id); s.storeRelay.Unlock(); cancel() }
}

func (s *Server) revokeStoreRelay(app string, shutdown bool) {
	s.storeRelay.Lock()
	defer s.storeRelay.Unlock()
	for id, call := range s.storeRelay.calls {
		if app == "" || call.app == app {
			call.cancel()
			delete(s.storeRelay.calls, id)
		}
	}
	if app == "" && s.storeRelay.listener != nil {
		_ = s.storeRelay.listener.Close()
		s.storeRelay.listener = nil
		s.storeRelay.address = ""
	}
	if shutdown {
		s.storeRelay.closed = true
	}
}

func (s *Server) restoreStoreRelay() {
	cfg := s.ConfigSnapshot()
	dir := cfg.Directories.DataDir
	if dir == "" {
		dir = "data"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	installed, err := desktopstore.DeepTutorNeedsLogin(ctx, filepath.Join(dir, config.DesktopStoreDBFilename))
	if err != nil || !installed {
		return
	}
	if _, err := (storeLLMRelayProvider{s}).RelayInfo(ctx, ""); err != nil && s.Logger != nil {
		s.Logger.Warn("Store relay unavailable; check the Store connection dialog")
	}
}
