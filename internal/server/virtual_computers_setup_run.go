package server

import (
	"context"
	"sync"
	"time"

	"aurago/internal/security"
	"aurago/internal/virtualcomputers"
)

const virtualComputersRepairProbeTTL = 30 * time.Second

// virtualComputersSetupInstall runs the install. Tests replace it so no host is touched.
var virtualComputersSetupInstall = func(ctx context.Context, m virtualcomputers.SetupManager) (virtualcomputers.SetupStatus, error) {
	return m.Install(ctx)
}

// virtualComputersSetupRun tracks the current / last Install / Repair run (manual or automatic).
var virtualComputersSetupRun struct {
	sync.Mutex
	running     bool
	action      string
	startedAt   time.Time
	finishedAt  time.Time
	lastState   string
	lastMessage string
}

var virtualComputersRepairProbe struct {
	sync.Mutex
	at       time.Time
	url      string
	required bool
}

type virtualComputersSetupRunInfo struct {
	Running     bool   `json:"running"`
	Action      string `json:"action"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
	LastState   string `json:"last_state"`
	LastMessage string `json:"last_message"`
}

func virtualComputersBeginSetupRun(action string) {
	virtualComputersSetupRun.Lock()
	virtualComputersSetupRun.running = true
	virtualComputersSetupRun.action = action
	virtualComputersSetupRun.startedAt = time.Now().UTC()
	virtualComputersSetupRun.finishedAt = time.Time{}
	virtualComputersSetupRun.Unlock()
	resetVirtualComputersRepairProbe()
}

func virtualComputersEndSetupRun(state, message string) {
	virtualComputersSetupRun.Lock()
	virtualComputersSetupRun.running = false
	virtualComputersSetupRun.finishedAt = time.Now().UTC()
	virtualComputersSetupRun.lastState = state
	virtualComputersSetupRun.lastMessage = security.Scrub(message)
	virtualComputersSetupRun.Unlock()
	resetVirtualComputersRepairProbe()
}

func virtualComputersSetupRunning() bool {
	virtualComputersSetupRun.Lock()
	defer virtualComputersSetupRun.Unlock()
	return virtualComputersSetupRun.running
}

func virtualComputersSetupRunSnapshot() virtualComputersSetupRunInfo {
	virtualComputersSetupRun.Lock()
	defer virtualComputersSetupRun.Unlock()
	info := virtualComputersSetupRunInfo{
		Running:     virtualComputersSetupRun.running,
		Action:      virtualComputersSetupRun.action,
		LastState:   virtualComputersSetupRun.lastState,
		LastMessage: virtualComputersSetupRun.lastMessage,
	}
	if !virtualComputersSetupRun.startedAt.IsZero() {
		info.StartedAt = virtualComputersSetupRun.startedAt.Format(time.RFC3339)
	}
	if !virtualComputersSetupRun.finishedAt.IsZero() {
		info.FinishedAt = virtualComputersSetupRun.finishedAt.Format(time.RFC3339)
	}
	return info
}

func resetVirtualComputersRepairProbe() {
	virtualComputersRepairProbe.Lock()
	virtualComputersRepairProbe.at = time.Time{}
	virtualComputersRepairProbe.Unlock()
}

// virtualComputersRepairRequired reports a healthy control plane whose workspace protocol or asset
// fingerprint does not match this AuraGo build. The probe is cached and skipped during a setup run.
func virtualComputersRepairRequired(s *Server, cfg virtualcomputers.ToolConfig, controlPlaneHealthy bool) bool {
	if !controlPlaneHealthy || !cfg.AgentControl.Enabled || virtualComputersSetupRunning() {
		return false
	}
	virtualComputersRepairProbe.Lock()
	if virtualComputersRepairProbe.url == cfg.BoringdURL && !virtualComputersRepairProbe.at.IsZero() &&
		time.Since(virtualComputersRepairProbe.at) < virtualComputersRepairProbeTTL {
		required := virtualComputersRepairProbe.required
		virtualComputersRepairProbe.Unlock()
		return required
	}
	virtualComputersRepairProbe.Unlock()

	required := false
	probed := false
	if client, err := virtualComputersClient(s); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		status, probeErr := client.WorkspaceCapabilities(ctx)
		cancel()
		if probeErr == nil {
			probed = true
			required = status.ProtocolVersion != virtualcomputers.WorkspaceProtocolVersion ||
				status.AssetFingerprint != virtualcomputers.WorkspaceAssetFingerprint()
		}
	}
	if !probed {
		return false
	}
	// A run that started during the probe makes the result stale.
	if virtualComputersSetupRunning() {
		return false
	}
	virtualComputersRepairProbe.Lock()
	virtualComputersRepairProbe.at = time.Now()
	virtualComputersRepairProbe.url = cfg.BoringdURL
	virtualComputersRepairProbe.required = required
	virtualComputersRepairProbe.Unlock()
	return required
}
