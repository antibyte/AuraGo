#!/usr/bin/env node
/*
 * System World sound survey (phase 5). Renders the real soundscape (ui/js/desktop/apps/sysworld-audio.js with all its
 * layers) for named listener situations through an OfflineAudioContext in headless Chrome and writes
 * reports/system-world-sound/<label>/<situation>.wav (32-bit float, or 24-bit PCM with --pcm24; stereo, 48 kHz) and
 * report.json with integrated loudness (BS.1770, gated), true peak (4x oversampled), sample peak and RMS per file, the
 * real-time factor of each render, the engine's stats, and the plan's checks.
 *
 *   node scripts/system-world-sound-survey.mjs --label <name> [--only a,b] [--situations <file>] [--tier low|medium|high|ultra]
 *        [--baseline <label>] [--calibrate] [--strict] [--pcm24] [--timeout 900] [--skip 2]
 *   --calibrate  renders a 1 kHz sine at -30 dBFS and at +6 dBFS through createLimiter() into calibration.json: the gain
 *                below the threshold must be 0 +- 0.25 dB, the peak above it under LIMITER.ceiling; suggestedTrim is
 *                the trim that would cancel the measured makeup gain.
 *   --pcm24      writes 24-bit integer PCM WAV files instead of 32-bit float (for tools that do not read float WAV).
 *   --tier       overrides every situation's tier (budget and real-time comparisons).
 *   --baseline   adds each situation's loudness change against reports/system-world-sound/<label>/report.json.
 *   --strict     exits 1 when a check fails: true peak above -1 dBTP; street-day and street-night-rain more than 2 LU
 *                apart; any situation more than 3 LU over the baseline; a shut lobby less than 3 LU under street-day.
 *   --skip       seconds dropped at the start of every file before measuring (the master fades in), default 2.
 *
 * The page (scripts/system-world-sound-survey.page.mjs) is bundled in memory with esbuild; nothing under ui/ changes.
 * Situation fields: name, seconds (30), seed, tier, volume (.18), listener {x,z,fx,fz[,y]} (y defaults to the ground
 * height + 2.4), origin [x,z] (acoustic origin; default the listener), walk [[x,z],...] with walkSpeed (m/s; visitor
 * steps every 1.8 m), mood {day,evening,busy,weather}, inside (the experience's indoor flag), door (openness returned
 * for every door key), seaTime and windOffset (clock starts), trams [{u,speed}] on the real route, walkers
 * [{id,from,to,speed,carrying,style}] (style "glide" = hover robot), companion (true: the quadruped trots its loop),
 * trace (true: the report lists the first tram's position and its distance to the listener once per second).
 */
import {build} from 'esbuild';
import fs from 'node:fs/promises';
import {existsSync} from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import {spawn,spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {readWav,measure} from './system-world-sound-meter.mjs';

const here=path.dirname(fileURLToPath(import.meta.url)),root=path.resolve(here,'..');
function parseArgs(argv){
  const o={label:null,only:null,situations:path.join(here,'system-world-sound-survey.situations.json'),tier:null,baseline:null,calibrate:false,strict:false,pcm24:false,timeout:900,skip:2};
  for(let i=0;i<argv.length;i++){const a=argv[i],next=()=>{if(i+1>=argv.length)throw Error('Missing value for '+a);return argv[++i];};
    if(a==='--label')o.label=next();else if(a==='--only')o.only=next().split(',').map(s=>s.trim()).filter(Boolean);else if(a==='--situations')o.situations=next();
    else if(a==='--tier')o.tier=next();else if(a==='--baseline')o.baseline=next();else if(a==='--calibrate')o.calibrate=true;else if(a==='--strict')o.strict=true;else if(a==='--pcm24')o.pcm24=true;
    else if(a==='--timeout')o.timeout=Number(next());else if(a==='--skip')o.skip=Number(next());else throw Error('Unknown argument '+a);}
  if(!o.label||!/^[a-z0-9-]+$/.test(o.label))throw Error('--label <name> ([a-z0-9-]) is required');
  if(o.tier&&!['low','medium','high','ultra'].includes(o.tier))throw Error('--tier must be low|medium|high|ultra');
  if(!(o.timeout>0)||!(o.skip>=0))throw Error('bad --timeout or --skip');return o;
}
function findChrome(){const found=[process.env.CHROME_PATH,'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe','C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe','C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe'].find(p=>p&&existsSync(p));if(!found)throw Error('No Chrome/Edge found; set CHROME_PATH');return found;}
function readBody(req,limit=96*1024*1024){return new Promise((resolve,reject)=>{const chunks=[];let size=0;
  req.on('data',c=>{size+=c.length;if(size>limit){reject(Error('body too large'));req.destroy();}else chunks.push(c);});req.on('end',()=>resolve(Buffer.concat(chunks)));req.on('error',reject);});}
const db=v=>Number.isFinite(v)?+v.toFixed(2):null;
function checks(rows,baseline){
  const by=new Map(rows.map(r=>[r.name,r])),out={},worst=rows.reduce((m,r)=>Math.max(m,r.truePeakDb??-Infinity),-Infinity);
  out.truePeak={pass:worst<=-1,worst:db(worst),limit:-1};
  const day=by.get('street-day'),rain=by.get('street-night-rain'),shut=by.get('lobby-agent-shut');
  if(day&&rain){const d=Math.abs(day.lufs-rain.lufs);out.dayNight={pass:d<=2,delta:db(d),limit:2};}
  if(day&&shut){const d=day.lufs-shut.lufs;out.lobby={pass:d>=3,below:db(d),limit:3};}
  if(baseline){const over=[];for(const r of rows){const b=baseline.situations?.find(s=>s.name===r.name);if(b&&Number.isFinite(b.lufs)&&Number.isFinite(r.lufs)){r.deltaLu=db(r.lufs-b.lufs);if(r.lufs-b.lufs>3)over.push(r.name);}}out.baseline={pass:!over.length,over,limit:3};}
  return out;
}
async function main(){
  const o=parseArgs(process.argv.slice(2)),outDir=path.join(root,'reports','system-world-sound',o.label);
  let situations=JSON.parse(await fs.readFile(path.resolve(root,o.situations),'utf8'));if(!Array.isArray(situations))throw Error('Situations file must be a JSON array');
  if(o.only){const missing=o.only.filter(n=>!situations.some(s=>s.name===n));if(missing.length)throw Error('Unknown situation(s): '+missing.join(', '));situations=situations.filter(s=>o.only.includes(s.name));}
  if(o.tier)situations=situations.map(s=>({...s,tier:o.tier}));
  await fs.mkdir(outDir,{recursive:true});
  console.log('building sound page bundle ...');
  const result=await build({entryPoints:[path.join(here,'system-world-sound-survey.page.mjs')],bundle:true,format:'esm',target:'es2022',write:false,absWorkingDir:root,logLevel:'warning',outdir:path.join(root,'.sound-survey-unused')});
  const page=Buffer.from(result.outputFiles.find(f=>f.path.endsWith('.js')).contents),config=Buffer.from(JSON.stringify({situations,calibrate:o.calibrate,pcm24:o.pcm24}));
  const html='<!doctype html><meta charset="utf-8"><title>System World sound survey</title><script type="module" src="/__sound/page.js"></script>';
  const stats=new Map();let report=null,calibration=null,finish;const done=new Promise(r=>{finish=r;});
  const server=http.createServer(async(req,res)=>{try{
    const url=new URL(req.url,'http://127.0.0.1'),send=(c,t,b)=>{res.writeHead(c,{'Content-Type':t,'Cache-Control':'no-store'});res.end(b);};
    if(req.method==='POST'){const body=await readBody(req),name=(url.searchParams.get('name')||'').toLowerCase().replace(/[^a-z0-9-]/g,'-').slice(0,80);
      if(url.pathname==='/__sound/wav'&&name){await fs.writeFile(path.join(outDir,name+'.wav'),body);console.log('saved '+name+'.wav ('+Math.round(body.length/1024)+' KB)');return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/stats'&&name){stats.set(name,JSON.parse(body.toString('utf8')));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/calibration'){calibration=JSON.parse(body.toString('utf8'));await fs.writeFile(path.join(outDir,'calibration.json'),JSON.stringify(calibration,null,2));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/report'){report=JSON.parse(body.toString('utf8'));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/log'){console.log('[page] '+body.toString('utf8'));return send(200,'text/plain','ok');}
      if(url.pathname==='/__sound/done'){send(200,'text/plain','ok');finish('done');return;}
      return send(404,'text/plain','not found');}
    if(url.pathname==='/__sound/index.html')return send(200,'text/html; charset=utf-8',html);
    if(url.pathname==='/__sound/page.js')return send(200,'text/javascript; charset=utf-8',page);
    if(url.pathname.startsWith('/__sound/audio/')){const file=decodeURIComponent(url.pathname.slice('/__sound/audio/'.length));
      if(!/^(manifest\.json|[a-z0-9-]+\.ogg)$/.test(file))return send(404,'text/plain','not found');
      try{return send(200,file.endsWith('.json')?'application/json':'audio/ogg',await fs.readFile(path.join(root,'ui','3d','system-world','audio',file)));}catch(_){return send(404,'text/plain','not found');}}
    if(url.pathname==='/__sound/config.json')return send(200,'application/json',config);
    return send(404,'text/plain','not found');
  }catch(e){console.error('server error: '+e.message);try{res.writeHead(500);res.end();}catch(_){}}});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  const pageUrl='http://127.0.0.1:'+server.address().port+'/__sound/index.html',profile=await fs.mkdtemp(path.join(os.tmpdir(),'sw-sound-'));
  const chrome=spawn(findChrome(),['--headless=new','--user-data-dir='+profile,'--no-first-run','--no-default-browser-check','--disable-extensions','--autoplay-policy=no-user-gesture-required',pageUrl],{stdio:'ignore',windowsHide:true});
  let spawnError=null;chrome.on('error',e=>{spawnError=e;finish('spawn-error');});chrome.on('exit',code=>{if(!report){console.error('Chrome exited early with code '+code);finish('chrome-exit');}});
  const timer=setTimeout(()=>finish('timeout'),o.timeout*1000),reason=await done;clearTimeout(timer);
  if(chrome.pid&&chrome.exitCode===null){if(process.platform==='win32')spawnSync('taskkill',['/PID',String(chrome.pid),'/T','/F'],{stdio:'ignore'});else try{chrome.kill('SIGKILL');}catch(_){}}
  server.closeAllConnections?.();server.close();
  for(let i=0;i<10;i++){try{await fs.rm(profile,{recursive:true,force:true});break;}catch(_){await new Promise(r=>setTimeout(r,300));}}
  if(spawnError){console.error('Could not start Chrome: '+spawnError.message);process.exit(1);}
  // Measured in Node from the files on disk: the same files the coordinator analyses.
  const rows=[];
  for(const s of situations){const file=path.join(outDir,s.name+'.wav');if(!existsSync(file))continue;
    const wav=readWav(await fs.readFile(file)),m=measure(wav.channels,wav.sampleRate,{skip:o.skip}),st=stats.get(s.name)||{};
    rows.push({name:s.name,file:s.name+'.wav',seconds:wav.channels[0].length/wav.sampleRate,tier:s.tier||'high',lufs:db(m.lufs),truePeakDb:db(m.truePeakDb),samplePeakDb:db(m.samplePeakDb),rmsDb:db(m.rmsDb),
      renderMs:st.renderMs,realtime:st.realtime,steps:st.steps,transientSamples:st.transientSamples,track:st.track?.length?st.track:undefined,recordings:st.sound?.soundscape?.layers?.recordings||null,stats:st.sound||null});}
  let base=null;if(o.baseline){try{base=JSON.parse(await fs.readFile(path.join(root,'reports','system-world-sound',o.baseline,'report.json'),'utf8'));}catch(_){console.warn('baseline '+o.baseline+' not found');}}
  const out={label:o.label,generatedAt:new Date().toISOString(),reason,skip:o.skip,wavFormat:o.pcm24?'pcm24':'float32',calibration,situations:rows,checks:checks(rows,base),errors:report?.errors||[]};
  await fs.writeFile(path.join(outDir,'report.json'),JSON.stringify(out,null,2));
  console.log('--- summary ---');
  for(const r of rows)console.log(r.name.padEnd(24)+String(r.lufs).padStart(8)+' LUFS  TP'+String(r.truePeakDb).padStart(7)+' dBTP'+(r.deltaLu!=null?'  delta '+r.deltaLu+' LU':'')+(r.realtime?'  x'+r.realtime+' real time':''));
  if(calibration)console.log('calibration: '+JSON.stringify(calibration));
  for(const [k,c]of Object.entries(out.checks))console.log('check '+k+': '+(c.pass?'pass':'FAIL')+' '+JSON.stringify(c));
  for(const e of out.errors)console.log('  error ['+e.where+'] '+String(e.message).split('\n')[0]);
  console.log('report: '+path.relative(root,path.join(outDir,'report.json')));
  const failed=Object.values(out.checks).some(c=>!c.pass);
  process.exit(reason!=='done'||out.errors.length||rows.length<situations.length||(o.strict&&failed)?1:0);
}
main().catch(e=>{console.error(e.stack||e.message||e);process.exit(1);});
