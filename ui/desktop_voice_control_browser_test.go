package ui

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestDesktopVoiceControlOpensAppsAndFilesBrowser(t *testing.T) {
	var fileReads, fileWrites atomic.Int32
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/file": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				fileWrites.Add(1)
				http.Error(w, "opening a file must not write it", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("path") != "Documents/Notizen.txt" {
				http.Error(w, "wrong document", http.StatusNotFound)
				return
			}
			fileReads.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `"fixture-version"`)
			fmt.Fprint(w, `{"content":"Vorhandenes Dokument: Grüße.","entry":{"path":"Documents/Notizen.txt","name":"Notizen.txt"},"version":{"etag":"\"fixture-version\""}}`)
		}),
		"/api/desktop/integrations/realtime-speech/actions": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"virtual_desktop_event\",\"payload\":{\"id\":\"voice-calculator\",\"type\":\"open_app\",\"payload\":{\"app_id\":\"calculator\"}}}\n\n")
			fmt.Fprint(w, "data: {\"type\":\"virtual_desktop_event\",\"payload\":{\"id\":\"voice-document\",\"type\":\"open_app\",\"payload\":{\"app_id\":\"editor\",\"path\":\"Documents/Notizen.txt\"}}}\n\n")
			fmt.Fprint(w, "data: {\"event\":\"final_response\",\"detail\":\"Geöffnet.\"}\n\ndata: [DONE]\n\n")
		}),
	})
	// This fixture has no SSE transport and a disconnected WebSocket: only the
	// real voice response stream can deliver the two launches to the real shell.
	page.MustEval(`async()=>{
        for(const src of ['/js/realtime-speech/provider-common.js','/js/realtime-speech/core.js']){
            await new Promise((resolve,reject)=>{const script=document.createElement('script');script.src=src;script.onload=resolve;script.onerror=reject;document.body.append(script)});
        }
        AuraRealtimeSpeech.surface='desktop';
        AuraRealtimeSpeech.chatSessionId='virtual-desktop';
        window.voiceResult=await AuraRealtimeSpeech.executeAction('Öffne den Taschenrechner und Documents/Notizen.txt im Editor');
    }`)
	if err := page.Timeout(8 * time.Second).Wait(rod.Eval(`()=>document.querySelector('.vd-window[data-app-id="calculator"] [data-result]') && document.querySelector('.vd-window[data-app-id="editor"] textarea')?.value==='Vorhandenes Dokument: Grüße.'`)); err != nil {
		t.Fatalf("%v: %s", err, page.MustEval(`()=>JSON.stringify({result:voiceResult,errors:fixtureErrors,windows:[...auditTest.state.windows.values()].map(w=>({app:w.appId,context:w.context})),editors:[...document.querySelectorAll('textarea')].map(e=>e.value)})`).Str())
	}
	if !page.MustEval(`()=>voiceResult.status==='completed' && auditTest.state.windows.size===2`).Bool() {
		t.Fatal("voice action did not open both apps", page.MustEval(`()=>JSON.stringify(voiceResult)`).Str())
	}
	// A second delivery from the global channel must not reload the file or open
	// another window. Keep an unsaved edit in the existing editor to detect it.
	page.MustEval(`()=>{
        document.querySelector('.vd-window[data-app-id="editor"] textarea').value='Ungespeicherter Entwurf';
        window.dispatchEvent(new CustomEvent('aurago:desktop-event',{detail:{id:'voice-document',type:'open_app',payload:{app_id:'editor',path:'Documents/Notizen.txt'}}}));
    }`)
	if !page.MustEval(`()=>auditTest.state.windows.size===2 && document.querySelector('.vd-window[data-app-id="editor"] textarea').value==='Ungespeicherter Entwurf'`).Bool() {
		t.Fatal("duplicate delivery replaced the open document")
	}
	if fileReads.Load() != 1 || fileWrites.Load() != 0 {
		t.Fatalf("file reads=%d writes=%d", fileReads.Load(), fileWrites.Load())
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
