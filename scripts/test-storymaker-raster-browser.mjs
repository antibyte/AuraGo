// Real opaque-origin player smoke: pinned renderer, strict CSP, texture transport, seek parity and software fallback.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { spawn, execFileSync } from 'node:child_process';
import { buildOutputs } from '../assets/storymaker/lib/catalog.mjs';
import { decodePNG, encodePNG } from '../assets/storymaker/lib/png.mjs';
const root=path.resolve(import.meta.dirname,'..'),ui=path.join(root,'ui');
const fixture=path.join(ui,'reports/storymaker-raster/prototype');
const reviewIndex=process.argv.indexOf('--review'),previews=process.argv.includes('--previews')||reviewIndex>=0;
const installed=path.join(ui,'img/storymaker');
const catalogDir=JSON.parse(fs.readFileSync(path.join(installed,'catalog.json'))).schema==='storymaker-catalog/2'?installed:fixture;
const baseline=process.argv.includes('--baseline'),legacy=baseline?buildOutputs():null;
const matrix=process.argv.includes('--matrix'),environments=process.argv.includes('--environments');
const previousBundles=new Map(baseline?['host','player'].map(name=>['/js/storymaker/'+name+'.bundle.js',execFileSync('git',['show','HEAD:ui/js/storymaker/'+name+'.bundle.js'],{cwd:root,maxBuffer:4*1048576})]):[]);
const browser=[process.env.CHROME_BIN,'C:/Program Files/Google/Chrome/Application/chrome.exe','C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe','/usr/bin/google-chrome','/usr/bin/chromium'].filter(Boolean).find(fs.existsSync);
assert.ok(browser,'An installed Chrome/Edge is required');
const story={schema:'storymaker/1',title:'Raster reference',format:{aspect:'16:9',fps:30},style:{palette:'sunny',font:'default',captions:false},voice:null,music:null,
    cast:[{id:'anna',character:'char.office-woman-1'},{id:'ben',character:'char.office-man-1'}],scenes:[{id:'reference',environment:'env.office',duration:30,silent:true,narration:'',
        actors:[{cast:'anna',at:'left'},{cast:'ben',at:'right',pose:'sit'}],props:[{id:'chair',item:'prop.chair',at:'right',layer:'back'}],beats:[{do:'walk',who:'anna',to:'center',duration:2},{do:'look',who:'ben',target:'anna',with:'prev'},{do:'gesture',who:'anna',gesture:'wave'},
            {do:'show',item:'prop.laptop',at:{x:78,y:35},entry:'pop'},{do:'show',text:'Storymaker',at:{x:50,y:15},style:'title'}]}]};
const parentScript=`
window.fixture={messages:[],errors:[],backends:[],frame:-1};
window.addEventListener('message',async event=>{
 if(event.source!==document.querySelector('iframe').contentWindow)return;
 const m=event.data;if(!m||m.sm!==1)return;fixture.messages.push(m.type);
 const post=data=>event.source.postMessage(Object.assign({sm:1},data),'*');
 try{
  if(m.type==='ready'){
   const catalog=await fetch('/img/storymaker/catalog.json').then(r=>r.json());
   const assets={};assets['catalog:'+catalog.rig.file]=await fetch('/img/storymaker/'+catalog.rig.file).then(r=>r.json());
   catalog.glyph_data=await fetch('/img/storymaker/'+catalog.glyphs.file).then(r=>r.json());
   const fonts=await Promise.all(catalog.fonts.map(async f=>({name:f.name,family:f.family,weight:f.weight,data:await fetch(f.file).then(r=>r.arrayBuffer())})));
   const story=${JSON.stringify(story)},params=new URL(location.href).searchParams,entry=catalog.entries.find(e=>e.id===params.get('preview'));
   story.format.aspect=params.get('aspect')||'16:9';
   if(params.has('matrix')){
    story.cast=story.cast.slice(0,1);story.cast[0].character=params.get('matrix');
    catalog.entries.filter(e=>e.kind==='mouth').forEach(e=>catalog.entries.push({id:'expr.qa-mouth-'+e.name,name:'qa-mouth-'+e.name,kind:'expression'}));
    catalog.entries.filter(e=>e.kind==='pose').forEach(e=>{e.view=params.get('view');});
    story.style.captions=false;story.format.aspect='1:1';
    story.scenes=catalog.entries.filter(e=>['pose','gesture','expression'].includes(e.kind)).map((e,i)=>({id:'state-'+i,environment:'env.plain-stage',duration:2,silent:true,narration:'',transition:'cut',camera:{framing:'medium',target:'anna'},actors:[{cast:'anna',at:'center',facing:params.get('facing'),pose:e.kind==='pose'?e.name:'stand',expression:e.kind==='expression'?e.name:'neutral'}],beats:e.kind==='gesture'?[{do:'gesture',who:'anna',gesture:e.name,duration:1.5}]:[]}));
    fixture.states=catalog.entries.filter(e=>['pose','gesture','expression'].includes(e.kind)).map(e=>e.id);
   }
   if(params.has('environment')){story.scenes[0].environment=params.get('environment');story.scenes[0].variant=params.get('variant');story.scenes[0].beats=[{do:'show',text:'Storymaker',style:'title',at:{x:50,y:15},entry:'none'}];}
   if(entry){
    story.cast=story.cast.slice(0,1);story.scenes=story.scenes.slice(0,1);story.scenes[0].actors=story.scenes[0].actors.slice(0,1);
    story.format.aspect='1:1';const scene=story.scenes[0];scene.environment='env.plain-stage';scene.duration=3;scene.beats=[];scene.props=[];scene.actors[0].at='center';scene.camera={framing:'medium',target:'anna'};
    if(entry.kind==='character')story.cast[0].character=entry.id;
    if(entry.kind==='pose')scene.beats=[{do:'pose',who:'anna',pose:entry.name,duration:0.25}];
    if(entry.kind==='gesture')scene.beats=[{do:'gesture',who:'anna',gesture:entry.name,duration:1.5}];
    if(entry.kind==='cycle'&&entry.name==='walk'){scene.actors[0].at='left';scene.beats=[{do:'walk',who:'anna',to:'right',duration:3}];}
    if(entry.kind==='text_style'){scene.actors=[];scene.beats=[{do:'show',text:'Storymaker',style:entry.name,at:'center'}];}
    if(entry.kind==='transition'){scene.duration=2;story.scenes.push({...scene,id:'second',environment:'env.office',transition:entry.name});}
    fixture.previewFrame=entry.kind==='gesture'?18:entry.kind==='transition'?(entry.name==='cut'?70:52):30;
   }
   const code_bundles={};
   if(params.has('code')){
    code_bundles.code=await fetch('/fixture-code.js').then(r=>r.text());
    story.scenes.unshift({id:'code',kind:'code',duration:5,silent:true,narration:''});
   }
   fixture.story=story;post({type:'load',story,catalog,assets,fonts,code_bundles,format:params.has('matrix')?{width:512}:undefined});
  }
  if(m.type==='need_assets'){
   const assets={};await Promise.all(m.keys.map(async key=>{
    const colon=key.indexOf(':'),type=key.slice(0,colon),file=key.slice(colon+1);
    if(type==='catalog')assets[key]=await fetch('/img/storymaker/'+file).then(r=>r.json());
    else if(type==='texture')assets[key]=await fetch('/img/storymaker/'+file).then(r=>r.arrayBuffer());
    else throw new Error('Unexpected asset '+key);
   }));post({type:'assets',req:m.req,assets,missing:[]});
  }
  if(m.type==='time')fixture.frame=m.frame;
  if(m.type==='renderer_info'){fixture.backends.push(m.backend);}
  if(m.type==='error')fixture.errors.push(m);
 }catch(e){fixture.errors.push({message:e.message});}
});
window.seek=frame=>{fixture.frame=-1;document.querySelector('iframe').contentWindow.postMessage({sm:1,type:'seek',frame},'*');};
`;
const server=http.createServer((request,response)=>{
    try{
        const address='http://127.0.0.1:'+server.address().port,url=new URL(request.url,address);
        if(url.pathname==='/'){
            const size=url.searchParams.has('preview')?'256px;height:256px':'960px;height:540px';
            response.setHeader('Content-Type','text/html');response.end('<!doctype html><style>body{margin:0}iframe{width:'+size+';border:0;display:block}</style><iframe sandbox="allow-scripts" src="/player"></iframe><script>'+parentScript+'</script>');return;
        }
        if(url.pathname==='/player'){
            response.setHeader('Content-Type','text/html');response.setHeader('Content-Security-Policy',"sandbox allow-scripts; default-src 'none'; script-src "+address+"/js/storymaker/ "+address+"/js/vendor/storymaker/runtime.js blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src "+address+"/css/storymaker-player.css; frame-ancestors 'self'");
            response.end('<!doctype html><html data-sm-mode="app" data-sm-base="'+address+'"><link rel="stylesheet" href="/css/storymaker-player.css"><canvas id="sm-stage"></canvas><script src="/js/storymaker/host.bundle.js"></script></html>');return;
        }
        if(url.pathname==='/fixture-code.js'){
            response.setHeader('Content-Type','text/javascript');response.end(fs.readFileSync(path.join(root,'internal/storymaker/testdata/scenes/s3.bundle.js')));return;
        }
        const decoded=decodeURIComponent(url.pathname),base=decoded.startsWith('/img/storymaker/')?catalogDir:ui;
        const relative=base===catalogDir?decoded.slice('/img/storymaker/'.length):decoded.slice(1),file=path.resolve(base,relative);
        if(!file.startsWith(base+path.sep))throw new Error('Unsafe fixture path');
        response.setHeader('Access-Control-Allow-Origin','*');response.setHeader('X-Content-Type-Options','nosniff');
        response.setHeader('Content-Type',{'.js':'text/javascript','.json':'application/json','.css':'text/css','.png':'image/png','.woff2':'font/woff2','.ttf':'font/ttf'}[path.extname(file)]||'application/octet-stream');
        let contents=previousBundles.get(decoded)||legacy&&decoded.startsWith('/img/storymaker/')&&legacy.files.get(relative)||fs.readFileSync(file);contents=Buffer.from(contents);
        // Test-only instrumentation observes the real renderer without adding a product diagnostics API.
        if(decoded==='/js/storymaker/player.bundle.js'&&process.argv.includes('--webgl'))contents=Buffer.concat([contents,Buffer.from("\nStorymakerCore.raster.softwareGL = function(){return false;};\n")]);
        if(decoded==='/js/storymaker/player.bundle.js'&&matrix)contents=Buffer.concat([contents,Buffer.from("\n;(function(){const original=StorymakerCore.render.rigState;StorymakerCore.render.rigState=function(state){const result=original.apply(this,arguments);if(state.expression.indexOf('qa-mouth-')===0){result.mouth=state.expression.slice(9);result.expression='neutral';}return result;};})();\n")]);
        if(decoded==='/js/storymaker/player.bundle.js'&&process.argv.includes('--canvas')&&!baseline)contents=Buffer.concat([contents,Buffer.from("\nStorymakerVendor.WebGLRenderer.prototype.init = async function(){throw new Error('Test: WebGL is unavailable');};\n")]);
        if(decoded==='/js/storymaker/player.bundle.js')contents=Buffer.concat([contents,Buffer.from(`
;(function(){for(const name of ['workerPlayer','workerScene']){const p=StorymakerCore[name].Runtime.prototype,original=p.handle;p.handle=async function(m){const result=await original.call(this,m);if((m.type==='render'||m.type==='frame')&&!m.prepareOnly){const d=this.renderers&&Object.values(this.renderers)[0];result.payload.backend=d?(d.renderer instanceof StorymakerVendor.WebGLRenderer?'webgl':'canvas'):'legacy';${process.argv.includes('--loss')?"if(m.n===120&&!this.__lostOnce&&d&&d.renderer.gl){this.__lostOnce=true;d.renderer.gl.getExtension('WEBGL_lose_context').loseContext();}":''}}return result;};}})();
`)]);
        if(decoded==='/js/storymaker/host.bundle.js')contents=Buffer.concat([contents,Buffer.from(`
;(function(){const p=StorymakerHost.Channel.prototype,original=p.call;p.call=async function(){const result=await original.apply(this,arguments);if(result.backend)parent.postMessage({sm:1,type:'renderer_info',backend:result.backend},'*');return result;};})();
window.addEventListener('message',function(event){if(event.source!==parent||!event.data?.qaCapture)return;event.stopImmediatePropagation();const m=event.data,source=document.getElementById('sm-stage');let canvas=source;if(m.width>0){canvas=document.createElement('canvas');canvas.width=m.width;canvas.height=m.height;const ctx=canvas.getContext('2d');ctx.imageSmoothingQuality='high';ctx.drawImage(source,0,0,canvas.width,canvas.height);}canvas.toBlob(async function(blob){const data=await blob.arrayBuffer();parent.postMessage({sm:1,type:'qa_capture',req:m.req,data,width:canvas.width,height:canvas.height},'*',[data]);},'image/png');},true);
`)]);
        response.end(contents);
    }catch(e){response.statusCode=404;response.end();}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
fs.mkdirSync(fixture,{recursive:true});
const profile=fs.mkdtempSync(path.join(fixture,'profile-'));
const graphics=process.argv.includes('--canvas')?['--disable-gpu','--disable-webgl','--disable-webgl2']:process.argv.includes('--webgl')||process.argv.includes('--software')?['--use-gl=angle','--use-angle=swiftshader']:['--disable-gpu'];
const child=spawn(browser,['--headless=new','--no-first-run',...graphics,'--remote-debugging-port=0','--window-size=1000,650','--user-data-dir='+profile,'about:blank'],{windowsHide:true,stdio:['ignore','ignore','pipe']});
let socket,closeBrowser,failure;
try{
    const endpoint=await new Promise((resolve,reject)=>{
        let output='';const timeout=setTimeout(()=>reject(new Error('Browser launch timed out')),20000);
        child.stderr.on('data',bytes=>{output+=bytes;const match=/DevTools listening on (ws:\/\/\S+)/.exec(output);if(match){clearTimeout(timeout);resolve(match[1]);}});
        child.once('error',e=>{clearTimeout(timeout);reject(e);});
        child.once('exit',code=>{clearTimeout(timeout);reject(new Error('Browser exited '+code+': '+output.slice(-1200)));});
    });
    const target=await fetch('http://'+new URL(endpoint).host+'/json/new?about:blank',{method:'PUT'}).then(r=>r.json());
    socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true});});
    let id=0,pageSession;const pending=new Map();
    socket.addEventListener('message',event=>{const reply=JSON.parse(event.data),entry=pending.get(reply.id);if(!entry)return;pending.delete(reply.id);clearTimeout(entry.timer);reply.error?entry.reject(new Error(JSON.stringify(reply.error))):entry.resolve(reply.result);});
    const call=(method,params={})=>new Promise((resolve,reject)=>{const next=++id,timer=setTimeout(()=>reject(new Error(method+' timed out')),45000);pending.set(next,{resolve,reject,timer});const message={id:next,method,params};if(pageSession&&!/^(?:Browser|SystemInfo|Target)\./.test(method))message.sessionId=pageSession;socket.send(JSON.stringify(message));});
    closeBrowser=()=>call('Browser.close');
    pageSession=(await call('Target.attachToTarget',{targetId:target.id,flatten:true})).sessionId;
    const evaluate=async expression=>{const result=await call('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});if(result.exceptionDetails)throw new Error(JSON.stringify(result.exceptionDetails));return result.result.value;};
    await call('Page.enable');await call('Runtime.enable');await call('Emulation.setDeviceMetricsOverride',{width:1000,height:650,deviceScaleFactor:1,mobile:false});
    await call('Page.bringToFront');
    const address='http://127.0.0.1:'+server.address().port;
    const wait=frame=>evaluate(`(async()=>{for(let i=0;i<600;i++){if(window.fixture?.errors.length)throw new Error(JSON.stringify(fixture));if(window.fixture?.frame===${frame})return fixture;await new Promise(r=>setTimeout(r,50));}throw new Error(JSON.stringify(window.fixture));})()`);
    const seekOnly=frame=>evaluate(`new Promise((resolve,reject)=>{const timer=setTimeout(()=>{window.removeEventListener('message',on);reject(new Error('Seek timed out'));},30000);const on=e=>{if(e.source!==document.querySelector('iframe').contentWindow)return;const m=e.data;if(m?.type==='error'||m?.type==='time'&&m.frame===${frame}){clearTimeout(timer);window.removeEventListener('message',on);m.type==='error'?reject(new Error(JSON.stringify(m))):resolve();}};window.addEventListener('message',on);seek(${frame});})`);let captureRequest=0;
    const capture=async(frame,width=0,height=0)=>{
        await seekOnly(frame);const request=++captureRequest;
        return evaluate(`new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(new Error('Canvas capture timed out')),15000);const on=event=>{const m=event.data;if(event.source!==document.querySelector('iframe').contentWindow||m?.type!=='qa_capture'||m.req!==${request})return;clearTimeout(timer);window.removeEventListener('message',on);const reader=new FileReader();reader.onload=()=>resolve(reader.result.split(',')[1]);reader.onerror=reject;reader.readAsDataURL(new Blob([m.data],{type:'image/png'}));};window.addEventListener('message',on);document.querySelector('iframe').contentWindow.postMessage({qaCapture:true,req:${request},width:${width},height:${height}},'*');})`);
    };
    const sameFrame=(actual,expected,label)=>{
        const a=decodePNG(Buffer.from(actual,'base64')),b=decodePNG(Buffer.from(expected,'base64'));
        if(a.width===b.width&&a.height===b.height&&a.pixels.equals(b.pixels))return;
        fs.writeFileSync(path.join(fixture,'parity-first.png'),Buffer.from(expected,'base64'));fs.writeFileSync(path.join(fixture,'parity-second.png'),Buffer.from(actual,'base64'));
        throw new Error(label+'; saved parity-first.png and parity-second.png');
    };
    if(matrix){
        const catalog=JSON.parse(fs.readFileSync(path.join(catalogDir,'catalog.json'))),directory=path.join(fixture,'rig-matrix');fs.mkdirSync(directory,{recursive:true});
        for(const character of catalog.entries.filter(e=>e.kind==='character'))for(const view of ['front','three_quarter','profile'])for(const facing of ['left','right']){
            await call('Page.navigate',{url:address+'/?matrix='+encodeURIComponent(character.id)+'&view='+view+'&facing='+facing});await wait(0);
            const states=await evaluate('fixture.states'),sheet={width:1536,height:Math.ceil(states.length/6)*256,pixels:Buffer.alloc(1536*Math.ceil(states.length/6)*256*4)};
            for(let i=0;i<states.length;i++){const bytes=await capture(i*60+18,256,256),png=decodePNG(Buffer.from(bytes,'base64'));for(let y=0;y<256;y++)png.pixels.copy(sheet.pixels,((Math.floor(i/6)*256+y)*sheet.width+(i%6)*256)*4,y*256*4,(y+1)*256*4);}
            fs.writeFileSync(path.join(directory,character.name+'-'+view+'-'+facing+'.png'),encodePNG(sheet));
            fs.writeFileSync(path.join(directory,character.name+'-'+view+'-'+facing+'.json'),JSON.stringify({states,story:await evaluate('fixture.story')},null,2)+'\n');
            console.log(JSON.stringify({character:character.id,view,facing,states:states.length,status:'passed'}));
        }
    }else if(environments){
        const catalog=JSON.parse(fs.readFileSync(path.join(catalogDir,'catalog.json'))),directory=path.join(fixture,'environments');fs.mkdirSync(directory,{recursive:true});
        for(const environment of catalog.entries.filter(e=>e.kind==='environment'))for(const variant of ['day','night'])for(const aspect of ['16:9','9:16','1:1']){
            await call('Page.navigate',{url:address+'/?environment='+encodeURIComponent(environment.id)+'&variant='+variant+'&aspect='+encodeURIComponent(aspect)});await wait(0);
            const png=Buffer.from(await capture(30),'base64'),size=decodePNG(png);assert.ok(png.length<Math.max(4*1048576,size.width*size.height*4*1.01+65536),'Export frame must fit the bounded PNG transport');
            fs.writeFileSync(path.join(directory,environment.name+'-'+variant+'-'+aspect.replace(':','x')+'.png'),png);console.log(JSON.stringify({environment:environment.id,variant,aspect,bytes:png.length,status:'passed'}));
        }
    }else if(previews){
        const catalog=JSON.parse(fs.readFileSync(path.join(catalogDir,'catalog.json'))),directory=reviewIndex>=0?path.resolve(process.argv[reviewIndex+1]):path.join(root,'assets/storymaker/masters/previews');fs.mkdirSync(directory,{recursive:true});
        for(const entry of catalog.entries.filter(e=>['character','pose','gesture','cycle','text_style','transition'].includes(e.kind))){
            await call('Page.navigate',{url:address+'/?preview='+encodeURIComponent(entry.id)});await wait(0);
            const png=await capture(await evaluate('fixture.previewFrame'),256,256);fs.writeFileSync(path.join(directory,entry.id+'.png'),Buffer.from(png,'base64'));console.log('Preview '+entry.id);
        }
        fs.writeFileSync(path.join(directory,'manifest.json'),JSON.stringify({schema:'storymaker-previews/1',renderer:'PixiJS 8.22.0',animation:'GSAP 3.15.1',source_catalog:catalog.version},null,2)+'\n');
    }else{
        for(const code of [false,true])for(const aspect of ['16:9','9:16','1:1']){
            await call('Page.navigate',{url:address+'/?aspect='+encodeURIComponent(aspect)+(code?'&code=1':'' )});await wait(0);
            const frame=code?45:60,first=await capture(frame);for(const n of code?[300,3,87,13]:[120,3,87,30,119,0])await seekOnly(n);const second=await capture(frame);sameFrame(second,first,'Seek/reload parity for '+aspect+'/'+code);
            if(aspect==='16:9'){await call('Page.reload',{ignoreCache:true});await wait(0);sameFrame(await capture(frame),first,'Fresh project parity');}
            const backend=baseline?'legacy':process.argv.includes('--webgl')?'webgl':'canvas';
            fs.writeFileSync(path.join(fixture,backend+'-'+(code?'code-':'reference-')+aspect.replace(':','x')+'.png'),Buffer.from(first,'base64'));
            const state=await evaluate('fixture');assert.ok(state.backends.length,'The renderer must be observed');
            if(baseline||process.argv.includes('--canvas')||process.argv.includes('--webgl')||process.argv.includes('--software'))assert.ok(state.backends.every(b=>b===backend),JSON.stringify(state));
            console.log(JSON.stringify({status:'passed',aspect,code,backend:state.backends[0],errors:state.errors}));
            if(process.argv.includes('--benchmark')&&!code&&aspect==='16:9'){
                const processes=(await call('SystemInfo.getProcessInfo')).processInfo.map(p=>Number(p.id)).filter(Number.isSafeInteger);let peak=0;
                const sample=()=>{try{let bytes=0;if(process.platform==='win32')bytes=Number(execFileSync('powershell.exe',['-NoProfile','-Command','(Get-Process -Id @('+processes.join(',')+') -ErrorAction SilentlyContinue | Measure-Object -Property WorkingSet64 -Sum).Sum'],{encoding:'utf8',windowsHide:true,timeout:5000}));else for(const pid of processes){const m=/VmRSS:\s+(\d+)/.exec(fs.readFileSync('/proc/'+pid+'/status','utf8'));bytes+=m?Number(m[1])*1024:0;}peak=Math.max(peak,bytes);}catch{}};
                sample();const sampler=setInterval(sample,500);let measured;
                try{measured=await evaluate(`(async()=>{const send=n=>new Promise(resolve=>{const on=e=>{if(e.source===document.querySelector('iframe').contentWindow&&e.data?.type==='time'&&e.data.frame===n){window.removeEventListener('message',on);resolve();}};window.addEventListener('message',on);seek(n);});const start=performance.now();for(let n=60;n<150;n++)await send(n);return{frames:90,seconds:(performance.now()-start)/1000};})()`);}finally{clearInterval(sampler);sample();}
                const mode=process.argv.includes('--software')?'software-fallback':backend;
                const metric={backend,mode,platform:process.platform,...measured,fps:measured.frames/measured.seconds,browser_peak_rss_mib:peak/1048576};fs.writeFileSync(path.join(fixture,'benchmark-'+process.platform+'-'+mode+'.json'),JSON.stringify(metric,null,2)+'\n');console.log(JSON.stringify(metric));
            }
        }
    }
}catch(error){failure=error;throw error;}finally{
    if(closeBrowser)await closeBrowser().catch(()=>{});
    socket?.close();await new Promise(resolve=>server.close(resolve));
    if(child.exitCode===null)await new Promise(resolve=>{const timer=setTimeout(()=>{child.kill();resolve();},5000);child.once('exit',()=>{clearTimeout(timer);resolve();});});
    assert.ok(path.resolve(profile).startsWith(path.resolve(fixture)+path.sep));
    try{fs.rmSync(profile,{recursive:true,force:true,maxRetries:20,retryDelay:200});}catch(error){if(!failure)throw error;console.warn('Profile cleanup failed: '+profile);}
}
