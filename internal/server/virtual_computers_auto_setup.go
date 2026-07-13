package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

const virtualComputersAutoSetupRetryCooldown = 5 * time.Minute

var (
	virtualComputersAutoSetupNeeded = defaultVirtualComputersAutoSetupNeeded
	virtualComputersAutoSetupRunner = runVirtualComputersAutoSetup
)

var virtualComputersAutoSetupState = struct {
	sync.Mutex
	running          bool
	retryAfter       time.Time
	generation       uint64
	desiredServer    *Server
	desiredConfig    virtualcomputers.ToolConfig
	cancel           context.CancelFunc
	cancelGeneration uint64
}{}

func virtualComputersTriggerAutoSetup(s *Server, cfg virtualcomputers.ToolConfig) bool {
	enabled := cfg.Enabled && cfg.AutoSetup
	virtualComputersAutoSetupState.Lock()
	configChanged := virtualComputersAutoSetupState.desiredConfig != cfg
	if !enabled {
		virtualComputersAutoSetupState.generation++
		virtualComputersAutoSetupState.desiredServer = s
		virtualComputersAutoSetupState.desiredConfig = cfg
		virtualComputersAutoSetupState.retryAfter = time.Time{}
		cancel := virtualComputersAutoSetupState.cancel
		virtualComputersAutoSetupState.Unlock()
		if cancel != nil {
			cancel()
		}
		resetVirtualComputersManagementTunnel()
		clearWebhostsCache()
		return false
	}
	if virtualComputersAutoSetupState.running {
		if !configChanged {
			virtualComputersAutoSetupState.Unlock()
			return false
		}
		virtualComputersAutoSetupState.generation++
		virtualComputersAutoSetupState.desiredServer = s
		virtualComputersAutoSetupState.desiredConfig = cfg
		cancel := virtualComputersAutoSetupState.cancel
		virtualComputersAutoSetupState.Unlock()
		if cancel != nil {
			cancel()
		}
		return false
	}
	if !configChanged && time.Now().Before(virtualComputersAutoSetupState.retryAfter) {
		virtualComputersAutoSetupState.Unlock()
		return false
	}
	virtualComputersAutoSetupState.generation++
	virtualComputersAutoSetupState.desiredServer = s
	virtualComputersAutoSetupState.desiredConfig = cfg
	virtualComputersAutoSetupState.running = true
	virtualComputersAutoSetupState.Unlock()

	go reconcileVirtualComputersAutoSetup()
	return true
}

func reconcileVirtualComputersAutoSetup() {
	for {
		virtualComputersAutoSetupState.Lock()
		generation := virtualComputersAutoSetupState.generation
		s := virtualComputersAutoSetupState.desiredServer
		cfg := virtualComputersAutoSetupState.desiredConfig
		if !cfg.Enabled || !cfg.AutoSetup {
			virtualComputersAutoSetupState.running = false
			virtualComputersAutoSetupState.cancel = nil
			virtualComputersAutoSetupState.cancelGeneration = 0
			virtualComputersAutoSetupState.Unlock()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
		virtualComputersAutoSetupState.cancel = cancel
		virtualComputersAutoSetupState.cancelGeneration = generation
		virtualComputersAutoSetupState.Unlock()

		attempted := virtualComputersAutoSetupNeeded(s, cfg)
		var runErr error
		if attempted {
			runErr = virtualComputersAutoSetupRunner(ctx, s, cfg)
		}
		cancel()

		virtualComputersAutoSetupState.Lock()
		if virtualComputersAutoSetupState.cancelGeneration == generation {
			virtualComputersAutoSetupState.cancel = nil
			virtualComputersAutoSetupState.cancelGeneration = 0
		}
		if virtualComputersAutoSetupState.generation != generation {
			virtualComputersAutoSetupState.Unlock()
			continue
		}
		virtualComputersAutoSetupState.running = false
		if attempted && runErr != nil {
			virtualComputersAutoSetupState.retryAfter = time.Now().Add(virtualComputersAutoSetupRetryCooldown)
		} else {
			virtualComputersAutoSetupState.retryAfter = time.Time{}
		}
		virtualComputersAutoSetupState.Unlock()

		if runErr != nil {
			virtualComputersLogManagementError(s, "automatic setup failed", runErr)
		} else if attempted && s != nil && s.Logger != nil {
			s.Logger.Info("[VirtualComputers] Automatic Boring Computers provisioning completed")
		}
		return
	}
}

func defaultVirtualComputersAutoSetupNeeded(s *Server, cfg virtualcomputers.ToolConfig) bool {
	if virtualComputersEnsureControlPlaneAccess(s, cfg) != nil {
		return true
	}
	if virtualComputersEnsureManagementAccess(s, cfg) != nil {
		return true
	}
	return !virtualComputersManagementRevisionMatches(virtualComputersManagementURL)
}

func virtualComputersManagementRevisionMatches(baseURL string) bool {
	client := http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(virtualcomputers.ManagementRevisionURL(baseURL))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	return err == nil && strings.TrimSpace(string(body)) == virtualcomputers.PinnedUpstreamRevision
}

func runVirtualComputersAutoSetup(ctx context.Context, s *Server, cfg virtualcomputers.ToolConfig) error {
	token, _, err := virtualComputersEnsureBoringToken(s, cfg)
	if err != nil {
		return errors.New("prepare automatic setup credentials: " + err.Error())
	}
	manager, err := virtualComputersSetupManager(s, cfg, token)
	if err != nil {
		return errors.New("prepare automatic setup: " + err.Error())
	}
	manager.InstallOptions = virtualComputersSetupOptions(cfg, token, virtualComputersSetupRequest{})
	status, err := manager.Install(ctx)
	if err != nil {
		return errors.New(manager.RedactInstallLog(err.Error()))
	}
	if !status.Healthy {
		return errors.New(manager.RedactInstallLog(status.Message))
	}
	resetVirtualComputersManagementTunnel()
	clearWebhostsCache()
	return nil
}

func virtualComputersAutoSetupAfterConfigChange(s *Server, oldCfg, newCfg config.Config) {
	if reflect.DeepEqual(oldCfg.VirtualComputers, newCfg.VirtualComputers) {
		return
	}
	virtualComputersTriggerAutoSetup(s, virtualcomputers.FromAuraConfig(&newCfg))
}

func resetVirtualComputersAutoSetupState() {
	virtualComputersAutoSetupState.Lock()
	cancel := virtualComputersAutoSetupState.cancel
	virtualComputersAutoSetupState.running = false
	virtualComputersAutoSetupState.retryAfter = time.Time{}
	virtualComputersAutoSetupState.generation++
	virtualComputersAutoSetupState.desiredServer = nil
	virtualComputersAutoSetupState.desiredConfig = virtualcomputers.ToolConfig{}
	virtualComputersAutoSetupState.cancel = nil
	virtualComputersAutoSetupState.cancelGeneration = 0
	virtualComputersAutoSetupState.Unlock()
	if cancel != nil {
		cancel()
	}
}
