package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDesktopArchiveActiveContentBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writeDesktopTestZip(t, svc, "Documents/payload.zip", map[string][]byte{
		"page.html": []byte(`<script>fetch('/attack');window.pwned=true</script><form action="/attack"></form>`),
		"code.js":   []byte(`window.pwned=true;fetch('/attack')`),
		"image.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="fetch('/attack');window.pwned=true"><script>fetch('/attack')</script><image href="/attack"/></svg>`),
		"image.png": desktopAuditPNG(t),
	})
	var attacks atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/desktop/archive/entry", handleDesktopArchiveEntry(s))
	mux.HandleFunc("/attack", func(w http.ResponseWriter, r *http.Request) { attacks.Add(1); w.WriteHeader(204) })
	const entry = "/api/desktop/archive/entry?path=Documents/payload.zip&entry="
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!doctype html><script>window.pwned=false</script><script src="%scode.js"></script><iframe src="%spage.html"></iframe><iframe src="%simage.svg"></iframe><img id="preview" src="%simage.png">`, entry, entry, entry, entry)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage().Timeout(30 * time.Second)
	defer page.Close()
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	if !page.MustEval(`()=>!window.pwned&&document.getElementById('preview').naturalWidth===2`).Bool() {
		t.Fatal("archive embedding executed script or broke PNG")
	}
	page.MustNavigate(origin.URL + entry + "page.html").MustWaitLoad()
	if !page.MustEval(`()=>typeof window.pwned==='undefined'&&document.body.textContent.includes('<script>')&&!document.querySelector('script,form')`).Bool() {
		t.Fatal("direct HTML navigation executed content")
	}
	page.MustNavigate(origin.URL + entry + "image.svg").MustWaitLoad()
	if !page.MustEval(`()=>typeof window.pwned==='undefined'`).Bool() {
		t.Fatal("direct SVG navigation executed content")
	}
	if attacks.Load() != 0 {
		t.Fatalf("archive made %d active requests", attacks.Load())
	}
}
