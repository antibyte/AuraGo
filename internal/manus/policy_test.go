package manus

import "testing"

func TestPolicyDefaultsDenyMutations(t *testing.T) {
	t.Parallel()

	policy := Policy{ReadOnly: true}
	for _, operation := range []string{"create_task", "send_message", "stop_task", "download_attachments"} {
		if err := policy.Authorize(operation); err == nil {
			t.Errorf("Authorize(%q) error = nil, want denial", operation)
		}
	}
	if err := policy.Authorize("get_credits"); err != nil {
		t.Fatalf("Authorize(read) error = %v", err)
	}
}

func TestPolicyRequiresExplicitPermissionForEachMutation(t *testing.T) {
	t.Parallel()

	policy := Policy{
		AllowCreateTasks:   true,
		AllowSendMessages:  true,
		AllowStopTasks:     true,
		AllowFileUploads:   true,
		AllowFileDownloads: true,
	}
	for _, operation := range []string{"create_task", "send_message", "stop_task", "download_attachments"} {
		if err := policy.Authorize(operation); err != nil {
			t.Errorf("Authorize(%q) error = %v", operation, err)
		}
	}
	if err := policy.AuthorizeUpload(); err != nil {
		t.Fatalf("AuthorizeUpload() error = %v", err)
	}
}

func TestPolicyEnforcesResourceAllowlists(t *testing.T) {
	t.Parallel()

	policy := Policy{
		AllowedProjectIDs:   []string{"project-1"},
		AllowedConnectorIDs: []string{"connector-1"},
		AllowedSkillIDs:     []string{"skill-1"},
	}
	if err := policy.ValidateResources("project-1", []string{"connector-1"}, []string{"skill-1"}, []string{"skill-1"}); err != nil {
		t.Fatalf("ValidateResources(allowed) error = %v", err)
	}
	for name, call := range map[string]func() error{
		"project":   func() error { return policy.ValidateResources("project-2", nil, nil, nil) },
		"connector": func() error { return policy.ValidateResources("", []string{"connector-2"}, nil, nil) },
		"skill":     func() error { return policy.ValidateResources("", nil, []string{"skill-2"}, nil) },
	} {
		if err := call(); err == nil {
			t.Errorf("%s allowlist violation was accepted", name)
		}
	}
}
