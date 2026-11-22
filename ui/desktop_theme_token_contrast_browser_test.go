package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// desktopThemeAccentPresetPattern finds the accent presets of the Settings picker: ['teal', 'desktop.settings_accent_teal'].
var desktopThemeAccentPresetPattern = regexp.MustCompile(`\['(\w+)', 'desktop\.settings_accent_\w+'\]`)

// desktopThemeAccentPresets reads the fixed accent preset ids from the Settings catalog (js/desktop/core/settings-catalog-runtime.js),
// in picker order. The leading 'theme' entry means "keep the theme's own accent" and is not a fixed preset, so it is skipped.
func desktopThemeAccentPresets(t *testing.T) []string {
	t.Helper()
	var presets []string
	seen := map[string]bool{}
	for _, match := range desktopThemeAccentPresetPattern.FindAllStringSubmatch(readDesktopAssetText(t, "js/desktop/core/settings-catalog-runtime.js"), -1) {
		if match[1] != "theme" && !seen[match[1]] {
			seen[match[1]] = true
			presets = append(presets, match[1])
		}
	}
	if len(presets) < 2 {
		t.Fatalf("js/desktop/core/settings-catalog-runtime.js lists %d accent presets %v, want at least 2", len(presets), presets)
	}
	return presets
}

// desktopThemeTokenProbeJS installs window.desktopTokenProbe(): it resolves tokens through a hidden probe element
// (color: var(<token>)), parses the computed colour by painting it on a 1x1 canvas and checks the status text colours
// on the three shell surfaces and the text on the accent (the pair behind every app's primary button), all in WCAG
// contrast with a 4.5 floor. A token that resolves to nothing or to alpha 0 is "unresolved".
const desktopThemeTokenProbeJS = `()=>{
	const canvas=document.createElement('canvas');canvas.width=canvas.height=1;
	const ctx=canvas.getContext('2d',{willReadFrequently:true});
	const rgba=value=>{ctx.clearRect(0,0,1,1);ctx.fillStyle='rgba(0,0,0,0)';ctx.fillStyle=value;ctx.fillRect(0,0,1,1);const d=ctx.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
	const over=(top,bottom)=>{const a=top[3]+bottom[3]*(1-top[3]);if(!a)return [0,0,0,0];return [0,1,2].map(i=>(top[i]*top[3]+bottom[i]*bottom[3]*(1-top[3]))/a).concat([a]);};
	const lum=c=>{const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);};return 0.2126*f(c[0])+0.7152*f(c[1])+0.0722*f(c[2]);};
	const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05);};
	const show=value=>(Math.floor(value*100)/100).toFixed(2);
	const STATUS=['--ds-color-success-fg','--ds-color-warning-fg','--ds-color-danger-fg','--ds-color-info-fg'];
	window.desktopTokenProbe=()=>{
		const probe=document.createElement('span');
		probe.style.cssText='position:fixed;left:0;top:0;visibility:hidden;pointer-events:none';
		document.body.appendChild(probe);
		const failures=[];let minStatus=Infinity,primary=0;
		const read=token=>{probe.style.color='var('+token+', rgba(0, 0, 0, 0))';const colour=rgba(getComputedStyle(probe).color);if(colour[3]>0)return colour;failures.push(token+' unresolved');return null;};
		try{
			const white=[255,255,255,1];
			const appBg=read('--vd-theme-app-bg'),panelBg=read('--vd-theme-panel-bg'),strongBg=read('--vd-theme-panel-bg-strong');
			const app=appBg&&over(appBg,white);
			const panel=panelBg&&over(panelBg,app||white);
			const strong=strongBg&&over(strongBg,app||white);
			for(const token of STATUS){
				const fg=read(token);
				if(!fg)continue;
				for(const [name,bg] of [['app-bg',app],['panel-bg',panel],['panel-bg-strong',strong]]){
					if(!bg)continue;
					const value=ratio(over(fg,bg),bg);
					minStatus=Math.min(minStatus,value);
					if(value<4.5)failures.push(token+' on '+name+' '+show(value));
				}
			}
			const accentRaw=read('--vd-accent'),inkRaw=read('--vd-accent-contrast');
			if(accentRaw&&inkRaw){
				const accent=over(accentRaw,panel||app||white);
				primary=ratio(over(inkRaw,accent),accent);
				if(primary<4.5)failures.push('--vd-accent-contrast on --vd-accent '+show(primary));
			}
		}finally{probe.remove();}
		return JSON.stringify({failures,minStatus:Number.isFinite(minStatus)?minStatus:0,primary});
	};
	return true;
}`

// desktopThemeTokenVariant is one theme setup of the token contrast check.
type desktopThemeTokenVariant struct {
	name, theme, setting, mode, scheme, accent string
}

// desktopThemeTokenVariants lists every variant of the registry: a theme with a scheme setting runs in light and
// dark, a theme with a user accent runs with every accent preset, every other theme once.
func desktopThemeTokenVariants(themes []desktopThemeDescriptor, presets []string) []desktopThemeTokenVariant {
	var variants []desktopThemeTokenVariant
	for _, theme := range themes {
		modes, accents := []string{theme.Scheme}, []string{"teal"}
		if theme.SchemeSetting != "" {
			modes = []string{"light", "dark"}
		}
		if theme.Accent == "user" {
			accents = presets
		}
		for _, mode := range modes {
			for _, accent := range accents {
				variant := desktopThemeTokenVariant{name: theme.ID, theme: theme.ID, scheme: mode, accent: accent}
				if theme.SchemeSetting != "" {
					variant.name += "-" + mode
					variant.setting, variant.mode = theme.SchemeSetting, mode
				}
				if theme.Accent == "user" {
					variant.name += "/" + accent
				}
				variants = append(variants, variant)
			}
		}
	}
	return variants
}

// TestDesktopThemeTokenContrastBrowser measures the computed tokens in the real desktop shell for every theme
// variant: the four status text colours (--ds-color-success-fg, -warning-fg, -danger-fg, -info-fg) on the app,
// panel and strong panel backgrounds, and --vd-accent-contrast on --vd-accent, the pair behind every app's primary
// button. The floor is 4.5:1. It covers Standard and Fruity, whose tokens live in desktop-base.css and which the
// static skin contract (TestDesktopThemeCSSContract) cannot read, both schemes of themes with a scheme setting and
// every accent preset of the themes with a user accent. Run it with
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopThemeTokenContrastBrowser -count=1 -v
//
// The shell CSS comes from the generated desktop-shell.bundle.css: rebuild it (npm run build:ui) after a token change.
func TestDesktopThemeTokenContrastBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	variants := desktopThemeTokenVariants(parseDesktopThemeRegistry(t), desktopThemeAccentPresets(t))
	page = page.CancelTimeout().Timeout(150 * time.Second) // the audit page allows 30 s, which the variants outlast
	page.MustEval(desktopThemeTokenProbeJS)
	var failures []string
	for _, v := range variants {
		page.MustEval(`(theme,setting,mode,accent)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']=theme;if(setting)s[setting]=mode;s['appearance.accent']=accent;auditTest.applyDesktopSettings();}`, v.theme, v.setting, v.mode, v.accent)
		page.MustWait(`(theme,scheme,accent)=>document.body.dataset.theme===theme&&document.body.dataset.scheme===scheme&&document.body.dataset.accent===accent`, v.theme, v.scheme, v.accent)
		var measured struct {
			Failures  []string `json:"failures"`
			MinStatus float64  `json:"minStatus"`
			Primary   float64  `json:"primary"`
		}
		if err := json.Unmarshal([]byte(page.MustEval(`()=>desktopTokenProbe()`).Str()), &measured); err != nil {
			t.Fatalf("%s: unreadable measurement: %v", v.name, err)
		}
		t.Logf("%-26s status min %5.2f  primary %5.2f", v.name, measured.MinStatus, measured.Primary)
		for _, failure := range measured.Failures {
			failures = append(failures, fmt.Sprintf("%s: %s", v.name, failure))
		}
	}
	if len(failures) > 0 {
		t.Fatalf("%d token contrast failures in %d variants (floor 4.5:1):\n%s", len(failures), len(variants), strings.Join(failures, "\n"))
	}
}
