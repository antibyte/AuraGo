package desktop

import (
	"context"
	"testing"
)

func TestGalleryCategoriesValidatePersistAndConflict(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	categories, version, err := svc.GalleryCategories(ctx)
	if err != nil || len(categories) != 0 {
		t.Fatalf("initial categories: %v %v", categories, err)
	}
	rules := []GalleryCategory{{Name: "Music covers", Keywords: []string{"cover", "noisemaker"}}}
	if ok, err := svc.SaveGalleryCategories(ctx, rules, version); !ok || err != nil {
		t.Fatalf("save: %v %v", ok, err)
	}
	if ok, err := svc.SaveGalleryCategories(ctx, nil, version); ok || err != nil {
		t.Fatalf("stale write accepted: %v %v", ok, err)
	}
	categories, version, err = svc.GalleryCategories(ctx)
	if err != nil || len(categories) != 1 || categories[0].Name != "Music covers" {
		t.Fatalf("read: %v %v", categories, err)
	}
	if ok, err := svc.SaveGalleryCategories(ctx, []GalleryCategory{}, version); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}
	for _, raw := range []string{`{}`, `[{"name":"", "keywords":["x"]}]`, `[{"name":"X", "keywords":[]}]`, `[{"name":"X", "keywords":[" "]}]`, `[{"name":"X", "keywords":["a"]},{"name":"x", "keywords":["b"]}]`} {
		if err := validateDesktopSetting("gallery.categories", raw); err == nil {
			t.Fatalf("accepted invalid categories: %s", raw)
		}
	}
	svc.SetReadOnly(true)
	_, version, _ = svc.GalleryCategories(ctx)
	if ok, err := svc.SaveGalleryCategories(ctx, rules, version); ok || err == nil {
		t.Fatal("readonly accepted category write")
	}
}
