package ui

import (
	"testing"
	"time"
)

func TestConfigVideoStudioBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(30 * time.Second)
	page.MustWaitLoad()
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
        const originalFetch = window.fetch;
        window.videoStatusCalls = 0;
        window.fetch = (url, options = {}) => {
            if (String(url) === '/api/desktop/video-studio/status') {
                videoStatusCalls++;
                return Promise.resolve(new Response(JSON.stringify({enabled:true,desktop_enabled:true,ffmpeg_ready:true}), {headers:{'Content-Type':'application/json'}}));
            }
            if (String(url) === '/api/config' && options.method === 'PUT') window.videoSavedConfig = JSON.parse(options.body);
            return originalFetch(url, options);
        };
        await selectSection('virtual_desktop'); resetDirtySnapshot();
    }`)
	if !page.MustEval(`() => document.querySelector('[data-path="video_studio.max_asset_size_mb"]').value==='1024' && !document.querySelector('[data-path="video_studio.enabled"]').classList.contains('on')`).Bool() {
		t.Fatal("Video Studio defaults must be visible and disabled")
	}
	page.MustEval(`() => {
        toggleBool(document.querySelector('[data-path="video_studio.enabled"]'));
        const input = document.querySelector('[data-path="video_studio.ffmpeg_path"]');
        input.value = 'C:/Media Tools/ffmpeg.exe'; input.dispatchEvent(new Event('change', {bubbles:true}));
    }`)
	page.MustEval(`async () => vdCfgTestVideoStudio(document.querySelector('[onclick="vdCfgTestVideoStudio(this)"]'))`)
	if !page.MustEval(`() => videoStatusCalls===0 && document.getElementById('vd-video-studio-result').textContent===t('config.video_studio.save_first')`).Bool() {
		t.Fatal("readiness must not test unsaved settings")
	}
	if !page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("Video Studio settings could not be saved")
	}
	if !page.MustEval(`() => videoSavedConfig.video_studio.enabled===true && videoSavedConfig.video_studio.ffmpeg_path==='C:/Media Tools/ffmpeg.exe'`).Bool() {
		t.Fatal("saving the Desktop section omitted Video Studio settings")
	}
	page.MustEval(`async () => vdCfgTestVideoStudio(document.querySelector('[onclick="vdCfgTestVideoStudio(this)"]'))`)
	if !page.MustEval(`() => videoStatusCalls===1 && document.getElementById('vd-video-studio-result').textContent===t('config.video_studio.ready')`).Bool() {
		t.Fatal("readiness did not reflect the saved settings")
	}
	page.MustSetViewport(390, 844, 1, true)
	if page.MustEval(`() => document.documentElement.scrollWidth>innerWidth+1`).Bool() {
		t.Fatal("Video Studio configuration overflows the narrow viewport")
	}
}
