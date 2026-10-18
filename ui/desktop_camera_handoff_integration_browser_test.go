package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestDesktopCameraHandoffIntegrationBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newPrecisionSmokeOrigin(t) + "/")
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1200, 800, 1, false)
	page.MustSetDocumentContent(`<!doctype html><html><head><meta charset="utf-8"></head><body class="desktop-body" data-theme="standard">
<div id="camera" class="app-slot"></div><div id="pixel" class="app-slot"></div><div id="studio" class="app-slot"></div>
</body></html>`)
	for _, name := range []string{
		"css/camera.css",
		"css/pixel.css",
		"css/desktop-stickers.css",
		"css/pixel-stickers.css",
		"css/desktop-app-common.css",
		"css/desktop-app-video-studio.css",
	} {
		if err := page.AddStyleTag("", normalizeAssetText(mustReadUIFile(t, name))); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	page.MustEval(handoffFixtureJS)
	for _, name := range handoffAppScripts {
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, name))); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	page.MustEval(`() => window.__mountApps()`)
	page.Timeout(20 * time.Second).MustWait(`() => {
		const video = document.querySelector('#camera [data-video]');
		const studio = window.VideoStudioApp.instances.get('studio-1');
		return !!video && video.videoWidth > 0 && !!document.querySelector('#pixel .pixel-app') && studio && studio.studioReady === true && studio.projectId === 'p1';
	}`)

	for _, view := range []struct {
		theme string
		w, h  int
	}{
		{"standard", 1200, 800},
		{"fruity", 1200, 800},
		{"standard", 390, 844},
		{"fruity", 390, 844},
	} {
		page.MustSetViewport(view.w, view.h, 1, false)
		report := page.MustEval(`(theme) => window.__layoutReport(theme)`, view.theme).Str()
		if report != "" {
			t.Fatalf("layout %s %dx%d: %s", view.theme, view.w, view.h, report)
		}
	}
	page.MustSetViewport(1200, 800, 1, false)
	checkHandoffLayout := func(selectors []string) {
		t.Helper()
		for _, width := range []int{1200, 390} {
			for _, theme := range []string{"standard", "fruity"} {
				page.MustSetViewport(width, 844, 1, false)
				page.MustEval(`theme => { document.body.dataset.theme = theme; }`, theme)
				for _, selector := range selectors {
					if !page.MustEval(`selector => {
						const el = document.querySelector(selector);
						if (!el) return false;
						el.scrollIntoView({block:'center', inline:'nearest'});
						const r = el.getBoundingClientRect();
						return r.width > 0 && r.height > 0 && r.left >= 0 && r.right <= innerWidth + 1 && r.top >= 0 && r.bottom <= innerHeight + 1 && el.contains(document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2));
					}`, selector).Bool() {
						t.Fatalf("handoff control unreachable: %s at %s/%d", selector, theme, width)
					}
				}
			}
		}
		page.MustSetViewport(1200, 800, 1, false)
	}

	clickSel(t, page, `#camera [data-action="capture"]`)
	page.Timeout(5 * time.Second).MustWait(`() => document.querySelectorAll('#camera [data-recent]').length === 1 && !document.querySelector('#camera [data-action="open-pixel"]').hidden`)
	checkHandoffLayout([]string{`#camera [data-action="open-pixel"]`, `#camera [data-action="save"]`})
	clickSel(t, page, `#camera [data-action="download"]`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__downloads.some(name => name.endsWith('.jpg'))`)
	clickSel(t, page, `#camera [data-action="save"]`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__notes.includes('desktop.camera_saved') && Object.keys(window.__files).some(path => path.startsWith('Pictures/'))`)
	clickSel(t, page, `#camera [data-action="open-pixel"]`)
	page.Timeout(8 * time.Second).MustWait(`() => {
		const canvas = document.querySelector('#pixel-1 [data-canvas]');
		const recent = JSON.parse(localStorage.getItem('pixel_recent_files') || '[]');
		return !!canvas && canvas.width > 0 && canvas.height > 0 && recent.includes(window.__pixelPath);
	}`)
	before := page.MustEval(`() => document.querySelector('#pixel-1 [data-canvas]').width`).Int()
	clickSel(t, page, `#pixel-1 [data-panel="transform"]`)
	clickSel(t, page, `#pixel-1 [data-action="rotate-cw"]`)
	page.Timeout(5*time.Second).MustWait(`(before) => document.querySelector('#pixel-1 [data-canvas]').width !== before`, before)
	clickSel(t, page, `#pixel-1 [data-action="save"]`)
	page.Timeout(8 * time.Second).MustWait(`() => {
		const saved = window.__saves[0];
		const canvas = document.querySelector('#pixel-1 [data-canvas]');
		return !!saved && saved.path === window.__pixelPath && saved.format === 'jpeg' && saved.w === canvas.width && saved.h === canvas.height && saved.w !== saved.origW;
	}`)
	uploads := page.MustEval(`() => window.__uploadCount`).Int()
	clickSel(t, page, `#camera [data-action="open-pixel"]`)
	page.Timeout(8 * time.Second).MustWait(`() => {
		const canvas = document.querySelector('#pixel-2 [data-canvas]');
		const saved = window.__saves[0];
		return !!canvas && canvas.width === saved.w && canvas.height === saved.h && window.__uploadCount === ` + strconv.Itoa(uploads) + `;
	}`)

	clickSel(t, page, `#camera [data-action="retake"]`)
	page.Timeout(5 * time.Second).MustWait(`() => document.querySelector('#camera .camera-app').dataset.state === 'live'`)
	clickSel(t, page, `#camera [data-action="capture"]`)
	page.Timeout(5 * time.Second).MustWait(`() => !document.querySelector('#camera [data-action="open-pixel"]').hidden`)
	page.MustEval(`() => { window.__failUpload = true; }`)
	clickSel(t, page, `#camera [data-action="open-pixel"]`)
	page.Timeout(5 * time.Second).MustWait(`() => window.__notes.includes('desktop.camera_open_error') && !document.querySelector('#pixel-3')`)
	page.MustEval(`() => { window.__failUpload = false; }`)
	clickSel(t, page, `#camera [data-action="retake"]`)
	page.Timeout(5 * time.Second).MustWait(`() => document.querySelector('#camera .camera-app').dataset.state === 'live' && !document.querySelector('#camera [data-mode="video"]').disabled`)

	clickSel(t, page, `#camera [data-mode="video"]`)
	clickSel(t, page, `#camera [data-action="capture"]`)
	time.Sleep(1200 * time.Millisecond)
	clickSel(t, page, `#camera [data-action="capture"]`)
	if err := page.Timeout(8 * time.Second).Wait(rod.Eval(`() => { const button = document.querySelector('#camera [data-action="open-studio"]'); return !!button && !button.hidden; }`)); err != nil {
		t.Fatalf("studio handoff button: %v %s", err, page.MustEval(`() => JSON.stringify(window.__notes)`).Str())
	}
	clickSel(t, page, `#camera [data-action="open-studio"]`)
	page.Timeout(8 * time.Second).MustWait(`() => !!document.querySelector('#studio [data-camera-project="p1"]')`)
	checkHandoffLayout([]string{`#camera [data-action="open-studio"]`, `#studio [data-camera-project="p1"]`, `#studio [data-camera-new]`, `#studio [data-camera-cancel]`})
	page.MustElement(`#studio [data-camera-project="p1"]`).MustClick()
	page.Timeout(8 * time.Second).MustWait(`() => !!document.querySelector('#studio [data-asset-id="clip-1"]') && (document.querySelector('#studio [data-notice-text]')?.textContent || '').includes('media library')`)
	page.MustEval(`() => document.querySelector('#studio [data-asset-id="clip-1"]').dispatchEvent(new MouseEvent('dblclick', { bubbles: true }))`)
	page.Timeout(5 * time.Second).MustWait(`() => {
		const source = document.querySelector('#studio [data-source]');
		const name = document.querySelector('#studio [data-source-name]')?.textContent || '';
		return !!source && !source.hidden && name && window.__videoName && name.includes(window.__videoName);
	}`)
	if got := page.MustEval(`() => window.VideoStudioApp.instances.get('studio-1').project.tracks.reduce((sum, track) => sum + (track.clips || []).length, 0)`).Int(); got != 0 {
		t.Fatalf("import put the clip on the timeline: %d", got)
	}

	clickSel(t, page, `#camera [data-action="open-studio"]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('#studio [data-camera-cancel]')`)
	media := page.MustEval(`() => window.__media.length`).Int()
	clickSel(t, page, `#studio [data-camera-cancel]`)
	page.Timeout(5 * time.Second).MustWait(`() => !document.querySelector('#studio [data-camera-import]') && (document.querySelector('#studio [data-notice-text]')?.textContent || '').includes('Videos')`)
	if got := page.MustEval(`() => window.__media.length`).Int(); got != media {
		t.Fatalf("cancel imported media: %d", got)
	}

	clickSel(t, page, `#camera [data-action="open-studio"]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('#studio [data-camera-new]')`)
	clickSel(t, page, `#studio [data-camera-new]`)
	page.Timeout(5 * time.Second).MustWait(`() => !!document.querySelector('#studio [data-settings-name]')`)
	page.MustEval(`() => { const input = document.querySelector('#studio [data-settings-name]'); input.value = 'Fresh'; input.form.requestSubmit(); }`)
	page.Timeout(8 * time.Second).MustWait(`() => window.__media.some(row => row.projectId === 'p3') && !!document.querySelector('#studio [data-asset-id="clip-2"]')`)

	page.MustEval(`() => { window.__ro = true; window.__mountReadOnlyStudio(); }`)
	page.Timeout(15 * time.Second).MustWait(`() => window.VideoStudioApp.instances.get('studio-ro')?.studioReady === true`)
	page.MustEval(`() => window.VideoStudioApp.receiveImport('studio-ro', { importPath: window.__videoPath, importToken: '66666666-6666-4666-8666-666666666666' })`)
	state := page.MustEval(`() => JSON.stringify({ dialog: !!document.querySelector('#studio-ro [data-camera-import]'), notice: document.querySelector('#studio-ro [data-notice-text]')?.textContent || '' })`).Str()
	if page.MustEval(`() => !!document.querySelector('#studio-ro [data-camera-import]')`).Bool() || !page.MustEval(`() => (document.querySelector('#studio-ro [data-notice-text]')?.textContent || '').includes('Read-only')`).Bool() {
		t.Fatalf("read-only studio import = %s", state)
	}
}

var handoffAppScripts = []string{
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
	"js/desktop/apps/pixel-state.js",
	"js/desktop/apps/pixel-view.js",
	"js/desktop/apps/pixel-canvas.js",
	"js/desktop/apps/pixel-tools.js",
	"js/desktop/apps/pixel-actions.js",
	"js/desktop/apps/pixel-filters.js",
	"js/desktop/apps/pixel-events.js",
	"js/desktop/apps/pixel-stickers.js",
	"js/desktop/apps/pixel.js",
	"js/desktop/apps/camera.js",
}

const handoffFixtureJS = `() => {
	window.__files = {};
	window.__saves = [];
	window.__notes = [];
	window.__downloads = [];
	window.__media = [];
	window.__assets = [];
	window.__uploadCount = 0;
	window.__clip = 0;
	window.__failUpload = false;
	window.__ro = false;
	window.__pixelPath = '';
	window.__videoPath = '';
	window.__videoName = '';
	localStorage.removeItem('pixel_recent_files');
	const originalAdd = navigator.mediaDevices.addEventListener.bind(navigator.mediaDevices);
	navigator.mediaDevices.addEventListener = (type, fn) => { if (type !== 'devicechange') originalAdd(type, fn); };
	navigator.mediaDevices.removeEventListener = () => {};
	navigator.mediaDevices.getUserMedia = async () => {
		const canvas = document.createElement('canvas');
		canvas.width = 320; canvas.height = 180;
		const paint = canvas.getContext('2d');
		let frame = 0;
		const draw = () => { frame += 1; paint.fillStyle = frame % 2 ? '#1f8f6a' : '#d4552a'; paint.fillRect(0, 0, 320, 180); requestAnimationFrame(draw); };
		draw();
		return canvas.captureStream(15);
	};
	document.addEventListener('click', event => {
		const link = event.target.closest && event.target.closest('a[download]');
		if (link) window.__downloads.push(link.download || '');
	}, true);
	window.XMLHttpRequest = class {
		open() {}
		setRequestHeader() {}
		send(body) {
			const done = (status, payload) => { this.status = status; this.responseText = JSON.stringify(payload); this.readyState = 4; if (this.onreadystatechange) this.onreadystatechange(); if (this.onload) this.onload(); };
			if (!(body instanceof FormData)) { done(200, {}); return; }
			if (window.__failUpload) { done(500, { error: 'upload' }); return; }
			const file = body.get('file');
			const folder = String(body.get('path') || '');
			file.arrayBuffer().then(bytes => {
				window.__uploadCount += 1;
				const path = folder + '/' + file.name;
				window.__files[path] = { type: file.type, bytes };
				if (folder === 'Pictures') window.__pixelPath = path;
				if (folder === 'Videos') { window.__videoPath = path; window.__videoName = file.name; }
				done(200, { status: 'ok', path });
			}).catch(() => done(500, { error: 'upload' }));
		}
	};
	const json = (data, status = 200, etag = '"v1"') => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json', ETag: etag } });
	const tracks = () => [
		{ id: 'v1', kind: 'video', name: 'Video 1', muted: false, hidden: false, locked: false, clips: [] },
		{ id: 'a1', kind: 'audio', name: 'Audio 1', muted: false, hidden: false, locked: false, clips: [] }
	];
	const projectOf = id => ({
		id,
		project: {
			version: 1, name: id === 'p3' ? 'Fresh' : 'One', width: 1280, height: 720, fps: 30, tracks: tracks(),
			assets: window.__assets.filter(asset => asset.projectId === id).map(({ projectId, ...asset }) => asset)
		},
		desktop_path: 'Documents/Video Studio/' + id + '/project.json'
	});
	const nativeFetch = window.fetch.bind(window);
	window.fetch = async (url, options = {}) => {
		const raw = String(url);
		if (raw.startsWith('data:') || raw.startsWith('blob:')) return nativeFetch(url, options);
		const path = raw.split('?')[0];
		const method = String(options.method || 'GET').toUpperCase();
		if (path.endsWith('/api/desktop/preview')) {
			const name = new URL(raw, location.origin).searchParams.get('path');
			const file = window.__files[name];
			if (!file) return new Response('', { status: 404 });
			return new Response(file.bytes, { status: 200, headers: { 'Content-Type': file.type || 'image/jpeg', ETag: '"e1"' } });
		}
		if (path.endsWith('/api/desktop/files')) return json({ files: [] });
		if (path.endsWith('/api/pixel/save') && method === 'POST') {
			const body = JSON.parse(options.body);
			const size = await new Promise(resolve => { const image = new Image(); image.onload = () => resolve({ w: image.naturalWidth, h: image.naturalHeight }); image.onerror = () => resolve({ w: 0, h: 0 }); image.src = body.data; });
			const saved = { path: body.path, format: body.format, w: size.w, h: size.h, origW: 320 };
			const blob = await (await fetch(body.data)).blob();
			window.__files[body.path] = { type: blob.type || 'image/jpeg', bytes: await blob.arrayBuffer() };
			window.__saves.push(saved);
			return json({ path: body.path, version: '"e2"' });
		}
		if (path.endsWith('/status')) return json({ enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: !!window.__ro, limits: { canvas_sizes: [{ width: 1280, height: 720 }], frame_rates: [{ fps: 30 }] }, generation: { enabled: false } });
		if (path.endsWith('/projects') && method === 'GET') return json({ projects: [{ id: 'p1', project: { name: 'One' } }, { id: 'p2', project: { name: 'Two' } }] });
		if (path.endsWith('/projects') && method === 'POST') return json(projectOf('p3'));
		if (/\/projects\/[^/]+$/.test(path) && method === 'GET') return json(projectOf(path.split('/').pop()));
		if (method === 'PUT') return json(projectOf('p1'));
		if (path.endsWith('/media') && method === 'POST') {
			const projectId = path.split('/projects/')[1].split('/')[0];
			const body = JSON.parse(options.body);
			window.__clip += 1;
			window.__assets.push({ projectId, id: 'clip-' + window.__clip, name: String(body.source_path || '').split('/').pop(), kind: 'video', duration_frames: 30, width: 320, height: 180, size_bytes: 1000 });
			window.__media.push({ projectId, body });
			return json({ job: { id: 'job-cam', status: 'succeeded', kind: 'probe' } });
		}
		if (path.endsWith('/jobs/job-cam')) return json({ job: { id: 'job-cam', status: 'succeeded', kind: 'probe' } });
		if (path.includes('/jobs')) return json({ jobs: [] });
		return json({});
	};
	const t = (key, fallback) => fallback || key;
	const esc = value => String(value == null ? '' : value);
	const notify = message => window.__notes.push(typeof message === 'string' ? message : (message && message.message) || '');
	const base = { t, esc, iconMarkup: () => '<i></i>', notify, setWindowMenus() {}, clearWindowMenus() {}, showContextMenu() {}, wireContextMenuBoundary() {}, confirmDialog: async () => false, promptDialog: async () => '', openFileDialog: async () => ({ canceled: true }), saveFileDialog: async () => ({ canceled: true }) };
	window.__openApp = (id, ctx) => {
		if (id === 'pixel') {
			const n = document.querySelectorAll('[data-pixel-window]').length + 1;
			const host = document.createElement('div');
			host.id = 'pixel-' + n;
			host.dataset.pixelWindow = String(n);
			host.className = 'app-slot';
			document.body.appendChild(host);
			window.PixelApp.render(host, host.id, Object.assign({}, base, ctx, { readonly: false }));
			return;
		}
		if (id === 'video-studio') window.VideoStudioApp.receiveImport('studio-1', ctx || {});
	};
	window.__mountApps = () => {
		window.CameraApp.render(document.getElementById('camera'), 'cam', Object.assign({}, base, { openApp: window.__openApp }));
		window.PixelApp.render(document.getElementById('pixel'), 'pixel-shell', Object.assign({}, base, { readonly: false }));
		window.VideoStudioApp.render(document.getElementById('studio'), 'studio-1', Object.assign({}, base, { readonly: false }));
	};
	window.__mountReadOnlyStudio = () => {
		const host = document.createElement('div');
		host.id = 'studio-ro';
		host.className = 'app-slot';
		document.body.appendChild(host);
		window.VideoStudioApp.render(host, 'studio-ro', Object.assign({}, base, { readonly: true }));
	};
	window.__layoutReport = theme => {
		document.body.dataset.theme = theme;
		const problems = [];
		[['#camera .camera-app', '#camera [data-action="capture"]'], ['#pixel .pixel-app', '#pixel [data-action="save"]'], ['#studio .vs-app', '#studio [data-action="play"]']].forEach(([root, control]) => {
			const box = document.querySelector(root);
			const button = document.querySelector(control);
			if (!box || !button) { problems.push('missing ' + root); return; }
			if (box.getBoundingClientRect().width > window.innerWidth + 1) problems.push(root + ' wider than viewport');
			button.scrollIntoView({ block: 'center', inline: 'nearest' });
			const rect = button.getBoundingClientRect();
			if (rect.width < 2 || rect.height < 2 || rect.left < -1 || rect.right > window.innerWidth + 1 || rect.top < -1 || rect.bottom > window.innerHeight + 1) problems.push(control + ' unreachable');
		});
		if (document.documentElement.scrollWidth > window.innerWidth + 2) problems.push('page overflow');
		return problems.join('; ');
	};
}`
