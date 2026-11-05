package flows

import (
	"errors"
	"math"
	"sort"
	"unicode/utf8"
)

const (
	collectionMaxItems  = 10000
	collectionMaxFields = 256
	collectionMaxValues = 100000
	collectionMaxBytes  = 2 << 20
	collectionMaxDepth  = 32
)

var errCollection = errors.New("invalid or oversized collection")

// Check the entire input before constructing output or sorting. Only JSON data
// is accepted; references can remain shared because no filter mutates them.
func checkCollectionInput(in any) error {
	type entry struct {
		v     any
		depth int
	}
	stack := []entry{{in, 0}}
	count, size := 0, 0
	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > collectionMaxValues || e.depth > collectionMaxDepth {
			return errCollection
		}
		switch v := e.v.(type) {
		case nil, bool:
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return errCollection
			}
		case string:
			if !utf8.ValidString(v) {
				return errCollection
			}
			size += len(v)
		case []any:
			if len(v) > collectionMaxItems || count+len(stack)+len(v) > collectionMaxValues {
				return errCollection
			}
			for _, x := range v {
				stack = append(stack, entry{x, e.depth + 1})
			}
		case map[string]any:
			if len(v) > collectionMaxFields || count+len(stack)+len(v) > collectionMaxValues {
				return errCollection
			}
			for k, x := range v {
				if !utf8.ValidString(k) {
					return errCollection
				}
				size += len(k)
				stack = append(stack, entry{x, e.depth + 1})
			}
		default:
			return errCollection
		}
		if size > collectionMaxBytes {
			return errCollection
		}
	}
	return nil
}

func collectionField(args []any, i int) (string, error) {
	s, err := stringArg(args, i, "")
	if err != nil || !utf8.ValidString(s) || len(s) > collectionMaxBytes {
		return "", errCollection
	}
	return s, nil
}

func filterPick(in any, args []any, _ *Env) (any, error) {
	if err := checkCollectionInput(in); err != nil {
		return nil, err
	}
	m, ok := in.(map[string]any)
	if !ok {
		return nil, errCollection
	}
	out := make(map[string]any, len(args))
	for i := range args {
		k, err := collectionField(args, i)
		if err != nil {
			return nil, err
		}
		if v, exists := m[k]; exists {
			out[k] = v
		}
	}
	return out, nil
}

func filterRename(in any, args []any, _ *Env) (any, error) {
	if err := checkCollectionInput(in); err != nil {
		return nil, err
	}
	m, ok := in.(map[string]any)
	if !ok {
		return nil, errCollection
	}
	from, err := collectionField(args, 0)
	if err != nil {
		return nil, err
	}
	to, err := collectionField(args, 1)
	if err != nil {
		return nil, err
	}
	v, exists := m[from]
	if !exists {
		return nil, errCollection
	}
	if _, exists = m[to]; exists && from != to {
		return nil, errCollection
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k != from {
			out[k] = v
		}
	}
	out[to] = v
	if err := checkCollectionInput(out); err != nil {
		return nil, err
	}
	return out, nil
}

type collectionKey struct {
	missing bool
	value   any
}

func primitive(v any) bool {
	switch v := v.(type) {
	case nil, bool, string:
		return true
	case float64:
		return !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	return false
}
func collectionList(in any) ([]any, error) {
	if err := checkCollectionInput(in); err != nil {
		return nil, err
	}
	list, ok := in.([]any)
	if !ok {
		return nil, errCollection
	}
	return list, nil
}
func collectionItem(item any, field string) (collectionKey, error) {
	if field == "" {
		return collectionKey{value: item}, nil
	}
	m, ok := item.(map[string]any)
	if !ok {
		return collectionKey{}, errCollection
	}
	v, ok := m[field]
	return collectionKey{missing: !ok, value: v}, nil
}
func compareCollection(a, b any) (int, error) {
	switch a := a.(type) {
	case string:
		b, ok := b.(string)
		if !ok {
			return 0, errCollection
		}
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
	case float64:
		b, ok := b.(float64)
		if !ok || !primitive(a) || !primitive(b) {
			return 0, errCollection
		}
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
	default:
		return 0, errCollection
	}
	return 0, nil
}

func filterWhere(in any, args []any, _ *Env) (any, error) {
	list, err := collectionList(in)
	if err != nil {
		return nil, err
	}
	field, err := collectionField(args, 0)
	if err != nil {
		return nil, err
	}
	op, err := collectionField(args, 1)
	if err != nil || !primitive(args[2]) || checkCollectionInput(args[2]) != nil {
		return nil, errCollection
	}
	if op != "eq" && op != "ne" && op != "lt" && op != "le" && op != "gt" && op != "ge" {
		return nil, errCollection
	}
	out := make([]any, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, errCollection
		}
		v, exists := m[field]
		if !exists {
			continue
		}
		if !primitive(v) {
			return nil, errCollection
		}
		match := false
		if op == "eq" {
			match = v == args[2]
		} else if op == "ne" {
			match = v != args[2]
		} else {
			c, err := compareCollection(v, args[2])
			if err != nil {
				return nil, err
			}
			switch op {
			case "lt":
				match = c < 0
			case "le":
				match = c <= 0
			case "gt":
				match = c > 0
			case "ge":
				match = c >= 0
			}
		}
		if match {
			out = append(out, item)
		}
	}
	return out, nil
}

func filterSort(in any, args []any, _ *Env) (any, error) {
	list, err := collectionList(in)
	if err != nil {
		return nil, err
	}
	field := ""
	if len(args) > 0 {
		field, err = collectionField(args, 0)
		if err != nil {
			return nil, err
		}
	}
	dir := "asc"
	if len(args) > 1 {
		dir, err = collectionField(args, 1)
		if err != nil || (dir != "asc" && dir != "desc") {
			return nil, errCollection
		}
	}
	type row struct {
		item any
		key  collectionKey
	}
	rows := make([]row, 0, len(list))
	var first any
	for _, item := range list {
		k, err := collectionItem(item, field)
		if err != nil {
			return nil, err
		}
		if !k.missing && k.value != nil {
			if _, err = compareCollection(k.value, k.value); err != nil {
				return nil, err
			}
			if first == nil {
				first = k.value
			} else if _, err = compareCollection(first, k.value); err != nil {
				return nil, err
			}
		}
		rows = append(rows, row{item, k})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].key, rows[j].key
		aNull, bNull := a.missing || a.value == nil, b.missing || b.value == nil
		if aNull || bNull {
			return !aNull && bNull
		}
		c, _ := compareCollection(a.value, b.value)
		if dir == "desc" {
			return c > 0
		}
		return c < 0
	})
	out := make([]any, len(rows))
	for i, row := range rows {
		out[i] = row.item
	}
	return out, nil
}

func filterUnique(in any, args []any, _ *Env) (any, error) {
	list, err := collectionList(in)
	if err != nil {
		return nil, err
	}
	field := ""
	if len(args) > 0 {
		field, err = collectionField(args, 0)
		if err != nil {
			return nil, err
		}
	}
	seen := make(map[collectionKey]bool)
	out := make([]any, 0, len(list))
	for _, item := range list {
		k, err := collectionItem(item, field)
		if err != nil || !primitive(k.value) {
			return nil, errCollection
		}
		if !seen[k] {
			seen[k] = true
			out = append(out, item)
		}
	}
	return out, nil
}

func filterSum(in any, args []any, _ *Env) (any, error) {
	list, err := collectionList(in)
	if err != nil {
		return nil, err
	}
	field := ""
	if len(args) > 0 {
		field, err = collectionField(args, 0)
		if err != nil {
			return nil, err
		}
	}
	sum := 0.0
	for _, item := range list {
		k, err := collectionItem(item, field)
		if err != nil || k.missing {
			return nil, errCollection
		}
		n, ok := k.value.(float64)
		if !ok {
			return nil, errCollection
		}
		sum += n
		if !primitive(sum) {
			return nil, errCollection
		}
	}
	return sum, nil
}
