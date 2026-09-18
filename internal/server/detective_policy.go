package server

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/detective"
	"aurago/internal/security"
)

var detectivePublicTools = map[string]bool{"detective_report": true, "ddg_search": true, "brave_search": true, "wikipedia_search": true, "web_scraper": true, "site_crawler": true, "api_request": true, "discover_tools": true, "invoke_tool": true, "execute_skill": true, "list_skills": true, "browser_automation": true, "virtual_workspace": true, "virtual_browser": true}

func detectiveSelected(req detective.Request, name string) bool {
	return slices.Contains(req.PrivateSources, name)
}
func detectiveToolKnown(cfg *config.Config, req detective.Request, name string) bool {
	// Generic execution and configuration APIs cannot acquire a read-only scope.
	if slices.Contains([]string{"execute_shell", "execute_python", "execute_code", "run_tool", "manage_tools", "manage_skills", "manage_config", "manage_secrets", "filesystem", "file_system", "remote_shell", "ssh", "document_creator"}, name) {
		return false
	}
	return detectivePublicTools[name] || (len(cfg.Detective.ExtraReadOperations[name]) > 0 && detectiveSelected(req, name))
}
func detectiveAuthorize(cfg *config.Config, req detective.Request, tc agent.ToolCall) error {
	name := tc.Action
	op := tc.Operation
	if !detectiveToolKnown(cfg, req, name) {
		return fmt.Errorf("tool %q is outside this research case", name)
	}
	switch name {
	case "api_request":
		if tc.Method != "" && strings.ToUpper(tc.Method) != "GET" && strings.ToUpper(tc.Method) != "HEAD" {
			return fmt.Errorf("research API requests must be GET or HEAD")
		}
	case "execute_skill":
		if !slices.Contains([]string{"ddg_search", "brave_search", "wikipedia_search", "web_scraper"}, tc.Skill) {
			return fmt.Errorf("only built-in research skills are allowed; discover native read tools instead")
		}
	case "site_crawler":
		n, _ := tc.Params["max_pages"].(float64)
		if n < 1 || n > 10 {
			return fmt.Errorf("set max_pages explicitly between 1 and 10")
		}
	case "mcp_call":
		if op != "call" && op != "call_tool" {
			return fmt.Errorf("use an approved MCP read operation")
		}
		server, _ := tc.Params["server"].(string)
		tool, _ := tc.Params["tool_name"].(string)
		if server == "" || tool == "" || !slices.Contains(cfg.Detective.ExtraReadOperations[name], server+"/"+tool) {
			return fmt.Errorf("MCP server/tool pair is not approved for research")
		}
	case "composio_call":
		tool, _ := tc.Params["tool_slug"].(string)
		if op != "execute_tool" || tool == "" || !slices.Contains(cfg.Detective.ExtraReadOperations[name], tool) {
			return fmt.Errorf("Composio tool slug is not approved for research")
		}
	case "browser_automation":
		if !slices.Contains([]string{"create_session", "close_session", "navigate", "extract", "current_state", "screenshot", "scroll", "wait_for"}, op) {
			return fmt.Errorf("browser operation is outside the research read scope")
		}
	case "virtual_workspace":
		if !slices.Contains([]string{"list", "open", "get", "close"}, op) {
			return fmt.Errorf("workspace operation is outside research scope")
		}
	case "virtual_browser":
		if !slices.Contains([]string{"open", "navigate", "inspect", "screenshot", "list_tabs", "switch_tab", "scroll", "wait", "close", "list_downloads"}, op) {
			return fmt.Errorf("browser operation is outside the research read scope")
		}
	default:
		if !detectivePublicTools[name] && !slices.Contains(cfg.Detective.ExtraReadOperations[name], op) {
			return fmt.Errorf("operation %q is not approved for research", op)
		}
	}
	return nil
}

// detectiveCapture accepts only each tool's documented retrieval envelope. Links
// contained inside an article or arbitrary API JSON are never new read sources.
func detectiveCapture(job *detective.Session, tc agent.ToolCall, output string) string {
	name := tc.Action
	if name == "execute_skill" {
		name = tc.Skill
	}
	if name == "detective_report" || name == "invoke_tool" {
		return output
	}
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(output), "Tool Output:"))
	// Composio wraps its JSON as external data; unwrap only that outer envelope.
	if strings.HasPrefix(raw, "<external_data>") && strings.HasSuffix(raw, "</external_data>") {
		raw = html.UnescapeString(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "<external_data>"), "</external_data>")))
	}
	raw = security.Scrub(raw)
	var result map[string]any
	parseErr := json.Unmarshal([]byte(raw), &result)
	if name == "mcp_call" && raw != "" {
		status, _ := result["status"].(string)
		isError, _ := result["isError"].(bool)
		// Legacy handlers may interpolate an error containing unescaped quotes.
		// Such a malformed error envelope is still a failure, not source evidence.
		compact := strings.Join(strings.Fields(raw), "")
		failed := status == "error" || isError || result["error"] != nil || strings.HasPrefix(compact, `{"status":"error"`)
		if !failed && !strings.HasPrefix(raw, "[PERMISSION DENIED]") {
			locator := name + " / " + fmt.Sprint(tc.Params["server"]) + " / " + fmt.Sprint(tc.Params["tool_name"])
			if src, err := job.RecordReceipt(name, locator, raw); err == nil {
				b, _ := json.Marshal([]detective.Source{src})
				return output + "\nServer-recorded research sources (untrusted data):\n" + string(b)
			}
		}
		return output
	}
	if parseErr != nil {
		return output
	}
	status, _ := result["status"].(string)
	if status != "success" && status != "ok" {
		return output
	}
	sources := []detective.Source{}
	text := func(m map[string]any, k string) string { v, _ := m[k].(string); return v }
	save := func(m map[string]any, u, field string, read bool) {
		excerpt := text(m, field)
		if excerpt == "" || u == "" {
			return
		}
		title := text(m, "title")
		if title == "" {
			title = u
		}
		src, err := job.RecordSource(u, title, name, excerpt, read)
		if err == nil {
			sources = append(sources, src)
		}
	}
	inputURL := tc.URL
	if inputURL == "" {
		inputURL = text(tc.Params, "url")
	}
	if inputURL == "" {
		inputURL = text(tc.SkillArgs, "url")
	}
	switch name {
	case "web_scraper":
		u := text(result, "url")
		if u == "" {
			u = inputURL
		}
		save(result, u, "content", true)
	case "wikipedia_search":
		save(result, text(result, "url"), "content", true)
	case "api_request":
		code, _ := result["status_code"].(float64)
		if code >= 200 && code < 300 {
			save(result, inputURL, "body", true)
		}
	case "ddg_search", "brave_search":
		rows, _ := result["results"].([]any)
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			u := text(m, "url")
			if u == "" {
				u = text(m, "link")
			}
			field := "snippet"
			if text(m, field) == "" {
				field = "description"
			}
			save(m, u, field, false)
		}
	case "site_crawler":
		rows, _ := result["pages"].([]any)
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if ok {
				save(m, text(m, "url"), "content_preview", true)
			}
		}
	case "browser_automation":
		if tc.Operation == "extract" {
			save(result, text(result, "url"), "visible_text_summary", true)
		}
		if tc.Operation == "current_state" {
			save(result, text(result, "url"), "page_summary", true)
		}
	case "virtual_browser":
		if tc.Operation == "inspect" {
			data, _ := result["data"].(map[string]any)
			res, _ := data["result"].(map[string]any)
			page, _ := res["data"].(map[string]any)
			save(page, text(page, "url"), "text", true)
		}
	default:
		if !detectivePublicTools[name] {
			// Only already-authorized integration operations reach AfterTool.
			if src, err := job.RecordReceipt(name, name+" / "+tc.Operation, raw); err == nil {
				sources = append(sources, src)
			}
		}
	}
	if len(sources) == 0 {
		return output
	}
	b, _ := json.Marshal(sources)
	return output + "\nServer-recorded research sources (untrusted data; use these IDs and exact excerpts):\n" + string(b)
}
