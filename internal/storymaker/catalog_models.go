package storymaker

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
)

// validateCatalogModel permits only bounded, self-contained glTF 2 models. The
// browser repeats these checks before allocating geometry or decoding textures.
func validateCatalogModel(data []byte) error {
	bad := func() error { return fmt.Errorf("invalid or unsupported Storymaker GLB") }
	if len(data) < 28 || len(data) > CatalogTextureLimit {
		return bad()
	}
	u32 := func(at int) uint32 { return binary.LittleEndian.Uint32(data[at : at+4]) }
	n := int(u32(12))
	if u32(0) != 0x46546c67 || u32(4) != 2 || int(u32(8)) != len(data) || u32(16) != 0x4e4f534a || n > 1<<20 || n%4 != 0 || n+28 > len(data) {
		return bad()
	}
	if u32(n+24) != 0x004e4942 || uint64(n+28)+uint64(u32(n+20)) != uint64(len(data)) {
		return bad()
	}
	var raw any
	if json.Unmarshal(data[20:20+n], &raw) != nil {
		return bad()
	}
	fields := 0
	var safe func(any) bool
	safe = func(v any) bool {
		switch v := v.(type) {
		case map[string]any:
			for k, value := range v {
				fields++
				if fields > 50000 || k == "uri" || k == "__proto__" || k == "extensionsRequired" || !safe(value) {
					return false
				}
			}
		case []any:
			for _, value := range v {
				fields++
				if fields > 50000 || !safe(value) {
					return false
				}
			}
		}
		return true
	}
	if !safe(raw) {
		return bad()
	}
	var model struct {
		Asset       struct{ Version string }
		Buffers     []struct{ ByteLength int }
		BufferViews []struct{ Buffer, ByteOffset, ByteLength int }
		Images      []struct {
			BufferView int
			MimeType   string
		}
		Nodes  []struct{ Children []int }
		Meshes []struct {
			Primitives []struct {
				Attributes map[string]int
				Indices    *int
				Mode       *int
				Targets    []json.RawMessage
			}
		}
		Skins     []struct{ Joints []int }
		Accessors []struct {
			Count, ComponentType int
			Type                 string
		}
	}
	if json.Unmarshal(data[20:20+n], &model) != nil {
		return bad()
	}
	bin := data[n+28:]
	if model.Asset.Version != "2.0" || len(model.Buffers) != 1 || model.Buffers[0].ByteLength > len(bin) || model.Buffers[0].ByteLength < len(bin)-3 || len(model.Nodes) == 0 || len(model.Nodes) > 128 || len(model.Meshes) == 0 || len(model.Meshes) > 16 || len(model.Skins) == 0 || len(model.Skins) > 4 || len(model.Images) > 8 || len(model.Accessors) > 512 || len(model.BufferViews) > 512 {
		return bad()
	}
	for _, v := range model.BufferViews {
		if v.Buffer != 0 || v.ByteOffset < 0 || v.ByteLength < 1 || v.ByteOffset > len(bin) || v.ByteLength > len(bin)-v.ByteOffset {
			return bad()
		}
	}
	elements := 0
	for _, a := range model.Accessors {
		if a.Count < 1 || a.Count > 180000 || a.ComponentType < 5120 || a.ComponentType > 5126 || a.ComponentType == 5124 {
			return bad()
		}
		switch a.Type {
		case "SCALAR", "VEC2", "VEC3", "VEC4", "MAT4":
		default:
			return bad()
		}
		elements += a.Count
	}
	if elements > 3000000 {
		return bad()
	}
	marks := make([]uint8, len(model.Nodes))
	var visit func(int) bool
	visit = func(i int) bool {
		if i < 0 || i >= len(marks) || marks[i] == 1 {
			return false
		}
		if marks[i] == 2 {
			return true
		}
		marks[i] = 1
		for _, child := range model.Nodes[i].Children {
			if !visit(child) {
				return false
			}
		}
		marks[i] = 2
		return true
	}
	for i := range model.Nodes {
		if !visit(i) {
			return bad()
		}
	}
	for _, skin := range model.Skins {
		if len(skin.Joints) == 0 || len(skin.Joints) > 72 {
			return bad()
		}
		for _, joint := range skin.Joints {
			if joint < 0 || joint >= len(model.Nodes) {
				return bad()
			}
		}
	}
	triangles := 0
	for _, mesh := range model.Meshes {
		for _, p := range mesh.Primitives {
			i, ok := p.Attributes["POSITION"]
			if !ok || i < 0 || i >= len(model.Accessors) || model.Accessors[i].Count > 90000 || p.Mode != nil && *p.Mode != 4 || len(p.Targets) > 16 {
				return bad()
			}
			if p.Indices != nil {
				i = *p.Indices
				if i < 0 || i >= len(model.Accessors) {
					return bad()
				}
			}
			triangles += model.Accessors[i].Count / 3
		}
	}
	if triangles > 60000 {
		return bad()
	}
	for _, img := range model.Images {
		if img.BufferView < 0 || img.BufferView >= len(model.BufferViews) || img.MimeType != "image/png" && img.MimeType != "image/jpeg" {
			return bad()
		}
		v := model.BufferViews[img.BufferView]
		config, format, err := image.DecodeConfig(bytes.NewReader(bin[v.ByteOffset : v.ByteOffset+v.ByteLength]))
		if err != nil || "image/"+format != img.MimeType || config.Width < 1 || config.Height < 1 || config.Width > 1024 || config.Height > 1024 {
			return bad()
		}
	}
	return nil
}
