package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"aurago/internal/virtualcomputers"
)

// virtualComputersManualSetup serialises manual Install / Repair runs. The
// auto-setup runner has its own state; both are checked before a run starts.
var virtualComputersManualSetup sync.Mutex

const virtualComputersDefaultMaxForks = 3

func virtualComputersMaxForks(cfg virtualcomputers.ToolConfig) int {
	if cfg.MaxForks <= 0 {
		return virtualComputersDefaultMaxForks
	}
	return cfg.MaxForks
}

func virtualComputersMaxTTL(cfg virtualcomputers.ToolConfig) int {
	if cfg.MaxTTLSeconds <= 0 {
		return virtualcomputers.MaxTTLSeconds
	}
	return cfg.MaxTTLSeconds
}

func virtualComputersActiveWorkspaceState(state string) bool {
	switch state {
	case virtualcomputers.WorkspaceStateOpening, virtualcomputers.WorkspaceStateReady, virtualcomputers.WorkspaceStateClosing:
		return true
	}
	return false
}

// virtualComputersMachineList syncs the ledger, prunes vanished machines and
// merges AuraGo labels, volumes and workspace ownership into the response.
func virtualComputersMachineList(s *Server, w http.ResponseWriter, r *http.Request, client *virtualcomputers.Client) {
	machines, err := client.ListMachines(r.Context())
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		ids := make([]string, 0, len(machines))
		for _, machine := range machines {
			ids = append(ids, machine.ID)
			if err := ledger.UpsertMachine(r.Context(), machine); err != nil {
				return err
			}
		}
		// Only a successful list may prune: a failed list must never erase labels.
		if err := ledger.PruneMachines(r.Context(), ids); err != nil {
			return err
		}
		meta, err := ledger.MachineMetadata(r.Context())
		if err != nil {
			return err
		}
		workspaces, err := ledger.ListWorkspaces(r.Context(), "", false, 500)
		if err != nil {
			return err
		}
		owners := make(map[string]virtualcomputers.Workspace, len(workspaces))
		for _, workspace := range workspaces {
			if virtualComputersActiveWorkspaceState(workspace.State) {
				owners[workspace.MachineID] = workspace
			}
		}
		for i := range machines {
			if m, ok := meta[machines[i].ID]; ok {
				machines[i].Label, machines[i].VolumeID = m.Label, m.VolumeID
			}
			if owner, ok := owners[machines[i].ID]; ok {
				machines[i].WorkspaceID, machines[i].WorkspaceOrigin = owner.ID, owner.Origin
			}
		}
		return nil
	})
	if machines == nil {
		machines = []virtualcomputers.Machine{}
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "machines": machines})
}

// virtualComputersMachineLaunch validates and launches a machine.
func virtualComputersMachineLaunch(s *Server, w http.ResponseWriter, r *http.Request, client *virtualcomputers.Client) {
	var req virtualcomputers.LaunchMachineRequest
	if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(req.Volumes) > 1 {
		writeVirtualComputersAPIError(w, "invalid_argument", "boringd supports at most one volume_id per machine", http.StatusBadRequest)
		return
	}
	if req.VolumeID == "" && len(req.Volumes) == 1 {
		req.VolumeID = req.Volumes[0]
	}
	req.VolumeID = strings.TrimSpace(req.VolumeID)
	cfg := virtualComputersConfigSnapshot(s)
	label, err := virtualComputersNormalizeLabel(req.Name)
	if err != nil {
		writeVirtualComputersAPIError(w, "invalid_name", err.Error(), http.StatusBadRequest)
		return
	}
	if req.AllowInternet && !cfg.AllowInternet {
		writeVirtualComputersAPIError(w, "internet_disabled", "internet access for virtual computers is disabled", http.StatusForbidden)
		return
	}
	if req.Persistent && !cfg.AllowPersistent {
		writeVirtualComputersAPIError(w, "persistent_disabled", "persistent virtual computers are disabled", http.StatusForbidden)
		return
	}
	if req.TTLSeconds < 0 {
		writeVirtualComputersAPIError(w, "invalid_argument", "ttl_seconds must not be negative", http.StatusBadRequest)
		return
	}
	if req.TTLSeconds == 0 {
		req.TTLSeconds = cfg.DefaultTTLSeconds
		if req.TTLSeconds <= 0 {
			req.TTLSeconds = virtualcomputers.DefaultTTLSeconds
		}
		if max := virtualComputersMaxTTL(cfg); req.TTLSeconds > max {
			req.TTLSeconds = max
		}
	}
	if max := virtualComputersMaxTTL(cfg); !req.Persistent && req.TTLSeconds > max {
		writeVirtualComputersAPIErrorDetails(w, "ttl_too_long", "ttl_seconds exceeds the configured maximum", http.StatusBadRequest, map[string]interface{}{"max_ttl_seconds": max})
		return
	}
	// The client's volume_format is never trusted; the ledger decides it.
	req.VolumeFormat = ""
	if req.VolumeID != "" {
		if !cfg.AllowVolumes {
			jsonError(w, "virtual computer volumes are disabled", http.StatusForbidden)
			return
		}
		ledger, owned, _ := virtualComputersLedger(s)
		if owned {
			defer ledger.Close()
		}
		var volume virtualcomputers.Volume
		var found bool
		if ledger != nil {
			volume, found, _ = ledger.GetVolume(r.Context(), req.VolumeID)
		}
		if !found {
			writeVirtualComputersAPIError(w, "volume_not_found", "volume is not known to AuraGo; import it first", http.StatusNotFound)
			return
		}
		if volume.Availability != "" && volume.Availability != "available" {
			writeVirtualComputersAPIError(w, "volume_unavailable", "volume is not available", http.StatusConflict)
			return
		}
		if volume.Format == virtualcomputers.WorkspaceVolumeFormat {
			req.VolumeFormat = virtualcomputers.WorkspaceVolumeFormat
		}
	}
	if req.Template != "" {
		listCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		templates, listErr := client.ListTemplates(listCtx)
		cancel()
		if listErr == nil && len(templates) > 0 {
			known := false
			for _, template := range templates {
				if template.ID == req.Template {
					known = true
					break
				}
			}
			if !known {
				writeVirtualComputersAPIError(w, "invalid_template", "unknown template", http.StatusBadRequest)
				return
			}
		}
	}
	machine, err := client.LaunchMachine(r.Context(), req)
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	virtualComputersUpsertMachine(s, r, machine)
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		if label != "" {
			if err := ledger.SetMachineLabel(r.Context(), machine.ID, label); err != nil {
				return err
			}
		}
		if req.VolumeID != "" {
			return ledger.SetMachineVolume(r.Context(), machine.ID, req.VolumeID)
		}
		return nil
	})
	machine.Label, machine.VolumeID = label, req.VolumeID
	virtualComputersRecordAction(s, r, "launch", "machine", machine.ID, map[string]interface{}{"template": machine.Template})
	writeJSON(w, map[string]interface{}{"status": "ok", "machine": machine})
}

func virtualComputersMachineRename(s *Server, w http.ResponseWriter, r *http.Request, machineID string) {
	if !virtualComputersMutationAllowed(s, w) {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeVirtualComputersJSON(w, r, &body); err != nil {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	label, err := virtualComputersNormalizeLabel(body.Name)
	if err != nil {
		writeVirtualComputersAPIError(w, "invalid_name", err.Error(), http.StatusBadRequest)
		return
	}
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.SetMachineLabel(r.Context(), machineID, label)
	})
	virtualComputersRecordAction(s, r, "rename_machine", "machine", machineID, map[string]interface{}{"label": label})
	writeJSON(w, map[string]interface{}{"status": "ok", "machine_id": machineID, "label": label})
}

// virtualComputersMachineOwner returns the active workspace that owns a machine.
func virtualComputersMachineOwner(s *Server, r *http.Request, machineID string) (virtualcomputers.Workspace, bool) {
	var owner virtualcomputers.Workspace
	var found bool
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		var err error
		owner, found, err = ledger.ActiveWorkspaceForMachine(r.Context(), machineID)
		return err
	})
	return owner, found
}

func virtualComputersMachineDestroy(s *Server, w http.ResponseWriter, r *http.Request, machineID string, client *virtualcomputers.Client) {
	if !virtualComputersMutationAllowed(s, w) {
		return
	}
	// Opening a new ledger here could mistake a missing ownership database for an empty one.
	ledger := s.VirtualComputersDB
	if ledger == nil {
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	owner, found, err := ledger.ActiveWorkspaceForMachine(r.Context(), machineID)
	if err != nil {
		virtualComputersLogLedgerError(s, err)
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	if found {
		writeVirtualComputersAPIErrorDetails(w, "machine_owned_by_workspace",
			"this computer belongs to an agent workspace; close or release the workspace instead", http.StatusConflict,
			map[string]interface{}{"workspace_id": owner.ID, "origin": owner.Origin})
		return
	}
	if err := client.DestroyMachine(r.Context(), machineID); err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	virtualComputersDeleteMachine(s, r, machineID)
	virtualComputersRecordAction(s, r, "destroy", "machine", machineID, nil)
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

func virtualComputersMachineExtend(s *Server, w http.ResponseWriter, r *http.Request, machineID string, client *virtualcomputers.Client) {
	if !virtualComputersMutationAllowed(s, w) {
		return
	}
	var req struct {
		TTLSeconds int `json:"ttl_seconds"`
	}
	if err := decodeVirtualComputersJSON(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	cfg := virtualComputersConfigSnapshot(s)
	max := virtualComputersMaxTTL(cfg)
	if req.TTLSeconds == 0 {
		req.TTLSeconds = cfg.DefaultTTLSeconds
		if req.TTLSeconds <= 0 {
			req.TTLSeconds = virtualcomputers.DefaultTTLSeconds
		}
		if req.TTLSeconds > max {
			req.TTLSeconds = max
		}
	}
	if req.TTLSeconds < 1 || req.TTLSeconds > max {
		writeVirtualComputersAPIErrorDetails(w, "ttl_too_long", "ttl_seconds must be between 1 and the configured maximum", http.StatusBadRequest, map[string]interface{}{"max_ttl_seconds": max})
		return
	}
	machine, err := client.ExtendMachine(r.Context(), machineID, req.TTLSeconds)
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	virtualComputersUpsertMachine(s, r, machine)
	// An adopted workspace follows the machine expiry the user just moved.
	if owner, ok := virtualComputersMachineOwner(s, r, machineID); ok && owner.Origin == virtualcomputers.WorkspaceOriginAdopted && machine.ExpiresAt != nil {
		virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
			return ledger.UpdateWorkspaceMaxExpiry(r.Context(), owner.ID, machine.ExpiresAt.UTC())
		})
	}
	virtualComputersRecordAction(s, r, "extend", "machine", machine.ID, map[string]interface{}{"ttl_seconds": machine.TTLSeconds})
	writeJSON(w, map[string]interface{}{"status": "ok", "machine": machine})
}

// virtualComputersSafeDownloadName reduces a guest path to a harmless file name.
func virtualComputersSafeDownloadName(remotePath string) string {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(remotePath), "\\", "/"))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := strings.Trim(b.String(), ".")
	if name == "" || name == "_" {
		return "download"
	}
	return name
}

func virtualComputersMachineDownload(s *Server, w http.ResponseWriter, r *http.Request, machineID string, client *virtualcomputers.Client) {
	remotePath := r.URL.Query().Get("path")
	data, _, err := client.Download(r.Context(), machineID, remotePath)
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	// Guest files are untrusted: never let the browser render them inline.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+virtualComputersSafeDownloadName(remotePath)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func virtualComputersMachineAssignAgent(s *Server, w http.ResponseWriter, r *http.Request, machineID string) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Handing a machine to the agent is an admin decision, not just a write.
	if !authenticateDesktopPermission(s, w, r, desktopScopeAdmin) {
		return
	}
	if !virtualComputersMutationAllowed(s, w) {
		return
	}
	if virtualComputersSetupRunning() {
		writeVirtualComputersAPIError(w, "setup_in_progress", "Install / Repair is running. Wait until it has finished, then start a new computer.", http.StatusConflict)
		return
	}
	manager := virtualWorkspaceManager(s)
	if manager == nil {
		writeVirtualComputersAPIError(w, "workspace_manager_unavailable", "virtual workspace manager is unavailable", http.StatusServiceUnavailable)
		return
	}
	cfg := virtualComputersConfigSnapshot(s)
	identity := virtualWorkspaceIdentity()
	identity.Admin = true // verified by the admin scope gate above
	workspace, reused, err := manager.AdoptMachine(r.Context(), cfg, identity, machineID)
	if err != nil {
		writeVirtualWorkspaceError(w, err)
		return
	}
	virtualComputersRecordAction(s, r, "assign_machine_to_agent", "machine", machineID, map[string]interface{}{"workspace_id": workspace.ID, "reused": reused})
	writeJSON(w, map[string]interface{}{"status": "ok", "workspace": workspace, "reused": reused})
}

// virtualComputersAutoSetupRunning reports whether the auto-setup runner is active.
func virtualComputersAutoSetupRunning() bool {
	virtualComputersAutoSetupState.Lock()
	defer virtualComputersAutoSetupState.Unlock()
	return virtualComputersAutoSetupState.running
}
