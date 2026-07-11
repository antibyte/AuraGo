package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

const defaultInClauseChunkSize = 999

// chunkedRowsAffected implements sql.Result for operations executed in chunks.
type chunkedRowsAffected struct {
	total int64
}

func (r chunkedRowsAffected) LastInsertId() (int64, error) { return 0, nil }
func (r chunkedRowsAffected) RowsAffected() (int64, error) { return r.total, nil }

// execChunkedInDelete runs DELETE ... WHERE <column> IN (...) in chunks to stay
// within SQLite host-parameter limits and avoid parser overhead for huge slices.
func execChunkedInDelete(tx *sql.Tx, table, column string, ids []int64, chunkSize int) error {
	_, err := execChunkedInDeleteResult(tx, table, column, ids, chunkSize)
	return err
}

// execChunkedInDeleteResult is like execChunkedInDelete but returns the combined
// sql.Result with the summed RowsAffected across all chunks.
func execChunkedInDeleteResult(tx *sql.Tx, table, column string, ids []int64, chunkSize int) (sql.Result, error) {
	if chunkSize <= 0 {
		chunkSize = defaultInClauseChunkSize
	}
	if len(ids) == 0 {
		return chunkedRowsAffected{}, nil
	}
	if !validIdentifier(table) {
		return nil, fmt.Errorf("invalid table identifier %q", table)
	}
	if !validIdentifier(column) {
		return nil, fmt.Errorf("invalid column identifier %q", column)
	}
	var total int64
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", quoteIdentifier(table), quoteIdentifier(column), strings.Join(placeholders, ","))
		res, err := tx.Exec(query, args...)
		if err != nil {
			return nil, fmt.Errorf("delete %s by %s chunk %d-%d: %w", table, column, start, end-1, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return chunkedRowsAffected{total: total}, nil
}

// execChunkedInDeleteStrings is the string-ID variant of execChunkedInDelete.
func execChunkedInDeleteStrings(tx *sql.Tx, table, column string, ids []string, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = defaultInClauseChunkSize
	}
	if len(ids) == 0 {
		return nil
	}
	if !validIdentifier(table) {
		return fmt.Errorf("invalid table identifier %q", table)
	}
	if !validIdentifier(column) {
		return fmt.Errorf("invalid column identifier %q", column)
	}
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", quoteIdentifier(table), quoteIdentifier(column), strings.Join(placeholders, ","))
		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("delete %s by %s chunk %d-%d: %w", table, column, start, end-1, err)
		}
	}
	return nil
}

// execChunkedInDeleteStringsResult is like execChunkedInDeleteStrings but returns
// the combined sql.Result with the summed RowsAffected across all chunks.
func execChunkedInDeleteStringsResult(tx *sql.Tx, table, column string, ids []string, chunkSize int) (sql.Result, error) {
	if chunkSize <= 0 {
		chunkSize = defaultInClauseChunkSize
	}
	if len(ids) == 0 {
		return chunkedRowsAffected{}, nil
	}
	if !validIdentifier(table) {
		return nil, fmt.Errorf("invalid table identifier %q", table)
	}
	if !validIdentifier(column) {
		return nil, fmt.Errorf("invalid column identifier %q", column)
	}
	var total int64
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", quoteIdentifier(table), quoteIdentifier(column), strings.Join(placeholders, ","))
		res, err := tx.Exec(query, args...)
		if err != nil {
			return nil, fmt.Errorf("delete %s by %s chunk %d-%d: %w", table, column, start, end-1, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return chunkedRowsAffected{total: total}, nil
}

// escapeFTS5 quotes a user-supplied keyword so it is treated as a literal phrase
// by SQLite FTS5. This prevents special characters such as *, " and - from being
// interpreted as query syntax while still allowing prefix matches.
func escapeFTS5(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return ""
	}
	// Escape embedded double quotes by doubling them, then wrap the whole term in
	// double quotes so FTS5 treats it as a literal phrase.
	keyword = strings.ReplaceAll(keyword, `"`, `""`)
	return `"` + keyword + `"*`
}
