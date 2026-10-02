package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// openRealtimeHeadsetFixture serves the UI assets plus one fixture page and
// returns a page with autoplay allowed so AudioContexts run headless.
func openRealtimeHeadsetFixture(t *testing.T, body string) *rod.Page {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body>`+body+`</body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("autoplay-policy", "no-user-gesture-required")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	t.Cleanup(launch.Cleanup)
	t.Cleanup(func() { _ = browser.Close() })
	page := browser.MustPage(srv.URL + "/fixture")
	page.MustWaitLoad()
	return page
}

func TestRealtimeSpeechAudioGateInputBrowser(t *testing.T) {
	page := openRealtimeHeadsetFixture(t, `<script>
window.AuraSileroVAD={SileroVAD:class{async load(){} async isSpeech(frame){return frame[0]>0.5;} reset(){}}};
window.gumCalls=0;
navigator.mediaDevices.getUserMedia=async()=>{
    window.gumCalls++;
    const context=new AudioContext();const destination=context.createMediaStreamDestination();
    const oscillator=context.createOscillator();oscillator.connect(destination);oscillator.start();
    return destination.stream;
};
</script><script src="/js/realtime-speech/audio-engine.js"></script>`)
	page.MustEval(`async()=>{
        const gate=new AuraRealtimeAudio.RealtimeAudioGate();
        let starts=0;gate.addEventListener('speechstart',()=>starts++);
        await gate.start({input:'external'});
        if(gumCalls!==0||gate.inputMode!=='external')throw Error('external start must not open the browser microphone');
        const loud=new Float32Array(512).fill(0.9);
        for(let i=0;i<4;i++)gate.pushExternalFrame(loud);
        await new Promise(r=>setTimeout(r,100));
        if(starts!==1)throw Error('external frames did not reach the VAD: '+starts);
        await gate.useBrowserInput();
        if(gumCalls!==1||gate.inputMode!=='browser')throw Error('browser input did not open the microphone');
        gate.pushExternalFrame(loud);
        await gate.useExternalInput();
        if(gate.inputMode!=='external'||gate.stream)throw Error('switching back must release the browser microphone');
        await gate.stop();
    }`)
}

func TestRealtimeSpeechAudioOutputBrowser(t *testing.T) {
	page := openRealtimeHeadsetFixture(t, `<script src="/js/realtime-speech/provider-common.js"></script>`)
	page.MustEval(`async()=>{
        const output=window.AuraRealtimeAudioOutput;
        const context=new AudioContext();
        await context.resume();
        const destination=output.destination(context,0);
        if(output.destination(context,0)!==destination)throw Error('one destination per context and stream');
        const oscillator=context.createOscillator();oscillator.connect(destination);oscillator.start();
        const sent=[];const flushed=[];
        output.setBridge({sendOutput(stream,samples){sent.push([stream,samples.length,Math.max(...samples.map(Math.abs))]);},flush(stream){flushed.push(stream);}});
        if(output.mode!=='bridge')throw Error('bridge mode expected');
        await new Promise(r=>setTimeout(r,400));
        if(!sent.length||sent.some(([stream,length])=>stream!==0||length!==480)||!sent.some(item=>item[2]>0.1))throw Error('tap did not forward 24 kHz chunks: '+JSON.stringify(sent.slice(0,3)));
        output.flush(0);
        if(flushed.join()!=='0')throw Error('flush not forwarded');
        output.setBridge(null);
        const count=sent.length;
        await new Promise(r=>setTimeout(r,200));
        if(output.mode!=='local'||sent.length>count+1)throw Error('local mode must stop forwarding');
        output.release(context);
        await context.close();
    }`)
}

func TestRealtimeSpeechHeadsetBridgeBrowser(t *testing.T) {
	page := openRealtimeHeadsetFixture(t, `<script src="/js/realtime-speech/headset-bridge.js"></script><script>
window.sockets=[];
class FakeSocket extends EventTarget{
    constructor(url){super();this.url=url;this.readyState=1;this.sent=[];sockets.push(this);}
    send(data){this.sent.push(data);}
    close(){this.readyState=3;this.dispatchEvent(new Event('close'));}
    receive(data){this.dispatchEvent(new MessageEvent('message',{data}));}
}
window.FakeSocket=FakeSocket;
</script>`)
	page.MustEval(`async()=>{
        const {HeadsetBridge}=AuraRealtimeHeadsetBridge;
        const bridge=new HeadsetBridge({sessionId:'rts-1',clientId:'browser',device:'AA:BB:CC:DD:EE:FF',WebSocketClass:FakeSocket,retryDelays:[20,20]});
        const states=[];const frames=[];
        bridge.addEventListener('device',e=>states.push(e.detail));
        bridge.addEventListener('frame',e=>frames.push(e.detail.frame));
        bridge.open();
        const socket=sockets[0];
        if(!socket.url.includes('/api/realtime-speech/headset?session=rts-1&client=browser&device=AA%3ABB%3ACC%3ADD%3AEE%3AFF'))throw Error('url '+socket.url);
        socket.receive(JSON.stringify({type:'ready',input_rate:16000,output_rate:24000}));
        const pcm=new Int16Array(512).fill(16384).buffer;
        socket.receive(pcm);
        if(frames.length)throw Error('frames before device_ready must be dropped');
        socket.receive(JSON.stringify({type:'device_ready'}));
        socket.receive(pcm);
        if(frames.length!==1||frames[0].length!==512||Math.abs(frames[0][0]-0.5)>0.001)throw Error('frame conversion');
        bridge.sendOutput(0,new Float32Array(480));
        if(socket.sent.length)throw Error('silence must not be sent');
        bridge.sendOutput(1,new Float32Array(480).fill(0.5));
        const payload=new Uint8Array(socket.sent[0]);
        if(payload.length!==961||payload[0]!==1||new DataView(payload.buffer).getInt16(1,true)!==16383)throw Error('output encoding');
        bridge.flush(0);
        if(socket.sent[1]!=='{"type":"flush","stream":0}')throw Error('flush message');
        socket.receive(JSON.stringify({type:'device_lost'}));
        if(states.length!==2||states[0].ready!==true||states[1].ready!==false||states[1].wasReady!==true)throw Error('states '+JSON.stringify(states));
        socket.close();
        await new Promise(r=>setTimeout(r,60));
        if(sockets.length!==2)throw Error('bridge did not retry after a drop');
        sockets[1].receive(JSON.stringify({type:'error',code:'headset_unknown'}));
        sockets[1].close();
        await new Promise(r=>setTimeout(r,60));
        if(sockets.length!==2)throw Error('an unknown headset must not be retried');
        bridge.close();
    }`)
}

func TestRealtimeSpeechAudioPickerBrowser(t *testing.T) {
	page := openRealtimeHeadsetFixture(t, `<div id="panel"></div><script>
window.audioList={devices:[],reason:''};
window.AuraRealtimeHeadsetBridge={listDevices:async()=>window.audioList};
window.runtime=window.AuraRealtimeSpeech=Object.assign(new EventTarget(),{
    state:'idle',sessionId:'',profile:null,audioDevice:'',headsetNotice:'',started:null,switched:[],
    config:{default_profile:'primary',profiles:[{id:'primary',name:'Primary',provider:'openai',enabled:true,api_key_set:true}]},
    async initialize(){},
    headsetStatusText(){return this.headsetNotice;},
    async start(options){this.started=options;this.audioDevice=options.audioDevice||'';this.profile=this.config.profiles[0];this.sessionId='s';this.state='listening';this.dispatchEvent(new Event('state'));},
    async stop(){this.sessionId='';this.state='idle';this.dispatchEvent(new Event('state'));},
    async setAudioDevice(id){this.switched.push(id);this.audioDevice=id;}
});
</script><script src="/js/realtime-speech/panel.js"></script><script>
window.root=document.getElementById('panel');
window.remount=()=>AuraRealtimeSpeechUI.mount(root,{surface:'webchat'});
window.field=()=>root.querySelector('[data-realtime-audio-field]');
window.select=()=>root.querySelector('[data-realtime-audio]');
window.settle=()=>new Promise(r=>setTimeout(r,50));
remount();
</script>`)
	page.MustEval(`async()=>{
        await settle();
        if(!field().hidden)throw Error('no headsets and nothing stored: the field must stay hidden');
        audioList={devices:[{id:'AA:BB:CC:DD:EE:FF',name:'Earbuds',connected:true,busy:false},{id:'AA:BB:CC:DD:EE:01',name:'Office',connected:false,busy:false}],reason:''};
        remount();await settle();
        const labels=[...select().options].map(o=>o.textContent);
        if(field().hidden||labels[0]!=='This device'||labels[1]!=='Earbuds'||labels[2]!=='Office (not connected)')throw Error('options '+JSON.stringify(labels));
        select().value='AA:BB:CC:DD:EE:FF';select().dispatchEvent(new Event('change'));
        if(localStorage.getItem('aurago.realtimeSpeech.audioDevice.v1')!=='AA:BB:CC:DD:EE:FF')throw Error('choice not stored');
        root.querySelector('[data-realtime-start]').click();await settle();
        if(runtime.started.audioDevice!=='AA:BB:CC:DD:EE:FF')throw Error('start did not pass the headset');
        select().value='';select().dispatchEvent(new Event('change'));await settle();
        if(runtime.switched.length!==1||runtime.switched[0]!=='')throw Error('switch during the session missing: '+JSON.stringify(runtime.switched));
        runtime.headsetNotice='Headset connected.';runtime.dispatchEvent(new Event('headset'));
        const status=root.querySelector('[data-realtime-audio-status]');
        if(status.hidden||status.textContent!=='Headset connected.')throw Error('status line');
        await runtime.stop();
        localStorage.setItem('aurago.realtimeSpeech.audioDevice.v1','AA:BB:CC:DD:EE:01');
    }`)
	// After a reload the fixture lists no headsets again.
	page.MustReload().MustWaitLoad()
	page.MustEval(`async()=>{
        await settle();
        const labels=[...select().options].map(o=>o.textContent);
        if(field().hidden||select().value!=='AA:BB:CC:DD:EE:01'||labels[1]!=='AA:BB:CC:DD:EE:01 (not connected)')throw Error('a stored but unlisted headset must stay selectable: '+JSON.stringify(labels));
    }`)
}

func TestRealtimeSpeechHeadsetSwitchingBrowser(t *testing.T) {
	page := openRealtimeHeadsetFixture(t, `<script src="/js/realtime-speech/provider-common.js"></script>
<script src="/js/realtime-speech/headset-bridge.js"></script><script src="/js/realtime-speech/core.js"></script>`)
	page.MustEval(`async()=>{
        const runtime=window.AuraRealtimeSpeech;
        const calls=[];
        runtime.audioGate={inputMode:'external',async useExternalInput(){calls.push('external');this.inputMode='external';},async useBrowserInput(){calls.push('browser');this.inputMode='browser';},pushExternalFrame(){calls.push('frame');}};
        const bridge=Object.assign(new EventTarget(),{closed:false,sendOutput(){},flush(){},close(){this.closed=true;}});
        runtime.sessionId='rts-1';runtime.audioDevice='AA:BB:CC:DD:EE:FF';runtime.headsetBridge=bridge;runtime.headsetPending=true;
        await runtime.applyHeadsetState({ready:true});
        if(calls.join()!=='external'||AuraRealtimeAudioOutput.mode!=='bridge'||!runtime.headsetActive||runtime.headsetNotice!=='Headset connected.')throw Error('ready '+calls.join()+' '+runtime.headsetNotice);
        await runtime.applyHeadsetState({ready:false,wasReady:true,reason:'device'});
        if(calls.join()!=='external,browser'||AuraRealtimeAudioOutput.mode!=='local'||runtime.headsetNotice!=='Headset disconnected – using this device.')throw Error('lost '+runtime.headsetNotice);
        await runtime.applyHeadsetState({ready:false,reason:'error',error:'headset_busy'});
        if(runtime.headsetStatusText()!=='The headset is being used by another browser.')throw Error('busy '+runtime.headsetStatusText());
        await runtime.setAudioDevice('');
        if(!bridge.closed||runtime.headsetBridge||runtime.audioDevice!=='')throw Error('switching to this device must close the bridge');
    }`)
}
