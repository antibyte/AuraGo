package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type hdFitText struct {
	Kind      string  `json:"kind"`
	Text      string  `json:"text"`
	Total     int     `json:"total"`
	Shown     int     `json:"shown"`
	Truncated bool    `json:"truncated"`
	Width     float64 `json:"w"`
}

type hdFitControl struct {
	Kind   string  `json:"k"`
	Width  float64 `json:"w"`
	Height float64 `json:"h"`
}

type hdFitTile struct {
	Type     string                          `json:"type"`
	Variant  string                          `json:"variant"`
	Family   string                          `json:"family"`
	Bp       string                          `json:"bp"`
	State    string                          `json:"state"`
	Mode     string                          `json:"mode"`
	Tags     []string                        `json:"tags"`
	Size     struct{ W, H int }              `json:"size"`
	Px       struct{ Width, Height float64 } `json:"px"`
	Fails    []string                        `json:"fails"`
	Warns    []string                        `json:"warns"`
	Texts    []hdFitText                     `json:"texts"`
	Controls []hdFitControl                  `json:"controls"`
}

type hdFitRender struct {
	Tiles []hdFitTile                           `json:"tiles"`
	Sheet struct{ X, Y, Width, Height float64 } `json:"sheet"`
}

func (t hdFitTile) label() string {
	return fmt.Sprintf("%s/%s %dx%d %s %s %s (%.0fx%.0f px) [%s]", t.Type, t.Variant, t.Size.W, t.Size.H, t.Bp, t.Family, t.State, t.Px.Width, t.Px.Height, strings.Join(t.Tags, "+"))
}

// hdFitKind groups a failure text into title, tiny, overlap, outside, ctl (a control value or label cut under 4 characters), mark (a stale, read-only or alert tile without a visible mark) or empty (a body that paints nothing); anything else is render.
func hdFitKind(f string) string {
	switch {
	case strings.HasPrefix(f, "title"):
		return "title"
	case strings.HasPrefix(f, "tiny"):
		return "tiny"
	case strings.HasPrefix(f, "overlap"):
		return "overlap"
	case strings.HasPrefix(f, "outside"):
		return "outside"
	case strings.HasPrefix(f, "ctl"):
		return "ctl"
	case strings.HasPrefix(f, "mark"):
		return "mark"
	case strings.HasPrefix(f, "empty"):
		return "empty"
	}
	return "render"
}

// hdFitOnly reads HOMEDASH_WF_ONLY (a comma list of type/variant, empty for all) to render a few widgets only.
func hdFitOnly() []string {
	var out []string
	for _, v := range strings.Split(os.Getenv("HOMEDASH_WF_ONLY"), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// TestDesktopHomeDashWidgetFitBrowser is the G13 survey: every registered widget type x variant in every family at the default and the minimum size on
// the cells of a lg (960 px), md (768 px) and sm (390 px) page, with realistic long names, plus the palette thumbs at 80 px cells and (extended) every
// allowed size up to 6x4. It measures that the title shows its full name or at least 8 characters, that no header text is cut to 2 characters or fewer,
// and that no control value or label is cut under 4 characters, that no text box meets a control or another text box, that a stale, read-only or alert tile shows a visible mark (8 px, 3:1 on the card, with a name and a tooltip) and that no body is empty. It writes probes.json and one contact sheet per family and page into HOMEDASH_WF_DIR.
//
//	set AURAGO_RUN_BROWSER_SMOKE=1 and HOMEDASH_WF_DIR (default C:\disposable\homedash-m4\g13\before; a directory inside the repository is refused);
//	HOMEDASH_WF_FAMILIES (comma list, default all four), HOMEDASH_WF_PALETTE (dark or light), HOMEDASH_WF_EXTENDED=0 drops the extended sizes,
//	HOMEDASH_WF_REPORT_ONLY=1 writes the report without failing, HOMEDASH_WF_SHOW_EXT=1 also puts the extended sizes on the sheet image, HOMEDASH_WF_ONLY=type/variant,... renders only those widgets, HOMEDASH_WF_EXTRA_CSS=file.css injects a stylesheet after the bundle (to try rules before they go into the bundle), HOMEDASH_WF_DUMP=type/variant/WxH[/stale] with HOMEDASH_WF_DUMP_BP=lg logs the element boxes of that tile.
func TestDesktopHomeDashWidgetFitBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dirRaw := os.Getenv("HOMEDASH_WF_DIR")
	if dirRaw == "" {
		dirRaw = `C:\disposable\homedash-m4\g13\before`
	}
	outDir := hdSafeOutDir(t, dirRaw)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create %s: %v", outDir, err)
	}
	families := []string{"glass", "tactile", "minimal", "instrument"}
	if raw := strings.TrimSpace(os.Getenv("HOMEDASH_WF_FAMILIES")); raw != "" {
		families = strings.Split(raw, ",")
	}
	palette := os.Getenv("HOMEDASH_WF_PALETTE")
	if palette != "light" {
		palette = "dark"
	}
	extended := os.Getenv("HOMEDASH_WF_EXTENDED") != "0"
	reportOnly := os.Getenv("HOMEDASH_WF_REPORT_ONLY") == "1"
	catalog, err := os.ReadFile(filepath.Join("..", "internal", "homedash", "testdata", "widget_catalog.json"))
	if err != nil {
		t.Fatalf("read the widget catalog golden file: %v", err)
	}

	base := newHomeDashLabServer(t)
	page := newSmokeBrowserWith(t, nil).MustPage()
	t.Cleanup(func() { _ = page.Close() })
	scheme := palette
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}, {Name: "prefers-color-scheme", Value: scheme}}}).Call(page); err != nil {
		t.Fatalf("emulate media: %v", err)
	}
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1500, Height: 900, DeviceScaleFactor: 1}); err != nil {
		t.Fatalf("viewport: %v", err)
	}
	if err := page.Timeout(60 * time.Second).Navigate(base + "/testdata/homedash-widget-fit.html"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		obj, err := page.Eval(`()=>String(!!window.hdFit&&(hdFit.ready===true||!!hdFit.error))`)
		if err != nil {
			t.Fatalf("wait for the survey page: %v", err)
		}
		if obj.Value.Str() == "true" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the survey page was not ready within 30s")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if obj, err := page.Eval(`()=>String(hdFit.error||'')`); err != nil || obj.Value.Str() != "" {
		t.Fatalf("survey page error: %v %v", err, obj)
	}
	if _, err := page.Eval(`(raw)=>{hdFit.setCatalog(JSON.parse(raw));return 'ok';}`, string(catalog)); err != nil {
		t.Fatalf("set the catalog: %v", err)
	}

	if extra := os.Getenv("HOMEDASH_WF_EXTRA_CSS"); extra != "" {
		css, err := os.ReadFile(extra)
		if err != nil {
			t.Fatalf("read HOMEDASH_WF_EXTRA_CSS: %v", err)
		}
		if _, err := page.Eval(`(css)=>{const s=document.createElement('style');s.textContent=css;document.head.appendChild(s);return 'ok';}`, string(css)); err != nil {
			t.Fatalf("inject HOMEDASH_WF_EXTRA_CSS: %v", err)
		}
	}
	var all []hdFitTile
	for _, family := range families {
		for _, bp := range []string{"lg", "md", "sm", "thumb"} {
			spec, _ := json.Marshal(map[string]interface{}{"family": family, "palette": palette, "bp": bp, "extended": extended && bp != "thumb", "showExt": os.Getenv("HOMEDASH_WF_SHOW_EXT") == "1", "only": hdFitOnly()})
			obj, err := page.Timeout(120*time.Second).Eval(`async(spec)=>JSON.stringify(await hdFit.render(JSON.parse(spec)))`, string(spec))
			if err != nil {
				t.Fatalf("render %s %s: %v", family, bp, err)
			}
			var rendered hdFitRender
			if err := json.Unmarshal([]byte(obj.Value.Str()), &rendered); err != nil {
				t.Fatalf("decode %s %s: %v", family, bp, err)
			}
			all = append(all, rendered.Tiles...)
			if specs := os.Getenv("HOMEDASH_WF_DUMP"); specs != "" && bp == os.Getenv("HOMEDASH_WF_DUMP_BP") {
				for _, spec := range strings.Split(specs, ";") {
					if dump, err := page.Eval(`(s)=>hdFit.dump(s)`, spec); err == nil {
						t.Logf("dump %s %s %s: %s", spec, family, bp, dump.Value.Str())
					}
				}
			}
			height := int(rendered.Sheet.Y+rendered.Sheet.Height) + 40
			if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1500, Height: height, DeviceScaleFactor: 1}); err != nil {
				t.Fatalf("viewport: %v", err)
			}
			time.Sleep(150 * time.Millisecond)
			data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng,
				Clip: &proto.PageViewport{X: 0, Y: rendered.Sheet.Y, Width: 1500, Height: rendered.Sheet.Height, Scale: 1}, CaptureBeyondViewport: true})
			if err != nil {
				t.Fatalf("screenshot %s %s: %v", family, bp, err)
			}
			if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("sheet-%s-%s-%s.png", palette, family, bp)), data, 0o644); err != nil {
				t.Fatalf("write the sheet: %v", err)
			}
		}
	}
	if obj, err := page.Eval(`()=>JSON.stringify(hdFit.errors||[])`); err == nil {
		var list []string
		if json.Unmarshal([]byte(obj.Value.Str()), &list) == nil {
			for _, e := range list {
				t.Errorf("page error: %s", e)
			}
		}
	}

	counts := map[string]int{}
	primary := map[string]int{}
	var failures []string
	for _, tile := range all {
		isPrimary := false
		for _, tag := range tile.Tags {
			if tag != "ext" {
				isPrimary = true
			}
		}
		for _, f := range tile.Fails {
			k := hdFitKind(f)
			counts[k]++
			if isPrimary {
				primary[k]++
			}
			failures = append(failures, tile.label()+": "+f)
		}
	}
	sort.Strings(failures)
	summary := map[string]interface{}{"generated": time.Now().UTC().Format(time.RFC3339), "palette": palette, "tiles": len(all), "failuresByKind": counts, "primaryFailuresByKind": primary, "failures": failures, "tileReports": all}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("encode probes.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "probes.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write probes.json: %v", err)
	}
	t.Logf("widget fit survey: %d tiles, %d failures (all sizes %v, default+min %v); report in %s", len(all), len(failures), counts, primary, outDir)
	if len(failures) > 0 {
		shown := failures
		if len(shown) > 60 {
			shown = shown[:60]
		}
		msg := fmt.Sprintf("%d widget fit failures:\n%s", len(failures), strings.Join(shown, "\n"))
		if reportOnly {
			t.Log(msg)
			return
		}
		t.Fatal(msg)
	}
}
