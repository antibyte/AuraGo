package agent

import (
	"strings"
	"testing"

	"aurago/internal/security"
)

// Readable source isolation must survive payload extraction and re-wrapping by
// compression and bounding; entity-like text inside code stays literal.
func TestToolResultPayloadKeepsRawSourceExact(t *testing.T) {
	output := `Tool Output: {"content":"if (s === \"&amp;\" && a < b) return 'x';"}`
	isolated := security.NewGuardian(nil).SanitizeToolOutput("game_maker_file", output)
	if isolated != "<external_data>\n"+output+"\n</external_data>" {
		t.Fatalf("fixture was not raw-isolated: %q", isolated)
	}
	payload, ok, raw := toolResultPayloadForm(isolated)
	if !ok || !raw || payload != output {
		t.Fatalf("payload = %q isolated=%v raw=%v", payload, ok, raw)
	}
	if again := isolateToolPayload(payload, raw); again != isolated {
		t.Fatalf("re-isolation changed the form: %q", again)
	}
}

func TestToolResultPayloadStillDecodesEscapedIsolation(t *testing.T) {
	output := `Tool Output: {"content":"a < b & c"}`
	isolated := security.IsolateExternalData(output)
	payload, ok, raw := toolResultPayloadForm(isolated)
	if !ok || raw || payload != output {
		t.Fatalf("payload = %q isolated=%v raw=%v", payload, ok, raw)
	}
	if again := isolateToolPayload(payload, raw); again != isolated {
		t.Fatalf("escaped form was not preserved: %q", again)
	}
	if legacy, ok := toolResultPayload(isolated); !ok || legacy != output {
		t.Fatalf("legacy extraction = %q, %v", legacy, ok)
	}
}

func TestBoundedToolResultKeepsSingleBoundaryForRawSource(t *testing.T) {
	output := `Tool Output: {"status":"ok","content":"` + strings.Repeat("const a = 'b';", 400) + `"}`
	isolated := security.NewGuardian(nil).SanitizeToolOutput("game_maker_file", output)
	bounded := boundedToolResult("game_maker_file", isolated, 400, ToolResultSuccess)
	if len(bounded) > 400 || strings.Count(bounded, "</external_data>") > 1 {
		t.Fatalf("bounded raw result broke limits or boundary: %q", bounded)
	}
	if !strings.Contains(bounded, `"truncated":true`) {
		t.Fatalf("bounded result lost its readable envelope: %q", bounded)
	}
}

// Tool JSON goes to a model, not an HTML page: <, > and & stay literal so code
// reads and copies exactly as stored.
func TestGameMakerToolJSONKeepsCodeCharactersLiteral(t *testing.T) {
	got := gameMakerToolJSON(map[string]any{"content": "if (a < b && c > d) {}"})
	if !strings.Contains(got, "if (a < b && c > d) {}") {
		t.Fatalf("code characters were escaped: %s", got)
	}
}
