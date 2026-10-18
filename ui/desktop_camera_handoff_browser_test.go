package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

type uploadInfo struct {
	Unique string `json:"unique"`
	Folder string `json:"folder"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Header string `json:"header"`
}

func clickSel(t *testing.T, page *rod.Page, sel string) {
	t.Helper()
	page.MustEval(`(sel) => {
		const el = document.querySelector(sel);
		if (!el) throw new Error('missing ' + sel);
		el.click();
	}`, sel)
}

func TestDesktopCameraHandoffBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><body><div id=\"host\"></div></body></html>"))
	}))
	t.Cleanup(server.Close)
	page := browser.MustPage(server.URL + "/")
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1100, 760, 1, false)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/camera.js"))); err != nil {
		t.Fatalf("load camera: %v", err)
	}
	page.MustEval(`() => {
		window.__canvases = [];
		navigator.mediaDevices.getUserMedia = async () => {
			const canvas = document.createElement('canvas');
			canvas.width = 320; canvas.height = 180;
			const paint = canvas.getContext('2d');
			let frame = 0;
			const draw = () => {
				frame += 1;
				paint.fillStyle = frame % 2 ? '#1f8f6a' : '#146b4e';
				paint.fillRect(0, 0, 320, 180);
				requestAnimationFrame(draw);
			};
			draw();
			window.__canvases.push(canvas);
			return canvas.captureStream(15);
		};
		const originalAdd = navigator.mediaDevices.addEventListener.bind(navigator.mediaDevices);
		navigator.mediaDevices.addEventListener = (type, fn) => { if (type !== 'devicechange') originalAdd(type, fn); };
		navigator.mediaDevices.removeEventListener = () => {};
		window.__uploads = [];
		window.__opened = [];
		window.__notes = [];
		window.XMLHttpRequest = class {
			open() {}
			setRequestHeader(name, value) { this.headers = Object.assign({}, this.headers, {[name]: value}); }
			send(form) {
				const file = form.get('file');
				const row = { unique: form.get('unique'), folder: form.get('path'), name: file && file.name, type: file && file.type, header: this.headers && this.headers['If-None-Match'] };
				row.finish = (status, body) => { this.status = status; this.responseText = JSON.stringify(body); if (this.onload) this.onload(); };
				window.__uploads.push(row);
			}
		};
		window.CameraApp.render(document.getElementById('host'), 'cam-1', {
			t: key => key,
			esc: value => String(value == null ? '' : value),
			iconMarkup: () => '<i></i>',
			notify: message => window.__notes.push(message),
			openApp: (id, ctx) => window.__opened.push({ id, ctx }),
			setWindowMenus() {}, clearWindowMenus() {}, showContextMenu() {}, wireContextMenuBoundary() {}
		});
	}`)
	page.Timeout(8 * time.Second).MustWait(`() => { const video = document.querySelector('[data-video]'); return !!video && video.videoWidth > 0; }`)
	clickSel(t, page, `[data-action="capture"]`)
	page.Timeout(5 * time.Second).MustWait(`() => { const button = document.querySelector('[data-action="open-pixel"]'); return !!button && !button.hidden && document.querySelector('[data-action="open-studio"]').hidden; }`)
	page.MustEval(`() => { const btn = document.querySelector('[data-action="open-pixel"]'); btn.click(); btn.disabled = false; btn.click(); }`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__uploads.length === 1`)
	var info uploadInfo
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__uploads[0])`).Str()), &info); err != nil {
		t.Fatal(err)
	}
	if info.Unique != "1" || info.Header != "*" || info.Folder != "Pictures" || len(info.Name) < 4 || info.Name[len(info.Name)-4:] != ".jpg" || info.Type != "image/jpeg" {
		t.Fatalf("photo upload = %+v", info)
	}
	page.MustEval(`() => window.__uploads[0].finish(200, {status:'ok', path:'Pictures/photo_test.jpg'})`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__opened.length === 1 && window.__opened[0].ctx.forceNew === true && window.__opened[0].ctx.path === 'Pictures/photo_test.jpg'`)
	clickSel(t, page, `[data-action="open-pixel"]`)
	if got := page.MustEval(`() => window.__uploads.length`).Int(); got != 1 {
		t.Fatalf("second pixel open uploaded again: %d", got)
	}
	page.Timeout(5 * time.Second).MustWait(`() => window.__opened.length === 2`)

	clickSel(t, page, `[data-action="retake"]`)
	page.Timeout(5 * time.Second).MustWait(`() => document.querySelector('.camera-app').dataset.state === 'live'`)
	clickSel(t, page, `[data-action="capture"]`)
	page.Timeout(5 * time.Second).MustWait(`() => !document.querySelector('[data-action="open-pixel"]').hidden`)
	clickSel(t, page, `[data-action="open-pixel"]`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__uploads.length === 2`)
	page.MustEval(`() => window.__uploads[1].finish(500, {error:'nope'})`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__notes.includes('desktop.camera_open_error') && window.__opened.length === 2`)

	clickSel(t, page, `[data-action="open-pixel"]`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__uploads.length === 3`)
	page.MustEval(`() => window.CameraApp.dispose('cam-1')`)
	page.MustEval(`() => window.__uploads[2].finish(200, {status:'ok', path:'Pictures/late.jpg'})`)
	if got := page.MustEval(`() => window.__opened.length`).Int(); got != 2 {
		t.Fatalf("disposed camera opened an app from a late upload: %d", got)
	}

	page.MustEval(`() => window.CameraApp.render(document.getElementById('host'), 'cam-2', {
		t: key => key, esc: value => String(value == null ? '' : value), iconMarkup: () => '<i></i>',
		notify: message => window.__notes.push(message), openApp: (id, ctx) => window.__opened.push({ id, ctx }),
		setWindowMenus() {}, clearWindowMenus() {}, showContextMenu() {}, wireContextMenuBoundary() {}
	})`)
	page.Timeout(8 * time.Second).MustWait(`() => document.querySelector('[data-camera-app="cam-2"] [data-video]')?.videoWidth > 0`)
	clickSel(t, page, `[data-camera-app="cam-2"] [data-mode="video"]`)
	clickSel(t, page, `[data-camera-app="cam-2"] [data-action="capture"]`)
	time.Sleep(1500 * time.Millisecond)
	clickSel(t, page, `[data-camera-app="cam-2"] [data-action="capture"]`)
	if err := page.Timeout(8 * time.Second).Wait(rod.Eval(`() => { const button = document.querySelector('[data-camera-app="cam-2"] [data-action="open-studio"]'); return !!button && !button.hidden; }`)); err != nil {
		info := page.MustEval(`() => JSON.stringify({
			notes: window.__notes,
			mode: document.querySelector('[data-camera-app="cam-2"]')?.dataset.mode,
			state: document.querySelector('[data-camera-app="cam-2"]')?.dataset.state,
			recording: document.querySelector('[data-camera-app="cam-2"]')?.classList.contains('is-recording'),
			width: document.querySelector('[data-camera-app="cam-2"] [data-video]')?.videoWidth
		})`).Str()
		t.Fatalf("studio button: %v %s", err, info)
	}
	clickSel(t, page, `[data-camera-app="cam-2"] [data-action="open-studio"]`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__uploads.length === 4`)
	var video uploadInfo
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__uploads[3])`).Str()), &video); err != nil {
		t.Fatal(err)
	}
	ext := ""
	if video.Type == "video/webm" {
		ext = ".webm"
	} else if video.Type == "video/mp4" {
		ext = ".mp4"
	} else if video.Type == "video/quicktime" {
		ext = ".mov"
	}
	if video.Folder != "Videos" || video.Header != "*" || video.Unique != "1" || ext == "" || len(video.Name) < len(ext) || video.Name[len(video.Name)-len(ext):] != ext {
		t.Fatalf("video upload = %+v", video)
	}
	page.MustEval(`() => window.__uploads[3].finish(200, {status:'ok', path:'Videos/' + window.__uploads[3].name})`)
	page.Timeout(5 * time.Second).MustWait(`() => { const last = window.__opened.at(-1); return last && last.id === 'video-studio' && last.ctx.importPath.startsWith('Videos/') && !last.ctx.path && /^[A-Za-z0-9-]{8,80}$/.test(last.ctx.importToken); }`)
}

func TestDesktopVideoStudioCameraImportBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	server := httptest.NewServer(http.FileServer(http.FS(Content)))
	t.Cleanup(server.Close)
	page := browser.MustPage(server.URL + "/")
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1200, 860, 1, false)
	page.MustSetDocumentContent(`<!doctype html><html><body><div id="studio" style="width:1100px;height:760px"></div></body></html>`)
	page.MustEval(`() => {
		window.__media = [];
		window.__projects = [];
		window.__failSave = false;
		window.__readonly = false;
		window.__release = null;
		const project = () => ({ version: 1, name: 'One', width: 1280, height: 720, fps: 30, assets: window.__assets || [], tracks: [
			{ id: 'v1', kind: 'video', name: 'Video 1', muted: false, hidden: false, locked: false, clips: [] },
			{ id: 'a1', kind: 'audio', name: 'Audio 1', muted: false, hidden: false, locked: false, clips: [] }
		] });
		window.__project = project;
		const json = (data, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json', ETag: '"v1"' } });
		window.fetch = async (url, options = {}) => {
			const path = String(url).split('?')[0];
			const method = String(options.method || 'GET').toUpperCase();
			if (path.endsWith('/status')) return json({ enabled: !window.__disabled, desktop_enabled: true, ffmpeg_ready: !window.__disabled, read_only: !!window.__readonly, limits: { canvas_sizes: [{ width: 1280, height: 720 }], frame_rates: [{ fps: 30 }] }, generation: { enabled: false } });
			if (path.endsWith('/projects') && method === 'GET') return json({ projects: [{ id: 'p1', project: { name: 'One' } }, { id: 'p2', project: { name: 'Two' } }] });
			if (/\/projects\/[^/]+$/.test(path) && method === 'GET') return json({ id: path.split('/').pop(), project: window.__project(), desktop_path: 'Documents/Video Studio/p1/project.json' });
			if (path.endsWith('/projects') && method === 'POST') { window.__projects.push(JSON.parse(options.body)); return json({ id: 'p3', project: { version: 1, name: 'Fresh', width: 1280, height: 720, fps: 30, assets: [], tracks: [] }, desktop_path: 'Documents/Video Studio/p3/project.json' }); }
			if (method === 'PUT') { if (window.__failSave) return json({ code: 'save_failed' }, 500); return json({ project: window.__project(), desktop_path: 'Documents/Video Studio/p1/project.json' }); }
			if (path.endsWith('/media') && method === 'POST') {
				const key = new Headers(options.headers).get('Idempotency-Key');
				const body = JSON.parse(options.body);
				window.__media.push({ path, key, body });
				if (window.__hold) return new Promise(resolve => { window.__release = () => resolve(json({ job: { id: 'job-cam', status: 'succeeded', kind: 'probe' } })); });
				return json({ job: { id: 'job-cam', status: 'succeeded', kind: 'probe' } });
			}
			if (path.endsWith('/jobs/job-cam')) return json({ job: { id: 'job-cam', status: 'succeeded', kind: 'probe' } });
			if (path.includes('/jobs')) return json({ jobs: [] });
			return json({});
		};
	}`)
	scripts := []string{
		"js/desktop/shared/sticker-catalog.js",
		"js/desktop/shared/sticker-library.js",
		"js/desktop/apps/video-studio-icons.js",
		"js/desktop/apps/video-studio-time.js",
		"js/desktop/apps/video-studio-effects.js",
		"js/desktop/apps/video-studio-picture.js",
		"js/desktop/apps/video-studio-media.js",
		"js/desktop/apps/video-studio-gl.js",
		"js/desktop/apps/video-studio-shaders.js",
		"js/desktop/apps/video-studio-compositor.js",
		"js/desktop/apps/video-studio-painter2d.js",
		"js/desktop/apps/video-studio-preview.js",
		"js/desktop/apps/video-studio-timeline.js",
		"js/desktop/apps/video-studio-ruler.js",
		"js/desktop/apps/video-studio-edit.js",
		"js/desktop/apps/video-studio-keyframe-edit.js",
		"js/desktop/apps/video-studio-navigation.js",
		"js/desktop/apps/video-studio-source-edit.js",
		"js/desktop/apps/video-studio-folders.js",
		"js/desktop/apps/video-studio-inspector.js",
		"js/desktop/apps/video-studio-colorwheel.js",
		"js/desktop/apps/video-studio-core.js",
		"js/desktop/apps/video-studio-library.js",
		"js/desktop/apps/video-studio-effects-library.js",
		"js/desktop/apps/video-studio-artwork.js",
		"js/desktop/apps/video-studio-jobs.js",
		"js/desktop/apps/video-studio-stage.js",
		"js/desktop/apps/video-studio-mask.js",
		"js/desktop/apps/video-studio-editing.js",
		"js/desktop/apps/video-studio-keyframes.js",
		"js/desktop/apps/video-studio-inspector-effects.js",
		"js/desktop/apps/video-studio-source.js",
		"js/desktop/apps/video-studio-tracks.js",
		"js/desktop/apps/video-studio-markers.js",
		"js/desktop/apps/video-studio-transport.js",
		"js/desktop/apps/video-studio-rangerender.js",
		"js/desktop/apps/video-studio-derive.js",
		"js/desktop/apps/video-studio-projects.js",
		"js/desktop/apps/video-studio-mediabin.js",
		"js/desktop/apps/video-studio-menus.js",
		"js/desktop/apps/video-studio.js",
	}
	for _, name := range scripts {
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, name))); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	page.MustEval(`() => {
		const ctx = {
			esc: value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch]),
			t: key => key, iconMarkup: () => '', confirmDialog: async () => false, promptDialog: async () => '',
			openFileDialog: async () => ({ canceled: true }), setWindowMenus() {}, clearWindowMenus() {}, setWindowBeforeClose() {}
		};
		window.__ctx = ctx;
		window.VideoStudioApp.render(document.getElementById('studio'), 'w1', ctx);
	}`)
	page.Timeout(15 * time.Second).MustWait(`() => window.VideoStudioApp.instances.get('w1')?.studioReady === true && window.VideoStudioApp.instances.get('w1')?.projectId === 'p1'`)
	clipCount := page.MustEval(`() => window.VideoStudioApp.instances.get('w1').project.tracks.reduce((sum, track) => sum + (track.clips || []).length, 0)`).Int()
	page.MustEval(`() => window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/clip.webm', importToken: '11111111-1111-4111-8111-111111111111', sessionRestore: { key: 'nope' } })`)
	if got := page.MustEval(`() => document.querySelectorAll('[data-camera-import]').length`).Int(); got != 0 {
		t.Fatalf("session restore opened import chooser: %d", got)
	}
	page.MustEval(`() => window.VideoStudioApp.receiveImport('w1', { importPath: 'https://evil.example/clip.webm', importToken: '11111111-1111-4111-8111-111111111111' })`)
	if got := page.MustEval(`() => document.querySelectorAll('[data-camera-import]').length`).Int(); got != 0 {
		t.Fatal("unsafe path opened import chooser")
	}
	page.MustEval(`() => window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/clip.webm', importToken: '11111111-1111-4111-8111-111111111111' })`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-camera-import]')`)
	clickSel(t, page, `[data-camera-cancel]`)
	page.Timeout(5 * time.Second).MustWait(`() => !document.querySelector('[data-camera-import]') && document.querySelector('[data-notice-text]')?.textContent.includes('Videos')`)
	if got := page.MustEval(`() => window.__media.length`).Int(); got != 0 {
		t.Fatalf("cancel imported media: %d", got)
	}
	page.MustEval(`() => { window.__hold = true; window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/clip.webm', importToken: '22222222-2222-4222-8222-222222222222' }); }`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-camera-project="p1"]')`)
	page.MustEval(`() => { const btn = document.querySelector('[data-camera-project="p1"]'); btn.click(); btn.click(); }`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__media.length === 1`)
	page.MustEval(`() => window.__release()`)
	page.Timeout(8 * time.Second).MustWait(`() => !document.querySelector('[data-camera-import]') && document.querySelector('[data-notice-text]')?.textContent.length > 0`)
	var media struct {
		Key  string `json:"key"`
		Body struct {
			SourcePath string `json:"source_path"`
			Folder     string `json:"folder"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__media[0])`).Str()), &media); err != nil {
		t.Fatal(err)
	}
	if media.Key == "" || media.Body.SourcePath != "Videos/clip.webm" || media.Body.Folder != "" {
		t.Fatalf("import request = %+v", media)
	}
	if got := page.MustEval(`() => window.VideoStudioApp.instances.get('w1').project.tracks.reduce((sum, track) => sum + (track.clips || []).length, 0)`).Int(); got != clipCount {
		t.Fatalf("import changed timeline clips from %d to %d", clipCount, got)
	}
	page.MustEval(`() => {
		window.__errors = [];
		window.addEventListener('unhandledrejection', event => window.__errors.push(String(event.reason && (event.reason.stack || event.reason.message) || event.reason)));
		const s = window.VideoStudioApp.instances.get('w1');
		s.dirty = true; s.project.name = 'Edited'; window.__failSave = true;
		window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/clip.webm', importToken: '33333333-3333-4333-8333-333333333333' });
	}`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-camera-project="p2"]')`)
	clickSel(t, page, `[data-camera-project="p2"]`)
	if err := page.Timeout(8 * time.Second).Wait(rod.Eval(`() => { const s = window.VideoStudioApp.instances.get('w1'); return s.projectId === 'p1' && !!document.querySelector('[data-camera-import]'); }`)); err != nil {
		info := page.MustEval(`() => { const s = window.VideoStudioApp.instances.get('w1'); return JSON.stringify({ projectId: s.projectId, dirty: s.dirty, phase: s.importOffer && s.importOffer.phase, dialog: !!document.querySelector('[data-camera-import]'), notice: document.querySelector('[data-notice-text]')?.textContent, media: window.__media.length, errors: window.__errors, saveError: s.saveError, conflict: s.conflict }); }`).Str()
		t.Fatalf("dirty switch: %v %s", err, info)
	}
	if got := page.MustEval(`() => window.__media.length`).Int(); got != 1 {
		t.Fatalf("dirty switch imported anyway: %d", got)
	}
	page.MustEval(`() => { window.__failSave = false; const s = window.VideoStudioApp.instances.get('w1'); s.dirty = false; s.importOffer = null; document.querySelector('[data-modal-host]').replaceChildren(); window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/clip.webm', importToken: '44444444-4444-4444-8444-444444444444' }); }`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-camera-new]')`)
	clickSel(t, page, `[data-camera-new]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-new-project-form]')`)
	clickSel(t, page, `[data-modal-close]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-camera-import]') && window.__projects.length === 0`)
	clickSel(t, page, `[data-camera-new]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-settings-name]')`)
	page.MustEval(`() => { window.__hold = false; const input = document.querySelector('[data-settings-name]'); input.value = 'Fresh'; input.form.requestSubmit(); }`)
	page.Timeout(8 * time.Second).MustWait(`() => window.__media.length === 2 && window.__media[1].path.includes('/projects/p3/media') && !window.VideoStudioApp.instances.get('w1').importOffer`)
	page.MustEval(`() => {
		const s = window.VideoStudioApp.instances.get('w1');
		window.VideoStudioParts.openProjectSettings(s);
		document.querySelector('[data-settings-name]').value = 'Unsaved settings';
		window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/other.webm', importToken: '66666666-6666-4666-8666-666666666666' });
	}`)
	if !page.MustEval(`() => document.querySelector('[data-settings-name]')?.value === 'Unsaved settings' && !window.VideoStudioApp.instances.get('w1').importOffer`).Bool() {
		t.Fatal("incoming clip replaced unsaved project settings")
	}
	page.Keyboard.MustType(input.Escape)
	page.MustEval(`() => window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/escape.webm', importToken: '77777777-7777-4777-8777-777777777777' })`)
	clickSel(t, page, `[data-camera-new]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-new-project-form]')`)
	page.Keyboard.MustType(input.Escape)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-camera-import]') && !window.VideoStudioApp.instances.get('w1').cameraImportAwaitCreate`)
	page.Keyboard.MustType(input.Escape)
	page.Timeout(5 * time.Second).MustWait(`() => !document.querySelector('[data-camera-import]') && !window.VideoStudioApp.instances.get('w1').importOffer`)
	page.MustEval(`() => window.VideoStudioParts.openNewProject(window.VideoStudioApp.instances.get('w1'))`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('[data-new-project-form]')`)
	page.MustEval(`() => document.querySelector('[data-new-project-form]').requestSubmit()`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__projects.length === 2 && !document.querySelector('[data-new-project-form]')`)
	if got := page.MustEval(`() => window.__media.length`).Int(); got != 2 {
		t.Fatalf("cancelled handoff imported into an unrelated new project: %d", got)
	}
	state := page.MustEval(`() => {
		const s = window.VideoStudioApp.instances.get('w1');
		s.readonly = true; s.importOffer = null;
		window.VideoStudioApp.receiveImport('w1', { importPath: 'Videos/clip.webm', importToken: '55555555-5555-4555-8555-555555555555' });
		return JSON.stringify({ readonly: s.readonly, offer: !!s.importOffer, dialog: !!document.querySelector('[data-camera-import]'), notice: document.querySelector('[data-notice-text]')?.textContent || '', noticeEl: !!document.querySelector('[data-notice]') });
	}`).Str()
	var readonlyState struct {
		Dialog bool   `json:"dialog"`
		Notice string `json:"notice"`
	}
	if err := json.Unmarshal([]byte(state), &readonlyState); err != nil {
		t.Fatal(err)
	}
	if readonlyState.Dialog || readonlyState.Notice == "" {
		t.Fatalf("read-only import = %s", state)
	}
	if got := page.MustEval(`() => window.__media.length`).Int(); got != 2 {
		t.Fatalf("read-only receive imported media: %d", got)
	}
}
