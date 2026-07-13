package tools

import "testing"

func TestManusAPIKeyCannotBeExportedToPythonTools(t *testing.T) {
	t.Parallel()

	if IsPythonAccessibleSecret("manus_api_key") {
		t.Fatal("manus_api_key must be blocked from Python tool export")
	}
}
