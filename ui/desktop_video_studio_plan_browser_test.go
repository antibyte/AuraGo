package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Loader order (module-loader.js): the plan scripts sit between the menus and the shell.
var videoStudioPlanFixtureScripts = []string{
	"shared/sticker-catalog.js", "shared/sticker-library.js", "apps/video-studio-icons.js", "apps/video-studio-time.js",
	"apps/video-studio-effects.js", "apps/video-studio-picture.js", "apps/video-studio-anim.js", "apps/video-studio-scope-math.js",
	"apps/video-studio-media.js", "apps/video-studio-gl.js", "apps/video-studio-shaders.js", "apps/video-studio-compositor.js",
	"apps/video-studio-painter2d.js", "apps/video-studio-preview.js", "apps/video-studio-timeline.js", "apps/video-studio-ruler.js",
	"apps/video-studio-edit.js", "apps/video-studio-keyframe-edit.js", "apps/video-studio-navigation.js", "apps/video-studio-source-edit.js",
	"apps/video-studio-folders.js", "apps/video-studio-inspector.js", "apps/video-studio-colorwheel.js", "apps/video-studio-core.js",
	"apps/video-studio-library.js", "apps/video-studio-effects-library.js", "apps/video-studio-artwork.js", "apps/video-studio-jobs.js",
	"apps/video-studio-stage.js", "apps/video-studio-mask.js", "apps/video-studio-editing.js", "apps/video-studio-keyframes.js",
	"apps/video-studio-inspector-effects.js", "apps/video-studio-source.js", "apps/video-studio-tracks.js", "apps/video-studio-markers.js",
	"apps/video-studio-transport.js", "apps/video-studio-rangerender.js", "apps/video-studio-scopes.js", "apps/video-studio-derive.js",
	"apps/video-studio-projects.js", "apps/video-studio-mediabin.js", "apps/video-studio-menus.js",
	"apps/video-studio-plan-board.js", "apps/video-studio-plan.js", "apps/video-studio.js",
}

// The fake API: status with plan availability and a generation profile, one project, and the plan routes of brief §7.
// After a create, the first read answers "planning" (ETag "p1"); the next read answers the plan in review ("p2").
// ?plan=review seeds the reviewed plan, ?readonly makes the studio read-only, ?agent=running seeds a running job.
// window.fixturePutFailures = n makes the next n plan PUTs answer 500 (the server stores nothing then).
const videoStudioPlanFixtureJS = `
window.fixtureErrors=[];window.addEventListener('error',e=>fixtureErrors.push(String(e.message)));window.addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
const params=new URLSearchParams(location.search);
window.fixtureLocales=__LOCALES__;window.fixtureCalls=[];window.fixturePlanGets=[];window.fixturePlanPuts=[];window.fixtureAgentBodies=[];
window.fixtureReviewPlan=__REVIEW__;window.fixtureReviewIssues=__ISSUES__;
const profile=__PROFILE__,availability={enabled:true,supported:true,image_generation:true,tts:true,music:true},project=__PROJECT__;
window.fixturePlan=null;window.fixturePlanTag='';window.fixtureIssues=[];window.fixtureAgent=null;window.fixtureGetsAfterStart=null;
if(params.get('plan')==='review'){window.fixturePlan=structuredClone(window.fixtureReviewPlan);window.fixtureIssues=structuredClone(window.fixtureReviewIssues);window.fixturePlanTag='"p2"';}
window.fixturePutFailures=0;
if(params.get('agent')==='running')window.fixtureAgent={state:'running',mode:'create',phase:'details',tokens:10,started_at:'2026-11-24T09:00:00Z',finished_at:'0001-01-01T00:00:00Z'};
const json=(data,status,headers)=>new Response(JSON.stringify(data),{status:status||200,headers:Object.assign({'Content-Type':'application/json'},headers||{})});
const planBody=()=>({plan:window.fixturePlan,etag:window.fixturePlanTag,issues:window.fixtureIssues,profile,availability,agent:window.fixtureAgent});
const base='/api/desktop/video-studio',planPath=base+'/projects/p1/plan',originalFetch=window.fetch;
window.fetch=async(url,options={})=>{const path=String(url),method=String(options.method||'GET').toUpperCase(),headers=new Headers(options.headers||{});window.fixtureCalls.push([method,path]);
 if(path===base+'/status')return json({enabled:true,desktop_enabled:true,ffmpeg_ready:true,read_only:params.has('readonly'),limits:{max_duration_frames:18000,max_duration_seconds:3600,frame_rates:[{fps:24,ntsc:true},{fps:25},{fps:30},{fps:60}],canvas_sizes:[{width:1280,height:720},{width:720,height:1280},{width:720,height:720}]},generation:{enabled:true,configured:true,provider:'minimax',model:'MiniMax-Hailuo-02',durations_seconds:[6,10],image_modes:['first_frame'],profile},plan:availability});
 if(path===base+'/projects'&&method==='GET')return json({projects:[{id:'p1',project:{name:'Plan fixture'},desktop_path:'Documents/Video Studio/p1/project.json'}]});
 if(path===base+'/projects/p1'&&method==='GET')return json({id:'p1',project,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v1"'});
 if(path===base+'/jobs?project_id=p1')return json({jobs:[]});
 if(path===planPath&&method==='GET'){window.fixturePlanGets.push(headers.get('If-None-Match')||'');
  if(window.fixtureGetsAfterStart!==null&&++window.fixtureGetsAfterStart>=2){window.fixturePlan=structuredClone(window.fixtureReviewPlan);window.fixtureIssues=structuredClone(window.fixtureReviewIssues);window.fixtureAgent={state:'done',mode:'create',phase:'check',tokens:1800,started_at:'2026-11-24T09:00:00Z',finished_at:'2026-11-24T09:01:00Z'};window.fixturePlanTag='"p2"';window.fixtureGetsAfterStart=null;}
  if(!window.fixturePlan)return json({error:'plan_not_found',code:'plan_not_found',message:'This project has no plan yet.'},404);
  // Part B's 304 rule: only for a settled plan (no job status known, stored status not planning).
  if(headers.get('If-None-Match')===window.fixturePlanTag&&!window.fixtureAgent&&window.fixturePlan.status!=='planning')return new Response(null,{status:304,headers:{ETag:window.fixturePlanTag}});
  return json(planBody(),200,{ETag:window.fixturePlanTag});}
 if(path===planPath&&method==='PUT'){const ifMatch=headers.get('If-Match')||'';window.fixturePlanPuts.push({ifMatch,body:JSON.parse(options.body)});
  if(window.fixturePutFailures>0){window.fixturePutFailures--;return json({error:'internal',code:'internal',message:'Could not save the plan.'},500);}
  if(!ifMatch)return json({error:'precondition_required',code:'precondition_required',message:'Save requires the current plan ETag in If-Match.'},428);
  if(window.fixtureAgent&&window.fixtureAgent.state==='running')return json({error:'plan_agent_running',code:'plan_agent_running',message:'The planning agent is working on this plan.'},409);
  if(!window.fixturePlan)return json({error:'plan_not_found',code:'plan_not_found',message:'This project has no plan yet.'},404);
  if(ifMatch!==window.fixturePlanTag)return json({error:'file_conflict',code:'file_conflict',conflict:{code:'file_conflict',path:'Documents/Video Studio/p1/plan.json',version:window.fixturePlanTag}},412);
  const stored=window.fixturePlan;window.fixturePlan=Object.assign(JSON.parse(options.body),{id:stored.id,version:stored.version,profile:stored.profile,status:stored.status==='planning'?'review':stored.status,cost:stored.cost});window.fixturePlanTag='"p'+(window.fixturePlanPuts.length+2)+'"';return json(planBody(),200,{ETag:window.fixturePlanTag});}
 if(path===planPath+'/agent'&&method==='POST'){const body=JSON.parse(options.body);window.fixtureAgentBodies.push(body);
  if(window.fixtureAgent&&window.fixtureAgent.state==='running')return json({error:'plan_agent_running',code:'plan_agent_running',message:'The planning agent is already working on this project.'},409);
  if(body.mode==='create'&&window.fixturePlan&&!body.replace)return json({error:'plan_exists',code:'plan_exists',message:'This project already has a plan. Confirm to replace it.'},409);
  if(body.mode==='create')window.fixturePlan={version:1,id:'plan_fixture',status:'planning',idea:body.idea,target_seconds:body.target_seconds,language:body.language,profile:{provider:'minimax',model:'MiniMax-Hailuo-02',resolution:'768P'},style:'',characters:[],scenes:[],audio:{music:{instrumental:true},clip_audio:'duck'},cost:{estimate:0,spent:0}};
  window.fixtureAgent={state:'running',mode:body.mode,phase:'outline',tokens:0,started_at:'2026-11-24T09:00:00Z',finished_at:'0001-01-01T00:00:00Z'};window.fixtureIssues=[];window.fixturePlanTag='"p1"';window.fixtureGetsAfterStart=0;
  // Part C's Start answers before the job has a phase.
  return json({agent:{state:'running',mode:body.mode,tokens:0,started_at:'2026-11-24T09:00:00Z',finished_at:'0001-01-01T00:00:00Z'}},202);}
 if(path===planPath+'/agent/stop'&&method==='POST'){const running=!!(window.fixtureAgent&&window.fixtureAgent.state==='running');if(running)window.fixtureAgent=Object.assign({},window.fixtureAgent,{state:'stopped',finished_at:'2026-11-24T09:02:00Z'});return json({stopped:running});}
 return originalFetch(url,options);};
document.documentElement.lang='en';
const t=key=>window.fixtureLocales[key]||key;
const ctx={t,esc:v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),iconMarkup:()=>'',setWindowBeforeClose:()=>{},setWindowMenus:()=>{},clearWindowMenus:()=>{},promptDialog:async()=>false,confirmDialog:async()=>false};
window.VideoStudioApp.render(document.getElementById('studio'),'fixture',ctx);
`

func videoStudioPlanFixture(t *testing.T) *httptest.Server {
	t.Helper()
	var poster bytes.Buffer
	art := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			art.Set(x, y, color.RGBA{R: 40, G: uint8(60 + y), B: 120, A: 255})
		}
	}
	if err := png.Encode(&poster, art); err != nil {
		t.Fatal(err)
	}
	locales := map[string]string{}
	for key, value := range readDesktopLocale(t, "en") {
		if strings.HasPrefix(key, "videoStudio.") || strings.HasPrefix(key, "stickers.") {
			locales[key] = value
		}
	}
	profile := map[string]any{"plan_supported": true, "provider": "minimax", "model": "MiniMax-Hailuo-02", "resolution": "768P", "durations_seconds": []int{6, 10}, "aspect_ratios": []string{}, "aspect_from_first_frame": true, "first_frame": true, "last_frame": true, "native_audio": false, "max_prompt_chars": 2000, "cost_per_clip_estimate": 0.2, "parallel": 2}
	scene := func(id, title, prompt string, use float64, transition, keyframe, narration map[string]any) map[string]any {
		return map[string]any{"id": id, "title": title, "video_prompt": prompt, "generate_seconds": 10, "use_seconds": use, "transition_in": transition, "keyframe": keyframe, "narration": narration, "state": "pending"}
	}
	// Total 9.5 + 10 + 10 - 0.8 (blend) = 28.7 s; scene 2 has no video prompt (one blocking issue).
	reviewPlan := map[string]any{
		"version": 1, "id": "plan_fixture", "status": "review", "idea": "A fisherman heads out at dawn", "target_seconds": 30, "language": "en",
		"profile": map[string]any{"provider": "minimax", "model": "MiniMax-Hailuo-02", "resolution": "768P"},
		"style":   "Soft morning light, slow camera moves", "characters": []any{map[string]any{"name": "Fisher", "description": "Old man in a yellow raincoat"}},
		"scenes": []any{
			scene("s1", "Harbour in fog", "Slow push-in over a foggy harbour at dawn", 9.5, map[string]any{"kind": "cut"}, map[string]any{"source": "generated", "image_prompt": "A foggy harbour at dawn, wooden boats"}, map[string]any{"text": "The bay is still asleep."}),
			scene("s2", "The boat leaves", "", 10, map[string]any{"kind": "continue"}, map[string]any{"source": "chain"}, map[string]any{}),
			scene("s3", "Sun over the bay", "Wide shot, the sun rises over the bay", 10, map[string]any{"kind": "blend", "type": "dissolve", "seconds": 0.8}, map[string]any{"source": "none"}, map[string]any{}),
		},
		"audio": map[string]any{"voice": "", "music": map[string]any{"prompt": "Calm acoustic guitar", "instrumental": true}, "clip_audio": "duck"},
		"cost":  map[string]any{"estimate": 0, "spent": 0}, "updated_at": "2026-11-24T09:00:00Z",
	}
	reviewIssues := []any{map[string]any{"code": "prompt_missing", "scene_id": "s2", "field": "scenes[1].video_prompt", "message": "The video prompt is empty.", "blocking": true}}
	project := map[string]any{
		"version": 1, "name": "Plan fixture", "width": 1280, "height": 720, "fps": 30,
		"assets": []any{map[string]any{"id": "art1", "name": "Coast.png", "path": "media/art1_coast.png", "kind": "image", "duration_frames": 18000, "width": 64, "height": 36, "has_audio": false}},
		"tracks": []any{
			map[string]any{"id": "v1", "kind": "video", "name": "Video 1", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "a1", "kind": "audio", "name": "Audio 1", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "o1", "kind": "overlay", "name": "Overlay 1", "muted": false, "hidden": false, "locked": false, "clips": []any{map[string]any{
				"id": "c1", "asset_id": "art1", "start": 0, "offset": 0, "duration": 150, "x": 0, "y": 0, "width": 1, "height": 1, "rotation": 0, "opacity": 1, "volume": 1,
				"fade_in": 0, "fade_out": 0, "fit": "contain", "text": "", "text_style": nil, "transition": nil,
			}}},
		},
	}
	var scripts strings.Builder
	for _, script := range videoStudioPlanFixtureScripts {
		scripts.WriteString(`<script src="/js/desktop/` + script + `"></script>`)
	}
	script := strings.NewReplacer("__LOCALES__", mustJSON(t, locales), "__REVIEW__", mustJSON(t, reviewPlan), "__ISSUES__", mustJSON(t, reviewIssues), "__PROFILE__", mustJSON(t, profile), "__PROJECT__", mustJSON(t, project)).Replace(videoStudioPlanFixtureJS)
	html := `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-video-studio.css"><link rel="stylesheet" href="/css/desktop-stickers.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.desktop-body{font-family:system-ui,sans-serif;color:var(--vd-text,#edf0f5);background:var(--vd-theme-app-bg)}.vd-window-content{height:100%;width:100%;display:flex}#studio{flex:1;min-width:0;min-height:0}.vs-app{flex:1}</style></head><body class="desktop-body" data-theme="standard"><main class="vd-window-content"><div id="studio"></div></main>` + scripts.String() + `<script>` + script + `</script></body></html>`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, html)
	})
	mux.HandleFunc("/api/desktop/video-studio/projects/p1/media/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(poster.Bytes())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// One tour: open a project, switch to Plan, empty state, start a plan (202, planning, poll, review), an issue next
// to its field, a title edit saved with If-Match, phone width without sideways scrolling, back to Edit, read-only.
func TestDesktopVideoStudioPlanBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := videoStudioPlanFixture(t)
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>{const sw=document.querySelector('[data-plan-switch]');return !!sw&&!sw.hidden&&!!document.querySelector('.vs-clip')}`)

	page.MustElement(`[data-plan-mode="plan"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-create-form] [data-plan-idea]')`)
	layout := page.MustEval(`()=>{const shown=s=>{const el=document.querySelector(s);return !!el&&el.getClientRects().length>0};return JSON.stringify({plan:document.querySelector('.vs-app').classList.contains('vs-plan-mode'),board:shown('[data-plan-board]'),stage:shown('.vs-preview-stage'),timeline:shown('.vs-timeline-panel'),library:shown('.vs-library'),pressed:document.querySelector('[data-plan-mode="plan"]').getAttribute('aria-pressed')})}`).Str()
	if want := `{"plan":true,"board":true,"stage":false,"timeline":false,"library":true,"pressed":"true"}`; layout != want {
		t.Fatalf("plan mode layout: got %s, want %s", layout, want)
	}

	page.MustElement(`[data-plan-idea]`).MustInput("A fisherman heads out at dawn")
	page.MustElement(`[data-plan-submit]`).MustClick()
	waitForJSBool(t, page, `()=>window.fixtureAgentBodies.length===1`)
	if got := page.MustEval(`()=>{const b=window.fixtureAgentBodies[0];return JSON.stringify([b.mode,b.idea,b.target_seconds,b.language,'replace' in b])}`).Str(); got != `["create","A fisherman heads out at dawn",30,"en",false]` {
		t.Fatalf("create request body: %s", got)
	}
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-agent] [data-plan-action="stop"]')`)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-card="s3"]')`)
	if got := page.MustEval(`()=>JSON.stringify(window.fixturePlanGets)`).Str(); !strings.Contains(got, `"\"p1\""`) {
		t.Fatalf("the poll did not send If-None-Match: %s", got)
	}

	review := page.MustEval(`()=>{const en=window.fixtureLocales,issue=document.querySelector('[data-plan-issues="s2:video_prompt"] [data-plan-issue-code="prompt_missing"]');return JSON.stringify({issue:!!issue&&issue.textContent.trim()===en['videoStudio.plan.issue.prompt_missing'],blocking:document.querySelector('[data-plan-blocking]').textContent.trim()===en['videoStudio.plan.blocking_one'],total:document.querySelector('[data-plan-total]').textContent.trim(),chained:!!document.querySelector('[data-plan-card="s2"] [data-plan-source="chain"]'),stop:!!document.querySelector('[data-plan-action="stop"]')})}`).Str()
	if want := `{"issue":true,"blocking":true,"total":"28.7 s of 30 s","chained":true,"stop":false}`; review != want {
		t.Fatalf("review board: got %s, want %s", review, want)
	}

	page.MustEval(`()=>{const input=document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]');input.focus();input.value='Harbour at dawn';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	waitForJSBool(t, page, `()=>window.fixturePlanPuts.length===1`)
	if got := page.MustEval(`()=>{const put=window.fixturePlanPuts[0];return JSON.stringify([put.ifMatch,put.body.scenes[0].title,put.body.scenes.length,document.activeElement===document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]')])}`).Str(); got != `["\"p2\"","Harbour at dawn",3,true]` {
		t.Fatalf("title save: %s", got)
	}
	if errs := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).Str(); errs != "[]" {
		t.Fatalf("page errors: %s", errs)
	}

	page.MustSetViewport(390, 844, 1, false)
	waitForJSBool(t, page, `()=>document.querySelector('.vs-app').getBoundingClientRect().width<=390`)
	phone := page.MustEval(`()=>{const q=s=>document.querySelector(s),box=el=>el.getBoundingClientRect(),app=q('.vs-app'),scroll=q('.vs-plan-scroll'),area=box(scroll),cards=[...document.querySelectorAll('[data-plan-card]')].map(box),sw=box(q('[data-plan-switch]')),problems=[];
		if(app.scrollWidth>app.clientWidth+1)problems.push('the app scrolls sideways');
		if(scroll.scrollWidth>scroll.clientWidth+1)problems.push('the board scrolls sideways');
		cards.forEach((r,i)=>{if(r.left<area.left-1||r.right>area.right+1)problems.push('card '+(i+1)+' is wider than the board');if(i&&r.top<cards[i-1].bottom)problems.push('card '+(i+1)+' is not below card '+i);});
		if(sw.width===0||sw.left<0||sw.right>window.innerWidth+1)problems.push('the switch is off screen');
		return problems.length?JSON.stringify({problems,area,scroll:[scroll.scrollWidth,scroll.clientWidth],app:[app.scrollWidth,app.clientWidth]}):'ok'}`).Str()
	if phone != "ok" {
		t.Fatalf("phone plan board: %s", phone)
	}

	page.MustSetViewport(1280, 900, 1, false)
	page.MustElement(`[data-plan-mode="edit"]`).MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('.vs-app').classList.contains('vs-plan-mode')&&document.querySelector('.vs-preview-stage').getClientRects().length>0&&document.querySelector('[data-plan-board]').hidden`)

	page.MustNavigate(server.URL + "/fixture?readonly&plan=review")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>{const sw=document.querySelector('[data-plan-switch]');return !!sw&&!sw.hidden}`)
	page.MustElement(`[data-plan-mode="plan"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-card="s1"]')`)
	page.MustEval(`()=>{const input=document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]');input.value='Changed';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	time.Sleep(1200 * time.Millisecond)
	readonly := page.MustEval(`()=>{const board=document.querySelector('[data-plan-board]'),fields=[...board.querySelectorAll('[data-plan-field]')];return JSON.stringify({actions:board.querySelectorAll('[data-plan-action],[data-plan-submit]').length,fields:fields.length>10,editable:fields.filter(el=>!el.disabled).length,puts:window.fixturePlanPuts.length,errors:window.fixtureErrors})}`).Str()
	if want := `{"actions":0,"fields":true,"editable":0,"puts":0,"errors":[]}`; readonly != want {
		t.Fatalf("read-only board: got %s, want %s", readonly, want)
	}
}

// Stop stays available on a read-only desktop while the agent plans (brief §13.7); every other action stays blocked.
func TestDesktopVideoStudioPlanReadonlyStop(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := videoStudioPlanFixture(t)
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture?readonly&plan=review&agent=running")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>{const sw=document.querySelector('[data-plan-switch]');return !!sw&&!sw.hidden}`)
	page.MustElement(`[data-plan-mode="plan"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-agent] [data-plan-action="stop"]:not([disabled])')`)
	if got := page.MustEval(`()=>document.querySelectorAll('[data-plan-board] [data-plan-action]').length`).Int(); got != 1 {
		t.Fatalf("read-only planning board shows %d actions, want only Stop", got)
	}
	page.MustElement(`[data-plan-agent] [data-plan-action="stop"]`).MustClick()
	waitForJSBool(t, page, `()=>window.fixtureCalls.some(c=>c[0]==='POST'&&c[1].endsWith('/plan/agent/stop'))&&!document.querySelector('[data-plan-action="stop"]')`)
	if errs := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).Str(); errs != "[]" {
		t.Fatalf("page errors: %s", errs)
	}
}

// A failed save must not start the agent: the reload after the start would drop the unsaved edits.
func TestDesktopVideoStudioPlanFailedSaveBlocksStart(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := videoStudioPlanFixture(t)
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture?plan=review")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>{const sw=document.querySelector('[data-plan-switch]');return !!sw&&!sw.hidden}`)
	page.MustElement(`[data-plan-mode="plan"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-card="s1"]')`)
	// The debounced save and the save before the start both fail.
	page.MustEval(`()=>{window.fixturePutFailures=2;const input=document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]');input.value='Unsaved title';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	waitForJSBool(t, page, `()=>window.fixturePlanPuts.length===1`)
	page.MustElement(`[data-plan-action="replan"]`).MustClick()
	page.MustElement(`[data-plan-start-form] button[type="submit"]`).MustClick()
	waitForJSBool(t, page, `()=>window.fixturePlanPuts.length===2`)
	time.Sleep(600 * time.Millisecond)
	got := page.MustEval(`()=>JSON.stringify({agentPosts:window.fixtureAgentBodies.length,title:document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]').value,locked:!!document.querySelector('[data-plan-agent] [data-plan-action="stop"]'),errors:window.fixtureErrors})`).Str()
	if want := `{"agentPosts":0,"title":"Unsaved title","locked":false,"errors":[]}`; got != want {
		t.Fatalf("start after a failed save: got %s, want %s", got, want)
	}
}
