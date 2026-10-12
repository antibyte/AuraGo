package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// outputVaultPayload is the model-facing reply for a tool result that the
// primary output vault archived.
type outputVaultPayload struct {
	Status        string `json:"status"`
	OutputRef     string `json:"output_ref"`
	ToolName      string `json:"tool_name"`
	Summary       string `json:"summary"`
	View          string `json:"view"`
	OriginalChars int    `json:"original_chars"`
	Message       string `json:"message"`
}

// vaultHeadTailMinLines is the line count from which a vault view shows the
// head and the tail of an output (a log ends with its verdict) instead of a
// prefix of it.
const vaultHeadTailMinLines = 12

// vaultReadablePayload is the readable text of a dispatch output: the
// "Tool Output:" prefixes and isolation wrappers (handler-owned and Guardian)
// are removed and escaped isolation is decoded. Vault views and
// read_tool_output count lines and byte offsets in this text. isolated
// reports that what a caller shows of it must be isolated again.
func vaultReadablePayload(output string) (string, bool) {
	payload, isolated, _ := toolResultPayloadForm(output)
	for range 2 {
		inner, wrapped, _ := toolResultPayloadForm(payload)
		payload = inner
		if !wrapped {
			break
		}
		isolated = true
	}
	return payload, isolated
}

// vaultOutputStats is the summary in a vault reply; its view already shows
// the first lines.
func vaultOutputStats(payload string) string {
	return fmt.Sprintf("%d bytes, %d lines", len(payload), len(splitOutputLines(payload)))
}

// summarizeToolOutputForVault is the stored summary (read_tool_output
// view=summary): size plus the first non-empty lines.
func summarizeToolOutputForVault(content string) string {
	lines := splitOutputLines(content)
	var nonEmpty []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty = append(nonEmpty, line)
		if len(nonEmpty) >= 5 {
			break
		}
	}
	prefix := fmt.Sprintf("%d chars", len(content))
	if len(lines) > 0 {
		prefix = fmt.Sprintf("%s, %d lines", prefix, len(lines))
	}
	if len(nonEmpty) == 0 {
		return prefix
	}
	summary := prefix + ". First lines: " + strings.Join(nonEmpty, " | ")
	if len(summary) > 800 {
		summary = truncateUTF8ToLimit(summary, 800, "...")
	}
	return summary
}

// toolOutputVaultView is what a vault reply shows of an output: the text,
// which part that is, and the read_tool_output arguments for the rest.
type toolOutputVaultView struct {
	text, shown, next string
}

// buildToolOutputVaultView shows at most n bytes of payload: the head and the
// tail of a line-oriented output, otherwise a prefix.
func buildToolOutputVaultView(payload string, n int) toolOutputVaultView {
	if len(payload) <= n {
		return toolOutputVaultView{text: payload, shown: "the complete output"}
	}
	if lines := splitOutputLines(payload); len(lines) >= vaultHeadTailMinLines {
		if view, ok := headTailVaultView(lines, n); ok {
			return view
		}
	}
	text := truncateUTF8Prefix(payload, n)
	return toolOutputVaultView{
		text:  text,
		shown: fmt.Sprintf("bytes 0-%d of %d", len(text), len(payload)),
		next:  fmt.Sprintf("offset=%d", len(text)),
	}
}

// headTailVaultView keeps whole lines: about a third of n for the tail, the
// rest (and what the tail leaves) for the head, joined by an omission line.
// ok is false when not even the first line fits.
func headTailVaultView(lines []string, n int) (toolOutputVaultView, bool) {
	total := len(lines)
	const markerReserve = 40
	tail, tailBytes := 0, 0
	for tail < total-1 && tailBytes+len(lines[total-1-tail])+1 <= n/3 {
		tailBytes += len(lines[total-1-tail]) + 1
		tail++
	}
	head, headBytes := 0, 0
	for head < total-tail && headBytes+len(lines[head])+1 <= n-tailBytes-markerReserve {
		headBytes += len(lines[head]) + 1
		head++
	}
	if head == 0 || head+tail >= total {
		return toolOutputVaultView{}, false
	}
	omitted := total - head - tail
	parts := make([]string, 0, head+tail+1)
	parts = append(parts, lines[:head]...)
	parts = append(parts, fmt.Sprintf("[... %d lines omitted ...]", omitted))
	parts = append(parts, lines[total-tail:]...)
	shown := fmt.Sprintf("lines 1-%d", head)
	if tail > 0 {
		shown += fmt.Sprintf(" and %d-%d", total-tail+1, total)
	}
	return toolOutputVaultView{
		text:  strings.Join(parts, "\n"),
		shown: fmt.Sprintf("%s of %d", shown, total),
		next:  fmt.Sprintf(`view="range", start_line=%d, end_line=%d`, head+1, total-tail),
	}, true
}

// vaultReplyMessage tells the model what the view covers and how to read on.
func vaultReplyMessage(ref string, view toolOutputVaultView) string {
	if view.next == "" {
		return "Full output archived; the view shows " + view.shown + "."
	}
	return fmt.Sprintf(`Full output archived; the view shows %s. Read more with read_tool_output ref=%q and %s, or view="grep" with a query.`, view.shown, ref, view.next)
}

// fitOutputVaultReply builds the vault reply with the largest view that keeps
// the whole reply within maxInline bytes, and returns it with that view. When
// not even an empty view fits, the reply without a view is returned anyway.
func fitOutputVaultReply(reply outputVaultPayload, payload string, isolated bool, suffix string, maxInline int) (string, string) {
	render := func(n int) (string, string) {
		view := buildToolOutputVaultView(payload, n)
		candidate := reply
		candidate.View = view.text
		candidate.Message = vaultReplyMessage(reply.OutputRef, view)
		return encodeOutputVaultReply(candidate, isolated, suffix), view.text
	}
	best, bestView := render(0)
	lo, hi := 1, len(payload)
	for lo <= hi {
		mid := lo + (hi-lo)/2
		if content, view := render(mid); len(content) <= maxInline {
			best, bestView = content, view
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best, bestView
}

// encodeOutputVaultReply renders the reply as the model receives it. An
// isolated original keeps escaped isolation: summaries and views are
// untrusted output fragments. The isolation escapes <, > and & itself, so
// JSON need not encode them as unicode escapes.
func encodeOutputVaultReply(reply outputVaultPayload, isolated bool, suffix string) string {
	if !isolated {
		b, _ := json.Marshal(reply)
		return "Tool Output: " + string(b) + suffix
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(reply)
	return "Tool Output: " + isolateToolPayload(strings.TrimSuffix(buf.String(), "\n"), false) + suffix
}

// utf8Suffix returns the longest suffix of s of at most maxBytes bytes that
// starts at a rune boundary.
func utf8Suffix(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	start := len(s) - maxBytes
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return s[start:]
}
