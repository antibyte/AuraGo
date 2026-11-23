package catalogsync

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// compactCatalog mirrors the compact models.json encoding that
// @oh-my-pi/pi-catalog ships since 19.x (src/model-catalog-format.ts).
//
// Encoded values are JSON scalars or a one-element array referencing a node;
// a reference of -1 marks an explicitly absent value. Object nodes store their
// non-boolean properties as [keyID, value, ...] pairs and their booleans as a
// shared key shape plus 32-bit masks. Each provider row carries the values all
// of its models share, then one row per model. The per-model compat delta is
// not needed by AuraGo and is ignored.
type compactCatalog struct {
	Version       int                 `json:"version"`
	Keys          []string            `json:"keys"`
	BooleanShapes [][]int             `json:"booleanShapes"`
	Nodes         []json.RawMessage   `json:"nodes"`
	Providers     [][]json.RawMessage `json:"providers"`
}

type compactDecoder struct {
	catalog compactCatalog
	nodes   [][]json.RawMessage
	depth   int
}

// isCompactCatalog reports whether data uses the compact encoding: an object
// whose top-level "version" field is a number.
func isCompactCatalog(data []byte) bool {
	var probe struct {
		Version json.RawMessage `json:"version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	v := bytes.TrimSpace(probe.Version)
	return len(v) > 0 && (v[0] == '-' || (v[0] >= '0' && v[0] <= '9'))
}

// expandCompactCatalog decodes the compact encoding into the legacy
// provider -> model ID -> model object layout.
func expandCompactCatalog(data []byte) ([]byte, error) {
	var c compactCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse compact models.json: %w", err)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("unsupported compact models.json version %d", c.Version)
	}
	d := &compactDecoder{catalog: c, nodes: make([][]json.RawMessage, len(c.Nodes))}
	for i, raw := range c.Nodes {
		if err := json.Unmarshal(raw, &d.nodes[i]); err != nil {
			return nil, fmt.Errorf("compact node %d: %w", i, err)
		}
	}
	out := make(map[string]map[string]map[string]any, len(c.Providers))
	for _, row := range c.Providers {
		if len(row) != 3 {
			return nil, fmt.Errorf("compact provider row has %d fields, want 3", len(row))
		}
		var provider string
		if err := json.Unmarshal(row[0], &provider); err != nil {
			return nil, fmt.Errorf("compact provider name: %w", err)
		}
		constantsValue, present, err := d.decodeValue(row[1])
		if err != nil {
			return nil, fmt.Errorf("provider %s constants: %w", provider, err)
		}
		constants, _ := constantsValue.(map[string]any)
		if present && constantsValue != nil && constants == nil {
			return nil, fmt.Errorf("provider %s constants are not an object", provider)
		}
		var modelRows [][]json.RawMessage
		if err := json.Unmarshal(row[2], &modelRows); err != nil {
			return nil, fmt.Errorf("provider %s model rows: %w", provider, err)
		}
		models := make(map[string]map[string]any, len(modelRows))
		for _, modelRow := range modelRows {
			if len(modelRow) < 2 {
				return nil, fmt.Errorf("provider %s: compact model row has %d fields", provider, len(modelRow))
			}
			var id string
			if err := json.Unmarshal(modelRow[0], &id); err != nil {
				return nil, fmt.Errorf("provider %s model ID: %w", provider, err)
			}
			ref, ok, err := d.reference(modelRow[1])
			if err != nil || !ok || ref < 0 {
				return nil, fmt.Errorf("provider %s model %s: invalid compact model row", provider, id)
			}
			model := map[string]any{"id": id}
			for key, value := range constants {
				model[key] = value
			}
			if err := d.decodeObjectInto(model, ref); err != nil {
				return nil, fmt.Errorf("provider %s model %s: %w", provider, id, err)
			}
			models[id] = model
		}
		out[provider] = models
	}
	return json.Marshal(out)
}

// reference returns the node ID when raw is a one-element array.
func (d *compactDecoder) reference(raw json.RawMessage) (int, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return 0, false, nil
	}
	var ref []int
	if err := json.Unmarshal(trimmed, &ref); err != nil || len(ref) != 1 {
		return 0, false, fmt.Errorf("invalid node reference %s", trimmed)
	}
	if ref[0] >= len(d.nodes) {
		return 0, false, fmt.Errorf("node reference %d out of range", ref[0])
	}
	return ref[0], true, nil
}

// decodeValue returns the decoded value and whether it is present (a -1
// reference means the key is explicitly absent).
func (d *compactDecoder) decodeValue(raw json.RawMessage) (any, bool, error) {
	ref, isRef, err := d.reference(raw)
	if err != nil {
		return nil, false, err
	}
	if !isRef {
		var scalar any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&scalar); err != nil {
			return nil, false, err
		}
		return scalar, true, nil
	}
	if ref == -1 {
		return nil, false, nil
	}
	if ref < 0 {
		return nil, false, fmt.Errorf("invalid node reference %d", ref)
	}
	d.depth++
	defer func() { d.depth-- }()
	if d.depth > 64 {
		return nil, false, fmt.Errorf("compact catalog nesting too deep")
	}
	node := d.nodes[ref]
	if len(node) == 0 {
		return nil, false, fmt.Errorf("empty node %d", ref)
	}
	var kind int
	if err := json.Unmarshal(node[0], &kind); err != nil {
		return nil, false, fmt.Errorf("node %d kind: %w", ref, err)
	}
	switch kind {
	case 1:
		if len(node) != 2 {
			return nil, false, fmt.Errorf("array node %d malformed", ref)
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(node[1], &entries); err != nil {
			return nil, false, fmt.Errorf("array node %d: %w", ref, err)
		}
		values := make([]any, 0, len(entries))
		for _, entry := range entries {
			value, _, err := d.decodeValue(entry)
			if err != nil {
				return nil, false, err
			}
			// JSON.stringify turns undefined array entries into null.
			values = append(values, value)
		}
		return values, true, nil
	case 0:
		object := map[string]any{}
		if err := d.decodeObjectInto(object, ref); err != nil {
			return nil, false, err
		}
		return object, true, nil
	default:
		return nil, false, fmt.Errorf("node %d has unknown kind %d", ref, kind)
	}
}

func (d *compactDecoder) decodeObjectInto(object map[string]any, ref int) error {
	node := d.nodes[ref]
	if len(node) != 4 {
		return fmt.Errorf("object node %d malformed", ref)
	}
	var kind int
	if err := json.Unmarshal(node[0], &kind); err != nil || kind != 0 {
		return fmt.Errorf("node %d is not an object", ref)
	}
	var properties []json.RawMessage
	if err := json.Unmarshal(node[1], &properties); err != nil || len(properties)%2 != 0 {
		return fmt.Errorf("object node %d properties malformed", ref)
	}
	for i := 0; i < len(properties); i += 2 {
		var keyID int
		if err := json.Unmarshal(properties[i], &keyID); err != nil || keyID < 0 || keyID >= len(d.catalog.Keys) {
			return fmt.Errorf("object node %d has invalid key", ref)
		}
		value, present, err := d.decodeValue(properties[i+1])
		if err != nil {
			return err
		}
		key := d.catalog.Keys[keyID]
		if present {
			object[key] = value
		} else {
			delete(object, key)
		}
	}
	var shapeID int
	if err := json.Unmarshal(node[2], &shapeID); err != nil || shapeID < 0 || shapeID >= len(d.catalog.BooleanShapes) {
		return fmt.Errorf("object node %d has invalid boolean shape", ref)
	}
	var masks []uint32
	if err := json.Unmarshal(node[3], &masks); err != nil {
		return fmt.Errorf("object node %d boolean masks: %w", ref, err)
	}
	for i, keyID := range d.catalog.BooleanShapes[shapeID] {
		if keyID < 0 || keyID >= len(d.catalog.Keys) {
			return fmt.Errorf("boolean shape %d has invalid key", shapeID)
		}
		var mask uint32
		if word := i / 32; word < len(masks) {
			mask = masks[word]
		}
		object[d.catalog.Keys[keyID]] = (mask>>(uint(i)%32))&1 != 0
	}
	return nil
}
