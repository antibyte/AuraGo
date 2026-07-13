package server

import (
	"context"
	"errors"
	"reflect"
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
	running    bool
	retryAfter time.Time
}{}

func virtualComputersTriggerAutoSetup(s *Server, cfg virtualcomputers.ToolConfig) bool {
	if !cfg.Enabled || !cfg.AutoSetup {
		return false
	}
	virtualComputersAutoSetupState.Lock()
	if virtualComputersAutoSetupState.running || time.Now().Before(virtualComputersAutoSetupState.retryAfter) {
		virtualComputersAutoSetupState.Unlock()
		return false
	}
	virtualComputersAutoSetupState.running = true
	virtualComputersAutoSetupState.Unlock()

	go func() {
		attempted := false
		var runErr error
		defer func() {
			virtualComputersAutoSetupState.Lock()
			virtualComputersAutoSetupState.running = false
			if attempted && runErr != nil {
				virtualComputersAutoSetupState.retryAfter = time.Now().Add(virtualComputersAutoSetupRetryCooldown)
			} else {
				virtualComputersAutoSetupState.retryAfter = time.Time{}
			}
			virtualComputersAutoSetupState.Unlock()
		}()
		if !virtualComputersAutoSetupNeeded(s, cfg) {
			return
		}
		attempted = true
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
		defer cancel()
		runErr = virtualComputersAutoSetupRunner(ctx, s, cfg)
		if runErr != nil {
			virtualComputersLogManagementError(s, "automatic setup failed", runErr)
			return
		}
		if s != nil && s.Logger != nil {
			s.Logger.Info("[VirtualComputers] Automatic Boring Computers provisioning completed")
		}
	}()
	return true
}

func defaultVirtualComputersAutoSetupNeeded(s *Server, cfg virtualcomputers.ToolConfig) bool {
	if virtualComputersEnsureControlPlaneAccess(s, cfg) != nil {
		return true
	}
	return !virtualComputersManagementHealthy(s, cfg)
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
	virtualComputersAutoSetupState.running = false
	virtualComputersAutoSetupState.retryAfter = time.Time{}
	virtualComputersAutoSetupState.Unlock()
}
