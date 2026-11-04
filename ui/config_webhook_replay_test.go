package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebhookReplayEditorLocales(t *testing.T) {
	source := string(mustReadUIFile(t, "cfg/webhooks.js"))
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var data map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/webhooks/"+lang+".json"), &data); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"label", "help", "requires_signature"} {
			full := "config.webhooks.replay_" + key
			if strings.TrimSpace(data[full]) == "" || !strings.Contains(source, full) {
				t.Errorf("%s missing %s", lang, full)
			}
		}
	}
}

func TestWebhookReplaySaveReopenBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
        const prior=window.fetch;
        window.fetch=(url,options={})=>{
            if(!options.method && ['/api/webhooks','/api/webhooks/presets','/api/tokens','/api/outgoing-webhooks'].includes(String(url))) return Promise.resolve(new Response('[]'));
            return prior(url,options);
        };
        configData.webhooks={enabled:true,readonly:false};
        await selectSection('webhooks',{scrollBehavior:'auto'});
    }`)
	page.MustEval(`()=>{
        window.replaySaved=null;
        whShowEditor();
        document.getElementById('wh-f-name').value='Replay fixture';
        document.getElementById('wh-f-slug').value='replay-fixture';
        window.replayInitiallyOff=!document.getElementById('wh-f-replay').checked;
        document.getElementById('wh-f-replay').checked=true;
        const prior=window.fetch;
        window.fetch=async(url,options={})=>{
            if(options.method==='POST' && url==='/api/webhooks'){
                replaySaved=JSON.parse(options.body);
                return new Response(JSON.stringify({...replaySaved,id:'replay-fixture'}),{status:201});
            }
            return prior(url,options);
        };
    }`)
	page.MustEval(`async()=>{await whSaveWebhook();}`)
	if !page.MustEval(`()=>replayInitiallyOff && replaySaved===null && document.getElementById('wh-editor-error-host').textContent.includes('HMAC-SHA256')`).Bool() {
		t.Fatal("missing signature accepted or default enabled")
	}
	page.MustEval(`async()=>{
        document.getElementById('wh-f-sighdr').value='X-Signature';
        document.getElementById('wh-f-sigalgo').value='sha256';
        document.getElementById('wh-f-sigsec').value='synthetic-fixture';
        await whSaveWebhook();
        whWebhooks=[{...replaySaved,id:'replay-fixture'}];
        whShowEditor('replay-fixture');
    }`)
	if !page.MustEval(`()=>replaySaved.format.replay_protection===true && document.getElementById('wh-f-replay').checked && !document.getElementById('wh-editor-save-btn').disabled`).Bool() {
		t.Fatal("saved boolean lost on reopen")
	}
	for _, width := range []int{1280, 390} {
		page.MustSetViewport(width, 844, 1, false)
		if !page.MustEval(`()=>document.documentElement.scrollWidth<=innerWidth+2`).Bool() {
			t.Fatal("webhook editor overflows viewport")
		}
	}
}
