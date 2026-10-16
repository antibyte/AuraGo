package ui

import (
	"testing"
)

// The WebGL2 helper on SwiftShader: quiet compile failures that are remembered, cleared and drawn targets read
// top-down, uploads without a flip, the target pool, and loss counting with restore.
func TestDesktopVideoStudioGLHelperBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, ""))
	requireWebGL2(t, page)
	result := page.MustEval(`async () => {
		const S = window.VideoStudioShaders, out = {};
		const stage = document.getElementById('stage');
		const canvas = document.createElement('canvas'); canvas.width = 8; canvas.height = 4; stage.replaceChildren(canvas);
		let lostCalls = 0, restoredCalls = 0;
		const ctx = window.VideoStudioGL.create(canvas, { onLost: () => { lostCalls++; }, onRestored: () => { restoredCalls++; } });
		out.created = !!ctx && ctx.gl instanceof WebGL2RenderingContext;
		let threw = false, broken;
		try { broken = ctx.program('broken', S.QUAD_VERTEX, '#version 300 es\nthis is not glsl'); } catch (_) { threw = true; }
		out.brokenNull = !threw && broken === null && ctx.failed('broken') === true && ctx.program('broken', S.QUAD_VERTEX, S.COPY) === null;
		const copy = ctx.program('copy', S.QUAD_VERTEX, S.COPY);
		out.copyCached = !!copy && ctx.program('copy', S.QUAD_VERTEX, S.COPY) === copy && !ctx.failed('copy');
		const px = (data, x, y, width) => Array.from(data.slice((y * width + x) * 4, (y * width + x) * 4 + 4));
		const target = ctx.target(8, 4);
		ctx.clear(target, [1, 0, 0, 1]);
		const cleared = ctx.read(target);
		out.clearRed = cleared.length === 128 && Array.from(cleared).every((value, index) => value === [255, 0, 0, 255][index % 4]);
		const art = document.createElement('canvas'); art.width = 8; art.height = 4;
		const art2d = art.getContext('2d');
		art2d.fillStyle = '#ff0000'; art2d.fillRect(0, 0, 8, 2);
		art2d.fillStyle = '#0000ff'; art2d.fillRect(0, 2, 8, 2);
		const texture = ctx.texture();
		out.upload = texture.upload(art) === true && texture.width === 8 && texture.height === 4;
		ctx.draw(copy, target, { u_flipY: 0 }, { u_source: texture });
		const rows = ctx.read(target);
		out.targetTop = px(rows, 3, 0, 8); out.targetBottom = px(rows, 3, 3, 8);
		const part = ctx.read(target, { x: 2, y: 3, width: 2, height: 1 });
		out.rectRead = part.length === 8 && part[0] === 0 && part[2] === 255 && part[6] === 255;
		ctx.draw(copy, null, { u_flipY: 1 }, { u_source: target });
		const screen = ctx.read(null);
		out.canvasTop = px(screen, 3, 0, 8); out.canvasBottom = px(screen, 3, 3, 8);
		const probe = document.createElement('canvas'); probe.width = 8; probe.height = 4;
		const view = probe.getContext('2d'); view.drawImage(canvas, 0, 0);
		out.shownTop = Array.from(view.getImageData(3, 0, 1, 1).data);
		ctx.release(target);
		const again = ctx.target(8, 4), other = ctx.target(4, 4);
		out.pooled = again === target && !!other && other !== target;
		ctx.release(again); ctx.release(other);
		const lose = ctx.gl.getExtension('WEBGL_lose_context');
		// An event that never comes fails the run after 10 s instead of hanging it. The pause after a loss is needed:
		// Chrome only accepts restoreContext() once the lost event has been dispatched, and an await continuation runs
		// inside the dispatch (microtask after the listener), where the call is silently refused.
		const next = name => new Promise((resolve, reject) => {
			const timer = setTimeout(() => reject(new Error('timed out waiting for ' + name)), 10000);
			canvas.addEventListener(name, event => { clearTimeout(timer); resolve(event); }, { once: true });
		});
		const afterDispatch = () => new Promise(resolve => setTimeout(resolve, 50));
		let lost = next('webglcontextlost'); lose.loseContext(); await lost; await afterDispatch();
		out.lostState = ctx.lost() && ctx.lossesWithin(60000) === 1 && lostCalls === 1 && ctx.program('copy', S.QUAD_VERTEX, S.COPY) === null && !ctx.failed('copy') && ctx.target(8, 4) === null;
		const restored = next('webglcontextrestored'); lose.restoreContext(); await restored;
		const rebuilt = ctx.program('copy', S.QUAD_VERTEX, S.COPY), fresh = ctx.target(8, 4);
		if (fresh) ctx.clear(fresh, [0, 1, 0, 1]);
		out.restoredState = !ctx.lost() && restoredCalls === 1 && !!rebuilt && rebuilt !== copy && !!fresh && px(ctx.read(fresh), 0, 0, 8).join() === '0,255,0,255' && texture.upload(art) === true;
		lost = next('webglcontextlost'); lose.loseContext(); await lost;
		out.secondLoss = ctx.lossesWithin(60000) === 2 && lostCalls === 2;
		ctx.dispose();
		return out;
	}`)
	checks := result.Map()
	for _, name := range []string{"created", "brokenNull", "copyCached", "clearRed", "upload", "rectRead", "pooled", "lostState", "restoredState", "secondLoss"} {
		if !checks[name].Bool() {
			t.Errorf("GL helper check %s failed: %s", name, result.JSON("", ""))
		}
	}
	for name, want := range map[string][4]int{
		"targetTop": {255, 0, 0, 255}, "targetBottom": {0, 0, 255, 255},
		"canvasTop": {255, 0, 0, 255}, "canvasBottom": {0, 0, 255, 255}, "shownTop": {255, 0, 0, 255},
	} {
		got := checks[name].Arr()
		if len(got) != 4 || got[0].Int() != want[0] || got[1].Int() != want[1] || got[2].Int() != want[2] || got[3].Int() != want[3] {
			t.Errorf("%s = %s, want %v (rows must read top-down and the canvas must show upright)", name, checks[name].JSON("", ""), want)
		}
	}
}
