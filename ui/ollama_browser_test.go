package ui

import (
	"testing"
	"time"
)

func TestOllamaManagedLifecycleBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#ollama").Timeout(35 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>typeof renderOllamaSection==='function'&&!!document.querySelector('[data-path="ollama.url"]')`)
	page.MustEval(`async()=>{
        configData.docker={enabled:true,readonly:false};
        configData.ollama={enabled:true,url:'http://remote.saved:11434',managed_instance:{enabled:true,container_port:11434,default_models:[]}};
        runtimeData.features={docker:{available:true}};
        runtimeData.runtime={is_docker:true};
        AuraConfigState.init(configData);
        window.__ollamaPosts=0;window.__ollamaStatusReads=0;window.__ollamaReady=false;window.__ollamaModelsReads=0;
        window.__ollamaRealFetch=window.fetch.bind(window);
        window.showConfirm=async()=>false;
        window.fetch=async(url,options)=>{
            if(String(url).includes('/recreate')){window.__ollamaPosts++;return {ok:true,status:202,json:async()=>({status:'accepted',operation:{id:'fixture-ollama'}})}}
            if(String(url).includes('/models')){window.__ollamaModelsReads++;return {ok:true,status:200,json:async()=>({version:'fixture',models:['one','two']})}};
            if(String(url).includes('/status')){window.__ollamaStatusReads++;const state=window.__ollamaState||(window.__ollamaReady?'ready':'model_pull');return {ok:true,status:200,json:async()=>({running:true,operations:[{id:'fixture-ollama',state,models:{'fixture-embedding':state},error:state==='failed'?'fixture failure':'',updated:new Date().toISOString()}]})}}
            return window.__ollamaRealFetch(url,options);
        };
        await renderOllamaSection({label:'Ollama',desc:''});
    }`)
	if !page.MustEval(`()=>document.querySelector('[data-path="ollama.url"]').readOnly&&document.querySelector('[data-path="ollama.url"]').value==='http://aurago_ollama_managed:11434'`).Bool() {
		t.Fatal("managed URL not derived and read-only")
	}
	waitForJSBool(t, page, `()=>window.__ollamaStatusReads>=2`)
	page.MustElement("#ollama-test-btn").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#ollama-test-result').textContent.includes('fixture')&&document.querySelector('#ollama-test-result').textContent.includes('2')`)
	page.MustElement("#ollama-recreate-btn").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#ollama-recreate-btn').dataset.configActionBusy==='false'`)
	if page.MustEval(`()=>window.__ollamaPosts`).Int() != 0 {
		t.Fatal("cancelled confirmation mutated container")
	}
	page.MustEval(`()=>{window.showConfirm=async()=>true;window.__ollamaReady=true}`)
	page.MustElement("#ollama-recreate-btn").MustClick()
	waitForJSBool(t, page, `()=>window.__ollamaPosts===1&&document.querySelector('#ollama-managed-banner').textContent.includes('Bereit')`)
	page.MustEval(`()=>AuraConfigState.set('ollama.managed_instance.container_port',11436)`)
	waitForJSBool(t, page, `()=>document.querySelector('#ollama-test-btn').dataset.configActionLocked==='true'&&document.querySelector('#ollama-recreate-btn').dataset.configActionLocked==='true'`)
	page.MustElement("#ollama-test-btn").MustClick()
	page.MustElement("#ollama-recreate-btn").MustClick()
	if !page.MustEval(`()=>window.__ollamaPosts===1&&window.__ollamaModelsReads===1`).Bool() {
		t.Fatal("unsaved settings allowed saved-endpoint actions")
	}
	page.MustEval(`async()=>{AuraConfigState.init(configData);window.__ollamaState='failed';await ollamaManagedCheckStatus(true)}`)
	if !page.MustEval(`()=>document.querySelector('#ollama-managed-banner').textContent.includes('Fehlgeschlagen')&&document.querySelector('#ollama-managed-banner').textContent.includes('fixture failure')`).Bool() {
		t.Fatal("failed operation was not displayed")
	}
	page.MustEval(`async()=>{
        window.__ollamaState='ready';configData.embeddings={local_ollama:{enabled:true}};
        const element=document.createElement('div');element.id='ollama-embedding-status';document.getElementById('content').appendChild(element);
        await refreshOllamaEmbeddingStatus();
    }`)
	if !page.MustEval(`()=>document.querySelector('#ollama-embedding-status').textContent.includes('fixture-embedding: Bereit')`).Bool() {
		t.Fatal("embedding status omitted individual models")
	}
	page.MustEval(`async()=>{configData.docker.readonly=true;AuraConfigState.init(configData);await renderOllamaSection({label:'Ollama',desc:''})}`)
	if !page.MustElement("#ollama-recreate-btn").MustProperty("disabled").Bool() {
		t.Fatal("read-only Docker left recreate enabled")
	}
	page.MustSetViewport(390, 844, 1, true)
	if !page.MustEval(`()=>{const content=document.getElementById('content');return content.scrollWidth<=content.clientWidth+1}`).Bool() {
		t.Fatal("Ollama settings overflow mobile viewport")
	}
	page.MustEval(`()=>{
        window.__ollamaOldBanner=document.getElementById('ollama-managed-banner');
        window.fetch=(url,options)=>String(url).includes('/api/ollama/managed/status')?new Promise(resolve=>{window.__ollamaPendingSignal=options.signal;window.__ollamaResolveStatus=resolve}):window.__ollamaRealFetch(url,options);
        ollamaManagedCheckStatus(true);location.hash='#overview';
    }`)
	waitForJSBool(t, page, `()=>window.__ollamaPendingSignal?.aborted&&!window.__ollamaOldBanner.isConnected`)
	page.MustEval(`async()=>{
        const before=window.__ollamaOldBanner.textContent;
        window.__ollamaResolveStatus({ok:true,status:200,json:async()=>({running:true,operations:[{state:'ready'}]})});
        await new Promise(resolve=>setTimeout(resolve,0));
        if(window.__ollamaOldBanner.textContent!==before)throw new Error('stale response changed detached status');
    }`)
}
