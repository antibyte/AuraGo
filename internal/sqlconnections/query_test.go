package sqlconnections

import (
	"strings"
	"testing"
)

func TestEffectiveMaxRowsUsesDefaultForNonPositiveValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "zero falls back to default", input: 0, want: defaultMaxResultRows},
		{name: "negative falls back to default", input: -25, want: defaultMaxResultRows},
		{name: "positive stays unchanged", input: 250, want: 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveMaxRows(tt.input); got != tt.want {
				t.Fatalf("effectiveMaxRows(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestListTablesStatementUsesCurrentSchemaForPostgres(t *testing.T) {
	t.Parallel()

	query, err := listTablesStatement("postgres")
	if err != nil {
		t.Fatalf("listTablesStatement() error = %v", err)
	}
	if !strings.Contains(query, "current_schema()") {
		t.Fatalf("query = %q, want current_schema()", query)
	}
	if strings.Contains(query, "'public'") {
		t.Fatalf("query = %q, should not hardcode public schema", query)
	}
}

func TestPostgresDescribeTableStatementUsesCurrentSchema(t *testing.T) {
	t.Parallel()

	query := postgresDescribeTableStatement()
	if strings.Count(query, "current_schema()") < 2 {
		t.Fatalf("query = %q, want current_schema() in primary-key and column filters", query)
	}
	if strings.Contains(query, "'public'") {
		t.Fatalf("query = %q, should not hardcode public schema", query)
	}
	if !strings.Contains(query, "tc.table_schema = ku.table_schema") {
		t.Fatalf("query = %q, want schema-aware key column join", query)
	}
}
