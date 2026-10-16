package codegraph

import (
	"context"
	"path"
	"strings"
)

func resolve(ctx context.Context, files map[string]*fileIndex, module string) ([]Symbol, []Edge) {
	var symbols []Symbol
	var edges []Edge
	byID := map[string]Symbol{}
	for _, f := range files {
		for _, s := range f.symbols {
			symbols = append(symbols, s)
			byID[s.ID] = s
		}
	}
	for _, f := range files {
		for _, original := range f.edges {
			if ctx.Err() != nil {
				return nil, nil
			}
			e := original
			candidates := []Symbol{}
			shadowed := false
			for _, b := range f.bindings {
				if (b.name == e.Name || (e.receiver != "" && b.name == e.receiver)) && e.offset >= b.start && e.offset < b.end {
					shadowed = true
					break
				}
			}
			if e.receiver == "" && !shadowed {
				owner := e.From
				for {
					for _, s := range f.symbols {
						if s.Name == e.Name && s.Owner == owner && s.Kind != "method" && byID[owner].Kind != "class" {
							candidates = append(candidates, s)
						}
					}
					if len(candidates) > 0 || owner == "" {
						break
					}
					owner = byID[owner].Owner
				}
				if len(candidates) == 0 && f.language == "go" {
					for _, other := range files {
						if other.language == "go" && other.pkg == f.pkg && path.Dir(other.path) == path.Dir(f.path) {
							for _, s := range other.symbols {
								if s.Name == e.Name && s.Owner == "" && s.Kind != "method" {
									candidates = append(candidates, s)
								}
							}
						}
					}
				}
			}
			if len(candidates) == 0 && !shadowed {
				for _, imp := range f.imports {
					name := e.Name
					if e.receiver != "" {
						if !imp.namespace || imp.alias != e.receiver {
							continue
						}
					} else {
						if imp.namespace || imp.alias != e.Name {
							continue
						}
						name = imp.name
					}
					for _, target := range importedFiles(files, f, imp.module, module) {
						resolvedName := name
						if target.language != "go" && target.language != "python" {
							resolvedName = target.exports[name]
							if resolvedName == "" {
								continue
							}
						}
						for _, s := range target.symbols {
							if s.Name == resolvedName && s.Owner == "" && s.Kind != "method" {
								candidates = append(candidates, s)
							}
						}
					}
				}
			}
			if len(candidates) == 1 {
				e.To = candidates[0].ID
			} else if len(candidates) > 1 {
				e.Reason = "ambiguous"
			} else if shadowed {
				e.Reason = "local_binding"
			} else {
				e.Reason = "dynamic_or_external"
			}
			edges = append(edges, e)
		}
	}
	return symbols, edges
}

func importedFiles(files map[string]*fileIndex, from *fileIndex, imp, module string) []*fileIndex {
	var result []*fileIndex
	switch from.language {
	case "go":
		if module == "" || !strings.HasPrefix(imp, module+"/") {
			return result
		}
		dir := strings.TrimPrefix(imp, module+"/")
		for _, f := range files {
			if f.language == "go" && path.Dir(f.path) == dir && !strings.HasSuffix(f.path, "_test.go") {
				result = append(result, f)
			}
		}
	case "python":
		dir := ""
		if strings.HasPrefix(imp, ".") {
			dir = path.Dir(from.path)
			imp = strings.TrimPrefix(imp, ".")
			for strings.HasPrefix(imp, ".") {
				dir = path.Dir(dir)
				imp = strings.TrimPrefix(imp, ".")
			}
		}
		name := path.Join(dir, strings.ReplaceAll(imp, ".", "/"))
		for _, p := range []string{name + ".py", path.Join(name, "__init__.py")} {
			if f := files[p]; f != nil {
				result = append(result, f)
			}
		}
	default:
		if !strings.HasPrefix(imp, ".") {
			return result
		}
		name := path.Clean(path.Join(path.Dir(from.path), imp))
		if strings.HasPrefix(name, "../") {
			return result
		}
		paths := []string{name}
		for _, ext := range []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"} {
			paths = append(paths, name+ext, path.Join(name, "index"+ext))
		}
		for _, p := range paths {
			if f := files[p]; f != nil {
				result = append(result, f)
			}
		}
	}
	return result
}
