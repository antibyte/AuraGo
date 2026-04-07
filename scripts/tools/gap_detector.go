// gap_detector.go — Tool Manual Gap Analysis
// Run: go run scripts/tools/gap_detector.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type ToolInfo struct {
	Action      string   `json:"action"`
	Operations  []string `json:"operations,omitempty"`
	HasManual   bool     `json:"has_manual"`
	ManualPath  string   `json:"manual_path,omitempty"`
	ManualScore *Score   `json:"score,omitempty"`
}

type Score struct {
	Operations int `json:"operations"`
	Examples   int `json:"examples"`
	Parameters int `json:"parameters"`
	Notes      int `json:"notes"`
	Config     int `json:"config"`
	Total      int `json:"total"`
}

type GapReport struct {
	TotalDispatchTools int        `json:"total_dispatch_tools"`
	TotalManuals       int        `json:"total_manuals"`
	Missing            []ToolInfo `json:"missing"`
	Outdated           []ToolInfo `json:"outdated"`
	Unscored           []ToolInfo `json:"unscored"`
	LowScore           []ToolInfo `json:"low_score"`
	UpToDate           []ToolInfo `json:"up_to_date"`
}

func main() {
	// Paths
	dispatchPath := filepath.Join("internal", "agent", "agent_dispatch_infra.go")
	manualsDir := filepath.Join("prompts", "tools_manuals")
	reportPath := filepath.Join("reports", "tool_manual_gaps.json")

	// Parse dispatch file
	tools := parseDispatch(dispatchPath)
	fmt.Printf("Found %d tools in dispatch\n", len(tools))

	// Scan manuals
	manualFiles := scanManuals(manualsDir)
	fmt.Printf("Found %d manual files\n", len(manualFiles))

	// Match and analyze
	report := analyzeGaps(tools, manualFiles, manualsDir)

	// Write report
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(reportPath, data, 0644)
	fmt.Printf("\nReport written to: %s\n", reportPath)

	// Print summary
	printSummary(report)
}

// Known top-level tools from dispatchInfra function signature
var knownTools = []string{
	"co_agent", "co_agents",
	"mdns_scan",
	"mac_lookup",
	"tts",
	"chromecast",
	"manage_webhooks",
	"proxmox", "proxmox_ve",
	"ollama", "ollama_management",
	"tailscale",
	"cloudflare_tunnel",
	"ansible",
	"invasion_control",
	"remote_control",
	"github",
	"netlify",
	"mqtt_publish", "mqtt_subscribe", "mqtt_unsubscribe", "mqtt_get_messages",
	"mcp_call",
	"adguard", "adguard_home",
	"fritzbox", "fritzbox_system", "fritzbox_network", "fritzbox_telephony", "fritzbox_smarthome", "fritzbox_storage", "fritzbox_tv",
	"truenas",
	"jellyfin",
	"firewall", "firewall_rules", "iptables",
	"webhook",
	"homeassistant",
	"docker",
	"execute_shell",
	"execute_python",
	"send_notification",
	"manage_updates",
	"manage_missions",
	"manage_notes",
	"manage_plan",
	"manage_sql_connections",
	"memory",
	"smart_memory",
	"core_memory",
	"knowledge_graph",
	"remember",
	"context_memory",
	"optimize_memory",
	"site_monitor",
	"cron_scheduler",
	"wait_for_event",
	"follow_up",
	"execute_skill",
	"skill_manager",
	"skills_engine",
	"process_analyzer",
	"content_summary",
	"form_automation",
	"web_scraper",
	"site_crawler",
	"web_performance_audit",
	"pdf_operations", "pdf_extractor",
	"json_editor", "yaml_editor", "xml_editor",
	"file_search", "file_reader_advanced", "smart_file_read",
	"filesystem",
	"detect_file_type",
	"image_processing", "image_generation",
	"transcribe_audio", "analyze_image", "send_image", "send_document",
	"email",
	"onedrive",
	"webdav",
	"koofr",
	"s3_storage",
	"paperless",
	"truenas",
	"adguard",
	"meshcentral",
	"homepage",
	"homepage_deploy",
	"homepage_build_autofix",
	"homepage_container",
	"homepage_editors",
	"homepage_files",
	"homepage_git",
	"homepage_local_server",
	"homepage_revisions",
	"homepage_registry",
	"homepage_templates",
	"google_workspace",
	"brave_search",
	"ddg_search",
	"wikipedia_search",
	"dns_lookup",
	"network_ping",
	"port_scanner",
	"whois_lookup",
	"mac_lookup",
	"mdns",
	"upnp_scan",
	"virustotal_scan",
	"archive",
	"sandbox",
	"execute_surgery",
	"exit_lifeboat",
	"initiate_handover",
	"process_management",
	"secrets_vault",
	"system_metrics",
	"truenas",
	"jellyfin",
	"telnyx",
	"address_book",
	"discord",
	"koofr",
	"netlify_extras",
	"ollama_embeddings",
	"ollama_managed",
	"media_seed",
	"media_registry",
	"pipecat",
	"piper_tts",
	"wyoming",
	"wol",
}

func parseDispatch(path string) map[string]*ToolInfo {
	tools := make(map[string]*ToolInfo)

	for _, toolName := range knownTools {
		tools[toolName] = &ToolInfo{Action: toolName, Operations: []string{}}
	}

	return tools
}

func scanManuals(dir string) map[string]string {
	files := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			name := strings.TrimSuffix(e.Name(), ".md")
			files[name] = filepath.Join(dir, e.Name())
		}
	}
	return files
}

func analyzeGaps(tools map[string]*ToolInfo, manuals map[string]string, manualsDir string) *GapReport {
	report := &GapReport{
		TotalDispatchTools: len(tools),
		TotalManuals:       len(manuals),
	}

	for action, tool := range tools {
		manualPath, hasManual := manuals[action]

		if !hasManual {
			report.Missing = append(report.Missing, *tool)
			continue
		}

		tool.HasManual = true
		tool.ManualPath = manualPath

		// Read and score manual
		score := scoreManual(manualPath, tool.Operations)
		tool.ManualScore = &score

		if score.Total == 0 {
			report.Unscored = append(report.Unscored, *tool)
		} else if score.Total < 50 {
			report.LowScore = append(report.LowScore, *tool)
		} else {
			report.UpToDate = append(report.UpToDate, *tool)
		}
	}

	// Check for orphaned manuals (have file but no dispatch)
	for manualName := range manuals {
		if _, exists := tools[manualName]; !exists {
			// Orphaned - manual exists but no dispatch case
			// Could be renamed tool, skip for now
		}
	}

	// Sort each category by action name
	sort.Slice(report.Missing, func(i, j int) bool { return report.Missing[i].Action < report.Missing[j].Action })
	sort.Slice(report.LowScore, func(i, j int) bool {
		return report.LowScore[i].ManualScore.Total < report.LowScore[j].ManualScore.Total
	})
	sort.Slice(report.UpToDate, func(i, j int) bool {
		return report.UpToDate[i].ManualScore.Total > report.UpToDate[j].ManualScore.Total
	})

	return report
}

func scoreManual(path string, expectedOps []string) Score {
	content, err := os.ReadFile(path)
	if err != nil {
		return Score{}
	}

	text := string(content)

	score := Score{}

	// Count documented operations in table
	opPattern := regexp.MustCompile(`\|\s*` + "`([^`]+)" + `` + `\s*\|`)
	opMatches := opPattern.FindAllStringSubmatch(text, -1)
	docOps := make(map[string]bool)
	for _, m := range opMatches {
		docOps[m[1]] = true
	}

	// Count expected ops that are documented
	opsCovered := 0
	for _, op := range expectedOps {
		if docOps[op] {
			opsCovered++
		}
	}
	if len(expectedOps) > 0 {
		score.Operations = (opsCovered * 100) / len(expectedOps)
	} else {
		score.Operations = 100
	}

	// Count JSON examples (```json blocks)
	jsonBlocks := strings.Count(text, "```json")
	if len(expectedOps) > 0 {
		score.Examples = min(100, (jsonBlocks*100)/len(expectedOps))
	} else {
		score.Examples = 100
	}

	// Check for Parameters section
	if strings.Contains(text, "## Parameters") || strings.Contains(text, "### Parameters") {
		score.Parameters = 50 // Has section
	}

	// Check for Notes section
	if strings.Contains(text, "## Notes") || strings.Contains(text, "### Notes") {
		score.Notes = 50
	}

	// Check for config.yaml examples
	if strings.Contains(text, "config.yaml") || strings.Contains(text, "```yaml") {
		score.Config = 50
	}

	// Calculate total
	score.Total = (score.Operations*20 + score.Examples*25 + score.Parameters*20 + score.Notes*20 + score.Config*15) / 100

	return score
}

func printSummary(r *GapReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("TOOL MANUAL GAP ANALYSIS — SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("\n📊 OVERVIEW\n")
	fmt.Printf("  Dispatch tools:    %d\n", r.TotalDispatchTools)
	fmt.Printf("  Manual files:      %d\n", r.TotalManuals)
	fmt.Printf("  Coverage:          %.0f%%\n", float64(r.TotalManuals)/float64(r.TotalDispatchTools)*100)

	fmt.Printf("\n🚨 MISSING (%d)\n", len(r.Missing))
	if len(r.Missing) == 0 {
		fmt.Printf("  (none)\n")
	} else {
		for _, t := range r.Missing {
			fmt.Printf("  - %s (%d ops)\n", t.Action, len(t.Operations))
		}
	}

	fmt.Printf("\n⚠️  LOW SCORE < 50%% (%d)\n", len(r.LowScore))
	if len(r.LowScore) == 0 {
		fmt.Printf("  (none)\n")
	} else {
		for _, t := range r.LowScore {
			fmt.Printf("  - %s: %d%%\n", t.Action, t.ManualScore.Total)
		}
	}

	fmt.Printf("\n📝 UNSCORED (%d)\n", len(r.Unscored))
	if len(r.Unscored) == 0 {
		fmt.Printf("  (none)\n")
	} else {
		for _, t := range r.Unscored {
			fmt.Printf("  - %s\n", t.Action)
		}
	}

	fmt.Printf("\n✅ UP TO DATE (%d)\n", len(r.UpToDate))
	if len(r.UpToDate) == 0 {
		fmt.Printf("  (none)\n")
	} else {
		count := len(r.UpToDate)
		if count > 10 {
			fmt.Printf("  Top 10 by score:\n")
			count = 10
		}
		for i := 0; i < count; i++ {
			t := r.UpToDate[i]
			fmt.Printf("  - %s: %d%%\n", t.Action, t.ManualScore.Total)
		}
	}
	fmt.Println()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
