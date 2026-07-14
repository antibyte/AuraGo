package ui

import (
	"strings"
	"testing"
)

func TestOutgoingWebhookSaveReloadsMaskedServerState(t *testing.T) {
	t.Parallel()

	source := normalizeAssetText(mustReadUIFile(t, "cfg/webhooks.js"))
	saveStart := strings.Index(source, "async function ogSave()")
	deleteStart := strings.Index(source, "async function ogDelete(")
	if saveStart < 0 || deleteStart <= saveStart {
		t.Fatalf("could not isolate ogSave: save=%d delete=%d", saveStart, deleteStart)
	}
	saveBody := source[saveStart:deleteStart]
	if strings.Contains(saveBody, "ogWebhooks = newList") {
		t.Fatal("ogSave retains the unmasked form payload after persistence")
	}
	for _, marker := range []string{
		"const maskedResp = await fetch('/api/outgoing-webhooks')",
		"ogWebhooks = await maskedResp.json()",
	} {
		if !strings.Contains(saveBody, marker) {
			t.Fatalf("ogSave does not reload masked server state; missing %q", marker)
		}
	}
}
