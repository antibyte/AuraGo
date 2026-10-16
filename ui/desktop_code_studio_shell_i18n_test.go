package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCodeStudioShellI18n(t *testing.T) {
	t.Parallel()

	terminal := readDesktopAssetText(t, "js/desktop/apps/code-studio/terminal.js")
	for _, want := range []string{
		"tr('codeStudio.shell_n', 'Shell {{n}}', { n: index + 1 })",
		"tr('codeStudio.title', 'Code Studio') + ' - ' + shellName(index)",
	} {
		if !strings.Contains(terminal, want) {
			t.Fatalf("code studio terminal i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"'Shell ' +",
		"'Shell 1'",
		"\"Shell \"",
		"'Code Studio - Shell '",
	} {
		if strings.Contains(terminal, forbidden) {
			t.Fatalf("code studio terminal still hardcodes %q", forbidden)
		}
	}

	core := readDesktopAssetText(t, "js/desktop/apps/code-studio/core.js")
	if !strings.Contains(core, `title="${esc(tr('codeStudio.exitZen', 'Exit Zen Mode'))}"`) {
		t.Fatal("code studio zen exit must localize codeStudio.exitZen")
	}
	if strings.Contains(core, `title="Exit Zen Mode (Esc)"`) {
		t.Fatal("code studio zen exit still hardcodes Exit Zen Mode (Esc)")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["codeStudio.shell_n"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty codeStudio.shell_n", path)
		}
		if !strings.Contains(got, "{{n}}") {
			t.Fatalf("%s codeStudio.shell_n must keep {{n}}", path)
		}
		if lang == "de" && got == "Shell {{n}}" {
			t.Fatalf("%s must not copy the English shell tab label", path)
		}
		if lang == "fr" && got == "Shell {{n}}" {
			t.Fatalf("%s must not copy the English shell tab label", path)
		}
		if strings.TrimSpace(values["codeStudio.exitZen"]) == "" {
			t.Fatalf("%s missing non-empty codeStudio.exitZen", path)
		}
		if strings.TrimSpace(values["codeStudio.title"]) == "" {
			t.Fatalf("%s missing non-empty codeStudio.title", path)
		}
		lineMode := strings.TrimSpace(values["codeStudio.terminalLineMode"])
		if lineMode == "" {
			t.Fatalf("%s missing non-empty codeStudio.terminalLineMode", path)
		}
		if lang != "en" && lineMode == "Line mode" {
			t.Fatalf("%s must not copy the English line-mode label", path)
		}
	}
}

func TestDesktopCodeStudioTerminalFrames(t *testing.T) {
	t.Parallel()

	terminal := readDesktopAssetText(t, "js/desktop/apps/code-studio/terminal.js")
	for _, want := range []string{
		"ws.send(new TextEncoder().encode(data))",
		"ws.send(JSON.stringify({ type: 'resize', cols, rows }))",
		"JSON.stringify({ type: 'end' })",
		"text === '{\"type\":\"line\"}'",
		"codeStudio.terminalLineMode",
	} {
		if !strings.Contains(terminal, want) {
			t.Fatalf("code studio terminal frame marker missing %q", want)
		}
	}
	if strings.Contains(terminal, "ws.send(data)") {
		t.Fatal("code studio keystrokes must not be sent as text frames")
	}
	core := readDesktopAssetText(t, "js/desktop/apps/code-studio/core.js")
	if !strings.Contains(core, "endTerminalSocket(socket)") {
		t.Fatal("closing code studio must end the terminal socket")
	}
}
