package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// videoStudioAppScripts returns every 'video-studio' script of the module loader in load order (index §5), the shared
// sticker scripts included, so the keyframe fixture follows later insertions without a list of its own.
func videoStudioAppScripts(t *testing.T) []string {
	t.Helper()
	loader, err := os.ReadFile(filepath.Join("js", "desktop", "core", "module-loader.js"))
	if err != nil {
		t.Fatalf("module loader: %v", err)
	}
	entry := regexp.MustCompile(`'video-studio':\s*\{[^}]*?scripts:\s*\[([^\]]*)\]`).FindSubmatch(loader)
	if entry == nil {
		t.Fatal("module loader has no 'video-studio' scripts list")
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`'(/js/desktop/(?:shared/sticker|apps/video-studio)[a-z0-9-]*\.js)'`).FindAllSubmatch(entry[1], -1) {
		scripts = append(scripts, string(match[1]))
	}
	return scripts
}

// videoStudioKeyframesProject is the keyframe fixture: a 30 fps project with a 512x288 picture (c1, 150 frames) at half
// size in the top left corner of the first overlay track. Later parts may append clips here.
func videoStudioKeyframesProject() map[string]any {
	track := func(id, kind, name string, clips ...any) map[string]any {
		return map[string]any{"id": id, "kind": kind, "name": name, "muted": false, "hidden": false, "locked": false, "clips": append([]any{}, clips...)}
	}
	return map[string]any{
		"version": 3, "name": "Keyframe fixture", "width": 1280, "height": 720, "fps": 30,
		"assets": []any{map[string]any{"id": "art1", "name": "Coast.png", "path": "media/art1_coast.png", "kind": "image", "duration_frames": 18000, "width": 512, "height": 288, "has_audio": false}},
		"tracks": []any{
			track("v1", "video", "Video 1"),
			track("a1", "audio", "Audio 1"),
			track("o1", "overlay", "Overlay 1", map[string]any{
				"id": "c1", "asset_id": "art1", "start": 0, "offset": 0, "duration": 150,
				"x": 0, "y": 0, "width": 0.5, "height": 0.5, "rotation": 0, "opacity": 1, "volume": 1,
				"fade_in": 0, "fade_out": 0, "fit": "contain", "text": "", "text_style": nil, "transition": nil,
			}),
		},
	}
}

// newVideoStudioKeyframesServer serves the shipped UI, a gradient PNG for every media request of project p1, and /kf:
// the whole Video Studio app (module-loader order) as window 'kf' with fetch stubbed for that one project. Saves land in
// window.kfSaves; script errors in window.kfErrors.
func newVideoStudioKeyframesServer(t *testing.T, project map[string]any) *httptest.Server {
	t.Helper()
	var art bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 512, 288))
	for y := 0; y < 288; y++ {
		for x := 0; x < 512; x++ {
			picture.Set(x, y, color.RGBA{R: uint8(60 + x/4), G: uint8(90 + y/3), B: 200, A: 255})
		}
	}
	if err := png.Encode(&art, picture); err != nil {
		t.Fatal(err)
	}
	var english map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &english); err != nil {
		t.Fatalf("parse en desktop locale: %v", err)
	}
	locale := map[string]string{}
	for key, value := range english {
		if strings.HasPrefix(key, "videoStudio.") || strings.HasPrefix(key, "stickers.") {
			locale[key] = value
		}
	}
	var page strings.Builder
	page.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-video-studio.css"><link rel="stylesheet" href="/css/desktop-stickers.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.desktop-body{font-family:system-ui,sans-serif;color:var(--vd-text,#edf0f5);background:var(--vd-theme-app-bg)}.vd-window-content{height:100%;width:100%;display:flex}#studio{flex:1;min-width:0;min-height:0}.vs-app{flex:1}</style></head><body class="desktop-body" data-theme="standard"><main class="vd-window-content"><div id="studio"></div></main>`)
	page.WriteString(`<script>window.kfErrors=[];addEventListener('error',e=>kfErrors.push(String(e.message)));addEventListener('unhandledrejection',e=>kfErrors.push(String(e.reason)));</script>`)
	for _, script := range videoStudioAppScripts(t) {
		fmt.Fprintf(&page, `<script src="%s"></script>`, script)
	}
	fmt.Fprintf(&page, `<script>
window.kfLocale=%s;window.kfProject=%s;window.kfSaves=[];
const kfJSON=(data,status=200,headers={})=>new Response(JSON.stringify(data),{status,headers:Object.assign({'Content-Type':'application/json'},headers)});
const kfFetch=window.fetch;
window.fetch=async(url,options={})=>{const path=String(url),method=String(options.method||'GET').toUpperCase();
if(path==='/api/desktop/video-studio/status')return kfJSON({enabled:true,desktop_enabled:true,ffmpeg_ready:true,read_only:false,limits:{max_duration_frames:108000,max_duration_seconds:3600,frame_rates:[{fps:30}],canvas_sizes:[{width:1280,height:720}]},generation:{enabled:false,configured:false,durations_seconds:[],image_modes:[]}});
if(path==='/api/desktop/video-studio/projects'&&method==='GET')return kfJSON({projects:[{id:'p1',project:{name:window.kfProject.name},desktop_path:'Documents/Video Studio/p1/project.json'}]});
if(path==='/api/desktop/video-studio/projects/p1'&&method==='GET')return kfJSON({id:'p1',project:window.kfProject,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v1"'});
if(path==='/api/desktop/video-studio/projects/p1'&&method==='PUT'){const body=JSON.parse(options.body);window.kfSaves.push(body);return kfJSON({project:body,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v'+(window.kfSaves.length+1)+'"'});}
if(path==='/api/desktop/video-studio/jobs?project_id=p1')return kfJSON({jobs:[]});
return kfFetch(url,options);};
const kfEsc=v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.VideoStudioApp.render(document.getElementById('studio'),'kf',{t:key=>window.kfLocale[key]||key,esc:kfEsc,setWindowBeforeClose:()=>{},setWindowMenus:()=>{},clearWindowMenus:()=>{},promptDialog:async()=>false,confirmDialog:async()=>false});
</script></body></html>`, mustJSON(t, locale), mustJSON(t, project))
	html := page.String()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/api/desktop/video-studio/projects/p1/media/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(art.Bytes())
	})
	mux.HandleFunc("/kf", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(html))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// openVideoStudioKeyframes opens /kf at 1280x900 and waits for clip c1 on the timeline without script errors.
func openVideoStudioKeyframes(t *testing.T, browser *rod.Browser, server *httptest.Server) *rod.Page {
	t.Helper()
	page := browser.MustPage().Timeout(3 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/kf")
	page.MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.vs-clip[data-clip-id="c1"]') || window.kfErrors.length > 0`)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("keyframe fixture errors: %s", errs)
	}
	return page
}

// videoStudioKeyframesShotDir is AURAGO_VIDEO_STUDIO_SCREENSHOTS, else a test temp dir (never ui/reports).
func videoStudioKeyframesShotDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("AURAGO_VIDEO_STUDIO_SCREENSHOTS")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Logf("keyframe screenshots: %s", dir)
	return dir
}

// videoStudioKeyframe is one stored key as the app holds it.
type videoStudioKeyframe struct {
	F int     `json:"f"`
	V float64 `json:"v"`
	E string  `json:"e"`
}

// videoStudioClipKeys reads the keys of one value of a clip in the open fixture (nil when the value is not animated).
func videoStudioClipKeys(t *testing.T, page *rod.Page, clipID, path string) []videoStudioKeyframe {
	t.Helper()
	raw := page.MustEval(`(id, path) => { const clip = window.VideoStudioApp.instances.get('kf').project.tracks.flatMap(track => track.clips).find(item => item.id === id); return JSON.stringify(clip && clip.keyframes && clip.keyframes[path] || null); }`, clipID, path).Str()
	var keys []videoStudioKeyframe
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		t.Fatalf("keys of %s.%s: %v (%s)", clipID, path, err, raw)
	}
	return keys
}

// requireVideoStudioKeys fails unless got matches want: frames and easings exactly, values within 1e-9.
func requireVideoStudioKeys(t *testing.T, step string, got []videoStudioKeyframe, want ...videoStudioKeyframe) {
	t.Helper()
	ok := len(got) == len(want)
	for i := 0; ok && i < len(want); i++ {
		ok = got[i].F == want[i].F && got[i].E == want[i].E && math.Abs(got[i].V-want[i].V) <= 1e-9
	}
	if !ok {
		t.Fatalf("%s: keys %+v, want %+v", step, got, want)
	}
}

// TestDesktopVideoStudioKeyframesBrowser drives the 2b keyframe UI in the real app (default launcher; either painter).
// Part 1 (Task 12): the stopwatch sets the first key, a slider change at another frame adds the second, ◀▶ and
// Alt+ArrowUp/ArrowDown jump between them, Alt+K adds and removes keys, the preview differs between two frames, fine
// tune opens by itself for an animated position, and a drag of the picture keys the animated x. Tasks 13 and 14 append
// parts 2 and 3.
func TestDesktopVideoStudioKeyframesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := openVideoStudioKeyframes(t, newSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioKeyframesProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	control := func(path, action string) string {
		return `[data-inspector] [data-kf-path="` + path + `"][data-kf-action="` + action + `"]`
	}
	frame := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.frame`).Int() }
	seek := func(f int) {
		t.Helper()
		page.MustEval(`f => { const s = `+app+`; s.preview.pause(); s.preview.seek(f); }`, f)
	}
	focusApp := func() { t.Helper(); page.MustEval(`() => ` + app + `.app.focus()`) }
	attr := func(selector, name string) string {
		t.Helper()
		return page.MustEval(`(sel, name) => { const el = document.querySelector(sel); return el ? String(el.getAttribute(name)) : 'missing'; }`, selector, name).Str()
	}

	page.MustEval(`() => { const s = ` + app + `; s.timelineOptions.onSelect('c1'); s.preview.pause(); s.preview.seek(0); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('`+control("opacity", "stopwatch")+`')`)
	painter := page.MustEval(`() => ` + app + `.preview.painterKind()`).Str()
	if painter != "gl" && painter != "2d" {
		t.Fatalf("painterKind() = %q", painter)
	}
	t.Logf("stage painter: %s", painter)

	// The stopwatch: one key at the playhead holding the value shown; focus stays on the button.
	page.MustElement(control("opacity", "stopwatch")).MustClick()
	requireVideoStudioKeys(t, "stopwatch", videoStudioClipKeys(t, page, "c1", "opacity"), videoStudioKeyframe{0, 1, "linear"})
	if got := attr(control("opacity", "stopwatch"), "aria-pressed"); got != "true" {
		t.Fatalf("stopwatch aria-pressed = %q", got)
	}
	if !page.MustEval(`sel => document.activeElement === document.querySelector(sel)`, control("opacity", "stopwatch")).Bool() {
		t.Error("the stopwatch lost keyboard focus after the click")
	}

	// Auto-key: a slider change at another frame adds the second key; the controls follow the playhead.
	seek(30)
	waitForJSBool(t, page, `() => document.querySelector('`+control("opacity", "key")+`').getAttribute('aria-pressed') === 'false' && !document.querySelector('`+control("opacity", "prev")+`').disabled`)
	page.MustEval(`() => { const input = document.querySelector('[data-inspector] input[data-field="opacity"]'); input.value = '40'; input.dispatchEvent(new Event('input', { bubbles: true })); input.dispatchEvent(new Event('change', { bubbles: true })); }`)
	requireVideoStudioKeys(t, "slider at frame 30", videoStudioClipKeys(t, page, "c1", "opacity"), videoStudioKeyframe{0, 1, "linear"}, videoStudioKeyframe{30, 0.4, "linear"})
	waitForJSBool(t, page, `() => document.querySelector('`+control("opacity", "key")+`').getAttribute('aria-pressed') === 'true'`)

	// ◀▶ buttons and Alt+ArrowUp / Alt+ArrowDown jump between the keys; Alt+K adds a key between them and removes it.
	page.MustElement(control("opacity", "prev")).MustClick()
	if got := frame(); got != 0 {
		t.Fatalf("◀ went to frame %d, want 0", got)
	}
	page.MustElement(control("opacity", "next")).MustClick()
	if got := frame(); got != 30 {
		t.Fatalf("▶ went to frame %d, want 30", got)
	}
	focusApp()
	page.KeyActions().Press(input.AltLeft).Type(input.ArrowUp).MustDo()
	if got := frame(); got != 0 {
		t.Fatalf("Alt+ArrowUp went to frame %d, want 0", got)
	}
	page.KeyActions().Press(input.AltLeft).Type(input.ArrowDown).MustDo()
	if got := frame(); got != 30 {
		t.Fatalf("Alt+ArrowDown went to frame %d, want 30", got)
	}
	seek(15)
	focusApp()
	page.KeyActions().Press(input.AltLeft).Type(input.KeyK).MustDo()
	requireVideoStudioKeys(t, "Alt+K adds", videoStudioClipKeys(t, page, "c1", "opacity"), videoStudioKeyframe{0, 1, "linear"}, videoStudioKeyframe{15, 0.7, "linear"}, videoStudioKeyframe{30, 0.4, "linear"})
	focusApp()
	page.KeyActions().Press(input.AltLeft).Type(input.KeyK).MustDo()
	requireVideoStudioKeys(t, "Alt+K removes", videoStudioClipKeys(t, page, "c1", "opacity"), videoStudioKeyframe{0, 1, "linear"}, videoStudioKeyframe{30, 0.4, "linear"})

	// The preview follows the keys: the picture at opacity 0.4 is darker than at opacity 1 (over black).
	pixel := func(f int) []int {
		t.Helper()
		values := page.MustEval(`async f => { const s = `+app+`; s.preview.pause(); s.preview.seek(f); await s.preview.settle(10000); const canvas = document.querySelector('[data-preview]'); const shot = window.VideoStudioApp.capturePreview('kf', { x: Math.floor(canvas.width / 4), y: Math.floor(canvas.height / 4), width: 1, height: 1 }); return shot ? Array.from(shot.data) : []; }`, f).Arr()
		out := make([]int, len(values))
		for i, value := range values {
			out[i] = value.Int()
		}
		return out
	}
	bright, dim := pixel(0), pixel(30)
	if len(bright) != 4 || len(dim) != 4 || (bright[0]+bright[1]+bright[2])-(dim[0]+dim[1]+dim[2]) < 60 {
		t.Fatalf("the preview does not follow the opacity keys: frame 0 %v, frame 30 %v (painter %s)", bright, dim, painter)
	}

	// Animate x, check that fine tune opens by itself, then drag the picture at frame 60: the drag keys x there.
	if attr(`[data-action="toggle-fine"]`, "aria-expanded") != "true" {
		page.MustElement(`[data-action="toggle-fine"]`).MustClick()
	}
	seek(30)
	page.MustElement(control("x", "stopwatch")).MustClick()
	requireVideoStudioKeys(t, "x stopwatch", videoStudioClipKeys(t, page, "c1", "x"), videoStudioKeyframe{30, 0, "linear"})
	if !page.MustEval(`() => { const s = ` + app + `; s.fineOpen = undefined; window.VideoStudioParts.renderInspector(s); return !document.querySelector('[data-inspector] .vs-field-grid').hidden; }`).Bool() {
		t.Error("fine tune did not open by itself for an animated position")
	}
	seek(60)
	waitForJSBool(t, page, `() => !document.querySelector('[data-transform]').hidden`)
	center := page.MustEval(`() => { const r = document.querySelector('[data-transform]').getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; }`)
	x0, y0 := center.Get("x").Num(), center.Get("y").Num()
	page.Mouse.MustMoveTo(x0, y0).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x0+60, y0).MustMoveTo(x0+120, y0).MustUp(proto.InputMouseButtonLeft)
	moved := videoStudioClipKeys(t, page, "c1", "x")
	if len(moved) != 2 || moved[0].F != 30 || moved[1].F != 60 || moved[1].V < 0.05 || moved[1].V > 0.5 {
		t.Fatalf("a drag at frame 60 must add an x key: %+v", moved)
	}
	if keys := videoStudioClipKeys(t, page, "c1", "y"); keys != nil {
		t.Errorf("a horizontal drag keyed y: %+v", keys)
	}
	if !page.MustEval(`() => { const s = ` + app + `; return window.VideoStudioTimeline.validTimeline(s.project) && s.history.length >= 6; }`).Bool() {
		t.Error("the keyframe edits left an invalid timeline or fewer than six undo steps")
	}
	page.MustScreenshot(filepath.Join(shots, "keyframes-inspector-1280x900.png"))
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors: %s", errs)
	}
	keyframeStripBrowserPart(t, page)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors after the keyframe strip: %s", errs)
	}
	rampBrowserPart(t, page)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors after the speed ramps: %s", errs)
	}
}

// keyframeStripPagePrelude finds the editor of the keyframe browser test, whatever id part 1 gave it.
const keyframeStripPagePrelude = `const s=[...window.VideoStudioApp.instances.values()][0],P=window.VideoStudioParts,T=window.VideoStudioTimeline;
const until=async(test,message)=>{for(let i=0;i<150;i++){const value=test();if(value)return value;await new Promise(resolve=>setTimeout(resolve,20));}throw new Error(message+': '+JSON.stringify({notice:document.querySelector('[data-notice]')?.textContent,selected:s.selectedClipId}));};
const clipOf=id=>T.clipFor(s.project,id).clip;
`

// keyframeStripPageFixture puts two 90-frame clips of a 600-frame video with sound on their own video track: kfA with
// opacity keys at source frames 0 and 60 and an x key at 30, and kfB without keys. Snapping is off, so drags land exactly.
// It does not depend on what part 1 left behind, and parts 2 and 3 both start from it.
const keyframeStripPageFixture = `s.snap=false;
if(!P.mutate(s,'keyframe strip fixture',project=>{
 if(!project.assets.some(asset=>asset.id==='kf-video'))project.assets.push({id:'kf-video',name:'Strip.mp4',path:'media/kf-video.mp4',kind:'video',duration_frames:600,width:1280,height:720,has_audio:true});
 let track=project.tracks.find(item=>item.id==='kf-track');
 if(!track){track=P.addTrackTo(s,project,'video');track.id='kf-track';track.name='Strip';}
 const make=(id,start,extra)=>Object.assign(T.makeClip({id:'kf-video',kind:'video',duration_frames:600},start,id,project),{duration:90},extra||{});
 track.clips=[make('kfA',0,{keyframes:{opacity:[{f:0,v:0,e:'linear'},{f:60,v:1,e:'linear'}],x:[{f:30,v:0.25,e:'linear'}]}}),make('kfB',120)];
}))throw new Error('the strip fixture was refused: '+document.querySelector('[data-notice]')?.textContent);
`

// Part 2 of TestDesktopVideoStudioKeyframesBrowser (Task 13): the keyframe strip under the selected clip. The checks:
// - the summary row appears and the chevron expands it;
// - a real pointer drag moves a key ten frames;
// - Del deletes the key and never the clip;
// - the context menu sets the easing;
// - Copy and Paste move two keys onto a second clip at the playhead;
// - the chevron collapses the strip again.
// It reads no pixels and assumes no painter.
func keyframeStripBrowserPart(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + keyframeStripPagePrelude + keyframeStripPageFixture + `
		P.setSelection(s,['kfA'],'kfA');P.renderTimeline(s);
		const strip=await until(()=>document.querySelector('[data-kf-strip="kfA"]'),'no strip under the keyframed clip');
		if(strip.closest('[data-track-row]')||strip.querySelector('[data-track-row],.vs-track-lane')||!strip.querySelector('.vs-kf-row[data-kf-row="summary"]'))throw new Error('the strip must hold .vs-kf-row rows outside the track rows: '+strip.outerHTML.slice(0,300));
		const ids=[...strip.querySelectorAll('[data-kf-row="summary"] [data-kf-key]')].map(button=>button.dataset.kfKey);
		if(JSON.stringify(ids)!=='["summary:0","summary:30","summary:60"]')throw new Error('summary diamonds: '+JSON.stringify(ids));
		strip.querySelector('[data-kf-action="expand"]').click();
		await until(()=>document.querySelector('[data-kf-strip="kfA"] [data-kf-row="opacity"] [data-kf-key="opacity:60"]')&&document.querySelector('[data-kf-strip="kfA"] [data-kf-row="x"]'),'the chevron did not open the per-property rows');
		const expand=document.querySelector('[data-kf-strip="kfA"] [data-kf-action="expand"]');
		if(expand.getAttribute('aria-expanded')!=='true'||document.activeElement!==expand)throw new Error('the chevron lost its state or its focus');
		const key=document.querySelector('[data-kf-key="opacity:60"]');
		key.scrollIntoView({block:'center',inline:'center'});
		const rect=key.getBoundingClientRect();
		window.kfStripProbe={x:rect.x+rect.width/2,y:rect.y+rect.height/2,dx:10/window.VideoStudioTime.rate(s.project)*T.BASE_PX_PER_SECOND*(s.zoom||1),history:s.history.length};
	}`)
	probe := page.MustEval(`()=>window.kfStripProbe`)
	x, y, dx := probe.Get("x").Num(), probe.Get("y").Num(), probe.Get("dx").Num()
	// A real drag through the window listeners: ten timeline frames to the right, in two moves.
	page.Mouse.MustMoveTo(x, y).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x+dx/2, y).MustMoveTo(x+dx, y).MustUp(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `()=>{const s=[...window.VideoStudioApp.instances.values()][0],c=window.VideoStudioTimeline.clipFor(s.project,'kfA').clip;return JSON.stringify(c.keyframes.opacity.map(k=>k.f))==='[0,70]'&&s.history.length===window.kfStripProbe.history+1&&!s.timelineDragging;}`)
	// Del on the dragged diamond deletes the key and leaves the clip alone.
	page.MustEval(`()=>document.querySelector('[data-kf-key="opacity:70"]').focus()`)
	page.Keyboard.MustType(input.Delete)
	waitForJSBool(t, page, `()=>{const s=[...window.VideoStudioApp.instances.values()][0],found=window.VideoStudioTimeline.clipFor(s.project,'kfA');return !!found&&JSON.stringify(found.clip.keyframes.opacity.map(k=>k.f))==='[0]'&&s.selectedClipId==='kfA';}`)
	page.MustEval(`async()=>{` + keyframeStripPagePrelude + `
		const open=element=>{const r=element.getBoundingClientRect();element.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.x+r.width/2,clientY:r.y+r.height/2}));return until(()=>document.querySelector('.vs-context-menu'),'no keyframe menu');};
		// Easing: five radio items in EASINGS order, the current one checked; item 4 is Hold.
		let menu=await open(document.querySelector('[data-kf-row="x"] [data-kf-key="x:30"]'));
		const radios=[...menu.querySelectorAll('[role="menuitemradio"]')];
		if(radios.length!==5||radios[0].getAttribute('aria-checked')!=='true'||radios[4].getAttribute('aria-checked')!=='false')throw new Error('easing items: '+menu.innerHTML.slice(0,400));
		menu.querySelector('[data-menu-index="4"]').click();
		await until(()=>clipOf('kfA').keyframes.x[0].e==='hold','the menu did not set Hold');
		// Enter selects one summary diamond and Shift+Enter adds another; Copy keyframes (item 6) puts both on the board.
		const summaryKey=f=>document.querySelector('[data-kf-strip="kfA"] [data-kf-key="summary:'+f+'"]');
		summaryKey(0).focus();summaryKey(0).dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));
		summaryKey(30).focus();summaryKey(30).dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',shiftKey:true,bubbles:true,cancelable:true}));
		if(JSON.stringify(s.kfStrip.selected.slice().sort())!=='["opacity:0","x:30"]')throw new Error('Enter and Shift+Enter did not select both keys: '+JSON.stringify(s.kfStrip.selected));
		menu=await open(summaryKey(30));
		menu.querySelector('[data-menu-index="6"]').click();
		if(!s.kfBoard)throw new Error('Copy keyframes left the board empty');
		// The board opens the strip under kfB, which has no keys; Paste keyframes (item 7) places them at frame 150.
		P.setSelection(s,['kfB'],'kfB');P.renderTimeline(s);s.preview.seek(150);
		await until(()=>s.frame===150,'the playhead did not reach frame 150');
		const other=await until(()=>document.querySelector('[data-kf-strip="kfB"]'),'copied keys did not open the strip under the second clip');
		menu=await open(other.querySelector('[data-kf-action="expand"]'));
		menu.querySelector('[data-menu-index="7"]').click();
		await until(()=>{const k=clipOf('kfB').keyframes||{};return JSON.stringify((k.opacity||[]).map(item=>item.f))==='[30]'&&JSON.stringify((k.x||[]).map(item=>[item.f,item.e]))==='[[60,"hold"]]';},'the keys were not pasted 30 frames into kfB with their spacing and easing');
	}`)
	if dir := os.Getenv("AURAGO_VIDEO_STUDIO_SCREENSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(dir, "keyframe-strip-expanded.png"))
	}
	page.MustEval(`async()=>{` + keyframeStripPagePrelude + `
		document.querySelector('[data-kf-strip="kfB"] [data-kf-action="expand"]').click();
		await until(()=>{const strip=document.querySelector('[data-kf-strip="kfB"]');return strip&&!strip.querySelector('[data-kf-row="x"]')&&strip.querySelector('[data-kf-action="expand"]').getAttribute('aria-expanded')==='false';},'the chevron did not collapse the strip');
		s.snap=true;
	}`)
}

// Part 3 of TestDesktopVideoStudioKeyframesBrowser (Task 14): speed ramps. The checks:
// - Ramp up keeps the source range and ripples kfB;
// - the badge carries the muted speaker and the inspector shows the muted-sound notice;
// - the speed field is empty with its placeholder;
// - Edit in the timeline opens the speed lane with focus;
// - a real pointer drag raises the end point's speed;
// - a typed speed replaces the ramp.
// It starts from the part 2 fixture again and reads no pixels.
func rampBrowserPart(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + keyframeStripPagePrelude + keyframeStripPageFixture + `
		P.setSelection(s,['kfA'],'kfA');P.renderTimeline(s);
		const preset=await until(()=>document.querySelector('[data-inspector] [data-ramp-preset="ramp_up"]'),'no Animation section for the video clip');
		preset.click();
		await until(()=>clipOf('kfA').ramp&&clipOf('kfA').duration===60&&clipOf('kfB').start===90,'Ramp up did not keep the source range and ripple the track');
		const badge=await until(()=>document.querySelector('[data-clip-id="kfA"] .vs-clip-ramp'),'no ramp badge on the clip');
		if(badge.querySelectorAll('svg').length!==2)throw new Error('the badge lacks the muted speaker: '+badge.outerHTML);
		const notice=await until(()=>document.querySelector('[data-inspector] [data-ramp-notice]'),'no muted-sound notice');
		if(!notice.textContent.trim())throw new Error('the muted-sound notice is empty');
		const speed=document.querySelector('[data-inspector] [data-field="speed"]');
		if(speed.value!==''||!speed.placeholder)throw new Error('the speed field must be empty with a placeholder while ramped: '+speed.outerHTML);
		document.querySelector('[data-inspector] [data-action="ramp-edit"]').click();
		const point=await until(()=>document.querySelector('[data-kf-strip="kfA"] [data-kf-row="speed"] [data-ramp-point="1"]'),'Edit in the timeline did not open the speed lane');
		if(!document.activeElement||!document.activeElement.matches('[data-ramp-point]'))throw new Error('the speed lane did not take focus');
		point.scrollIntoView({block:'center',inline:'center'});
		const rect=point.getBoundingClientRect();
		window.kfRampProbe={x:rect.x+rect.width/2,y:rect.y+rect.height/2,speed:clipOf('kfA').ramp[1].speed,history:s.history.length};
	}`)
	probe := page.MustEval(`()=>window.kfRampProbe`)
	x, y := probe.Get("x").Num(), probe.Get("y").Num()
	// Up is faster on the log-scale lane; a purely vertical drag keeps the point's frame.
	page.Mouse.MustMoveTo(x, y).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x, y-4).MustMoveTo(x, y-8).MustUp(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `()=>{const s=[...window.VideoStudioApp.instances.values()][0],c=window.VideoStudioTimeline.clipFor(s.project,'kfA').clip;return !!c.ramp&&c.ramp[1].speed>window.kfRampProbe.speed&&c.ramp[1].f===59&&s.history.length===window.kfRampProbe.history+1&&!s.timelineDragging;}`)
	if dir := os.Getenv("AURAGO_VIDEO_STUDIO_SCREENSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(dir, "speed-lane.png"))
		// The Animation section and the clip with its badge, for a visual review.
		page.MustEval(`()=>{document.querySelector('[data-inspector] .vs-ramp-section').scrollIntoView({block:'center'});document.querySelector('[data-clip-id="kfA"]').scrollIntoView({block:'center',inline:'nearest'});}`)
		page.MustScreenshot(filepath.Join(dir, "ramp-animation-section.png"))
		page.MustElement(`[data-clip-id="kfA"]`).MustScreenshot(filepath.Join(dir, "ramp-clip-badge.png"))
	}
	page.MustEval(`async()=>{` + keyframeStripPagePrelude + `
		const speed=document.querySelector('[data-inspector] [data-field="speed"]');
		speed.value='1.5';speed.dispatchEvent(new Event('input',{bubbles:true}));speed.dispatchEvent(new Event('change',{bubbles:true}));
		await until(()=>!clipOf('kfA').ramp&&clipOf('kfA').speed===1.5,'a typed speed did not replace the ramp');
		await until(()=>!document.querySelector('[data-clip-id="kfA"] .vs-clip-ramp')&&!document.querySelector('[data-inspector] [data-ramp-notice]'),'the badge or the notice outlived the ramp');
		s.snap=true;
	}`)
}
