package agent

import openai "github.com/sashabaranov/go-openai"

func appendStoreAppSchemas(out []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	page := func(operations ...string) map[string]interface{} {
		return map[string]interface{}{
			"operation": map[string]interface{}{"type": "string", "enum": operations},
			"limit":     map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 50, "description": "Page size (default 25)."},
			"offset":    map[string]interface{}{"type": "integer", "minimum": 0, "description": "Page offset (default 0)."},
		}
	}
	if ff.NodeREDEnabled {
		properties := page("list_flows", "trigger")
		properties["node_id"] = prop("string", "Administratively allowed Inject ID, required for trigger; uses the node's existing payload.")
		out = append(out, tool("node_red", "List existing Node-RED flow names and allowed Inject nodes, or trigger one. A trigger acknowledges acceptance, not completion. No flow editing or runtime deployment.", schema(properties, "operation")))
	}
	if ff.OliveTinEnabled {
		properties := page("list_actions", "execute", "get_execution")
		properties["action_id"] = prop("string", "Allowed action binding ID, required for execute and get_execution.")
		properties["execution_id"] = prop("string", "Execution tracking ID returned by execute; required for get_execution.")
		properties["arguments"] = map[string]interface{}{"type": "array", "maxItems": 64, "description": "Configured action arguments only; no shell commands.", "items": schema(map[string]interface{}{"name": prop("string", "Argument name from the action definition."), "value": prop("string", "Argument value, validated by OliveTin.")}, "name", "value")}
		out = append(out, tool("olivetin", "List allowed OliveTin actions, execute a predefined action, or inspect its execution. Unknown outcomes must not be retried automatically.", schema(properties, "operation")))
	}
	if ff.BeszelEnabled {
		properties := page("list_systems", "host_metrics", "container_metrics")
		properties["system_id"] = prop("string", "Beszel system ID, required for host_metrics and container_metrics.")
		out = append(out, tool("beszel", "Read connected Beszel systems and latest host/container metrics, including timestamps and stale-data indicators. No administration or alerts.", schema(properties, "operation")))
	}
	if ff.FileBrowserEnabled {
		properties := page("list", "search", "read")
		properties["path"] = prop("string", "Path within the administrator-selected FileBrowser source (default root for list).")
		properties["query"] = prop("string", "Literal filename search term, required for search.")
		out = append(out, tool("filebrowser", "Read the connected FileBrowser source. Small text is returned inline; larger/binary files are copied to a new Shared path when file publication is allowed. No changes to source files.", schema(properties, "operation")))
	}
	if ff.StirlingPDFEnabled {
		properties := map[string]interface{}{
			"operation": map[string]interface{}{"type": "string", "enum": []string{"ocr", "split", "merge", "cert_sign"}},
			"files":     map[string]interface{}{"type": "array", "minItems": 1, "maxItems": 20, "items": prop("string", "Saved PDF path relative to the Desktop workspace."), "description": "One PDF, or 2-20 PDFs to merge in the specified order."},
			"languages": map[string]interface{}{"type": "array", "minItems": 1, "maxItems": 8, "items": prop("string", "Installed OCR language, e.g. deu or eng."), "description": "Required for OCR."},
			"pages":     prop("string", "Required for split: all, page numbers or ranges such as 2,5-8. Split points are inclusive."),
		}
		out = append(out, tool("stirling_pdf", "Process saved PDFs through the connected Stirling API. Outputs use new Shared paths; originals stay unchanged. Signing separately requires the administrator-approved server certificate; no private keys or certificate management.", schema(properties, "operation", "files")))
	}
	if ff.ZiroExportEnabled {
		out = append(out, tool("ziro_export", "Export saved KiCad projects below Shared in an isolated KiCad 10 container. Produces a new KiCad netlist or a ZIP with Gerber-X2, Excellon drill files and a manifest. Never includes unsaved editor drafts or changes sources.", schema(map[string]interface{}{
			"operation": map[string]interface{}{"type": "string", "enum": []string{"netlist", "manufacturing"}},
			"path":      prop("string", "Saved Shared project path (.kicad_pro), or schematic for netlist / PCB for manufacturing. Keep relative libraries inside its project directory."),
		}, "operation", "path")))
	}
	return out
}
