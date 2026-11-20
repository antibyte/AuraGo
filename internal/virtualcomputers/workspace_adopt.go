package virtualcomputers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"aurago/internal/security"
	"aurago/internal/uid"
)

// adoptedMachineDeadStatuses are machine states that cannot be handed to the agent.
var adoptedMachineDeadStatuses = map[string]bool{
	"stopped": true, "stopping": true, "exited": true, "expired": true,
	"error": true, "failed": true, "destroyed": true, "dead": true,
}

// AdoptMachine hands a running, user-launched machine to the agent as a
// workspace of origin "adopted". The guest handshake runs before any ledger
// write and a failure never touches the machine.
func (m *WorkspaceManager) AdoptMachine(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, machineID string) (Workspace, bool, error) {
	m.rememberConfig(cfg)
	if err := validateWorkspaceFeature(cfg, identity); err != nil {
		return Workspace{}, false, err
	}
	if !identity.Admin {
		return Workspace{}, false, WorkspaceRPCError{Code: "forbidden", Message: "administrator permission is required to assign a computer to the agent"}
	}
	machineID = strings.TrimSpace(machineID)
	if machineID == "" {
		return Workspace{}, false, WorkspaceRPCError{Code: "invalid_argument", Message: "machine id is required"}
	}
	m.adoptMu.Lock()
	defer m.adoptMu.Unlock()

	reopenID := ""
	existing, found, err := m.ledger.WorkspaceByMachine(ctx, machineID)
	if err != nil {
		return Workspace{}, false, err
	}
	if found {
		if workspaceStateActive(existing.State) {
			if existing.Origin == WorkspaceOriginAdopted && existing.OwnerSessionID == AdoptedWorkspaceOwnerSessionID &&
				existing.MissionID == "" && existing.State == WorkspaceStateReady {
				return existing, true, nil
			}
			return Workspace{}, false, WorkspaceRPCError{Code: "machine_owned_by_workspace", Message: "this computer already belongs to agent workspace " + existing.ID}
		}
		reopenID = existing.ID
	}
	active, err := m.ledger.CountActiveWorkspaces(ctx)
	if err != nil {
		return Workspace{}, false, err
	}
	if active >= cfg.AgentControl.MaxActiveWorkspaces {
		return Workspace{}, false, WorkspaceRPCError{Code: "workspace_limit_reached", Message: "maximum active agent workspaces reached; close or release one first"}
	}
	client, err := m.clientFactory(cfg)
	if err != nil {
		return Workspace{}, false, err
	}
	if err := requireWorkspaceControlPlane(ctx, client); err != nil {
		return Workspace{}, false, err
	}
	machine, err := client.GetMachine(ctx, machineID)
	if err != nil {
		var restErr RESTError
		if errors.As(err, &restErr) && restErr.StatusCode == http.StatusNotFound {
			return Workspace{}, false, WorkspaceRPCError{Code: "not_found", Message: "computer was not found"}
		}
		return Workspace{}, false, err
	}
	if adoptedMachineDeadStatuses[strings.ToLower(strings.TrimSpace(machine.Status))] {
		return Workspace{}, false, WorkspaceRPCError{Code: "machine_not_running", Message: "the computer is not running"}
	}
	handshakeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	capabilities, handshakeErr := WorkspaceHandshake(handshakeCtx, m.transportFactory(client), machineID, "")
	cancel()
	if handshakeErr != nil {
		m.reportIssue(WorkspaceOperationalIssue{Kind: "guest_protocol", Detail: security.Scrub(handshakeErr.Error()), Severity: "warning"})
		return Workspace{}, false, WorkspaceRPCError{
			Code:    "workspace_agent_upgrade_required",
			Message: "This computer was started from an image without the current AuraGo workspace agent. Run Install / Repair, then start a new computer.",
		}
	}

	now := m.now()
	template := machine.Template
	if template != "desktop" && template != "python" {
		template = "python"
		if machine.Display {
			template = "desktop"
		}
	}
	volumeID := ""
	if meta, metaErr := m.ledger.MachineMetadata(ctx); metaErr == nil {
		if id := meta[machineID].VolumeID; id != "" {
			if volume, ok, volErr := m.ledger.GetVolume(ctx, id); volErr == nil && ok && volume.Format == WorkspaceVolumeFormat {
				volumeID = id
			}
		}
	}
	maxExpires := now.AddDate(100, 0, 0)
	if !machine.Persistent && machine.ExpiresAt != nil {
		maxExpires = machine.ExpiresAt.UTC()
	}
	lease := now.Add(time.Duration(cfg.AgentControl.IdleTTLSeconds) * time.Second)
	if lease.After(maxExpires) {
		lease = maxExpires
	}
	workspace := Workspace{
		ID: firstNonEmpty(reopenID, uid.NewString()), OwnerSessionID: AdoptedWorkspaceOwnerSessionID, MissionID: "",
		Actor: firstNonEmpty(identity.Actor, "user"), MachineID: machineID, State: WorkspaceStateReady,
		Template: template, NetworkProfile: WorkspaceNetworkProfileMachine, VolumeID: volumeID,
		Capabilities: append([]string(nil), capabilities.Capabilities...), InstanceNonce: capabilities.InstanceNonce,
		ControlOwner: ControlOwnerAgent, CreatedAt: now, UpdatedAt: now, LastActivityAt: now,
		LeaseExpiresAt: lease, MaxExpiresAt: maxExpires, Origin: WorkspaceOriginAdopted,
	}
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Workspace{}, false, WorkspaceRPCError{Code: "machine_owned_by_workspace", Message: "this computer already belongs to an agent workspace"}
		}
		return Workspace{}, false, err
	}
	if reopenID != "" {
		m.forgetGUI(reopenID)
	}
	_ = m.recordEvent(ctx, workspace.ID, "workspace_adopted", "Computer assigned to the agent", map[string]interface{}{
		"machine_id": machineID, "template": template, "reopened": reopenID != "",
	})
	return workspace, false, nil
}

// forgetGUI drops cached GUI gate state so a reopened row starts unblocked.
func (m *WorkspaceManager) forgetGUI(workspaceID string) {
	m.guiMu.Lock()
	delete(m.gui, workspaceID)
	m.guiMu.Unlock()
}

// releaseAdoptedWorkspace ends the agent's use of a user-owned machine. The
// machine, its managed browser and its jobs' host processes stay untouched:
// no DestroyMachine, no guest workspace.close, no machine extension.
func (m *WorkspaceManager) releaseAdoptedWorkspace(ctx context.Context, cfg ToolConfig, workspace Workspace) error {
	if workspace.State == WorkspaceStateClosed || workspace.State == WorkspaceStateFailed || workspace.State == WorkspaceStateLost {
		if workspace.State != WorkspaceStateClosed {
			workspace.State = WorkspaceStateClosed
			workspace.LastError = ""
			workspace.UpdatedAt = m.now()
			return m.ledger.UpsertWorkspace(ctx, workspace)
		}
		return nil
	}
	// closeWorkspace=false: true would destroy the machine when the guest is silent.
	releaseGUI, _ := m.revokeGUI(ctx, cfg, workspace, false)
	if releaseGUI != nil {
		defer releaseGUI()
	}
	workspace.State = WorkspaceStateClosing
	workspace.UpdatedAt = m.now()
	_ = m.ledger.UpsertWorkspace(ctx, workspace)

	if client, err := m.clientFactory(cfg); err == nil && client != nil {
		transport := m.transportFactory(client)
		rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if jobs, listErr := m.ledger.ListWorkspaceJobs(rpcCtx, workspace.ID, 500); listErr == nil {
			for _, job := range jobs {
				if job.State == JobStateQueued || job.State == JobStateRunning {
					_ = transport.Call(rpcCtx, workspace.MachineID, "job.cancel", map[string]string{"job_id": job.ID}, nil)
				}
			}
		}
		if grants, listErr := m.ledger.ListCredentialGrants(rpcCtx, workspace.ID); listErr == nil {
			for _, grant := range grants {
				if grant.Status == GrantActive {
					_ = transport.Call(rpcCtx, workspace.MachineID, "credential.revoke", map[string]string{"grant_id": grant.ID}, nil)
				}
			}
		}
		cancel()
	}
	now := m.now()
	_ = m.ledger.InterruptActiveWorkspaceJobs(ctx, workspace.ID, "workspace released")
	m.closeBrowserSessions(ctx, workspace.ID)
	m.revokeWorkspaceGrants(ctx, workspace.ID, now)
	workspace.ControlOwner = ControlOwnerAgent
	workspace.ControlLeaseExpiresAt = nil
	workspace.State = WorkspaceStateClosed
	workspace.LastError = ""
	workspace.UpdatedAt = now
	workspace.LastActivityAt = now
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		return err
	}
	_ = m.recordEvent(ctx, workspace.ID, "workspace_released", "Computer released from the agent", map[string]interface{}{"machine_id": workspace.MachineID})
	return nil
}

// markWorkspaceLost records that the machine behind a workspace is gone.
func (m *WorkspaceManager) markWorkspaceLost(ctx context.Context, workspace Workspace, reason string) {
	workspace.State = WorkspaceStateLost
	workspace.LastError = reason
	workspace.UpdatedAt = m.now()
	_ = m.ledger.UpsertWorkspace(ctx, workspace)
	_ = m.ledger.InterruptActiveWorkspaceJobs(ctx, workspace.ID, reason)
	m.closeBrowserSessions(ctx, workspace.ID)
	m.revokeWorkspaceGrants(ctx, workspace.ID, m.now())
	_ = m.recordEvent(ctx, workspace.ID, "workspace_lost", reason, nil)
	m.reportIssue(WorkspaceOperationalIssue{Kind: "machine_lost", WorkspaceID: workspace.ID, Detail: reason, Severity: "error"})
}

// nextWorkspaceLease is now+idle TTL, never past the workspace hard expiry.
func nextWorkspaceLease(cfg ToolConfig, workspace Workspace, now time.Time) time.Time {
	lease := now.Add(time.Duration(cfg.AgentControl.IdleTTLSeconds) * time.Second)
	if !workspace.MaxExpiresAt.IsZero() && lease.After(workspace.MaxExpiresAt) {
		lease = workspace.MaxExpiresAt
	}
	return lease
}

// ActiveWorkspaceForMachine reports the active workspace that owns a machine.
func (m *WorkspaceManager) ActiveWorkspaceForMachine(ctx context.Context, machineID string) (Workspace, bool, error) {
	return m.ledger.ActiveWorkspaceForMachine(ctx, machineID)
}
