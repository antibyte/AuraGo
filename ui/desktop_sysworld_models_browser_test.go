package ui

import (
	"encoding/base64"
	"fmt"
	"github.com/go-rod/rod"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func verifySystemWorldModels(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(dir, "system-world-review.mjs"))
	if err != nil {
		t.Fatal("run node scripts/build-system-world-review.mjs: ", err)
	}
	page.MustEval(`async source=>{document.querySelector('[data-sw-mode="map"]').click();window.worldModelReview=await import('data:text/javascript;base64,'+source);}`, base64.StdEncoding.EncodeToString(source))
	for _, animated := range []bool{false, true} {
		pages := 4
		for i := 0; i < pages; i++ {
			result := page.MustEval(`async args=>await worldModelReview.reviewSystemWorld(args.page,args.animations)`, map[string]any{"page": i, "animations": animated})
			if animated {
				pages = result.Get("total").Int()
			}
			data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(result.Get("png").Str(), "data:image/png;base64,"))
			if err != nil {
				t.Fatal(err)
			}
			kind := "lod"
			if animated {
				kind = "animation"
			}
			if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("world2-%s-%02d.png", kind, i)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustEval(`()=>document.querySelector('[data-sw-mode="orbit"]').click()`)
}
