package ui

import (
	"archive/zip"
	"aurago/internal/qrcode"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// The browser loads the shipped dialog and editor modules; every QR request uses
// the real Go core. Authentication and permission coverage belongs to server tests.
func TestQRDialogWriterPixelBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	var mu sync.Mutex
	files := map[string][]byte{}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	jsonOut := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(value)
	}
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, map[string]any{"data": map[string]string{}}) })
	mux.HandleFunc("/api/qr/presets", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, qrcode.Presets()) })
	mux.HandleFunc("/api/qr/render", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		spec, err := qrcode.ParseConfig(r.Context(), body)
		if err == nil && spec.Content.Text == "old-response" {
			time.Sleep(400 * time.Millisecond)
		}
		var result qrcode.Result
		if err == nil {
			result, err = qrcode.Render(r.Context(), spec)
		}
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			jsonOut(w, map[string]string{"error": "Invalid QR", "code": "qr_invalid"})
			return
		}
		meta, _ := json.Marshal(result.Metadata)
		w.Header().Set("X-QR-Metadata", string(meta))
		w.Header().Set("X-QR-Width", fmt.Sprint(result.Metadata.Width))
		w.Header().Set("X-QR-Height", fmt.Sprint(result.Metadata.Height))
		w.Header().Set("X-QR-Verified", fmt.Sprint(result.Metadata.Verified))
		w.Header().Set("Content-Type", result.Metadata.MIME)
		w.Write(result.Data)
	})
	mux.HandleFunc("/api/qr/config", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		spec, err := qrcode.ParseConfig(r.Context(), body)
		if err != nil {
			w.WriteHeader(422)
			return
		}
		data, err := qrcode.ExportConfig(r.Context(), spec)
		if err != nil {
			w.WriteHeader(422)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
	mux.HandleFunc("/api/qr/scan", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		result, err := qrcode.Decode(r.Context(), body)
		if err != nil {
			w.WriteHeader(422)
			jsonOut(w, map[string]string{"code": "qr_decode"})
			return
		}
		jsonOut(w, result)
	})
	mux.HandleFunc("/api/qr/batch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Spec        qrcode.QRSpec `json:"spec"`
			CSV         string        `json:"csv"`
			ContentType string        `json:"content_type"`
			DryRun      bool          `json:"dry_run"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		var result qrcode.BatchResult
		var err error
		if body.DryRun {
			result, err = qrcode.ValidateBatch(r.Context(), []byte(body.CSV), body.Spec, qrcode.BatchOptions{ContentType: body.ContentType})
		} else {
			result, err = qrcode.Batch(r.Context(), []byte(body.CSV), body.Spec, qrcode.BatchOptions{ContentType: body.ContentType})
		}
		if err != nil {
			w.WriteHeader(422)
			jsonOut(w, map[string]string{"code": "qr_batch"})
			return
		}
		if body.DryRun {
			jsonOut(w, result)
		} else {
			w.Header().Set("Content-Type", "application/zip")
			w.Write(result.Data)
		}
	})
	mux.HandleFunc("/api/desktop/office/document", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := r.URL.Query().Get("path")
		if r.Method == "PUT" {
			files[path], _ = io.ReadAll(r.Body)
		}
		if files[path] == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", `"qr-test"`)
		if r.Method == "GET" {
			w.Write(files[path])
		} else {
			jsonOut(w, map[string]any{})
		}
	})
	mux.HandleFunc("/qr-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><style>
html,body{margin:0;height:100%;font:13px system-ui}body{--vd-text:#e4e8f0;--vd-theme-app-bg:#171c25;--vd-theme-panel-bg:#222935;--vd-theme-control-bg:#2b3543;--vd-theme-border:#ffffff30;--vd-theme-muted:#a5b3c9;--vd-accent:#8ebaff}body.light{--vd-text:#1e2a3b;--vd-theme-app-bg:#eff2f6;--vd-theme-panel-bg:#e4eaf1;--vd-theme-control-bg:#fff;--vd-theme-border:#52647b55;--vd-theme-muted:#4a5a70;--vd-accent:#276dc4}.host{height:100%;position:relative}#owner1,#owner2{height:1px}</style></head><body><div id="owner1"></div><div id="owner2"></div><div class="host" id="writer"></div><div class="host" id="pixel" hidden></div>
<script src="/js/desktop/core/module-loader.js"></script><script>
window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();window.t=(k,p)=>String(labels[k]||k).replace(/\{\{(\w+)\}\}/g,(_,v)=>p?.[v]??'');
window.waitFor=async fn=>{const end=Date.now()+15000;while(!fn()){if(Date.now()>end)throw Error('QR wait: '+document.querySelector('[data-status]')?.textContent+' '+(window.lastQRFailure||''));await new Promise(r=>setTimeout(r,25));}};
window.fill=(selector,value)=>{const el=document.querySelector(selector);if(!el)throw Error('Missing '+selector);if(el.type==='checkbox')el.checked=value;else el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));if(el.tagName==='SELECT'||el.type==='checkbox')el.dispatchEvent(new Event('change',{bubbles:true}));};
window.exports=[];HTMLAnchorElement.prototype.click=function(){window.exports.push({href:this.href,name:this.download});};
await AuraDesktopModules.loadAppAssets('qr');await AuraDesktopModules.loadAppAssets('writer');
const openQR=AuraQRDialog.open;AuraQRDialog.open=options=>openQR({...options,onInsert:options.onInsert&&async function(result){try{return await options.onInsert(result);}catch(error){window.lastQRFailure=error.message;throw error;}}});
window.ctx={t:window.t,api:async()=>({}),notify:()=>{},promptDialog:async(_,value)=>value,confirmDialog:async()=>true,setWindowMenus:()=>{},setWindowBeforeClose:()=>{},registerWindowCleanup:()=>{}};
WriterApp.render(document.querySelector('#writer'),'writer',ctx);
})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/qr-fixture").Timeout(180 * time.Second)
	defer page.Close()
	page.MustSetViewport(1280, 850, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>{await ready;await waitFor(()=>WriterApp.instances.get('writer')?.editor&&!document.querySelector('#writer [data-loading]').offsetHeight);}`)

	t.Run("dialog", func(t *testing.T) {
		page.MustEval(`async()=>{
const host=document.querySelector('#owner1');window.qr=AuraQRDialog.open({host,t,windowId:'dialog'});
await waitFor(()=>document.querySelector('[data-content-type]'));fill('[data-field="content.text"]','Grüße 東京 مرحبا 😀');
await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
document.querySelector('[data-tab=design]').click();fill('[data-full]',true);document.querySelector('[data-full]').dispatchEvent(new Event('change',{bubbles:true}));
fill('[data-field="frame.position"]','bottom');fill('[data-field="frame.text"]','Hallo AuraGo\nZweite Zeile');
await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
const before=JSON.stringify(qr.spec);fill('[data-full]',false);document.querySelector('[data-full]').dispatchEvent(new Event('change',{bubbles:true}));
if(JSON.stringify(qr.spec)!==before)throw Error('Simple view changed configuration');
fill('[data-full]',true);document.querySelector('[data-full]').dispatchEvent(new Event('change',{bubbles:true}));
document.querySelector('[data-pin="style.finder_color"]').click();document.querySelector('[data-tab=content]').click();
if(!document.querySelector('[data-field="style.finder_color"]'))throw Error('Pinned control missing');
document.querySelector('[data-tab=design]').click();fill('[data-field="style.quiet_zone"]',0);
await waitFor(()=>!document.querySelector('[data-action=download]').disabled);if(!document.querySelector('[data-diagnostics]').textContent)throw Error('Quiet zone warning missing');
fill('[data-field="style.quiet_zone"]',4);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
document.querySelector('[data-action=save_config]').click();await waitFor(()=>exports.length);const cfg=await(await fetch(exports.pop().href)).json();
if(cfg.content.text!=='Grüße 東京 مرحبا 😀'||!cfg.frame.text.includes('Zweite'))throw Error('Config export lost content');
for(const format of ['png','jpeg','svg','ascii','unicode','unicode_half','txt','markdown']){fill('[data-format]',format);document.querySelector('[data-action=download]').click();await waitFor(()=>exports.length);const item=exports.pop(),r=await fetch(item.href);if((await r.blob()).size<20)throw Error('Empty '+format);}
document.querySelector('[data-tab=content]').click();fill('[data-field="content.text"]','unchanged');await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
const realFetch=window.fetch;window.fetch=(url,init)=>realFetch(url,String(url).includes('/api/qr/render')?{...init,signal:undefined}:init);
fill('[data-field="content.text"]','old-response');await new Promise(r=>setTimeout(r,290));fill('[data-field="content.text"]','new-response');await waitFor(()=>!document.querySelector('[data-action=download]').disabled);await new Promise(r=>setTimeout(r,500));window.fetch=realFetch;
const image=await(await fetch(document.querySelector('[data-preview] img').src)).blob(),decoded=await(await fetch('/api/qr/scan',{method:'POST',headers:{'Content-Type':'image/png'},body:image})).json();
if(decoded.text!=='new-response')throw Error('Late preview replaced current content');
document.querySelector('[data-tab=scan]').click();const transfer=new DataTransfer();transfer.items.add(new File([image],'qr.png',{type:'image/png'}));document.querySelector('[data-drop]').dispatchEvent(new DragEvent('drop',{bubbles:true,dataTransfer:transfer}));
await waitFor(()=>document.querySelector('[data-scan-result]').textContent==='new-response');
if(document.querySelector('[data-scan-result] a'))throw Error('Scan executed a link');
document.querySelector('[data-action=use_result]').click();await waitFor(()=>document.querySelector('[data-field="content.text"]'));if(qr.spec.content.text!=='new-response')throw Error('Scanned content lost');
fill('[data-format]','png');document.querySelector('[data-tab=batch]').click();fill('[data-csv]','text,filename,caption\n"Hallo, Welt",a,"erste\nzweite"\n"東京",a,"Mehr"');document.querySelector('[data-action=validate_batch]').click();
await waitFor(()=>document.querySelector('.vd-qr-batch-table tr[data-error=true]'));if(!document.querySelector('[data-action=export_zip]').disabled)throw Error('Invalid batch export enabled');
fill('[data-csv]','text,filename,caption\n"Hallo, Welt",a,"erste\nzweite"\n"東京",b,"Mehr"');document.querySelector('[data-action=validate_batch]').click();
await waitFor(()=>!document.querySelector('[data-action=export_zip]').disabled);
if(document.querySelectorAll('.vd-qr-batch-table img').length!==2)throw Error('Batch thumbnails missing');
document.querySelector('[data-action=export_zip]').click();await waitFor(()=>exports.length);const zip=await(await fetch(exports.pop().href)).arrayBuffer();if(new Uint8Array(zip)[0]!==80)throw Error('ZIP missing');
document.querySelector('[data-tab=scan]').click();let cameraResolve;window.stopped=0;const media=new MediaStream();media.getTracks=()=>[{stop(){window.stopped++;}}];
Object.defineProperty(navigator.mediaDevices,'getUserMedia',{configurable:true,value:()=>new Promise(r=>cameraResolve=r)});document.querySelector('[data-action=camera]').click();qr.close();cameraResolve(media);await new Promise(r=>setTimeout(r,30));if(stopped!==1)throw Error('Late camera stream not stopped');
const one=AuraQRDialog.open({host,t,windowId:'one'}),two=AuraQRDialog.open({host:document.querySelector('#owner2'),t,windowId:'two'});if(document.querySelectorAll('dialog').length!==2)throw Error('Windows share dialog');one.close();if(document.querySelectorAll('dialog').length!==1)throw Error('Closing another owner closed active dialog');two.close();
if(localStorage.getItem('aurago.qr.view.v1').includes('new-response'))throw Error('Payload persisted');
}`)
	})
	t.Run("writer", func(t *testing.T) {
		encoded := page.MustEval(`async()=>{
const app=WriterApp.instances.get('writer'),editor=app.editor;editor.exec({type:'paste',text:'before after'});const match=editor.findMatches('after')[0];editor.selectMatch(match);
void app.act('qr');await waitFor(()=>document.querySelector('[data-content-type]'));fill('[data-field="content.text"]','Writer QR Grüße 東京');await waitFor(()=>!document.querySelector('dialog [data-action=insert]').disabled);
fill('[data-insert=width]',42);document.querySelector('dialog [data-action=insert]').click();await waitFor(()=>!document.querySelector('dialog'));
if(editor.surface.session.bodyText().replace(/\ufffc/g,'')!=='before after')throw Error('Selected text changed: '+JSON.stringify(editor.surface.session.bodyText()));
const bytes=await editor.save();editor.exec({type:'undo'});const undone=await editor.save();editor.exec({type:'redo'});await app.session.save();
const base64=b=>{let s='';for(const v of new Uint8Array(b))s+=String.fromCharCode(v);return btoa(s);};
return JSON.stringify({saved:base64(bytes),undone:base64(undone),path:app.path});
}`).Str()
		var result struct{ Saved, Undone, Path string }
		if err := json.Unmarshal([]byte(encoded), &result); err != nil {
			t.Fatal(err)
		}
		checkQRPackage(t, result.Saved, "word/media/", "Writer QR Grüße 東京", true)
		checkQRPackage(t, result.Undone, "word/media/", "", false)
		page.MustEval(`async path=>{WriterApp.dispose('writer');WriterApp.render(document.querySelector('#writer'),'writer',{...ctx,path});await waitFor(()=>WriterApp.instances.get('writer')?.editor&&!document.querySelector('#writer [data-loading]').offsetHeight);}`, result.Path)
		reopened := page.MustEval(`async()=>{const b=await WriterApp.instances.get('writer').editor.save();let s='';for(const v of new Uint8Array(b))s+=String.fromCharCode(v);return btoa(s);}`).Str()
		checkQRPackage(t, reopened, "word/media/", "Writer QR Grüße 東京", true)
	})
	t.Run("templates-and-lifecycle", func(t *testing.T) {
		page.MustEval(`async()=>{
const host=document.querySelector('#owner1');window.qr=AuraQRDialog.open({host,t,windowId:'templates'});await waitFor(()=>document.querySelector('[data-content-type]'));
const examples=[['url',{url:'https://example.invalid/qr'}],['email',{to:'one@example.invalid,two@example.invalid',subject:'Grüße',body:'Hello\nWorld',cc:'copy@example.invalid',bcc:'hidden@example.invalid'}],['telephone',{number:'03012345'}],['sms',{number:'+49123456789;ext=12',body:'Grüße'}],['wifi',{ssid:'AuraGo; Test',password:'sicher;123',security:'WPA',hidden:true}],['vcard',{version:'2.1',first_name:'Jörg',last_name:'Müller',phone_work:'+12025550101',phone_private:'+12025550102',phone_mobile:'+12025550103',note:'Hallo'}],['location',{latitude:52.52,longitude:13.405}],['calendar',{title:'Termin',start:'2026-10-27T10:00:00+01:00',end:'2026-10-27T11:00:00+01:00',timezone:'Europe/Berlin'}],['epc',{version:'002',name:'Alice',iban:'DE89370400440532013000',bic:'COBADEFFXXX',amount:'12.30'}]];
for(const [type,fields]of examples){fill('[data-content-type]',type);for(const [key,value]of Object.entries(fields))fill('[data-field="content.'+key+'"]',value);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);const b=await(await fetch(document.querySelector('[data-preview] img').src)).blob(),res=await(await fetch('/api/qr/scan',{method:'POST',headers:{'Content-Type':'image/png'},body:b})).json();if(res.type!==type)throw Error('Template '+type+' decoded as '+res.type);}
fill('[data-content-type]','calendar');if(!qr.spec.content.calendar.created_at)throw Error('Calendar creation timestamp missing');const created=qr.spec.content.calendar.created_at;document.querySelector('[data-tab=design]').click();fill('[data-preset]','soft');if(qr.spec.content.calendar.created_at!==created)throw Error('Style changed calendar timestamp');
const oldCalendar=qr.spec;delete oldCalendar.content.calendar.created_at;oldCalendar.content.calendar.title='Imported old calendar';const oldFile=new DataTransfer();oldFile.items.add(new File([JSON.stringify(oldCalendar)],'old-calendar.qr.json',{type:'application/json'}));const oldPicker=document.querySelector('[data-config-file]');oldPicker.files=oldFile.files;oldPicker.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>qr.spec.content.calendar.title==='Imported old calendar'&&!!qr.spec.content.calendar.created_at);const importedStamp=qr.spec.content.calendar.created_at;fill('[data-preset]','classic');if(qr.spec.content.calendar.created_at!==importedStamp)throw Error('Imported calendar timestamp changed');document.querySelector('[data-tab=content]').click();fill('[data-content-type]','epc');for(const[key,value]of Object.entries(examples.find(row=>row[0]==='epc')[1]))fill('[data-field="content.'+key+'"]',value);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
if(qr.spec.style.error_correction!=='M')throw Error('EPC default correction');document.querySelector('[data-tab=design]').click();fill('[data-preset]','forest');await waitFor(()=>!document.querySelector('[data-action=download]').disabled);if(qr.spec.style.error_correction!=='M'||qr.spec.content.type!=='epc')throw Error('Preset changed correction or content');
fill('[data-full]',true);fill('[data-field="style.size"]','');await new Promise(r=>setTimeout(r,400));if(!document.querySelector('[data-action=download]').disabled)throw Error('Empty numeric field exported stale QR');fill('[data-field="style.size"]',512);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
const current=JSON.stringify(qr.spec),bad=new DataTransfer();bad.items.add(new File([JSON.stringify({...qr.spec,unknown:true})],'bad.qr.json',{type:'application/json'}));const picker=document.querySelector('[data-config-file]');picker.files=bad.files;picker.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>document.querySelector('[data-status]').textContent.includes('version-1'));if(JSON.stringify(qr.spec)!==current)throw Error('Failed config import changed draft');
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{}});fill('[data-format]','png');document.querySelector('[data-action=copy]').click();await waitFor(()=>exports.length);if(!exports.pop().name.endsWith('.png'))throw Error('Clipboard fallback missing');
document.querySelector('[data-tab=scan]').click();Object.defineProperty(navigator.mediaDevices,'getUserMedia',{configurable:true,value:async()=>{throw new DOMException('Denied','NotAllowedError');}});document.querySelector('[data-action=camera]').click();await waitFor(()=>document.querySelector('[data-status]').textContent.includes('Camera access failed'));
let stops=0;const media=new MediaStream();media.getTracks=()=>[{stop(){stops++;}}];Object.defineProperty(navigator.mediaDevices,'getUserMedia',{configurable:true,value:async()=>media});document.querySelector('[data-camera]').play=async()=>{};document.querySelector('[data-action=camera]').click();await waitFor(()=>!document.querySelector('[data-action=camera_stop]').hidden);qr.close();if(stops!==1)throw Error('Active camera not stopped');
}`)
	})
	t.Run("resources-and-config", func(t *testing.T) {
		page.MustEval(`async()=>{
window.qr=AuraQRDialog.open({host:document.querySelector('#owner1'),t,windowId:'resources'});await waitFor(()=>document.querySelector('[data-content-type]'));fill('[data-field="content.text"]','Resource QR');document.querySelector('[data-tab=design]').click();fill('[data-full]',true);fill('[data-field="style.error_correction"]','H');fill('[data-color-picker="style.module_color"]','#17324d');
if(qr.spec.style.module_color!=='#17324d')throw Error('Native color picker did not update spec');fill('[data-field="logo.size_percent"]',10);
const logo=new DataTransfer();logo.items.add(new File(['<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><rect x="0" y="0" width="24" height="24" fill="#17324d"/></svg>'],'logo.svg',{type:'image/svg+xml'}));const upload=document.querySelector('[data-resource=logo]');upload.files=logo.files;upload.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>qr.spec.logo?.image);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
fill('[data-field="frame.position"]','left');fill('[data-field="frame.text"]','AuraGo\nShared QR');const canvas=document.createElement('canvas');canvas.width=canvas.height=24;const paint=canvas.getContext('2d');paint.fillStyle='#ffffff';paint.fillRect(0,0,24,24);const image=await new Promise(r=>canvas.toBlob(r,'image/png')),bg=new DataTransfer();bg.items.add(new File([image],'bg.png',{type:'image/png'}));const background=document.querySelector('[data-resource=background]');background.files=bg.files;background.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>qr.spec.frame?.background_image);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);
document.querySelector('[data-action=save_config]').click();await waitFor(()=>exports.length);const saved=await(await fetch(exports.pop().href)).json();if(saved.logo.image.mime!=='image/png'||!saved.frame.background_image.data)throw Error('Resources not normalized/embedded');
document.querySelector('[data-tab=content]').click();fill('[data-field="content.text"]','edited');document.querySelector('[data-tab=design]').click();const load=new DataTransfer();load.items.add(new File([JSON.stringify(saved)],'saved.qr.json',{type:'application/json'}));const picker=document.querySelector('[data-config-file]');picker.files=load.files;picker.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>qr.spec.content.text==='Resource QR');await waitFor(()=>!document.querySelector('[data-action=download]').disabled);if(qr.spec.logo.image.mime!=='image/png')throw Error('Normalized import lost logo');
const before=JSON.stringify(qr.spec),invalid=new DataTransfer();invalid.items.add(new File([JSON.stringify({...saved,output:{...saved.output,format:'invalid'}})],'bad-format.json',{type:'application/json'}));const currentPicker=document.querySelector('[data-config-file]');currentPicker.files=invalid.files;currentPicker.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>document.querySelector('[data-status]').textContent.includes('version-1'));if(JSON.stringify(qr.spec)!==before)throw Error('Unknown output format changed draft');
const raw=JSON.stringify(saved),invalidUTF8=new TextEncoder().encode(raw);invalidUTF8[raw.indexOf('Resource QR')]=255;
for(const[name,bytes]of [['duplicate-content',raw.replace('"content":','"content":{"type":"text","text":"first"},"content":')],['duplicate-type',raw.replace('"type":"text"','"type":"url","type":"text"')],['invalid-utf8',invalidUTF8]]){
fill('[data-field="style.module_color"]',qr.spec.style.module_color);await waitFor(()=>!document.querySelector('[data-action=download]').disabled);const draft=JSON.stringify(qr.spec),files=new DataTransfer();files.items.add(new File([bytes],name+'.qr.json',{type:'application/json'}));const input=document.querySelector('[data-config-file]');input.files=files.files;input.dispatchEvent(new Event('change',{bubbles:true}));await waitFor(()=>document.querySelector('[data-status]').dataset.error==='true');if(JSON.stringify(qr.spec)!==draft)throw Error(name+' import changed draft');
}
document.querySelector('[data-tab=batch]').click();for(const type of ['text','url','vcard']){fill('[data-batch-type]',type);document.querySelector('[data-action=csv_template_download]').click();await waitFor(()=>exports.length);const item=exports.pop(),csv=await(await fetch(item.href)).text();if(!csv.includes(type==='vcard'?'phone_work,phone_private,phone_mobile':type+',filename'))throw Error('CSV template '+type);}
document.querySelector('[data-tab=content]').click();fill('[data-field="content.text"]','x'.repeat(20000));await waitFor(()=>document.querySelector('[data-status]').dataset.error==='true');if(document.querySelector('[data-field="content.text"]').value.length!==20000||!document.querySelector('[data-action=download]').disabled)throw Error('Oversized payload truncated or published');
qr.close();
}`)
	})
	t.Run("appearance-and-locales", func(t *testing.T) {
		for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
			page.MustEval(`async lang=>{const labels=await(await fetch('/lang/desktop/'+lang+'.json')).json(),translate=(k,p)=>String(labels[k]||k).replace(/\{\{(\w+)\}\}/g,(_,v)=>p?.[v]??'');window.qr=AuraQRDialog.open({host:document.querySelector('#owner1'),windowId:'locale',t:translate});await waitFor(()=>document.querySelector('[data-content-type]'));fill('[data-field="content.text"]','AuraGo QR');await waitFor(()=>!document.querySelector('[data-action=download]').disabled);if(document.querySelector('.vd-qr-head h2').textContent!==labels['desktop.qr_title'])throw Error(lang+' title');qr.close();}`, lang)
		}
		for _, variant := range []struct {
			name  string
			width int
			light bool
		}{{"standard-wide", 1280, false}, {"fruity-narrow", 390, true}} {
			page.MustSetViewport(variant.width, 850, 1, false)
			page.MustEval(`async light=>{document.body.classList.toggle('light',light);window.qr=AuraQRDialog.open({host:document.querySelector('#owner1'),windowId:'appearance',t,insertion:{unit:'mm',width:40},onInsert:async()=>{}});await waitFor(()=>document.querySelector('[data-content-type]'));fill('[data-field="content.text"]','AuraGo · QR');await waitFor(()=>!document.querySelector('[data-action=download]').disabled);const box=document.querySelector('dialog').getBoundingClientRect();if(box.left<0||box.right>innerWidth||box.top<0||box.bottom>innerHeight)throw Error('Dialog exceeds viewport');if(document.querySelector('dialog').scrollWidth>document.querySelector('dialog').clientWidth)throw Error('Horizontal overflow');}`, variant.light)
			if err := os.MkdirAll("../reports/qr", 0700); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot("../reports/qr/" + variant.name + ".png")
			page.MustEval(`()=>qr.close()`)
		}
		page.MustSetViewport(1280, 850, 1, false)
	})
	t.Run("pixel", func(t *testing.T) {
		encoded := page.MustEval(`async()=>{
document.querySelector('#writer').hidden=true;document.querySelector('#pixel').hidden=false;await AuraDesktopModules.loadAppAssets('pixel');AuraPixelApp.registerModule('qr-browser-runtime',r=>window.rt=r);PixelApp.render(document.querySelector('#pixel'),'pixel',ctx);rt.newBlankCanvas(600,500);
rt.selection={type:'rect',x:2,y:3,w:20,h:20};const selected=rt.selection,old=rt.layers[0],before=rt.history.length;void rt.insertQR();await waitFor(()=>document.querySelector('[data-content-type]'));fill('[data-field="content.text"]','Pixel QR Grüße 東京');await waitFor(()=>!document.querySelector('dialog [data-action=insert]').disabled);fill('[data-insert=width]',300);document.querySelector('dialog [data-action=insert]').click();await waitFor(()=>!document.querySelector('dialog'));
if(rt.layers.length!==2||rt.layers[0]!==old||rt.selection!==selected||rt.history.length!==before+1)throw Error('Pixel insert changed layers/selection/history');
rt.undo();if(rt.layers.length!==1)throw Error('QR undo');rt.redo();if(rt.layers.length!==2)throw Error('QR redo');const bytes=await(await rt.oraProjectBlob()).arrayBuffer(),project=await AuraPixelApp.ora.read(bytes);rt.applyOraProject(project);if(rt.layers.length!==2)throw Error('ORA reopen lost layers');
const scan=await new Promise(r=>rt.layers[1].canvas.toBlob(r,'image/png')),res=await(await fetch('/api/qr/scan',{method:'POST',headers:{'Content-Type':'image/png'},body:scan})).json();if(res.text!=='Pixel QR Grüße 東京')throw Error('Reopened ORA QR not decodable');
let s='';for(const v of new Uint8Array(bytes))s+=String.fromCharCode(v);return btoa(s);
}`).Str()
		checkQRPackage(t, encoded, "data/", "Pixel QR Grüße 東京", true)
	})
}

func checkQRPackage(t *testing.T, encoded, prefix, payload string, expected bool) {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range archive.File {
		if !strings.HasPrefix(file.Name, prefix) || !strings.HasSuffix(file.Name, ".png") {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		result, err := qrcode.Decode(t.Context(), raw)
		if err == nil && result.Text == payload {
			found = true
		}
		if !expected {
			t.Fatalf("undo retained image %s", file.Name)
		}
	}
	if expected && !found {
		t.Fatal("saved package contains no decodable QR image")
	}
	if expected && prefix == "word/media/" {
		for _, file := range archive.File {
			if file.Name != "word/document.xml" {
				continue
			}
			reader, _ := file.Open()
			xml, _ := io.ReadAll(reader)
			reader.Close()
			if !bytes.Contains(xml, []byte(`cx="1512000"`)) || !bytes.Contains(xml, []byte(`cy="1512000"`)) {
				t.Fatal("Writer did not preserve the requested 42 mm insertion size")
			}
		}
	}
}
