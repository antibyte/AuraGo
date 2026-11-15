package storymaker

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
	"testing/fstest"
)

func rewriteModelJSON(t *testing.T, raw []byte, change func(map[string]any)) []byte {
	t.Helper()
	n := int(binary.LittleEndian.Uint32(raw[12:16]))
	var doc map[string]any
	if err := json.Unmarshal(raw[20:20+n], &doc); err != nil {
		t.Fatal(err)
	}
	change(doc)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for len(data)%4 != 0 {
		data = append(data, ' ')
	}
	out := append([]byte(nil), raw[:20]...)
	out = append(out, data...)
	out = append(out, raw[20+n:]...)
	binary.LittleEndian.PutUint32(out[8:12], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[12:16], uint32(len(data)))
	return out
}

func TestCatalogModelValidation(t *testing.T) {
	for _, id := range []string{"hoodie", "professor", "punk"} {
		t.Run(id, func(t *testing.T) {
			raw, err := os.ReadFile("../../ui/img/storymaker/models/" + id + ".glb")
			if err != nil {
				t.Fatal(err)
			}
			if err := validateCatalogModel(raw); err != nil {
				t.Fatal(err)
			}
			for name, mutate := range map[string]func(map[string]any){
				"external": func(d map[string]any) {
					d["buffers"].([]any)[0].(map[string]any)["uri"] = "https://example.invalid/model.bin"
				},
				"oversized buffer": func(d map[string]any) { d["bufferViews"].([]any)[0].(map[string]any)["byteLength"] = 1 << 30 },
				"cycle":            func(d map[string]any) { d["nodes"].([]any)[0].(map[string]any)["children"] = []int{0} },
				"image type":       func(d map[string]any) { d["images"].([]any)[0].(map[string]any)["mimeType"] = "image/svg+xml" },
				"allocation":       func(d map[string]any) { d["accessors"].([]any)[0].(map[string]any)["count"] = 100000000 },
			} {
				if validateCatalogModel(rewriteModelJSON(t, raw, mutate)) == nil {
					t.Errorf("accepted %s", name)
				}
			}
			if validateCatalogModel(raw[:len(raw)-1]) == nil {
				t.Fatal("accepted truncated binary")
			}
		})
	}
}

func TestCatalogCharacterAliases(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-mini/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var character map[string]any
	for _, value := range doc["entries"].([]any) {
		entry := value.(map[string]any)
		if entry["kind"] == "character" {
			character = entry
			break
		}
	}
	load := func(aliases []string) (*Catalog, error) {
		character["aliases"] = aliases
		data, _ := json.Marshal(doc)
		return LoadCatalog(fstest.MapFS{"catalog.json": &fstest.MapFile{Data: data}}, "catalog.json")
	}
	c, err := load([]string{"char.retired-person"})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.Lookup("char.retired-person")
	if !ok || entry.ID != character["id"] {
		t.Fatalf("alias lookup: %+v", entry)
	}
	for _, entry := range c.Search("", "character", 12) {
		if entry.ID == "char.retired-person" {
			t.Fatal("alias leaked into picker/search")
		}
	}
	for _, aliases := range [][]string{{"char.retired-person", "char.retired-person"}, {character["id"].(string)}, {"char.bad/path"}, {"char.BAD"}, {"char."}} {
		if _, err := load(aliases); err == nil {
			t.Errorf("accepted aliases %v", aliases)
		}
	}
}
