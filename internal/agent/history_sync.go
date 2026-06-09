package agent

// ShouldAppendHistoryMessage reports whether a SQLite insert succeeded with a
// valid row ID. HistoryManager must only mirror messages that were persisted to
// short-term memory so history.json and SQLite stay aligned.
func ShouldAppendHistoryMessage(sqliteID int64, insertErr error) bool {
	return insertErr == nil && sqliteID > 0
}