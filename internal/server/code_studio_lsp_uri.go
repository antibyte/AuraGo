package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// The browser always speaks file:///workspace URIs (lsp-protocol.js). A
// language server on an SSH host sees the real folder. The rewriter maps the
// URIs of one session in both directions on the JSON string tokens of a
// message: object keys (WorkspaceEdit.changes is keyed by URI) and values.
// Every other byte is copied, so key order, numbers and file texts stay as
// they were.

const (
	codeStudioLSPPageRootURI = "file:///workspace"
	// codeStudioLSPRewriteMaxLiteral skips long string tokens: URIs are short,
	// file texts (didOpen, didChange, newText) are long and never rewritten.
	codeStudioLSPRewriteMaxLiteral = 8 << 10
)

type codeStudioLSPURIRewriter struct {
	// roots are absolute remote folders without a trailing slash ("" is /).
	// roots[0] is the folder used towards the server; the others (the
	// symlink-resolved folder) are only recognised in server messages.
	roots []string
}

// newCodeStudioLSPURIRewriter returns nil when no root is absolute.
func newCodeStudioLSPURIRewriter(roots ...string) *codeStudioLSPURIRewriter {
	r := &codeStudioLSPURIRewriter{}
	seen := map[string]bool{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if !strings.HasPrefix(root, "/") {
			continue
		}
		root = strings.TrimRight(root, "/")
		if seen[root] {
			continue
		}
		seen[root] = true
		r.roots = append(r.roots, root)
	}
	if len(r.roots) == 0 {
		return nil
	}
	return r
}

// toServer maps page URIs (file:///workspace/...) and the plain rootPath
// "/workspace" to the remote folder.
func (r *codeStudioLSPURIRewriter) toServer(msg []byte) []byte {
	if r == nil || !bytes.Contains(msg, []byte("/workspace")) {
		return msg
	}
	return rewriteCodeStudioLSPStrings(msg, r.serverString)
}

// toPage maps server URIs under one of the roots back to file:///workspace.
func (r *codeStudioLSPURIRewriter) toPage(msg []byte) []byte {
	if r == nil {
		return msg
	}
	return rewriteCodeStudioLSPStrings(msg, r.pageString)
}

func (r *codeStudioLSPURIRewriter) serverPath(rest string) string {
	if p := r.roots[0] + rest; p != "" {
		return p
	}
	return "/"
}

func (r *codeStudioLSPURIRewriter) serverString(s string) (string, bool) {
	if s == "/workspace" {
		return r.serverPath(""), true
	}
	if s != codeStudioLSPPageRootURI && !strings.HasPrefix(s, codeStudioLSPPageRootURI+"/") {
		return "", false
	}
	pathPart, suffix := splitCodeStudioURISuffix(s[len(codeStudioLSPPageRootURI):])
	decoded, err := url.PathUnescape(pathPart)
	if err != nil {
		return "", false
	}
	return "file://" + codeStudioEncodeURIPath(r.serverPath(decoded)) + suffix, true
}

func (r *codeStudioLSPURIRewriter) pageString(s string) (string, bool) {
	const scheme = "file://"
	if len(s) < len(scheme) || !strings.EqualFold(s[:len(scheme)], scheme) {
		return "", false
	}
	rest := s[len(scheme):]
	if len(rest) > len("localhost") && strings.EqualFold(rest[:len("localhost")], "localhost") && rest[len("localhost")] == '/' {
		rest = rest[len("localhost"):]
	}
	if !strings.HasPrefix(rest, "/") {
		return "", false
	}
	pathPart, suffix := splitCodeStudioURISuffix(rest)
	decoded, err := url.PathUnescape(pathPart)
	if err != nil {
		return "", false
	}
	for _, root := range r.roots {
		if decoded == root || strings.HasPrefix(decoded, root+"/") {
			return codeStudioLSPPageRootURI + codeStudioEncodeURIPath(decoded[len(root):]) + suffix, true
		}
	}
	return "", false
}

func splitCodeStudioURISuffix(rest string) (string, string) {
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		return rest[:i], rest[i:]
	}
	return rest, ""
}

// codeStudioEncodeURIPath percent-encodes every byte of each segment except
// the RFC 3986 unreserved set, the way lsp-protocol.js uriForPath does
// (encodeURIComponent plus !'()*).
func codeStudioEncodeURIPath(p string) string {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		var b strings.Builder
		for j := 0; j < len(segment); j++ {
			c := segment[j]
			if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
				b.WriteByte(c)
				continue
			}
			fmt.Fprintf(&b, "%%%02X", c)
		}
		segments[i] = b.String()
	}
	return strings.Join(segments, "/")
}

// rewriteCodeStudioLSPStrings walks msg: every JSON string literal (object
// keys included) whose decoded value maps through fn is replaced; everything
// else is copied byte for byte. Malformed input comes back unchanged.
func rewriteCodeStudioLSPStrings(msg []byte, fn func(string) (string, bool)) []byte {
	var out []byte // nil until the first change
	last := 0
	for i := 0; i < len(msg); i++ {
		if msg[i] != '"' {
			continue
		}
		end := i + 1
		for end < len(msg) && msg[end] != '"' {
			if msg[end] == '\\' {
				end++
			}
			end++
		}
		if end >= len(msg) {
			return msg
		}
		literal := msg[i : end+1]
		if len(literal) <= codeStudioLSPRewriteMaxLiteral && codeStudioLSPMaybeURI(literal) {
			var value string
			if json.Unmarshal(literal, &value) == nil {
				if mapped, ok := fn(value); ok {
					if encoded, err := codeStudioLSPEncodeString(mapped); err == nil {
						if out == nil {
							out = make([]byte, 0, len(msg)+64)
						}
						out = append(out, msg[last:i]...)
						out = append(out, encoded...)
						last = end + 1
					}
				}
			}
		}
		i = end
	}
	if out == nil {
		return msg
	}
	return append(out, msg[last:]...)
}

// codeStudioLSPMaybeURI is a cheap pre-check on the raw literal: "file:..." in
// any case, or the plain rootPath "/workspace".
func codeStudioLSPMaybeURI(literal []byte) bool {
	if len(literal) >= 7 && bytes.EqualFold(literal[1:6], []byte("file:")) {
		return true
	}
	return string(literal) == `"/workspace"`
}

func codeStudioLSPEncodeString(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
