package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPhononConfigDownloadAndDraftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	page.MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
        const realFetch = window.fetch;
        window.phononTest = {reject:true, downloads:0, installed:false};
        window.fetch = async (url, options) => {
            const path = String(url);
            if (path === '/api/tts/phonon/voices') return new Response(JSON.stringify({ready:true, language:'de', voices:[
                {id:'female',name:'Nele',language:'de',locale:'de-de',gender:'female',installed:true},
                {id:'male',name:'Anton',language:'de',locale:'de-de',gender:'male',installed:true},
                {id:'more',name:'Jette',language:'de',locale:'de-de',gender:'female',installed:phononTest.installed},
                {id:'english',name:'Zoey',language:'en',locale:'en-us',gender:'female',installed:true}
            ]}), {status:200,headers:{'Content-Type':'application/json'}});
            if (path.endsWith('/more/download')) {
                phononTest.downloads++;
                if (options.method !== 'POST') throw Error('download is not explicit POST');
                if (phononTest.reject) return new Response('{}', {status:502});
                phononTest.installed = true;
                return new Response(JSON.stringify({installed:true}), {status:200});
            }
            return realFetch(url, options);
        };
        await selectSection('tts', {scrollBehavior:'auto'});
        await ttsPhononLoadVoices();
        const voice = document.getElementById('phonon-voice-select');
        if (voice.value !== '' || phononTest.downloads !== 0) throw Error('catalog read changed selection or downloaded a voice');
        if (document.querySelectorAll('#phonon-voice-list .tts-voice-item').length !== 3) throw Error('language filter ignored');
        voice.value = 'male'; voice.dispatchEvent(new Event('change', {bubbles:true}));
        AuraConfigState.syncFromDOM();
        const row = Array.from(document.querySelectorAll('#phonon-voice-list .tts-voice-item')).find(row=>row.textContent.includes('Jette'));
        row.querySelector('button').click();
    }`)
	waitForJSBool(t, page, `() => document.getElementById('phonon-status').textContent === t('config.tts.phonon_download_error')`)
	page.MustEval(`() => {
        if (document.getElementById('phonon-voice-select').value !== 'male' || phononTest.downloads !== 1) throw Error('failed download changed the voice');
        phononTest.reject = false;
        const row = Array.from(document.querySelectorAll('#phonon-voice-list .tts-voice-item')).find(row=>row.textContent.includes('Jette'));
        row.querySelector('button').click();
    }`)
	waitForJSBool(t, page, `() => Array.from(document.getElementById('phonon-voice-select').options).some(option=>option.value==='more')`)
	page.MustEval(`async () => {
        const voice = document.getElementById('phonon-voice-select');
        if (voice.value !== 'male' || phononTest.downloads !== 2) throw Error('successful download selected a voice automatically');
        voice.value = 'more'; voice.dispatchEvent(new Event('change', {bubbles:true}));
        AuraConfigState.syncFromDOM();
        const sent = AuraConfigState.snapshot().draft;
        AuraConfigState.commitSent(sent, sent);
        await selectSection('overview', {scrollBehavior:'auto'});
        await selectSection('tts', {scrollBehavior:'auto'});
        await ttsPhononLoadVoices();
        if (document.getElementById('phonon-voice-select').value !== 'more' || AuraConfigState.get('tts').phonon.voice_id !== 'more') throw Error('save/navigation lost the manual voice');
        if (document.getElementById('tts-phonon-section').textContent.includes('config.tts.phonon_')) throw Error('untranslated Phonon controls');
        document.getElementById('tts-phonon-section').scrollIntoView({block:'start'});
    }`)
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "phonon-config.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
		page.MustSetViewport(390, 844, 1, false)
		if !page.MustEval(`() => document.documentElement.scrollWidth <= window.innerWidth + 1`).Bool() {
			t.Fatal("Phonon config overflows mobile viewport")
		}
		if !page.MustEval(`() => ['tts-phonon-section','phonon-voice-list'].every(id => {const node=document.getElementById(id);return node.scrollWidth <= node.clientWidth + 1;})`).Bool() {
			t.Fatal("Phonon controls have horizontal overflow")
		}
		if err := os.WriteFile(filepath.Join(dir, "phonon-config-mobile.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPhononLocaleKeys(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := os.ReadFile(filepath.Join("lang", "config", "tts", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"help", "voice_label", "voice_auto", "voice_help", "catalog_label", "all_languages", "ready", "fallback", "license_help", "male", "female", "installed", "download", "downloading", "catalog_error", "download_error"} {
			if translations["config.tts.phonon_"+key] == "" {
				t.Fatalf("%s: missing %s", locale, key)
			}
		}
	}
}
