package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"aurago/internal/acestep"
	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/fritzbox"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

type desktopInfraSourceEditResponse struct {
	Source   string `json:"source"`
	Fields   any    `json:"fields"`
	Version  string `json:"version"`
	Editable bool   `json:"editable"`
	Status   string `json:"status,omitempty"`
	Pending  bool   `json:"pending,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type desktopInfraRegistryFields struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Protocol     string   `json:"protocol"`
	IPAddress    string   `json:"ip_address"`
	Port         int      `json:"port"`
	Username     string   `json:"username"`
	CredentialID string   `json:"credential_id"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	MACAddress   string   `json:"mac_address"`
}

type desktopInfraSourceEditRequest struct {
	Source  string          `json:"source"`
	Version string          `json:"version"`
	Fields  json.RawMessage `json:"fields"`
}

type desktopInfraDockerEditFields struct {
	Name string `json:"name"`
}

type desktopInfraProxmoxEditFields struct {
	Name        string `json:"name,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
}

type desktopInfraFritzEditFields struct {
	FriendlyName string `json:"friendly_name"`
}

type desktopInfraDockerInspect struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	Image string `json:"Image"`
	State struct {
		Status    string `json:"Status"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	RestartCount int `json:"RestartCount"`
	Config       struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

type desktopInfraActionRequest struct {
	Source    string `json:"source"`
	Action    string `json:"action"`
	RequestID string `json:"request_id"`
	Confirmed bool   `json:"confirmed"`
}

type desktopInfraActionResponse struct {
	Status    string `json:"status"`
	Action    string `json:"action"`
	RequestID string `json:"request_id"`
	UPID      string `json:"upid,omitempty"`
}

var desktopInfraRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

var errDesktopInfraWriteOutcomeUnknown = errors.New("desktop infra write outcome unknown")

func desktopInfraSourceEdit(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, nodeID string) {
	sourceKind := strings.TrimSpace(r.URL.Query().Get("source"))
	if sourceKind != "registry" && sourceKind != "docker" && sourceKind != "proxmox" && sourceKind != "fritzbox" {
		desktopInfraError(w, http.StatusBadRequest, "invalid_source", "Source is not supported")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		desktopInfraMethodNotAllowed(w)
		return
	}
	if r.Method == http.MethodPut && !cfg.VirtualDesktop.Infra.AllowSourceEdit {
		desktopInfraError(w, http.StatusForbidden, "source_edit_disabled", "Source editing is disabled")
		return
	}
	node, err := store.GetNode(r.Context(), nodeID)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	source, ok := desktopInfraFindConfiguredNodeSource(node, cfg, sourceKind)
	if !ok {
		desktopInfraError(w, http.StatusNotFound, "source_not_linked", "This source is not linked to the node")
		return
	}
	if source.Kind == "proxmox" && strings.HasPrefix(source.Key, "node:") {
		if r.Method == http.MethodPut {
			desktopInfraError(w, http.StatusForbidden, "source_readonly", "Proxmox node metadata is not editable here")
			return
		}
		desktopInfraJSON(w, http.StatusOK, desktopInfraSourceEditResponse{Source: source.Kind, Editable: false, Reason: "source_readonly"})
		return
	}
	if !desktopInfraSourceConfigured(cfg, source) {
		desktopInfraError(w, http.StatusForbidden, "source_disabled", "This source integration is disabled")
		return
	}
	if r.Method == http.MethodGet {
		response, err := desktopInfraReadSourceEdit(r.Context(), s, cfg, runtime, source)
		if err != nil {
			desktopInfraProviderError(w, err)
			return
		}
		desktopInfraJSON(w, http.StatusOK, response)
		return
	}
	var request desktopInfraSourceEditRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if request.Source != sourceKind || strings.TrimSpace(request.Version) == "" || len(request.Fields) == 0 {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "Source, version, and fields are required")
		return
	}
	result, status, err := desktopInfraWriteSourceEdit(r.Context(), s, cfg, store, runtime, nodeID, source, request)
	if err != nil {
		if status == http.StatusConflict {
			desktopInfraError(w, status, "conflict", "The source changed; refresh before editing")
		} else if status == http.StatusForbidden {
			desktopInfraError(w, status, "source_readonly", "This source is not writable")
		} else if status == http.StatusBadRequest {
			desktopInfraError(w, status, "invalid_request", "Source fields are invalid")
		} else {
			desktopInfraError(w, status, "source_edit_failed", "The source update could not be confirmed")
		}
		return
	}
	desktopInfraJSON(w, http.StatusOK, result)
}

func desktopInfraReadSourceEdit(ctx context.Context, s *Server, cfg *config.Config, runtime *desktopInfraRuntime, source infra.Source) (desktopInfraSourceEditResponse, error) {
	response := desktopInfraSourceEditResponse{Source: source.Kind, Editable: desktopInfraCapabilities(cfg).SourceEdit}
	if !response.Editable {
		response.Reason = "source_edit_disabled"
	}
	switch source.Kind {
	case "registry":
		device, err := inventory.GetDeviceByID(s.InventoryDB, source.Key)
		if err != nil {
			return response, err
		}
		fields := desktopInfraFieldsFromRegistry(device)
		response.Fields = fields
		response.Version = desktopInfraRegistryVersion(device)
	case "docker":
		inspect, err := desktopInfraInspectDocker(ctx, cfg, source.Key, runtime)
		if err != nil {
			return response, err
		}
		if desktopInfraDockerProtected(inspect) {
			response.Editable = false
			response.Reason = "managed_container"
		}
		if cfg.Docker.ReadOnly {
			response.Editable = false
			response.Reason = "source_readonly"
		}
		response.Fields = desktopInfraDockerEditFields{Name: strings.TrimPrefix(inspect.Name, "/")}
		response.Version = desktopInfraDockerVersion(inspect)
	case "proxmox":
		resource, err := desktopInfraProxmoxResource(ctx, cfg, source)
		if err != nil {
			return response, err
		}
		values, err := tools.ProxmoxInfraReadConfig(ctx, desktopInfraProxmoxConfig(cfg), resource.Node, resource.Type, resource.VMID)
		if err != nil {
			return response, err
		}
		if cfg.Proxmox.ReadOnly {
			response.Editable = false
			response.Reason = "source_readonly"
		}
		response.Fields, err = desktopInfraProxmoxFields(resource.Type, values)
		if err != nil {
			return response, err
		}
		response.Version = desktopInfraProxmoxVersion(resource, values)
		response.Pending = values.Pending
	case "fritzbox":
		client, cancel, err := desktopInfraNewFritzClient(ctx, s, cfg)
		if err != nil {
			return response, err
		}
		defer func() { client.Close(); cancel() }()
		info, err := client.InfraHostEdit(source.Key)
		if err != nil {
			return response, err
		}
		if !info.Writable || cfg.FritzBox.Network.ReadOnly {
			response.Editable = false
			response.Reason = "source_readonly"
		}
		response.Fields = desktopInfraFritzEditFields{FriendlyName: info.FriendlyName}
		response.Version = desktopInfraFritzVersion(source.Key, info)
	default:
		return response, errors.New("unsupported source")
	}
	return response, nil
}

func desktopInfraWriteSourceEdit(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, nodeID string, source infra.Source, request desktopInfraSourceEditRequest) (desktopInfraSourceEditResponse, int, error) {
	response := desktopInfraSourceEditResponse{Source: source.Kind, Editable: true, Status: infra.ActionCompleted}
	providerKey := desktopInfraProviderConfigKey(s, cfg, source.Kind)
	switch source.Kind {
	case "registry":
		var fields desktopInfraRegistryFields
		if err := desktopInfraUnmarshalFields(request.Fields, &fields); err != nil {
			return response, http.StatusBadRequest, err
		}
		if _, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, true, ""); !ok {
			return response, http.StatusForbidden, errors.New("Infra source-edit permission changed")
		}
		updated, err := desktopInfraUpdateRegistry(ctx, s, source, request.Version, fields)
		if err != nil {
			if errors.Is(err, errDesktopInfraWriteOutcomeUnknown) {
				response.Status = infra.ActionUnknown
				response.Version = request.Version
				return response, http.StatusOK, nil
			}
			return response, desktopInfraSourceEditStatus(err), err
		}
		response.Fields = desktopInfraFieldsFromRegistry(updated)
		response.Version = desktopInfraRegistryVersion(updated)
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		if err := desktopInfraObserveOneRegistry(ctx, store, updated); err != nil {
			// The inventory write is durable; a failed source projection is reconciled
			// by the next explicit snapshot and must not be presented as a failed write.
			markCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = store.MarkSourcesStale(markCtx, "registry", "inventory")
			cancel()
		}
	case "docker":
		var fields desktopInfraDockerEditFields
		if err := desktopInfraUnmarshalFields(request.Fields, &fields); err != nil {
			return response, http.StatusBadRequest, err
		}
		inspect, err := desktopInfraInspectDocker(ctx, cfg, source.Key, runtime)
		if err != nil {
			return response, http.StatusBadGateway, err
		}
		if desktopInfraDockerProtected(inspect) || cfg.Docker.ReadOnly {
			return response, http.StatusForbidden, errors.New("Docker target is protected or read-only")
		}
		if desktopInfraDockerVersion(inspect) != request.Version {
			return response, http.StatusConflict, errors.New("Docker container changed")
		}
		newName := strings.TrimSpace(fields.Name)
		if !tools.DockerNameValid(newName) {
			return response, http.StatusBadRequest, errors.New("invalid or reserved Docker name")
		}
		oldName := strings.TrimPrefix(inspect.Name, "/")
		readConfig := cfg
		if newName != oldName {
			currentCfg, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, true, "")
			if !ok {
				return response, http.StatusForbidden, errors.New("Infra source-edit permission changed")
			}
			if desktopInfraDockerReservedName(currentCfg, newName) {
				return response, http.StatusBadRequest, errors.New("invalid or reserved Docker name")
			}
			readConfig = currentCfg
			callCtx, cancel := desktopInfraContext(ctx)
			_, code, err := desktopInfraDockerRequest(runtime, callCtx, tools.DockerConfig{Host: currentCfg.Docker.Host}, http.MethodPost,
				"/containers/"+url.PathEscape(source.Key)+"/rename?name="+url.QueryEscape(newName), "")
			cancel()
			desktopInfraInvalidate(runtime, source.Kind, source.Instance)
			if err != nil || code == http.StatusRequestTimeout || code >= 500 {
				response.Status = infra.ActionUnknown
				response.Version = request.Version
				return response, http.StatusOK, nil
			}
			if code != http.StatusNoContent && code != http.StatusOK {
				return response, http.StatusBadGateway, errors.New("Docker rejected the rename")
			}
		}
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		readback, err := desktopInfraInspectDocker(ctx, readConfig, source.Key, runtime)
		if err != nil || strings.TrimPrefix(readback.Name, "/") != newName {
			response.Status = infra.ActionUnknown
			response.Version = request.Version
			return response, http.StatusOK, nil
		}
		response.Fields = desktopInfraDockerEditFields{Name: newName}
		response.Version = desktopInfraDockerVersion(readback)
	case "proxmox":
		var fields desktopInfraProxmoxEditFields
		if err := desktopInfraUnmarshalFields(request.Fields, &fields); err != nil {
			return response, http.StatusBadRequest, err
		}
		if cfg.Proxmox.ReadOnly {
			return response, http.StatusForbidden, errors.New("Proxmox is read-only")
		}
		resource, err := desktopInfraProxmoxResource(ctx, cfg, source)
		if err != nil {
			return response, http.StatusBadGateway, err
		}
		before, err := tools.ProxmoxInfraReadConfig(ctx, desktopInfraProxmoxConfig(cfg), resource.Node, resource.Type, resource.VMID)
		if err != nil {
			return response, http.StatusBadGateway, err
		}
		if desktopInfraProxmoxVersion(resource, before) != request.Version {
			return response, http.StatusConflict, errors.New("Proxmox configuration changed")
		}
		nameField, fieldErr := desktopInfraProxmoxNameField(resource.Type, fields)
		if fieldErr != nil {
			return response, http.StatusBadRequest, fieldErr
		}
		name, fieldErr := desktopInfraInputText(nameField, 200, true)
		if fieldErr != nil {
			return response, http.StatusBadRequest, fieldErr
		}
		description, fieldErr := desktopInfraInputText(fields.Description, 256, false)
		if fieldErr != nil {
			return response, http.StatusBadRequest, fieldErr
		}
		tags, fieldErr := desktopInfraInputText(fields.Tags, 256, false)
		if fieldErr != nil {
			return response, http.StatusBadRequest, fieldErr
		}
		currentCfg, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, true, "")
		if !ok {
			return response, http.StatusForbidden, errors.New("Infra source-edit permission changed")
		}
		status, err := tools.ProxmoxInfraEditConfig(ctx, desktopInfraProxmoxConfig(currentCfg), resource.Node, resource.Type, resource.VMID, before, name, description, tags)
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		if status == "conflict" {
			return response, http.StatusConflict, err
		}
		if status == infra.ActionUnknown {
			response.Status = infra.ActionUnknown
			response.Version = request.Version
			return response, http.StatusOK, nil
		}
		if status != infra.ActionCompleted && status != "pending" {
			return response, http.StatusBadGateway, err
		}
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		after, err := tools.ProxmoxInfraReadConfig(ctx, desktopInfraProxmoxConfig(cfg), resource.Node, resource.Type, resource.VMID)
		if err != nil {
			response.Status = infra.ActionUnknown
			response.Version = request.Version
			return response, http.StatusOK, nil
		}
		response.Fields, err = desktopInfraProxmoxFields(resource.Type, after)
		if err != nil {
			response.Status = infra.ActionUnknown
			response.Version = request.Version
			return response, http.StatusOK, nil
		}
		response.Version = desktopInfraProxmoxVersion(resource, after)
		response.Pending = after.Pending
		if after.Pending {
			response.Status = "pending"
		}
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
	case "fritzbox":
		var fields desktopInfraFritzEditFields
		if err := desktopInfraUnmarshalFields(request.Fields, &fields); err != nil {
			return response, http.StatusBadRequest, err
		}
		if cfg.FritzBox.Network.ReadOnly || !cfg.FritzBox.Network.Enabled || !cfg.FritzBox.Network.SubFeatures.Hosts {
			return response, http.StatusForbidden, errors.New("Fritz!Box Hosts is read-only or disabled")
		}
		client, cancel, err := desktopInfraNewFritzClient(ctx, s, cfg)
		if err != nil {
			return response, http.StatusBadGateway, err
		}
		defer func() { client.Close(); cancel() }()
		before, err := client.InfraHostEdit(source.Key)
		if err != nil {
			return response, http.StatusBadGateway, err
		}
		if desktopInfraFritzVersion(source.Key, before) != request.Version {
			return response, http.StatusConflict, errors.New("Fritz!Box host changed")
		}
		if _, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, true, ""); !ok {
			return response, http.StatusForbidden, errors.New("Infra source-edit permission changed")
		}
		status, err := client.InfraSetHostFriendlyName(source.Key, fields.FriendlyName)
		if status == infra.ActionUnknown {
			desktopInfraInvalidate(runtime, source.Kind, source.Instance)
			response.Status = infra.ActionUnknown
			response.Version = request.Version
			return response, http.StatusOK, nil
		}
		if status != infra.ActionCompleted {
			return response, http.StatusBadGateway, err
		}
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		after, err := client.InfraHostEdit(source.Key)
		if err != nil {
			response.Status = infra.ActionUnknown
			response.Version = request.Version
			return response, http.StatusOK, nil
		}
		response.Fields = desktopInfraFritzEditFields{FriendlyName: after.FriendlyName}
		response.Version = desktopInfraFritzVersion(source.Key, after)
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
	default:
		return response, http.StatusBadRequest, errors.New("unsupported source")
	}
	response.Editable = true
	return response, http.StatusOK, nil
}

func desktopInfraUpdateRegistry(ctx context.Context, s *Server, source infra.Source, version string, fields desktopInfraRegistryFields) (inventory.DeviceRecord, error) {
	if source.Instance != "inventory" || source.Key == "" {
		return inventory.DeviceRecord{}, errors.New("invalid inventory source")
	}
	existing, err := inventory.GetDeviceByID(s.InventoryDB, source.Key)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	if desktopInfraRegistryVersion(existing) != version {
		return inventory.DeviceRecord{}, infra.ErrConflict
	}
	updated := existing
	updated.Name, err = desktopInfraInputText(fields.Name, 200, true)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	updated.Type, err = desktopInfraInputText(fields.Type, 128, false)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	if updated.Type == "" {
		updated.Type = "generic"
	}
	updated.Protocol = normalizeDeviceProtocolForAPI(fields.Protocol)
	if !isAllowedDeviceProtocol(updated.Protocol) {
		return inventory.DeviceRecord{}, errors.New("invalid device protocol")
	}
	updated.IPAddress, err = desktopInfraInputText(fields.IPAddress, 64, false)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	updated.Port = fields.Port
	updated.Username, err = desktopInfraInputText(fields.Username, 256, false)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	updated.CredentialID, err = desktopInfraInputText(fields.CredentialID, 128, false)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	updated.Description, err = desktopInfraInputText(fields.Description, 4096, false)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	updated.Tags, err = desktopInfraInputTags(fields.Tags)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	macAddress, err := desktopInfraInputText(fields.MACAddress, 64, false)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	updated.MACAddress = desktopInfraValidMAC(macAddress)
	if updated.Name == "" || len([]rune(updated.Name)) > 200 || len([]rune(updated.Type)) > 128 ||
		updated.Port < 0 || updated.Port > 65535 || (updated.IPAddress != "" && net.ParseIP(updated.IPAddress) == nil) ||
		(fields.MACAddress != "" && updated.MACAddress == "") {
		return inventory.DeviceRecord{}, errors.New("invalid registry fields")
	}
	if updated.CredentialID != "" {
		if _, err := credentials.GetByID(s.InventoryDB, updated.CredentialID); err != nil {
			return inventory.DeviceRecord{}, errors.New("linked credential not found")
		}
		updated.Username = ""
		updated.VaultSecretID = ""
	} else if existing.CredentialID != "" && updated.Username == "" {
		// A blank credential and username is the unchanged linked-secret shape.
		// Replacing an existing credential requires explicitly supplying a username.
		updated.CredentialID = existing.CredentialID
		updated.Username = ""
		updated.VaultSecretID = ""
	} else {
		// Legacy Vault references are never part of the client form. Preserve the
		// existing reference unless a credential record replaces it.
		updated.VaultSecretID = existing.VaultSecretID
	}
	if ctx.Err() != nil {
		return inventory.DeviceRecord{}, ctx.Err()
	}
	updated.Tags = nonNilStrings(updated.Tags)
	tagsJSON, err := json.Marshal(updated.Tags)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	oldTagsJSON, err := sqlReadTagsJSON(ctx, s.InventoryDB, existing.ID)
	if err != nil {
		return inventory.DeviceRecord{}, err
	}
	res, err := s.InventoryDB.ExecContext(ctx, `UPDATE devices SET name=?, type=?, protocol=?, ip_address=?, port=?, username=?, vault_secret_id=?, credential_id=?, description=?, tags=?, mac_address=?
		WHERE id=? AND COALESCE(name,'')=? AND COALESCE(type,'')=? AND COALESCE(protocol,'none')=? AND COALESCE(ip_address,'')=? AND port=? AND COALESCE(username,'')=? AND COALESCE(vault_secret_id,'')=? AND COALESCE(credential_id,'')=? AND COALESCE(description,'')=? AND COALESCE(tags,'[]')=? AND COALESCE(mac_address,'')=?`,
		updated.Name, updated.Type, updated.Protocol, updated.IPAddress, updated.Port, updated.Username, updated.VaultSecretID, updated.CredentialID,
		updated.Description, string(tagsJSON), updated.MACAddress,
		existing.ID, existing.Name, existing.Type, existing.Protocol, existing.IPAddress, existing.Port, existing.Username, existing.VaultSecretID,
		existing.CredentialID, existing.Description, oldTagsJSON, existing.MACAddress)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return inventory.DeviceRecord{}, fmt.Errorf("%w: %v", errDesktopInfraWriteOutcomeUnknown, err)
		}
		return inventory.DeviceRecord{}, fmt.Errorf("update registry device: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return inventory.DeviceRecord{}, fmt.Errorf("%w: count updated registry devices: %v", errDesktopInfraWriteOutcomeUnknown, err)
	}
	if rows != 1 {
		return inventory.DeviceRecord{}, infra.ErrConflict
	}
	return updated, nil
}

func sqlReadTagsJSON(ctx context.Context, db *sql.DB, id string) (string, error) {
	var value string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(tags,'[]') FROM devices WHERE id=?`, id).Scan(&value); err != nil {
		return "", fmt.Errorf("read registry tags: %w", err)
	}
	return value, nil
}

func desktopInfraObserveOneRegistry(ctx context.Context, store *infra.Store, device inventory.DeviceRecord) error {
	return store.ObserveSources(ctx, "registry", "inventory", []infra.SourceObservation{{
		Identity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: device.ID},
		Name:     device.Name, Observed: desktopInfraRegistryObserved(device),
	}}, false)
}

func desktopInfraFieldsFromRegistry(device inventory.DeviceRecord) desktopInfraRegistryFields {
	return desktopInfraRegistryFields{
		Name: device.Name, Type: device.Type, Protocol: device.Protocol, IPAddress: device.IPAddress,
		Port: device.Port, Username: device.Username, CredentialID: device.CredentialID,
		Description: device.Description, Tags: nonNilStrings(device.Tags), MACAddress: device.MACAddress,
	}
}

func desktopInfraRegistryVersion(device inventory.DeviceRecord) string {
	device.Tags = nonNilStrings(device.Tags)
	data, _ := json.Marshal(struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		Protocol     string   `json:"protocol"`
		IPAddress    string   `json:"ip_address"`
		Port         int      `json:"port"`
		Username     string   `json:"username"`
		CredentialID string   `json:"credential_id"`
		Description  string   `json:"description"`
		Tags         []string `json:"tags"`
		MACAddress   string   `json:"mac_address"`
	}{
		ID: device.ID, Name: device.Name, Type: device.Type, Protocol: device.Protocol,
		IPAddress: device.IPAddress, Port: device.Port, Username: device.Username,
		CredentialID: device.CredentialID, Description: device.Description, Tags: device.Tags,
		MACAddress: device.MACAddress,
	})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func desktopInfraDockerVersion(info desktopInfraDockerInspect) string {
	return desktopInfraHash(info.ID, strings.TrimPrefix(info.Name, "/"), info.Config.Image, info.State.Status)
}

func desktopInfraProxmoxVersion(resource tools.InfraProxmoxResource, values tools.InfraProxmoxConfig) string {
	return desktopInfraHash(resource.Type, resource.VMID, resource.Node, values.Digest, values.Name, values.Description, values.Tags, fmt.Sprint(values.Pending))
}

func desktopInfraProxmoxFields(kind string, values tools.InfraProxmoxConfig) (desktopInfraProxmoxEditFields, error) {
	fields := desktopInfraProxmoxEditFields{Description: values.Description, Tags: values.Tags}
	switch kind {
	case "qemu":
		fields.Name = values.Name
	case "lxc":
		fields.Hostname = values.Name
	default:
		return desktopInfraProxmoxEditFields{}, errors.New("unsupported proxmox config type")
	}
	return fields, nil
}

func desktopInfraProxmoxNameField(kind string, fields desktopInfraProxmoxEditFields) (string, error) {
	switch kind {
	case "qemu":
		if fields.Hostname != "" {
			return "", errors.New("QEMU metadata uses name")
		}
		return fields.Name, nil
	case "lxc":
		if fields.Name != "" {
			return "", errors.New("LXC metadata uses hostname")
		}
		return fields.Hostname, nil
	default:
		return "", errors.New("unsupported proxmox config type")
	}
}

func desktopInfraFritzVersion(key string, info fritzbox.InfraHostEditInfo) string {
	return desktopInfraHash(key, info.FriendlyName, fmt.Sprint(info.Writable))
}

func desktopInfraHash(values ...string) string {
	hash := sha256.New()
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func desktopInfraUnmarshalFields(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return errors.New("source fields are required")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("source fields contain multiple values")
	}
	return nil
}

func desktopInfraInputText(value string, maxRunes int, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes || (required && value == "") {
		return "", errors.New("invalid text field")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", errors.New("text field contains control characters")
		}
	}
	return value, nil
}

func desktopInfraInputTags(values []string) ([]string, error) {
	if len(values) > 64 {
		return nil, errors.New("too many tags")
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value, err := desktopInfraInputText(value, 64, false)
		if err != nil {
			return nil, err
		}
		if value != "" {
			result = append(result, value)
		}
	}
	return result, nil
}

func desktopInfraFindNodeSource(node infra.Node, kind string) (infra.Source, bool) {
	var result infra.Source
	found := false
	for _, source := range node.Sources {
		if source.Kind != kind {
			continue
		}
		if found {
			return infra.Source{}, false
		}
		result = source
		found = true
	}
	return result, found
}

// desktopInfraFindConfiguredNodeSource ignores stale records from previous
// integration settings while retaining the ambiguity guard for multiple
// linked targets from the active provider instance.
func desktopInfraFindConfiguredNodeSource(node infra.Node, cfg *config.Config, kind string) (infra.Source, bool) {
	var result infra.Source
	found := false
	for _, source := range node.Sources {
		if source.Kind != kind || !desktopInfraSourceConfigured(cfg, source) {
			continue
		}
		if found {
			return infra.Source{}, false
		}
		result = source
		found = true
	}
	return result, found
}

func desktopInfraSourceConfigured(cfg *config.Config, source infra.Source) bool {
	switch source.Kind {
	case "registry":
		return source.Instance == "inventory"
	case "docker":
		return cfg.Docker.Enabled && source.Instance == desktopInfraDockerInstance(cfg)
	case "proxmox":
		return cfg.Proxmox.Enabled && source.Instance == desktopInfraProxmoxInstance(cfg)
	case "fritzbox":
		return desktopInfraFritzAllowed(cfg) && source.Instance == desktopInfraFritzInstance(cfg)
	default:
		return false
	}
}

func desktopInfraProviderConfigKey(s *Server, cfg *config.Config, kind string) string {
	if cfg == nil {
		return ""
	}
	switch kind {
	case "docker":
		return desktopInfraConfigKey("docker", cfg.Docker.Host, fmt.Sprint(cfg.Docker.Enabled), fmt.Sprint(cfg.Docker.ReadOnly))
	case "proxmox":
		values := desktopInfraProxmoxConfig(cfg)
		return desktopInfraConfigKey("proxmox", values.URL, values.TokenID, values.Secret, values.Node,
			fmt.Sprint(values.Insecure), fmt.Sprint(values.ReadOnly), fmt.Sprint(values.AllowDestructive), fmt.Sprint(cfg.Proxmox.Enabled))
	case "fritzbox":
		fritz, network := cfg.FritzBox, cfg.FritzBox.Network
		return desktopInfraConfigKey("fritzbox", fritz.Host, fmt.Sprint(fritz.Port), fmt.Sprint(fritz.HTTPS),
			fmt.Sprint(fritz.WebPort), fmt.Sprint(fritz.InsecureSkipVerify), fmt.Sprint(fritz.Timeout), fritz.Username,
			desktopInfraFritzPassword(s, cfg), fmt.Sprint(fritz.Enabled), fmt.Sprint(network.Enabled), fmt.Sprint(network.ReadOnly),
			fmt.Sprint(network.SubFeatures.Hosts), fmt.Sprint(network.SubFeatures.WakeOnLAN))
	default:
		return desktopInfraConfigKey(kind)
	}
}

func desktopInfraRuntimeConfigKey(s *Server, cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	fritz, network := cfg.FritzBox, cfg.FritzBox.Network
	fritzAdmissionKey := desktopInfraConfigKey("fritzbox-admission", fritz.Host, fmt.Sprint(fritz.Port), fmt.Sprint(fritz.HTTPS),
		fmt.Sprint(fritz.WebPort), fmt.Sprint(fritz.InsecureSkipVerify), fmt.Sprint(fritz.Timeout), fritz.Username,
		fritz.Password, fmt.Sprint(fritz.Enabled), fmt.Sprint(network.Enabled), fmt.Sprint(network.ReadOnly),
		fmt.Sprint(network.SubFeatures.Hosts), fmt.Sprint(network.SubFeatures.WakeOnLAN))
	return desktopInfraConfigKey("infra-runtime", fmt.Sprint(cfg.VirtualDesktop.Enabled), fmt.Sprint(cfg.VirtualDesktop.ReadOnly),
		fmt.Sprint(cfg.VirtualDesktop.Infra.Enabled), fmt.Sprint(cfg.VirtualDesktop.Infra.ReadOnly),
		fmt.Sprint(cfg.VirtualDesktop.Infra.AllowSourceEdit), fmt.Sprint(cfg.VirtualDesktop.Infra.AllowControl),
		desktopInfraProviderConfigKey(s, cfg, "docker"), desktopInfraProviderConfigKey(s, cfg, "proxmox"),
		fritzAdmissionKey)
}

func desktopInfraConfigChanged(s *Server, previous, next *config.Config) bool {
	return previous != nil && next != nil && desktopInfraRuntimeConfigKey(s, previous) != desktopInfraRuntimeConfigKey(s, next)
}

func desktopInfraCurrentWriteConfig(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, nodeID string, source infra.Source, providerKey string, sourceEdit bool, action string) (*config.Config, bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	current := cfg
	if s != nil {
		if snapshot := s.ConfigSnapshot(); snapshot != nil {
			current = snapshot
		}
	}
	if s != nil && !s.desktopInfraConfigAdmitted(current) {
		return nil, false
	}
	if current == nil || !desktopInfraCapabilities(current).Write || !desktopInfraSourceConfigured(current, source) ||
		desktopInfraProviderConfigKey(s, current, source.Kind) != providerKey {
		return nil, false
	}
	if nodeID != "" {
		if store == nil {
			return nil, false
		}
		node, err := store.GetNode(ctx, nodeID)
		if err != nil {
			return nil, false
		}
		linked, ok := desktopInfraFindConfiguredNodeSource(node, current, source.Kind)
		if !ok || linked.SourceIdentity != source.SourceIdentity {
			return nil, false
		}
	}
	capabilities := desktopInfraCapabilities(current)
	if sourceEdit {
		if !capabilities.SourceEdit {
			return nil, false
		}
		switch source.Kind {
		case "docker":
			if current.Docker.ReadOnly {
				return nil, false
			}
		case "proxmox":
			if current.Proxmox.ReadOnly {
				return nil, false
			}
		case "fritzbox":
			if current.FritzBox.Network.ReadOnly || !desktopInfraFritzAllowed(current) || !current.FritzBox.Network.SubFeatures.Hosts {
				return nil, false
			}
		}
	} else if !capabilities.Control || !desktopInfraActionAllowed(current, source.Kind, action) || !desktopInfraActionTargetAllowed(source, action) {
		return nil, false
	}
	return current, true
}

func desktopInfraInspectDocker(ctx context.Context, cfg *config.Config, id string, runtime *desktopInfraRuntime) (desktopInfraDockerInspect, error) {
	if !cfg.Docker.Enabled || !tools.DockerNameValid(id) {
		return desktopInfraDockerInspect{}, errors.New("Docker source is unavailable")
	}
	ctx, cancel := desktopInfraContext(ctx)
	defer cancel()
	data, code, err := desktopInfraDockerRequest(runtime, ctx, tools.DockerConfig{Host: cfg.Docker.Host}, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", "")
	if err != nil {
		return desktopInfraDockerInspect{}, fmt.Errorf("read Docker container: %w", err)
	}
	if code != http.StatusOK {
		return desktopInfraDockerInspect{}, fmt.Errorf("Docker inspect returned HTTP %d", code)
	}
	var info desktopInfraDockerInspect
	if err := json.Unmarshal(data, &info); err != nil {
		return desktopInfraDockerInspect{}, fmt.Errorf("decode Docker inspect: %w", err)
	}
	if info.ID != id || info.ID == "" {
		return desktopInfraDockerInspect{}, errors.New("Docker identity changed")
	}
	return info, nil
}

func desktopInfraDockerProtected(info desktopInfraDockerInspect) bool {
	name := strings.TrimPrefix(strings.TrimSpace(info.Name), "/")
	owners := tools.DockerContainerOwnersFromMetadata([]string{name}, info.Config.Labels, containerProtectedOwners...)
	return len(owners) > 0
}

func desktopInfraDockerReservedName(cfg *config.Config, name string) bool {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	if len(tools.DockerContainerOwnersFromMetadata([]string{name}, nil, containerProtectedOwners...)) > 0 {
		return true
	}
	for _, managedName := range tools.ManagedSidecarContainerNames(cfg) {
		if strings.EqualFold(name, managedName) {
			return true
		}
	}
	return acestep.IsResourceName(name)
}

func desktopInfraProxmoxResource(ctx context.Context, cfg *config.Config, source infra.Source) (tools.InfraProxmoxResource, error) {
	if !cfg.Proxmox.Enabled || source.Instance != desktopInfraProxmoxInstance(cfg) {
		return tools.InfraProxmoxResource{}, errors.New("Proxmox source is unavailable")
	}
	parts := strings.Split(source.Key, ":")
	if len(parts) != 2 || (parts[0] != "qemu" && parts[0] != "lxc") || strings.TrimSpace(parts[1]) == "" {
		return tools.InfraProxmoxResource{}, errors.New("invalid Proxmox source identity")
	}
	resources, err := tools.ProxmoxInfraResources(ctx, desktopInfraProxmoxConfig(cfg))
	if err != nil {
		return tools.InfraProxmoxResource{}, err
	}
	for _, resource := range resources {
		if resource.Type == parts[0] && resource.VMID == parts[1] {
			return resource, nil
		}
	}
	return tools.InfraProxmoxResource{}, infra.ErrSourceRemoved
}

func desktopInfraNewFritzClient(ctx context.Context, s *Server, cfg *config.Config) (*fritzbox.Client, context.CancelFunc, error) {
	if !desktopInfraFritzAllowed(cfg) {
		return nil, func() {}, errors.New("Fritz!Box Hosts is disabled")
	}
	clientCfg := *cfg
	clientCfg.FritzBox.Password = desktopInfraFritzPassword(s, cfg)
	parent := ctx
	cancel := func() {}
	if s != nil && s.integrationCtx != nil {
		combined, stop := context.WithCancel(ctx)
		stopServer := context.AfterFunc(s.integrationCtx, stop)
		parent = combined
		cancel = func() { stopServer(); stop() }
	}
	client, err := fritzbox.NewClientContext(parent, clientCfg)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	return client, cancel, nil
}

func desktopInfraAction(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, nodeID string) {
	if !cfg.VirtualDesktop.Infra.AllowControl {
		desktopInfraError(w, http.StatusForbidden, "control_disabled", "Infra control is disabled")
		return
	}
	var request desktopInfraActionRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	request.Source = strings.TrimSpace(request.Source)
	request.Action = strings.TrimSpace(request.Action)
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.Source == "" || request.Action == "" || !desktopInfraRequestIDPattern.MatchString(request.RequestID) {
		desktopInfraError(w, http.StatusBadRequest, "invalid_action", "Source, action, and a valid request ID are required")
		return
	}
	node, err := store.GetNode(r.Context(), nodeID)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	source, ok := desktopInfraFindConfiguredNodeSource(node, cfg, request.Source)
	if !ok || source.State != infra.SourceCurrent {
		desktopInfraError(w, http.StatusNotFound, "source_not_linked", "This source is not available for the node")
		return
	}
	if !desktopInfraActionAllowed(cfg, source.Kind, request.Action) || !desktopInfraActionTargetAllowed(source, request.Action) {
		desktopInfraError(w, http.StatusForbidden, "action_disabled", "This action is disabled by source policy")
		return
	}
	if desktopInfraActionNeedsConfirm(source.Kind, request.Action) && !request.Confirmed {
		desktopInfraError(w, http.StatusBadRequest, "confirmation_required", "Confirm this action before continuing")
		return
	}
	fingerprint := desktopInfraHash(nodeID, source.Kind, source.Instance, source.Key, request.Action, fmt.Sprint(request.Confirmed))
	reservation, fresh, err := store.ReserveAction(r.Context(), infra.ActionRequest{
		RequestID: request.RequestID, Fingerprint: fingerprint, Action: request.Action,
	})
	if err != nil {
		if errors.Is(err, infra.ErrActionConflict) {
			desktopInfraError(w, http.StatusConflict, "request_id_conflict", "Request ID was already used for a different action")
		} else {
			desktopInfraError(w, http.StatusServiceUnavailable, "action_ledger_unavailable", "The action could not be safely recorded")
		}
		return
	}
	response := desktopInfraActionResponse{Status: reservation.Status, Action: reservation.Action, RequestID: reservation.RequestID, UPID: reservation.UPID}
	if !fresh {
		desktopInfraJSON(w, http.StatusOK, response)
		return
	}
	response = desktopInfraRunAction(r.Context(), s, cfg, store, runtime, nodeID, source, request)
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 2*time.Second)
	finished, finishErr := store.FinishAction(finishCtx, request.RequestID, fingerprint, response.Status, response.UPID)
	finishCancel()
	if finishErr != nil {
		// The reservation remains running and becomes unknown on restart. A
		// duplicate request cannot dispatch the provider write again.
		response.Status = infra.ActionUnknown
		response.UPID = ""
	} else {
		if response.Status == infra.ActionAccepted {
			// The ledger's terminal accepted status records a Proxmox UPID. The
			// returned response keeps the provider's accepted semantics.
			response.Status = infra.ActionAccepted
		} else {
			response.Status = finished.Status
		}
		response.UPID = finished.UPID
	}
	desktopInfraJSON(w, http.StatusOK, response)
}

func desktopInfraRunAction(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, nodeID string, source infra.Source, request desktopInfraActionRequest) desktopInfraActionResponse {
	response := desktopInfraActionResponse{Status: "failed", Action: request.Action, RequestID: request.RequestID}
	providerKey := desktopInfraProviderConfigKey(s, cfg, source.Kind)
	switch source.Kind {
	case "docker":
		if cfg.Docker.ReadOnly {
			return response
		}
		inspect, err := desktopInfraInspectDocker(ctx, cfg, source.Key, runtime)
		if err != nil || desktopInfraDockerProtected(inspect) {
			return response
		}
		currentCfg, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, false, request.Action)
		if !ok {
			return response
		}
		before := inspect
		endpoint := "/containers/" + url.PathEscape(source.Key) + "/" + request.Action
		callCtx, cancel := desktopInfraContext(ctx)
		_, code, err := desktopInfraDockerRequest(runtime, callCtx, tools.DockerConfig{Host: currentCfg.Docker.Host}, http.MethodPost, endpoint, "")
		cancel()
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		if err != nil {
			response.Status = "unknown"
		} else if code == http.StatusNoContent || code == http.StatusNotModified || code == http.StatusOK || code == http.StatusAccepted {
			readback, readErr := desktopInfraInspectDocker(ctx, cfg, source.Key, runtime)
			if readErr != nil || !desktopInfraDockerActionConfirmed(request.Action, before, readback) {
				response.Status = infra.ActionUnknown
			} else {
				response.Status = infra.ActionCompleted
			}
		} else if code == http.StatusRequestTimeout || code >= 500 {
			response.Status = infra.ActionUnknown
		}
	case "proxmox":
		resource, err := desktopInfraProxmoxResource(ctx, cfg, source)
		if err != nil {
			return response
		}
		currentCfg, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, false, request.Action)
		if !ok {
			return response
		}
		status, upid, err := tools.ProxmoxInfraAction(ctx, desktopInfraProxmoxConfig(currentCfg), resource.Node, resource.Type, resource.VMID, request.Action)
		desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		if err != nil {
			if status == "unknown" {
				response.Status = "unknown"
			}
			return response
		}
		response.Status = status
		response.UPID = upid
	case "fritzbox":
		client, cancel, err := desktopInfraNewFritzClient(ctx, s, cfg)
		if err != nil {
			return response
		}
		defer func() { client.Close(); cancel() }()
		if _, ok := desktopInfraCurrentWriteConfig(ctx, s, cfg, store, nodeID, source, providerKey, false, request.Action); !ok {
			return response
		}
		status, err := client.InfraWakeHost(source.Key)
		if status == infra.ActionUnknown || err != nil && status != infra.ActionFailed {
			desktopInfraInvalidate(runtime, source.Kind, source.Instance)
			response.Status = "unknown"
			return response
		}
		if status == infra.ActionCompleted {
			response.Status = infra.ActionAccepted
			desktopInfraInvalidate(runtime, source.Kind, source.Instance)
		}
	}
	return response
}

func desktopInfraDockerActionConfirmed(action string, before, after desktopInfraDockerInspect) bool {
	afterRunning := strings.EqualFold(after.State.Status, "running")
	switch action {
	case "start":
		return afterRunning
	case "stop":
		status := strings.ToLower(strings.TrimSpace(after.State.Status))
		return status == "exited" || status == "dead"
	case "restart":
		return afterRunning && (after.RestartCount > before.RestartCount || (before.State.StartedAt != "" && after.State.StartedAt != "" && before.State.StartedAt != after.State.StartedAt))
	default:
		return false
	}
}

func desktopInfraActionAllowed(cfg *config.Config, source, action string) bool {
	switch source {
	case "docker":
		return cfg.Docker.Enabled && !cfg.Docker.ReadOnly && (action == "start" || action == "stop" || action == "restart")
	case "proxmox":
		if !cfg.Proxmox.Enabled || cfg.Proxmox.ReadOnly {
			return false
		}
		if action == "start" {
			return true
		}
		return cfg.Proxmox.AllowDestructive && (action == "shutdown" || action == "reboot")
	case "fritzbox":
		return desktopInfraFritzAllowed(cfg) && !cfg.FritzBox.Network.ReadOnly && cfg.FritzBox.Network.SubFeatures.WakeOnLAN && action == "wake"
	default:
		return false
	}
}

func desktopInfraActionTargetAllowed(source infra.Source, action string) bool {
	if source.Kind != "proxmox" {
		return true
	}
	parts := strings.SplitN(source.Key, ":", 2)
	return len(parts) == 2 && (parts[0] == "qemu" || parts[0] == "lxc") && parts[1] != "" &&
		(action == "start" || action == "shutdown" || action == "reboot")
}

func desktopInfraActionNeedsConfirm(source, action string) bool {
	if source == "docker" {
		return action == "stop" || action == "restart"
	}
	if source == "proxmox" {
		return action == "shutdown" || action == "reboot"
	}
	return false
}

func desktopInfraInvalidate(runtime *desktopInfraRuntime, kind, instance string) {
	runtime.mu.Lock()
	if entry := runtime.cache[kind+"\x00"+instance]; entry != nil {
		entry.generation++
		entry.retryAt = time.Time{}
	}
	runtime.mu.Unlock()
}

func desktopInfraSourceEditStatus(err error) int {
	if errors.Is(err, infra.ErrConflict) {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
