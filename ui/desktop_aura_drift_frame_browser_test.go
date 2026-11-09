package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

// MSAA can interpolate UVs beyond a silhouette: negative heights must not poison HDR bloom.
func TestDesktopAuraDriftAuraShaderBoundsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Delete("disable-gpu").Set("enable-unsafe-swiftshader")
	}).MustPage("about:blank")
	requireWebGL2(t, page)
	page.MustEval(`source => (0, eval)(source)`, readDesktopAssetText(t, "js/vendor/aura-drift/three-runtime.js"))
	page.MustEval(`()=>{window.AuraDriftCore={};}`)
	page.MustEval(`source => (0, eval)(source)`, readDesktopAssetText(t, "js/desktop/apps/aura-drift-fx-resources.js"))
	page.MustEval(`()=>{
        const T = AuraDriftThree, resources = AuraDriftCore.createFxResources(T);
        const renderer = new T.WebGLRenderer(), target = new T.WebGLRenderTarget(16, 4, {type:T.FloatType});
        const scene = new T.Scene(), camera = new T.Camera();
        const material = new T.ShaderMaterial({
            vertexShader:'uniform float probeHeight; varying vec2 vUv; void main(){vUv=vec2(uv.x,probeHeight);gl_Position=vec4(position.xy,0.0,1.0);}',
            fragmentShader:resources.AURA_S_FS,
            uniforms:{probeHeight:{value:0},uColor:{value:new T.Color(2,1,0.3)},uAlpha:{value:0.6},uSpin:{value:0.7}}
        });
        const mesh = new T.Mesh(new T.PlaneGeometry(2, 2), material);
        scene.add(mesh);
        renderer.setClearColor(0, 0);
        renderer.setRenderTarget(target);
        try {
            let lit = 0;
            for (const height of [-0.0001, 0, 0.25, 0.75, 1, 1.0001]) {
                material.uniforms.probeHeight.value = height;
                renderer.render(scene, camera);
                const pixels = new Float32Array(16 * 4 * 4);
                renderer.readRenderTargetPixels(target, 0, 0, 16, 4, pixels);
                if (pixels.some(value=>!Number.isFinite(value)))
                    throw new Error('Drift aura produced non-finite HDR pixels at UV height '+height);
                if (height < 0 && pixels.some(value=>value!==0))
                    throw new Error('Drift aura must fade out below its lower edge');
                if (height === 0.75) lit = pixels.filter((value,i)=>i%4===3 && value>0).length;
            }
            if (!lit || renderer.getContext().getError()) throw new Error('Drift aura shader did not render');
        } finally {
            mesh.geometry.dispose(); material.dispose(); resources.atlas.dispose(); target.dispose(); renderer.dispose();
        }
    }`)
}

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
