package tools

import (
	"encoding/json"
	"fmt"
	pathpkg "path"
	"strings"
)

// dockerInspectProjection trims a full Engine inspect document to the fields
// the agent and the UI may see: credential-like environment values, command
// line secrets and labels are masked, and bind-mount sources are reduced to
// their last path element unless fullBindSources is set. The result has no
// "status" key; dockerInspectContainer adds it.
func dockerInspectProjection(full map[string]interface{}, fullBindSources bool) map[string]interface{} {
	result := map[string]interface{}{
		"id":     full["Id"],
		"name":   full["Name"],
		"state":  full["State"],
		"mounts": projectDockerInspectMounts(full["Mounts"], fullBindSources),
		"config": nil,
	}
	if cfg, ok := full["Config"].(map[string]interface{}); ok {
		result["config"] = map[string]interface{}{
			"image":  cfg["Image"],
			"env":    redactDockerInspectEnv(cfg["Env"]),
			"cmd":    redactDockerInspectArgs(cfg["Cmd"]),
			"labels": redactDockerInspectLabels(cfg["Labels"]),
		}
	}
	if netSettings, ok := full["NetworkSettings"].(map[string]interface{}); ok {
		result["network"] = map[string]interface{}{
			"ip_address": netSettings["IPAddress"],
			"ports":      netSettings["Ports"],
		}
	}
	if hostConfig, ok := full["HostConfig"].(map[string]interface{}); ok {
		if mode, ok := hostConfig["NetworkMode"].(string); ok {
			result["network_mode"] = mode
		}
		if readonly, ok := hostConfig["ReadonlyRootfs"].(bool); ok && readonly {
			result["readonly_rootfs"] = true
		}
		if tmpfs := dockerInspectTmpfs(hostConfig["Tmpfs"]); len(tmpfs) > 0 {
			result["tmpfs"] = tmpfs
		}
	}
	return result
}

// ProjectDockerInspect applies to a raw Engine inspect document (the body of
// GET /containers/{id}/json) exactly the trimming and redaction that
// DockerInspectContainer applies for the agent and the admin UI, without the
// "status" key. The Docker Manager uses it for the hosts it reaches through
// its own clients. Bind-mount sources are reduced to their last path element.
func ProjectDockerInspect(raw []byte) (map[string]interface{}, error) {
	var full map[string]interface{}
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, fmt.Errorf("parse inspect data: %w", err)
	}
	return dockerInspectProjection(full, false), nil
}

// RedactDockerLabels masks label values whose key names a credential and
// scrubs URL credentials and key=value secrets in the others, with the rules
// of the admin inspect. It returns a new map, except for a nil or empty map,
// which it returns as it is.
func RedactDockerLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return labels
	}
	generic := make(map[string]interface{}, len(labels))
	for key, value := range labels {
		generic[key] = value
	}
	redacted, _ := redactDockerInspectLabels(generic).(map[string]interface{})
	out := make(map[string]string, len(labels))
	for key, value := range redacted {
		out[key], _ = value.(string)
	}
	return out
}

// RedactDockerMountSource applies the admin inspect's rule to one mount source:
// a volume's source (Docker's own storage) stays complete, any other source
// (a bind's host path) keeps only its last path element.
func RedactDockerMountSource(mountType, source string) string {
	if source == "" || mountType == "volume" {
		return source
	}
	return pathpkg.Base(strings.ReplaceAll(source, `\`, "/"))
}
