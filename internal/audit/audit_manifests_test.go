package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestToolPermissionMatrixCoversHighRiskBuiltins(t *testing.T) {
	t.Parallel()

	highRisk := []string{
		"api_request", "call_webhook", "chromecast", "docker", "execute_python",
		"execute_shell", "execute_sudo", "file_editor", "filesystem", "home_assistant",
		"homepage", "invasion_control", "json_editor", "manage_outgoing_webhooks",
		"meshcentral", "netlify", "remote_execution", "secrets_vault",
		"transfer_remote_file", "truenas", "vercel", "video_download", "xml_editor", "yaml_editor",
	}
	matrix := ToolPermissionMatrix()
	byName := map[string]ToolPermission{}
	for _, entry := range matrix {
		if entry.Name == "" {
			t.Fatal("tool permission matrix contains an unnamed entry")
		}
		if _, exists := byName[entry.Name]; exists {
			t.Fatalf("duplicate tool permission entry for %q", entry.Name)
		}
		if entry.ConfigGate == "" {
			t.Fatalf("%s has no config gate", entry.Name)
		}
		if len(entry.Capabilities) == 0 {
			t.Fatalf("%s has no capabilities", entry.Name)
		}
		byName[entry.Name] = entry
	}

	for _, name := range highRisk {
		entry, ok := byName[name]
		if !ok {
			t.Fatalf("high-risk tool %q is missing from ToolPermissionMatrix", name)
		}
		if hasAnyCapability(entry, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityExecute) && entry.ConfigGate == "" {
			t.Fatalf("high-risk tool %q has mutating/executing capability without a gate", name)
		}
	}
}

func TestHomeLabIntegrationMatrixRequiresGuardsForMutatingLocalIntegrations(t *testing.T) {
	t.Parallel()

	entries := HomeLabIntegrationMatrix()
	if len(entries) < 8 {
		t.Fatalf("home-lab matrix unexpectedly small: %d entries", len(entries))
	}
	for _, entry := range entries {
		if entry.Name == "" || entry.ConfigGate == "" {
			t.Fatalf("invalid home-lab matrix entry: %+v", entry)
		}
		if entry.EnabledByDefault {
			t.Fatalf("%s must not be enabled by default", entry.Name)
		}
		if entry.AllowsLocalNetwork && entry.CredentialSource == "" {
			t.Fatalf("%s allows local network access without credential/source classification", entry.Name)
		}
		if entry.HasWriteOrDelete && entry.ReadOnlyGate == "" && entry.DestructiveGuardConfig == "" {
			t.Fatalf("%s mutates local integration state without read-only or destructive guard config", entry.Name)
		}
		if entry.HasWriteOrDelete && !entry.HasTestEndpoint {
			t.Fatalf("%s mutates integration state but has no test endpoint classification", entry.Name)
		}
	}
}

func TestRouteContractManifestCoversRegisteredServerRoutes(t *testing.T) {
	t.Parallel()

	contracts := RouteContractManifest()
	if len(contracts) == 0 {
		t.Fatal("route contract manifest is empty")
	}
	for _, c := range contracts {
		if c.Pattern == "" || c.Auth == "" || c.Category == "" || len(c.Methods) == 0 {
			t.Fatalf("invalid route contract: %+v", c)
		}
		if routeCanMutate(c.Methods) && strings.HasPrefix(c.Auth, "public") && c.Category != "setup" && c.Category != "auth" {
			t.Fatalf("mutating route %s is public without a bootstrap exception", c.Pattern)
		}
	}

	routes := extractLiteralRoutes(t, repoPath("internal/server"))
	for _, route := range routes {
		if !isRouteCovered(route, contracts) {
			t.Fatalf("registered route %q is missing from RouteContractManifest", route)
		}
	}
}

func TestNetworkClientInventoryCoversProductionHTTPClients(t *testing.T) {
	t.Parallel()

	entries := NetworkClientInventory()
	for _, entry := range entries {
		if entry.Path == "" || entry.Classification == "" {
			t.Fatalf("invalid network client inventory entry: %+v", entry)
		}
	}

	clientPattern := regexp.MustCompile(`(&http\.Client\s*\{|http\.DefaultClient|http\.(Get|Post|Head)\s*\()`)
	var uncovered []string
	walkGoFiles(t, repoPath("."), func(path string, content string) {
		if strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/disposable/") {
			return
		}
		if !clientPattern.MatchString(content) {
			return
		}
		if !isNetworkPathCovered(path, entries) {
			uncovered = append(uncovered, path)
		}
	})
	if len(uncovered) > 0 {
		t.Fatalf("production HTTP client use is missing from NetworkClientInventory:\n%s", strings.Join(uncovered, "\n"))
	}
}

func TestDBMigrationManifestTracksRuntimeSQLiteDomains(t *testing.T) {
	t.Parallel()

	entries := DBMigrationManifest()
	if len(entries) == 0 {
		t.Fatal("DB migration manifest is empty")
	}

	var unversionedRuntime []string
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Domain == "" || entry.PackagePath == "" {
			t.Fatalf("invalid DB migration manifest entry: %+v", entry)
		}
		if seen[entry.Domain] {
			t.Fatalf("duplicate DB migration domain %q", entry.Domain)
		}
		seen[entry.Domain] = true
		if entry.OwnsRuntimeData && !entry.SchemaVersioned {
			unversionedRuntime = append(unversionedRuntime, entry.Domain)
		}
	}
	if len(unversionedRuntime) == 0 {
		t.Fatal("DB migration manifest should keep tracking remaining unversioned runtime domains")
	}
	if !seen["planner"] {
		t.Fatal("planner migration domain with legacy fixture tests is missing")
	}
}

func TestToolManualFilenamesAreKnownOrAllowlisted(t *testing.T) {
	t.Parallel()

	allowlist := map[string]string{
		"address_book":          "legacy/manual alias for contacts",
		"archive":               "archive operations are routed through filesystem/tooling docs",
		"brave_search":          "Python skill/manual",
		"budget_status":         "legacy/manual alias",
		"certificate_manager":   "server/config manual",
		"code_analysis":         "Python skill/manual",
		"core_memory":           "manual alias for manage_memory",
		"detect_file_type":      "Python skill/manual",
		"discord":               "messaging integration manual",
		"email":                 "messaging integration manual",
		"execute_surgery":       "maintenance/lifeboat manual",
		"exit_lifeboat":         "lifeboat manual",
		"fritzbox_smarthome":    "manual alias for FritzBox smart home feature",
		"homeassistant":         "legacy alias for home_assistant",
		"homepage_local_server": "homepage sub-feature manual",
		"image_processing":      "Python skill/manual",
		"initiate_handover":     "lifeboat manual",
		"invoke_tool":           "meta-tool parser action",
		"log_analyzer":          "skill template/manual",
		"mac_lookup":            "server endpoint/manual",
		"mcp":                   "MCP runtime/manual",
		"mqtt":                  "messaging integration manual",
		"optimize_memory":       "memory maintenance manual alias",
		"paperless":             "document integration manual",
		"mdns":                  "network discovery manual",
		"pdf_extractor":         "Python skill/manual",
		"port_scanner":          "network scan manual alias",
		"remote_control":        "remote feature manual alias",
		"sandbox":               "sandbox feature manual",
		"send_notification":     "notification manual alias",
		"service_manager":       "system service manual",
		"site_crawler":          "web scraper manual alias",
		"site_monitor":          "tool/manual stored outside native schema",
		"skill_manager":         "UI/server feature manual",
		"skill_manifest_spec":   "documentation schema manual",
		"skill_templates":       "skill template manual",
		"skills_engine":         "skills subsystem manual",
		"smart_memory":          "manual alias for context_memory",
		"ssh_key_manager":       "inventory/credential feature manual",
		"toml_editor":           "manual alias for config file editing",
		"telnyx":                "messaging integration manual",
		"tts_minimax":           "provider-specific TTS manual",
		"web_performance_audit": "homepage/lighthouse manual alias",
		"webdav":                "Python skill/manual",
		"whois_lookup":          "Python skill/manual",
		"yepapi":                "YepAPI family manual",
	}
	nativeNames := nativeToolNamesFromSource(t)
	for _, name := range manualNames(t) {
		if nativeNames[name] || allowlist[name] != "" {
			continue
		}
		t.Fatalf("tool manual %q is not a native tool and is not documented in the allowlist", name)
	}
}

func hasAnyCapability(entry ToolPermission, caps ...Capability) bool {
	for _, have := range entry.Capabilities {
		if slices.Contains(caps, have) {
			return true
		}
	}
	return false
}

func routeCanMutate(methods []string) bool {
	for _, method := range methods {
		switch method {
		case "POST", "PUT", "PATCH", "DELETE":
			return true
		}
	}
	return false
}

func isRouteCovered(route string, contracts []RouteContract) bool {
	for _, c := range contracts {
		if c.Pattern == route {
			return true
		}
		if strings.HasSuffix(c.Pattern, "/") && strings.HasPrefix(route, c.Pattern) {
			return true
		}
		if strings.HasPrefix(route, c.Pattern+"/") {
			return true
		}
	}
	return false
}

func isNetworkPathCovered(path string, entries []NetworkClientUse) bool {
	for _, entry := range entries {
		if strings.HasPrefix(path, filepath.ToSlash(entry.Path)) {
			return true
		}
	}
	return false
}

func extractLiteralRoutes(t *testing.T, root string) []string {
	t.Helper()
	routeRe := regexp.MustCompile(`mux\.Handle(?:Func)?\("([^"]+)"`)
	routes := map[string]bool{}
	walkGoFiles(t, root, func(path string, content string) {
		for _, match := range routeRe.FindAllStringSubmatch(content, -1) {
			routes[match[1]] = true
		}
	})
	out := make([]string, 0, len(routes))
	for route := range routes {
		out = append(out, route)
	}
	slices.Sort(out)
	return out
}

func manualNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(repoPath("prompts/tools_manuals"))
	if err != nil {
		t.Fatalf("read tool manuals: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}
	slices.Sort(names)
	return names
}

func nativeToolNamesFromSource(t *testing.T) map[string]bool {
	t.Helper()
	toolRe := regexp.MustCompile(`tool\("([a-z][a-z0-9_]*)"`)
	names := map[string]bool{}
	walkGoFiles(t, repoPath("internal/agent"), func(path string, content string) {
		if !strings.Contains(path, "native_tools") {
			return
		}
		for _, match := range toolRe.FindAllStringSubmatch(content, -1) {
			names[match[1]] = true
		}
	})
	return names
}

func walkGoFiles(t *testing.T, root string, visit func(path string, content string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		readPath := path
		path = filepath.ToSlash(path)
		if rel, relErr := filepath.Rel(repoPath("."), path); relErr == nil {
			path = filepath.ToSlash(rel)
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".kilo", "bin", "deploy", "disposable", "node_modules", "reports":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		raw, readErr := os.ReadFile(readPath)
		if readErr != nil {
			return nil
		}
		visit(path, string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func repoPath(parts ...string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Clean(filepath.Join(parts...))
	}
	elems := append([]string{filepath.Dir(file), "..", ".."}, parts...)
	return filepath.Clean(filepath.Join(elems...))
}
