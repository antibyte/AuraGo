import {createServer} from 'node:http';
import {readFile,mkdir} from 'node:fs/promises';
import path from 'node:path';
import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {randomBytes,createHash} from 'node:crypto';
import {checkControls} from './control-browser.mjs';
import {checkDevices,checkPolicy} from './ui-browser.mjs';
const require=createRequire(import.meta.url);
const {chromium}=process.env.AURAGO_PLAYWRIGHT?require(process.env.AURAGO_PLAYWRIGHT):require('playwright');
const locales=['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
let writes=0,readonly=false,offline=false,mode='external',source='esp32',failure='',statusError='',firmwareError=false,sourceState='disconnected',sourceProbeFailed=false;
let serviceState='';
const nodeFixture={reads:0,active:0,max:0,fail:false,statusFail:false,hold:null,offline:false,unavailable:false,scope:'fixture'};
const sensingSockets=new Set(),poseSockets=new Set(),poseRequests=[],healthRequests=new Set();
function sendFrames(sockets,data){
 const body=Buffer.from(JSON.stringify(data));assert.ok(body.length<65536);
 const header=body.length<126?Buffer.from([0x81,body.length]):Buffer.from([0x81,126,0,0]);
 if(body.length>=126)header.writeUInt16BE(body.length,2);
 for(const socket of sockets)socket.write(Buffer.concat([header,body]));
}
const sendSensing=data=>sendFrames(sensingSockets,data);
const server=createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://localhost'),p=url.pathname;
  if(p.startsWith('/api/')&&!p.includes('/ui/')){
   res.setHeader('Content-Type','application/json');if(req.method==='POST')writes++;
   if(p.endsWith('/ruview/nodes')){
    nodeFixture.reads++;nodeFixture.active++;nodeFixture.max=Math.max(nodeFixture.max,nodeFixture.active);
    try {
     if(nodeFixture.hold)await nodeFixture.hold;
     if(nodeFixture.fail){res.statusCode=503;return res.end('{}');}
     return res.end(JSON.stringify({scope:nodeFixture.scope,available:!nodeFixture.unavailable,nodes:nodeFixture.offline||nodeFixture.unavailable?[{node_id:7,csi_status:'offline',csi_last_seen_ms:null,csi_sequence:null,rssi_dbm:null}]:[{node_id:7,csi_status:'active',csi_last_seen_ms:42,csi_sequence:3,rssi_dbm:-51}]}));
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
   if(p.endsWith('/status')&&nodeFixture.statusFail){res.statusCode=503;return res.end('{}');}
   return res.end(JSON.stringify({enabled:true,readonly,mode,source,ready:!offline&&!serviceState,fresh:!offline&&!serviceState,data_state:serviceState||(offline?'disconnected':'live_unverified'),signal:'CSI',observed_at:new Date().toISOString(),error:statusError||undefined}));
  }
  if(p==='/'){
   const locale=JSON.parse(await readFile(`ui/lang/ruview/${url.searchParams.get('lang')||'de'}.json`));
   res.setHeader('Content-Type','text/html; charset=utf-8');return res.end(`<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/css/ruview.css"><style>body{margin:0}#app{height:100vh}</style></head><body><main id="app"></main><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/apps/ruview.js"></script><script>const dict=${JSON.stringify(locale)};window.render=()=>RuViewApp.render(document.getElementById('app'),'test',{t:k=>dict[k]||k,esc:s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';')});render();</script></body></html>`);
  }
  const rel=p.includes('/ui/')?'js/vendor/ruview/'+p.split('/ui/')[1]:p.slice(1);
  if(!rel||rel.includes('..')){res.writeHead(404).end();return;}
  let data=await readFile(path.join('ui',rel));
  if(p.includes('/ui/')){const nonce=randomBytes(18).toString('base64url');if(rel.endsWith('.html'))data=Buffer.from(data.toString().replaceAll('<script',`<script nonce="${nonce}"`));res.setHeader('Content-Security-Policy',`default-src 'none'; script-src 'self' 'nonce-${nonce}' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; worker-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'none'`);res.setHeader('Permissions-Policy','serial=(), camera=(), microphone=()');}
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
 await Promise.race([
 new Promise((_,reject)=>page.on('pageerror',reject)),
 (async()=>{
 for(const theme of ['standard','fruity-light','fruity-dark'])for(const width of [1100,390])for(const lang of locales){
  await page.setViewportSize({width,height:800});await page.goto(`http://127.0.0.1:${server.address().port}/?lang=${lang}`);
  if(theme==='fruity-light')await page.addStyleTag({content:':root{--vd-theme-app-bg:#f5f6f7;--vd-text:#253044;--vd-theme-panel-bg:#fff;--vd-theme-border:#ccd3da}'});
  if(theme==='fruity-dark')await page.addStyleTag({content:':root{--vd-theme-app-bg:#16171b;--vd-text:#f0f0f2;--vd-theme-panel-bg:#24252b;--vd-theme-border:#46474d}'});
  await page.locator('[data-action=test]').waitFor();assert.equal(writes,0,'render must not probe');
  assert.ok(!/ruview\.[a-z_]+/.test(await page.locator('#app').innerText()),`missing ${lang} labels`);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow ${lang} ${width}`);
  if(lang==='de')await page.screenshot({path:`reports/ruview-browser/${theme}-${width}.png`});
  nodeFixture.offline=true;
  await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
  assert.ok(!/ruview\.[a-z_]+/.test(await page.locator('#app').innerText()),`device translations ${lang}`);
  assert.equal(await page.locator('[data-age]').innerText(),'—','known offline boards must not invent measurements');
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`device overflow ${lang} ${width}`);
  if(lang==='de')await page.screenshot({path:`reports/ruview-browser/devices-${theme}-${width}.png`});
 }
 nodeFixture.offline=false;
 await page.setViewportSize({width:1100,height:800});await page.emulateMedia({colorScheme:'dark'});
 await page.goto(`http://127.0.0.1:${server.address().port}/?lang=de`);await page.locator('[data-action=test]').waitFor();
 for(const [state,label] of [['unchecked','Nicht geprüft'],['starting','Wird ausgeführt…'],['','Bereit']]){
  serviceState=state;await page.evaluate(()=>render());
  await page.waitForFunction(label=>document.querySelector('.ruview-summary article strong')?.textContent===label,label);
  assert.equal(writes,0,'opening the app must not start or probe a service');
 }
 await page.evaluate(()=>render());await page.waitForFunction(()=>document.querySelector('.ruview-summary article strong')?.textContent==='Bereit');
 assert.equal(writes,0,'reopening preserves backend readiness without another start');
 await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
 await checkDevices(page,nodeFixture);
 await page.locator('[data-action=flash]').click();await page.locator('dialog').waitFor();
 await page.locator('dialog input[name=ssid]').fill('memory-only');await page.locator('dialog input[name=password]').fill('memory-password');await page.locator('dialog button[value=close]').click();await page.locator('dialog').waitFor({state:'detached'});
 assert.equal(await page.evaluate(()=>localStorage.length),0);
 await page.locator('[data-tab=overview]').click();await page.locator('[data-action=test]').click();assert.equal(writes,1);
 for(failure of ['error','message','html']){
  await page.locator('[data-action=test]').click();
  const detail=failure==='html'?'HTTP 502':'registry denied';
  await page.waitForFunction(detail=>document.querySelector('.ruview-feedback')?.textContent.includes(detail),detail);
  assert.equal(await page.locator('.ruview-feedback img').count(),0,'backend errors must remain text');
  await page.waitForTimeout(3200);
  assert.ok((await page.locator('.ruview-feedback').innerText()).includes(detail),'passive status polling must preserve the action failure');
 }
 failure='';statusError='RuView service could not be reached';offline=true;
 await page.evaluate(()=>render());await page.waitForFunction(()=>document.querySelector('.ruview-feedback')?.textContent.includes('could not be reached'));
 firmwareError=true;await page.locator('[data-action=flash]').click();await page.locator('dialog').waitFor();
 await page.waitForFunction(()=>document.querySelector('dialog [role=status]')?.textContent.includes('Firmware catalog unavailable'));
 await page.locator('dialog button[value=close]').click();firmwareError=false;statusError='';
 readonly=true;await page.evaluate(()=>render());await page.waitForFunction(()=>document.querySelector('[data-action=flash]')?.disabled);assert.equal(await page.locator('[data-action=test]').isDisabled(),false);
 assert.equal(await page.locator('[data-action="runtime/stop"]').isDisabled(),true,'external service cannot be stopped');
 mode='native';await page.evaluate(()=>render());await page.waitForFunction(()=>document.querySelector('[data-action="runtime/stop"]')&&!document.querySelector('[data-action="runtime/stop"]').disabled);
 assert.equal(await page.locator('[data-action="runtime/start"]').isDisabled(),true,'read-only still blocks start');
 await page.locator('[data-action="runtime/stop"]').click();mode='external';
 offline=true;await page.evaluate(()=>render());await page.locator('.ruview-summary').waitFor();
 for(const view of ['index.html','observatory.html','pose-fusion.html']){
  await page.locator(`[data-tab="${view}"]`).click();await page.locator('iframe').waitFor();await page.waitForTimeout(1500);assert.equal(await page.locator('iframe').getAttribute('allow'),"serial 'none'; camera 'none'; microphone 'none'");
  const frame=page.frames().find(f=>f.url().endsWith('/'+view));
  if(view==='index.html'){
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
 await fusion.goto(`http://127.0.0.1:${server.address().port}/api/desktop/integrations/ruview/ui/viz.html`);
 await fusion.waitForFunction(()=>document.querySelector('#viz-container > canvas')?.style.visibility==='hidden');
 source='simulated';await fusion.waitForFunction(()=>document.querySelector('#viz-container > canvas')?.style.visibility==='');
 source='esp32';await fusion.waitForFunction(()=>document.querySelector('#viz-container > canvas')?.style.visibility==='hidden');
 // Tab changes, app reopening and full reload must preserve a dismissed tour.
 for(const reopen of [
  async()=>{},
  async()=>{await page.evaluate(()=>{RuViewApp.dispose('test');render();});},
  async()=>{await page.reload();}
 ]){
  await reopen();await page.locator('[data-tab="index.html"]').click();await page.locator('iframe').waitFor();
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
 await page.reload();await page.locator('[data-tab="index.html"]').click();
 await page.frameLocator('iframe').locator('.conn-status-label').waitFor();await page.waitForTimeout(1100);
 assert.equal(await page.frameLocator('iframe').locator('.onboarding-overlay').count(),0,'completed tour must not restart');
 await page.evaluate(()=>RuViewApp.dispose('test'));assert.equal(await page.locator('iframe').count(),0);
 console.log('RuView: 96 locale/theme/size cases, pose WebSocket and frame delivery, tour dismissal/replay/persistence, device aging, read failures, stop permissions, controls, source provenance, live/stale recovery, health routing, wizard cleanup and disposal passed');
 })()
 ]);
}finally{await browser.close();for(const socket of [...sensingSockets,...poseSockets])socket.destroy();await new Promise(resolve=>server.close(resolve));}
