package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTTSCloudProviderConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	page.MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
        await selectSection('tts', {scrollBehavior:'auto'});
        const realFetch = window.fetch;
        const writes = [];
        let rejectVault = true;
        window.fetch = async (url, options) => {
            if (String(url) === '/api/vault/secrets') {
                writes.push(JSON.parse(options.body));
                if (rejectVault) return new Response(JSON.stringify({status:'error'}), {status:403,headers:{'Content-Type':'application/json'}});
                return new Response(JSON.stringify({status:'ok'}), {status:200,headers:{'Content-Type':'application/json'}});
            }
            return realFetch(url, options);
        };
        const providerSelect = document.querySelector('[data-path="tts.provider"]');
        for (const provider of ['xai','openai','stepfun','resemble']) {
            if (!Array.from(providerSelect.options).some(option => option.value === provider)) throw Error('Missing provider: ' + provider);
            providerSelect.value = provider;
            providerSelect.dispatchEvent(new Event('change', {bubbles:true}));
            const section = document.getElementById('tts-' + provider + '-section');
            if (section.classList.contains('is-hidden')) throw Error('Provider settings hidden: ' + provider);
            for (const other of ['xai','openai','stepfun','resemble']) {
                if (other !== provider && !document.getElementById('tts-' + other + '-section').classList.contains('is-hidden')) throw Error('Other provider settings still visible');
            }
            const input = document.getElementById('tts-' + provider + '-api-key');
            if (input.type !== 'password' || input.value) throw Error('Key is exposed');
            input.value = 'fixture-' + provider;
            rejectVault = true;
            await ttsSaveCloudKey(provider);
            if (input.value !== 'fixture-' + provider) throw Error('Rejected key save cleared the key');
            rejectVault = false;
            await ttsSaveCloudKey(provider);
            const write = writes.at(-1);
            if (write.key !== 'tts_' + provider + '_api_key' || write.value !== 'fixture-' + provider || input.value) throw Error('Vault key save failed: ' + provider);
            const voice = document.querySelector('[data-path="tts.' + provider + '.voice_id"]');
            if (!voice) throw Error('Voice field missing');
            if (voice.tagName === 'SELECT') {
                voice.value = CFG_OPTION_OTHER_CUSTOM;
                voice.dispatchEvent(new Event('change', {bubbles:true}));
                const custom = document.querySelector('[data-custom-for="tts.' + provider + '.voice_id"]');
                if (custom.classList.contains('is-hidden')) throw Error('Custom voice is hidden');
                custom.value = 'custom-' + provider;
                custom.dispatchEvent(new Event('input', {bubbles:true}));
            } else {
                voice.value = 'custom-' + provider;
                voice.dispatchEvent(new Event('input', {bubbles:true}));
            }
            AuraConfigState.syncFromDOM();
            const draft = AuraConfigState.snapshot().draft.tts;
            if (draft.provider !== provider || draft[provider].voice_id !== 'custom-' + provider) throw Error('Draft lost provider or custom voice: ' + JSON.stringify({provider,selected:providerSelect.value,savedProvider:draft.provider,savedVoice:draft[provider].voice_id}));
        }
        const sent = AuraConfigState.snapshot().draft;
        AuraConfigState.commitSent(sent, sent);
        for (const provider of ['xai','openai','stepfun']) {
            const voice = document.querySelector('[data-path="tts.' + provider + '.voice_id"]');
            if (voice.value !== CFG_OPTION_OTHER_CUSTOM || voice.nextElementSibling.value !== 'custom-' + provider) throw Error('Custom voice did not survive save response');
        }
        const text = document.getElementById('content').textContent;
        if (!text.includes('6. Januar 2027') || !text.includes('archiviert')) throw Error('Provider availability notices missing');
        if (text.includes('config.tts.cloud_')) throw Error('Untranslated cloud TTS fields');
        window.fetch = realFetch;
        await selectSection('overview', {scrollBehavior:'auto'});
        await selectSection('tts', {scrollBehavior:'auto'});
        const draft = AuraConfigState.snapshot().draft.tts;
        for (const provider of ['xai','openai','stepfun','resemble']) {
            if (draft[provider].voice_id !== 'custom-' + provider) throw Error('Navigation lost voice settings');
            if (document.getElementById('tts-' + provider + '-api-key').value) throw Error('Navigation re-exposed API key');
            if (document.getElementById('tts-' + provider + '-api-key').placeholder !== t('config.providers.key_placeholder_existing')) throw Error('Stored key indicator was lost');
        }
        AuraConfigState.syncFromDOM();
        if (AuraConfigState.snapshot().draft.tts.resemble.voice_id !== 'custom-resemble') throw Error('Navigation lost the last voice');
        const select = document.querySelector('[data-path="tts.provider"]');
        select.value = 'openai';
        select.dispatchEvent(new Event('change', {bubbles:true}));
    }`)
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`() => document.getElementById('tts-openai-section').scrollIntoView({block:'start'})`)
		if err := os.WriteFile(filepath.Join(dir, "tts-cloud-openai.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
		page.MustSetViewport(390, 844, 1, false)
		page.MustEval(`() => document.getElementById('tts-openai-section').scrollIntoView({block:'start'})`)
		if !page.MustEval(`() => document.documentElement.scrollWidth <= window.innerWidth + 1`).Bool() {
			t.Fatal("cloud TTS settings overflow a mobile viewport")
		}
		if err := os.WriteFile(filepath.Join(dir, "tts-cloud-mobile.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

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
