package agent

import (
	"regexp"
	"strings"

	"aurago/internal/security"
)

// networkFetchPattern matches what makes a shell or code call read from the
// network: a URL, a download or request tool, or a remote copy.
var networkFetchPattern = regexp.MustCompile(`(?i)https?://|\b(?:curl|wget|invoke-webrequest|iwr|invoke-restmethod|irm|urllib|httpx|aiohttp|ncat)\b|\brequests\.|\bfetch\(|\b(?:nc|ssh|scp|rsync)\s|\bgit\s+(?:clone|pull)\b`)

// networkCapableTools run commands or code that can fetch third-party content.
// Their output is untrusted only when the call itself reaches for the network.
var networkCapableTools = map[string]bool{
	"execute_shell": true, "execute_python": true, "run_tool": true, "execute_remote_shell": true,
	"execute_sandbox": true, "execute_command": true, "execute_sudo": true, "execute_tool": true,
	"execute_skill": true,
}

// untrustedToolCall reports whether the output of the call is third-party
// text: a tool of security.UntrustedContentTool, or a shell or code tool whose
// command or code fetches from the network ("df -h" does not, "curl https://x" does).
func untrustedToolCall(tc ToolCall) bool {
	if security.UntrustedContentTool(tc.Action, tc.Operation) {
		return true
	}
	if !networkCapableTools[strings.ToLower(strings.TrimSpace(tc.Action))] {
		return false
	}
	return networkFetchPattern.MatchString(copiedSpanArguments(tc, false))
}
