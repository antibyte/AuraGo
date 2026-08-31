package virtualcomputers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/uid"
)

const legacyVolumeImportTTL = 10 * time.Minute

type pendingLegacyVolumeImport struct {
	Preview        LegacyVolumeImportPreview
	MachineID      string
	Digest         string
	TargetVolumeID string
	InProgress     bool
}

type legacyVolumeScanRPCResult struct {
	Paths      []string                  `json:"paths"`
	Entries    []LegacyVolumeImportEntry `json:"entries"`
	Digest     string                    `json:"digest"`
	TotalBytes int64                     `json:"total_bytes"`
	FileCount  int                       `json:"file_count"`
}

// BeginLegacyVolumeImport starts an isolated, short-lived migration VM and
// scans selected /root-relative paths. Copying does not begin until an
// authenticated administrator calls ConfirmLegacyVolumeImport.
func (m *WorkspaceManager) BeginLegacyVolumeImport(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, sourceVolumeID string, paths []string) (LegacyVolumeImportPreview, error) {
	m.rememberConfig(cfg)
	if !identity.Admin {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "forbidden", Message: "administrator permission is required to import a legacy volume"}
	}
	if err := validateWorkspaceFeature(cfg, identity); err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	if !cfg.AllowVolumes {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "volumes_disabled", Message: "virtual computer volumes are disabled"}
	}
	if !cfg.AllowInternet {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "internet_disabled", Message: "virtual_computers.allow_internet must be enabled for the migration VM"}
	}
	if err := ValidateWorkspaceNetworkCIDRs(cfg.AgentControl.AllowedPrivateCIDRs); err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	active, err := m.ledger.CountActiveWorkspaces(ctx)
	if err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	m.legacyImportMu.Lock()
	pendingCount := len(m.legacyImports)
	m.legacyImportMu.Unlock()
	if active+pendingCount >= cfg.AgentControl.MaxActiveWorkspaces {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "workspace_limit_reached", Message: "maximum active workspaces and volume imports reached"}
	}
	sourceVolumeID = strings.TrimSpace(sourceVolumeID)
	source, ok, err := m.workspaceVolume(ctx, sourceVolumeID)
	if err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	if !ok {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "not_found", Message: "legacy volume was not found in the AuraGo ledger"}
	}
	if source.Format != "legacy_root" {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "legacy_volume_required", Message: "only legacy_root volumes require import"}
	}
	if source.Availability == "previous_store" || source.Availability == "unavailable" {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "legacy_volume_unavailable", Message: "legacy volume is not available in the active store"}
	}

	client, err := m.clientFactory(cfg)
	if err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	if err := requireWorkspaceControlPlane(ctx, client); err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	if _, err := client.GetVolume(ctx, sourceVolumeID); err != nil {
		return LegacyVolumeImportPreview{}, fmt.Errorf("verify legacy volume: %w", err)
	}
	machine, err := client.LaunchMachine(ctx, LaunchMachineRequest{
		Template: "python", TTLSeconds: int(legacyVolumeImportTTL.Seconds()), AllowInternet: true,
		VolumeID: sourceVolumeID, VolumeFormat: "legacy_root", NetworkProfile: firstNonEmpty(cfg.AgentControl.NetworkProfile, "internet_lan"),
		AllowedPrivateCIDRs: append([]string(nil), cfg.AgentControl.AllowedPrivateCIDRs...), ProtectedCIDRs: localProtectedCIDRs(),
	})
	if err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	keepMachine := false
	defer func() {
		if !keepMachine {
			_ = client.DestroyMachine(context.Background(), machine.ID)
		}
	}()
	transport := m.transportFactory(client)
	handshakeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	capabilities, handshakeErr := WorkspaceHandshake(handshakeCtx, transport, machine.ID, "")
	cancel()
	if handshakeErr != nil || !containsWorkspaceCapability(capabilities.Capabilities, "volume.legacy_import") {
		return LegacyVolumeImportPreview{}, WorkspaceRPCError{Code: "workspace_agent_upgrade_required", Message: "the migration rootfs does not provide legacy volume import support"}
	}
	var scan legacyVolumeScanRPCResult
	if err := transport.Call(ctx, machine.ID, "volume.legacy_scan", map[string]interface{}{"paths": paths}, &scan); err != nil {
		return LegacyVolumeImportPreview{}, err
	}
	now := m.now()
	preview := LegacyVolumeImportPreview{
		ID: uid.NewString(), SourceVolumeID: sourceVolumeID, Paths: append([]string(nil), scan.Paths...),
		Entries: append([]LegacyVolumeImportEntry(nil), scan.Entries...), TotalBytes: scan.TotalBytes,
		FileCount: scan.FileCount, ExpiresAt: now.Add(legacyVolumeImportTTL),
	}
	m.legacyImportMu.Lock()
	m.legacyImports[preview.ID] = &pendingLegacyVolumeImport{Preview: preview, MachineID: machine.ID, Digest: scan.Digest}
	m.legacyImportMu.Unlock()
	keepMachine = true
	return preview, nil
}

// ConfirmLegacyVolumeImport revalidates the scan digest, copies only the
// selected paths to /workspace, and saves a new workspace_v2 volume.
func (m *WorkspaceManager) ConfirmLegacyVolumeImport(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, importID string, ttlSeconds int) (Volume, error) {
	if !identity.Admin {
		return Volume{}, WorkspaceRPCError{Code: "forbidden", Message: "administrator permission is required to confirm a legacy volume import"}
	}
	if err := validateWorkspaceFeature(cfg, identity); err != nil {
		return Volume{}, err
	}
	importID = strings.TrimSpace(importID)
	m.legacyImportMu.Lock()
	pending := m.legacyImports[importID]
	if pending == nil || !pending.Preview.ExpiresAt.After(m.now()) {
		m.legacyImportMu.Unlock()
		return Volume{}, WorkspaceRPCError{Code: "not_found", Message: "legacy volume import preview was not found or has expired"}
	}
	if pending.InProgress {
		m.legacyImportMu.Unlock()
		return Volume{}, WorkspaceRPCError{Code: "legacy_import_in_progress", Message: "legacy volume import is already being confirmed"}
	}
	pending.InProgress = true
	machineID := pending.MachineID
	preview := pending.Preview
	digest := pending.Digest
	targetVolumeID := pending.TargetVolumeID
	m.legacyImportMu.Unlock()

	succeeded := false
	defer func() {
		m.legacyImportMu.Lock()
		if current := m.legacyImports[importID]; current != nil {
			current.InProgress = false
			if succeeded {
				delete(m.legacyImports, importID)
			}
		}
		m.legacyImportMu.Unlock()
	}()
	client, err := m.clientFactory(cfg)
	if err != nil {
		return Volume{}, err
	}
	if err := requireWorkspaceControlPlane(ctx, client); err != nil {
		return Volume{}, err
	}
	transport := m.transportFactory(client)
	var imported legacyVolumeScanRPCResult
	if err := transport.Call(ctx, machineID, "volume.legacy_import", map[string]interface{}{
		"paths": preview.Paths, "expected_digest": digest,
	}, &imported); err != nil {
		return Volume{}, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 30 * 24 * 60 * 60
	}
	var volume Volume
	if targetVolumeID == "" {
		volume, err = client.CreateVolume(ctx, ttlSeconds)
		if err != nil {
			return Volume{}, err
		}
		targetVolumeID = volume.ID
		m.legacyImportMu.Lock()
		if current := m.legacyImports[importID]; current != nil {
			current.TargetVolumeID = targetVolumeID
		}
		m.legacyImportMu.Unlock()
	} else {
		volume.ID = targetVolumeID
	}
	if _, err := client.SaveMachineWithFormat(ctx, machineID, targetVolumeID, WorkspaceVolumeFormat); err != nil {
		return Volume{}, err
	}
	now := m.now()
	volume.ID = targetVolumeID
	volume.Format = WorkspaceVolumeFormat
	volume.Availability = "available"
	volume.VerificationStatus = "verified"
	volume.LastVerifiedAt = &now
	if err := m.ledger.UpsertVolume(ctx, volume); err != nil {
		return Volume{}, err
	}
	succeeded = true
	_ = client.DestroyMachine(context.Background(), machineID)
	return volume, nil
}

func (m *WorkspaceManager) CancelLegacyVolumeImport(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, importID string) error {
	if !identity.Admin {
		return WorkspaceRPCError{Code: "forbidden", Message: "administrator permission is required to cancel a legacy volume import"}
	}
	importID = strings.TrimSpace(importID)
	m.legacyImportMu.Lock()
	pending := m.legacyImports[importID]
	if pending == nil {
		m.legacyImportMu.Unlock()
		return WorkspaceRPCError{Code: "not_found", Message: "legacy volume import preview was not found"}
	}
	if pending.InProgress {
		m.legacyImportMu.Unlock()
		return WorkspaceRPCError{Code: "legacy_import_in_progress", Message: "legacy volume import is currently being confirmed"}
	}
	delete(m.legacyImports, importID)
	m.legacyImportMu.Unlock()
	client, err := m.clientFactory(cfg)
	if err != nil {
		return err
	}
	return client.DestroyMachine(ctx, pending.MachineID)
}

func (m *WorkspaceManager) cleanupLegacyImports(ctx context.Context, all bool) {
	now := m.now()
	m.legacyImportMu.Lock()
	machineIDs := make([]string, 0)
	for id, pending := range m.legacyImports {
		if pending.InProgress || (!all && pending.Preview.ExpiresAt.After(now)) {
			continue
		}
		machineIDs = append(machineIDs, pending.MachineID)
		delete(m.legacyImports, id)
	}
	m.legacyImportMu.Unlock()
	if len(machineIDs) == 0 {
		return
	}
	client, err := m.clientFactory(m.configSnapshot())
	if err != nil {
		return
	}
	for _, machineID := range machineIDs {
		_ = client.DestroyMachine(ctx, machineID)
	}
}

func containsWorkspaceCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}
