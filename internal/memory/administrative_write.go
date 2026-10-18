package memory

import (
	"context"
	"errors"
	"fmt"
)

// StoreAdministrativeMemoryDocument preserves the explicit administrative write
// path while publishing its corpus and tracking metadata as one snapshot unit.
func StoreAdministrativeMemoryDocument(ctx context.Context, stm *SQLiteMemory, vector VectorDB, concept, content string) (ids []string, resultErr error) {
	if stm == nil || vector == nil {
		return nil, fmt.Errorf("memory stores are required")
	}
	ctx, publication := prepareMemoryPublication(ctx, stm, nil)
	complete := false
	defer func() { resultErr = errors.Join(resultErr, publication.finish(complete)) }()
	if _, native := vector.(*ChromemVectorDB); !native {
		if err := startMemoryPublication(ctx); err != nil {
			return nil, err
		}
	}
	stored, err := StoreDocumentWithOwnershipContext(ctx, vector, concept, content)
	ids = append(append(append([]string{}, stored.CreatedIDs...), stored.ReusedIDs...), stored.UnknownIDs...)
	if err != nil {
		return ids, err
	}
	for _, id := range ids {
		if err := stm.UpsertMemoryMeta(id); err != nil {
			return ids, err
		}
	}
	complete = true
	return ids, nil
}
