package server

import (
	"strings"
	"testing"
)

func starterSourceEnvelope(jobID, revision, source string) string {
	return "<tool_call>game_maker_file" +
		"<arg_key>expected_sha256</arg_key><arg_value>" + revision + "</arg_value>" +
		"<arg_key>job_id</arg_key><arg_value>" + jobID + "</arg_value>" +
		"<arg_key>operation</arg_key><arg_value>write</arg_value>" +
		"<arg_key>path</arg_key><arg_value>src/main.ts</arg_value>" +
		"<arg_key>content</arg_key><arg_value>" + source + "</arg_value></tool_call>"
}

func TestGameStarterWrappedSourceRequiresExactSingleWrite(t *testing.T) {
	const jobID, revision = "job_current", "current-revision"
	const code = "import { startGame } from './common';\nconst active = (x: number) => x < 3 && x > 0;\n"
	valid := starterSourceEnvelope(jobID, revision, code)
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"source", valid, true},
		{"outer whitespace", " \n" + valid + "\n", true},
		{"wrong job", starterSourceEnvelope("job_other", revision, code), false},
		{"stale revision", starterSourceEnvelope(jobID, "stale", code), false},
		{"other file", strings.Replace(valid, "src/main.ts", "src/common.ts", 1), false},
		{"traversal", strings.Replace(valid, "src/main.ts", "../src/main.ts", 1), false},
		{"other operation", strings.Replace(valid, ">write<", ">delete<", 1), false},
		{"other tool", strings.Replace(valid, "game_maker_file", "execute_shell", 1), false},
		{"multiple writes", valid + valid, false},
		{"nested call", starterSourceEnvelope(jobID, revision, valid), false},
		{"commentary prefix", "I will write the file.\n" + valid, false},
		{"commentary suffix", valid + "\nDone", false},
		{"truncated envelope", strings.TrimSuffix(valid, "</tool_call>"), false},
		{"truncated value", strings.Replace(valid, "</arg_value></tool_call>", "</tool_call>", 1), false},
		{"missing field", strings.Replace(valid, "<arg_key>job_id</arg_key><arg_value>job_current</arg_value>", "", 1), false},
		{"duplicate field", strings.Replace(valid, "</tool_call>", "<arg_key>path</arg_key><arg_value>src/main.ts</arg_value></tool_call>", 1), false},
		{"extra field", strings.Replace(valid, "</tool_call>", "<arg_key>command</arg_key><arg_value>ignored</arg_value></tool_call>", 1), false},
		{"empty source", starterSourceEnvelope(jobID, revision, " \n"), false},
		{"oversized", starterSourceEnvelope(jobID, revision, strings.Repeat("x", 4*1024*1024)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gameStarterWrappedSource(tc.text, jobID, revision)
			if tc.want {
				if err != nil || got != strings.TrimSpace(code) {
					t.Fatalf("source changed or was rejected: %q, %v", got, err)
				}
			} else if err == nil || got != "" {
				t.Fatalf("unexpectedly accepted source: %v", err)
			}
		})
	}
}
