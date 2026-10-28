package dockerstacks

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	restartPattern  = regexp.MustCompile(`^(no|always|unless-stopped|on-failure(:\d+)?)$`)
	bytesPattern    = regexp.MustCompile(`^\d+(\.\d+)?[bkmgBKMG]?$`)
	durationPattern = regexp.MustCompile(`^(0|(\d+(\.\d+)?(ns|us|ms|s|m|h))+)$`)
	pwdPattern      = regexp.MustCompile(`(?i)\$(?:\(pwd\)|\{pwd\}|pwd\b)` + "|`pwd`")
	userIDPattern   = regexp.MustCompile(`^\d+:\d+$`)
)

// ---- YAML node helpers: nodes (not maps) keep the key order of a compose file readable ----

func strNode(v string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
	switch strings.ToLower(v) {
	case "y", "n", "yes", "no", "on", "off": // YAML 1.1 tools read these as booleans
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

func quotedNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v, Style: yaml.DoubleQuotedStyle}
}

func quotedNodes(values []string) []*yaml.Node {
	out := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		out = append(out, quotedNode(v))
	}
	return out
}

func boolNode(b bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(b)}
}

func nullNode() *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"} }

// numberNode writes digits (and a decimal point) as a YAML number and anything else as a string.
func numberNode(v string) *yaml.Node {
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: v}
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil && !strings.ContainsAny(v, "eEnN+") {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: v}
	}
	return strNode(v)
}

func seqNode(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items}
}

func flowSeqNode(items ...*yaml.Node) *yaml.Node {
	n := seqNode(items...)
	n.Style = yaml.FlowStyle
	return n
}

func strSeq(values []string, quote bool) *yaml.Node {
	n := seqNode()
	for _, v := range values {
		if quote {
			n.Content = append(n.Content, quotedNode(v))
		} else {
			n.Content = append(n.Content, strNode(v))
		}
	}
	return n
}

// mapping is a YAML mapping node that is filled in the order the keys are added.
type mapping struct{ n *yaml.Node }

func newMapping() *mapping { return &mapping{&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}} }

func (m *mapping) add(key string, val *yaml.Node) { m.set(key, val) }

// set adds key, or replaces its value when the key is already there (the key keeps its place).
func (m *mapping) set(key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.n.Content); i += 2 {
		if m.n.Content[i].Value == key {
			m.n.Content[i+1] = val
			return
		}
	}
	m.n.Content = append(m.n.Content, strNode(key), val)
}

func (m *mapping) empty() bool { return len(m.n.Content) == 0 }

// ---- names ----

// sanitizeServiceName makes s fit the stack name rules ^[a-z0-9][a-z0-9_-]{0,62}$ ("app" when nothing is left).
func sanitizeServiceName(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-':
			b.WriteRune(c)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.TrimLeft(b.String(), "_-")
	if len(out) > 63 {
		out = out[:63]
	}
	if out == "" {
		return "app"
	}
	return out
}

// serviceNameFromImage is the last path segment of an image reference without its tag or digest.
func serviceNameFromImage(image string) string {
	if i := strings.IndexByte(image, '@'); i >= 0 {
		image = image[:i]
	}
	last := image[strings.LastIndexByte(image, '/')+1:]
	if i := strings.IndexByte(last, ':'); i >= 0 {
		last = last[:i]
	}
	return sanitizeServiceName(last)
}
