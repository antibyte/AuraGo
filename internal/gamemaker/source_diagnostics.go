package gamemaker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source maps live only with the current build in memory. Keep the map and its
// embedded source snapshots out of the project filesystem and exported archive.
type buildSourceMap struct {
	root     string
	sources  []string
	contents []string
	lines    [][]sourceMapping
}

type sourceMapping struct{ column, source, line, originalColumn int }

type RuntimeFrame struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func decodeBuildSourceMap(root string, data []byte) (*buildSourceMap, error) {
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("source map exceeds private diagnostic limit")
	}
	var raw struct {
		Version        int
		Sources        []string
		SourcesContent []string
		Mappings       string
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Version != 3 || len(raw.Sources) != len(raw.SourcesContent) {
		return nil, fmt.Errorf("invalid source map")
	}
	result := &buildSourceMap{root: root, sources: raw.Sources, contents: raw.SourcesContent}
	for i, source := range raw.Sources {
		rel := filepath.ToSlash(filepath.Clean(filepath.Join("dist", filepath.FromSlash(source))))
		if safe, err := safeRelativePath(rel, false); err == nil && strings.HasPrefix(safe, "src/") {
			result.sources[i] = safe
		} else {
			result.sources[i] = ""
		}
	}
	source, line, column := 0, 0, 0
	for _, encodedLine := range strings.Split(raw.Mappings, ";") {
		var mappings []sourceMapping
		generated := 0
		for _, segment := range strings.Split(encodedLine, ",") {
			if segment == "" {
				continue
			}
			values, err := decodeSourceVLQ(segment)
			if err != nil {
				return nil, err
			}
			if len(values) != 1 && len(values) != 4 && len(values) != 5 {
				return nil, fmt.Errorf("invalid source map segment")
			}
			generated += values[0]
			if generated < 0 || len(mappings) > 0 && generated < mappings[len(mappings)-1].column {
				return nil, fmt.Errorf("unordered source map")
			}
			if len(values) == 1 {
				mappings = append(mappings, sourceMapping{column: generated, source: -1})
				continue
			}
			source += values[1]
			line += values[2]
			column += values[3]
			if source < 0 || source >= len(raw.Sources) || line < 0 || column < 0 {
				return nil, fmt.Errorf("invalid source map location")
			}
			mappings = append(mappings, sourceMapping{generated, source, line, column})
		}
		result.lines = append(result.lines, mappings)
	}
	return result, nil
}

func decodeSourceVLQ(segment string) ([]int, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out []int
	value, shift := 0, 0
	for _, r := range segment {
		digit := strings.IndexRune(alphabet, r)
		if digit < 0 || shift > 25 {
			return nil, fmt.Errorf("invalid source map VLQ")
		}
		value |= (digit & 31) << shift
		if digit&32 != 0 {
			shift += 5
			continue
		}
		n := value >> 1
		if value&1 != 0 {
			n = -n
		}
		out = append(out, n)
		value, shift = 0, 0
	}
	if shift != 0 {
		return nil, fmt.Errorf("incomplete source map VLQ")
	}
	return out, nil
}

func (m *buildSourceMap) diagnostic(message string, frames []RuntimeFrame) Diagnostic {
	d := Diagnostic{Level: "runtime", Message: message}
	if m == nil {
		return d
	}
	for _, frame := range frames[:min(5, len(frames))] {
		if frame.File != "dist/game.js" || frame.Line < 1 || frame.Line > len(m.lines) || frame.Column < 1 || frame.Column > 10000000 {
			continue
		}
		line := m.lines[frame.Line-1]
		i := sort.Search(len(line), func(i int) bool { return line[i].column > frame.Column-1 }) - 1
		if i < 0 || line[i].source < 0 {
			continue
		}
		location := line[i]
		path := m.sources[location.source]
		if path == "" {
			continue
		}
		full, _, err := secureJoin(m.root, path, false)
		if err != nil {
			continue
		}
		current, err := os.ReadFile(full)
		if err != nil || sourceHash(string(current)) != sourceHash(m.contents[location.source]) {
			continue
		}
		d.File, d.Line, d.Column = path, location.line+1, location.originalColumn+1
		d.SourceSHA256 = sourceHash(string(current))
		d.Excerpt = sourceExcerpt(string(current), d.Line)
		return d
	}
	return d
}

func sourceExcerpt(content string, line int) string {
	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	var out []string
	for i := max(0, line-2); i < min(len(lines), line+1); i++ {
		runes := []rune(lines[i])
		out = append(out, fmt.Sprintf("%d: %s", i+1, string(runes[:min(400, len(runes))])))
	}
	return strings.Join(out, "\n")
}
