package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const homedashLabMaxPanelHeight = 7000

type homedashLabCount struct {
	Checked  int      `json:"checked"`
	Failures []string `json:"failures"`
}

// homedashLabProbes mirrors the JSON of hdLab.probes() (contract C6).
type homedashLabProbes struct {
	Contrast struct {
		Checked    int      `json:"checked"`
		Failures   []string `json:"failures"`
		Warnings   []string `json:"warnings"`
		Unmeasured int      `json:"unmeasured"`
	} `json:"contrast"`
	Hit        homedashLabCount `json:"hit"`
	Shift      homedashLabCount `json:"shift"`
	Animations struct {
		Checked  bool     `json:"checked"`
		Running  []string `json:"running"`
		Failures []string `json:"failures"`
	} `json:"animations"`
	Backdrop struct {
		Count    int      `json:"count"`
		Limit    int      `json:"limit"`
		Failures []string `json:"failures"`
	} `json:"backdrop"`
	Resolution homedashLabCount           `json:"resolution"`
	Extra      map[string]json.RawMessage `json:"extra"`
}

type homedashLabPanelReport struct {
	Label     string                     `json:"label"`
	URL       string                     `json:"url"`
	Probes    *homedashLabProbes         `json:"probes"`
	Extra     map[string]json.RawMessage `json:"extra,omitempty"`
	Errors    []string                   `json:"errors"`
	Oversized []string                   `json:"oversized"`
}

type homedashLabSheetReport struct {
	ID     string                   `json:"id"`
	Images []string                 `json:"images"`
	Panels []homedashLabPanelReport `json:"panels"`
}

type homedashLabReport struct {
	Generated  string                   `json:"generated"`
	ReportOnly bool                     `json:"report_only"`
	Sheets     []homedashLabSheetReport `json:"sheets"`
	Failures   []string                 `json:"failures"`
}

// failures lists every problem of the panel as a readable string.
func (r homedashLabPanelReport) failures() []string {
	var out []string
	add := func(prefix string, list []string) {
		for _, s := range list {
			out = append(out, prefix+" "+s)
		}
	}
	add("error:", r.Errors)
	add("size:", r.Oversized)
	if p := r.Probes; p != nil {
		add("contrast:", p.Contrast.Failures)
		add("hit:", p.Hit.Failures)
		add("shift:", p.Shift.Failures)
		add("animations:", p.Animations.Failures)
		add("backdrop:", p.Backdrop.Failures)
		add("resolution:", p.Resolution.Failures)
	}
	keys := make([]string, 0, len(r.Extra))
	for k := range r.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var v struct {
			OK      *bool  `json:"ok"`
			Message string `json:"message"`
		}
		if json.Unmarshal(r.Extra[k], &v) != nil || v.OK == nil || *v.OK {
			continue
		}
		msg := v.Message
		if msg == "" {
			msg = string(r.Extra[k])
		}
		out = append(out, "extra "+k+": "+msg)
	}
	return out
}

// newHomeDashLabServer serves ui/ with the two CSS bundles assembled from their parts on every request,
// so an edit shows up on the next page load.
func newHomeDashLabServer(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	for _, name := range []string{"homedash.bundle.css", "desktop-shell.bundle.css"} {
		parts := desktopBundleParts(t, "ui/css/"+name)
		if _, err := themeSweepConcat(parts, false); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		mux.HandleFunc("/css/"+name, func(w http.ResponseWriter, r *http.Request) {
			body, err := themeSweepConcat(parts, false)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/css")
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprint(w, body)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

func homedashLabViewport(t *testing.T, page *rod.Page, height int, dpr float64) {
	t.Helper()
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: homedashLabWidth, Height: height, DeviceScaleFactor: dpr}); err != nil {
		t.Fatalf("set the lab viewport: %v", err)
	}
}

func homedashLabShot(t *testing.T, page *rod.Page, top, height float64) image.Image {
	t.Helper()
	data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatPng,
		Clip:                  &proto.PageViewport{X: 0, Y: top, Width: homedashLabWidth, Height: height, Scale: 1},
		CaptureBeyondViewport: true,
	})
	if err != nil {
		t.Fatalf("screenshot at %.0f+%.0f: %v", top, height, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode the screenshot: %v", err)
	}
	return img
}

// homedashLabEval runs a function in the page and returns its string result.
func homedashLabEval(page *rod.Page, js string, args ...interface{}) (string, error) {
	obj, err := page.Timeout(60*time.Second).Eval(js, args...)
	if err != nil {
		return "", fmt.Errorf("eval: %w", err)
	}
	return obj.Value.Str(), nil
}

// homedashLabRunPanel renders one panel and returns its screenshots (several for a motion panel) and report.
func homedashLabRunPanel(t *testing.T, page *rod.Page, base string, panel homedashLabPanel) ([]image.Image, homedashLabPanelReport) {
	t.Helper()
	target := base + "/testdata/homedash-lab.html?" + panel.Query.Encode()
	report := homedashLabPanelReport{Label: panel.Label, URL: target, Errors: []string{}, Oversized: []string{}}
	value := "no-preference"
	if panel.Motion != "full" {
		value = "reduce"
	}
	// The result must not depend on the host: pin the colour scheme to the panel's scheme (the lab default is dark).
	// Both features go into one call, because a second call would reset the first.
	scheme := "dark"
	if panel.Query.Get("scheme") == "light" {
		scheme = "light"
	}
	media := proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "prefers-reduced-motion", Value: value},
		{Name: "prefers-color-scheme", Value: scheme},
	}}
	if err := media.Call(page); err != nil {
		t.Fatalf("emulate prefers-reduced-motion and prefers-color-scheme: %v", err)
	}
	fail := func(format string, args ...interface{}) {
		report.Errors = append(report.Errors, fmt.Sprintf(format, args...))
	}

	open := func() bool {
		homedashLabViewport(t, page, 900, panel.DPR)
		if err := page.Timeout(60 * time.Second).Navigate(target); err != nil {
			fail("navigate: %v", err)
			return false
		}
		deadline := time.Now().Add(30 * time.Second)
		for {
			done, err := homedashLabEval(page, `()=>String(!!window.hdLab&&(hdLab.ready===true||!!hdLab.error))`)
			if err != nil {
				fail("wait for the lab: %v", err)
				return false
			}
			if done == "true" {
				break
			}
			if time.Now().After(deadline) {
				fail("the lab was not ready within 30s")
				return false
			}
			time.Sleep(50 * time.Millisecond)
		}
		msg, err := homedashLabEval(page, `()=>String(hdLab.error||'')`)
		if err != nil {
			fail("read the lab error: %v", err)
			return false
		}
		if msg != "" {
			fail("lab: %s", msg)
			return false
		}
		return true
	}

	type layout struct {
		Top    float64          `json:"top"`
		Height float64          `json:"height"`
		Rows   []homedashLabRow `json:"rows"`
	}
	readLayout := func() (layout, bool) {
		var l layout
		raw, err := homedashLabEval(page, `()=>JSON.stringify(hdLab.layout())`)
		if err == nil {
			err = json.Unmarshal([]byte(raw), &l)
		}
		if err != nil {
			fail("layout: %v", err)
			return l, false
		}
		return l, true
	}
	var images []image.Image
	capture := func() {
		l, ok := readLayout()
		if !ok {
			return
		}
		if l.Height > homedashLabMaxPanelHeight {
			report.Oversized = append(report.Oversized, fmt.Sprintf("panel %q is %.0f css px tall, capped at %d", panel.Label, l.Height, homedashLabMaxPanelHeight))
			l.Height = homedashLabMaxPanelHeight
		}
		homedashLabViewport(t, page, int(math.Ceil(l.Height)), panel.DPR)
		if _, err := homedashLabEval(page, `async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));return 'ok';}`); err != nil {
			fail("settle: %v", err)
			return
		}
		if l, ok = readLayout(); !ok {
			return
		}
		for _, g := range homedashLabGroups(l.Rows, math.Floor(homedashLabMaxSide/panel.DPR)) {
			img := homedashLabShot(t, page, g[0], g[1]-g[0])
			if b := img.Bounds(); b.Dy() > homedashLabMaxSide || b.Dx() > homedashLabMaxSide {
				report.Oversized = append(report.Oversized, fmt.Sprintf("band %.0f-%.0f is %dx%d device px, limit %d per side", g[0], g[1], b.Dx(), b.Dy(), homedashLabMaxSide))
			}
			images = append(images, img)
		}
	}

	if len(panel.Frames) > 0 {
		for _, ms := range panel.Frames {
			if !open() {
				break
			}
			if _, err := homedashLabEval(page, `async(sc,ms)=>{await hdLab.applyState(sc);await hdLab.seek(ms);return 'ok';}`, panel.Scenario, ms); err != nil {
				fail("scenario %q at %dms: %v", panel.Scenario, ms, err)
				break
			}
			capture()
		}
	} else if open() {
		capture()
		raw, err := homedashLabEval(page, `async()=>JSON.stringify(await hdLab.probes())`)
		var probes homedashLabProbes
		if err == nil {
			err = json.Unmarshal([]byte(raw), &probes)
		}
		if err != nil {
			fail("probes: %v", err)
		} else {
			report.Extra, probes.Extra = probes.Extra, nil
			report.Probes = &probes
		}
	}
	if raw, err := homedashLabEval(page, `()=>JSON.stringify((window.hdLab&&hdLab.errors)||[])`); err == nil {
		var list []string
		if json.Unmarshal([]byte(raw), &list) == nil {
			for _, e := range list {
				fail("page: %s", e)
			}
		}
	}
	return images, report
}

// homedashLabWriteSheet packs the images, replaces the sheet's old PNGs and returns the new file names.
func homedashLabWriteSheet(t *testing.T, dir, id string, images []image.Image) []string {
	t.Helper()
	old := regexp.MustCompile(`^` + regexp.QuoteMeta(id) + `(-[0-9][0-9])?\.png$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && old.MatchString(e.Name()) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				t.Fatalf("remove the old sheet image: %v", err)
			}
		}
	}
	if len(images) == 0 {
		return []string{}
	}
	parts := homedashLabPack(images, homedashLabMaxSide)
	names := homedashLabFileNames(id, len(parts))
	for i, part := range parts {
		var buf bytes.Buffer
		if err := png.Encode(&buf, part); err != nil {
			t.Fatalf("encode %s: %v", names[i], err)
		}
		if err := os.WriteFile(filepath.Join(dir, names[i]), buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write %s: %v", names[i], err)
		}
	}
	return names
}

// TestDesktopHomeDashControlLabBrowser renders the HomeDash control lab sheets (ui/testdata/homedash-lab-sheets.json)
// in headless Chrome, runs the lab probes and writes the sheet PNGs and probes.json to reports/homedash-lab/.
//
// Run (PowerShell, from the repository root; build once, then run the binary from the ui directory):
//
//	go test -c -o D:\tmp\go-build\hdlab-ui.test.exe ./ui
//	$env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hdlab-ui.test.exe '-test.run=^TestDesktopHomeDashControlLabBrowser$' '-test.v' '-test.timeout=30m'
//
// PowerShell splits an unquoted -test.run at the dot, so quote the flags as shown. Add
// $env:HOMEDASH_LAB_SHEETS='prim-glass,themes-glass' to choose sheets.
//
// HOMEDASH_LAB_SHEETS is a comma list of sheet ids (empty means the defaultSheets of the file).
// HOMEDASH_LAB_REPORT_ONLY=1 writes the report and logs probe failures without failing the test.
func TestDesktopHomeDashControlLabBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	file, err := homedashLabLoadFile(filepath.Join("testdata", "homedash-lab-sheets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sheets []homedashLabSheet
	for _, id := range homedashLabSheetIDs(os.Getenv("HOMEDASH_LAB_SHEETS"), file) {
		sheet, err := homedashLabResolve(file, id)
		if err != nil {
			t.Fatal(err)
		}
		sheets = append(sheets, sheet)
	}
	reportOnly := os.Getenv("HOMEDASH_LAB_REPORT_ONLY") == "1"
	base := newHomeDashLabServer(t)
	page := newSmokeBrowserWith(t, nil).MustPage()
	t.Cleanup(func() { _ = page.Close() })
	outDir := filepath.Join("..", "reports", "homedash-lab")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create %s: %v", outDir, err)
	}

	report := homedashLabReport{Generated: time.Now().UTC().Format(time.RFC3339), ReportOnly: reportOnly, Sheets: []homedashLabSheetReport{}, Failures: []string{}}
	// probes.json is written once: after the loop, or by the deferred call when a helper's t.Fatalf ends the
	// test early, so the report of the sheets done so far survives.
	written := false
	writeReport := func() {
		if written {
			return
		}
		written = true
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Errorf("encode probes.json: %v", err)
			return
		}
		if err := os.WriteFile(filepath.Join(outDir, "probes.json"), append(data, '\n'), 0o644); err != nil {
			t.Errorf("write probes.json: %v", err)
		}
	}
	defer writeReport()
	for _, sheet := range sheets {
		report.Sheets = append(report.Sheets, homedashLabSheetReport{ID: sheet.ID, Panels: []homedashLabPanelReport{}})
		sr := &report.Sheets[len(report.Sheets)-1]
		var images []image.Image
		failed := 0
		for _, panel := range sheet.Panels {
			imgs, pr := homedashLabRunPanel(t, page, base, panel)
			images = append(images, imgs...)
			sr.Panels = append(sr.Panels, pr)
			for _, f := range pr.failures() {
				failed++
				report.Failures = append(report.Failures, fmt.Sprintf("%s [%s]: %s", sheet.ID, pr.Label, f))
			}
		}
		sr.Images = homedashLabWriteSheet(t, outDir, sheet.ID, images)
		t.Logf("sheet %s: %d panels, images %s, %d failures", sheet.ID, len(sheet.Panels), strings.Join(sr.Images, ","), failed)
	}
	writeReport()
	if len(report.Failures) > 0 {
		if reportOnly {
			t.Logf("%d HomeDash lab probe failures (report only):\n%s", len(report.Failures), strings.Join(report.Failures, "\n"))
			return
		}
		t.Fatalf("%d HomeDash lab probe failures (HOMEDASH_LAB_REPORT_ONLY=1 reports without failing):\n%s", len(report.Failures), strings.Join(report.Failures, "\n"))
	}
}
