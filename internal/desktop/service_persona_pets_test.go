package desktop

import (
	"crypto/sha256"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/webp"
)

// Exercise the actual shipped pixels, including every frame the runtime selects.
func TestBundledPersonaSprites(t *testing.T) {
	counts := []int{6, 8, 8, 4, 5, 8, 6, 6, 6}
	personas := 0
	for _, pet := range bundledDefaultPets() {
		if !strings.HasPrefix(pet.Manifest.ID, "aurago-") {
			continue
		}
		personas++
		t.Run(pet.Manifest.ID, func(t *testing.T) {
			if pet.Manifest.Category != "persona" {
				t.Fatal("persona missing from catalog category")
			}
			file, err := os.Open(filepath.Join("pets_assets", pet.Manifest.ID, "spritesheet.webp"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			sheet, err := webp.Decode(file)
			if err != nil {
				t.Fatal(err)
			}
			if sheet.Bounds().Dx() != 1536 || sheet.Bounds().Dy() != 1872 {
				t.Fatalf("wrong OpenPets dimensions: %v", sheet.Bounds())
			}
			for row, frames := range counts {
				unique := map[[32]byte]bool{}
				for col := 0; col < frames; col++ {
					pixels := make([]byte, 0, 192*208*4)
					opaque := 0
					for y := 0; y < 208; y++ {
						for x := 0; x < 192; x++ {
							p := color.NRGBAModel.Convert(sheet.At(col*192+x, row*208+y)).(color.NRGBA)
							pixels = append(pixels, p.R, p.G, p.B, p.A)
							if p.A > 240 {
								opaque++
							}
							if (x < 3 || x > 188 || y < 3 || y > 204) && p.A != 0 {
								t.Fatalf("nontransparent cell border at row %d frame %d", row, col)
							}
						}
					}
					if opaque < 800 {
						t.Fatalf("empty or translucent frame at row %d frame %d", row, col)
					}
					unique[sha256.Sum256(pixels)] = true
				}
				if len(unique) < 2 {
					t.Fatalf("row %d has no animated frame variation", row)
				}
			}
		})
	}
	if personas != 12 {
		t.Fatalf("got %d persona pets, want 12", personas)
	}
}
