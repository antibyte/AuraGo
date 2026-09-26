package server

import (
	"encoding/json"
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
		{"server bound job", strings.Replace(valid, "<arg_key>job_id</arg_key><arg_value>job_current</arg_value>", "", 1), true},
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

func starterFunctionSourceEnvelope(source string) string {
	return "<tool_call><function=game_maker_file>\n" +
		"<parameter=path>src/main.ts</parameter>\n" +
		"<parameter=content>" + source + "</parameter>\n</function></tool_call>"
}

func TestGameStarterProviderSourceFormats(t *testing.T) {
	const code = "const active = (x: number) => x < 3 && x > 0;\nconst literal = '&lt;tag&gt;';"
	fields := map[string]string{"path": "src/main.ts", "content": code}
	args, _ := json.Marshal(fields)
	encodedArgs, _ := json.Marshal(string(args))
	flat, _ := json.Marshal(map[string]string{"action": "game_maker_file", "path": fields["path"], "content": code})
	jsonCall := `{"name":"game_maker_file","arguments":` + string(args) + `}`
	functionCall := starterFunctionSourceEnvelope(code)
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"function", functionCall, true},
		{"function explicit write", strings.Replace(functionCall, "<parameter=path>", "<parameter=operation>write</parameter><parameter=path>", 1), true},
		{"json", jsonCall, true},
		{"wrapped json", "<tool_call>" + jsonCall + "</tool_call>", true},
		{"json string arguments", `{"name":"game_maker_file","arguments":` + string(encodedArgs) + `}`, true},
		{"flat action", string(flat), true},
		{"other function", strings.Replace(functionCall, "game_maker_file", "execute_shell", 1), false},
		{"other function path", strings.Replace(functionCall, "src/main.ts", "src/common.ts", 1), false},
		{"function replace", strings.Replace(functionCall, "<parameter=path>", "<parameter=operation>replace</parameter><parameter=path>", 1), false},
		{"function stale hash", strings.Replace(functionCall, "<parameter=path>", "<parameter=expected_sha256>stale</parameter><parameter=path>", 1), false},
		{"function foreign job", strings.Replace(functionCall, "<parameter=path>", "<parameter=job_id>another-job</parameter><parameter=path>", 1), false},
		{"function duplicate field", strings.Replace(functionCall, "<parameter=path>", "<parameter=path>src/main.ts</parameter><parameter=path>", 1), false},
		{"function extra field", strings.Replace(functionCall, "<parameter=path>", "<parameter=command>ignored</parameter><parameter=path>", 1), false},
		{"function incomplete", strings.Replace(functionCall, "</function>", "", 1), false},
		{"function mixed text", strings.Replace(functionCall, "</function>", "Done</function>", 1), false},
		{"two functions", strings.Replace(functionCall, "</tool_call>", "<function=game_maker_file></function></tool_call>", 1), false},
		{"json duplicate tool", strings.Replace(jsonCall, `"name":`, `"name":"execute_shell","name":`, 1), false},
		{"json duplicate field", strings.Replace(jsonCall, `"path":`, `"path":"src/common.ts","path":`, 1), false},
		{"json extra field", strings.Replace(jsonCall, `"arguments":`, `"command":"ignored","arguments":`, 1), false},
		{"json extra argument", strings.Replace(jsonCall, `"path":`, `"command":"ignored","path":`, 1), false},
		{"json other tool", strings.Replace(jsonCall, "game_maker_file", "execute_shell", 1), false},
		{"json wrong job", strings.Replace(jsonCall, `"path":`, `"job_id":"another-job","path":`, 1), false},
		{"json null operation", strings.Replace(jsonCall, `"path":`, `"operation":null,"path":`, 1), false},
		{"json wrong type", strings.Replace(jsonCall, `"path":`, `"operation":[],"path":`, 1), false},
		{"two json calls", jsonCall + jsonCall, false},
		{"json array", "[" + jsonCall + "]", false},
		{"json prefix", "I will write it. " + jsonCall, false},
		{"json suffix", jsonCall + " Done", false},
		{"json truncated", strings.TrimSuffix(jsonCall, "}"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gameStarterWrappedSource(tc.text, "current-job", "current-revision")
			if tc.want {
				if err != nil || got != code {
					t.Fatalf("source changed or was rejected: %q, %v", got, err)
				}
			} else if err == nil || got != "" {
				t.Fatal("accepted an ambiguous or unbound source envelope")
			}
		})
	}
}
