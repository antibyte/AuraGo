package agent

import "testing"

func TestShouldSuppressStreamedToolCallJSONRecognizesToolParametersWrapper(t *testing.T) {
	input := `{"tool": "invasion_control", "parameters": {"operation": "egg_status", "nest_id": "7680f451-bad4-4908-92da-e286eb5f7c2a"}}`

	if !shouldSuppressStreamedToolCallJSON(input) {
		t.Fatal("expected streamed tool/parameters JSON to be suppressed")
	}
}

func TestShouldSuppressStreamedToolCallJSONAllowsOrdinaryJSON(t *testing.T) {
	input := `{"status": "ok", "message": "plain JSON answer"}`

	if shouldSuppressStreamedToolCallJSON(input) {
		t.Fatal("did not expect ordinary JSON answer to be suppressed")
	}
}

func TestShouldHoldPotentialStreamedToolCallJSONPrefix(t *testing.T) {
	input := `{"tool": "invas`

	if !shouldHoldPotentialStreamedToolCallJSON(input) {
		t.Fatal("expected partial tool JSON prefix to be held until the router can classify it")
	}
}
