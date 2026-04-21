package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurago/internal/security"
)

const defaultMCPContainerWorkdir = "/workspace"

var (
	mcpTemplateRe       = regexp.MustCompile(`\{\{([a-zA-Z0-9._-]+)\}\}`)
	mcpWorkspaceSubdirs = []string{"input", "output", "cache", "tmp"}
)

func mcpRuntimeMode(runtimeName string) string {
	switch strings.ToLower(strings.TrimSpace(runtimeName)) {
	case "docker":
		return "docker"
	default:
		return "local"
	}
}

func ensureMCPHostWorkdir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create MCP workdir: %w", err)
	}
	for _, name := range mcpWorkspaceSubdirs {
		if err := os.MkdirAll(filepath.Join(path, name), 0o755); err != nil {
			return fmt.Errorf("create MCP workdir %q: %w", name, err)
		}
	}
	return nil
}

func resolveMCPTemplateValue(input string, server MCPServerConfig, useContainerPaths bool) (string, error) {
	if input == "" {
		return "", nil
	}
	hostWorkdir := strings.TrimSpace(server.HostWorkdir)
	containerWorkdir := strings.TrimSpace(server.ContainerWorkdir)
	if containerWorkdir == "" {
		containerWorkdir = defaultMCPContainerWorkdir
	}
	for alias, value := range server.Secrets {
		if strings.TrimSpace(value) != "" {
			security.RegisterSensitive(value)
		}
		server.Secrets[alias] = value
	}
	return mcpTemplateRe.ReplaceAllStringFunc(input, func(match string) string {
		parts := mcpTemplateRe.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		key := strings.ToLower(strings.TrimSpace(parts[1]))
		var base string
		switch key {
		case "workdir":
			if useContainerPaths {
				return containerWorkdir
			}
			return hostWorkdir
		case "workdir.input":
			base = "input"
		case "workdir.output":
			base = "output"
		case "workdir.cache":
			base = "cache"
		case "workdir.tmp":
			base = "tmp"
		default:
			if value, ok := server.Secrets[key]; ok && strings.TrimSpace(value) != "" {
				return value
			}
			// preserve unknown placeholders; caller validates afterward
			return match
		}
		if useContainerPaths {
			return filepath.ToSlash(filepath.Join(containerWorkdir, base))
		}
		return filepath.Join(hostWorkdir, base)
	}), nil
}

func resolveMCPLaunchArgsAndEnv(server MCPServerConfig, useContainerPaths bool) ([]string, map[string]string, error) {
	args := make([]string, 0, len(server.Args))
	for _, arg := range server.Args {
		resolved, err := resolveMCPTemplateValue(arg, server, useContainerPaths)
		if err != nil {
			return nil, nil, err
		}
		if strings.Contains(resolved, "{{") {
			return nil, nil, fmt.Errorf("unresolved placeholder in MCP arg %q", arg)
		}
		args = append(args, expandMCPPathValue(resolved))
	}

	env := make(map[string]string, len(server.Env))
	for key, value := range server.Env {
		resolved, err := resolveMCPTemplateValue(value, server, useContainerPaths)
		if err != nil {
			return nil, nil, err
		}
		if strings.Contains(resolved, "{{") {
			return nil, nil, fmt.Errorf("unresolved placeholder in MCP env %q", key)
		}
		env[key] = expandMCPPathValue(resolved)
	}
	return args, env, nil
}

func normalizeMCPResultText(text, hostWorkdir, containerWorkdir string) string {
	hostWorkdir = strings.TrimSpace(hostWorkdir)
	containerWorkdir = strings.TrimSpace(containerWorkdir)
	if text == "" || hostWorkdir == "" || containerWorkdir == "" {
		return text
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(text), &decoded); err == nil {
		normalized := normalizeMCPResultValue(decoded, hostWorkdir, containerWorkdir)
		if data, err := json.Marshal(normalized); err == nil {
			return string(data)
		}
	}
	return strings.ReplaceAll(text, filepath.ToSlash(containerWorkdir), hostWorkdir)
}

func normalizeMCPResultValue(value interface{}, hostWorkdir, containerWorkdir string) interface{} {
	switch typed := value.(type) {
	case string:
		containerPath := filepath.ToSlash(containerWorkdir)
		// Only treat as a prefix match if the container path is followed by
		// a slash or the end of the string — prevents /workspace matching
		// inside /workspace-old.
		if strings.HasPrefix(typed, containerPath) {
			after := len(containerPath)
			if after == len(typed) || typed[after] == '/' {
				rel := strings.TrimPrefix(typed, containerPath)
				rel = strings.TrimPrefix(rel, "/")
				if rel == "" {
					return hostWorkdir
				}
				return filepath.Join(hostWorkdir, filepath.FromSlash(rel))
			}
		}
		// Only replace whole-path occurrences to avoid partial substring
		// replacements (e.g. /work matching inside /workspace).
		sepPath := containerPath
		if !strings.HasSuffix(sepPath, "/") {
			sepPath += "/"
		}
		sepHost := hostWorkdir
		if !strings.HasSuffix(sepHost, string(filepath.Separator)) {
			sepHost += string(filepath.Separator)
		}
		return strings.ReplaceAll(typed, sepPath, sepHost)
	case map[string]interface{}:
		for key, item := range typed {
			typed[key] = normalizeMCPResultValue(item, hostWorkdir, containerWorkdir)
		}
		return typed
	case []interface{}:
		for i, item := range typed {
			typed[i] = normalizeMCPResultValue(item, hostWorkdir, containerWorkdir)
		}
		return typed
	default:
		return value
	}
}
