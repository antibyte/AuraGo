package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ── V5: AWS Compressor Tests ──────────────────────────────────────────

func TestCompressAwsTable_Small(t *testing.T) {
	output := "INSTANCE_ID   TYPE       STATE\ni-12345       t3.micro   running"
	result := compressAwsTable(output)
	if !strings.Contains(result, "i-12345") {
		t.Error("small AWS table should be unchanged")
	}
}

func TestCompressAwsTable_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("INSTANCE_ID     TYPE       STATE       NAME\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("i-%08d     t3.micro   running     app-%d\n", i, i))
	}
	sb.WriteString("i-99999999     t3.micro   stopped     legacy\n")
	result := compressAwsTable(sb.String())
	if !strings.Contains(result, "total rows") {
		t.Error("should contain row summary")
	}
	if !strings.Contains(result, "stopped") {
		t.Error("should include stopped/error rows")
	}
}

func TestCompressAwsTable_JSON(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": null,`, i) + "\n")
	}
	sb.WriteString(`  "name": "test"` + "\n")
	sb.WriteString("}")
	result := compressAwsTable(sb.String())
	// Should use JSON compaction
	if strings.Contains(result, ": null") {
		t.Error("should compact JSON and remove null fields")
	}
}

func TestCompressAws_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"aws ec2 describe-instances", "aws-ec2"},
		{"aws s3 ls", "aws-s3"},
		{"aws lambda list-functions", "aws-lambda"},
		{"aws cloudformation describe-stacks", "aws-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V5: Ansible Compressor Tests ──────────────────────────────────────

func TestCompressAnsible_Small(t *testing.T) {
	output := "PLAY [webservers] **********************************************************\nTASK [Gathering Facts] *****************************************************\nok: [host1]\nPLAY RECAP *****************************************************************\nhost1 : ok=2  changed=0  unreachable=0  failed=0"
	result := compressAnsible(output)
	// Small output (<=10 lines) should pass through
	if !strings.Contains(result, "PLAY") {
		t.Error("small ansible output should be preserved")
	}
}

func TestCompressAnsible_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("PLAY [webservers] **********************************************************\n")
	sb.WriteString("TASK [Gathering Facts] *****************************************************\n")
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("ok: [host%d]\n", i))
	}
	sb.WriteString("TASK [Install nginx] *******************************************************\n")
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("changed: [host%d]\n", i))
	}
	sb.WriteString("TASK [Start nginx] *********************************************************\n")
	sb.WriteString("fatal: [host5]: UNREACHABLE! => {\"changed\": false, \"msg\": \"Connection refused\"}\n")
	for i := 0; i < 14; i++ {
		if i != 5 {
			sb.WriteString(fmt.Sprintf("changed: [host%d]\n", i))
		}
	}
	sb.WriteString("PLAY RECAP *****************************************************************\n")
	sb.WriteString("host0 : ok=3  changed=2  unreachable=0    failed=0\n")
	sb.WriteString("host5 : ok=2  changed=1  unreachable=1    failed=1\n")
	result := compressAnsible(sb.String())
	if !strings.Contains(result, "PLAY") {
		t.Error("should contain PLAY headers")
	}
	if !strings.Contains(result, "fatal") {
		t.Error("should contain fatal/error lines")
	}
	if !strings.Contains(result, "PLAY RECAP") {
		t.Error("should contain PLAY RECAP section")
	}
	if !strings.Contains(result, "Summary") {
		t.Error("should contain summary counts")
	}
}

func TestCompressAnsible_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"ansible all -m ping", "ansible"},
		{"ansible-playbook site.yml", "ansible"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ─── V7: Home Assistant Compressor Tests ─────────────────────────────────────

// buildHAStatesJSON builds a HA get_states JSON envelope for testing.
func buildHAStatesJSON(entities []map[string]interface{}) string {
	envelope := map[string]interface{}{
		"status": "success",
		"count":  len(entities),
		"states": entities,
	}
	data, _ := json.Marshal(envelope)
	return string(data)
}

func TestCompressHAGetStates_Large(t *testing.T) {
	var entities []map[string]interface{}
	// Add 50 lights (30 on, 20 off)
	for i := 0; i < 30; i++ {
		entities = append(entities, map[string]interface{}{
			"entity_id":     fmt.Sprintf("light.light_%d", i),
			"state":         "on",
			"friendly_name": fmt.Sprintf("Light %d", i),
		})
	}
	for i := 30; i < 50; i++ {
		entities = append(entities, map[string]interface{}{
			"entity_id":     fmt.Sprintf("light.light_%d", i),
			"state":         "off",
			"friendly_name": fmt.Sprintf("Light %d", i),
		})
	}
	// Add 20 sensors (all "measuring")
	for i := 0; i < 20; i++ {
		entities = append(entities, map[string]interface{}{
			"entity_id":     fmt.Sprintf("sensor.temp_%d", i),
			"state":         fmt.Sprintf("%.1f", 20.0+float64(i)*0.5),
			"friendly_name": fmt.Sprintf("Temp Sensor %d", i),
		})
	}
	// Add 2 unavailable entities
	entities = append(entities, map[string]interface{}{
		"entity_id":     "switch.garage",
		"state":         "unavailable",
		"friendly_name": "Garage Switch",
	})
	entities = append(entities, map[string]interface{}{
		"entity_id":     "sensor.old_sensor",
		"state":         "unknown",
		"friendly_name": "Old Sensor",
	})

	output := buildHAStatesJSON(entities)
	result, filter := compressHAOutput(output)

	if filter != "ha-states" {
		t.Errorf("expected ha-states filter, got %q", filter)
	}
	if !strings.Contains(result, "HA States: 72 entities") {
		t.Errorf("expected entity count, got: %s", result[:min(200, len(result))])
	}
	if !strings.Contains(result, "3 domains") {
		t.Errorf("expected domain count, got: %s", result[:min(200, len(result))])
	}
	if !strings.Contains(result, "light:") {
		t.Error("expected light domain in summary")
	}
	if !strings.Contains(result, "sensor:") {
		t.Error("expected sensor domain in summary")
	}
	if !strings.Contains(result, "30 on") {
		t.Error("expected 30 lights on")
	}
	if !strings.Contains(result, "20 off") {
		t.Error("expected 20 lights off")
	}
	if !strings.Contains(result, "Unavailable entities (2)") {
		t.Error("expected 2 unavailable entities")
	}
	if !strings.Contains(result, "Garage Switch") {
		t.Error("expected Garage Switch in unavailable list")
	}
	// Verify compression ratio
	if len(result) >= len(output) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(output))
	}
}

func TestCompressHAGetStates_Empty(t *testing.T) {
	output := buildHAStatesJSON(nil)
	result, filter := compressHAOutput(output)

	if filter != "ha-states" {
		t.Errorf("expected ha-states filter, got %q", filter)
	}
	if !strings.Contains(result, "0 entities") {
		t.Errorf("expected 0 entities, got: %s", result)
	}
}

func TestCompressHAGetState_Compact(t *testing.T) {
	envelope := map[string]interface{}{
		"status": "success",
		"entity": map[string]interface{}{
			"entity_id": "climate.living_room",
			"state":     "heat",
			"attributes": map[string]interface{}{
				"friendly_name":       "Living Room Climate",
				"temperature":         22.5,
				"current_temperature": 21.0,
				"hvac_mode":           "heat",
				"hvac_action":         "heating",
				"target_temp_high":    25.0,
				"target_temp_low":     18.0,
				"preset_mode":         "comfort",
				"min_temp":            5.0,
				"max_temp":            35.0,
				"some_internal_attr":  "should_be_removed",
				"another_useless":     42,
			},
			"last_changed": "2024-01-14T15:20:00.000000+00:00",
			"last_updated": "2024-01-14T15:25:00.000000+00:00",
			"context": map[string]interface{}{
				"id":        "abc123",
				"parent_id": nil,
				"user_id":   nil,
			},
		},
	}
	data, _ := json.Marshal(envelope)
	output := string(data)

	result, filter := compressHAOutput(output)

	if filter != "ha-state" {
		t.Errorf("expected ha-state filter, got %q", filter)
	}
	if !strings.Contains(result, "Entity: climate.living_room") {
		t.Error("expected entity_id")
	}
	if !strings.Contains(result, "State: heat") {
		t.Error("expected state")
	}
	if !strings.Contains(result, "temperature: 22.5") {
		t.Error("expected temperature attribute")
	}
	if !strings.Contains(result, "hvac_mode: heat") {
		t.Error("expected hvac_mode attribute")
	}
	if strings.Contains(result, "some_internal_attr") {
		t.Error("internal attribute should be filtered out")
	}
	if strings.Contains(result, "another_useless") {
		t.Error("useless attribute should be filtered out")
	}
	if !strings.Contains(result, "attributes omitted") {
		t.Error("expected omitted count")
	}
	if !strings.Contains(result, "Last changed:") {
		t.Error("expected last_changed")
	}
	if strings.Contains(result, "context") {
		t.Error("context should be removed")
	}
}

func TestCompressHACallService(t *testing.T) {
	envelope := map[string]interface{}{
		"status":            "success",
		"service":           "light.turn_on",
		"affected_entities": []string{"light.living_room", "light.bedroom", "light.kitchen"},
		"count":             3,
	}
	data, _ := json.Marshal(envelope)

	result, filter := compressHAOutput(string(data))

	if filter != "ha-call-service" {
		t.Errorf("expected ha-call-service filter, got %q", filter)
	}
	if !strings.Contains(result, "✓ Service light.turn_on called successfully") {
		t.Error("expected success message")
	}
	if !strings.Contains(result, "Affected entities (3)") {
		t.Error("expected affected entities count")
	}
	if !strings.Contains(result, "light.living_room") {
		t.Error("expected entity in list")
	}
}

func TestCompressHAListServices_Large(t *testing.T) {
	type svcEntry struct {
		Domain   string   `json:"domain"`
		Services []string `json:"services"`
	}

	// Create domains with varying service counts
	var services []svcEntry
	for i := 0; i < 20; i++ {
		var svcNames []string
		numSvcs := 5 + i*2 // 5,7,9,...43 services per domain
		for j := 0; j < numSvcs; j++ {
			svcNames = append(svcNames, fmt.Sprintf("service_%d", j))
		}
		services = append(services, svcEntry{
			Domain:   fmt.Sprintf("domain_%02d", i),
			Services: svcNames,
		})
	}

	envelope := map[string]interface{}{
		"status":   "success",
		"count":    len(services),
		"services": services,
	}
	data, _ := json.Marshal(envelope)

	result, filter := compressHAOutput(string(data))

	if filter != "ha-list-services" {
		t.Errorf("expected ha-list-services filter, got %q", filter)
	}
	if !strings.Contains(result, "HA Services: 20 domains") {
		t.Error("expected domain count")
	}
	// Domains with >15 services should show "+N more"
	if !strings.Contains(result, " more") {
		t.Error("expected truncation for domains with >15 services")
	}
	// Verify compression
	if len(result) >= len(data) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(data))
	}
}

func TestCompress_HA_Routing(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, APICompression: true}

	// Test that homeassistant tool name routes to HA compressor
	states := buildHAStatesJSON([]map[string]interface{}{
		{"entity_id": "light.test", "state": "on", "friendly_name": "Test Light"},
	})
	// Pad to exceed MinChars
	for i := 0; i < 30; i++ {
		states = strings.Replace(states, "]", fmt.Sprintf(",{\"entity_id\":\"sensor.p%d\",\"state\":\"%d\",\"friendly_name\":\"S%d\"}]", i, i, i), 1)
	}

	tests := []struct {
		toolName string
		want     string
	}{
		{"homeassistant", "ha-states"},
		{"home_assistant", "ha-states"},
	}
	for _, tt := range tests {
		_, stats := Compress(tt.toolName, "", states, cfg)
		if stats.FilterUsed != tt.want {
			t.Errorf("Compress(%q) filter = %q, want %q", tt.toolName, stats.FilterUsed, tt.want)
		}
	}
}

func TestCompress_HA_SubToggleOff(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, APICompression: false}

	states := buildHAStatesJSON([]map[string]interface{}{
		{"entity_id": "light.test", "state": "on"},
	})
	// Pad to exceed MinChars
	for i := 0; i < 30; i++ {
		states += fmt.Sprintf(`{"entity_id":"sensor.p%d","state":"%d"}`, i, i)
	}

	_, stats := Compress("homeassistant", "", states, cfg)
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter with APICompression=false, got %q", stats.FilterUsed)
	}
}
func TestCompressGitHub_Repos(t *testing.T) {
	type repo struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		Language    string `json:"language"`
		UpdatedAt   string `json:"updated_at"`
		HTMLURL     string `json:"html_url"`
		CloneURL    string `json:"clone_url"`
	}

	repos := make([]repo, 25)
	for i := 0; i < 25; i++ {
		repos[i] = repo{
			Name:        fmt.Sprintf("repo%d", i),
			FullName:    fmt.Sprintf("user/repo%d", i),
			Description: fmt.Sprintf("Repository number %d with a somewhat long description", i),
			Private:     i < 3,
			Language:    "Go",
			UpdatedAt:   "2024-01-15T10:30:00Z",
			HTMLURL:     fmt.Sprintf("https://github.com/user/repo%d", i),
			CloneURL:    fmt.Sprintf("https://github.com/user/repo%d.git", i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(repos),
		"repos":  repos,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-repos" {
		t.Errorf("expected filter github-repos, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "25 repos") {
		t.Error("expected repo count in output")
	}
	if !strings.Contains(result, "3 private") {
		t.Error("expected private count")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation indicator")
	}
	if stats.Ratio >= 1.0 {
		t.Errorf("expected compression, got ratio %.2f", stats.Ratio)
	}
}

func TestCompressGitHub_Issues(t *testing.T) {
	type issue struct {
		Number    int      `json:"number"`
		Title     string   `json:"title"`
		State     string   `json:"state"`
		User      string   `json:"user"`
		Labels    []string `json:"labels"`
		CreatedAt string   `json:"created_at"`
		HTMLURL   string   `json:"html_url"`
	}

	issues := make([]issue, 30)
	for i := 0; i < 30; i++ {
		issues[i] = issue{
			Number:    100 + i,
			Title:     fmt.Sprintf("Bug: something broke in module %d", i),
			State:     "open",
			User:      "developer",
			Labels:    []string{"bug", "priority-high"},
			CreatedAt: "2024-01-15T10:30:00Z",
			HTMLURL:   fmt.Sprintf("https://github.com/user/repo/issues/%d", 100+i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(issues),
		"issues": issues,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-issues" {
		t.Errorf("expected filter github-issues, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "30 issues") {
		t.Error("expected issue count")
	}
	if !strings.Contains(result, "#100") {
		t.Error("expected issue number")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation")
	}
}

func TestCompressGitHub_PRs(t *testing.T) {
	type pr struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		User      string `json:"user"`
		Head      string `json:"head"`
		Base      string `json:"base"`
		CreatedAt string `json:"created_at"`
	}

	prs := []pr{
		{Number: 42, Title: "Fix login bug", State: "open", User: "dev1", Head: "fix/login", Base: "main"},
		{Number: 41, Title: "Add feature X", State: "closed", User: "dev2", Head: "feat/x", Base: "develop"},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"count":         len(prs),
		"pull_requests": prs,
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("github", output)

	if filter != "github-prs" {
		t.Errorf("expected filter github-prs, got %s", filter)
	}
	if !strings.Contains(result, "2 PRs") {
		t.Error("expected PR count")
	}
	if !strings.Contains(result, "fix/login → main") {
		t.Error("expected branch info")
	}
}

func TestCompressGitHub_Commits(t *testing.T) {
	type commit struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}

	commits := make([]commit, 30)
	for i := 0; i < 30; i++ {
		commits[i] = commit{
			SHA:     fmt.Sprintf("abc%d", i),
			Message: fmt.Sprintf("Fix issue #%d with a detailed commit message", i),
			Author:  "Developer",
			Date:    "2024-01-15T10:30:00Z",
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(commits),
		"commits": commits,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-commits" {
		t.Errorf("expected filter github-commits, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "30 commits") {
		t.Error("expected commit count")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation")
	}
}

func TestCompressGitHub_WorkflowRuns(t *testing.T) {
	type run struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Branch     string `json:"branch"`
		CreatedAt  string `json:"created_at"`
	}

	runs := []run{
		{ID: 123, Name: "CI", Status: "completed", Conclusion: "success", Branch: "main", CreatedAt: "2024-01-15T10:30:00Z"},
		{ID: 122, Name: "Deploy", Status: "completed", Conclusion: "failure", Branch: "develop", CreatedAt: "2024-01-14T08:00:00Z"},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(runs),
		"runs":   runs,
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("github", output)

	if filter != "github-runs" {
		t.Errorf("expected filter github-runs, got %s", filter)
	}
	if !strings.Contains(result, "2 workflow runs") {
		t.Error("expected workflow run count")
	}
	if !strings.Contains(result, "completed/success") {
		t.Error("expected status/conclusion")
	}
	if !strings.Contains(result, "completed/failure") {
		t.Error("expected failure run")
	}
}

func TestCompressGitHub_Error(t *testing.T) {
	output := `Tool Output: {"status":"error","message":"GitHub API error (HTTP 401): Bad credentials"}`

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("github", output)

	if filter != "github-error" {
		t.Errorf("expected filter github-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompressGitHub_Branches(t *testing.T) {
	type branch struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}

	branches := make([]branch, 35)
	for i := 0; i < 35; i++ {
		branches[i] = branch{
			Name:      fmt.Sprintf("feature/branch-%d", i),
			Protected: i == 0,
		}
	}
	branches[0] = branch{Name: "main", Protected: true}

	data, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"branches": branches,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-branches" {
		t.Errorf("expected filter github-branches, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "branches") {
		t.Error("expected branches header")
	}
	if !strings.Contains(result, "[protected]") {
		t.Error("expected protected marker")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation")
	}
}
func TestCompressSQL_QueryResult(t *testing.T) {
	rows := make([]map[string]interface{}, 30)
	for i := 0; i < 30; i++ {
		rows[i] = map[string]interface{}{
			"id":    i + 1,
			"name":  fmt.Sprintf("user%d", i),
			"email": fmt.Sprintf("user%d@example.com", i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"result": rows,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("sql_query", "", output, cfg)

	if stats.FilterUsed != "sql-query" {
		t.Errorf("expected filter sql-query, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "30 rows") {
		t.Error("expected row count")
	}
	if !strings.Contains(result, "3 cols") {
		t.Error("expected column count")
	}
	if !strings.Contains(result, "+ 10 more rows") {
		t.Error("expected truncation")
	}
}

func TestCompressSQL_QueryEmpty(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"result": []map[string]interface{}{},
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-query" {
		t.Errorf("expected filter sql-query, got %s", filter)
	}
	if !strings.Contains(result, "0 rows") {
		t.Errorf("expected '0 rows returned', got: %s", result)
	}
}

func TestCompressSQL_Describe(t *testing.T) {
	columns := []map[string]interface{}{
		{"name": "id", "type": "INTEGER", "notnull": true, "pk": true},
		{"name": "name", "type": "TEXT", "notnull": true, "pk": false},
		{"name": "email", "type": "TEXT", "notnull": false, "pk": false, "unique": true},
		{"name": "created_at", "type": "TIMESTAMP", "notnull": false, "pk": false, "default_value": "NOW()"},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"table":   "users",
		"columns": columns,
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-describe" {
		t.Errorf("expected filter sql-describe, got %s", filter)
	}
	if !strings.Contains(result, "Table users") {
		t.Error("expected table name")
	}
	if !strings.Contains(result, "4 columns") {
		t.Error("expected column count")
	}
	if !strings.Contains(result, "PK") {
		t.Error("expected PK marker")
	}
	if !strings.Contains(result, "NOT NULL") {
		t.Error("expected NOT NULL marker")
	}
	if !strings.Contains(result, "UNIQUE") {
		t.Error("expected UNIQUE marker")
	}
	if !strings.Contains(result, "DEFAULT") {
		t.Error("expected DEFAULT marker")
	}
}

func TestCompressSQL_ListTables(t *testing.T) {
	tables := make([]string, 60)
	for i := 0; i < 60; i++ {
		tables[i] = fmt.Sprintf("table_%d", i)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"tables": tables,
		"count":  len(tables),
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-list-tables" {
		t.Errorf("expected filter sql-list-tables, got %s", filter)
	}
	if !strings.Contains(result, "60 tables") {
		t.Error("expected table count")
	}
	if !strings.Contains(result, "+ 10 more") {
		t.Error("expected truncation")
	}
}

func TestCompressSQL_Error(t *testing.T) {
	output := `Tool Output: {"status":"error","message":"'sql_query' is required for query operation"}`

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-error" {
		t.Errorf("expected filter sql-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompress_APIRouting_GitHubAndSQL(t *testing.T) {
	// Verify github and sql_query are recognized as API tools
	if !isAPITool("github") {
		t.Error("expected 'github' to be an API tool")
	}
	if !isAPITool("sql_query") {
		t.Error("expected 'sql_query' to be an API tool")
	}
	if !isGitHubTool("github") {
		t.Error("expected isGitHubTool('github') = true")
	}
	if !isSQLTool("sql_query") {
		t.Error("expected isSQLTool('sql_query') = true")
	}
}

func TestCompressSQL_QueryResult_LargeValues(t *testing.T) {
	rows := make([]map[string]interface{}, 5)
	for i := 0; i < 5; i++ {
		rows[i] = map[string]interface{}{
			"id":          i + 1,
			"description": strings.Repeat("x", 100), // long value
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"result": rows,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("sql_query", "", output, cfg)

	if stats.FilterUsed != "sql-query" {
		t.Errorf("expected filter sql-query, got %s", stats.FilterUsed)
	}
	// Long values should be truncated
	if strings.Contains(result, strings.Repeat("x", 100)) {
		t.Error("expected long values to be truncated")
	}
	if !strings.Contains(result, "...") {
		t.Error("expected truncation indicator")
	}
}
