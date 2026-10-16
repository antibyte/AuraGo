package codegraph

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	ts "github.com/odvcencio/gotreesitter"
	golang "github.com/odvcencio/gotreesitter/grammars/go"
	"github.com/odvcencio/gotreesitter/grammars/javascript"
	"github.com/odvcencio/gotreesitter/grammars/python"
	"github.com/odvcencio/gotreesitter/grammars/tsx"
	"github.com/odvcencio/gotreesitter/grammars/typescript"
)

const MaxFileBytes = 1 << 20
const MaxIndexBytes = 256 << 20

type Symbol struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	EndLine int    `json:"end_line"`
	Start   uint32 `json:"start_byte"`
	End     uint32 `json:"end_byte"`
	Owner   string `json:"owner,omitempty"`
	Snippet string `json:"snippet"`
}

type Edge struct {
	From     string `json:"from"`
	To       string `json:"to,omitempty"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Reason   string `json:"unresolved_reason,omitempty"`
	receiver string
	offset   uint32
}

type fileIndex struct {
	path, language, hash, source, pkg string
	symbols                           []Symbol
	edges                             []Edge
	imports                           []importBinding
	bindings                          []binding
	exports                           map[string]string
}

type importBinding struct {
	alias, name, module string
	namespace           bool
}
type binding struct {
	name       string
	start, end uint32
}

func languageFor(file string) *ts.Language {
	switch strings.ToLower(path.Ext(file)) {
	case ".go":
		return golang.Language()
	case ".js", ".jsx", ".mjs", ".cjs":
		return javascript.Language()
	case ".ts", ".mts", ".cts":
		return typescript.Language()
	case ".tsx":
		return tsx.Language()
	case ".py":
		return python.Language()
	}
	return nil
}

func supported(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".go", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".tsx", ".py":
		return true
	}
	return false
}

func point(source string, offset uint32) (int, int) {
	if int(offset) > len(source) {
		offset = uint32(len(source))
	}
	prefix := source[:offset]
	line := strings.Count(prefix, "\n") + 1
	last := strings.LastIndexByte(prefix, '\n')
	return line, utf8.RuneCountInString(prefix[last+1:]) + 1
}

func parseFile(ctx context.Context, file, source, hash string) (*fileIndex, error) {
	if len(source) > MaxFileBytes {
		return nil, fmt.Errorf("file_limit")
	}
	lang := languageFor(file)
	if lang == nil {
		return nil, fmt.Errorf("unsupported_language")
	}
	sourceBytes := []byte(source)
	p := ts.NewParser(lang)
	p.SetTimeoutMicros(250000)
	p.SetMemoryBudgetBytes(64 << 20)
	var cancelled uint32
	p.SetCancellationFlag(&cancelled)
	stop := context.AfterFunc(ctx, func() { atomic.StoreUint32(&cancelled, 1) })
	defer stop()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	tree, err := p.ParseStrict(sourceBytes)
	if tree != nil {
		defer tree.Release()
	}
	if err != nil {
		return nil, fmt.Errorf("parse_incomplete")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if tree == nil || tree.RootNode() == nil || tree.RootNode().HasErrorOrMissing() {
		return nil, fmt.Errorf("syntax_error")
	}
	// v0.56.0 can recover an unfinished JS declaration as an empty program
	// without propagating its error bit. Nonempty source must have syntax nodes.
	if tree.RootNode().ChildCount() == 0 && strings.TrimSpace(source) != "" {
		return nil, fmt.Errorf("empty_parse")
	}
	program, err := ts.NewFactProgram(lang, ts.FactAll)
	if err != nil {
		return nil, fmt.Errorf("extractor_unavailable")
	}
	facts := program.Extract(tree)
	if len(facts.Definitions) > 4000 || len(facts.Calls) > 20000 || len(facts.Imports) > 4000 {
		return nil, fmt.Errorf("fact_limit")
	}
	f := &fileIndex{path: file, language: lang.Name, source: source, hash: hash, exports: map[string]string{}}
	type exportRange struct {
		start, end uint32
		isDefault  bool
	}
	var exported []exportRange
	for _, d := range facts.Definitions {
		f.addSymbol(d.Name, d.Kind, d.StartByte, d.EndByte)
	}
	// The shared fact extractor deliberately omits JS import bindings and bound arrows.
	var walk func(*ts.Node, uint32, uint32)
	walk = func(n *ts.Node, scopeStart, scopeEnd uint32) {
		if n == nil || ctx.Err() != nil {
			return
		}
		kind := n.Type(lang)
		if kind == "function_declaration" || kind == "function_definition" || kind == "method_declaration" || kind == "method_definition" || kind == "arrow_function" {
			scopeStart, scopeEnd = n.StartByte(), n.EndByte()
		}
		field := func(name string) *ts.Node { return n.ChildByFieldName(name, lang) }
		text := func(node *ts.Node) string {
			if node == nil {
				return ""
			}
			return node.Text(sourceBytes)
		}
		if kind == "package_clause" {
			f.pkg = strings.TrimSpace(strings.TrimPrefix(n.Text(sourceBytes), "package"))
		}
		if kind == "export_statement" && field("source") == nil {
			if declaration := field("declaration"); declaration != nil {
				exported = append(exported, exportRange{declaration.StartByte(), declaration.EndByte(), strings.HasPrefix(strings.TrimSpace(n.Text(sourceBytes)), "export default ")})
			}
		}
		if kind == "export_specifier" {
			name, alias := text(field("name")), text(field("alias"))
			if alias == "" {
				alias = name
			}
			f.exports[alias] = name
		}
		if kind == "short_var_declaration" || kind == "assignment" || kind == "var_spec" {
			left := field("left")
			if left == nil {
				left = field("name")
			}
			var collect func(*ts.Node)
			collect = func(node *ts.Node) {
				if node == nil {
					return
				}
				if node.Type(lang) == "identifier" {
					f.bindings = append(f.bindings, binding{text(node), scopeStart, scopeEnd})
					return
				}
				if node.Type(lang) == "expression_list" || node.Type(lang) == "pattern_list" || node.Type(lang) == "tuple_pattern" {
					for i := 0; i < node.NamedChildCount(); i++ {
						collect(node.NamedChild(i))
					}
				}
			}
			collect(left)
		}
		if kind == "variable_declarator" {
			value := field("value")
			if value != nil && (value.Type(lang) == "arrow_function" || value.Type(lang) == "function_expression") {
				f.addSymbol(text(field("name")), "function", n.StartByte(), n.EndByte())
			} else {
				f.bindings = append(f.bindings, binding{text(field("name")), scopeStart, scopeEnd})
			}
		}
		if kind == "parameter" || kind == "required_parameter" || kind == "optional_parameter" || kind == "parameter_declaration" || kind == "default_parameter" {
			name := text(field("name"))
			if name == "" {
				name = text(field("pattern"))
			}
			if name != "" {
				f.bindings = append(f.bindings, binding{name, scopeStart, scopeEnd})
			}
		}
		if kind == "parameters" || kind == "formal_parameters" {
			for i := 0; i < n.NamedChildCount(); i++ {
				child := n.NamedChild(i)
				if child.Type(lang) == "identifier" {
					f.bindings = append(f.bindings, binding{child.Text(sourceBytes), scopeStart, scopeEnd})
				}
			}
		}
		if kind == "import_statement" && (lang.Name == "javascript" || lang.Name == "typescript" || lang.Name == "tsx") {
			module := strings.Trim(text(field("source")), "\"'")
			var imports func(*ts.Node)
			imports = func(node *ts.Node) {
				if node == nil {
					return
				}
				switch node.Type(lang) {
				case "import_specifier":
					name := text(node.ChildByFieldName("name", lang))
					alias := text(node.ChildByFieldName("alias", lang))
					if alias == "" {
						alias = name
					}
					f.imports = append(f.imports, importBinding{alias: alias, name: name, module: module})
					return
				case "namespace_import":
					if node.NamedChildCount() > 0 {
						f.imports = append(f.imports, importBinding{alias: text(node.NamedChild(node.NamedChildCount() - 1)), module: module, namespace: true})
					}
					return
				case "import_clause":
					for i := 0; i < node.NamedChildCount(); i++ {
						child := node.NamedChild(i)
						if child.Type(lang) == "identifier" {
							f.imports = append(f.imports, importBinding{alias: text(child), name: "default", module: module})
						} else {
							imports(child)
						}
					}
					return
				}
				for i := 0; i < node.NamedChildCount(); i++ {
					imports(node.NamedChild(i))
				}
			}
			imports(n)
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			walk(n.NamedChild(i), scopeStart, scopeEnd)
		}
	}
	walk(tree.RootNode(), 0, uint32(len(source)))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	for i := range f.symbols {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s := &f.symbols[i]
		owner := f.enclosing(s.Start, s.End)
		s.Owner = owner
		if owner == "" {
			for _, e := range exported {
				if s.Start >= e.start && s.End <= e.end {
					name := s.Name
					if e.isDefault {
						name = "default"
					}
					f.exports[name] = s.Name
				}
			}
		}
	}
	for _, i := range facts.Imports {
		if i.Kind == "package" {
			continue
		}
		module := i.Path
		if i.From != "" {
			module = i.From
		}
		alias := i.Alias
		if alias == "" {
			alias = i.Name
		}
		if alias == "" {
			alias = path.Base(module)
		}
		f.imports = append(f.imports, importBinding{alias: alias, name: i.Name, module: strings.Repeat(".", i.Relative) + module, namespace: i.From == ""})
	}
	for _, c := range facts.Calls {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		line, _ := point(source, c.StartByte)
		f.edges = append(f.edges, Edge{From: f.enclosing(c.StartByte, c.EndByte), Name: c.Name, Kind: "call", File: file, Line: line, receiver: c.Receiver, offset: c.StartByte})
	}
	for _, h := range facts.Heritage {
		line, _ := point(source, h.StartByte)
		owner := ""
		for _, s := range f.symbols {
			if s.Name == h.Name {
				owner = s.ID
				break
			}
		}
		f.edges = append(f.edges, Edge{From: owner, Name: h.Parent, Kind: "inherits", File: file, Line: line, offset: h.StartByte})
	}
	for i := range f.edges {
		if f.edges[i].From == "" {
			f.edges[i].From = "file:" + file
		}
	}
	return f, nil
}

func (f *fileIndex) addSymbol(name, kind string, start, end uint32) {
	if name == "" || end > uint32(len(f.source)) || start >= end {
		return
	}
	for _, s := range f.symbols {
		if s.Name == name && s.Start == start {
			return
		}
	}
	line, col := point(f.source, start)
	endLine, _ := point(f.source, end)
	snippet := f.source[start:end]
	if len(snippet) > 180 {
		snippet = snippet[:180]
		for !utf8.ValidString(snippet) {
			snippet = snippet[:len(snippet)-1]
		}
	}
	f.symbols = append(f.symbols, Symbol{ID: fmt.Sprintf("%s:%d:%s", f.path, start, name), Name: name, Kind: kind, File: f.path, Line: line, Column: col, EndLine: endLine, Start: start, End: end, Snippet: snippet})
}

func (f *fileIndex) enclosing(start, end uint32) string {
	owner := ""
	width := uint32(len(f.source) + 1)
	for _, s := range f.symbols {
		if s.Start <= start && s.End >= end && (s.Start != start || s.End != end) && s.End-s.Start < width {
			owner = s.ID
			width = s.End - s.Start
		}
	}
	return owner
}
