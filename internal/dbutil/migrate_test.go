package dbutil

import (
	"log/slog"
	"strings"
	"testing"
)

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
