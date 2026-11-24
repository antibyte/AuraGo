package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// HomeDash control lab: data-defined sheets (ui/testdata/homedash-lab-sheets.json), their resolution into
// lab panels and image packing (pure Go). The browser runner is in desktop_homedash_control_lab_browser_test.go.
const (
	homedashLabMaxSide = 2400
	homedashLabWidth   = 1180
)

type homedashLabThemeRef struct {
	Theme  string `json:"theme"`
	Scheme string `json:"scheme"`
}
type homedashLabCustom struct {
	Name   string `json:"name"`
	Accent string `json:"accent"`
	Hue    int    `json:"hue"`
	Scheme string `json:"scheme"`
}
type homedashLabHero struct {
	Type    string `json:"type"`
	Variant string `json:"variant"`
}
type homedashLabSpec struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Families  []string        `json:"families"`
	Types     []string        `json:"types"`
	Variants  []string        `json:"variants"`
	Palettes  []string        `json:"palettes"`
	Themes    string          `json:"themes"`
	Custom    string          `json:"custom"`
	States    json.RawMessage `json:"states"`
	Sizes     []string        `json:"sizes"`
	DPR       json.RawMessage `json:"dpr"`
	Motion    json.RawMessage `json:"motion"`
	Backdrop  string          `json:"backdrop"`
	Ease      string          `json:"ease"`
	More      *bool           `json:"more"`
	Columns   int             `json:"columns"`
	Frames    []int           `json:"frames"`
	Scenarios []string        `json:"scenarios"`
	Graphics  []string        `json:"graphics"`
	Scripts   []string        `json:"scripts"`
}
type homedashLabFile struct {
	DefaultSheets []string `json:"defaultSheets"`
	Lists         struct {
		Families []string                         `json:"families"`
		States   map[string][]string              `json:"states"`
		Themes   map[string][]homedashLabThemeRef `json:"themes"`
		Custom   map[string][]homedashLabCustom   `json:"custom"`
		Heroes   map[string]homedashLabHero       `json:"heroes"`
	} `json:"lists"`
	Sheets []homedashLabSpec `json:"sheets"`
}
type homedashLabPanel struct {
	Label    string
	Query    url.Values
	DPR      float64
	Motion   string
	Frames   []int
	Scenario string
}
type homedashLabSheet struct {
	ID     string
	Panels []homedashLabPanel
}
type homedashLabRow struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
}

var homedashLabKinds = map[string]bool{"prim": true, "type": true, "themes": true, "risk": true, "hero": true, "motion": true, "side-by-side": true}
var homedashLabPlaceholder = regexp.MustCompile(`\{(family|hero|type)\}`)

func homedashLabLoadFile(path string) (homedashLabFile, error) {
	var f homedashLabFile
	data, err := os.ReadFile(path)
	if err != nil {
		return f, fmt.Errorf("read lab sheets: %w", err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("%s: %w", path, err)
	}
	return f, homedashLabValidate(f)
}

func homedashLabValidate(f homedashLabFile) error {
	seen := map[string]bool{}
	for _, s := range f.Sheets {
		if seen[s.ID] {
			return fmt.Errorf("duplicate sheet id %q", s.ID)
		}
		seen[s.ID] = true
		if !homedashLabKinds[s.Kind] {
			return fmt.Errorf("sheet %q: unknown kind %q", s.ID, s.Kind)
		}
		if err := homedashLabValidateRefs(f, s); err != nil {
			return err
		}
	}
	return nil
}

// homedashLabValidateRefs checks the named references of one sheet, so a typo is found when the file loads.
func homedashLabValidateRefs(f homedashLabFile, s homedashLabSpec) error {
	for _, p := range s.Palettes {
		if !homedashLabContains([]string{"theme", "dark", "light", "custom"}, p) {
			return fmt.Errorf("sheet %q: unknown palette %q", s.ID, p)
		}
	}
	if _, ok := f.Lists.Themes[s.Themes]; s.Themes != "" && s.Themes != "standard" && !ok {
		return fmt.Errorf("sheet %q: unknown theme list %q", s.ID, s.Themes)
	}
	if _, ok := f.Lists.Custom[s.Custom]; s.Custom != "" && !ok {
		return fmt.Errorf("sheet %q: unknown custom list %q", s.ID, s.Custom)
	}
	if _, err := homedashLabStates(f, s.States); err != nil {
		return fmt.Errorf("sheet %q: %w", s.ID, err)
	}
	if _, err := homedashLabFloats(s.DPR); err != nil {
		return fmt.Errorf("sheet %q: %w", s.ID, err)
	}
	motions, err := homedashLabStrings(s.Motion, nil)
	if err != nil {
		return fmt.Errorf("sheet %q: motion: %w", s.ID, err)
	}
	for _, m := range motions {
		if !homedashLabContains([]string{"full", "reduced", "none"}, m) {
			return fmt.Errorf("sheet %q: unknown motion %q", s.ID, m)
		}
	}
	if len(f.Lists.Families) > 0 {
		for _, fam := range s.Families {
			if !homedashLabContains(f.Lists.Families, fam) {
				return fmt.Errorf("sheet %q: family %q is not in the families list", s.ID, fam)
			}
		}
	}
	if strings.Contains(s.ID, "{hero}") && len(f.Lists.Heroes) == 0 {
		return fmt.Errorf("sheet %q: uses {hero} but the file defines no heroes", s.ID)
	}
	return nil
}

func homedashLabSheetIDs(env string, f homedashLabFile) []string {
	ids := f.DefaultSheets
	if strings.TrimSpace(env) != "" {
		ids = strings.Split(env, ",")
	}
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func homedashLabPattern(id string) *regexp.Regexp {
	parts, names := homedashLabPlaceholder.Split(id, -1), homedashLabPlaceholder.FindAllStringSubmatch(id, -1)
	var b strings.Builder
	b.WriteString("^")
	for i, p := range parts {
		b.WriteString(regexp.QuoteMeta(p))
		if i < len(names) {
			switch names[i][1] {
			case "family":
				b.WriteString(`(?P<family>[a-z]+)`)
			case "hero":
				b.WriteString(`(?P<hero>[A-Za-z0-9]+)`)
			default:
				b.WriteString(`(?P<type>[a-z][a-zA-Z0-9_]*)`)
			}
		}
	}
	return regexp.MustCompile(b.String() + "$")
}

func homedashLabContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func homedashLabResolve(f homedashLabFile, id string) (homedashLabSheet, error) {
	// The id becomes a file name under the report directory: it must not leave it.
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return homedashLabSheet{}, fmt.Errorf("HomeDash lab sheet id %q must not contain a path separator or \"..\"", id)
	}
	var order []homedashLabSpec
	for _, s := range f.Sheets {
		if !homedashLabPlaceholder.MatchString(s.ID) {
			order = append(order, s)
		}
	}
	for _, s := range f.Sheets {
		if homedashLabPlaceholder.MatchString(s.ID) {
			order = append(order, s)
		}
	}
	var reasons []string
	for _, s := range order {
		re := homedashLabPattern(s.ID)
		m := re.FindStringSubmatch(id)
		if m == nil {
			continue
		}
		vars := map[string]string{}
		for i, n := range re.SubexpNames() {
			if n != "" {
				vars[n] = m[i]
			}
		}
		allowed := s.Families
		if len(allowed) == 0 {
			allowed = f.Lists.Families
		}
		if fam, ok := vars["family"]; ok && !homedashLabContains(allowed, fam) {
			reasons = append(reasons, fmt.Sprintf("%s: family %q not in %v", s.ID, fam, allowed))
			continue
		}
		if h, ok := vars["hero"]; ok {
			if _, ok := f.Lists.Heroes[h]; !ok {
				reasons = append(reasons, fmt.Sprintf("%s: unknown hero %q", s.ID, h))
				continue
			}
		}
		panels, err := homedashLabExpand(f, s, vars)
		if err != nil {
			return homedashLabSheet{}, fmt.Errorf("sheet %s (%s): %w", id, s.ID, err)
		}
		return homedashLabSheet{ID: id, Panels: panels}, nil
	}
	var known []string
	for _, s := range f.Sheets {
		known = append(known, s.ID)
	}
	reason := ""
	if len(reasons) > 0 {
		reason = " (" + strings.Join(reasons, "; ") + ")"
	}
	return homedashLabSheet{}, fmt.Errorf("unknown HomeDash lab sheet id %q%s; known patterns: %s", id, reason, strings.Join(known, ", "))
}

func homedashLabStrings(raw json.RawMessage, def []string) ([]string, error) {
	if len(raw) == 0 {
		return def, nil
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, fmt.Errorf("want a string or a list: %s", raw)
	}
	return many, nil
}

func homedashLabFloats(raw json.RawMessage) ([]float64, error) {
	if len(raw) == 0 {
		return []float64{1}, nil
	}
	var one float64
	if json.Unmarshal(raw, &one) == nil {
		return []float64{one}, nil
	}
	var many []float64
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, fmt.Errorf("dpr: want a number or a list: %s", raw)
	}
	return many, nil
}

func homedashLabStates(f homedashLabFile, raw json.RawMessage) ([]string, error) {
	all := f.Lists.States["all"]
	if len(raw) == 0 {
		return all, nil
	}
	var name string
	if json.Unmarshal(raw, &name) == nil {
		var out []string
		for _, part := range strings.Split(name, "+") {
			set, ok := f.Lists.States[part]
			if !ok {
				return nil, fmt.Errorf("unknown state set %q", part)
			}
			out = append(out, set...)
		}
		return out, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("states: %s", raw)
	}
	for _, s := range list {
		if len(all) > 0 && !homedashLabContains(all, s) {
			return nil, fmt.Errorf("unknown state %q", s)
		}
	}
	return list, nil
}

type homedashLabLook struct{ Theme, Scheme, Accent, Hue string }

func homedashLabLooks(f homedashLabFile, s homedashLabSpec, palette string) ([]homedashLabLook, error) {
	switch palette {
	case "theme":
		name := s.Themes
		if name == "" {
			name = "standard"
		}
		list, ok := f.Lists.Themes[name]
		if !ok && name == "standard" {
			list, ok = []homedashLabThemeRef{{Theme: "standard", Scheme: "dark"}}, true
		}
		if !ok {
			return nil, fmt.Errorf("unknown theme list %q", name)
		}
		var out []homedashLabLook
		for _, r := range list {
			out = append(out, homedashLabLook{Theme: r.Theme, Scheme: r.Scheme})
		}
		return out, nil
	case "custom":
		if s.Custom == "" {
			return []homedashLabLook{{Scheme: "dark", Accent: "8E5BD9", Hue: "285"}}, nil
		}
		list, ok := f.Lists.Custom[s.Custom]
		if !ok {
			return nil, fmt.Errorf("unknown custom list %q", s.Custom)
		}
		var out []homedashLabLook
		for _, c := range list {
			out = append(out, homedashLabLook{Scheme: c.Scheme, Accent: strings.TrimPrefix(c.Accent, "#"), Hue: strconv.Itoa(c.Hue)})
		}
		return out, nil
	case "light":
		return []homedashLabLook{{Scheme: "light"}}, nil
	case "dark":
		return []homedashLabLook{{Scheme: "dark"}}, nil
	}
	return nil, fmt.Errorf("unknown palette %q", palette)
}

func homedashLabExpand(f homedashLabFile, s homedashLabSpec, vars map[string]string) ([]homedashLabPanel, error) {
	families := s.Families
	if fam := vars["family"]; fam != "" {
		families = []string{fam}
	}
	if len(families) == 0 {
		return nil, fmt.Errorf("no family")
	}
	types, variants := s.Types, s.Variants
	if t := vars["type"]; t != "" {
		types = []string{t}
	}
	if h := vars["hero"]; h != "" {
		hero := f.Lists.Heroes[h]
		types, variants = []string{hero.Type}, []string{hero.Variant}
	}
	if s.Kind == "hero" && len(types) == 0 {
		keys := make([]string, 0, len(f.Lists.Heroes))
		for k := range f.Lists.Heroes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h := f.Lists.Heroes[k]
			if !homedashLabContains(types, h.Type) {
				types = append(types, h.Type)
			}
			if !homedashLabContains(variants, h.Variant) {
				variants = append(variants, h.Variant)
			}
		}
	}
	states, err := homedashLabStates(f, s.States)
	if err != nil {
		return nil, err
	}
	dprs, err := homedashLabFloats(s.DPR)
	if err != nil {
		return nil, err
	}
	motions, err := homedashLabStrings(s.Motion, []string{"full"})
	if err != nil {
		return nil, err
	}
	palettes := s.Palettes
	if len(palettes) == 0 {
		palettes = []string{"dark"}
	}
	scenarios := []string{""}
	if len(s.Frames) > 0 {
		scenarios = s.Scenarios
		if len(scenarios) == 0 {
			scenarios = []string{"on"}
		}
	}
	var panels []homedashLabPanel
	for _, fam := range families {
		for _, pal := range palettes {
			looks, err := homedashLabLooks(f, s, pal)
			if err != nil {
				return nil, err
			}
			for _, look := range looks {
				for _, dpr := range dprs {
					for _, mo := range motions {
						for _, sc := range scenarios {
							v := url.Values{}
							set := func(k, val string) {
								if val != "" {
									v.Set(k, val)
								}
							}
							set("family", fam)
							set("palette", pal)
							set("scheme", look.Scheme)
							set("theme", look.Theme)
							set("accent", look.Accent)
							set("hue", look.Hue)
							set("types", strings.Join(types, ","))
							set("variants", strings.Join(variants, ","))
							set("states", strings.Join(states, ","))
							set("sizes", strings.Join(s.Sizes, ","))
							set("backdrop", s.Backdrop)
							set("ease", s.Ease)
							set("scripts", strings.Join(s.Scripts, ","))
							if s.More != nil && !*s.More {
								v.Set("more", "0")
							}
							if s.Columns > 0 {
								v.Set("columns", strconv.Itoa(s.Columns))
							}
							if len(s.Graphics) > 0 {
								g, _ := json.Marshal(s.Graphics)
								v.Set("graphics", string(g))
							}
							v.Set("motion", mo)
							v.Set("dpr", strconv.FormatFloat(dpr, 'f', -1, 64))
							v.Set("lang", "en")
							label := fmt.Sprintf("family=%s palette=%s", fam, pal)
							if look.Theme != "" {
								label += " theme=" + look.Theme
							}
							if look.Accent != "" {
								label += " accent=" + look.Accent
							}
							label += fmt.Sprintf(" scheme=%s dpr=%g motion=%s", look.Scheme, dpr, mo)
							if sc != "" {
								label += " scenario=" + sc
							}
							panels = append(panels, homedashLabPanel{Label: label, Query: v, DPR: dpr, Motion: mo, Frames: s.Frames, Scenario: sc})
						}
					}
				}
			}
		}
	}
	return panels, nil
}

// homedashLabGroups splits the rows of one panel into vertical bands of at most limit CSS pixels.
func homedashLabGroups(rows []homedashLabRow, limit float64) [][2]float64 {
	var groups [][2]float64
	for _, r := range rows {
		if n := len(groups); n > 0 && r.Bottom-groups[n-1][0] <= limit {
			groups[n-1][1] = r.Bottom
			continue
		}
		groups = append(groups, [2]float64{r.Top, r.Bottom})
	}
	return groups
}

// homedashLabPack lays the images out left to right, top to bottom, in parts of at most maxSide per side.
func homedashLabPack(images []image.Image, maxSide int) []*image.RGBA {
	type placed struct {
		img  image.Image
		x, y int
	}
	var parts []*image.RGBA
	var cur []placed
	x, y, rowH, w, h := 0, 0, 0, 0, 0
	flush := func() {
		if len(cur) == 0 {
			return
		}
		out := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
		for _, p := range cur {
			b := p.img.Bounds()
			draw.Draw(out, image.Rect(p.x, p.y, p.x+b.Dx(), p.y+b.Dy()), p.img, b.Min, draw.Src)
		}
		parts = append(parts, out)
		cur, x, y, rowH, w, h = nil, 0, 0, 0, 0, 0
	}
	for _, img := range images {
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		if x > 0 && x+iw > maxSide {
			y += rowH
			x, rowH = 0, 0
		}
		if y > 0 && y+ih > maxSide {
			flush()
		}
		cur = append(cur, placed{img, x, y})
		x += iw
		if ih > rowH {
			rowH = ih
		}
		if x > w {
			w = x
		}
		if y+rowH > h {
			h = y + rowH
		}
	}
	flush()
	return parts
}

func homedashLabFileNames(id string, n int) []string {
	if n == 1 {
		return []string{id + ".png"}
	}
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s-%02d.png", id, i+1)
	}
	return out
}

func TestHomeDashLabSheetResolve(t *testing.T) {
	file, err := homedashLabLoadFile(filepath.Join("testdata", "homedash-lab-sheets.json"))
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, th := range parseDesktopThemeRegistry(t) {
		known[th.ID] = true
	}
	for name, list := range file.Lists.Themes {
		for _, ref := range list {
			if !known[ref.Theme] {
				t.Errorf("list %s: unknown theme %q", name, ref.Theme)
			}
		}
	}
	if got := strings.Join(homedashLabSheetIDs("", file), ","); got != strings.Join(file.DefaultSheets, ",") {
		t.Fatalf("defaults %q", got)
	}
	for id, want := range map[string]int{"prim-placeholder": 3, "prim-placeholder~reduced": 1, "prim-glass~custom": 3, "type-entity-placeholder": 3, "themes-minimal": 16, "motion-type-toggle-glass": 1} {
		s, err := homedashLabResolve(file, id)
		if err != nil || len(s.Panels) != want {
			t.Errorf("%s: %d panels, %v; want %d", id, len(s.Panels), err, want)
		}
	}
	th, _ := homedashLabResolve(file, "themes-minimal")
	seen := map[string]bool{}
	for _, p := range th.Panels {
		seen[p.Query.Get("theme")+"/"+p.Query.Get("scheme")] = true
	}
	if !seen["fruity/dark"] || !seen["minimal/dark"] || !seen["amiga/light"] {
		t.Errorf("theme variants %v", seen)
	}
	c, _ := homedashLabResolve(file, "prim-glass~custom")
	if q := c.Panels[0].Query; q.Get("accent") != "C2410C" || q.Get("hue") != "40" || q.Get("scheme") != "light" {
		t.Errorf("custom query %v", q)
	}
	m, _ := homedashLabResolve(file, "motion-type-toggle-glass")
	if len(m.Panels[0].Frames) != 6 || m.Panels[0].Scenario != "on" {
		t.Errorf("motion panel %+v", m.Panels[0])
	}
	if _, err := homedashLabResolve(file, "hello"); err == nil || !strings.Contains(err.Error(), "prim-{family}") {
		t.Errorf("unknown id error %v", err)
	}
	if _, err := homedashLabResolve(file, "prim-neon"); err == nil {
		t.Error("prim-neon must fail")
	}
	var synth homedashLabFile
	if err := json.Unmarshal([]byte(`{"lists":{"families":["glass","tactile"],"states":{"all":["off","on","alert"],"core":["off","on"],"extra":["alert"]},"heroes":{"H1":{"type":"switch","variant":"pill"}}},
	 "sheets":[{"id":"hero-glass~x","kind":"hero","families":["glass"],"states":["on"]},{"id":"hero-{family}~x","kind":"hero","states":"core+extra","dpr":[1,2],"motion":["full","reduced"]},
	 {"id":"side-{hero}","kind":"side-by-side","families":["glass","tactile"],"palettes":["dark","light"]}]}`), &synth); err != nil {
		t.Fatal(err)
	}
	if s, err := homedashLabResolve(synth, "hero-glass~x"); err != nil || len(s.Panels) != 1 || s.Panels[0].Query.Get("states") != "on" {
		t.Errorf("literal first: %+v %v", s, err)
	}
	if s, err := homedashLabResolve(synth, "hero-tactile~x"); err != nil || len(s.Panels) != 4 || s.Panels[0].Query.Get("states") != "off,on,alert" || s.Panels[0].Query.Get("types") != "switch" {
		t.Errorf("pattern: %+v %v", s, err)
	}
	if s, err := homedashLabResolve(synth, "side-H1"); err != nil || len(s.Panels) != 4 || s.Panels[0].Query.Get("variants") != "pill" {
		t.Errorf("hero: %+v %v", s, err)
	}
	if _, err := homedashLabResolve(synth, "side-H9"); err == nil {
		t.Error("side-H9 must fail")
	}
	for _, bad := range []string{"../prim-glass", "prim-glass/x", `prim-glass\x`, "a..b"} {
		if _, err := homedashLabResolve(synth, bad); err == nil || !strings.Contains(err.Error(), "path") {
			t.Errorf("%q: want a path error, got %v", bad, err)
		}
	}
	synth.Sheets = append(synth.Sheets, synth.Sheets[0])
	if err := homedashLabValidate(synth); err == nil {
		t.Error("duplicate id must fail")
	}
	for name, sheet := range map[string]string{
		"unknown palette":        `"palettes":["neon"]`,
		"unknown theme list":     `"palettes":["theme"],"themes":"nope"`,
		"unknown custom list":    `"palettes":["custom"],"custom":"nope"`,
		"unknown state set":      `"states":"nope"`,
		"unknown state in array": `"states":["on","nope"]`,
		"bad motion":             `"motion":["wobble"]`,
		"family not in list":     `"families":["neon"]`,
	} {
		var broken homedashLabFile
		doc := `{"lists":{"families":["glass"],"states":{"all":["off","on"]}},"sheets":[{"id":"bad-sheet","kind":"prim",` + sheet + `}]}`
		if err := json.Unmarshal([]byte(doc), &broken); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := homedashLabValidate(broken); err == nil || !strings.Contains(err.Error(), "bad-sheet") {
			t.Errorf("%s: want an error naming the sheet, got %v", name, err)
		}
	}
	var good homedashLabFile
	if err := json.Unmarshal([]byte(`{"lists":{"families":["glass"],"states":{"all":["off","on"]}},"sheets":[{"id":"ok-sheet","kind":"prim","palettes":["dark"],"motion":"full"}]}`), &good); err != nil {
		t.Fatal(err)
	}
	if err := homedashLabValidate(good); err != nil {
		t.Errorf("minimal valid file: %v", err)
	}
}

func TestHomeDashLabPack(t *testing.T) {
	var images []image.Image
	for i := 0; i < 5; i++ {
		images = append(images, image.NewRGBA(image.Rect(0, 0, 1180, 1000)))
	}
	parts := homedashLabPack(images, 2400)
	if len(parts) != 2 || parts[0].Bounds().Dx() != 2360 || parts[0].Bounds().Dy() != 2000 || parts[1].Bounds().Dx() != 1180 || parts[1].Bounds().Dy() != 1000 {
		t.Fatalf("parts %v", parts)
	}
	if got := homedashLabFileNames("x", 1); len(got) != 1 || got[0] != "x.png" {
		t.Fatal(got)
	}
	if got := homedashLabFileNames("x", 3); strings.Join(got, ",") != "x-01.png,x-02.png,x-03.png" {
		t.Fatal(got)
	}
	g := homedashLabGroups([]homedashLabRow{{0, 100}, {100, 1300}, {1300, 2500}}, 2400)
	if len(g) != 2 || g[0] != [2]float64{0, 1300} || g[1] != [2]float64{1300, 2500} {
		t.Fatal(g)
	}
}
