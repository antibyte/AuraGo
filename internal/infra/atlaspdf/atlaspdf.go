// Package atlaspdf renders the Infra graph as a multi-page, vector PDF: a map
// overview with detail pages, hosts and guests, an inventory and a connection table.
package atlaspdf

import (
	"bytes"
	"fmt"
	"io/fs"
	"time"

	"aurago/internal/infra"

	"github.com/phpdave11/gofpdf"
)

// Paper is a landscape page size.
type Paper string

const (
	PaperA4     Paper = "a4"
	PaperA3     Paper = "a3"
	PaperLetter Paper = "letter"
)

// Options selects what is rendered and how.
type Options struct {
	Paper                 Paper
	Map, Hosts, Inventory bool
	// Text translates a label key; blank answers and the key itself mean "use the English default".
	Text func(key string) string
	// Icons is a root holding img/infra/<id>.webp; nil or a missing file selects the vector fallback.
	Icons fs.FS
	// CJKFonts holds NotoSansSC-Regular.ttf and NotoSansSC-Bold.ttf for text the Go fonts cannot print.
	CJKFonts fs.FS
	Now      time.Time
	Title    string
}

const (
	fontRegular = "gor"
	fontMedium  = "gom"
	fontBold    = "gob"
	fontMono    = "mono"

	ptMM = 25.4 / 72
)

type pageInfo struct{ title string }

type doc struct {
	pdf      *gofpdf.Fpdf
	opts     Options
	lb       *labels
	m        *model
	sc       *scene
	W, H     float64
	content  rect
	k        float64 // page-size factor against A4
	textCov  *coverage
	monoCov  *coverage
	ellipsis string
	fonts    fontSet
	dirty    bool // graphics state was reset behind gofpdf's back
	pages    []pageInfo
	title    string
	icons    *iconSet
	mapPages []mapPage
	islands  []island
	lineIdx  map[string][]int
	first    map[string]int
}

const (
	marginX    = 12.0
	headerBand = 10.0
	footerBand = 8.0
)

// Render draws snapshot as a PDF.
func Render(snapshot infra.Snapshot, opts Options) ([]byte, error) {
	if !opts.Map && !opts.Hosts && !opts.Inventory {
		opts.Map, opts.Hosts, opts.Inventory = true, true, true
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	m, err := newModel(snapshot)
	if err != nil {
		return nil, err
	}
	d, err := newDoc(opts, m)
	if err != nil {
		return nil, err
	}
	d.sc = buildScene(m, d.lb.get("desktop.infra_pdf_unplaced"))
	if opts.Map {
		d.drawMap()
	}
	if opts.Hosts {
		d.drawHosts()
	}
	if opts.Inventory {
		d.drawInventory()
		d.drawConnections()
	}
	if len(d.pages) == 0 {
		d.emptyPage()
	}
	d.drawChrome()
	var out bytes.Buffer
	if err := d.pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("atlaspdf: %w", err)
	}
	return out.Bytes(), nil
}

func paperSize(p Paper) (string, float64) {
	switch p {
	case PaperA3:
		return "A3", 420.0 / 297
	case PaperLetter:
		return "Letter", 279.4 / 297
	}
	return "A4", 1
}

func newDoc(opts Options, m *model) (*doc, error) {
	size, k := paperSize(opts.Paper)
	d := &doc{opts: opts, m: m, lb: newLabels(opts.Text), k: k, first: map[string]int{}}
	if err := d.chooseFonts(); err != nil {
		return nil, err
	}
	d.title = opts.Title
	if d.title == "" {
		d.title = d.lb.get("desktop.infra_pdf_title")
	}
	d.title = d.textCov.clean(d.title)
	pdf := gofpdf.New("L", "mm", size, "")
	d.pdf = pdf
	d.W, d.H = pdf.GetPageSize()
	d.content = rect{marginX, marginX + headerBand, d.W - 2*marginX, d.H - 2*marginX - headerBand - footerBand}
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetCompression(true)
	pdf.SetCreationDate(opts.Now)
	pdf.SetTitle(d.title, true)
	pdf.SetAuthor("AuraGo Infra", true)
	pdf.SetSubject(d.title, true)
	pdf.SetCreator("AuraGo Infra", true)
	d.icons = newIconSet(opts.Icons, pdf)
	d.registerFonts()
	return d, nil
}
