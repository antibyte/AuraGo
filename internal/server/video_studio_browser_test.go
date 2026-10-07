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
)

// This joins the production browser editor to the real HTTP handlers and FFmpeg.
// Authentication is injected by the test host; media elements cannot set bearer headers.
func TestVideoStudioBrowserRealExport(t *testing.T) {
	browser := personalRadioBrowser(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	s, _, _ := testDesktopPermissionServer(t)
	defer func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
	}()
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
	defer manager.close()
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
<style>html,body,#studio{margin:0;width:100%;height:100%;overflow:hidden}body{background:#151922}</style>
<div id="studio"></div>
<script>window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));</script>
<script src="/js/desktop/apps/video-studio-timeline.js"></script>
<script src="/js/desktop/apps/video-studio-preview.js"></script>
<script src="/js/desktop/apps/video-studio.js"></script>
<script>
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
VideoStudioApp.render(document.getElementById('studio'),'e2e',{
 esc,t:key=>key,iconMarkup:()=>'',promptDialog:async()=> 'Browser export',confirmDialog:async()=>false,
 setWindowBeforeClose:(id,guard)=>{window.closeGuard=guard;},setWindowMenus:()=>{},clearWindowMenus:()=>{}
});
</script>`)
			return
		}
		assets.ServeHTTP(w, r)
	}))
	defer origin.Close()
	page := browser.MustPage(origin.URL + "/fixture").Timeout(90 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustWaitLoad()
	wait := func(condition string) {
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
		t.Fatalf("browser condition timed out: %s; state=%s", condition, page.MustEval(`() => {const s=VideoStudioApp.instances.get('e2e');return JSON.stringify({notice:document.querySelector('[data-notice]')?.textContent,status:s?.status,jobs:s?.jobs,dirty:s?.dirty,saveError:s?.saveError});}`).Str())
	}
	wait(`() => VideoStudioApp.instances.get('e2e')?.status?.ffmpeg_ready===true`)
	page.MustElement(`[data-action="new-project"]`).MustClick()
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
	wait(`() => {
 const canvas=document.querySelector('[data-preview]'), ctx=canvas.getContext('2d');
 const pixel=ctx.getImageData(canvas.width/2,canvas.height/2,1,1).data;
 return pixel[2]>150 && pixel[1]>80 && pixel[0]<100;
}`)
	page.MustEval(`() => {
 const clip=document.querySelector('.vs-clip'), rect=clip.getBoundingClientRect();
 const x=rect.left+rect.width/2,y=rect.top+rect.height/2;
 clip.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,button:0,clientX:x,clientY:y}));
 window.dispatchEvent(new PointerEvent('pointermove',{clientX:x+104,clientY:y}));
 window.dispatchEvent(new PointerEvent('pointerup',{clientX:x+104,clientY:y}));
}`)
	wait(`() => VideoStudioApp.instances.get('e2e').project.tracks.some(t=>t.kind==='overlay'&&t.clips[0]?.start===60)`)
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
