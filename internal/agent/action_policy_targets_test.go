package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/inventory"
)

func TestActionPolicyDockerTargetCannotBeMaskedOrExpanded(t *testing.T) {
	for _, route := range []string{"direct", "invoke", "wrapper", "name_alias", "typed_precedence", "rm_alias"} {
		t.Run(route, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			rule := config.ActionPolicyRule{Tool: "docker", Operation: "remove", Target: "container_id:protected"}
			dc.Cfg.Agent.ActionPolicy.Deny = []config.ActionPolicyRule{rule}
			dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "docker", Operation: "remove", Target: "*"}}
			args := map[string]interface{}{"operation": "remove", "container_id": "protected", "endpoint_id": "decoy", "session_id": "not-authority", "url": "https://decoy.invalid/"}
			call := ToolCall{Action: "docker", Params: args}
			switch route {
			case "invoke":
				call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "docker", "arguments": args}}
			case "wrapper":
				call = ToolCall{Action: "execute_skill", Skill: "docker", SkillArgs: args}
			case "name_alias":
				delete(args, "container_id")
				args["name"] = "protected"
			case "typed_precedence":
				call.ContainerID = "protected"
				args["container_id"] = "decoy"
			case "rm_alias":
				args["operation"] = "rm"
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if result.Status != ToolResultDenied || calls != 0 || DescribeAction(call, dc).Target != rule.Target {
				t.Fatalf("target denial masked: result=%+v target=%s calls=%d", result, DescribeAction(call, dc).Target, calls)
			}
			// The same irrelevant fields cannot manufacture a narrow permission.
			dc.Cfg.Agent.ActionPolicy.Deny = nil
			dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "docker", Operation: "remove", Target: "endpoint_id:decoy"}}
			dc.MissionID = "fixture-mission"
			if result := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); result.Status != ToolResultDenied || calls != 0 {
				t.Fatalf("irrelevant field manufactured authority: %+v", result)
			}
		})
	}
}

func TestActionPolicyFilesystemCoversEveryEffectivePath(t *testing.T) {
	dc := &DispatchContext{Cfg: &config.Config{}}
	dc.Cfg.Directories.WorkspaceDir = t.TempDir()
	for _, fixture := range []struct {
		operation string
		args      map[string]interface{}
		paths     []string
	}{
		{"write_file", map[string]interface{}{"dest": "written.txt", "endpoint_id": "decoy"}, []string{"written.txt"}},
		{"delete", map[string]interface{}{"path": "deleted.txt", "destination": "ignored.txt"}, []string{"deleted.txt"}},
		{"move", map[string]interface{}{"path": "source.txt", "dest": "target.txt"}, []string{"source.txt", "target.txt"}},
		{"move_batch", map[string]interface{}{"items": []interface{}{map[string]interface{}{"path": "a.txt", "dest": "b.txt"}, map[string]interface{}{"file_path": "c.txt", "destination": "d.txt"}}}, []string{"a.txt", "b.txt", "c.txt", "d.txt"}},
	} {
		t.Run(fixture.operation, func(t *testing.T) {
			a := DescribeAction(ToolCall{Action: "filesystem", Operation: fixture.operation, Params: fixture.args}, dc)
			if len(a.targets) != len(fixture.paths) {
				t.Fatalf("effective paths lost: %s", a.Target)
			}
			var rules []config.ActionPolicyRule
			for _, path := range fixture.paths {
				target := "file:" + actionFilePath(path, dc)
				deny := config.ActionPolicyRule{Tool: a.Tool, Operation: a.Operation, Target: target}
				if !actionDenyRuleMatches(deny, a) {
					t.Fatalf("deny missed %s in %s", target, a.Target)
				}
				rules = append(rules, deny)
			}
			if !actionAllowRulesCover(rules, a) || len(rules) > 1 && actionAllowRulesCover(rules[:1], a) {
				t.Fatal("allow did not require complete path coverage")
			}
			if !actionAllowRulesCover([]config.ActionPolicyRule{{Tool: a.Tool, Operation: a.Operation, Target: a.Target}}, a) {
				t.Fatal("copied exact target set did not cover the call")
			}
		})
	}
}

func TestActionPolicyEmailCoversToCcBccAndAttachments(t *testing.T) {
	dc := &DispatchContext{Cfg: &config.Config{}}
	dc.Cfg.Directories.WorkspaceDir = t.TempDir()
	dc.Cfg.AgentMail.InboxID = "fixture-inbox"
	call := ToolCall{Action: "agentmail_messages", Operation: "send_message", Params: map[string]interface{}{
		"to": []interface{}{"allowed@EXAMPLE.INVALID"}, "cc": []interface{}{"Copy <copy@EXAMPLE.INVALID>"}, "bcc": []interface{}{"hidden@EXAMPLE.INVALID"},
		"attachments": []interface{}{map[string]interface{}{"path": "attachment.txt"}}, "endpoint_id": "decoy",
	}}
	a := DescribeAction(call, dc)
	if !a.HighRisk || a.targetUnknown || len(a.targets) != 5 || strings.Contains(a.Target, "decoy") {
		t.Fatalf("wrong complete email action: %+v", a)
	}
	for _, recipient := range []string{"allowed", "copy", "hidden"} {
		if !actionDenyRuleMatches(config.ActionPolicyRule{Tool: a.Tool, Operation: a.Operation, Target: "to:" + recipient + "@example.invalid"}, a) {
			t.Fatalf("domain case escaped recipient denial: %s", recipient)
		}
	}
	var rules []config.ActionPolicyRule
	for _, target := range a.targets {
		rule := config.ActionPolicyRule{Tool: a.Tool, Operation: a.Operation, Target: target}
		if !actionDenyRuleMatches(rule, a) {
			t.Fatalf("deny missed an email component: %s", target)
		}
		rules = append(rules, rule)
	}
	if actionAllowRulesCover(rules[:len(rules)-1], a) || !actionAllowRulesCover(rules, a) {
		t.Fatal("one recipient/source grant authorized the entire message")
	}
	deniedSet := config.ActionPolicyRule{Tool: a.Tool, Operation: a.Operation, Target: a.Target}
	call.Params["bcc"] = []interface{}{"hidden@example.invalid", "extra@example.invalid"}
	if expanded := DescribeAction(call, dc); !actionDenyRuleMatches(deniedSet, expanded) {
		t.Fatal("adding a recipient escaped an existing composite target deny")
	}
	for _, operation := range []string{"reply_message", "reply_all_message", "send_draft"} {
		call.Operation = operation
		a = DescribeAction(call, dc)
		if !a.HighRisk || !a.targetUnknown || !actionDenyRuleMatches(config.ActionPolicyRule{Tool: a.Tool, Operation: a.Operation, Target: "to:remote-unknown@example.invalid"}, a) {
			t.Fatalf("stored remote recipients escaped conservative policy: %+v", a)
		}
	}
	// SMTP's comma-separated recipient list uses its real typed-field precedence.
	a = DescribeAction(ToolCall{Action: "send_email", To: "one@example.invalid,two@example.invalid", Params: map[string]interface{}{"to": "decoy@example.invalid", "operation": "logs"}}, dc)
	if len(a.targets) != 2 || strings.Contains(a.Target, "decoy") || a.Operation != "execute" || !a.Mutating {
		t.Fatalf("SMTP target or operation was spoofed: %+v", a)
	}
	a = DescribeAction(ToolCall{Action: "send_email", To: "Local@EXAMPLE.INVALID"}, dc)
	if a.Target != "to:Local@example.invalid" || actionAllowRulesCover([]config.ActionPolicyRule{{Tool: a.Tool, Operation: a.Operation, Target: "to:local@example.invalid"}}, a) {
		t.Fatal("recipient normalization changed local-part authority")
	}
}

func TestActionPolicyURLAndSQLUseEffectiveDecoders(t *testing.T) {
	dc := &DispatchContext{Cfg: &config.Config{}}
	a := DescribeAction(ToolCall{Action: "api_request", Method: "POST", URL: "https://service.invalid/actual?secret=private#fragment", Params: map[string]interface{}{"url": "https://decoy.invalid/", "endpoint_id": "decoy", "method": "GET"}}, dc)
	if a.Target != "url:https://service.invalid/actual" || a.Operation != "post" || !a.HighRisk || a.targetUnknown {
		t.Fatalf("API policy disagreed with the handler or leaked URL data: %+v", a)
	}
	a = DescribeAction(ToolCall{Action: "sql_query", ConnectionName: "ignored-typed", Params: map[string]interface{}{"connection_name": "effective-db", "endpoint_id": "decoy"}}, dc)
	if a.Target != "connection_name:effective-db" {
		t.Fatalf("SQL policy disagreed with decoder: %+v", a)
	}
}

func TestActionPolicyInventoryAliasesResolveStableDeviceID(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "fixture-device-id", Name: "friendly-name", Protocol: "ssh"}); err != nil {
		t.Fatal(err)
	}
	dc := &DispatchContext{Cfg: &config.Config{}, InventoryDB: db}
	for _, args := range []map[string]interface{}{{"server_id": "fixture-device-id"}, {"name": "friendly-name"}, {"id": "fixture-device-id", "endpoint_id": "decoy"}} {
		a := DescribeAction(ToolCall{Action: "execute_remote_shell", Params: args}, dc)
		if a.Target != "server_id:fixture-device-id" || a.targetUnknown || a.Operation != "execute" {
			t.Fatalf("remote target alias differed from execution: %+v", a)
		}
	}
}

func TestActionPolicyUnknownIdentityCannotEscapeConcreteDeny(t *testing.T) {
	a := DescribeAction(ToolCall{Action: "browser_automation", Operation: "click", Params: map[string]interface{}{"endpoint_id": "caller-label", "session_id": "private-session"}}, &DispatchContext{Cfg: &config.Config{}})
	if a.Target != "tool:browser_automation" || !a.targetUnknown || strings.Contains(a.Target, "session") {
		t.Fatalf("unknown target used caller authority: %+v", a)
	}
	deny := config.ActionPolicyRule{Tool: a.Tool, Operation: a.Operation, Target: "url:https://protected.invalid/"}
	if !actionDenyRuleMatches(deny, a) || actionAllowRulesCover([]config.ActionPolicyRule{{Tool: a.Tool, Operation: a.Operation, Target: "endpoint_id:caller-label"}}, a) {
		t.Fatal("unknown effective identity escaped deny or manufactured allow")
	}
	deny.ArgumentsHash = strings.Repeat("a", 64)
	if actionDenyRuleMatches(deny, a) {
		t.Fatal("unrelated exact-arguments deny blocked another action")
	}
	encoded, _ := json.Marshal(a)
	var preview map[string]interface{}
	_ = json.Unmarshal(encoded, &preview)
	if !reflect.DeepEqual(preview["target"], a.Target) {
		t.Fatal("preview lost the copyable canonical target")
	}
}

func TestActionPolicyDockerAliasesCannotEvadeContainerDeny(t *testing.T) {
	fullID := strings.Repeat("a", 64)
	otherFullID := strings.Repeat("b", 64)
	for _, fixture := range []struct {
		selector, denied string
		want             bool
	}{
		{fullID, "protected-name", true},
		{fullID, fullID[:12], true},
		{"another-name", fullID, true},
		{fullID[:12], otherFullID, true},
		{fullID, otherFullID, false},
		{fullID, fullID, true},
	} {
		a := DescribeAction(ToolCall{Action: "docker", Operation: "remove", Params: map[string]interface{}{"container_id": fixture.selector}}, &DispatchContext{Cfg: &config.Config{}})
		deny := config.ActionPolicyRule{Tool: "docker", Operation: "remove", Target: "container_id:" + fixture.denied}
		if got := actionDenyRuleMatches(deny, a); got != fixture.want {
			t.Fatalf("selector %q vs deny %q: %v, want %v", fixture.selector, fixture.denied, got, fixture.want)
		}
	}
}

func TestActionPolicyCommandOperationsRequireExplicitAuthority(t *testing.T) {
	for _, fixture := range []struct {
		name, tool, operation, canonical string
		args                             map[string]interface{}
	}{
		{"docker_exec", "docker", "exec", "exec", map[string]interface{}{"container_id": strings.Repeat("a", 64), "command": "echo fixture"}},
		{"ansible_shell", "ansible", "adhoc", "adhoc", map[string]interface{}{"module": "shell", "hostname": "fixture-host", "command": "echo fixture"}},
		{"ansible_command_alias", "ansible", "command", "adhoc", map[string]interface{}{"module": "ansible.builtin.command", "hostname": "fixture-host", "command": "echo fixture"}},
		{"ansible_module_alias", "ansible", "run_module", "adhoc", map[string]interface{}{"module": "shell", "hostname": "fixture-host", "command": "echo fixture"}},
		{"ansible_playbook_alias", "ansible", "run_playbook", "playbook", map[string]interface{}{"name": "fixture.yml"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.Ansible.Enabled = true
			call := ToolCall{Action: fixture.tool, Operation: fixture.operation, Params: fixture.args}
			action := DescribeAction(call, dc)
			if !action.HighRisk || !action.Mutating || action.Operation != fixture.canonical {
				t.Fatalf("command operation escaped risk classification: %+v", action)
			}
			if result := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); result.Status != ToolResultDeferred || calls != 0 {
				t.Fatalf("activation alone executed interactive command: %+v calls=%d", result, calls)
			}
			CancelActionApprovals(dc.SessionID)
			dc.MissionID = "fixture-mission"
			if result := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); result.Status != ToolResultDenied || calls != 0 {
				t.Fatalf("activation alone executed unattended command: %+v calls=%d", result, calls)
			}
			action = DescribeAction(call, dc)
			dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: action.Tool, Operation: action.Operation, Target: "*", ArgumentsHash: action.ArgumentsHash}}
			if result := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); result.Status != ToolResultSuccess || calls != 1 {
				t.Fatalf("exact reviewed command was not admitted: %+v calls=%d", result, calls)
			}
		})
	}
	for _, tool := range []string{"execute_python", "execute_shell"} {
		if a := DescribeAction(ToolCall{Action: tool}, &DispatchContext{Cfg: &config.Config{}}); a.HighRisk {
			t.Fatalf("ordinary isolated %s was made high-risk", tool)
		}
	}
}
