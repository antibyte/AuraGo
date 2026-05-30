package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestShouldSuppressStreamedToolCallJSONRecognizesToolParametersWrapper(t *testing.T) {
	input := `{"tool": "invasion_control", "parameters": {"operation": "egg_status", "nest_id": "7680f451-bad4-4908-92da-e286eb5f7c2a"}}`

	if !shouldSuppressStreamedToolCallJSON(input) {
		t.Fatal("expected streamed tool/parameters JSON to be suppressed")
	}
}

func TestShouldSuppressStreamedToolCallJSONRecognizesFencedToolDiscoveryPayload(t *testing.T) {
	input := "```json\n" + `{
  "_todo": "- [ ] Discover available tools",
  "operation": "list_categories",
  "category": "system",
  "query": null,
  "tool_name": null
}` + "\n```"

	if !shouldSuppressStreamedToolCallJSON(input) {
		t.Fatal("expected fenced tool-discovery JSON to be suppressed")
	}
}

func TestShouldSuppressStreamedToolCallJSONAllowsOrdinaryJSON(t *testing.T) {
	input := `{"status": "ok", "message": "plain JSON answer"}`

	if shouldSuppressStreamedToolCallJSON(input) {
		t.Fatal("did not expect ordinary JSON answer to be suppressed")
	}
}

func TestShouldHoldPotentialStreamedToolCallJSONFencePrefix(t *testing.T) {
	input := "```json\n{\n  \"_todo\":"

	if !shouldHoldPotentialStreamedToolCallJSON(input) {
		t.Fatal("expected fenced JSON tool-call prefix to be held until classification")
	}
}

func TestShouldHoldPotentialStreamedToolCallJSONPrefix(t *testing.T) {
	input := `{"tool": "invas`

	if !shouldHoldPotentialStreamedToolCallJSON(input) {
		t.Fatal("expected partial tool JSON prefix to be held until the router can classify it")
	}
}

func TestShouldSuppressStreamedToolCallTextRecognizesKimiFunctionWrapper(t *testing.T) {
	input := "Lass mich schnell das Wetter checken.\n\n<function>\n<invoke name=\"api_request\">"

	idx, ok := shouldSuppressStreamedToolCallText(input)
	if !ok {
		t.Fatal("expected Kimi <function><invoke> wrapper to be suppressed")
	}
	if got := input[:idx]; got != "Lass mich schnell das Wetter checken.\n\n" {
		t.Fatalf("prefix before tool wrapper = %q", got)
	}
}

func TestUTF8SafePrefixSplitDoesNotCutMultibyteRune(t *testing.T) {
	input := "aaaaaü" + strings.Repeat("b", 16)

	prefix, suffix := utf8SafePrefixSplit(input, len(input)-17)

	if !utf8.ValidString(prefix) || !utf8.ValidString(suffix) {
		t.Fatalf("split produced invalid UTF-8: prefix=%q suffix=%q", prefix, suffix)
	}
	if prefix != "aaaaa" {
		t.Fatalf("prefix = %q, want %q", prefix, "aaaaa")
	}
	if suffix != "ü"+strings.Repeat("b", 16) {
		t.Fatalf("suffix = %q", suffix)
	}
	if prefix+suffix != input {
		t.Fatalf("split did not preserve input: %q + %q", prefix, suffix)
	}
}

func TestUTF8SafeSuffixDoesNotCutMultibyteRune(t *testing.T) {
	input := "aaaaaü" + strings.Repeat("b", 16)

	suffix := utf8SafeSuffix(input, 17)

	if !utf8.ValidString(suffix) {
		t.Fatalf("suffix is invalid UTF-8: %q", suffix)
	}
	if suffix != "ü"+strings.Repeat("b", 16) {
		t.Fatalf("suffix = %q", suffix)
	}
}
