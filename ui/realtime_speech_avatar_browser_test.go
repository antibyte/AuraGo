package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestRealtimeSpeechAvatarFirstOpenBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/img/personas/animated/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(750 * time.Millisecond)
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/personalities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"active":"neutral","personalities":[{"name":"neutral","core":true}]}`)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="stylesheet" href="/css/realtime-speech.css"></head>
<body style="background:#17202a"><div id="panel"></div>
<script>
window.errors=[];addEventListener('error',e=>errors.push(e.message));
window.BUILD_VERSION='avatar-test';
window.surface=new URLSearchParams(location.search).get('surface');
if(surface==='webchat')window._activePersonaIconKey='friend';
window.AuraRealtimeSpeech=Object.assign(new EventTarget(),{state:'idle',initialize:async()=>{}});
</script><script src="/js/realtime-speech/avatar.js"></script><script src="/js/realtime-speech/panel.js"></script>
<script>
window.root=document.getElementById('panel');
window.unmount=AuraRealtimeSpeechUI.mount(root,{surface,compact:true,visible:surface!=='webchat'});
if(surface==='webchat')AuraRealtimeSpeechUI.setVisible(root,true);
</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, surface := range []string{"webchat", "desktop"} {
		t.Run(surface, func(t *testing.T) {
			page := browser.MustPage()
			defer page.Close()
			if err := (proto.NetworkSetCacheDisabled{CacheDisabled: true}).Call(page); err != nil {
				t.Fatal(err)
			}
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			page.MustNavigate(srv.URL + "/fixture?surface=" + surface).MustWaitLoad()
			page.Timeout(30 * time.Second).MustWait(`()=>document.querySelector('[data-realtime-avatar]')?.dataset.animated==='true'`)
			page.MustEval(`async()=>{await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);}`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				os.MkdirAll(dir, 0755)
				page.MustScreenshot(filepath.Join(dir, surface+"-first-open.png"))
			}
			page.MustEval(`()=>{
                const key=surface==='webchat'?'friend':'neutral',avatar=root.querySelector('[data-realtime-avatar]');
                const requests=performance.getEntriesByType('resource').map(entry=>new URL(entry.name).pathname);
                const portraits=requests.filter(path=>path.startsWith('/img/personas/')&&path.endsWith('.png'));
                if(!portraits.length||portraits.some(path=>path!=='/img/personas/'+key+'.png'))throw Error('Unselected loading persona: '+portraits.join(','));
                const animations=requests.filter(path=>path.endsWith('.riv'));
                if(animations.length!==1||animations[0]!=='/img/personas/animated/'+key+'.riv')throw Error('Unselected animation loaded');
                if(avatar.dataset.persona!==key||AuraRealtimeSpeech.sessionId)throw Error('Opening changed persona or started speech');
                const sample=document.createElement('canvas');sample.width=sample.height=140;
                const ctx=sample.getContext('2d');ctx.drawImage(avatar.querySelector('canvas'),0,0,140,140);
                if(!ctx.getImageData(0,0,140,140).data.some((value,i)=>i%4===3&&value>0))throw Error('Animated frame is empty');
                if(errors.length)throw Error(errors.join(','));
                unmount();if(root.childElementCount)throw Error('Avatar was not disposed');
            }`)
		})
	}
}
