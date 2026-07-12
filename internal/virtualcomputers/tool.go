package virtualcomputers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var readonlyOperations = map[string]bool{
	"status":         true,
	"list_machines":  true,
	"get":            true,
	"get_machine":    true,
	"screenshot":     true,
	"download":       true,
	"list_templates": true,
	"list_volumes":   true,
}

var mutatingOperations = map[string]bool{
	"launch":           true,
	"create":           true,
	"destroy":          true,
	"delete":           true,
	"exec":             true,
	"extend":           true,
	"fork":             true,
	"upload":           true,
	"publish":          true,
	"create_volume":    true,
	"save_machine":     true,
	"run_shell_task":   true,
	"run_desktop_task": true,
}

func ExecuteTool(ctx context.Context, cfg ToolConfig, args map[string]interface{}) string {
	if !cfg.Enabled {
		return toolJSON("error", "disabled", "virtual computers are disabled in config", nil)
	}
	if !cfg.ToolGate {
		return toolJSON("error", "tool_disabled", "virtual_computers tool is disabled in config", nil)
	}
	if strings.TrimSpace(cfg.Provider) != "" && cfg.Provider != ProviderBoringComputers {
		return toolJSON("error", "unsupported_provider", "only boring_computers is supported in this version", nil)
	}

	op := strings.ToLower(strings.TrimSpace(toolString(args, "operation", "action")))
	if op == "" {
		op = "status"
	}
	if cfg.ReadOnly && !readonlyOperations[op] {
		return toolJSON("error", "readonly", "virtual computers are in read-only mode", map[string]interface{}{"operation": op})
	}
	if mutatingOperations[op] && cfg.BoringdURL == "" && cfg.ControlPlane.BoringdURL == "" {
		return toolJSON("error", "not_configured", "boringd URL is not configured", nil)
	}
	if err := enforceOperationGates(cfg, op, args); err != nil {
		return toolJSON("error", err.code, err.message, nil)
	}

	client, err := NewClient(ClientConfig{
		BaseURL: firstNonEmpty(cfg.BoringdURL, cfg.ControlPlane.BoringdURL),
		Token:   cfg.BoringToken,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return toolJSON("error", "client_config", err.Error(), nil)
	}

	switch op {
	case "status":
		payload, err := client.Status(ctx)
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "virtual computers status", payload)
	case "list_machines", "list":
		machines, err := client.ListMachines(ctx)
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machines listed", map[string]interface{}{"machines": machines})
	case "get", "get_machine":
		machine, err := client.GetMachine(ctx, requiredMachineID(args))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine loaded", map[string]interface{}{"machine": machine})
	case "launch", "create":
		req := LaunchMachineRequest{
			Template:      firstNonEmpty(toolString(args, "template"), cfg.DefaultTemplate),
			Name:          toolString(args, "name"),
			TTLSeconds:    clampTTL(toolInt(args, cfg.DefaultTTLSeconds, "ttl_seconds", "ttl"), cfg.maxTTL()),
			AllowInternet: toolBool(args, "allow_internet", "internet"),
			Persistent:    toolBool(args, "persistent"),
			Volumes:       toolStringSlice(args, "volumes"),
		}
		machine, err := client.LaunchMachine(ctx, req)
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine launched", map[string]interface{}{"machine": machine})
	case "destroy", "delete":
		if err := client.DestroyMachine(ctx, requiredMachineID(args)); err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine destroyed", nil)
	case "extend":
		machine, err := client.ExtendMachine(ctx, requiredMachineID(args), clampTTL(toolInt(args, cfg.DefaultTTLSeconds, "ttl_seconds", "ttl"), cfg.maxTTL()))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine extended", map[string]interface{}{"machine": machine})
	case "fork":
		machine, err := client.ForkMachine(ctx, requiredMachineID(args), clampTTL(toolInt(args, cfg.DefaultTTLSeconds, "ttl_seconds", "ttl"), cfg.maxTTL()))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine forked", map[string]interface{}{"machine": machine})
	case "exec":
		result, err := client.Exec(ctx, requiredMachineID(args), ExecRequest{
			Command: toolString(args, "command"),
			Args:    toolStringSlice(args, "args"),
			Timeout: toolInt(args, 60, "timeout_seconds", "timeout"),
		})
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "command executed", map[string]interface{}{"result": result})
	case "screenshot":
		shot, err := client.Screenshot(ctx, requiredMachineID(args))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "screenshot captured", map[string]interface{}{"screenshot": shot})
	case "upload":
		content := []byte(toolString(args, "content"))
		if encoded := toolString(args, "content_base64", "data_base64"); encoded != "" {
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return toolJSON("error", "invalid_base64", "content_base64 is invalid", nil)
			}
			content = decoded
		}
		payload, err := client.Upload(ctx, requiredMachineID(args), toolString(args, "path", "remote_path"), content)
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "file uploaded", payload)
	case "download":
		data, contentType, err := client.Download(ctx, requiredMachineID(args), toolString(args, "path", "remote_path"))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "file downloaded", map[string]interface{}{
			"content_type":   contentType,
			"content_base64": base64.StdEncoding.EncodeToString(data),
		})
	case "list_templates":
		templates, err := client.ListTemplates(ctx)
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "templates listed", map[string]interface{}{"templates": templates})
	case "publish":
		payload, err := client.Publish(ctx, requiredMachineID(args))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine published", payload)
	case "list_volumes":
		volumes, err := client.ListVolumes(ctx)
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "volumes listed", map[string]interface{}{"volumes": volumes})
	case "create_volume":
		volume, err := client.CreateVolume(ctx, toolString(args, "name"), int64(toolInt(args, 0, "size_bytes")))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "volume created", map[string]interface{}{"volume": volume})
	case "save_machine":
		payload, err := client.SaveMachine(ctx, requiredMachineID(args), toolString(args, "name"))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "machine saved", payload)
	case "run_shell_task":
		payload, err := client.RunAgentTask(ctx, requiredMachineID(args), "shell-agent", firstNonEmpty(toolString(args, "instruction"), toolString(args, "command")))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "shell task started", payload)
	case "run_desktop_task":
		payload, err := client.RunAgentTask(ctx, requiredMachineID(args), "agent", toolString(args, "instruction"))
		if err != nil {
			return toolJSON("error", "upstream_error", err.Error(), nil)
		}
		return toolJSON("ok", "", "desktop task started", payload)
	default:
		return toolJSON("error", "unsupported_operation", "unsupported virtual_computers operation", map[string]interface{}{"operation": op})
	}
}

type gateError struct {
	code    string
	message string
}

func (e gateError) Error() string { return e.message }

func enforceOperationGates(cfg ToolConfig, op string, args map[string]interface{}) *gateError {
	if (op == "launch" || op == "create") && toolBool(args, "allow_internet", "internet") && !cfg.AllowInternet {
		return &gateError{code: "internet_disabled", message: "internet access for virtual computers is disabled"}
	}
	if (op == "launch" || op == "create") && toolBool(args, "persistent") && !cfg.AllowPersistent {
		return &gateError{code: "persistent_disabled", message: "persistent virtual computers are disabled"}
	}
	if op == "publish" && !cfg.AllowPublish {
		return &gateError{code: "publish_disabled", message: "publishing virtual computers is disabled"}
	}
	if (op == "create_volume" || (op == "launch" && len(toolStringSlice(args, "volumes")) > 0)) && !cfg.AllowVolumes {
		return &gateError{code: "volumes_disabled", message: "virtual computer volumes are disabled"}
	}
	if (op == "run_shell_task" || op == "run_desktop_task") && !cfg.AllowAgentTasks {
		return &gateError{code: "agent_tasks_disabled", message: "virtual computer agent tasks are disabled"}
	}
	return nil
}

func (cfg ToolConfig) maxTTL() int {
	if cfg.MaxTTLSeconds <= 0 {
		return MaxTTLSeconds
	}
	if cfg.MaxTTLSeconds > MaxTTLSeconds {
		return MaxTTLSeconds
	}
	return cfg.MaxTTLSeconds
}

func requiredMachineID(args map[string]interface{}) string {
	return firstNonEmpty(toolString(args, "machine_id"), toolString(args, "id"))
}

func toolJSON(status, code, message string, payload interface{}) string {
	out := map[string]interface{}{
		"status":  status,
		"message": message,
	}
	if code != "" {
		out["code"] = code
	}
	if payload != nil {
		out["data"] = payload
	}
	data, err := json.Marshal(out)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","code":"encode_error","message":%q}`, err.Error())
	}
	return string(data)
}

func toolString(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if args == nil {
			continue
		}
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case fmt.Stringer:
			if strings.TrimSpace(t.String()) != "" {
				return strings.TrimSpace(t.String())
			}
		default:
			s := strings.TrimSpace(fmt.Sprint(t))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func toolInt(args map[string]interface{}, def int, keys ...string) int {
	for _, key := range keys {
		if args == nil {
			continue
		}
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case int:
			return t
		case int64:
			return int(t)
		case float64:
			return int(t)
		case json.Number:
			i, _ := t.Int64()
			return int(i)
		case string:
			if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
				return i
			}
		}
	}
	return def
}

func toolBool(args map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if args == nil {
			continue
		}
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t
		case string:
			b, _ := strconv.ParseBool(strings.TrimSpace(t))
			return b
		}
	}
	return false
}

func toolStringSlice(args map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		if args == nil {
			continue
		}
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case []string:
			return t
		case []interface{}:
			out := make([]string, 0, len(t))
			for _, item := range t {
				if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
					out = append(out, s)
				}
			}
			return out
		case string:
			if strings.TrimSpace(t) == "" {
				continue
			}
			parts := strings.Split(t, ",")
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				if s := strings.TrimSpace(part); s != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
