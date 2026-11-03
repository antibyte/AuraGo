package dbutil

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestIsDuplicateColumnErrorRecognizesSQLiteMessage(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("SQL logic error: duplicate column name: name (1)"), true},
		{errors.New("SQL logic error: no such table: users (1)"), false},
	} {
		if got := isDuplicateColumnError(tc.err); got != tc.want {
			t.Errorf("error=%v got=%v want=%v", tc.err, got, tc.want)
		}
	}
}

func TestMigrateAddColumnRejectsUnsafeIdentifierBeforeQuery(t *testing.T) {
	t.Parallel()

	err := MigrateAddColumn(nil, "users; DROP TABLE users", "name", "TEXT", slog.Default())
	if err == nil || !strings.Contains(err.Error(), "invalid table name") {
		t.Fatalf("expected invalid table name error, got %v", err)
	}
}

func TestMigrateAddColumnCheckedRejectsUnsafeColumnDefinitionBeforeQuery(t *testing.T) {
	t.Parallel()

	err := MigrateAddColumnChecked(nil, "users", "display_name", "TEXT; DROP TABLE users", slog.Default())
	if err == nil || !strings.Contains(err.Error(), "invalid column definition") {
		t.Fatalf("expected invalid column definition error, got %v", err)
	}
}
