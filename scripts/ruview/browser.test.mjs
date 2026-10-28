import {createServer} from 'node:http';
import {readFile,mkdir} from 'node:fs/promises';
import path from 'node:path';
import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {randomBytes,createHash} from 'node:crypto';
import {checkControls} from './control-browser.mjs';
import {describedText,checkDevices,checkPolicy,checkStatusStates,checkStateFixtures,checkCommandBar,checkFocusKept,checkLiveRegion,checkImageDenied,checkAllowChanges,openView,openLab,resolved,waitState} from './ui-browser.mjs';
const require=createRequire(import.meta.url);
const {chromium}=process.env.AURAGO_PLAYWRIGHT?require(process.env.AURAGO_PLAYWRIGHT):require('playwright');
const locales=['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
let writes=0,readonly=false,offline=false,mode='external',source='esp32',failure='',statusError='',firmwareError=false,sourceState='disconnected',sourceProbeFailed=false,configRequests=0;
const configPuts=[];
let configPutStatus=0;
const until=async(done,ms=5000)=>{for(let waited=0;waited<ms&&!done();waited+=50)await new Promise(resolve=>setTimeout(resolve,50));assert.ok(done(),'condition not reached');};
const nodeFixture={reads:0,statusReads:0,active:0,max:0,fail:false,statusFail:false,hold:null,offline:false,unavailable:false,liveState:'',disabled:false,statusExtra:null,scope:'fixture'};
// What the checks in ui-browser.mjs may steer on the fixture server.
const ctl={configPuts,
 get mode(){return mode},set mode(value){mode=value},get source(){return source},set source(value){source=value},
 get readonly(){return readonly},set readonly(value){readonly=value},get putStatus(){return configPutStatus},set putStatus(value){configPutStatus=value},get offline(){return offline},set offline(value){offline=value}};
const sensingSockets=new Set(),poseSockets=new Set(),poseRequests=[],healthRequests=new Set();
function sendFrames(sockets,data){
 const body=Buffer.from(JSON.stringify(data));assert.ok(body.length<65536);
 const header=body.length<126?Buffer.from([0x81,body.length]):Buffer.from([0x81,126,0,0]);
 if(body.length>=126)header.writeUInt16BE(body.length,2);
 for(const socket of sockets)socket.write(Buffer.concat([header,body]));
}
const sendSensing=data=>sendFrames(sensingSockets,data);
// The status the backend sends (design 8.1): derived fields included, merged with the check's override.
function statusBody(){
 const live=!offline&&!nodeFixture.disabled;
 return {enabled:!nodeFixture.disabled,readonly,mode,source,runtime:mode==='external'?'external':'running',phase:'',phase_since:null,
  ready:live,fresh:live,simulated:false,data_state:nodeFixture.disabled?'disabled':offline?'disconnected':'live_unverified',waiting_for_frames:false,
  last_frame_age_ms:live?800:null,signal:'CSI',observed_at:new Date().toISOString(),last_fresh_at:live?new Date().toISOString():null,
  error:statusError||undefined,error_code:statusError?'unreachable':'',error_detail:statusError,stop_reason:'',stopped_at:null,hints:[],...nodeFixture.statusExtra};
}
const server=createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://localhost'),p=url.pathname;
  if(p.includes('/api/config'))configRequests++;
  if(p.startsWith('/api/')&&!p.includes('/ui/')){
   res.setHeader('Content-Type','application/json');if(req.method==='POST')writes++;
   if(p==='/api/config'&&req.method==='PUT'){let body='';for await(const chunk of req)body+=chunk;configPuts.push({body,type:req.headers['content-type']});if(configPutStatus){res.statusCode=configPutStatus;return res.end(JSON.stringify({error:'admin access required'}));}readonly=false;return res.end(JSON.stringify({status:'ok'}));}
   if(p.endsWith('/ruview/nodes')){
    nodeFixture.reads++;nodeFixture.active++;nodeFixture.max=Math.max(nodeFixture.max,nodeFixture.active);
    try {
     if(nodeFixture.hold)await nodeFixture.hold;
     if(nodeFixture.fail){res.statusCode=503;return res.end('{}');}
     // A disabled integration refuses the board list, as the real handler does.
     if(nodeFixture.disabled){res.statusCode=403;return res.end(JSON.stringify({error:'RuView is disabled'}));}
     // Mirrors the backend: live_state says why the live list is absent, available is true exactly for ok.
     const live=nodeFixture.liveState||(nodeFixture.unavailable?'unreachable':'ok');
     return res.end(JSON.stringify({scope:nodeFixture.scope,available:live==='ok',live_state:live,nodes:nodeFixture.offline||live!=='ok'?[{node_id:7,csi_status:'offline',csi_last_seen_ms:null,csi_sequence:null,rssi_dbm:null}]:[{node_id:7,csi_status:'active',csi_last_seen_ms:42,csi_sequence:3,rssi_dbm:-51}]}));
    }finally{nodeFixture.active--;}
   }
   if(p.includes('/proxy/')){
    const upstream=p.split('/proxy')[1];
    if(upstream.startsWith('/health/')){
     healthRequests.add(upstream);
     if(!['/health/live','/health/health','/health/ready','/health/version','/health/metrics'].includes(upstream)){res.statusCode=404;return res.end('{}');}
     return res.end(JSON.stringify({status:'healthy',source,source_state:sourceState,components:{}}));
    }
    if(upstream==='/api/v1/status'){
     if(sourceProbeFailed){res.statusCode=503;return res.end('{}');}
     return res.end(JSON.stringify({source,source_state:sourceState}));
    }
    if(upstream==='/api/v1/nodes'){
     nodeFixture.reads++;nodeFixture.active++;nodeFixture.max=Math.max(nodeFixture.max,nodeFixture.active);
     try {if(nodeFixture.hold)await nodeFixture.hold;if(nodeFixture.fail){res.statusCode=503;return res.end('{}');}return res.end(JSON.stringify({nodes:[{node_id:7,csi_status:'active',csi_last_seen_ms:42,csi_sequence:3,rssi_dbm:-51}]}));}finally{nodeFixture.active--;}
    }
    if(['/api/v1/recording/stop','/api/v1/train/stop'].includes(upstream)&&req.method==='POST')return res.end(JSON.stringify({success:true,status:'stopping'}));
    res.statusCode=404;return res.end('{}');
   }
   if(failure&&req.method==='POST'){res.statusCode=502;return res.end(failure==='html'?'<html>upstream unavailable</html>':JSON.stringify({[failure]:'RuView image pull returned HTTP 403: registry denied <img src=x onerror="window.ruviewInjected=1">'}));}
   if(firmwareError&&p.endsWith('/firmware')){res.statusCode=503;return res.end(JSON.stringify({error:'Firmware catalog unavailable'}));}
   if(p.endsWith('/firmware'))return res.end(await readFile('internal/ruview/artifacts.json'));
   if(p.endsWith('/capabilities'))return res.end(JSON.stringify({unavailable:['model_load','model_unload','lora_activate','camera','timed_recording']}));
   if(p.endsWith('/status')){nodeFixture.statusReads++;if(nodeFixture.statusFail){res.statusCode=503;return res.end('{}');}}
   return res.end(JSON.stringify(statusBody()));
  }
  if(p==='/'){
   const lang=locales.includes(url.searchParams.get('lang'))?url.searchParams.get('lang'):'de';
   const locale=JSON.parse(await readFile(`ui/lang/ruview/${lang}.json`));
   const theme=url.searchParams.get('theme')==='fruity'?'fruity':'standard',tone=url.searchParams.get('mode')==='dark'?'dark':'light';
   res.setHeader('Content-Type','text/html; charset=utf-8');return res.end(`<!doctype html><html lang="${lang}" data-theme="dark"><head><meta charset="utf-8"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/ruview.css"><style>body{margin:0}#app{height:100vh}</style></head><body class="desktop-body" data-theme="${theme}" data-fruity-mode="${tone}"><main id="app"></main><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/apps/ruview-model.js"></script><script src="/js/desktop/apps/ruview-store.js"></script><script src="/js/desktop/apps/ruview-views.js"></script><script src="/js/desktop/apps/ruview.js"></script><script>const dict=${JSON.stringify(locale)};window.render=()=>RuViewApp.render(document.getElementById('app'),'test',{t:k=>dict[k]||k,esc:s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),readonly:false,modalDialog:async options=>{(window.__ruviewModals=window.__ruviewModals||[]).push(options);return window.__ruviewAnswer===undefined?'confirm':window.__ruviewAnswer;},openApp(){},setWindowBeforeClose(){},setWindowMenus(){},clearWindowMenus(){}});render();</script></body></html>`);
  }
  const rel=p.includes('/ui/')?'js/vendor/ruview/'+p.split('/ui/')[1]:p.slice(1);
  if(!rel||rel.includes('..')){res.writeHead(404).end();return;}
  let data=await readFile(path.join('ui',rel));
  if(p.includes('/ui/')){const nonce=randomBytes(18).toString('base64url');if(rel.endsWith('.html'))data=Buffer.from(data.toString().replaceAll('<script',`<script nonce="${nonce}"`));const host=req.headers.host,origins=host&&!/[ ;'"]/.test(host)?` http://${host} https://${host}`:'';res.setHeader('Content-Security-Policy',`default-src 'none'; script-src 'self'${origins} 'nonce-${nonce}' 'wasm-unsafe-eval'; style-src 'self'${origins} 'unsafe-inline'; img-src 'self'${origins} data: blob:; font-src 'self'${origins}; connect-src 'none'; worker-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'none'`);res.setHeader('Permissions-Policy','serial=(), camera=(), microphone=()');res.setHeader('Access-Control-Allow-Origin','*');}
  if(p.startsWith('/fonts/'))res.setHeader('Access-Control-Allow-Origin','*');
  res.setHeader('Content-Type',({'.js':'application/javascript','.mjs':'application/javascript','.css':'text/css','.html':'text/html','.json':'application/json','.svg':'image/svg+xml','.wasm':'application/wasm'})[path.extname(rel)]||'application/octet-stream');res.end(data);
 }catch{res.writeHead(404).end();}
});
server.on('upgrade',(req,socket)=>{
 const posePath='/api/desktop/integrations/ruview/proxy/api/v1/stream/pose';
 if(req.url.startsWith(posePath))poseRequests.push(req.url);
 const sockets=req.url===posePath?poseSockets:req.url==='/api/desktop/integrations/ruview/proxy/ws/sensing'?sensingSockets:null;
 if(!sockets){socket.destroy();return;}
 const accept=createHash('sha1').update(req.headers['sec-websocket-key']+'258EAFA5-E914-47DA-95CA-C5AB0DC85B11').digest('base64');
 socket.write('HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: '+accept+'\r\n\r\n');
 sockets.add(socket);socket.on('close',()=>sockets.delete(socket));socket.on('error',()=>{});
 socket.on('data',data=>{if((data[0]&15)===8)socket.end(Buffer.from([0x88,0]));});
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const browser=await chromium.launch({headless:true,channel:'chrome'});
await mkdir('reports/ruview-browser',{recursive:true});
try{
 const page=await browser.newPage();
 const origin=`http://127.0.0.1:${server.address().port}`;
 const de=JSON.parse(await readFile('ui/lang/ruview/de.json','utf8'));
 await Promise.race([
 new Promise((_,reject)=>page.on('pageerror',reject)),
 (async()=>{
 for(const theme of ['standard','fruity-light','fruity-dark'])for(const width of [1100,390])for(const lang of locales){
  await page.setViewportSize({width,height:800});await page.goto(`${origin}/?lang=${lang}&theme=${theme==='standard'?'standard':'fruity'}&mode=${theme.endsWith('dark')?'dark':'light'}`);
  await resolved(page);assert.equal(writes,0,'render must not probe');
  assert.ok(!/ruview\.[a-z_]+/.test(await page.locator('#app').innerText()),`missing ${lang} labels`);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow ${lang} ${width}`);
  if(lang==='de')await page.screenshot({path:`reports/ruview-browser/${theme}-${width}.png`});
  nodeFixture.offline=true;
  await openView(page,'boards');await page.locator('.ruview-node').waitFor();
  assert.ok(!/ruview\.[a-z_]+/.test(await page.locator('#app').innerText()),`device translations ${lang}`);
  assert.equal(await page.locator('[data-age]').innerText(),'—','known offline boards must not invent measurements');
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`device overflow ${lang} ${width}`);
  if(lang==='de')await page.screenshot({path:`reports/ruview-browser/devices-${theme}-${width}.png`});
 }
 nodeFixture.offline=false;
 await page.setViewportSize({width:1100,height:800});await page.emulateMedia({colorScheme:'dark'});
 await page.goto(`${origin}/?lang=de`);await resolved(page);
 const idle={mode:'docker',ready:false,fresh:false,data_state:'unchecked',last_frame_age_ms:null,observed_at:null,last_fresh_at:null};
 for(const [extra,state] of [[{...idle,runtime:'not_started'},'stopped'],[{...idle,runtime:'starting',phase:'pulling_image',phase_since:new Date().toISOString()},'starting_pull'],[null,'live']]){
  nodeFixture.statusExtra=extra;await page.evaluate(()=>render());await waitState(page,state);
  assert.equal(writes,0,'opening the app must not start or probe a service');
 }
 await page.evaluate(()=>render());await waitState(page,'live');
 assert.equal(writes,0,'reopening preserves backend readiness without another start');
 await checkStatusStates(page,nodeFixture,ctl);
 await checkStateFixtures(page,nodeFixture);
 await checkCommandBar(page);
 await checkFocusKept(page,nodeFixture);
 await checkLiveRegion(page,nodeFixture,ctl);
 await checkImageDenied(page,nodeFixture);
 await checkAllowChanges(page,nodeFixture,ctl);
 await page.setViewportSize({width:1100,height:800});
 await page.evaluate(()=>render());await resolved(page);
 await openView(page,'boards');await page.locator('.ruview-node').waitFor();
 await checkDevices(page,nodeFixture);
 await page.locator('[data-ruview-primary]').click();await page.locator('dialog').waitFor();
 await page.locator('dialog input[name=ssid]').fill('memory-only');await page.locator('dialog input[name=password]').fill('memory-password');await page.locator('dialog button[value=close]').click();await page.locator('dialog').waitFor({state:'detached'});
 assert.equal(await page.evaluate(()=>localStorage.length),0);
 await openView(page,'status');await page.locator('[data-ruview-more]').click();await page.locator('[data-ruview-menu] [data-action=test]').click();
 await until(()=>writes===1);
 for(failure of ['error','message','html']){
  await page.locator('[data-ruview-more]').click();await page.locator('[data-ruview-menu] [data-action=test]').click();
  const detail=failure==='html'?'HTTP 502':'registry denied';
  await page.waitForFunction(detail=>document.querySelector('[data-ruview-action-error]')?.textContent.includes(detail),detail);
  assert.equal(await page.locator('[data-ruview-action-error] img').count(),0,'backend errors must remain text');
  await page.waitForTimeout(3200);
  assert.ok((await page.locator('[data-ruview-action-error]').textContent()).includes(detail),'passive status polling must preserve the action failure');
 }
 failure='';statusError='RuView service could not be reached';offline=true;
 await page.evaluate(()=>render());await waitState(page,'error');
 assert.equal(await page.locator('[data-ruview-headline-text]').innerText(),de['ruview.err_unreachable']);
 assert.ok((await page.locator('[data-ruview-technical]').textContent()).includes('could not be reached'));
 firmwareError=true;await openView(page,'boards');await page.locator('[data-ruview-primary]').click();await page.locator('dialog').waitFor();
 await page.waitForFunction(()=>document.querySelector('dialog [role=status]')?.textContent.includes('Firmware catalog unavailable'));
 await page.locator('dialog button[value=close]').click();firmwareError=false;statusError='';
 readonly=true;await page.evaluate(()=>render());await openView(page,'boards');
 await page.waitForFunction(()=>document.querySelector('[data-ruview-primary]')?.getAttribute('aria-disabled')==='true');
 // A blocked control names its reason: the visible line is what its description points to.
 await page.locator('button[data-node="7"]').waitFor();
 assert.equal(await page.locator('button[data-node="7"]').getAttribute('aria-disabled'),'true');
 for(const selector of ['[data-ruview-primary]','button[data-node="7"]'])assert.equal(await describedText(page,selector),de['ruview.reason_readonly'],selector);
 assert.equal(await page.locator('[data-ruview-add-reason]').isVisible(),true);
 await page.locator('[data-ruview-more]').click();
 assert.equal(await page.locator('[data-ruview-menu] [data-action=test]').getAttribute('aria-disabled'),null,'checking stays possible while read-only');
 assert.equal(await page.locator('[data-ruview-menu] [data-action="runtime/stop"]').count(),0,'external service cannot be stopped');
 await page.keyboard.press('Escape');
 mode='native';nodeFixture.statusExtra={runtime:'running'};await page.evaluate(()=>render());
 await page.locator('[data-ruview-more]').click();
 await page.locator('[data-ruview-menu] [data-action="runtime/stop"]').waitFor();
 assert.equal(await page.locator('[data-ruview-menu] [data-action="runtime/stop"]').getAttribute('aria-disabled'),null,'stopping stays possible while read-only');
 const before=writes;await page.locator('[data-ruview-menu] [data-action="runtime/stop"]').click();
 await until(()=>writes===before+1);
 // Read-only blocks a start: after a failed start the row offers "retry", blocked with its reason, and a click sends nothing.
 nodeFixture.statusExtra={runtime:'not_started',error_code:'image_denied',ready:false,fresh:false,data_state:'unchecked',last_frame_age_ms:null,observed_at:null,last_fresh_at:null};
 await page.evaluate(()=>render());await waitState(page,'error');
 const retry=page.locator('[data-ruview-next][data-action="runtime/start"]');
 assert.equal(await retry.getAttribute('aria-disabled'),'true','read-only blocks the start');
 assert.equal(await describedText(page,'[data-ruview-next]'),de['ruview.reason_readonly']);
 const sent=writes;await retry.click({force:true});await page.waitForTimeout(600);
 assert.equal(writes,sent,'a blocked start sends nothing');
 mode='external';nodeFixture.statusExtra=null;
 offline=true;await page.evaluate(()=>render());await openView(page,'status');
 for(const view of ['index.html','observatory.html','pose-fusion.html']){
  await openLab(page,view);await page.waitForTimeout(1500);assert.equal(await page.locator('iframe').getAttribute('sandbox'),'allow-scripts');assert.equal(await page.locator('iframe').getAttribute('allow'),"serial 'none'; camera 'none'; microphone 'none'");
  assert.equal(await page.locator('iframe').getAttribute('title'),de[`ruview.lab_frame_title_${view==='index.html'?'dashboard':view.replace('.html','').replace('-','_')}`],'the frame is named in the user language');
  const frame=page.frames().find(f=>f.url().endsWith('/'+view));
  if(view==='index.html'){
   assert.match(await page.evaluate(async()=>(await fetch(document.querySelector('iframe').src)).headers.get('content-security-policy')||''),/connect-src 'none'/);
   const configBefore=configRequests;
   assert.equal(await frame.evaluate(async()=>{try{await fetch('/api/config');return false}catch{return true}}),true);
   assert.equal(configRequests,configBefore,'iframe fetch of /api/config must not arrive');
   await frame.waitForFunction(()=>document.querySelector('.conn-status-label')?.textContent==='Waiting for data');
   assert.ok((await frame.locator('#ruview-data-source-banner').innerText()).includes('WAITING FOR SENSOR DATA'));
   assert.ok(!(await frame.locator('body').innerText()).includes('Backend unavailable'));
   assert.ok(healthRequests.has('/health/live'));
   await frame.getByText('Skip tour',{exact:true}).click();
   assert.equal(await page.evaluate(()=>localStorage.getItem('aurago.ruview.onboarding.done')),'true');
   await page.screenshot({path:'reports/ruview-browser/upstream-waiting.png'});
   sendSensing({source:'esp32',features:{mean_rssi:-50}});
   await frame.waitForFunction(()=>document.querySelector('.conn-status-label')?.textContent==='Live');
   await frame.getByRole('tab',{name:'Live Demo',exact:true}).click();
   await frame.waitForFunction(()=>document.querySelector('#status-text-pose-detection-main')?.textContent==='Connected');
   assert.equal(poseRequests.length,1,'Live Demo opens one pose stream');
   assert.equal(poseRequests[0],'/api/desktop/integrations/ruview/proxy/api/v1/stream/pose','pose stream has no unsupported query parameters');
   sendFrames(poseSockets,{type:'pose_data',zone_id:'zone_1',timestamp:Date.now()/1000,payload:{
    pose:{persons:[]},confidence:0,activity:'absent',pose_source:'signal_derived',
    metadata:{frame_id:'fixture_pose_1',processing_time_ms:1,source:'esp32',tick:1}
   }});
   await frame.waitForFunction(()=>Number(document.querySelector('#frame-count')?.textContent)>0);
   assert.equal(await frame.locator('#pose-source-badge').textContent(),'Signal-Derived');
   assert.equal(await frame.locator('#error-pose-detection-main').isVisible(),false);
   await page.screenshot({path:'reports/ruview-browser/upstream-pose-connected.png'});
   await frame.getByRole('tab',{name:'Dashboard',exact:true}).click();
   await frame.waitForFunction(()=>document.querySelector('.conn-status-label')?.textContent==='Stale data',{},{timeout:15000});
   sendSensing({source:'esp32',features:{mean_rssi:-50}});
   await frame.waitForFunction(()=>document.querySelector('.conn-status-label')?.textContent==='Live');
   sendSensing({source:'simulated',_simulated:true});
   await frame.waitForFunction(()=>document.querySelector('.conn-status-label')?.textContent==='Simulated');
   assert.ok((await frame.locator('#ruview-data-source-banner').innerText()).includes('SYNTHETIC DATA'));
   sourceProbeFailed=true;
   await frame.evaluate(async()=>{const {sensingService}=await import('./services/sensing.service.js');await sensingService._detectServerSource();});
   await frame.waitForFunction(()=>document.querySelector('.conn-status-label')?.textContent==='Waiting for data');
   sourceProbeFailed=false;
   assert.equal(await frame.evaluate(async()=>{
    const {default:Panel}=await import('./components/ModelPanel.js');const panel=Object.create(Panel.prototype);
    panel.state={models:[],loraProfiles:[],activeModel:{avg_inference_ms:1,frames_processed:'<img src=x onerror="window.ruviewInjected=1">'}};
    const card=panel._renderActive();document.body.append(card);const count=card.querySelectorAll('img').length;card.remove();
    const button=document.createElement('button');button.setAttribute('onclick','window.ruviewInjected=1');document.body.append(button);button.click();button.remove();
    return count===0&&!window.ruviewInjected;
   }),true,'untrusted counters and inline event handlers must not execute');
   await checkPolicy(frame,value=>{readonly=value;});
   readonly=false;await checkControls(frame);
  }
  if(view==='observatory.html'){
   await frame.waitForFunction(()=>document.getElementById('data-source-label')?.textContent==='NO LIVE DATA');
   assert.equal(await frame.locator('#hr-value').textContent(),'—');
   source='simulated';await frame.waitForFunction(()=>document.getElementById('data-source-label')?.textContent==='DEMO');
   source='esp32';await frame.waitForFunction(()=>document.getElementById('data-source-label')?.textContent==='NO LIVE DATA');
  }
 }
 const fusion=page.frames().find(f=>f.url().endsWith('/pose-fusion.html'));
 assert.equal(await fusion.locator('#mode-select').inputValue(),'csi');
 assert.equal(await fusion.locator('#mode-select option:disabled').count(),2);
 assert.equal(await fusion.locator('#start-camera-btn').isDisabled(),true);
 assert.equal(await fusion.locator('#camera-prompt').isVisible(),false);
 await fusion.waitForFunction(()=>document.getElementById('status-label')?.textContent==='NO LIVE CSI');
 await fusion.waitForFunction(()=>document.getElementById('cnn-backend')?.textContent.includes('RuVector WASM'));
 assert.equal(await fusion.locator('#csi-bar-val').textContent(),'Unavailable');
 source='simulated';await fusion.waitForFunction(()=>window.auragoRuViewSimulationAllowed());
 await fusion.waitForFunction(()=>document.getElementById('csi-bar-val')?.textContent!=='Unavailable');
 source='esp32';await fusion.waitForFunction(()=>document.getElementById('status-label')?.textContent==='NO LIVE CSI');
 await fusion.goto(`${origin}/api/desktop/integrations/ruview/ui/viz.html`);
 await fusion.waitForFunction(()=>document.querySelector('#viz-container > canvas')?.style.visibility==='hidden');
 source='simulated';await fusion.waitForFunction(()=>document.querySelector('#viz-container > canvas')?.style.visibility==='');
 source='esp32';await fusion.waitForFunction(()=>document.querySelector('#viz-container > canvas')?.style.visibility==='hidden');
 // Tab changes, app reopening and full reload must preserve a dismissed tour.
 for(const reopen of [
  async()=>{},
  async()=>{await page.evaluate(()=>{RuViewApp.dispose('test');render();});await resolved(page);},
  async()=>{await page.reload();await resolved(page);}
 ]){
  await reopen();await openLab(page,'index.html');
  const dashboard=page.frameLocator('iframe');
  await dashboard.locator('.conn-status-label').waitFor();await page.waitForTimeout(1100);
  assert.equal(await dashboard.locator('.onboarding-overlay').count(),0,'dismissed tour must not restart');
 }
 const dashboard=page.frames().find(f=>f.url().endsWith('/index.html'));
 // Explicit replay remains available and completing every step is also remembered.
 await dashboard.getByRole('button',{name:'Settings',exact:true}).click();
 await dashboard.locator('#qs-reset-tour').click();
 assert.equal(await page.evaluate(()=>localStorage.getItem('aurago.ruview.onboarding.done')),null);
 await dashboard.locator('.onboarding-overlay').waitFor();
 for(let step=0;step<6;step++){
  assert.equal(await dashboard.locator('.onboarding-next').evaluate(button=>button===document.activeElement),true);
  await dashboard.locator('.onboarding-next').press('Enter');
 }
 assert.equal(await dashboard.locator('.onboarding-overlay').count(),0);
 assert.equal(await page.evaluate(()=>localStorage.getItem('aurago.ruview.onboarding.done')),'true');
 await page.reload();await resolved(page);await openLab(page,'index.html');
 await page.frameLocator('iframe').locator('.conn-status-label').waitFor();await page.waitForTimeout(1100);
 assert.equal(await page.frameLocator('iframe').locator('.onboarding-overlay').count(),0,'completed tour must not restart');
 await page.evaluate(()=>RuViewApp.dispose('test'));assert.equal(await page.locator('iframe').count(),0);
 console.log('RuView: 96 locale/theme/size cases, 18-state walk without scrolling, 560 px command bar, kept focus, silent live region, image-denied cause, allow-changes confirmation, pose WebSocket and frame delivery, tour dismissal/replay/persistence, device aging, read failures, stop permissions, controls, source provenance, live/stale recovery, health routing, wizard cleanup and disposal passed');
 })()
 ]);
}finally{await browser.close();for(const socket of [...sensingSockets,...poseSockets])socket.destroy();await new Promise(resolve=>server.close(resolve));}
