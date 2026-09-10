package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestRealtimeSpeechProfileSelectionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><div id="panel"></div><script>
const params=new URLSearchParams(location.search);
if(params.has('blocked'))Object.defineProperty(window,'localStorage',{get(){throw Error('Storage blocked');}});
window.runtime=window.AuraRealtimeSpeech=Object.assign(new EventTarget(),{
    state:'idle',sessionId:'',profile:null,
    config:{default_profile:'primary',profiles:[
        {id:'primary',name:'Primary',provider:'openai',enabled:true,api_key_set:true},
        {id:'local',name:'Local',provider:'speech_lab',enabled:true},
        {id:'disabled',name:'Disabled',provider:'speech_lab',enabled:false},
        {id:'no-key',name:'No key',provider:'openai',enabled:true}
    ]},
    async initialize(){},
    async start({profileId}){this.profile=this.config.profiles.find(p=>p.id===profileId);this.sessionId='test';this.state='listening';this.dispatchEvent(new Event('state'));},
    async stop(){this.profile=null;this.sessionId='';this.state='idle';this.dispatchEvent(new Event('state'));}
});
</script><script src="/js/realtime-speech/panel.js"></script><script>
window.root=document.getElementById('panel');
window.remount=()=>AuraRealtimeSpeechUI.mount(root,{surface:params.get('surface')||'webchat'});
window.selected=()=>root.querySelector('[data-realtime-profile]').value;
remount();
</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture")
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`()=>{if(selected()!=='primary')throw Error('Initial default missing');}`)
	page.MustElement("[data-realtime-profile]").MustSelect("Local")
	page.MustElement("[data-realtime-start]").MustClick()
	page.MustEval(`async()=>{if(runtime.profile.id!=='local')throw Error('Started wrong profile');await runtime.stop();remount();if(selected()!=='local')throw Error('Last active profile lost on reopen');}`)
	page.MustReload().MustWaitLoad()
	page.MustEval(`()=>{if(selected()!=='local'||runtime.sessionId)throw Error('Reload lost selection or started speech');}`)
	page.MustNavigate(srv.URL + "/fixture?surface=desktop").MustWaitLoad()
	page.MustEval(`()=>{
        if(selected()!=='local')throw Error('Desktop did not reuse the last profile');
        runtime.config.profiles.find(p=>p.id==='local').enabled=false;AuraRealtimeSpeechUI.refresh();
        if(selected()!=='primary')throw Error('Disabled profile remained selected');
        runtime.config.default_profile='deleted';runtime.config.profiles=runtime.config.profiles.filter(p=>p.id!=='local');AuraRealtimeSpeechUI.refresh();
        if(selected()!=='primary')throw Error('Invalid preference/default must fall back to an available profile');
        runtime.config.profiles.forEach(p=>p.enabled=false);AuraRealtimeSpeechUI.refresh();
        if(selected()!==''||!root.querySelector('[data-realtime-start]').disabled)throw Error('Unavailable profiles remain startable');
    }`)
	page.MustNavigate(srv.URL + "/fixture?blocked=1").MustWaitLoad()
	page.MustElement("[data-realtime-profile]").MustSelect("Local")
	page.MustEval(`()=>{remount();if(selected()!=='local')throw Error('Blocked storage broke the in-memory selection');}`)
}
