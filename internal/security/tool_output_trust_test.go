package security

import "testing"

func TestIsExecutionToolOutputMatchesSemiTrustedClass(t *testing.T) {
	for _, action := range []string{"execute_shell", "execute_python", "run_tool", " Execute_Shell "} {
		if !IsExecutionToolOutput(action) {
			t.Errorf("IsExecutionToolOutput(%q) = false, want true", action)
		}
	}
	for _, action := range []string{"", "activate_tools", "file_reader", "fetch_url", "execute_remote_shell", "game_maker_file", "invoke_tool", "new_external_connector"} {
		if IsExecutionToolOutput(action) {
			t.Errorf("IsExecutionToolOutput(%q) = true, want false", action)
		}
	}
}
