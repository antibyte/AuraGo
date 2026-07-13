package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"aurago/internal/virtualcomputers"
)

const virtualComputersManagementUnavailable = "Boring Computers management service is unavailable; run setup install or repair"

var (
	virtualComputersManagementURL           = virtualcomputers.ManagementURL
	virtualComputersManagementSSHExecutor   = virtualComputersSSHSetupExecutor
	virtualComputersManagementTunnelStarter = startVirtualComputersSSHTunnel
	virtualComputersManagementHealthProbe   = defaultVirtualComputersManagementHealthProbe
)

var virtualComputersManagementTunnel = struct {
	sync.Mutex
	key   string
	close func()
}{}

func virtualComputersEnsureManagementAccess(s *Server, cfg virtualcomputers.ToolConfig) error {
	switch virtualComputersControlPlaneMode(cfg) {
	case virtualcomputers.ControlPlaneLocalHost:
		if virtualComputersManagementHealthProbe(virtualComputersManagementURL) {
			return nil
		}
		return errors.New(virtualComputersManagementUnavailable)
	case virtualcomputers.ControlPlaneSSHHost:
		return virtualComputersEnsureRemoteManagementAccess(s, cfg)
	default:
		return errors.New(virtualComputersManagementUnavailable)
	}
}

func virtualComputersEnsureRemoteManagementAccess(s *Server, cfg virtualcomputers.ToolConfig) error {
	executor, err := virtualComputersManagementSSHExecutor(s, cfg)
	if err != nil {
		virtualComputersLogManagementError(s, "SSH setup failed", err)
		return errors.New(virtualComputersManagementUnavailable)
	}
	localAddr, ok := virtualComputersLoopbackListenAddr(virtualComputersManagementURL)
	if !ok {
		virtualComputersLogManagementError(s, "invalid loopback management URL", fmt.Errorf("URL %q is not loopback HTTP(S)", virtualComputersManagementURL))
		return errors.New(virtualComputersManagementUnavailable)
	}
	key := fmt.Sprintf("%s:%d>%s", executor.Host, executor.Port, virtualcomputers.ManagementListenAddr)

	virtualComputersManagementTunnel.Lock()
	defer virtualComputersManagementTunnel.Unlock()
	if virtualComputersManagementTunnel.key == key && virtualComputersManagementTunnel.close != nil {
		if virtualComputersManagementHealthProbe(virtualComputersManagementURL) {
			return nil
		}
		virtualComputersManagementTunnel.close()
		virtualComputersManagementTunnel.key = ""
		virtualComputersManagementTunnel.close = nil
	}
	if virtualComputersManagementTunnel.close != nil {
		virtualComputersManagementTunnel.close()
		virtualComputersManagementTunnel.key = ""
		virtualComputersManagementTunnel.close = nil
	}

	closeFn, err := virtualComputersManagementTunnelStarter(executor, localAddr, virtualcomputers.ManagementListenAddr, s)
	if err != nil {
		virtualComputersLogManagementError(s, "SSH tunnel start failed", err)
		return errors.New(virtualComputersManagementUnavailable)
	}
	if !virtualComputersManagementHealthProbe(virtualComputersManagementURL) {
		closeFn()
		virtualComputersLogManagementError(s, "management health probe failed after SSH tunnel start", fmt.Errorf("health URL %s", virtualcomputers.ManagementHealthURL(virtualComputersManagementURL)))
		return errors.New(virtualComputersManagementUnavailable)
	}
	virtualComputersManagementTunnel.key = key
	virtualComputersManagementTunnel.close = closeFn
	return nil
}

func handleVirtualComputersManagementRedirect(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := virtualComputersConfigSnapshot(s)
		if !cfg.Enabled {
			http.NotFound(w, r)
			return
		}
		scope := virtualComputersManagementRequestScope(r)
		if !requireDesktopPermission(s, w, r, scope) {
			return
		}
		if cfg.ReadOnly && scope == desktopScopeWrite {
			jsonError(w, "Virtual computers are configured as read-only", http.StatusForbidden)
			return
		}
		http.Redirect(w, r, virtualcomputers.ManagementBasePath+"/", http.StatusTemporaryRedirect)
	}
}

func handleVirtualComputersManagement(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := virtualComputersConfigSnapshot(s)
		if !cfg.Enabled {
			http.NotFound(w, r)
			return
		}
		scope := virtualComputersManagementRequestScope(r)
		if !requireDesktopPermission(s, w, r, scope) {
			return
		}
		if cfg.ReadOnly && scope == desktopScopeWrite {
			jsonError(w, "Virtual computers are configured as read-only", http.StatusForbidden)
			return
		}
		if err := virtualComputersEnsureManagementAccess(s, cfg); err != nil {
			jsonError(w, virtualComputersManagementUnavailable, http.StatusServiceUnavailable)
			return
		}
		target, err := url.Parse(virtualComputersManagementURL)
		if err != nil {
			virtualComputersLogManagementError(s, "invalid proxy target", err)
			jsonError(w, virtualComputersManagementUnavailable, http.StatusServiceUnavailable)
			return
		}
		forwardedHost := r.Host
		forwardedProto := "http"
		if r.TLS != nil || strings.EqualFold(r.URL.Scheme, "https") {
			forwardedProto = "https"
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		director := proxy.Director
		proxy.Director = func(req *http.Request) {
			director(req)
			req.Host = target.Host
			req.Header.Set("X-Forwarded-Host", forwardedHost)
			req.Header.Set("X-Forwarded-Proto", forwardedProto)
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
			virtualComputersLogManagementError(s, "management proxy failed", err)
			jsonError(w, virtualComputersManagementUnavailable, http.StatusServiceUnavailable)
		}
		proxy.ServeHTTP(w, r)
	}
}

func virtualComputersManagementRequestScope(r *http.Request) string {
	if r != nil && strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return desktopScopeWrite
	}
	if r == nil {
		return desktopScopeRead
	}
	return desktopMethodScope(r.Method)
}

func virtualComputersManagementHealthy(s *Server, cfg virtualcomputers.ToolConfig) bool {
	return virtualComputersManagementHealthProbe(virtualComputersManagementURL)
}

func defaultVirtualComputersManagementHealthProbe(baseURL string) bool {
	parsed, err := url.Parse(virtualcomputers.ManagementHealthURL(strings.TrimSpace(baseURL)))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	client := http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
}

func resetVirtualComputersManagementTunnel() {
	virtualComputersManagementTunnel.Lock()
	defer virtualComputersManagementTunnel.Unlock()
	if virtualComputersManagementTunnel.close != nil {
		virtualComputersManagementTunnel.close()
	}
	virtualComputersManagementTunnel.key = ""
	virtualComputersManagementTunnel.close = nil
}

func virtualComputersLogManagementError(s *Server, message string, err error) {
	if s != nil && s.Logger != nil {
		s.Logger.Warn("[VirtualComputers] "+message, "error", err)
	}
}
