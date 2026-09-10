package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestTTSConfigSanoDefaultsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	page.MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
        await selectSection('tts', {scrollBehavior:'auto'});
        const provider = document.querySelector('[data-path="tts.provider"]');
        const language = document.querySelector('[data-path="tts.language"]');
        if (provider.value !== 'sanotts' || language.value !== 'auto') throw Error('Fresh TTS defaults missing');
        if (!language.selectedOptions[0].textContent.includes('Nutzersprache')) throw Error('Automatic language label missing');
        provider.value = 'google';
        provider.dispatchEvent(new Event('change', {bubbles:true}));
        AuraConfigState.syncFromDOM();
        if (AuraConfigState.snapshot().draft.tts.provider !== 'google') throw Error('Alternative provider not retained');
        provider.value = '';
        provider.dispatchEvent(new Event('change', {bubbles:true}));
        AuraConfigState.syncFromDOM();
        if (AuraConfigState.snapshot().draft.tts.provider !== '') throw Error('Disabled provider not retained');
        provider.value = 'sanotts';
        provider.dispatchEvent(new Event('change', {bubbles:true}));
        language.value = 'de';
        language.dispatchEvent(new Event('change', {bubbles:true}));
        AuraConfigState.syncFromDOM();
        const draft = AuraConfigState.snapshot().draft.tts;
        if (draft.provider !== 'sanotts' || draft.language !== 'de') throw Error('Explicit language not retained');
    }`)
}

func TestQuickSetupKeepsSanoDefaultBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newPrecisionSmokeOrigin(t) + "/setup")
	defer page.MustClose()
	page.MustWaitLoad()
	html := normalizeAssetText(mustReadUIFile(t, "setup.html"))
	html = strings.NewReplacer("{{.Lang}}", "en", "{{.BuildVersion}}", "test", "{{.TemplateDataJSON}}", "{}").Replace(html)
	html = regexp.MustCompile(`(?is)<script\s+src="[^"]+"[^>]*></script>`).ReplaceAllString(html, "")
	page.MustSetDocumentContent(html)
	page.MustEval(`() => {
        window.t = key => key;
        window.fetch = async input => {
            const payload = String(input).includes('/api/setup/status') ? {needs_setup:true,csrf_token:'fixture'} : {profiles:[],personalities:[],auth:{password_set:false}};
            return {ok:true,status:200,json:async()=>payload,text:async()=>JSON.stringify(payload)};
        };
    }`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => {
        selectedProfile = {id:'minimax_coding',name:'MiniMax',provider_type:'minimax',main_model:'fixture-model',features:{},models:{},tts:{provider:'minimax',voice_id:'fixture-voice',model_id:'fixture-tts'}};
        document.getElementById('quick-api-key').value = 'fixture-key';
        document.getElementById('quick-admin-password').value = 'fixture-password';
        document.getElementById('quick-language').value = 'de';
        const patch = buildQuickConfigPatch();
        if (patch.tts.provider !== 'sanotts' || patch.tts.language !== 'auto' || patch.server.ui_language !== 'de') throw Error('Cloud profile replaced local speech default');
        if (patch.tts.minimax.api_key !== 'fixture-key' || patch.tts.minimax.voice_id !== 'fixture-voice') throw Error('Optional cloud credentials lost');
    }`)
}
