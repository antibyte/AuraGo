package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// desktopAccentValues lists the accent choices in picker order. theme is the default and means the active theme's own
// colour; every other value is the same colour on every user-accent theme.
var desktopAccentValues = []string{"theme", "teal", "orange", "blue", "violet", "green"}

var desktopAccentPresets = map[string]string{
	"teal": "#27c7a6", "orange": "#ff9d4d", "blue": "#63b3ff", "violet": "#b794ff", "green": "#48d597",
}

// desktopAccentThemes are the user-accent themes with their own colour and the darker step that goes with it. An empty
// deep colour means the theme derives it from the accent.
var desktopAccentThemes = []struct{ theme, mode, ownAccent, ownDeep string }{
	{"standard", "light", "#e95420", "#c7431a"},
	{"fruity", "light", "#0a84ff", "#0066cc"},
	{"fruity", "dark", "#0a84ff", "#0066cc"},
	{"black_gloss", "light", "#27c7a6", ""},
}

var desktopAccentNumbers = regexp.MustCompile(`\d+`)

// desktopAccentRGB reads the first three decimal numbers of a computed colour such as rgba(233, 84, 32, 0.08).
func desktopAccentRGB(colour string) string {
	parts := desktopAccentNumbers.FindAllString(colour, 3)
	return strings.Join(parts, ",")
}

func desktopAccentHexRGB(t *testing.T, hex string) string {
	t.Helper()
	value, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		t.Fatalf("parse %q: %v", hex, err)
	}
	return fmt.Sprintf("%d,%d,%d", value>>16&255, value>>8&255, value&255)
}

// The picker, the backend allowlist, the shell default, the labels and the palette must name the same accents.
func TestDesktopThemeAccentSettingContract(t *testing.T) {
	t.Parallel()

	var definition *desktop.SettingDefinition
	definitions := desktop.DesktopSettingDefinitions()
	for index := range definitions {
		if definitions[index].Key == "appearance.accent" {
			definition = &definitions[index]
		}
	}
	if definition == nil {
		t.Fatal("appearance.accent is not a desktop setting")
	}
	if definition.Default != "theme" {
		t.Errorf("appearance.accent defaults to %q, want theme: the default keeps each theme's own colour", definition.Default)
	}
	if !slices.Equal(definition.Values, desktopAccentValues) {
		t.Errorf("appearance.accent allows %v, want %v", definition.Values, desktopAccentValues)
	}

	if foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js"); !strings.Contains(foundation, `'appearance.accent': 'theme',`) {
		t.Error("desktop-foundation.js must default appearance.accent to theme, like the backend")
	}

	settings := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	options := regexp.MustCompile(`\['(\w+)', 'desktop\.settings_accent_(\w+)'\]`).FindAllStringSubmatch(settings, -1)
	var picked []string
	for _, option := range options {
		if option[1] != option[2] {
			t.Errorf("accent option %q uses the label of %q", option[1], option[2])
		}
		picked = append(picked, option[1])
	}
	if !slices.Equal(picked, desktopAccentValues) {
		t.Errorf("the Settings picker offers %v, want %v", picked, desktopAccentValues)
	}

	english := desktopAccentLocale(t, "en")
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := desktopAccentLocale(t, locale)
		for _, value := range desktopAccentValues {
			key := "desktop.settings_accent_" + value
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s has no label for %s", locale, key)
			}
		}
		if locale != "en" && words["desktop.settings_accent_theme"] == english["desktop.settings_accent_theme"] {
			t.Errorf("%s fills desktop.settings_accent_theme with the English text", locale)
		}
	}

	base := readDesktopAssetText(t, "css/desktop-base.css")
	if strings.Contains(base, `[data-accent="theme"]`) {
		t.Error(`the theme accent must have no palette rule: without one the active theme's own colour applies`)
	}
	for value, hex := range desktopAccentPresets {
		rule := regexp.MustCompile(`\.desktop-body\[data-accent="` + value + `"\] \{[^}]*--vd-accent: ` + hex + `;`)
		if !rule.MatchString(base) {
			t.Errorf(`desktop-base.css needs .desktop-body[data-accent=%q] with --vd-accent: %s`, value, hex)
		}
	}
}

// A hard-coded theme colour in shell CSS keeps that colour whatever accent the user picked. The Fruity aurora wallpaper
// keeps its blue: that gradient is artwork, not an accent.
func TestDesktopThemeAccentShellCSSHasNoHardCodedThemeColour(t *testing.T) {
	t.Parallel()

	hardCoded := regexp.MustCompile(`rgba\(\s*(233,\s*84,\s*32|10,\s*132,\s*255)\s*,`)
	for _, path := range []string{"css/desktop-base.css", "css/desktop-shell-overrides.css", "css/desktop-start-menu.css", "css/desktop-taskbar.css", "css/desktop-windows.css"} {
		for number, line := range strings.Split(readDesktopAssetText(t, path), "\n") {
			if hardCoded.MatchString(line) && !strings.Contains(line, "rgba(111, 212, 255") {
				t.Errorf("%s:%d hard-codes a theme colour, use rgba(var(--vd-accent-r), var(--vd-accent-g), var(--vd-accent-b), alpha): %s", path, number+1, strings.TrimSpace(line))
			}
		}
	}
}

func desktopAccentLocale(t *testing.T, locale string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
	if err != nil {
		t.Fatalf("read %s locale: %v", locale, err)
	}
	words := map[string]string{}
	if err := json.Unmarshal(data, &words); err != nil {
		t.Fatalf("parse %s locale: %v", locale, err)
	}
	return words
}

// desktopAccentMeasureScript reads the accent tokens and what the chrome that uses them really paints.
const desktopAccentMeasureScript = `()=>{
	const body=document.body,style=getComputedStyle(body),out={accent:body.dataset.accent};
	for(const token of ['--vd-accent','--vd-accent-glow','--ds-color-accent-500','--ds-color-accent-600','--ds-color-border-focus','--ds-color-accent-glow']) out[token]=style.getPropertyValue(token).trim();
	const paint=(className,property,pseudo)=>{
		const element=document.createElement('div');
		element.className=className;
		element.style.transition='none';
		element.style.animation='none';
		body.appendChild(element);
		const value=getComputedStyle(element,pseudo||null)[property];
		element.remove();
		return value;
	};
	const probe=document.createElement('div');
	probe.style.color='rgb(var(--vd-accent-r), var(--vd-accent-g), var(--vd-accent-b))';
	body.appendChild(probe);
	out.channels=getComputedStyle(probe).color;
	probe.remove();
	out.dropTarget=paint('vd-window active vd-window-file-drop-target','borderTopColor');
	out.activeTask=paint('vd-task-button active','backgroundColor','::after');
	out.runningTask=paint('vd-task-button running','backgroundColor','::after');
	out.selectedIcon=paint('vd-icon selected','backgroundColor');
	out.marquee=paint('vd-desktop-selection-marquee','borderTopColor');
	return JSON.stringify(out);
}`

// Picks every accent in the real Settings dropdown on every user-accent theme: the row shows what was picked and the
// tokens and chrome paint it.
func TestDesktopThemeAccentBrowser(t *testing.T) {
	store := newThemeSweepStore()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/settings": store})

	seed := func(theme, mode string) {
		t.Helper()
		store.set(map[string]string{"appearance.theme": theme, "appearance.fruity_mode": mode, "appearance.accent": "theme"})
		page.MustEval(`(theme,mode)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']=theme;s['appearance.fruity_mode']=mode;s['appearance.accent']='theme';auditTest.applyDesktopSettings();}`, theme, mode)
	}

	if got := page.MustEval(`()=>document.body.dataset.accent`).Str(); got != "theme" {
		t.Errorf("a fresh desktop has data-accent=%q, want theme", got)
	}

	for _, testCase := range desktopAccentThemes {
		seed(testCase.theme, testCase.mode)
		page.MustEval(`async()=>{await AuraDesktopModules.loadAppAssets('settings');auditTest.openApp('settings');}`)
		page.MustWait(`()=>!!document.querySelector('.vd-window[data-app-id="settings"] select[data-setting-key="appearance.accent"]')`)
		for _, value := range desktopAccentValues {
			want := testCase.ownAccent
			wantDeep := testCase.ownDeep
			if preset, ok := desktopAccentPresets[value]; ok {
				want, wantDeep = preset, ""
			}
			label := fmt.Sprintf("%s/%s accent %s", testCase.theme, testCase.mode, value)
			page.MustEval(`(value)=>{
				const select=document.querySelector('.vd-window[data-app-id="settings"] select[data-setting-key="appearance.accent"]');
				if(select.value===value)return;
				select.value=value;
				select.dispatchEvent(new Event('change',{bubbles:true}));
			}`, value)
			page.MustWait(`(value)=>document.body.dataset.accent===value`, value)
			page.MustWait(`(value)=>{const select=document.querySelector('.vd-window[data-app-id="settings"] select[data-setting-key="appearance.accent"]');return !!select&&select.value===value;}`, value)
			text := page.MustEval(`()=>{const select=document.querySelector('.vd-window[data-app-id="settings"] select[data-setting-key="appearance.accent"]');return select.options[select.selectedIndex].textContent;}`).Str()
			if wantText := page.MustEval(`(key)=>window.t(key)`, "desktop.settings_accent_"+value).Str(); strings.TrimSpace(text) != wantText {
				t.Errorf("%s: Settings shows %q, want %q", label, text, wantText)
			}

			var got map[string]string
			if err := json.Unmarshal([]byte(page.MustEval(desktopAccentMeasureScript).Str()), &got); err != nil {
				t.Fatalf("%s: read measurement: %v", label, err)
			}
			wantRGB := desktopAccentHexRGB(t, want)
			for _, token := range []string{"--vd-accent", "--ds-color-accent-500", "--ds-color-border-focus"} {
				if !strings.EqualFold(got[token], want) {
					t.Errorf("%s: %s is %q, want %s", label, token, got[token], want)
				}
			}
			painted := []string{"channels", "--vd-accent-glow", "--ds-color-accent-glow"}
			switch testCase.theme {
			case "standard":
				painted = append(painted, "dropTarget", "activeTask", "runningTask")
			case "fruity":
				painted = append(painted, "selectedIcon", "marquee")
			case "black_gloss":
				painted = append(painted, "dropTarget", "activeTask")
			}
			for _, name := range painted {
				if rgb := desktopAccentRGB(got[name]); rgb != wantRGB {
					t.Errorf("%s: %s is %q (%s), want the accent %s", label, name, got[name], rgb, wantRGB)
				}
			}
			switch deep := got["--ds-color-accent-600"]; {
			case wantDeep != "" && !strings.EqualFold(deep, wantDeep):
				t.Errorf("%s: --ds-color-accent-600 is %q, want the theme's %s", label, deep, wantDeep)
			case wantDeep == "" && !strings.Contains(strings.ToLower(deep), strings.ToLower(want)):
				t.Errorf("%s: --ds-color-accent-600 is %q, want it derived from %s", label, deep, want)
			}
		}
		page.MustEval(`()=>auditTest.closeWindow(document.querySelector('.vd-window[data-app-id="settings"]').dataset.windowId)`)
		page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="settings"]')`)
	}

	// A fixed-accent theme keeps its colour whatever the saved setting says.
	for _, value := range desktopAccentValues {
		page.MustEval(`(value)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']='amiga';s['appearance.accent']=value;auditTest.applyDesktopSettings();}`, value)
		if got := page.MustEval(`()=>getComputedStyle(document.body).getPropertyValue('--vd-accent').trim()`).Str(); !strings.EqualFold(got, "#6688bb") {
			t.Errorf("amiga with accent %s has --vd-accent %q, want its fixed #6688bb", value, got)
		}
	}
}
