package agent

import (
	"strings"
	"testing"
)

func TestRoleToolRestrictionDeniesPersistenceToolsToCoAgents(t *testing.T) {
	main := &DispatchContext{SessionID: "default"}
	local := &DispatchContext{SessionID: "coagent-1", IsCoAgent: true, MessageSource: "co_agent"}
	a2a := &DispatchContext{SessionID: "a2a-task-1", IsCoAgent: true, MessageSource: "a2a"}
	for _, action := range []string{"manage_webhooks", "save_tool", "create_skill_from_template", "set_skill_documentation", "manage_schedule", "schedule_cron", "cron_scheduler"} {
		call := ToolCall{Action: action, Operation: "create"}
		for name, dc := range map[string]*DispatchContext{"co-agent": local, "a2a": a2a} {
			if got := roleToolRestriction(call, dc); !strings.Contains(got, "policy_denied") {
				t.Errorf("%s run may call %s: %q", name, action, got)
			}
		}
		if got := roleToolRestriction(call, main); got != "" {
			t.Errorf("main agent denied %s: %q", action, got)
		}
	}
	// Follow-ups, wait events and mission changes were already denied; pin them
	// next to the new entries.
	for _, call := range []ToolCall{{Action: "follow_up"}, {Action: "wait_for_event"}, {Action: "manage_missions", Operation: "create"}, {Action: "manage_missions", Operation: "run_now"}} {
		if got := roleToolRestriction(call, a2a); !strings.Contains(got, "policy_denied") {
			t.Errorf("a2a run may call %s %s: %q", call.Action, call.Operation, got)
		}
	}
	if got := roleToolRestriction(ToolCall{Action: "manage_missions", Operation: "list"}, a2a); got != "" {
		t.Errorf("a2a run denied mission list: %q", got)
	}
}

func TestRoleToolRestrictionKeepsFileWritesForLocalCoAgentsButNotA2A(t *testing.T) {
	local := &DispatchContext{SessionID: "coagent-1", IsCoAgent: true, MessageSource: "co_agent", CoAgentSpecialist: "coder"}
	a2a := &DispatchContext{SessionID: "a2a-task-1", IsCoAgent: true, MessageSource: "a2a"}
	// A session prefix alone identifies a peer run whose source was lost on the way.
	prefixOnly := &DispatchContext{SessionID: "a2a-task-2", IsCoAgent: true}

	writes := []ToolCall{
		{Action: "filesystem", Operation: "write_file"}, {Action: "filesystem", Operation: "delete"},
		{Action: "filesystem_op", Operation: "move"}, {Action: "filesystem", Operation: "create_dir_batch"},
		{Action: "filesystem"}, // An unreadable operation fails closed.
		{Action: "file_editor", Operation: "str_replace"},
		{Action: "yaml_editor", Operation: "set"}, {Action: "json_editor", Operation: "delete"},
		{Action: "toml_editor", Operation: "set"}, {Action: "xml_editor", Operation: "add_element"},
	}
	for _, call := range writes {
		for name, dc := range map[string]*DispatchContext{"a2a": a2a, "a2a session prefix": prefixOnly} {
			if got := roleToolRestriction(call, dc); !strings.Contains(got, "policy_denied") {
				t.Errorf("%s run may call %s %q: %q", name, call.Action, call.Operation, got)
			}
		}
		if got := roleToolRestriction(call, local); got != "" {
			t.Errorf("local co-agent lost %s %q: %q", call.Action, call.Operation, got)
		}
	}
	reads := []ToolCall{
		{Action: "filesystem", Operation: "read_file"}, {Action: "filesystem", Operation: "list_dir"}, {Action: "filesystem", Operation: "stat"},
		{Action: "yaml_editor", Operation: "get"}, {Action: "json_editor", Operation: "validate"}, {Action: "xml_editor", Operation: "format"},
		{Action: "file_search", Operation: "grep"}, {Action: "file_reader_advanced", Operation: "head"},
	}
	for _, call := range reads {
		if got := roleToolRestriction(call, a2a); got != "" {
			t.Errorf("a2a run denied the read %s %q: %q", call.Action, call.Operation, got)
		}
	}
}

func TestRoleToolRestrictionHoldsA2APeersToReadsOnDesktopHomepageAndCode(t *testing.T) {
	main := &DispatchContext{SessionID: "default"}
	local := &DispatchContext{SessionID: "coagent-1", IsCoAgent: true, MessageSource: "co_agent", CoAgentSpecialist: "coder"}
	a2a := &DispatchContext{SessionID: "a2a-task-1", IsCoAgent: true, MessageSource: "a2a"}
	prefixOnly := &DispatchContext{SessionID: "a2a-task-2", IsCoAgent: true}

	denied := []ToolCall{
		{Action: "execute_shell"}, {Action: "execute_python"}, {Action: "execute_sandbox"}, {Action: "execute_sudo"},
		{Action: "execute_remote_shell"}, {Action: "remote_execution"}, {Action: "run_tool"}, {Action: "execute_skill"},
		{Action: "run_agent_skill_script"}, {Action: "git_commit"}, // The git aliases resolve to execute_shell.
		{Action: "virtual_desktop", Operation: "write_file"}, {Action: "virtual_desktop", Operation: "install_app"},
		{Action: "virtual_desktop", Operation: "upsert_widget"}, {Action: "virtual_desktop", Operation: "patch_file"},
		{Action: "virtual_desktop", Operation: "delete_app"}, {Action: "virtual_desktop", Operation: "write_workbook"},
		{Action: "virtual_desktop", Operation: "open_app"}, {Action: "virtual_desktop", Operation: "show_notification"},
		{Action: "virtual_desktop", Operation: "bootstrap"}, {Action: "virtual_desktop", Operation: "set_pet"},
		{Action: "virtual_desktop"}, // An unreadable operation fails closed.
		{Action: "virtual_desktop_files", Operation: "write_file"}, {Action: "virtual_desktop_widgets", Operation: "upsert_widget"},
		{Action: "virtual_desktop_apps", Operation: "install_app"}, {Action: "virtual_desktop_app_install"},
		{Action: "homepage", Operation: "write_file"}, {Action: "homepage", Operation: "edit_file"},
		{Action: "homepage", Operation: "exec"}, {Action: "homepage", Operation: "deploy_netlify"},
		{Action: "homepage", Operation: "publish_local"}, {Action: "homepage", Operation: "save_revision"},
		{Action: "homepage"}, {Action: "homepage_file", Operation: "write_file"}, {Action: "homepage_deploy", Operation: "deploy"},
		// The dispatcher redirects a homepage operation named as the tool.
		{Action: "write_file"}, {Action: "deploy_vercel"}, {Action: "build"}, {Action: "restore_revision"},
	}
	for _, call := range denied {
		for name, dc := range map[string]*DispatchContext{"a2a": a2a, "a2a session prefix": prefixOnly} {
			if got := roleToolRestriction(call, dc); !strings.Contains(got, "policy_denied") {
				t.Errorf("%s run may call %s %q: %q", name, call.Action, call.Operation, got)
			}
		}
	}
	// Local runs keep code, desktop and homepage tools.
	for _, call := range []ToolCall{
		{Action: "execute_shell"}, {Action: "execute_python"}, {Action: "run_tool"}, {Action: "execute_skill"},
		{Action: "virtual_desktop", Operation: "write_file"}, {Action: "virtual_desktop", Operation: "install_app"},
		{Action: "homepage", Operation: "write_file"}, {Action: "homepage", Operation: "deploy_netlify"},
	} {
		for name, dc := range map[string]*DispatchContext{"main agent": main, "local co-agent": local} {
			if got := roleToolRestriction(call, dc); got != "" {
				t.Errorf("%s denied %s %q: %q", name, call.Action, call.Operation, got)
			}
		}
	}

	reads := []ToolCall{
		{Action: "virtual_desktop", Operation: "status"}, {Action: "virtual_desktop", Operation: "list_files"},
		{Action: "virtual_desktop", Operation: "read_file"}, {Action: "virtual_desktop", Operation: "search_file"},
		{Action: "virtual_desktop_files", Operation: "read_file_excerpt"}, {Action: "virtual_desktop_apps", Operation: "list_apps"},
		{Action: "virtual_desktop", Operation: "get_widget"}, {Action: "virtual_desktop", Operation: "diagnose_app"},
		{Action: "homepage", Operation: "list_files"}, {Action: "homepage", Operation: "read_file"},
		{Action: "homepage", Operation: "status"}, {Action: "homepage", Operation: "git_log"},
		{Action: "homepage", Operation: "list_revisions"}, {Action: "homepage_git", Operation: "git_status"},
		{Action: "query_memory"}, {Action: "web_search"}, {Action: "remote_control"},
		{Action: "manage_memory", Operation: "read"}, {Action: "knowledge_graph", Operation: "query"},
		{Action: "manage_journal", Operation: "search"}, {Action: "manage_notes", Operation: "list"},
		{Action: "filesystem", Operation: "read_file"},
	}
	for _, call := range reads {
		if got := roleToolRestriction(call, a2a); got != "" {
			t.Errorf("a2a run denied the read %s %q: %q", call.Action, call.Operation, got)
		}
	}
}
