package tools

import (
	"fmt"
	neturl "net/url"
	"strings"
)

// yepAPIArgAliases lists the aliases each operation accepts for its
// parameters. A parameter never falls back to "query" implicitly: only the
// alias that fits the operation is accepted, so a query cannot silently
// become a domain, an ASIN or a channel id.
var yepAPIArgAliases = map[string]map[string][]string{
	"keywords":      {"keywords": {"query"}},
	"keyword_ideas": {"seed": {"query"}},
	"trends":        {"keywords": {"query"}},
}

// stringArgForOperation reads a string argument for one operation. The
// canonical key is read first, the aliases of the operation and the listed
// aliases afterwards.
func stringArgForOperation(args map[string]interface{}, operation, key string, aliases ...string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	for _, alias := range yepAPIArgAliases[operation][key] {
		if v, ok := args[alias].(string); ok && v != "" {
			return v
		}
	}
	for _, alias := range aliases {
		if v, ok := args[alias].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// stringSliceForOperation reads a string slice for one operation. The
// canonical key is read first, the aliases of the operation and the listed
// aliases afterwards.
func stringSliceForOperation(args map[string]interface{}, operation, key string, aliases ...string) []string {
	if values := stringSliceFromArgs(args, key); len(values) > 0 {
		return values
	}
	for _, alias := range yepAPIArgAliases[operation][key] {
		if values := stringSliceFromArgs(args, alias); len(values) > 0 {
			return values
		}
	}
	for _, alias := range aliases {
		if values := stringSliceFromArgs(args, alias); len(values) > 0 {
			return values
		}
	}
	return nil
}

// domainArgForOperation reads a domain-shaped parameter. A domain must not
// carry a scheme, a path or whitespace, because a wrong domain produces a
// lookup error far away from the cause.
func domainArgForOperation(args map[string]interface{}, operation, key string, aliases ...string) (string, error) {
	value := strings.TrimSpace(stringArgForOperation(args, operation, key, aliases...))
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, " \t\r\n") || strings.Contains(value, "://") || strings.Contains(value, "/") {
		return "", fmt.Errorf("%s requires a '%s' without scheme, path or whitespace, got %q", operation, key, value)
	}
	return value, nil
}

// urlArgForOperation reads a URL-shaped parameter. Only an http or https URL
// with a host is accepted.
func urlArgForOperation(args map[string]interface{}, operation, key string, aliases ...string) (string, error) {
	value := strings.TrimSpace(stringArgForOperation(args, operation, key, aliases...))
	if value == "" {
		return "", nil
	}
	parsed, err := neturl.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%s requires a '%s' with an http:// or https:// URL and a host, got %q", operation, key, value)
	}
	return value, nil
}

// targetArgForOperation reads a parameter that accepts a domain or a URL.
func targetArgForOperation(args map[string]interface{}, operation, key string, aliases ...string) (string, error) {
	value := strings.TrimSpace(stringArgForOperation(args, operation, key, aliases...))
	if value == "" {
		return "", nil
	}
	if parsed, err := neturl.ParseRequestURI(value); err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return value, nil
	}
	if strings.ContainsAny(value, " \t\r\n") || strings.Contains(value, "://") || strings.Contains(value, "/") {
		return "", fmt.Errorf("%s requires a '%s' as a domain or an http:// or https:// URL, got %q", operation, key, value)
	}
	return value, nil
}

// stringSliceFromArgs extracts a string slice from map arguments.
// Handles both []interface{} (from JSON unmarshalling) and []string.
func stringSliceFromArgs(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}

	if ss, ok := raw.([]string); ok {
		return ss
	}

	if arr, ok := raw.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	// Fallback: if the value is a single string, wrap it in a slice
	if s, ok := raw.(string); ok && s != "" {
		return []string{s}
	}

	return nil
}

func addPositiveIntArg(payload map[string]interface{}, args map[string]interface{}, argKey, payloadKey string) {
	if n, ok := args[argKey].(float64); ok && n > 0 {
		payload[payloadKey] = int(n)
	}
}

func addOptionalStringArg(payload map[string]interface{}, args map[string]interface{}, argKey, payloadKey string) {
	if v, ok := args[argKey].(string); ok && v != "" {
		payload[payloadKey] = v
	}
}

func addCountryArg(payload map[string]interface{}, args map[string]interface{}) {
	if country, ok := args["country"].(string); ok && country != "" {
		payload["country"] = country
		return
	}
	payload["country"] = "US"
}
