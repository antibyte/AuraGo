package security

import (
	"encoding/json"
	"strings"
)

// These values remain data, scanned with every other field before transformation.
func datamarkTechnicalField(field string) bool {
	identifier := strings.HasSuffix(field, "_id") || strings.HasSuffix(field, "_ids") || strings.HasSuffix(field, "Id") || strings.HasSuffix(field, "Ids") || strings.HasSuffix(field, "ID") || strings.HasSuffix(field, "IDs")
	field = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(field, "_", ""), "-", ""))
	if identifier || field == "id" || field == "ids" {
		return true
	}
	for _, suffix := range []string{"path", "paths", "url", "urls", "uri", "uris", "filename", "filenames"} {
		if strings.HasSuffix(field, suffix) {
			return true
		}
	}
	switch field {
	case "status", "operation", "code", "command", "script", "selector", "key", "keys", "name", "names", "file", "files", "ref", "outputref", "from", "to", "cc", "bcc", "email", "emailaddress", "downloadedfile", "downloadname", "filterused", "view", "mimetype", "contenttype":
		return true
	}
	return false
}

// Repeated gates and archived JSON-in-JSON must preserve the same machine fields.
// Envelopes affect formatting only: the entire original was already scanned.
func datamarkContent(text string, depth int) (string, bool) {
	if depth > 32 {
		return "", false
	}
	trim := strings.TrimLeft(text, " \t\r\n")
	if trim == DatamarkNotice {
		return text, true
	}
	for _, label := range []string{"Tool Output:", "[Tool Output]"} {
		if strings.HasPrefix(trim, label) {
			n := len(text) - len(trim) + len(label)
			marked, ok := datamarkContent(text[n:], depth+1)
			return text[:n] + marked, ok
		}
	}
	const open, close = "<external_data>\n", "\n</external_data>"
	if strings.HasPrefix(trim, open) {
		if end := strings.Index(trim, close); end >= len(open) {
			payload, _ := IsolatedPayload(trim[len(open):end])
			marked, ok := datamarkContent(payload, depth+1)
			tail, tailOK := datamarkContent(trim[end+len(close):], depth+1)
			return text[:len(text)-len(trim)] + IsolateExternalData(marked) + tail, ok && tailOK
		}
	}
	leaves := jsonStringLeaves(text)
	if len(leaves) == 0 && jsonBodyStart(text) < 0 {
		return Datamark(text), true
	}
	dec := json.NewDecoder(strings.NewReader(text))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		if len(leaves) == 0 {
			return Datamark(text), true
		}
		return "", false
	}
	end := int(dec.InputOffset())
	leaves = jsonStringLeaves(text[:end])
	var out strings.Builder
	last := 0
	for _, leaf := range leaves {
		if leaf.key || datamarkTechnicalField(leaf.field) {
			continue
		}
		marked, ok := datamarkContent(leaf.value, depth+1)
		if !ok {
			return "", false
		}
		if marked != leaf.value {
			encoded, _ := json.Marshal(marked)
			out.WriteString(text[last:leaf.start])
			out.Write(encoded)
			last = leaf.end
		}
	}
	tail, ok := datamarkContent(text[end:], depth+1)
	out.WriteString(text[last:end])
	out.WriteString(tail)
	return out.String(), ok
}
