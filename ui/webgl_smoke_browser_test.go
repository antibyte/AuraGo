package ui

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// newWebGLSmokeBrowser is the smoke launcher with SwiftShader WebGL (index §4.12): no --disable-gpu, ANGLE on
// SwiftShader, and the unsafe-SwiftShader opt-in current Chrome needs. GL tests on it treat missing WebGL2 as a
// failure (requireWebGL2), never as a skip. Like newSmokeBrowser it launches under a 180 s context.
func newWebGLSmokeBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	return newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Delete("disable-gpu")
		l.Set("use-angle", "swiftshader")
		l.Set("enable-unsafe-swiftshader")
	})
}

// requireWebGL2 fails the test when the page offers no WebGL2 context.
func requireWebGL2(t *testing.T, page *rod.Page) {
	t.Helper()
	if page.MustEval(`() => typeof WebGL2RenderingContext === 'function' && !!document.createElement('canvas').getContext('webgl2')`).Bool() {
		return
	}
	renderer := page.MustEval(`() => { const gl = document.createElement('canvas').getContext('webgl'); const ext = gl && gl.getExtension('WEBGL_debug_renderer_info'); return ext ? String(gl.getParameter(ext.UNMASKED_RENDERER_WEBGL)) : 'none'; }`).Str()
	t.Fatalf("WebGL2 is not available under the SwiftShader launcher (WebGL renderer: %s)", renderer)
}
