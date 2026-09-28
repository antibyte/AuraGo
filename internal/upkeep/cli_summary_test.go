package upkeep

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupSummary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		report Report
		want   string
	}{
		{"empty", Report{}, "Artifact cleanup: 0 removed, 0 kept; 0 B freed.\n"},
		{"deleted", Report{Freed: 2221355008, Entries: []*Entry{{Action: "deleted"}, {Action: "deleted"}, {Action: "deleted"}, {Action: "keep", Path: "/private/backup"}}}, "Artifact cleanup: 3 removed, 1 kept; 2.07 GiB freed.\n"},
		{"preview", Report{Entries: []*Entry{{Action: "delete"}, {Action: "keep"}}}, "Artifact cleanup: 0 removed, 1 kept; 0 B freed; 1 pending removal.\n"},
		{"partial", Report{Freed: 1024, Entries: []*Entry{{Action: "deleted"}, {Action: "delete"}}, Warnings: []string{"private warning detail"}}, "Artifact cleanup: 1 removed, 0 kept; 1.00 KiB freed; 1 pending removal; warnings: 1.\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeCleanupSummary(&out, tc.report); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestMaintenanceCLISummaryPreservesErrorsAndDefaultJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-installation")
	for _, summary := range []bool{false, true} {
		args := []string{"--root", root}
		if summary {
			args = append(args, "--summary")
		}
		var out, stderr bytes.Buffer
		if code := RunCLI(args, &out, &stderr); code != 1 {
			t.Fatalf("summary=%t: exit %d, output %q, stderr %q", summary, code, &out, &stderr)
		}
		if summary {
			if !strings.HasPrefix(out.String(), "Artifact cleanup:") || stderr.Len() == 0 {
				t.Fatalf("missing summary or error: %q, %q", &out, &stderr)
			}
		} else {
			var r Report
			if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Error == "" {
				t.Fatalf("default JSON error report lost: %q", &out)
			}
		}
	}
}
