package outputcompress

import (
	"fmt"
	"strings"
	"testing"
)

func TestSmartCrushJSON_SkipsShortInput(t *testing.T) {
	input := `[{"id":1,"name":"a"}]`
	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if ok {
		t.Error("expected smartCrushJSON to skip short input")
	}
	if result != input {
		t.Errorf("expected unchanged input, got %q", result)
	}
}

func TestSmartCrushJSON_SkipsNonJSON(t *testing.T) {
	input := strings.Repeat("hello world ", 200)
	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if ok {
		t.Error("expected smartCrushJSON to skip non-JSON")
	}
	if result != input {
		t.Errorf("expected unchanged input, got %q", result)
	}
}

func TestSmartCrushJSON_SkipsInvalidJSON(t *testing.T) {
	input := strings.Repeat("{", 2000)
	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if ok {
		t.Error("expected smartCrushJSON to skip invalid JSON")
	}
	if result != input {
		t.Errorf("expected unchanged input, got %q", result)
	}
}

func TestSmartCrushJSON_ArrayOfObjects(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 60; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`  {"id": %d, "name": "item-%d", "status": "ok"}`, i, i))
	}
	sb.WriteString("\n]")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if !ok {
		t.Fatal("expected smartCrushJSON to compress array of objects")
	}
	if !strings.HasPrefix(result, "[JSON_ARRAY_COMPACT]") {
		t.Errorf("expected compact table header, got %q", result)
	}
	if !strings.Contains(result, "id\tname\tstatus") {
		t.Errorf("expected header row with keys, got %q", result)
	}
	if strings.Contains(result, "{\"") {
		t.Error("expected JSON objects to be replaced by table rows")
	}
}

func TestSmartCrushJSON_TailTruncation(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 120; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`  {"id": %d, "name": "item-%d"}`, i, i))
	}
	sb.WriteString("\n]")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 10, TailRows: 3, MaxCols: 20})
	if !ok {
		t.Fatal("expected smartCrushJSON to compress large array")
	}
	if !strings.Contains(result, "... (") {
		t.Error("expected tail truncation marker")
	}
	// Should contain head rows
	if !strings.Contains(result, "item-0") {
		t.Error("expected head rows to be present")
	}
	// Should contain tail rows (last index is 119)
	if !strings.Contains(result, "item-119") {
		t.Error("expected tail rows to be present")
	}
}

func TestCrushArrayOfObjects_Heterogeneous(t *testing.T) {
	// Objects share < 50% keys → should fall back to minified JSON.
	// Test crushArrayOfObjects directly to bypass the length gate.
	arr := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			arr[i] = map[string]interface{}{"id": i, "name": fmt.Sprintf("item-%d", i)}
		} else {
			arr[i] = map[string]interface{}{"uuid": fmt.Sprintf("u-%d", i), "value": i * 10}
		}
	}

	result := crushArrayOfObjects(arr, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	// With threshold=0.6 and 20 items where each key appears in exactly 10 items,
	// no key reaches the 60% threshold (needs >= 12), so we get minified JSON.
	if strings.HasPrefix(result, "[JSON_ARRAY_COMPACT]") {
		t.Error("expected minified JSON fallback, not table")
	}
	if strings.Contains(result, "\n  ") {
		t.Error("expected compact single-line JSON")
	}
}

func TestSmartCrushJSON_TooWideTable(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 30; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("  {")
		for j := 0; j < 30; j++ {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf(`"col_%d": %d`, j, j))
		}
		sb.WriteString("}")
	}
	sb.WriteString("\n]")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if !ok {
		t.Fatal("expected smartCrushJSON to compress wide array")
	}
	if strings.HasPrefix(result, "[JSON_ARRAY_COMPACT]") {
		t.Error("expected minified JSON fallback for wide tables")
	}
}

func TestSmartCrushJSON_MinifiedJSONObject(t *testing.T) {
	// Build a large object with lots of whitespace so minification saves > 15%.
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 120; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`    "field_%d" : { "a" : %d , "b" : "%s" }`, i, i, strings.Repeat("x", 20)))
	}
	sb.WriteString("\n}")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if !ok {
		t.Fatalf("expected smartCrushJSON to compress large object (input=%d chars)", len(input))
	}
	if strings.Contains(result, "\n    ") {
		t.Error("expected compact single-line JSON")
	}
}

func TestSmartCrushJSON_Disabled(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 20; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`  {"id": %d}`, i))
	}
	sb.WriteString("\n]")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: false, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if ok {
		t.Error("expected smartCrushJSON to respect Enabled=false")
	}
	if result != input {
		t.Error("expected unchanged input when disabled")
	}
}

func TestEscapeForTSV(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"hello\tworld", "hello world"},
		{"line1\nline2", "line1 line2"},
		{"carriage\rreturn", "carriagereturn"},
		{"normal", "normal"},
	}
	for _, c := range cases {
		got := escapeForTSV(c.input)
		if got != c.expected {
			t.Errorf("escapeForTSV(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestCompactValue(t *testing.T) {
	cases := []struct {
		input    interface{}
		expected string
	}{
		{nil, ""},
		{"hello", "hello"},
		{strings.Repeat("a", 100), strings.Repeat("a", 77) + "..."},
		{float64(42), "42"},
		{true, "true"},
		{[]interface{}{1, 2, 3}, "[...3 items]"},
		{map[string]interface{}{"a": 1}, "{...1 keys}"},
	}
	for _, c := range cases {
		got := compactValue(c.input)
		if got != c.expected {
			t.Errorf("compactValue(%v) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestSmartCrushJSON_NestedArraysInValues(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 60; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`  {"id": %d, "tags": ["a", "b", "c"]}`, i))
	}
	sb.WriteString("\n]")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if !ok {
		t.Fatal("expected smartCrushJSON to compress array with nested arrays")
	}
	if !strings.Contains(result, "[...3 items]") {
		t.Errorf("expected nested array compaction, got %q", result)
	}
}

func TestSmartCrushJSON_SavingsThreshold(t *testing.T) {
	// Very small objects with many whitespace → minification should save > 5%.
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 80; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`  { "id" : %d , "name" : "item-%d" }`, i, i))
	}
	sb.WriteString("\n]")
	input := sb.String()

	result, ok := smartCrushJSON(input, SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20})
	if !ok {
		t.Fatal("expected smartCrushJSON to compress whitespace-heavy JSON")
	}
	if len(result) >= len(input) {
		t.Error("expected output to be shorter than input")
	}
}

func BenchmarkSmartCrushJSON_LargeArray(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 500; i++ {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`  {"id": %d, "name": "item-%d", "status": "ok", "count": %d}`, i, i, i*2))
	}
	sb.WriteString("\n]")
	input := sb.String()
	cfg := SmartCrusherConfig{Enabled: true, MaxRows: 50, TailRows: 5, MaxCols: 20}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = smartCrushJSON(input, cfg)
	}
}
