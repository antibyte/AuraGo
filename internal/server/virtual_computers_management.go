package server

import (
	"fmt"
	"net/http"
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
		return fmt.Errorf(virtualComputersManagementUnavailable)
	case virtualcomputers.ControlPlaneSSHHost:
		return virtualComputersEnsureRemoteManagementAccess(s, cfg)
	default:
		return fmt.Errorf(virtualComputersManagementUnavailable)
	}
}

func virtualComputersEnsureRemoteManagementAccess(s *Server, cfg virtualcomputers.ToolConfig) error {
	executor, err := virtualComputersManagementSSHExecutor(s, cfg)
	if err != nil {
		virtualComputersLogManagementError(s, "SSH setup failed", err)
		return fmt.Errorf(virtualComputersManagementUnavailable)
	}
	localAddr, ok := virtualComputersLoopbackListenAddr(virtualComputersManagementURL)
	if !ok {
		virtualComputersLogManagementError(s, "invalid loopback management URL", fmt.Errorf("URL %q is not loopback HTTP(S)", virtualComputersManagementURL))
		return fmt.Errorf(virtualComputersManagementUnavailable)
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
		return fmt.Errorf(virtualComputersManagementUnavailable)
	}
	if !virtualComputersManagementHealthProbe(virtualComputersManagementURL) {
		closeFn()
		virtualComputersLogManagementError(s, "management health probe failed after SSH tunnel start", fmt.Errorf("health URL %s", virtualcomputers.ManagementHealthURL(virtualComputersManagementURL)))
		return fmt.Errorf(virtualComputersManagementUnavailable)
	}
	virtualComputersManagementTunnel.key = key
	virtualComputersManagementTunnel.close = closeFn
	return nil
}

func virtualComputersManagementHealthy(s *Server, cfg virtualcomputers.ToolConfig) bool {
	return virtualComputersEnsureManagementAccess(s, cfg) == nil
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
