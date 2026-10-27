package security

import (
	"bytes"
	"encoding/json"
	"strings"
)

// jsonLeaf is one decoded JSON string value and the byte span of its quoted
// token in the body it came from.
type jsonLeaf struct {
	start, end int
	value      string
}

// jsonBodyStart returns the offset of the JSON object or array in a tool
// output, past whitespace and the "Tool Output:" labels the dispatcher adds,
// or -1 when the output does not start like JSON.
func jsonBodyStart(body string) int {
	i := 0
	for {
		for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r') {
			i++
		}
		switch {
		case strings.HasPrefix(body[i:], "[Tool Output]"):
			i += len("[Tool Output]")
		case strings.HasPrefix(body[i:], "Tool Output:"):
			i += len("Tool Output:")
		default:
			if i < len(body) && (body[i] == '{' || body[i] == '[') {
				return i
			}
			return -1
		}
	}
}

// jsonStringLeaves decodes the string values (not keys) of the JSON in body.
// It reads until the first syntax error, so output cut off by a size budget or
// followed by guidance text still yields every complete value before the cut.
func jsonStringLeaves(body string) []jsonLeaf {
	base := jsonBodyStart(body)
	if base < 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(body[base:]))
	type frame struct{ object, wantKey bool }
	var stack []frame
	var leaves []jsonLeaf
	for {
		prev := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return leaves
		}
		end := dec.InputOffset()
		top := len(stack) - 1
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, frame{object: true, wantKey: true})
			case '[':
				stack = append(stack, frame{})
			default:
				if top < 0 {
					return leaves
				}
				stack = stack[:top]
				if top > 0 && stack[top-1].object {
					stack[top-1].wantKey = true
				}
			}
		case string:
			if top >= 0 && stack[top].object && stack[top].wantKey {
				stack[top].wantKey = false
				break
			}
			if q := strings.IndexByte(body[base+int(prev):base+int(end)], '"'); q >= 0 {
				leaves = append(leaves, jsonLeaf{start: base + int(prev) + q, end: base + int(end), value: v})
			}
			if top >= 0 && stack[top].object {
				stack[top].wantKey = true
			}
		default:
			if top >= 0 && stack[top].object {
				stack[top].wantKey = true
			}
		}
	}
}

// joinJSONLeaves joins the non-empty leaf values with newlines, the view in
// which a line break the JSON encoder escaped is a real line break again.
func joinJSONLeaves(leaves []jsonLeaf) string {
	var b strings.Builder
	for _, leaf := range leaves {
		if leaf.value == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(leaf.value)
	}
	return b.String()
}

// neutralizeLeafRoleMarkers applies the role-marker rewrite to every decoded
// leaf and splices only the changed tokens back, so the rest of the body stays
// byte-identical.
func neutralizeLeafRoleMarkers(body string, leaves []jsonLeaf) string {
	var b strings.Builder
	last := 0
	for _, leaf := range leaves {
		fixed := neutralizeRoleMarkers(leaf.value)
		if fixed == leaf.value {
			continue
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(fixed); err != nil {
			continue
		}
		b.WriteString(body[last:leaf.start])
		b.WriteString(strings.TrimSuffix(buf.String(), "\n"))
		last = leaf.end
	}
	if last == 0 {
		return body
	}
	b.WriteString(body[last:])
	return b.String()
}

// mergeScanResults keeps a and folds b's level, patterns and message into it.
func mergeScanResults(a, b ScanResult) ScanResult {
	if b.Level > a.Level {
		a.Level = b.Level
	}
	for _, p := range b.Patterns {
		a.Patterns = appendUniqueString(a.Patterns, p)
	}
	if b.Message != "" && !strings.Contains(a.Message, b.Message) {
		if a.Message != "" {
			a.Message += "; "
		}
		a.Message += b.Message
	}
	return a
}
