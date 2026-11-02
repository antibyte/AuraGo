package findit

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var searchQuantityPattern = regexp.MustCompile(`^([+-]?[0-9]+(?:[.,][0-9]+)?)\s*([pnumkKMGTµμ]?)\s*(Ω|Ω|[A-Za-z°%/µμ]+)?$`)

// Explicit engineering quantities search typed values and their declared units.
// This also works for existing databases without rebuilding their text index.
func quantitySearchClause(query string) (string, []any, bool) {
	parts := searchQuantityPattern.FindStringSubmatch(strings.TrimSpace(query))
	if parts == nil {
		raw := strings.NewReplacer(",", ".", "µ", "u", "μ", "u").Replace(strings.TrimSpace(query))
		if engineeringRKM.MatchString(raw) {
			if value, err := parseNumeric(raw); err == nil {
				return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.value_type='number' AND ip.number_value BETWEEN ? AND ?)`, []any{math.Nextafter(value, math.Inf(-1)), math.Nextafter(value, math.Inf(1))}, true
			}
		}
		return "", nil, false
	}
	// A separated "m" is a metre unit; the compact form "4.7m" retains
	// engineering milli notation. "4.7mm" is millimetres in either form.
	if words := strings.Fields(query); len(words) == 2 && words[1] == "m" {
		parts[2], parts[3] = "", "m"
	}
	if parts[2] == "" && parts[3] == "" {
		return "", nil, false
	}
	number, err := strconv.ParseFloat(strings.ReplaceAll(parts[1], ",", "."), 64)
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
		return "", nil, false
	}
	prefixes := []struct {
		text  string
		scale float64
	}{{"", 1}, {"p", 1e-12}, {"n", 1e-9}, {"u", 1e-6}, {"µ", 1e-6}, {"μ", 1e-6}, {"m", 1e-3}, {"k", 1e3}, {"K", 1e3}, {"M", 1e6}, {"G", 1e9}, {"T", 1e12}}
	for _, prefix := range prefixes {
		if prefix.text == parts[2] {
			number *= prefix.scale
			break
		}
	}
	if math.IsInf(number, 0) {
		return "", nil, false
	}
	if parts[3] == "" {
		return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.value_type='number' AND ip.number_value BETWEEN ? AND ?)`, []any{math.Nextafter(number, math.Inf(-1)), math.Nextafter(number, math.Inf(1))}, true
	}
	units := []string{parts[3]}
	if parts[3] == "Ω" || parts[3] == "Ω" || strings.EqualFold(parts[3], "ohm") || strings.EqualFold(parts[3], "ohms") {
		units = []string{"Ω", "Ω", "ohm", "Ohm", "ohms", "Ohms"}
	}
	bounds, args := []string{}, []any{}
	for _, prefix := range prefixes {
		value := number / prefix.scale
		if math.IsInf(value, 0) {
			continue
		}
		for _, unit := range units {
			bounds = append(bounds, `(?,?,?)`)
			args = append(args, prefix.text+unit, math.Nextafter(value, math.Inf(-1)), math.Nextafter(value, math.Inf(1)))
		}
	}
	// Resolve units once per template, then use the (field_key, number_value)
	// index. A correlated JSON scan would repeat this work for every item.
	return `i.id IN (WITH unit_bounds(unit,low,high) AS (VALUES ` + strings.Join(bounds, ",") + `),
		fields AS MATERIALIZED (
			SELECT t.id template_id,json_extract(field.value,'$.key') field_key,b.low,b.high
			FROM templates t JOIN json_each(t.fields_json) field
			JOIN unit_bounds b ON json_extract(field.value,'$.unit')=b.unit
			WHERE json_extract(field.value,'$.type')='number'
		)
		SELECT ip.item_id FROM fields f CROSS JOIN item_properties ip CROSS JOIN items matched
		WHERE ip.field_key=f.field_key AND ip.number_value BETWEEN f.low AND f.high
		AND ip.value_type='number' AND matched.id=ip.item_id AND matched.template_id=f.template_id)`, args, true
}
