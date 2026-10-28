package security

import (
	"bytes"
	"encoding/json"
	"strings"
)

// jsonLeaf is one decoded JSON string and the byte span of its quoted token in
// the body it came from. key marks an object key; every other leaf is a value.
type jsonLeaf struct {
	start, end int
	value      string
	key        bool
	field      string // Containing property, inherited by array items; never scan authority.
}

// jsonBodyStart returns the offset of the JSON object, array or string in a
// tool output, past whitespace and the "Tool Output:" labels the dispatcher
// adds, or -1 when the output does not start like JSON.
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
			if i < len(body) && (body[i] == '{' || body[i] == '[' || body[i] == '"') {
				return i
			}
			return -1
		}
	}
}

// jsonStringLeaves decodes the strings of the JSON in body: values and object
// keys. It reads until the first syntax error, so output cut off by a size
// budget or followed by guidance text still yields every complete string before
// the cut. A body that is one JSON string is a single leaf, but only when it is
// valid JSON in full: prose that merely starts with a quote is not.
func jsonStringLeaves(body string) []jsonLeaf {
	base := jsonBodyStart(body)
	if base < 0 {
		return nil
	}
	if body[base] == '"' && !json.Valid([]byte(strings.TrimSpace(body[base:]))) {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(body[base:]))
	dec.UseNumber()
	type frame struct {
		object, wantKey bool
		field, member   string
	}
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
		field := ""
		if top >= 0 {
			field = stack[top].field
			if stack[top].object {
				field = stack[top].member
			}
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, frame{object: true, wantKey: true, field: field})
			case '[':
				stack = append(stack, frame{field: field})
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
			isKey := top >= 0 && stack[top].object && stack[top].wantKey
			if isKey {
				stack[top].wantKey = false
				stack[top].member = v
			}
			if q := strings.IndexByte(body[base+int(prev):base+int(end)], '"'); q >= 0 {
				leaves = append(leaves, jsonLeaf{start: base + int(prev) + q, end: base + int(end), value: v, key: isKey, field: field})
			}
			if !isKey && top >= 0 && stack[top].object {
				stack[top].wantKey = true
			}
		default:
			if top >= 0 && stack[top].object {
				stack[top].wantKey = true
			}
		}
	}
}

// joinJSONLeaves joins the non-empty values (or, with keys, the distinct object
// keys) with newlines, the view in which a line break the JSON encoder escaped
// is a real line break again. Keys are a view of their own: a key and its value
// are not a sentence, and a list of records repeats the same few keys.
func joinJSONLeaves(leaves []jsonLeaf, keys bool) string {
	var b strings.Builder
	var seen map[string]bool
	if keys {
		seen = make(map[string]bool)
	}
	for _, leaf := range leaves {
		if leaf.key != keys || leaf.value == "" {
			continue
		}
		if keys {
			if seen[leaf.value] {
				continue
			}
			seen[leaf.value] = true
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(leaf.value)
	}
	return b.String()
}

// neutralizeLeafRoleMarkers applies the role-marker rewrite to every decoded
// value and splices only the changed tokens back, so the rest of the body stays
// byte-identical. Keys are structure and stay as they are.
func neutralizeLeafRoleMarkers(body string, leaves []jsonLeaf) string {
	var b strings.Builder
	last := 0
	for _, leaf := range leaves {
		if leaf.key {
			continue
		}
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
	a.Incomplete = a.Incomplete || b.Incomplete
	a.signal = a.signal || b.signal
	return a
}
