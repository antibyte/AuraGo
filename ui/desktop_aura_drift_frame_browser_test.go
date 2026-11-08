package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// The last completed frame must survive compositing while a busy GPU prepares
// the next one; the HUD is a separate canvas and must not cover an empty scene.
func TestDesktopAuraDriftKeepsLastFrameBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	requireWebGL2(t, page)
	page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
	pixel := page.MustEval(`async()=>{
        const canvas = document.createElement('canvas');
        canvas.width = 64; canvas.height = 64;
        canvas.style.cssText = 'position:fixed;left:0;top:0;z-index:2147483647';
        document.body.appendChild(canvas);
        let renderer;
        try {
            renderer = AuraDriftCore.createRenderer3D(canvas, {quality:'low'}, key=>key);
            if (!renderer) throw new Error('Aura Drift WebGL renderer unavailable');
            const gl = canvas.getContext('webgl2');
            gl.clearColor(0.3, 0.6, 0.9, 1);
            gl.clear(gl.COLOR_BUFFER_BIT);
            gl.finish();
            await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
            const pixel = new Uint8Array(4);
            gl.readPixels(32, 32, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, pixel);
            return Array.from(pixel);
        } finally {
            if (renderer) renderer.dispose();
            canvas.remove();
        }
    }`).Arr()
	if len(pixel) != 4 || pixel[0].Int() < 70 || pixel[1].Int() < 145 || pixel[2].Int() < 220 || pixel[3].Int() != 255 {
		t.Fatalf("Aura Drift lost its completed scene between draws: pixel=%v", pixel)
	}

	auraDriftReopen(page, map[string]interface{}{"quality": "high"}, "")
	for _, key := range []input.Key{input.ArrowRight, input.ArrowLeft} {
		page.MustEval(`async()=>{await AuraDriftApp.debug.start({mode:'time_trial',trackId:'alpenpass'});}`)
		auraDriftSkipCountdown(t, page)
		page.MustEval(`()=>{AuraDriftApp.debug.race().cars[0].v.vx=25;}`)
		auraDriftKeyDown(t, page, input.ArrowUp)
		auraDriftKeyDown(t, page, key)
		auraDriftDrive(page, 0.5)
		page.MustEval(`()=>{
            const canvas = document.querySelector('canvas.agdrift-gl'), gl = canvas.getContext('webgl2');
            const pixels = new Uint8Array(canvas.width * canvas.height * 4);
            gl.readPixels(0, 0, canvas.width, canvas.height, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
            let bright = 0;
            for (let i = 0; i < pixels.length; i += 4) if (pixels[i] + pixels[i+1] + pixels[i+2] > 64) bright++;
            const v = AuraDriftApp.debug.race().cars[0].v;
            if (Math.abs(v.steer) < 0.1 || v.slip <= 2 || bright < canvas.width * canvas.height / 8)
                throw new Error('Steering must leave the racing scene visible: '+JSON.stringify({steer:v.steer,slip:v.slip,bright}));
        }`)
		auraDriftKeyUp(t, page, key)
		auraDriftKeyUp(t, page, input.ArrowUp)
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}
