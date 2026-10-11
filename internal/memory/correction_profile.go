package memory

import (
	"context"
	"fmt"
)

func checkCorrectionProfileCapacity(ctx context.Context, q correctionQueryer, category string) error {
	var total, inCategory int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(category=?),0) FROM user_profile`, category).Scan(&total, &inCategory); err != nil {
		return err
	}
	if total >= maxTotalProfileEntries || inCategory >= maxProfileEntriesPerCategory {
		return fmt.Errorf("profile capacity reached; review entries before restoring an archived field")
	}
	return nil
}
