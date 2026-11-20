package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"aurago/internal/virtualcomputers"
)

const (
	virtualComputersVolumeDefaultTTLSeconds = 86400
	virtualComputersRetentionRepairMessage  = "Run Install / Repair in Virtual Computers settings to enable permanent and locked volumes"
)

// virtualComputersNormalizeLabel validates a user-chosen name for a machine or
// volume: trimmed, at most 64 runes, no control characters. Empty is allowed.
func virtualComputersNormalizeLabel(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) > 64 {
		return "", errors.New("name must be at most 64 characters")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", errors.New("name must not contain control characters")
		}
	}
	return value, nil
}

// virtualComputersVolumeRetention reports whether the control plane runs the
// volume retention patch. The probe is cached by the virtualcomputers package.
func virtualComputersVolumeRetention(ctx context.Context, client *virtualcomputers.Client) bool {
	if client == nil {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return virtualcomputers.CachedVolumeRetention(probeCtx, client)
}

// writeVirtualComputersAPIErrorDetails is writeVirtualComputersAPIError plus
// extra top-level fields (for example attached_to).
func writeVirtualComputersAPIErrorDetails(w http.ResponseWriter, code, message string, status int, details map[string]interface{}) {
	payload := map[string]interface{}{}
	for key, value := range details {
		payload[key] = value
	}
	payload["status"] = "error"
	payload["code"] = code
	payload["error"] = message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func virtualComputersVolumeMaxTTL(cfg virtualcomputers.ToolConfig) int {
	return cfg.VolumeMaxTTL()
}

func virtualComputersVolumeDefaultTTL(cfg virtualcomputers.ToolConfig) int {
	ttl := virtualComputersVolumeDefaultTTLSeconds
	if max := virtualComputersVolumeMaxTTL(cfg); ttl > max {
		ttl = max
	}
	return ttl
}

// virtualComputersRunningMachineSet lists machine IDs boringd still knows. A
// nil result means the list could not be read and callers must stay careful.
func virtualComputersRunningMachineSet(ctx context.Context, client *virtualcomputers.Client) map[string]bool {
	if client == nil {
		return nil
	}
	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	machines, err := client.ListMachines(listCtx)
	if err != nil {
		return nil
	}
	set := make(map[string]bool, len(machines))
	for _, machine := range machines {
		set[machine.ID] = true
	}
	return set
}

// virtualComputersVolumeAttachments lists active consumers of a volume. Machine
// references that boringd no longer lists are stale and ignored.
func virtualComputersVolumeAttachments(ctx context.Context, ledger *virtualcomputers.Ledger, volumeID string, machines map[string]bool) []virtualcomputers.VolumeAttachment {
	var out []virtualcomputers.VolumeAttachment
	if ledger == nil {
		return out
	}
	if workspaces, err := ledger.ActiveWorkspacesForVolume(ctx, volumeID); err == nil {
		for _, workspace := range workspaces {
			out = append(out, virtualcomputers.VolumeAttachment{Kind: "workspace", ID: workspace.ID})
		}
	}
	if ids, err := ledger.MachinesWithVolume(ctx, volumeID); err == nil {
		for _, id := range ids {
			if machines == nil || machines[id] {
				out = append(out, virtualcomputers.VolumeAttachment{Kind: "machine", ID: id})
			}
		}
	}
	return out
}

func virtualComputersVolumeList(s *Server, w http.ResponseWriter, r *http.Request, client *virtualcomputers.Client) {
	ledger, owned, err := virtualComputersLedger(s)
	if err != nil || ledger == nil {
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	if owned {
		defer ledger.Close()
	}
	volumes, err := virtualcomputers.ListTrackedVolumes(r.Context(), ledger, client)
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	machines := virtualComputersRunningMachineSet(r.Context(), client)
	for i := range volumes {
		volumes[i].AttachedTo = virtualComputersVolumeAttachments(r.Context(), ledger, volumes[i].ID, machines)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "volumes": volumes})
}

func virtualComputersVolumeCreate(s *Server, w http.ResponseWriter, r *http.Request, cfg virtualcomputers.ToolConfig, client *virtualcomputers.Client) {
	if cfg.ReadOnly {
		jsonError(w, "virtual computers are read-only", http.StatusForbidden)
		return
	}
	var req struct {
		Name       string `json:"name"`
		TTLSeconds int    `json:"ttl_seconds"`
		Persistent bool   `json:"persistent"`
		Locked     bool   `json:"locked"`
	}
	if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	name, err := virtualComputersNormalizeLabel(req.Name)
	if err != nil {
		writeVirtualComputersAPIError(w, "invalid_name", err.Error(), http.StatusBadRequest)
		return
	}
	ttl := req.TTLSeconds
	if !req.Persistent {
		if ttl <= 0 {
			ttl = virtualComputersVolumeDefaultTTL(cfg)
		}
		if ttl > virtualComputersVolumeMaxTTL(cfg) {
			writeVirtualComputersAPIError(w, "volume_ttl_too_long", "volume retention exceeds the configured maximum", http.StatusBadRequest)
			return
		}
	} else {
		ttl = 0
	}
	if (req.Persistent || req.Locked) && !virtualComputersVolumeRetention(r.Context(), client) {
		writeVirtualComputersAPIError(w, "volume_retention_requires_repair", virtualComputersRetentionRepairMessage, http.StatusConflict)
		return
	}
	volume, err := client.CreateVolumeWithOptions(r.Context(), virtualcomputers.VolumeCreateOptions{TTLSeconds: ttl, Persistent: req.Persistent, Locked: req.Locked})
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	now := time.Now().UTC()
	volume.Name = name
	volume.LastVerifiedAt = &now
	volume.VerificationStatus = "verified"
	virtualComputersUpsertVolume(s, r, volume)
	virtualComputersRecordAction(s, r, "create_volume", "volume", volume.ID, map[string]interface{}{"persistent": req.Persistent, "locked": req.Locked})
	writeJSON(w, map[string]interface{}{"status": "ok", "volume": volume})
}

func virtualComputersVolumeGet(s *Server, w http.ResponseWriter, r *http.Request, volumeID string, client *virtualcomputers.Client) {
	volume, err := client.GetVolume(r.Context(), volumeID)
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		tracked, trackErr := virtualcomputers.TrackUpstreamVolume(r.Context(), ledger, volume)
		if trackErr == nil {
			volume = tracked
		}
		return trackErr
	})
	writeJSON(w, map[string]interface{}{"status": "ok", "volume": volume})
}

func virtualComputersVolumePatch(s *Server, w http.ResponseWriter, r *http.Request, cfg virtualcomputers.ToolConfig, volumeID string, client *virtualcomputers.Client) {
	if !virtualComputersMutationAllowed(s, w) {
		return
	}
	var req struct {
		Name       *string `json:"name"`
		TTLSeconds *int    `json:"ttl_seconds"`
		Persistent *bool   `json:"persistent"`
		Locked     *bool   `json:"locked"`
	}
	if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Name == nil && req.TTLSeconds == nil && req.Persistent == nil && req.Locked == nil {
		writeVirtualComputersAPIError(w, "invalid_argument", "name, ttl_seconds, persistent or locked is required", http.StatusBadRequest)
		return
	}
	name := ""
	if req.Name != nil {
		normalized, err := virtualComputersNormalizeLabel(*req.Name)
		if err != nil {
			writeVirtualComputersAPIError(w, "invalid_name", err.Error(), http.StatusBadRequest)
			return
		}
		name = normalized
	}
	ledger, owned, err := virtualComputersLedger(s)
	if err != nil || ledger == nil {
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	if owned {
		defer ledger.Close()
	}
	tracked, found, err := ledger.GetVolume(r.Context(), volumeID)
	if err != nil {
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	if !found {
		writeVirtualComputersAPIError(w, "volume_not_found", "volume is not tracked; import or refresh it first", http.StatusNotFound)
		return
	}
	retention := req.TTLSeconds != nil || req.Persistent != nil || req.Locked != nil
	if retention {
		if tracked.Availability != "available" {
			writeVirtualComputersAPIError(w, "volume_unavailable", "volume belongs to a previous or unavailable storage and cannot be changed", http.StatusConflict)
			return
		}
		if req.TTLSeconds != nil {
			if *req.TTLSeconds <= 0 {
				writeVirtualComputersAPIError(w, "invalid_argument", "ttl_seconds must be positive", http.StatusBadRequest)
				return
			}
			if *req.TTLSeconds > virtualComputersVolumeMaxTTL(cfg) {
				writeVirtualComputersAPIError(w, "volume_ttl_too_long", "volume retention exceeds the configured maximum", http.StatusBadRequest)
				return
			}
		}
		if !virtualComputersVolumeRetention(r.Context(), client) {
			writeVirtualComputersAPIError(w, "volume_retention_requires_repair", virtualComputersRetentionRepairMessage, http.StatusConflict)
			return
		}
		update := virtualcomputers.VolumeUpdate{TTLSeconds: req.TTLSeconds, Persistent: req.Persistent, Locked: req.Locked}
		if req.Persistent != nil && !*req.Persistent && req.TTLSeconds == nil {
			// Ending permanence needs a concrete retention; use the default day.
			ttl := virtualComputersVolumeDefaultTTL(cfg)
			update.TTLSeconds = &ttl
		}
		fresh, err := client.UpdateVolume(r.Context(), volumeID, update)
		if err != nil {
			writeVirtualComputersError(w, err)
			return
		}
		if tracked, err = virtualcomputers.TrackUpstreamVolume(r.Context(), ledger, fresh); err != nil {
			writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	if req.Name != nil {
		if _, err := ledger.SetVolumeName(r.Context(), volumeID, name); err != nil {
			writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
			return
		}
		tracked.Name = name
	}
	virtualComputersRecordAction(s, r, "update_volume", "volume", volumeID, map[string]interface{}{
		"renamed": req.Name != nil, "persistent": req.Persistent != nil, "locked": req.Locked != nil, "ttl": req.TTLSeconds != nil,
	})
	writeJSON(w, map[string]interface{}{"status": "ok", "volume": tracked})
}

func virtualComputersVolumeDelete(s *Server, w http.ResponseWriter, r *http.Request, volumeID string, client *virtualcomputers.Client) {
	if !virtualComputersMutationAllowed(s, w) {
		return
	}
	// Fail closed: without the ledger the lock and attachment checks cannot run.
	ledger, owned, ledgerErr := virtualComputersLedger(s)
	if ledgerErr != nil || ledger == nil {
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	if owned {
		defer ledger.Close()
	}
	tracked, found, lookupErr := ledger.GetVolume(r.Context(), volumeID)
	if lookupErr != nil {
		writeVirtualComputersAPIError(w, "storage_unavailable", "virtual computer ledger is unavailable", http.StatusServiceUnavailable)
		return
	}
	if found && tracked.Locked {
		writeVirtualComputersAPIError(w, "volume_locked", "volume is locked; unlock it before deleting", http.StatusConflict)
		return
	}
	machines := virtualComputersRunningMachineSet(r.Context(), client)
	if attached := virtualComputersVolumeAttachments(r.Context(), ledger, volumeID, machines); len(attached) > 0 {
		writeVirtualComputersAPIErrorDetails(w, "volume_in_use", "volume is attached to a running computer or workspace", http.StatusConflict,
			map[string]interface{}{"attached_to": attached})
		return
	}
	if err := client.DeleteVolume(r.Context(), volumeID); err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	if err := ledger.DeleteVolume(r.Context(), volumeID); err != nil {
		virtualComputersLogLedgerError(s, err)
	}
	virtualComputersRecordAction(s, r, "delete_volume", "volume", volumeID, nil)
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

// virtualComputersSaveMachine saves a machine into a volume. A tracked
// workspace_v2 volume stays workspace_v2; anything else keeps the legacy root
// format. A 413 from boringd surfaces as volume_quota_exceeded.
func virtualComputersSaveMachine(s *Server, w http.ResponseWriter, r *http.Request, machineID, volumeID string, client *virtualcomputers.Client) {
	ledger, owned, _ := virtualComputersLedger(s)
	if owned {
		defer ledger.Close()
	}
	format := ""
	if ledger != nil {
		if tracked, found, err := ledger.GetVolume(r.Context(), strings.TrimSpace(volumeID)); err == nil && found && tracked.Format == virtualcomputers.WorkspaceVolumeFormat {
			format = virtualcomputers.WorkspaceVolumeFormat
		}
	}
	var payload map[string]interface{}
	var err error
	if format != "" {
		payload, err = client.SaveMachineWithFormat(r.Context(), machineID, volumeID, format)
	} else {
		payload, err = client.SaveMachine(r.Context(), machineID, volumeID)
	}
	if err != nil {
		writeVirtualComputersError(w, err)
		return
	}
	if ledger != nil {
		if fresh, getErr := client.GetVolume(r.Context(), strings.TrimSpace(volumeID)); getErr == nil {
			_, _ = virtualcomputers.TrackUpstreamVolume(r.Context(), ledger, fresh)
		}
	}
	virtualComputersRecordAction(s, r, "save", "machine", machineID, nil)
	writeJSON(w, map[string]interface{}{"status": "ok", "result": payload})
}

// virtualComputersLimits is the numeric counterpart of the boolean capability
// map. The Virtual Computers app reads it to cap its pickers.
func virtualComputersLimits(cfg virtualcomputers.ToolConfig) map[string]int {
	maxTTL := cfg.MaxTTLSeconds
	if maxTTL <= 0 {
		maxTTL = 900
	}
	defaultTTL := cfg.DefaultTTLSeconds
	if defaultTTL <= 0 {
		defaultTTL = 600
	}
	maxForks := cfg.MaxForks
	if maxForks <= 0 {
		maxForks = 3
	}
	return map[string]int{
		"max_ttl_seconds":        maxTTL,
		"default_ttl_seconds":    defaultTTL,
		"volume_max_ttl_seconds": cfg.VolumeMaxTTL(),
		"volume_quota_mb":        cfg.VolumeQuota(),
		"max_forks":              maxForks,
	}
}
