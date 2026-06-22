package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSkillResourceDialogTranslationsExist(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("lang", "skills", "*.json"))
	if err != nil {
		t.Fatalf("glob skills translations: %v", err)
	}
	if len(files) < 16 {
		t.Fatalf("expected all skills language files, got %d", len(files))
	}

	required := []string{
		"skills.agent_resource_new_title",
		"skills.agent_resource_rename_title",
		"skills.agent_resource_upload_title",
		"skills.agent_resource_path_label",
		"skills.agent_resource_path_help",
		"skills.agent_resource_path_invalid",
		"skills.agent_resource_confirm_create",
		"skills.agent_resource_confirm_rename",
		"skills.agent_resource_confirm_upload",
		"skills.agent_delete_file_title",
		"skills.agent_delete_file_text",
		"skills.agent_delete_file_button",
		"skills.agent_resource_empty_hint",
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", path, key)
			}
		}
	}
}
