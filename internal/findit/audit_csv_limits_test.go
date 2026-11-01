package findit

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseCSVRejectsOverwideHeaderBeforeParsingFields(t *testing.T) {
	header := strings.Repeat("column,", maxCSVColumns) + "last"
	_, err := parseCSV(header + "\nvalue\n")
	if ErrorCode(err) != "invalid" || ErrorDetail(err).Message != "CSV header must contain between 1 and 256 columns" {
		t.Fatalf("over-wide header error = %v, want header column limit", err)
	}
}

func TestParseCSVRejectsOverwideDataRecordBeforeParsingFields(t *testing.T) {
	values := strings.Split(strings.Repeat("x,", maxCSVColumns)+"last", ",")
	_, err := parseCSV("name\n" + strings.Join(values, ",") + "\n")
	if ErrorCode(err) != "invalid" || ErrorDetail(err).Message != "CSV row contains too many columns" {
		t.Fatalf("over-wide data row error = %v, want data row column limit", err)
	}
}

func TestParseCSVAllowsMaximumColumnCount(t *testing.T) {
	header := make([]string, maxCSVColumns)
	values := make([]string, maxCSVColumns)
	for i := range header {
		header[i] = "column" + strconv.Itoa(i)
		values[i] = "value"
	}
	parsed, err := parseCSV(strings.Join(header, ",") + "\n" + strings.Join(values, ",") + "\n")
	if err != nil {
		t.Fatalf("parse maximum-width CSV: %v", err)
	}
	if len(parsed.headers) != maxCSVColumns || len(parsed.records) != 1 || len(parsed.records[0]) != maxCSVColumns {
		t.Fatalf("maximum-width parse counts = %d headers, %d records", len(parsed.headers), len(parsed.records))
	}
}

func TestParseCSVColumnPreflightRespectsQuotedDelimitersAndNewlines(t *testing.T) {
	parsed, err := parseCSV("name,description\nPart,\"one, two\nthree\"\n")
	if err != nil {
		t.Fatalf("parse quoted comma/newline: %v", err)
	}
	if len(parsed.records) != 1 || len(parsed.records[0]) != 2 || parsed.records[0][1] != "one, two\nthree" {
		t.Fatalf("quoted CSV record = %#v, want one two-column record", parsed.records)
	}
}

func TestParseCSVMalformedQuotedRecordStillUsesCSVReaderValidation(t *testing.T) {
	content := "name,description\n\"unterminated," + strings.Repeat("value,", maxCSVColumns+1) + "tail\n"
	_, err := parseCSV(content)
	if ErrorCode(err) != "invalid" || ErrorDetail(err).Message != "CSV syntax is invalid" {
		t.Fatalf("malformed CSV error = %v, want syntax validation error", err)
	}
}
