package findit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type dateValue string

var (
	fieldKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	idPattern       = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)
	engineering     = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([pnumkKMG]?)$`)
	engineeringRKM  = regexp.MustCompile(`^([0-9]+)([rRkKmM])([0-9]+)$`)
)

func decodeData(raw json.RawMessage, target any) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return domainError("invalid", "command data is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return domainError("invalid", "command data is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domainError("invalid", "command data contains extra values")
	}
	return nil
}

func validateID(value, field string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" && !required {
		return nil
	}
	if !idPattern.MatchString(value) {
		return domainError("invalid", "invalid "+field)
	}
	return nil
}

func validateText(value, field string, max int, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", domainError("invalid", field+" is required")
	}
	if len([]rune(value)) > max || strings.ContainsRune(value, '\x00') {
		return "", domainError("invalid", field+" is too long or invalid")
	}
	return value, nil
}

func ParseQuantity(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") || strings.ContainsAny(value, "eE,") {
		return 0, invalidQuantity("quantity must be a non-negative decimal string")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, invalidQuantity("quantity must be a non-negative decimal string")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > math.MaxInt64/quantityScale {
		return 0, invalidQuantity("quantity is outside the supported range")
	}
	frac := int64(0)
	if len(parts) == 2 {
		if parts[1] == "" || len(parts[1]) > 6 {
			return 0, invalidQuantity("quantity supports at most six decimal places")
		}
		for _, r := range parts[1] {
			if r < '0' || r > '9' {
				return 0, invalidQuantity("quantity must be a decimal string")
			}
		}
		fraction := parts[1] + strings.Repeat("0", 6-len(parts[1]))
		frac, _ = strconv.ParseInt(fraction, 10, 64)
	}
	if whole > math.MaxInt64/quantityScale || whole*quantityScale > math.MaxInt64-frac {
		return 0, invalidQuantity("quantity is outside the supported range")
	}
	return whole*quantityScale + frac, nil
}

func invalidQuantity(message string) error {
	return detailedError("invalid", "invalid_quantity", "quantity", message, nil)
}

func FormatQuantity(value int64) string {
	if value <= 0 {
		return "0"
	}
	whole, fraction := value/quantityScale, value%quantityScale
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	decimal := fmt.Sprintf("%06d", fraction)
	decimal = strings.TrimRight(decimal, "0")
	return strconv.FormatInt(whole, 10) + "." + decimal
}

func parseNumeric(value any) (float64, error) {
	var raw string
	switch v := value.(type) {
	case json.Number:
		raw = string(v)
	case float64:
		raw = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		raw = strconv.FormatFloat(float64(v), 'f', -1, 64)
	case int:
		raw = strconv.Itoa(v)
	case int64:
		raw = strconv.FormatInt(v, 10)
	case string:
		raw = strings.TrimSpace(v)
	default:
		return 0, domainError("invalid", "numeric value is invalid")
	}
	if parts := engineeringRKM.FindStringSubmatch(raw); parts != nil {
		base, err := strconv.ParseFloat(parts[1]+"."+parts[3], 64)
		if err != nil || math.IsInf(base, 0) {
			return 0, domainError("invalid", "numeric value is invalid")
		}
		switch parts[2] {
		case "k", "K":
			base *= 1e3
		case "M":
			base *= 1e6
		case "m":
			base *= 1e-3
		}
		if math.IsInf(base, 0) {
			return 0, domainError("invalid", "numeric value is invalid")
		}
		return base, nil
	}
	if parts := engineering.FindStringSubmatch(raw); parts != nil {
		n, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, domainError("invalid", "numeric value is invalid")
		}
		switch parts[2] {
		case "p":
			n *= 1e-12
		case "n":
			n *= 1e-9
		case "u":
			n *= 1e-6
		case "m":
			n *= 1e-3
		case "k", "K":
			n *= 1e3
		case "M":
			n *= 1e6
		case "G":
			n *= 1e9
		}
		if math.IsInf(n, 0) || math.IsNaN(n) {
			return 0, domainError("invalid", "numeric value is invalid")
		}
		return n, nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, domainError("invalid", "numeric value is invalid")
	}
	return n, nil
}

func validUnit(unit string) bool {
	for _, item := range Catalog().Units {
		if item.ID == unit {
			return true
		}
	}
	return false
}

func validateURLs(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, raw := range values {
		value, err := validateText(raw, "datasheet URL", 2048, true)
		if err != nil {
			return nil, err
		}
		u, err := url.Parse(value)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
			return nil, domainError("invalid", "datasheet URLs must be absolute HTTP or HTTPS links without user information")
		}
		if !seen[value] {
			out = append(out, value)
			seen[value] = true
		}
	}
	if len(out) > 32 {
		return nil, domainError("invalid", "too many datasheet URLs")
	}
	return out, nil
}

func validateTags(values []string) ([]string, error) {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value, err := validateText(raw, "tag", 64, true)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(value)
		if !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	if len(out) > 64 {
		return nil, domainError("invalid", "too many tags")
	}
	sort.Strings(out)
	return out, nil
}

func validateAssetID(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !idPattern.MatchString(value) {
		return "", domainError("invalid", field+" must be an opaque asset id")
	}
	return value, nil
}

func validateTemplateFields(fields []PropertyDefinition) ([]PropertyDefinition, error) {
	if len(fields) > 128 {
		return nil, domainError("invalid", "template has too many fields")
	}
	out := make([]PropertyDefinition, len(fields))
	seen := make(map[string]bool)
	for i, field := range fields {
		field.Key = strings.TrimSpace(field.Key)
		field.Label = strings.TrimSpace(field.Label)
		if !fieldKeyPattern.MatchString(field.Key) || field.Label == "" || len(field.Label) > 128 || seen[field.Key] {
			return nil, domainError("invalid", "template fields need unique keys and labels")
		}
		seen[field.Key] = true
		switch field.Type {
		case "text", "date", "number", "boolean":
			if len(field.Options) != 0 {
				return nil, domainError("invalid", "only enum fields can have options")
			}
		case "enum":
			if len(field.Options) == 0 || len(field.Options) > 128 {
				return nil, domainError("invalid", "enum fields need one or more options")
			}
			optionSeen := make(map[string]bool)
			for j, option := range field.Options {
				option = strings.TrimSpace(option)
				if option == "" || len(option) > 128 || optionSeen[option] {
					return nil, domainError("invalid", "enum options must be unique non-empty values")
				}
				field.Options[j] = option
				optionSeen[option] = true
			}
		default:
			return nil, domainError("invalid", "template field type is unsupported")
		}
		if field.Type != "number" && (field.Min != nil || field.Max != nil || field.Unit != "") {
			return nil, domainError("invalid", "only numeric fields can define bounds or units")
		}
		if field.Min != nil && (math.IsNaN(*field.Min) || math.IsInf(*field.Min, 0)) || field.Max != nil && (math.IsNaN(*field.Max) || math.IsInf(*field.Max, 0)) {
			return nil, domainError("invalid", "template numeric bounds must be finite")
		}
		if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
			return nil, domainError("invalid", "template numeric bounds are reversed")
		}
		out[i] = field
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func validateProperties(fields []PropertyDefinition, input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	byKey := make(map[string]PropertyDefinition, len(fields))
	for _, field := range fields {
		byKey[field.Key] = field
	}
	if len(input) > len(fields) {
		return nil, domainError("invalid", "item has properties that are not defined by its template")
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		field, ok := byKey[key]
		if !ok {
			return nil, domainError("invalid", "item has properties that are not defined by its template")
		}
		if value == nil {
			return nil, domainError("invalid", "property values cannot be null")
		}
		switch field.Type {
		case "text":
			text, ok := value.(string)
			if !ok || len([]rune(text)) > 2048 {
				return nil, domainError("invalid", "text property value is invalid")
			}
			out[key] = strings.TrimSpace(text)
		case "date":
			text, ok := value.(string)
			if !ok || !validISODate(text) {
				return nil, detailedError("invalid", "invalid_date", key, "date properties must use a real YYYY-MM-DD date", nil)
			}
			out[key] = dateValue(text)
		case "number":
			number, err := parseNumeric(value)
			if err != nil || field.Min != nil && number < *field.Min || field.Max != nil && number > *field.Max {
				return nil, domainError("invalid", "numeric property value is outside its allowed range")
			}
			out[key] = json.Number(strconv.FormatFloat(number, 'f', -1, 64))
		case "boolean":
			b, ok := value.(bool)
			if !ok {
				return nil, domainError("invalid", "boolean property value is invalid")
			}
			out[key] = b
		case "enum":
			text, ok := value.(string)
			found := false
			for _, option := range field.Options {
				if ok && text == option {
					found = true
					break
				}
			}
			if !found {
				return nil, domainError("invalid", "enum property value is not an allowed option")
			}
			out[key] = text
		}
	}
	for _, field := range fields {
		if field.Required {
			if _, ok := out[field.Key]; !ok {
				return nil, domainError("invalid", "required template property is missing")
			}
		}
	}
	return out, nil
}

func validISODate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validateDimension(value *float64) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 || *value > 1_000_000 {
		return domainError("invalid", "dimension must be a finite positive number")
	}
	return nil
}

func normalizeEngineeringToken(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if engineeringRKM.MatchString(raw) {
		return raw
	}
	return raw
}

func countLetters(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return count
}
