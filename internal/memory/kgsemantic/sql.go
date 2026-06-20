package kgsemantic

import "fmt"

// EdgeDirtyCondition returns SQL that marks edges needing semantic reindexing.
func EdgeDirtyCondition(edgeAlias string) string {
	return fmt.Sprintf(`(
		%s.semantic_indexed_at IS NULL
		OR %s.semantic_indexed_at < COALESCE(%s.updated_at, '1970-01-01')
		OR EXISTS (
			SELECT 1 FROM kg_nodes n
			WHERE (n.id = %s.source OR n.id = %s.target)
			  AND n.updated_at > COALESCE(%s.semantic_indexed_at, '1970-01-01')
		)
	)`, edgeAlias, edgeAlias, edgeAlias, edgeAlias, edgeAlias, edgeAlias)
}