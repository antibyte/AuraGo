package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/ui"

	"github.com/go-rod/rod"
)

// videoStudioBrowserServer serves the production browser editor (/fixture?id=<instance>) next to the real HTTP
// handlers. Authentication is injected by the test host; media elements cannot set bearer headers. The page's confirm
// dialog answers window.fixtureConfirm (unset: cancel).
func videoStudioBrowserServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, _, _ := testDesktopPermissionServer(t)
	t.Cleanup(func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
	})
	s.Logger = slog.Default()
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.WorkspaceDir = t.TempDir()
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.VideoStudio.Enabled = true
	if err := config.NormalizeVideoStudioConfig(&s.Cfg.VideoStudio); err != nil {
		t.Fatal(err)
	}
	admin, _, err := s.TokenManager.Create("video browser fixture", []string{desktopScopeAdmin, desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.integrationCtx = context.Background()
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.close)
	assets := http.FileServer(http.FS(ui.Content))
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, videoStudioAPIBase) {
			r.Header.Set("Authorization", "Bearer "+admin)
			handleVideoStudio(s).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/fixture" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>Video Studio integration</title>
<link rel="stylesheet" href="/css/desktop-app-video-studio.css">
<link rel="stylesheet" href="/css/desktop-stickers.css">
<style>html,body,#studio{margin:0;width:100%;height:100%;overflow:hidden}body{background:#151922}</style>
<div id="studio"></div>
<script>window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));</script>
<script src="/js/desktop/shared/sticker-catalog.js"></script>
<script src="/js/desktop/shared/sticker-library.js"></script>
<script src="/js/desktop/apps/video-studio-icons.js"></script>
<script src="/js/desktop/apps/video-studio-time.js"></script>
<script src="/js/desktop/apps/video-studio-effects.js"></script>
<script src="/js/desktop/apps/video-studio-picture.js"></script>
<script src="/js/desktop/apps/video-studio-media.js"></script>
<script src="/js/desktop/apps/video-studio-gl.js"></script>
<script src="/js/desktop/apps/video-studio-shaders.js"></script>
<script src="/js/desktop/apps/video-studio-compositor.js"></script>
<script src="/js/desktop/apps/video-studio-painter2d.js"></script>
<script src="/js/desktop/apps/video-studio-preview.js"></script>
<script src="/js/desktop/apps/video-studio-timeline.js"></script>
<script src="/js/desktop/apps/video-studio-ruler.js"></script>
<script src="/js/desktop/apps/video-studio-edit.js"></script>
<script src="/js/desktop/apps/video-studio-keyframe-edit.js"></script>
<script src="/js/desktop/apps/video-studio-navigation.js"></script>
<script src="/js/desktop/apps/video-studio-source-edit.js"></script>
<script src="/js/desktop/apps/video-studio-folders.js"></script>
<script src="/js/desktop/apps/video-studio-inspector.js"></script>
<script src="/js/desktop/apps/video-studio-core.js"></script>
<script src="/js/desktop/apps/video-studio-library.js"></script>
<script src="/js/desktop/apps/video-studio-artwork.js"></script>
<script src="/js/desktop/apps/video-studio-jobs.js"></script>
<script src="/js/desktop/apps/video-studio-stage.js"></script>
<script src="/js/desktop/apps/video-studio-editing.js"></script>
<script src="/js/desktop/apps/video-studio-keyframes.js"></script>
<script src="/js/desktop/apps/video-studio-source.js"></script>
<script src="/js/desktop/apps/video-studio-tracks.js"></script>
<script src="/js/desktop/apps/video-studio-markers.js"></script>
<script src="/js/desktop/apps/video-studio-transport.js"></script>
<script src="/js/desktop/apps/video-studio-rangerender.js"></script>
<script src="/js/desktop/apps/video-studio-derive.js"></script>
<script src="/js/desktop/apps/video-studio-projects.js"></script>
<script src="/js/desktop/apps/video-studio-mediabin.js"></script>
<script src="/js/desktop/apps/video-studio-menus.js"></script>
<script src="/js/desktop/apps/video-studio.js"></script>
<script>
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.fixtureId=new URLSearchParams(location.search).get('id')||'e2e';window.fixtureConfirmCalls=0;
VideoStudioApp.render(document.getElementById('studio'),fixtureId,{
 esc,t:key=>key,iconMarkup:()=>'',promptDialog:async()=> 'Browser export',confirmDialog:async()=>{fixtureConfirmCalls++;return !!window.fixtureConfirm;},
 setWindowBeforeClose:(id,guard)=>{window.closeGuard=guard;},setWindowMenus:()=>{},clearWindowMenus:()=>{}
});
</script>`)
			return
		}
		assets.ServeHTTP(w, r)
	}))
	t.Cleanup(origin.Close)
	return origin
}

// videoStudioBrowserWait polls a page condition; script errors on the page fail the test at once.
func videoStudioBrowserWait(t *testing.T, page *rod.Page, condition string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(condition).Bool() {
			return
		}
		if errors := page.MustEval(`() => fixtureErrors.join('\n')`).Str(); errors != "" {
			t.Fatal(errors)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("browser condition timed out: %s; state=%s", condition, page.MustEval(`() => {const s=VideoStudioApp.instances.get(fixtureId);return JSON.stringify({notice:document.querySelector('[data-notice]')?.textContent,status:s?.status,jobs:s?.jobs,dirty:s?.dirty,conflict:s?.conflict,saveError:s?.saveError,assets:s?.project?.assets.map(a=>a.id)});}`).Str())
}

// This joins the production browser editor to the real HTTP handlers and FFmpeg.
func TestVideoStudioBrowserRealExport(t *testing.T) {
	browser := personalRadioBrowser(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	origin := videoStudioBrowserServer(t)
	page := browser.MustPage(origin.URL + "/fixture").Timeout(90 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustWaitLoad()
	wait := func(condition string) {
		t.Helper()
		videoStudioBrowserWait(t, page, condition)
	}
	wait(`() => VideoStudioApp.instances.get('e2e')?.status?.ffmpeg_ready===true`)
	page.MustElement(`[data-action="new-project"]`).MustClick()
	wait(`() => !!document.querySelector('[data-new-project-form]')`)
	page.MustEval(`() => {const form=document.querySelector('[data-new-project-form]');form.querySelector('[data-settings-name]').value='Browser export';form.querySelector('[data-settings-rate]').value='25';form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));}`)
	wait(`() => VideoStudioApp.instances.get('e2e')?.project?.fps===25`)
	wait(`() => {const s=VideoStudioApp.instances.get('e2e');return !!s?.projectId && s.project?.tracks.length>0 && !s.dirty;}`)
	page.MustEval(`async () => {
 const canvas=document.createElement('canvas');canvas.width=64;canvas.height=64;
 const ctx=canvas.getContext('2d');ctx.fillStyle='#3989db';ctx.fillRect(0,0,64,64);
 const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/png'));
 const transfer=new DataTransfer();transfer.items.add(new File([blob],'blue.png',{type:'image/png'}));
 const input=document.querySelector('[data-file-input]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
}`)
	wait(`() => VideoStudioApp.instances.get('e2e')?.project?.assets.length===1`)
	page.MustElement(`[data-place-asset]`).MustClick()
	wait(`() => {const s=VideoStudioApp.instances.get('e2e');return s.project.tracks.some(t=>t.kind==='overlay'&&t.clips.length===1)&&!s.dirty;}`)
	// The stage may draw with either painter; the app hook reads whichever one draws.
	wait(`() => {
 const canvas=document.querySelector('[data-preview]');
 const shot=VideoStudioApp.capturePreview('e2e',{x:Math.floor(canvas.width/2),y:Math.floor(canvas.height/2),width:1,height:1});
 const pixel=shot&&shot.data;
 return !!pixel && pixel.length===4 && pixel[2]>150 && pixel[1]>80 && pixel[0]<100;
}`)
	page.MustEval(`() => {
 const clip=document.querySelector('.vs-clip'), rect=clip.getBoundingClientRect();
 const x=rect.left+rect.width/2,y=rect.top+rect.height/2;
 clip.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,button:0,clientX:x,clientY:y}));
 window.dispatchEvent(new PointerEvent('pointermove',{clientX:x+104,clientY:y}));
 window.dispatchEvent(new PointerEvent('pointerup',{clientX:x+104,clientY:y}));
}`)
	// 104 px at zoom 1 is two seconds: 50 frames at the project's 25 fps.
	wait(`() => VideoStudioApp.instances.get('e2e').project.tracks.some(t=>t.kind==='overlay'&&t.clips[0]?.start===50)`)
	page.MustElement(`[data-action="undo"]`).MustClick()
	wait(`() => {const s=VideoStudioApp.instances.get('e2e');return s.project.tracks.some(t=>t.kind==='overlay'&&t.clips[0]?.start===0)&&!s.dirty;}`)
	page.MustElement(`[data-action="export"]`).MustClick()
	page.MustElement(`[data-export-form] button[type="submit"]`).MustClick()
	wait(`() => VideoStudioApp.instances.get('e2e')?.jobs.some(j=>j.kind==='render'&&['succeeded','failed','cancelled'].includes(j.status))`)
	if result := page.MustEval(`() => {const job=VideoStudioApp.instances.get('e2e').jobs.find(j=>j.kind==='render');return job.status==='succeeded' ? '' : JSON.stringify(job);}`).Str(); result != "" {
		t.Fatal(result)
	}
	if !page.MustEval(`async () => {
 const job=VideoStudioApp.instances.get('e2e').jobs.find(j=>j.kind==='render');
 const response=await fetch(job.artifact.download_url);
 return response.ok && response.headers.get('Content-Type').includes('video/mp4') && (await response.arrayBuffer()).byteLength>100;
}`).Bool() {
		t.Fatal("completed render did not produce a downloadable MP4")
	}
	if !page.MustEval(`async () => await closeGuard()`).Bool() {
		t.Fatal("saved editor refused close")
	}
	page.MustEval(`() => VideoStudioApp.dispose('e2e')`)
}

// Render range exactly in the production editor against the real handlers and FFmpeg: the transport button saves and
// starts a range_render job for 5 s around the playhead, the overlay plays the 540p result, the A/B switch keeps the
// timeline frame both ways, and Escape closes the comparison.
func TestVideoStudioBrowserRangeRender(t *testing.T) {
	browser := personalRadioBrowser(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	origin := videoStudioBrowserServer(t)
	page := browser.MustPage(origin.URL + "/fixture").Timeout(120 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustWaitLoad()
	wait := func(condition string) {
		t.Helper()
		videoStudioBrowserWait(t, page, condition)
	}
	wait(`() => VideoStudioApp.instances.get('e2e')?.status?.ffmpeg_ready===true`)
	page.MustElement(`[data-action="new-project"]`).MustClick()
	wait(`() => !!document.querySelector('[data-new-project-form]')`)
	page.MustEval(`() => {const form=document.querySelector('[data-new-project-form]');form.querySelector('[data-settings-name]').value='Range check';form.querySelector('[data-settings-rate]').value='25';form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));}`)
	wait(`() => {const s=VideoStudioApp.instances.get('e2e');return s?.project?.fps===25 && !!s.projectId && s.project.tracks.length>0 && !s.dirty;}`)
	page.MustEval(`async () => {
 const canvas=document.createElement('canvas');canvas.width=64;canvas.height=64;
 const ctx=canvas.getContext('2d');ctx.fillStyle='#3989db';ctx.fillRect(0,0,64,64);
 const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/png'));
 const transfer=new DataTransfer();transfer.items.add(new File([blob],'blue.png',{type:'image/png'}));
 const input=document.querySelector('[data-file-input]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
}`)
	wait(`() => VideoStudioApp.instances.get('e2e')?.project?.assets.length===1`)
	page.MustElement(`[data-place-asset]`).MustClick()
	wait(`() => {const s=VideoStudioApp.instances.get('e2e');return s.project.tracks.some(t=>t.kind==='overlay'&&t.clips.length===1)&&!s.dirty;}`)
	wait(`() => document.querySelector('[data-action="range-render"]')?.disabled===false`)
	page.MustElement(`[data-action="range-render"]`).MustClick()
	wait(`() => {const host=document.querySelector('[data-range-overlay]');return !!host && !host.hidden && ['ready','failed'].includes(host.dataset.rangeState);}`)
	if state := page.MustEval(`() => {const host=document.querySelector('[data-range-overlay]');const s=VideoStudioApp.instances.get('e2e');return host.dataset.rangeState==='ready' ? '' : JSON.stringify({state:host.dataset.rangeState,status:host.querySelector('[data-range-status]')?.textContent,job:s.rangeCompare&&s.rangeCompare.job});}`).Str(); state != "" {
		t.Fatal(state)
	}
	// The 1920x1080 project renders at 960x540; the 5 s image clip at the playhead is frames 0-125 at 25 fps.
	if got := page.MustEval(`() => {const job=VideoStudioApp.instances.get('e2e').rangeCompare.job;return JSON.stringify({kind:job.kind,width:job.result.width,height:job.result.height,range:{start:job.result.range.start,end:job.result.range.end}});}`).Str(); got !=`{"kind":"range_render","width":960,"height":540,"range":{"start":0,"end":125}}` {
		t.Fatalf("range check result: %s", got)
	}
	wait(`() => {const video=document.querySelector('[data-range-video]');return video.readyState>=1 && Math.abs(video.duration-5)<0.2;}`)
	page.MustEval(`() => new Promise(resolve => {const video=document.querySelector('[data-range-video]');video.addEventListener('seeked',()=>resolve(true),{once:true});video.currentTime=2;})`)
	page.MustElement(`[data-action="range-view"][data-view="live"]`).MustClick()
	wait(`() => {const host=document.querySelector('[data-range-overlay]');return host.dataset.rangeView==='live' && host.querySelector('[data-range-video]').hidden && VideoStudioApp.instances.get('e2e').frame===50;}`)
	page.MustElement(`[data-action="range-view"][data-view="rendered"]`).MustClick()
	wait(`() => {const host=document.querySelector('[data-range-overlay]');const video=host.querySelector('[data-range-video]');return host.dataset.rangeView==='rendered' && !video.hidden && Math.abs(video.currentTime-2)<0.05;}`)
	page.MustEval(`() => document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)
	wait(`() => document.querySelector('[data-range-overlay]').hidden && document.activeElement===document.querySelector('[data-action="range-render"]')`)
	if !page.MustEval(`async () => await closeGuard()`).Bool() {
		t.Fatal("saved editor refused close")
	}
	page.MustEval(`() => VideoStudioApp.dispose('e2e')`)
}

// Two editor windows of one project. Window A removes media; window B, which still lists it, keeps saving: unused
// media leaves B's copy and the save goes through once (no retry loop); a clip of removed media opens the conflict
// choices, and both "replace" and "reload" end clean. Undo is held while A's removal deletes the files.
func TestVideoStudioBrowserMediaRemovedInAnotherWindow(t *testing.T) {
	browser := personalRadioBrowser(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	origin := videoStudioBrowserServer(t)
	open := func(id string) *rod.Page {
		page := browser.MustPage(origin.URL + "/fixture?id=" + id).Timeout(120 * time.Second)
		t.Cleanup(func() { page.MustClose() })
		page.MustSetViewport(1280, 900, 1, false)
		page.MustWaitLoad()
		// Every request of the window is counted: a save loop shows as a growing number of PUTs.
		page.MustEval(`() => {window.fixturePuts=0;const original=window.fetch;window.fetch=async (url,options)=>{if(options&&options.method==='PUT')fixturePuts++;if(options&&options.method==='DELETE'&&window.fixtureHoldDeletes)await new Promise(resolve=>{window.fixtureReleaseDeletes=resolve;});return original(url,options);};}`)
		videoStudioBrowserWait(t, page, `() => VideoStudioApp.instances.get(fixtureId)?.status?.ffmpeg_ready===true`)
		return page
	}
	a := open("a")
	waitA := func(condition string) { t.Helper(); videoStudioBrowserWait(t, a, condition) }
	a.MustElement(`[data-action="new-project"]`).MustClick()
	waitA(`() => !!document.querySelector('[data-new-project-form]')`)
	a.MustEval(`() => {const form=document.querySelector('[data-new-project-form]');form.querySelector('[data-settings-name]').value='Two windows';form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));}`)
	waitA(`() => {const s=VideoStudioApp.instances.get('a');return !!s?.projectId && s.project?.tracks.length>0 && !s.dirty;}`)
	for i, colour := range []string{"#3989db", "#db8939", "#39db89"} {
		a.MustEval(`async (name, colour) => {
 const canvas=document.createElement('canvas');canvas.width=64;canvas.height=64;
 const ctx=canvas.getContext('2d');ctx.fillStyle=colour;ctx.fillRect(0,0,64,64);
 const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/png'));
 const transfer=new DataTransfer();transfer.items.add(new File([blob],name,{type:'image/png'}));
 const input=document.querySelector('[data-file-input]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
}`, fmt.Sprintf("m%d.png", i), colour)
		// Media cannot be removed while a probe of the project still counts as running (mediaBusy).
		waitA(fmt.Sprintf(`() => {const s=VideoStudioApp.instances.get('a');return s.project.assets.length===%d && s.project.assets.every(x=>x.kind) && !s.dirty && !s.saving && !s.uploads.size && s.jobs.every(j=>['succeeded','failed','cancelled','interrupted'].includes(j.status));}`, i+1))
	}
	ids := a.MustEval(`() => VideoStudioApp.instances.get('a').project.assets.map(x=>x.id)`).Arr()
	x, y, z := ids[0].Str(), ids[1].Str(), ids[2].Str()

	b := open("b")
	waitB := func(condition string) { t.Helper(); videoStudioBrowserWait(t, b, condition) }
	projectID := a.MustEval(`() => VideoStudioApp.instances.get('a').projectId`).Str()
	waitB(fmt.Sprintf(`() => {const s=VideoStudioApp.instances.get('b');return s.projectId===%q && s.project.assets.length===3 && !s.dirty;}`, projectID))

	// Window A removes x. While the server deletes the file, undo and edits wait; afterwards x is gone from the
	// project and its history.
	a.MustEval(`(id) => {window.fixtureConfirm=true;window.fixtureHoldDeletes=true;window.fixtureRemoval=VideoStudioParts.removeAssets(VideoStudioApp.instances.get('a'),[id],false);}`, x)
	waitA(`() => VideoStudioApp.instances.get('a').mediaRemoving>0 && typeof window.fixtureReleaseDeletes==='function'`)
	if held := a.MustEval(`(id) => {
 const s=VideoStudioApp.instances.get('a'), P=VideoStudioParts;
 P.undo(s,false);
 const edited=P.mutate(s,'blocked',project=>{project.name='blocked';});
 return JSON.stringify({back:s.project.assets.some(item=>item.id===id),edited,name:s.project.name,undoDisabled:document.querySelector('[data-action="undo"]').disabled});
}`, x).Str(); held != `{"back":false,"edited":false,"name":"Two windows","undoDisabled":true}` {
		t.Fatalf("undo or an edit ran during the removal: %s", held)
	}
	a.MustEval(`() => {window.fixtureHoldDeletes=false;window.fixtureReleaseDeletes();}`)
	waitA(`async () => (await window.fixtureRemoval)===true`)
	if state := a.MustEval(`(id) => {
 const s=VideoStudioApp.instances.get('a');
 const has=project=>project.assets.some(item=>item.id===id);
 return JSON.stringify({removing:s.mediaRemoving,project:has(s.project),history:s.history.some(has),redo:s.redo.some(has)});
}`, x).Str(); state != `{"removing":0,"project":false,"history":false,"redo":false}` {
		t.Fatalf("window A after the removal: %s", state)
	}

	// Window B renames the project. Its copy still lists x, unused: the stale save adopts the server copy without x.
	b.MustEval(`() => {window.fixturePuts=0;VideoStudioParts.mutate(VideoStudioApp.instances.get('b'),'rename',project=>{project.name='Renamed in B';});}`)
	waitB(`() => {const s=VideoStudioApp.instances.get('b');return fixturePuts>0 && !s.dirty && !s.saving;}`)
	time.Sleep(1500 * time.Millisecond) // an autosave loop would PUT again every 1.1 s
	if state := b.MustEval(`(id) => {const s=VideoStudioApp.instances.get('b');return JSON.stringify({puts:fixturePuts,conflict:!!s.conflict,dirty:s.dirty,x:s.project.assets.some(item=>item.id===id),dialog:!!document.querySelector('[data-conflict]')});}`, x).Str(); state != `{"puts":2,"conflict":false,"dirty":false,"x":false,"dialog":false}` {
		t.Fatalf("window B after saving over a media removal: %s", state)
	}

	// Window B places y on the timeline but has not saved yet; window A removes y meanwhile.
	b.MustEval(`(id) => {
 const s=VideoStudioApp.instances.get('b');
 VideoStudioParts.mutate(s,'place',project=>{const track=project.tracks.find(item=>item.kind==='overlay');track.clips.push({id:'clip-y',asset_id:id,start:0,offset:0,duration:60,x:0,y:0,width:1,height:1,rotation:0,opacity:1,volume:1,fade_in:0,fade_out:0,fit:'contain',text:'',transition:null});});
 clearTimeout(s.autosaveTimer);
}`, y)
	a.MustEval(`() => VideoStudioParts.refreshProject(VideoStudioApp.instances.get('a'))`)
	waitA(`() => VideoStudioApp.instances.get('a').project.name==='Renamed in B'`)
	a.MustEval(`(id) => {window.fixtureRemoval=VideoStudioParts.removeAssets(VideoStudioApp.instances.get('a'),[id],false);}`, y)
	waitA(`async () => (await window.fixtureRemoval)===true`)
	b.MustEval(`() => {window.fixturePuts=0;VideoStudioParts.saveProject(VideoStudioApp.instances.get('b'));}`)
	waitB(`() => !!document.querySelector('[data-conflict="replace"]')`)
	time.Sleep(1500 * time.Millisecond)
	if puts := b.MustEval(`() => fixturePuts`).Int(); puts != 1 {
		t.Fatalf("window B sent %d saves for a clip of removed media, want 1 and the conflict choices", puts)
	}
	// "Replace the latest version" keeps B's edits without the removed media and its clip.
	b.MustElement(`[data-conflict="replace"]`).MustClick()
	waitB(`() => {const s=VideoStudioApp.instances.get('b');return !s.conflict && !s.dirty && !document.querySelector('[data-conflict]');}`)
	if state := b.MustEval(`(id) => {const s=VideoStudioApp.instances.get('b');return JSON.stringify({y:s.project.assets.some(item=>item.id===id),clip:s.project.tracks.some(track=>track.clips.some(clip=>clip.asset_id===id)),notice:document.querySelector('[data-notice]').textContent.includes('left out'),name:s.project.name});}`, y).Str(); state != `{"y":false,"clip":false,"notice":true,"name":"Renamed in B"}` {
		t.Fatalf("window B after replacing: %s", state)
	}
	server := b.MustEval(`async (id) => {const response=await fetch('/api/desktop/video-studio/projects/'+id);const body=await response.json();return JSON.stringify(body.project.assets.map(item=>item.id));}`, projectID).Str()
	if server != fmt.Sprintf(`[%q]`, z) {
		t.Fatalf("the server copy after replacing lists %s, want only %s", server, z)
	}

	// A conflict that B resolves by loading the server version ends clean, without offering B's draft again.
	a.MustEval(`() => {const s=VideoStudioApp.instances.get('a');return VideoStudioParts.refreshProject(s).then(()=>{VideoStudioParts.mutate(s,'rename',project=>{project.name='Renamed in A';});return VideoStudioParts.saveUntilClean(s);});}`)
	waitA(`() => {const s=VideoStudioApp.instances.get('a');return s.project.name==='Renamed in A' && !s.dirty && !s.saving;}`)
	b.MustEval(`() => {window.fixtureConfirmCalls=0;VideoStudioParts.mutate(VideoStudioApp.instances.get('b'),'rename',project=>{project.name='Renamed in B again';});}`)
	waitB(`() => !!document.querySelector('[data-conflict="reload"]')`)
	b.MustElement(`[data-conflict="reload"]`).MustClick()
	waitB(`() => {const s=VideoStudioApp.instances.get('b');return s.project.name==='Renamed in A' && !s.conflict && !s.dirty && !s.saving;}`)
	if calls := b.MustEval(`() => fixtureConfirmCalls`).Int(); calls != 0 {
		t.Fatalf("loading the server version offered the discarded draft (%d confirm dialogs)", calls)
	}
}
