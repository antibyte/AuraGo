package cydcreator

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func readDiagFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "diagnostics", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseDiagnosticsCompileFixture(t *testing.T) {
	got := ParseDiagnostics(readDiagFixture(t, "compile_errors.txt"), "/work/ccb_0123456789ab/")
	want := []Diagnostic{
		{File: "src/app/sensors.h", Line: 5, Column: 13, Severity: "warning", Message: "'int counter' defined but not used [-Wunused-variable]"},
		{File: "src/app/app_main.cpp", Line: 42, Column: 5, Severity: "error", Message: "'update_weather' was not declared in this scope"},
		{File: "src/app/app_main.cpp", Line: 42, Column: 5, Severity: "note", Message: "suggested alternative: 'update_weathr'"},
		{File: "src/app/app_main.cpp", Line: 3, Column: 10, Severity: "error", Message: "missing_lib.h: No such file or directory"},
		{File: "src/generated/ui.cpp", Line: 118, Column: 22, Severity: "error", Message: "'ui_lbl_temp' was not declared in this scope", Generated: true},
		{File: "/root/.platformio/packages/framework-arduinoespressif32/libraries/WiFi/src/WiFiGeneric.h", Line: 120, Column: 10,
			Severity: "error", Message: "'wifi_event_t' has not been declared"},
		{File: "lib/cyd_template/src/cyd_wifi.cpp", Line: 77, Column: 9, Severity: "warning", Message: "unused variable 'retries' [-Wunused-variable]", Generated: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics:\n%s\nwant\n%s", dumpDiags(got), dumpDiags(want))
	}
}

func TestParseDiagnosticsLinkFixture(t *testing.T) {
	got := ParseDiagnostics(readDiagFixture(t, "link_errors.txt"), "/work/ccb_0123456789ab")
	want := []Diagnostic{
		{Severity: "error", Message: "undefined reference to `fetch_forecast()'"},
		{Severity: "error", Message: "multiple definition of `on_btn_refresh_clicked(_lv_event_t*)'"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics:\n%s\nwant\n%s", dumpDiags(got), dumpDiags(want))
	}
}

func dumpDiags(list []Diagnostic) string {
	var b strings.Builder
	for _, d := range list {
		fmt.Fprintf(&b, "  %+v\n", d)
	}
	return b.String()
}

func TestParseDiagnosticsLineForms(t *testing.T) {
	cases := []struct {
		name, line, root string
		want             []Diagnostic
	}{
		{"ansi colors",
			"\x1b[01m\x1b[K/work/ccb_x/src/app/ui_glue.cpp:7:1:\x1b[m\x1b[K \x1b[01;31m\x1b[Kerror: \x1b[m\x1b[Kexpected ';' before '}' token",
			"/work/ccb_x/",
			[]Diagnostic{{File: "src/app/ui_glue.cpp", Line: 7, Column: 1, Severity: "error", Message: "expected ';' before '}' token"}}},
		{"crlf relative", "src/app/a.cpp:3:4: warning: shadowed\r", "/work/ccb_x",
			[]Diagnostic{{File: "src/app/a.cpp", Line: 3, Column: 4, Severity: "warning", Message: "shadowed"}}},
		{"no column", "/work/ccb_x/src/app/a.cpp:12: error: bad thing", "/work/ccb_x",
			[]Diagnostic{{File: "src/app/a.cpp", Line: 12, Severity: "error", Message: "bad thing"}}},
		{"windows path", `C:\work\ccb_x\src\app\a.cpp:5:2: error: winerr`, `C:\work\ccb_x`,
			[]Diagnostic{{File: "src/app/a.cpp", Line: 5, Column: 2, Severity: "error", Message: "winerr"}}},
		{"dot slash", "./src/app/a.cpp:1:1: note: n", "",
			[]Diagnostic{{File: "src/app/a.cpp", Line: 1, Column: 1, Severity: "note", Message: "n"}}},
		{"board config is generated", "/work/ccb_x/include/board_config.h:9:1: error: e", "/work/ccb_x",
			[]Diagnostic{{File: "include/board_config.h", Line: 9, Column: 1, Severity: "error", Message: "e", Generated: true}}},
		{"warning outside project dropped", "/usr/include/x.h:1:1: warning: w", "/work/ccb_x", nil},
		{"library note dropped", "/work/ccb_x/.pio/libdeps/e/lvgl/src/lv.h:1:1: note: n", "/work/ccb_x", nil},
		{"context lines ignored", "/work/ccb_x/src/app/a.cpp: In function 'void f()':\n" +
			"In file included from /work/ccb_x/src/app/a.cpp:1:\n" +
			"                 from /work/ccb_x/src/app/b.cpp:2:\n" +
			"/work/ccb_x/src/app/a.cpp:10:5:   required from here", "/work/ccb_x", nil},
	}
	for _, tc := range cases {
		got := ParseDiagnostics(tc.line, tc.root)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n%s\nwant\n%s", tc.name, dumpDiags(got), dumpDiags(tc.want))
		}
	}
}

func TestParseDiagnosticsMessageCap(t *testing.T) {
	long := strings.Repeat("ä", 2000)
	got := ParseDiagnostics("src/app/a.cpp:1:1: error: "+long, "")
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics", len(got))
	}
	msg := got[0].Message
	if len(msg) > maxDiagMessageBytes+len("…") || !strings.HasSuffix(msg, "…") || !utf8.ValidString(msg) {
		t.Fatalf("message not capped cleanly: %d bytes", len(msg))
	}
}

func TestParseDiagnosticsCap(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&b, "src/app/a.cpp:%d:1: warning: w%d\n", i+1, i)
	}
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "src/app/a.cpp:%d:1: note: n%d\n", i+1, i)
	}
	b.WriteString("src/app/a.cpp:999:1: error: the real problem\n")
	got := ParseDiagnostics(b.String(), "")
	if len(got) != maxDiagnostics {
		t.Fatalf("%d diagnostics, want %d", len(got), maxDiagnostics)
	}
	last := got[len(got)-1]
	if last.Severity != "error" || last.Message != "the real problem" {
		t.Fatalf("error was dropped: last = %+v", last)
	}
	for _, d := range got {
		if d.Severity == "note" {
			t.Fatal("notes must be dropped before warnings")
		}
	}
	if got[0].Message != "w0" || got[198].Message != "w198" {
		t.Fatalf("log order not preserved: %q .. %q", got[0].Message, got[198].Message)
	}

	b.Reset()
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&b, "src/app/a.cpp:%d:1: error: e%d\n", i+1, i)
	}
	errs := ParseDiagnostics(b.String(), "")
	if len(errs) != maxDiagnostics || errs[0].Message != "e0" || errs[199].Message != "e199" {
		t.Fatalf("error cap: %d entries, first %q last %q", len(errs), errs[0].Message, errs[len(errs)-1].Message)
	}
}

func TestIsGeneratedPath(t *testing.T) {
	cases := map[string]bool{
		"src/generated/ui.cpp": true, "src/generated/fonts.json": true, "lib/cyd_template/src/cyd.h": true,
		"include/board_config.h": true, "platformio.ini": true, "partitions.csv": true, "./src/generated/ui.h": true,
		`src\generated\ui.cpp`: true,
		"src/app/app_main.cpp": false, "src/generatedx/ui.cpp": false, "lib/other/x.cpp": false, "cyd.project.json": false,
		"assets/sun.png": false, "": false,
	}
	for rel, want := range cases {
		if got := IsGeneratedPath(rel); got != want {
			t.Errorf("IsGeneratedPath(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestIsEditablePath(t *testing.T) {
	cases := map[string]bool{
		"src/app/app_main.cpp": true, "src/app/app_events.cpp": true, "src/app/net/weather.h": true,
		"src/app/a/b/c/d/x.hpp": true, "src/app/util.c": true, "src/app/my-file_2.cpp": true,
		"src/app/a/b/c/d/e/x.cpp": false, // five directory levels
		"src/app/x.ino":           false, "src/app/x.CPP": false, "src/app/x": false, "src/app/": false,
		"src/app/../x.cpp": false, "src/app/./x.cpp": false, "src/app/.hidden.cpp": false, "src/app/.git/x.cpp": false,
		"src/app//x.cpp": false, `src\app\x.cpp`: false, "/src/app/x.cpp": false, "src/generated/ui.cpp": false,
		"src/app/x y.cpp": false, "src/app/x\x00.cpp": false, "src/apps/x.cpp": false, "": false,
		"src/app/" + strings.Repeat("a", 150) + ".cpp": false,
	}
	for rel, want := range cases {
		if got := IsEditablePath(rel); got != want {
			t.Errorf("IsEditablePath(%q) = %v, want %v", rel, got, want)
		}
	}
}
