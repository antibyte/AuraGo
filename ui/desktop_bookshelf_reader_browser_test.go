package ui

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func bookshelfTestEPUB(t *testing.T, origin string) []byte {
	t.Helper()
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"book.opf":               `<package version="3.0" unique-identifier="id" xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">bookshelf-test</dc:identifier><dc:title>Reader fixture</dc:title><dc:creator>AuraGo</dc:creator><dc:language>en</dc:language></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="two.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="one"/><itemref idref="two"/></spine></package>`,
		"nav.xhtml":              `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="one.xhtml">First chapter</a></li><li><a href="two.xhtml">Second chapter</a></li></ol></nav></body></html>`,
		"one.xhtml":              `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>First chapter</title><style>@import url("` + origin + `/sink");body{background-image:url("` + origin + `/sink")}</style><script>top.bookshelfEscaped=true;fetch("` + origin + `/sink")</script></head><body onload="top.bookshelfEscaped=true"><h1>First chapter</h1><p>Opening page with a unique lighthouse phrase.</p><img src="` + origin + `/sink" onerror="top.bookshelfEscaped=true"/><iframe src="` + origin + `/sink"></iframe><form action="` + origin + `/sink"><input autofocus="autofocus"/></form><a href="two.xhtml">Continue</a></body></html>`,
		"two.xhtml":              `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Second chapter</title></head><body><h1>Second chapter</h1><p>The second lighthouse stands beyond the harbor.</p></body></html>`,
	}
	files["book.opf"] = strings.Replace(files["book.opf"], "</manifest>", `<item id="picture" href="picture.png" media-type="image/png"/></manifest>`, 1)
	files["one.xhtml"] = strings.Replace(files["one.xhtml"], "</body>", `<img src="picture.png" alt="Embedded illustration"/><a xmlns:epub="http://www.idpf.org/2007/ops" epub:type="noteref" href="#note">1</a><aside xmlns:epub="http://www.idpf.org/2007/ops" epub:type="footnote" id="note"><p>A local note.</p></aside></body>`, 1)
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 40, 60))); err != nil {
		t.Fatal(err)
	}
	files["picture.png"] = picture.String()
	for name, body := range files {
		writer, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestDesktopBookshelfReaderBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	var sink, exceptions atomic.Int32
	var epub []byte
	formats := bookshelfFormatFixtures(t)
	corpus := []string{"txt", "html", "fb2", "fb2.zip", "mobi", "cbz", "pdf", "epub-fixed"}
	if fixture := os.Getenv("AURAGO_BOOKSHELF_KF8_FIXTURE"); fixture != "" {
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		formats["azw3"] = data
		corpus = append(corpus, "azw3")
	}
	shell, err := os.ReadFile("bookshelf-reader.html")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/sink", func(w http.ResponseWriter, r *http.Request) { sink.Add(1); w.WriteHeader(204) })
	mux.HandleFunc("/api/desktop/bookshelf/books/fixture/content", func(w http.ResponseWriter, r *http.Request) {
		data := epub
		if f := r.URL.Query().Get("format"); f != "" {
			data = formats[f]
		}
		http.ServeContent(w, r, "fixture", time.Time{}, bytes.NewReader(data))
	})
	mux.HandleFunc("/api/desktop/bookshelf/reader", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src http://"+r.Host+"/js/vendor/foliate/reader.bundle.js http://"+r.Host+"/js/desktop/apps/bookshelf/reader-host.js; style-src 'unsafe-inline' blob:; img-src blob: data:; font-src blob: data:; connect-src blob:; frame-src data: blob:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
		w.Write(shell)
	})
	mux.HandleFunc("/bookshelf-reader-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>Bookshelf reader fixture</title><style>html,body{margin:0;height:100%}#host{height:600px;width:800px}</style><div id="host"></div><script src="/js/desktop/apps/bookshelf/reader.js"></script><script>
        window.progress=[];window.messages=[];window.bookshelfEscaped=false;
        window.openBook=async function(saved){window.controller?.dispose();window.readerError=null;try{window.controller=await BookshelfReader.create(document.querySelector('#host'),{book:{title:'Reader fixture',format:'epub'},contentURL:'/api/desktop/bookshelf/books/fixture/content',progress:saved,onProgress:p=>progress.push(p),onReady:r=>window.readerReady=r,onError:e=>window.readerError=e.message,settings:{fontSize:19}});return true}catch(e){window.readerError=e.message;return false}};
        window.openBook();</script>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	epub = bookshelfTestEPUB(t, srv.URL)
	formats["epub-fixed"] = bookshelfFixedFixture(t, epub)
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/bookshelf-reader-fixture").Timeout(90 * time.Second)
	defer page.Close()
	go page.EachEvent(func(e *proto.RuntimeExceptionThrown) {
		exceptions.Add(1)
		t.Logf("browser exception: %+v", e.ExceptionDetails)
	}, func(e *proto.RuntimeConsoleAPICalled) {
		if e.Type == "error" || e.Type == "warn" {
			t.Logf("browser console: %+v", e.Args)
		}
	})()
	page.MustWaitLoad()
	if err := page.Timeout(50 * time.Second).Wait(rod.Eval(`()=>!!(window.controller||window.readerError)`)); err != nil {
		t.Fatal(err, page.MustEval(`()=>({error:window.readerError,ready:window.readerReady,progress:window.progress})`))
	}
	if message := page.MustEval(`()=>window.readerError||''`).Str(); message != "" {
		t.Fatal(message)
	}
	if n := page.MustEval(`()=>window.readerReady.toc.length`).Int(); n != 2 {
		t.Fatalf("TOC entries: %d", n)
	}
	if result := page.MustEval(`async()=>await controller.search('lighthouse')`).Arr(); len(result) != 2 {
		t.Fatalf("search results: %v", result)
	}
	page.MustEval(`async()=>{await controller.goTo('two.xhtml');await controller.setPreferences({fontSize:24,theme:'dark'});}`)
	page.MustWait(`()=>progress.length>0&&progress[progress.length-1].fraction>0`)
	saved := page.MustEval(`()=>progress[progress.length-1]`)
	if !strings.Contains(saved.Get("locator.cfi").Str(), "epubcfi") {
		t.Fatal("missing stable CFI", saved)
	}
	page.MustEval(`async saved=>{window.saved=saved;return await openBook(saved)}`, saved)
	if message := page.MustEval(`()=>window.readerError||''`).Str(); message != "" {
		t.Fatal(message)
	}
	if !page.MustEval(`()=>{try{document.querySelector('iframe').contentDocument.querySelector('iframe').contentWindow.document;return false}catch{return true}}`).Bool() {
		t.Fatal("reader frame is not opaque")
	}
	if page.MustEval(`()=>window.bookshelfEscaped`).Bool() || sink.Load() != 0 {
		t.Fatalf("book crossed isolation boundary: %d requests", sink.Load())
	}
	engine := page.MustElement("#host > iframe").MustFrame().MustElement("iframe").MustFrame()
	if !engine.MustEval(`()=>{try{top.document.body;return false}catch{return true}}`).Bool() {
		t.Fatal("engine can reach Desktop document")
	}
	for _, format := range corpus {
		t.Run(format, func(t *testing.T) {
			result := page.MustEval(`async format=>{controller.dispose();window.readerReady=null;window.readerError=null;window.progress=[];const readerFormat=format==='epub-fixed'?'epub':format;try{controller=await BookshelfReader.create(document.querySelector('#host'),{book:{title:'Corpus',format:readerFormat,path:'fixture.'+readerFormat},contentURL:'/api/desktop/bookshelf/books/fixture/content?format='+encodeURIComponent(format),onReady:r=>window.readerReady=r,onProgress:p=>progress.push(p),onError:e=>window.readerError=e.message});return ''}catch(e){return e.message}}`, format).Str()
			if result != "" {
				t.Fatal(result)
			}
			if !page.MustEval(`()=>!!window.readerReady`).Bool() {
				t.Fatal("reader never ready")
			}
			if format == "epub-fixed" {
				if !page.MustEval(`()=>readerReady.isFixedLayout`).Bool() {
					t.Fatal("fixed-layout EPUB used reflow renderer")
				}
				page.MustEval(`async()=>{await controller.setPreferences({fit:'page',spread:true,direction:'rtl'});await controller.next();await controller.prev()}`)
			}
			if format == "cbz" || format == "pdf" {
				if !page.MustEval(`async()=>{const blob=await controller.getThumbnail(0);return blob instanceof Blob&&blob.size>0}`).Bool() {
					t.Fatal("missing page thumbnail")
				}
				page.MustEval(`async()=>{await controller.setPreferences({fit:'page',spread:true,direction:'rtl'});await controller.next();await controller.prev();}`)
			}
			if format != "cbz" {
				query := "lighthouse"
				if format == "azw3" {
					query = "Rabbit"
				}
				if hits := page.MustEval(`async query=>await controller.search(query)`, query).Arr(); len(hits) == 0 {
					t.Fatal("search missing corpus text")
				}
			}
		})
	}
	page.MustEval(`()=>controller.dispose()`)
	if page.MustEval(`()=>document.querySelectorAll('#host iframe').length`).Int() != 0 {
		t.Fatal("frame leaked after disposal")
	}
	if exceptions.Load() != 0 {
		t.Fatalf("%d unexpected reader exceptions", exceptions.Load())
	}
}

func bookshelfFixedFixture(t *testing.T, source []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, entry := range reader.File {
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if entry.Name == "book.opf" {
			text = strings.Replace(text, "</metadata>", `<meta property="rendition:layout">pre-paginated</meta></metadata>`, 1)
			text = strings.Replace(text, "<spine>", `<spine page-progression-direction="rtl">`, 1)
		} else if strings.HasSuffix(entry.Name, ".xhtml") {
			text = strings.Replace(text, "<head>", `<head><meta name="viewport" content="width=600, height=800"/>`, 1)
		}
		out, err := writer.Create(entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func bookshelfFormatFixtures(t *testing.T) map[string][]byte {
	makeZip := func(entries map[string][]byte) []byte {
		var out bytes.Buffer
		z := zip.NewWriter(&out)
		for name, data := range entries {
			w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	fb2 := []byte(`<?xml version="1.0" encoding="utf-8"?><FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><genre>adventure</genre><author><first-name>Aura</first-name><last-name>Go</last-name></author><book-title>Corpus</book-title><lang>en</lang></title-info></description><body><section><title><p>Chapter</p></title><p>The lighthouse shines over the harbor.</p></section></body></FictionBook>`)
	var cover bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 240, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 240; x++ {
			picture.Set(x, y, color.RGBA{40, 90, 120, 255})
		}
	}
	if err := png.Encode(&cover, picture); err != nil {
		t.Fatal(err)
	}
	text := []byte(`<html><head><title>Corpus</title></head><body><h1>Chapter</h1><p>The lighthouse shines over the harbor.</p></body></html>`)
	header := make([]byte, 272)
	binary.BigEndian.PutUint16(header[0:], 1)
	binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
	binary.BigEndian.PutUint16(header[8:], 1)
	binary.BigEndian.PutUint16(header[10:], 4096)
	copy(header[16:], "MOBI")
	binary.BigEndian.PutUint32(header[20:], 248)
	binary.BigEndian.PutUint32(header[28:], 65001)
	binary.BigEndian.PutUint32(header[32:], 42)
	binary.BigEndian.PutUint32(header[36:], 6)
	binary.BigEndian.PutUint32(header[84:], 264)
	binary.BigEndian.PutUint32(header[88:], 6)
	binary.BigEndian.PutUint32(header[108:], 2)
	binary.BigEndian.PutUint32(header[244:], 0xffffffff)
	copy(header[264:], "Corpus")
	pdb := make([]byte, 94)
	copy(pdb, "Corpus")
	copy(pdb[60:], "BOOKMOBI")
	binary.BigEndian.PutUint16(pdb[76:], 2)
	binary.BigEndian.PutUint32(pdb[78:], 94)
	binary.BigEndian.PutUint32(pdb[86:], 94+uint32(len(header)))
	mobi := append(append(pdb, header...), text...)
	var pdf bytes.Buffer
	fmt.Fprint(&pdf, "%PDF-1.4\n")
	offsets := []int{0}
	stream := "BT /F1 18 Tf 30 130 Td (The lighthouse shines.) Tj ET"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream), "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"}
	for i, obj := range objects {
		offsets = append(offsets, pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xref)
	return map[string][]byte{"txt": []byte("The lighthouse shines over the harbor."), "html": text, "fb2": fb2, "fb2.zip": makeZip(map[string][]byte{"book.fb2": fb2}), "mobi": mobi, "cbz": makeZip(map[string][]byte{"001.png": cover.Bytes(), "002.png": cover.Bytes()}), "pdf": pdf.Bytes()}
}
