package atlaspdf

import (
	"bytes"
	"image"
	"image/png"
	"io/fs"
	"strings"
	"sync"

	"aurago/internal/infra"

	"github.com/phpdave11/gofpdf"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const iconPixels = 128

// iconPNG caches the downscaled PNG of every icon that decoded, by icon id.
var iconPNG sync.Map

// iconSet registers icon images once per document.
type iconSet struct {
	fsys       fs.FS
	pdf        *gofpdf.Fpdf
	resolved   map[string]string // candidate list -> registered image name, "" when unavailable
	registered map[string]bool
}

func newIconSet(fsys fs.FS, pdf *gofpdf.Fpdf) *iconSet {
	return &iconSet{fsys: fsys, pdf: pdf, resolved: map[string]string{}, registered: map[string]bool{}}
}

func validIconID(id string) bool {
	if id == "" || len(id) > 40 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// loadPNG reads img/infra/<id>.webp and returns it as a PNG of at most iconPixels.
func loadPNG(fsys fs.FS, id string) ([]byte, bool) {
	if v, ok := iconPNG.Load(id); ok {
		return v.([]byte), true
	}
	if fsys == nil || !validIconID(id) {
		return nil, false
	}
	raw, err := fs.ReadFile(fsys, "img/infra/"+id+".webp")
	if err != nil {
		return nil, false
	}
	src, err := webp.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, false
	}
	w, h := b.Dx(), b.Dy()
	if m := max(w, h); m > iconPixels {
		w, h = max(1, w*iconPixels/m), max(1, h*iconPixels/m)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, false
	}
	iconPNG.Store(id, out.Bytes())
	return out.Bytes(), true
}

// register returns the PDF image name for the first loadable candidate.
func (s *iconSet) register(candidates ...string) string {
	key := strings.Join(candidates, "|")
	if name, ok := s.resolved[key]; ok {
		return name
	}
	name := ""
	for _, id := range candidates {
		data, ok := loadPNG(s.fsys, id)
		if !ok {
			continue
		}
		name = "ico-" + id
		if !s.registered[id] {
			s.registered[id] = true
			s.pdf.RegisterImageOptionsReader(name, gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(data))
		}
		break
	}
	s.resolved[key] = name
	return name
}

func defaultIcon(k infra.NodeKind) string {
	switch k {
	case infra.NodeVM:
		return "vm"
	case infra.NodeContainer:
		return "container"
	case infra.NodeNetwork:
		return "lan"
	case infra.NodeService:
		return "generic-app"
	case infra.NodeSite, infra.NodeRoom, infra.NodeRack, infra.NodeGroup:
		return string(k)
	}
	return "generic-device"
}

// iconName is the registered image for a node, or "" for the vector fallback.
func (d *doc) iconName(n *infra.Node) string {
	if isPlace(n.Kind) {
		return d.icons.register(string(n.Kind))
	}
	return d.icons.register(n.Icon, defaultIcon(n.Kind), "generic-device")
}

// drawIcon paints the node icon in a size x size square at (x, y).
func (d *doc) drawIcon(n *infra.Node, x, y, size float64) {
	d.drawIconKind(d.iconName(n), n.Kind, x, y, size)
}

func (d *doc) drawIconKind(name string, kind infra.NodeKind, x, y, size float64) {
	if size <= 0 {
		return
	}
	if name != "" {
		d.pdf.ImageOptions(name, x, y, size, size, false, gofpdf.ImageOptions{ImageType: "PNG", AllowNegativePosition: true}, 0, "")
		return
	}
	d.fill(kindColor(kind))
	d.rrect(x, y, size, size, size*0.22, "F")
	if size >= 2.2 {
		initial := "?"
		if kind != "" {
			initial = strings.ToUpper(string(kind[:1]))
		}
		d.text(fontBold, size/ptMM*0.5, colWhite, x+size/2, y+size/2, initial, 'C')
	}
}

// drawPlaceIcon paints the icon of a place kind.
func (d *doc) drawPlaceIcon(kind infra.NodeKind, x, y, size float64) {
	d.drawIconKind(d.icons.register(string(kind)), kind, x, y, size)
}
