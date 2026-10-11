import {createServer} from 'node:http';
import {readFile,mkdir} from 'node:fs/promises';
import path from 'node:path';
import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
const require=createRequire(import.meta.url);
const {chromium}=process.env.AURAGO_PLAYWRIGHT?require(process.env.AURAGO_PLAYWRIGHT):require('playwright');
const locales=['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
let writes=0,readonly=false,offline=false,source='esp32';
const server=createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://localhost'),p=url.pathname;
  if(p.startsWith('/api/')&&!p.includes('/ui/')){
   res.setHeader('Content-Type','application/json');if(req.method==='POST')writes++;
   if(p.endsWith('/firmware'))return res.end(await readFile('internal/ruview/artifacts.json'));
   if(p.endsWith('/nodes'))return res.end(JSON.stringify({nodes:[{node_id:7,csi_status:'active',csi_last_seen_ms:42,csi_sequence:3,rssi_dbm:-51}]}));
   return res.end(JSON.stringify({enabled:true,readonly,mode:'external',source,ready:!offline,fresh:!offline,data_state:offline?'disconnected':'live_unverified',signal:'CSI',observed_at:new Date().toISOString()}));
  }
  if(p==='/'){
   const locale=JSON.parse(await readFile(`ui/lang/ruview/${url.searchParams.get('lang')||'de'}.json`));
   res.setHeader('Content-Type','text/html; charset=utf-8');return res.end(`<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/css/ruview.css"><style>body{margin:0}#app{height:100vh}</style></head><body><main id="app"></main><script src="/js/desktop/apps/ruview.js"></script><script>const dict=${JSON.stringify(locale)};window.render=()=>RuViewApp.render(document.getElementById('app'),'test',{t:k=>dict[k]||k,esc:s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';')});render();</script></body></html>`);
  }
  const rel=p.includes('/ui/')?'js/vendor/ruview/'+p.split('/ui/')[1]:p.slice(1);
  if(!rel||rel.includes('..')){res.writeHead(404).end();return;}
  let data=await readFile(path.join('ui',rel));
  if(p.includes('/ui/')){const nonce=randomBytes(18).toString('base64url');if(rel.endsWith('.html'))data=Buffer.from(data.toString().replaceAll('<script',`<script nonce="${nonce}"`));res.setHeader('Content-Security-Policy',`default-src 'none'; script-src 'self' 'nonce-${nonce}' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; worker-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'none'`);res.setHeader('Permissions-Policy','serial=(), camera=(), microphone=()');}
  res.setHeader('Content-Type',({'.js':'application/javascript','.mjs':'application/javascript','.css':'text/css','.html':'text/html','.json':'application/json','.svg':'image/svg+xml','.wasm':'application/wasm'})[path.extname(rel)]||'application/octet-stream');res.end(data);
 }catch{res.writeHead(404).end();}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const browser=await chromium.launch({headless:true,channel:'chrome'});
await mkdir('reports/ruview-browser',{recursive:true});
try{
 const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 for(const theme of ['standard','fruity'])for(const width of [1100,390])for(const lang of locales){
  await page.setViewportSize({width,height:800});await page.goto(`http://127.0.0.1:${server.address().port}/?lang=${lang}`);
  if(theme==='fruity')await page.addStyleTag({content:':root{--vd-theme-app-bg:#f5f6f7;--vd-text:#253044;--vd-theme-panel-bg:#fff;--vd-theme-border:#ccd3da}'});
  await page.locator('[data-action=test]').waitFor();assert.equal(writes,0,'render must not probe');
  assert.ok(!/ruview\.[a-z_]+/.test(await page.locator('#app').innerText()),`missing ${lang} labels`);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow ${lang} ${width}`);
  if(lang==='de')await page.screenshot({path:`reports/ruview-browser/${theme}-${width}.png`});
 }
 await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
 await page.locator('[data-action=flash]').click();await page.locator('dialog').waitFor();
 await page.locator('dialog input[name=ssid]').fill('memory-only');await page.locator('dialog input[name=password]').fill('memory-password');await page.locator('dialog button[value=close]').click();await page.locator('dialog').waitFor({state:'detached'});
 assert.equal(await page.evaluate(()=>localStorage.length),0);
 await page.locator('[data-tab=overview]').click();await page.locator('[data-action=test]').click();assert.equal(writes,1);
 readonly=true;await page.evaluate(()=>render());await page.waitForFunction(()=>document.querySelector('[data-action=flash]')?.disabled);assert.equal(await page.locator('[data-action=test]').isDisabled(),false);
 offline=true;await page.evaluate(()=>render());await page.locator('.ruview-summary').waitFor();
 for(const view of ['index.html','observatory.html','pose-fusion.html']){
  await page.locator(`[data-tab="${view}"]`).click();await page.locator('iframe').waitFor();await page.waitForTimeout(1500);assert.equal(await page.locator('iframe').getAttribute('allow'),"serial 'none'; camera 'none'; microphone 'none'");
  const frame=page.frames().find(f=>f.url().endsWith('/'+view));
  if(view==='index.html'){
   assert.equal(await frame.evaluate(async()=>{
    const {default:Panel}=await import('./components/ModelPanel.js');const panel=Object.create(Panel.prototype);
    panel.state={models:[],loraProfiles:[],activeModel:{avg_inference_ms:1,frames_processed:'<img src=x onerror="window.ruviewInjected=1">'}};
    const card=panel._renderActive();document.body.append(card);const count=card.querySelectorAll('img').length;card.remove();
    const button=document.createElement('button');button.setAttribute('onclick','window.ruviewInjected=1');document.body.append(button);button.click();button.remove();
    return count===0&&!window.ruviewInjected;
   }),true,'untrusted counters and inline event handlers must not execute');
  }
  if(view==='observatory.html'){
   await frame.waitForFunction(()=>document.getElementById('data-source-label')?.textContent==='NO LIVE DATA');
   assert.equal(await frame.locator('#hr-value').textContent(),'—');
   source='simulated';await frame.waitForFunction(()=>document.getElementById('data-source-label')?.textContent==='DEMO');
   source='esp32';await frame.waitForFunction(()=>document.getElementById('data-source-label')?.textContent==='NO LIVE DATA');
  }
 }
 const fusion=page.frames().find(f=>f.url().endsWith('/pose-fusion.html'));
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
 await page.evaluate(()=>RuViewApp.dispose('test'));assert.equal(await page.locator('iframe').count(),0);
 assert.deepEqual(errors,[]);
 console.log('RuView: 64 locale/theme/size cases, passive status, wizard cleanup, read-only, offline and iframe disposal passed');
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
