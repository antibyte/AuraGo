package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// InfraProxmoxResource is the bounded projection used by Desktop Infra.
// Provider response maps never leave this package.
type InfraProxmoxResource struct {
	Type      string  `json:"type"`
	VMID      string  `json:"vmid"`
	Node      string  `json:"node"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu,omitempty"`
	MaxCPU    float64 `json:"max_cpu,omitempty"`
	Memory    int64   `json:"memory_bytes,omitempty"`
	MaxMemory int64   `json:"max_memory_bytes,omitempty"`
	Disk      int64   `json:"disk_bytes,omitempty"`
	MaxDisk   int64   `json:"max_disk_bytes,omitempty"`
	Tags      string  `json:"tags,omitempty"`
}

// ProxmoxInfraResources reads one cluster resource list and keeps only fields
// required for the Infra candidate view.
func ProxmoxInfraResources(ctx context.Context, cfg ProxmoxConfig) ([]InfraProxmoxResource, error) {
	rows, _, err := proxmoxGetClusterResourceList(ctx, cfg, "")
	if err != nil {
		return nil, fmt.Errorf("read proxmox resources: %w", err)
	}
	out := make([]InfraProxmoxResource, 0, len(rows))
	for _, row := range rows {
		kind := strings.TrimSpace(fmt.Sprint(row["type"]))
		if kind != "qemu" && kind != "lxc" && kind != "node" {
			continue
		}
		node := proxmoxInfraString(row["node"])
		if _, nodeErr := proxmoxNodePath(node); nodeErr != nil {
			return nil, fmt.Errorf("proxmox resource identity is incomplete")
		}
		vmid := ""
		if kind != "node" {
			vmid = proxmoxInfraVMID(row["vmid"])
			if proxmoxValidateVMID(vmid) != nil {
				return nil, fmt.Errorf("proxmox resource identity is incomplete")
			}
		}
		out = append(out, InfraProxmoxResource{
			Type: kind, VMID: vmid, Node: node,
			Name: proxmoxInfraString(row["name"]), Status: proxmoxInfraString(row["status"]),
			CPU: proxmoxInfraFloat(row["cpu"]), MaxCPU: proxmoxInfraFloat(row["maxcpu"]),
			Memory: proxmoxInfraInt(row["mem"]), MaxMemory: proxmoxInfraInt(row["maxmem"]),
			Disk: proxmoxInfraInt(row["disk"]), MaxDisk: proxmoxInfraInt(row["maxdisk"]),
			Tags: proxmoxInfraString(row["tags"]),
		})
	}
	return out, nil
}

// InfraProxmoxConfig contains the editable fields and PVE's optimistic
// concurrency digest for a single VM or container.
type InfraProxmoxConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Digest      string `json:"digest"`
	Pending     bool   `json:"pending,omitempty"`
}

func ProxmoxInfraReadConfig(ctx context.Context, cfg ProxmoxConfig, node, kind, vmid string) (InfraProxmoxConfig, error) {
	nodePath, err := proxmoxInfraPath(node, kind, vmid)
	if err != nil {
		return InfraProxmoxConfig{}, err
	}
	raw, code, err := proxmoxRequest(ctx, cfg, "GET", nodePath+"/config", "")
	if err != nil {
		return InfraProxmoxConfig{}, fmt.Errorf("read proxmox config: %w", err)
	}
	if code != 200 {
		return InfraProxmoxConfig{}, fmt.Errorf("proxmox config returned HTTP %d", code)
	}
	var values map[string]any
	if err := json.Unmarshal(proxmoxExtractData(raw), &values); err != nil {
		return InfraProxmoxConfig{}, fmt.Errorf("decode proxmox config: %w", err)
	}
	nameField := "name"
	if kind == "lxc" {
		nameField = "hostname"
	}
	result := InfraProxmoxConfig{
		Name:        proxmoxInfraString(values[nameField]),
		Description: proxmoxInfraString(values["description"]),
		Tags:        proxmoxInfraString(values["tags"]),
		Digest:      proxmoxInfraString(values["digest"]),
	}
	if kind == "qemu" {
		// The default QEMU config projection includes pending values. Read the
		// active view separately so a saved name/description/tag change that
		// still needs a restart can be reported as pending, not applied.
		currentRaw, currentCode, err := proxmoxRequest(ctx, cfg, "GET", nodePath+"/config?current=1", "")
		if err != nil {
			return InfraProxmoxConfig{}, fmt.Errorf("read current proxmox VM configuration: %w", err)
		}
		if currentCode != 200 {
			return InfraProxmoxConfig{}, fmt.Errorf("read current proxmox VM configuration returned HTTP %d", currentCode)
		}
		var current map[string]any
		if err := json.Unmarshal(proxmoxExtractData(currentRaw), &current); err != nil {
			return InfraProxmoxConfig{}, fmt.Errorf("decode current proxmox VM configuration: %w", err)
		}
		result.Pending = result.Name != proxmoxInfraString(current["name"]) ||
			result.Description != proxmoxInfraString(current["description"]) || result.Tags != proxmoxInfraString(current["tags"])
	}
	return result, nil
}

// ProxmoxInfraEditConfig performs one digest-conditional write and verifies it
// with a fresh read. A transport/read failure after the write is unknown.
func ProxmoxInfraEditConfig(ctx context.Context, cfg ProxmoxConfig, node, kind, vmid string, before InfraProxmoxConfig, name, description, tags string) (status string, err error) {
	if cfg.ReadOnly {
		return "failed", errors.New("proxmox is read-only")
	}
	nodePath, err := proxmoxInfraPath(node, kind, vmid)
	if err != nil {
		return "failed", err
	}
	current, err := ProxmoxInfraReadConfig(ctx, cfg, node, kind, vmid)
	if err != nil {
		return "failed", err
	}
	if current.Digest == "" || before.Digest == "" || current.Digest != before.Digest || current.Name != before.Name || current.Description != before.Description || current.Tags != before.Tags || current.Pending != before.Pending {
		return "conflict", errors.New("proxmox config changed")
	}
	form := url.Values{"digest": {current.Digest}}
	nameField := "name"
	if kind == "lxc" {
		nameField = "hostname"
	}
	form.Set(nameField, name)
	form.Set("description", description)
	form.Set("tags", tags)
	_, code, err := proxmoxRequest(ctx, cfg, "PUT", nodePath+"/config", form.Encode())
	if err != nil {
		return "unknown", fmt.Errorf("proxmox write outcome unknown: %w", err)
	}
	if code < 200 || code >= 300 {
		if code == http.StatusRequestTimeout || code >= 500 {
			return "unknown", fmt.Errorf("proxmox write outcome unknown after HTTP %d", code)
		}
		return "failed", fmt.Errorf("proxmox config update returned HTTP %d", code)
	}
	readback, err := ProxmoxInfraReadConfig(ctx, cfg, node, kind, vmid)
	if err != nil {
		return "unknown", fmt.Errorf("proxmox config readback unavailable: %w", err)
	}
	if readback.Name != name || readback.Description != description || readback.Tags != tags {
		return "unknown", errors.New("proxmox config readback mismatch")
	}
	if readback.Pending {
		return "pending", nil
	}
	return "completed", nil
}

// ProxmoxInfraAction admits only the Infra action subset. Transport or
// response decoding failures after a POST are reported as unknown.
func ProxmoxInfraAction(ctx context.Context, cfg ProxmoxConfig, node, kind, vmid, action string) (status, upid string, err error) {
	nodePath, err := proxmoxInfraPath(node, kind, vmid)
	if err != nil {
		return "failed", "", err
	}
	switch action {
	case "start":
	case "shutdown", "reboot":
		if !cfg.AllowDestructive {
			return "failed", "", errors.New("proxmox destructive actions are disabled")
		}
	default:
		return "failed", "", errors.New("unsupported proxmox action")
	}
	if cfg.ReadOnly {
		return "failed", "", errors.New("proxmox is read-only")
	}
	raw, code, err := proxmoxRequest(ctx, cfg, "POST", nodePath+"/status/"+action, "")
	if err != nil {
		return "unknown", "", fmt.Errorf("proxmox action outcome unknown: %w", err)
	}
	if code < 200 || code >= 300 {
		if code == http.StatusRequestTimeout || code >= 500 {
			return "unknown", "", fmt.Errorf("proxmox action outcome unknown after HTTP %d", code)
		}
		return "failed", "", fmt.Errorf("proxmox action returned HTTP %d", code)
	}
	var result struct {
		Data string `json:"data"`
	}
	if json.Unmarshal(raw, &result) != nil || strings.TrimSpace(result.Data) == "" {
		return "unknown", "", errors.New("proxmox action accepted without a valid task id")
	}
	upid = strings.TrimSpace(result.Data)
	if !proxmoxInfraSafeUPID(upid) {
		return "unknown", "", errors.New("proxmox action returned an invalid task id")
	}
	return "accepted", upid, nil
}

func proxmoxInfraSafeUPID(value string) bool {
	if len(value) > 512 || !strings.HasPrefix(value, "UPID:") {
		return false
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(":._-@!+", r)) {
			return false
		}
	}
	return true
}

func proxmoxInfraPath(node, kind, vmid string) (string, error) {
	_, nodeSegment, err := proxmoxRequireNode(ProxmoxConfig{}, node)
	if err != nil {
		return "", err
	}
	if kind != "qemu" && kind != "lxc" {
		return "", errors.New("invalid proxmox resource type")
	}
	if err := proxmoxValidateVMID(vmid); err != nil {
		return "", errors.New("invalid proxmox VMID")
	}
	return "/nodes/" + nodeSegment + "/" + kind + "/" + url.PathEscape(vmid), nil
}

func proxmoxInfraString(raw any) string {
	value, _ := raw.(string)
	return strings.TrimSpace(value)
}

func proxmoxInfraVMID(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		if value <= 0 || value > 9_007_199_254_740_991 || math.Trunc(value) != value {
			return ""
		}
		return strconv.FormatInt(int64(value), 10)
	case json.Number:
		parsed, err := strconv.ParseInt(string(value), 10, 64)
		if err == nil && parsed > 0 {
			return strconv.FormatInt(parsed, 10)
		}
	}
	return ""
}

func proxmoxInfraFloat(raw any) float64 {
	value, _ := raw.(float64)
	return value
}

func proxmoxInfraInt(raw any) int64 {
	switch value := raw.(type) {
	case float64:
		if value > 0 && value < float64(^uint64(0)>>1) {
			return int64(value)
		}
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		if parsed > 0 {
			return parsed
		}
	}
	return 0
}
