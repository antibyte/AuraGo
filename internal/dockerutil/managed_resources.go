package dockerutil

import (
	"strconv"
	"strings"
)

const (
	// LocalLLMOwner is the canonical owner value for AuraGo-Qwen resources.
	LocalLLMOwner = "local-llm"

	LocalLLMContainerName   = "aurago-local-llm"
	LocalLLMKeySeedName     = "aurago-local-llm-key-seed"
	LocalLLMModelVolumeName = "aurago_models"
	LocalLLMKeyVolumeName   = "aurago_local_llm_runtime"
)

// ManagedBy recognizes both the canonical AuraGo label and the legacy labels
// used by early AuraGo-Qwen integration builds.
func ManagedBy(labels map[string]string, owner string) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(labels["aurago.managed"]), owner) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(labels["com.aurago.managed"]), "true") &&
		strings.EqualFold(strings.TrimSpace(labels["com.aurago.owner"]), owner)
}

// ManagedLabels returns the canonical labels for an AuraGo-managed resource.
func ManagedLabels(owner, component, role, fingerprint string) map[string]string {
	labels := map[string]string{
		"aurago.managed":   strings.TrimSpace(owner),
		"aurago.component": strings.TrimSpace(component),
		"aurago.role":      strings.TrimSpace(role),
	}
	if fingerprint = strings.TrimSpace(fingerprint); fingerprint != "" {
		labels["aurago.fingerprint"] = fingerprint
	}
	return labels
}

// IsLocalLLMContainerName recognizes every reserved AuraGo-Qwen container.
func IsLocalLLMContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	if name == LocalLLMContainerName || name == LocalLLMKeySeedName {
		return true
	}
	return strings.Contains(name, "-"+LocalLLMContainerName+"-") ||
		strings.Contains(name, "_"+LocalLLMContainerName+"_") ||
		strings.Contains(name, "-"+LocalLLMKeySeedName+"-") ||
		strings.Contains(name, "_"+LocalLLMKeySeedName+"_")
}

// IsLocalLLMVolumeName recognizes every reserved AuraGo-Qwen volume.
func IsLocalLLMVolumeName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == LocalLLMModelVolumeName ||
		name == LocalLLMKeyVolumeName ||
		strings.HasSuffix(name, "_"+LocalLLMModelVolumeName) ||
		strings.HasSuffix(name, "_"+LocalLLMKeyVolumeName)
}

// ParseNumericGroupIDs validates, deduplicates, and bounds host GPU group IDs.
// Group zero is intentionally rejected and no more than 16 supplemental groups
// are accepted from an installer- or administrator-provided environment value.
func ParseNumericGroupIDs(raw string) []string {
	const maxGroups = 16
	result := make([]string, 0, maxGroups)
	seen := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		number, err := strconv.ParseUint(value, 10, 31)
		if err != nil || number == 0 {
			continue
		}
		normalized := strconv.FormatUint(number, 10)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
		if len(result) == maxGroups {
			break
		}
	}
	return result
}
