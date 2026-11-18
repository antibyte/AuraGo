package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// Read the scene in the drawing task so normal canvas compositing cannot clear the test pixels.
func TestDesktopAuraDriftSteeringFrameBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, quality := range []string{"high", "ultra"} {
		t.Run(quality, func(t *testing.T) {
			page := auraDriftFixture(t)
			requireWebGL2(t, page)
			page.MustSetViewport(960, 640, 1, false)
			auraDriftReopen(page, map[string]interface{}{"quality": quality}, "")
			for _, key := range []input.Key{input.ArrowRight, input.ArrowLeft} {
				page.MustEval(`async()=>{await AuraDriftApp.debug.start({mode:'time_trial',trackId:'alpenpass'});}`)
				auraDriftSkipCountdown(t, page)
				page.MustEval(`()=>{AuraDriftApp.debug.race().cars[0].v.vx=25;}`)
				auraDriftKeyDown(t, page, input.ArrowUp)
				auraDriftKeyDown(t, page, key)
				page.MustEval(`()=>{
            AuraDriftApp.debug.fastForward(0.5, {renderEvery:15});
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
		})
	}
}
