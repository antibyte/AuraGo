package chibisensei

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
)

// ParseGPUQuota reads the remaining Kaggle GPU quota in hours from the raw JSON answer of
// get_accelerator_quota_statistics. PROVISIONAL: the pinned SDK contract has no typed response, so the real
// shape is unknown until the proof run records one (add it as a fixture to TestParseGPUQuota then). The
// parser is tolerant:
//   - GPU entries are objects with a type-like key (containing "type", "accelerator", "name" or "kind")
//     whose string value contains "gpu", "nvidia" or "tesla"; every object below a key containing "gpu";
//     and the gpu-named numeric keys of any object.
//   - In an entry a key containing "remaining", "available" or "left" wins; otherwise limit ("limit",
//     "total", "quota", "max", "allowed") minus used ("used", "consumed", "spent"). "used" is tested
//     before "quota", so "quotaUsed" counts as used.
//   - A value is a JSON number, a numeric string, or a protobuf duration string ("99000s", "1500ms").
//     The unit comes from the suffix or the key ("milli"/…ms, "second"/…sec, "minute"/…mins, "hour");
//     without a hint a value above 500 is seconds, otherwise hours (the weekly quota is tens of hours).
//   - Several GPU entries give the smallest remaining value (conservative); negatives become 0.
//
// ok is false when no GPU entry carries a usable number or the JSON is invalid.
func ParseGPUQuota(raw []byte) (hours float64, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return 0, false
	}
	var q quotaScan
	q.walk(doc, false, 0)
	if !q.found {
		return 0, false
	}
	return math.Round(q.min*100) / 100, true
}

type quotaScan struct {
	min   float64
	found bool
}

func (q *quotaScan) note(h float64) {
	if h < 0 {
		h = 0
	}
	if !q.found || h < q.min {
		q.min, q.found = h, true
	}
}

func (q *quotaScan) walk(v any, gpu bool, depth int) {
	if depth > 10 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		entry := gpu || quotaTypedGPU(x)
		if h, ok := quotaRemaining(x, !entry); ok {
			q.note(h)
		}
		for k, child := range x {
			q.walk(child, entry || strings.Contains(strings.ToLower(k), "gpu"), depth+1)
		}
	case []any:
		for _, child := range x {
			q.walk(child, gpu, depth+1)
		}
	}
}

func quotaKeyHas(key string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(key, p) {
			return true
		}
	}
	return false
}

func quotaTypedGPU(m map[string]any) bool {
	for k, v := range m {
		s, ok := v.(string)
		if !ok || !quotaKeyHas(strings.ToLower(k), "type", "accelerator", "name", "kind") {
			continue
		}
		if quotaKeyHas(strings.ToLower(s), "gpu", "nvidia", "tesla") {
			return true
		}
	}
	return false
}

func quotaRemaining(m map[string]any, gpuKeysOnly bool) (float64, bool) {
	var remaining, limit, used []float64
	for k, v := range m {
		lk := strings.ToLower(k)
		if gpuKeysOnly && !strings.Contains(lk, "gpu") {
			continue
		}
		h, ok := quotaHours(lk, v)
		if !ok {
			continue
		}
		switch {
		case quotaKeyHas(lk, "remaining", "available", "left"):
			remaining = append(remaining, h)
		case quotaKeyHas(lk, "used", "consumed", "spent"):
			used = append(used, h)
		case quotaKeyHas(lk, "limit", "total", "quota", "max", "allowed"):
			limit = append(limit, h)
		}
	}
	if len(remaining) > 0 {
		return slices.Min(remaining), true
	}
	if len(limit) > 0 && len(used) > 0 {
		return slices.Min(limit) - slices.Max(used), true
	}
	return 0, false
}

func quotaHours(key string, v any) (float64, bool) {
	var n float64
	unit := ""
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		n = f
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		switch {
		case strings.HasSuffix(s, "ms"):
			unit, s = "ms", strings.TrimSuffix(s, "ms")
		case strings.HasSuffix(s, "s"):
			unit, s = "s", strings.TrimSuffix(s, "s")
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		n = f
	default:
		return 0, false
	}
	if unit == "" {
		switch {
		case strings.Contains(key, "milli") || strings.HasSuffix(key, "ms"):
			unit = "ms"
		case strings.Contains(key, "second") || strings.HasSuffix(key, "sec") || strings.HasSuffix(key, "secs"):
			unit = "s"
		case strings.Contains(key, "minute") || strings.HasSuffix(key, "mins"):
			unit = "min"
		case strings.Contains(key, "hour"):
			unit = "h"
		}
	}
	switch unit {
	case "ms":
		n /= 3_600_000
	case "s":
		n /= 3600
	case "min":
		n /= 60
	case "h":
	default:
		if n > 500 {
			n /= 3600
		}
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}
