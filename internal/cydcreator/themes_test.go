package cydcreator

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// themeSpecRows is the binding table: id font font_size button_style, then
// bg surface surface_alt text text_muted accent on_accent success warning
// danger border focus border_width radius.
var themeSpecRows = []string{
	"clear montserrat 16 filled #F4F6F9 #FFFFFF #E9EDF3 #1C2430 #6B7686 #2F6FEB #FFFFFF #1F9D55 #C98A00 #D64545 #D9DEE6 #2F6FEB 1 10",
	"night montserrat 16 outline #0F1522 #1A2333 #222D40 #E6EBF2 #8A96A8 #3FC6E0 #0F1522 #3DD68C #F2C14E #F06A6A #2A3446 #3FC6E0 1 10",
	"terminal jetbrains_mono 16 bracket #050805 #050805 #0B140B #33FF66 #1F9940 #33FF66 #050805 #33FF66 #E5FF33 #FF5533 #33FF66 #E5FF33 1 0",
	"sandstorm montserrat 16 filled #EAD9BC #F6EBD6 #E0CBA6 #4A3221 #8C6E52 #C2562B #F6EBD6 #5E8C3A #D08A1E #B23A2A #D4BE98 #C2562B 1 14",
	"dark_sun montserrat 16 outline #0B0907 #1A1410 #241B14 #F3E3C3 #A38B66 #F0A23B #0B0907 #8FBF5A #F0A23B #E0573A #6B4A1E #F0A23B 1 6",
	"synthwave montserrat 16 outline #1B0F2E #2A1747 #341D57 #F5E9FF #B39DD6 #FF4FB8 #1B0F2E #35E0F0 #FFC857 #FF5470 #3A2560 #35E0F0 1 12",
	"high_contrast montserrat 20 filled #000000 #000000 #1A1A1A #FFFFFF #E0E0E0 #FFD400 #000000 #00E676 #FFD400 #FF5252 #FFFFFF #FFD400 2 4",
}

func themeFromRow(t *testing.T, row string) Theme {
	t.Helper()
	f := strings.Fields(row)
	if len(f) != 18 {
		t.Fatalf("row has %d fields: %s", len(f), row)
	}
	num := func(s string) int {
		n, err := strconv.Atoi(s)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	return Theme{ID: f[0], Font: f[1], FontSize: num(f[2]), ButtonStyle: f[3], Tokens: ThemeTokens{
		Bg: f[4], Surface: f[5], SurfaceAlt: f[6], Text: f[7], TextMuted: f[8], Accent: f[9], OnAccent: f[10],
		Success: f[11], Warning: f[12], Danger: f[13], Border: f[14], Focus: f[15],
		BorderWidth: num(f[16]), Radius: num(f[17]),
	}}
}

func TestThemesMatchSpec(t *testing.T) {
	themes, err := Themes()
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != len(themeSpecRows) {
		t.Fatalf("%d themes, want %d", len(themes), len(themeSpecRows))
	}
	for i, row := range themeSpecRows {
		want := themeFromRow(t, row)
		if !reflect.DeepEqual(themes[i], want) {
			t.Errorf("theme %d =\n%+v\nwant\n%+v", i, themes[i], want)
		}
		byID, err := ThemeByID(want.ID)
		if err != nil || byID != want {
			t.Errorf("ThemeByID(%s) = %+v, %v", want.ID, byID, err)
		}
	}
}

func TestThemeByIDUnknown(t *testing.T) {
	if _, err := ThemeByID("klar"); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("err = %v, want CYD_BAD_REQUEST", err)
	}
}

func TestThemesJSONCanonical(t *testing.T) {
	raw := ThemesJSON()
	if len(raw) == 0 || raw[0] != '[' || bytes.ContainsAny(raw, "\n ") {
		t.Fatalf("ThemesJSON is not a compact array: %.80s", raw)
	}
	if !bytes.HasPrefix(raw, []byte(`[{"id":"clear","font":"montserrat","font_size":16,"button_style":"filled","tokens":{"bg":"#F4F6F9",`)) {
		t.Fatalf("unexpected key order: %.120s", raw)
	}
	var back []Theme
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	themes, _ := Themes()
	if !reflect.DeepEqual(back, themes) {
		t.Fatal("ThemesJSON does not round-trip to Themes()")
	}
	raw[0] = 'X'
	if ThemesJSON()[0] != '[' {
		t.Fatal("ThemesJSON must return a copy")
	}
}

func TestParseThemesRejects(t *testing.T) {
	good := `{"id":"x_theme","font":"montserrat","font_size":16,"button_style":"filled","tokens":{` +
		`"bg":"#000000","surface":"#000000","surface_alt":"#000000","text":"#FFFFFF","text_muted":"#FFFFFF",` +
		`"accent":"#FFFFFF","on_accent":"#000000","success":"#00FF00","warning":"#FFFF00","danger":"#FF0000",` +
		`"border":"#FFFFFF","focus":"#FFFFFF","border_width":1,"radius":4}}`
	if _, err := parseThemes([]byte("[" + good + "]")); err != nil {
		t.Fatalf("good theme rejected: %v", err)
	}
	cases := map[string]string{
		"lowercase color": strings.Replace(good, `"bg":"#000000"`, `"bg":"#00000a"`, 1),
		"short color":     strings.Replace(good, `"bg":"#000000"`, `"bg":"#000"`, 1),
		"unknown font":    strings.Replace(good, `"montserrat"`, `"comic_sans"`, 1),
		"bad size":        strings.Replace(good, `"font_size":16`, `"font_size":15`, 1),
		"bad button":      strings.Replace(good, `"filled"`, `"glossy"`, 1),
		"border width":    strings.Replace(good, `"border_width":1`, `"border_width":9`, 1),
		"radius":          strings.Replace(good, `"radius":4`, `"radius":65`, 1),
		"unknown key":     strings.Replace(good, `"radius":4`, `"radius":4,"shadow":2`, 1),
		"bad id":          strings.Replace(good, `"x_theme"`, `"X Theme"`, 1),
	}
	for name, theme := range cases {
		if _, err := parseThemes([]byte("[" + theme + "]")); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := parseThemes([]byte("[" + good + "," + good + "]")); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("duplicate id: %v", err)
	}
	if _, err := parseThemes([]byte("[]")); err == nil {
		t.Error("empty list accepted")
	}
	if _, err := parseThemes([]byte("[" + good + "] []")); err == nil {
		t.Error("trailing data accepted")
	}
}
