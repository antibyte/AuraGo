package codegraph

import "context"

// KnownWriteSymbol refines a line-bound advisory only when an existing snapshot
// still matches the file. It never starts indexing as a side effect of a write.
func (m *Manager) KnownWriteSymbol(ctx context.Context, p Project, file string, first, last int) string {
	if m == nil || first < 1 || !m.authorized(p) || ctx.Err() != nil {
		return ""
	}
	if last < first {
		last = first
	}
	m.mu.RLock()
	s := m.projects[p.ID]
	m.mu.RUnlock()
	if s == nil || s.project != p {
		return ""
	}
	f := s.files[file]
	if f == nil {
		return ""
	}
	hash, err := Fingerprint(p, file)
	if err != nil || hash != f.hash {
		return ""
	}
	found := ""
	width := int(^uint(0) >> 1)
	for _, symbol := range f.symbols {
		if symbol.Line <= first && symbol.EndLine >= last {
			size := int(symbol.End - symbol.Start)
			if size < width {
				found, width = symbol.ID, size
			} else if size == width {
				found = ""
			}
		}
	}
	if !m.authorized(p) || ctx.Err() != nil {
		return ""
	}
	return found
}
