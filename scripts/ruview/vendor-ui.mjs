import { readFile, writeFile, readdir, mkdir, unlink } from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import {patchControls} from './patch-controls.mjs';

const revision = 'a71634ae8a606bba5463489450c69e8b42de114e';
const source = process.argv.find(x => x.startsWith('--source='))?.slice(9);
const target = 'ui/js/vendor/ruview';
const check = process.argv.includes('--check');
const digest = b => crypto.createHash('sha256').update(b).digest('hex');
function patch(text, before, after) {
  if (!text.includes(before)) throw Error('Pinned RuView adapter patch no longer matches');
  return text.replace(before, after);
}
if (check) {
  const manifest = JSON.parse(await readFile(`${target}/provenance.json`));
  if (manifest.revision !== revision) throw Error('RuView revision mismatch');
  for (const [name, hash] of Object.entries(manifest.files)) if (digest(await readFile(`${target}/${name}`)) !== hash) throw Error(`RuView asset mismatch: ${name}`);
  console.log(`Verified ${Object.keys(manifest.files).length} pinned RuView assets`);
} else {
  if (!source) throw Error('Pass --source=<extracted pinned source root>');
  if (execFileSync('git', ['-C',source,'rev-parse','HEAD'], {encoding:'utf8'}).trim() !== revision) throw Error('Unexpected RuView source revision');
  let previous={files:{}};try{previous=JSON.parse(await readFile(`${target}/provenance.json`));}catch(error){if(error.code!=='ENOENT')throw error;}
  const files = {};
  async function put(name, bytes) { if(/\.(js|css|html|json|svg|txt)$/.test(name))bytes=Buffer.from(bytes.toString('utf8').replaceAll('\r\n','\n'));await mkdir(path.dirname(`${target}/${name}`), { recursive: true }); await writeFile(`${target}/${name}`, bytes); files[name] = digest(bytes); }
  async function copy(dir, prefix = '') {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      if (entry.name.startsWith('.') || ['tests', 'mobile', 'node_modules', 'test-results', 'sw.js', 'package.json', 'package-lock.json'].includes(entry.name)) continue;
      const name = prefix + entry.name, full = path.join(dir, entry.name);
      if (entry.isDirectory()) { await copy(full, `${name}/`); continue; }
      if (!/\.(js|css|html|svg|png|ico|woff2?|jpg|json|wasm)$/.test(name) || /(?:test|spec)\.js$/.test(name)) continue;
      let bytes = await readFile(full);
      if (/\.(js|css|html)$/.test(name)) {
        let text = bytes.toString('utf8').replaceAll('\r\n','\n');
        text = text.replaceAll('https://unpkg.com/three@0.160.0/', './deps/three/').replaceAll('https://unpkg.com/stats.js@0.17.0/build/stats.min.js', './deps/stats.min.js').replaceAll('https://cdnjs.cloudflare.com/ajax/libs/three.js/r128/three.min.js', './deps/three-global.js');
        if (name.endsWith('.html')) {
          text = text.replace(/<img\b[^>]*\bsrc="https:[^"]*"[^>]*>/g, '');
          text = text.replace(/<head([^>]*)>/i, '<head$1><script src="./aurago-adapter.js"></script>');
        }
        if (name === 'app.js') text = text.replace("if ('serviceWorker' in navigator)", 'if (false)').replace('const useMock = await backendDetector.shouldUseMockServer();','const useMock = false;');
        if (name === 'services/pose.service.js') {
          text=patch(text,`    // Use a lower confidence threshold when model inference is active
    const defaultThreshold = this.modelActive
      ? this.config.confidenceThresholdModelInference
      : this.config.confidenceThreshold;

    const params = {
      zone_ids: options.zoneIds?.join(','),
      min_confidence: options.minConfidence || defaultThreshold,
      max_fps: options.maxFps || 30,
      token: options.token || apiService.authToken
    };

    // Remove undefined values
    Object.keys(params).forEach(key =>${' '}
      params[key] === undefined && delete params[key]
    );`, `    // The pinned Rust pose stream has no query filters; AuraGo supplies authentication.
    const params = {};`);
        }
        if (name === 'services/sensing.service.js') {
          text = text.replace(/function clientSimulationAllowed\([^)]*\)\s*\{[\s\S]*?\n\}/, 'function clientSimulationAllowed() { return false; }');
          const start=text.indexOf('  async _detectServerSource() {'),end=text.indexOf('  /** @return {string|null} Raw server source');
          if(start<0||end<start)throw Error('Pinned sensing source detection changed');
          text=patch(text,text.slice(start,end),`  async _detectServerSource() {
    const epoch = this._connectEpoch, previous = this._lastMessage;
    const current = () => this._connectEpoch === epoch && this._state === 'connected' && this._lastMessage === previous;
    try {
      const resp = await fetch('/api/v1/status', { headers: apiService.getHeaders() });
      if (!resp.ok) throw new Error('Source status unavailable');
      const json = await resp.json();
      if (current()) this._applyServerSource(json.source, json.source_state || 'disconnected');
    } catch {
      if (current()) this._setDataSource('waiting');
    }
  }

  _applyServerSource(rawSource, sourceState) {
    this._serverSource = rawSource;
    if (sourceState) {
      const states = {live_verified:'live', live_unverified:'live', synthetic:'server-simulated', stale:'stale'};
      this._setDataSource(Object.hasOwn(states, sourceState) ? states[sourceState] : 'waiting');
      return;
    }
    // Only an arriving frame may establish live data without canonical status.
    const real = ['esp32', 'wifi', 'live'].includes(rawSource) || String(rawSource).startsWith('wifi:');
    this._setDataSource(real ? 'live' : ['simulated', 'simulate'].includes(rawSource) ? 'server-simulated' : 'waiting');
  }

`);
          text=patch(text,`      const raw = data.source;
      if (raw !== this._serverSource) {
        this._applyServerSource(raw);
      }`, `      this._applyServerSource(data._simulated ? 'simulated' : data.source, data._simulated ? 'synthetic' : data.source_state);`);
          text=patch(text,'  _setDataSource(source) {',`  _setDataSource(source) {
    clearTimeout(this._freshnessTimer);
    if (source === 'live' || source === 'server-simulated') {
      const epoch = this._connectEpoch;
      this._freshnessTimer = setTimeout(() => {
        if (epoch === this._connectEpoch && this._state === 'connected') this._setDataSource('stale');
      }, 10000);
    }`);
          text=patch(text,'  _clearTimers() {','  _clearTimers() {\n    clearTimeout(this._freshnessTimer);');
          text=patch(text,"'live'|'server-simulated'|'reconnecting'", "'live'|'server-simulated'|'waiting'|'stale'|'reconnecting'");
        }
        if (name === 'utils/data-source-banner.js') {
          text=patch(text,'const CONFIG = {',`const CONFIG = {
  waiting: {text: 'WAITING FOR SENSOR DATA — server connected, no current measurements', bg: '#33302a', fg: '#e8dcc0'},
  stale: {text: 'STALE SENSOR DATA — no fresh measurements received', bg: '#33302a', fg: '#e8dcc0'},`);
        }
        if (name === 'utils/connection-status.js') {
          text=patch(text,"      const label = source === 'live' ? 'Live' :",`      const label = source === 'waiting' ? 'Waiting for data' :
                    source === 'stale' ? 'Stale data' : source === 'live' ? 'Live' :`);
          text=patch(text,"      this.setStatus('connected', label);", "      this.setStatus(source === 'waiting' || source === 'stale' ? 'reconnecting' : 'connected', label);");
        }
        if (name === 'components/DashboardTab.js') {
          text=patch(text,"      'server-simulated':",`      'waiting': {text: 'NO DATA', status: 'degraded', msg: 'Server connected; waiting for sensor data'},
      'stale': {text: 'STALE', status: 'degraded', msg: 'No fresh sensor data'},
      'server-simulated':`);
        }
        if (name === 'components/SensingTab.js') {
          text=patch(text,"        'server-simulated':",`        'waiting': {text: 'WAITING FOR SENSOR DATA', cls: 'sensing-source-reconnecting'},
        'stale': {text: 'STALE SENSOR DATA', cls: 'sensing-source-reconnecting'},
        'server-simulated':`);
        }
        if (name === 'components/LiveDemoTab.js') {
          text=patch(text,"    if (ds === 'unreachable')",`    if (ds === 'waiting') return 'Waiting for sensor data';
    if (ds === 'stale') return 'No fresh sensor data';
    if (ds === 'unreachable')`);
          text=patch(text,"      'server-simulated':",`      'waiting': {text: 'WAITING FOR SENSOR DATA', cls: 'demo-source-reconnecting'},
      'stale': {text: 'STALE SENSOR DATA', cls: 'demo-source-reconnecting'},
      'server-simulated':`);
          text=patch(text,"        'server-simulated':",`        'waiting': 'Connected — waiting for sensor data',
        'stale': 'Connected — no fresh sensor data',
        'server-simulated':`);
        }
        if (name === 'pose-fusion/js/csi-simulator.js') {
          text=text.replace("return this.mode === 'live' && this.verifiedFrame === true;", "return this.mode === 'live' && this.verifiedFrame === true && performance.now()-this.auragoFrameAt<10000;");
          text=text.replace('this.verifiedFrame = true;', 'this.verifiedFrame = true; this.auragoFrameAt = performance.now();');
          text=text.replace('_handleJsonFrame(msg) {', "_handleJsonFrame(msg) { if(msg.source!=='esp32'||msg._simulated){this.verifiedFrame=false;this.mode='demo';return;}");
        }
        if (name === 'pose-fusion/js/main.js') {
          text=text.replace('if (isPaused) return;', `if (isPaused) return;
  if(!csiSimulator.isLive&&!window.auragoRuViewSimulationAllowed?.()){
    statusLabel.textContent='NO LIVE CSI';statusDot.classList.add('offline');
    for(const canvas of [skeletonCanvas,csiCanvas,embeddingCanvas])canvas.getContext('2d').clearRect(0,0,canvas.width,canvas.height);
    for(const el of [videoBarVal,csiBarVal,fusedBarVal,rssiValueEl,rssiQualityEl,crossModalEl])if(el)el.textContent='Unavailable';
    for(const bar of [videoBar,csiBar,fusedBar,rssiBarEl])if(bar)bar.style.width='0%';
    return;
  }`);
          text=text.replace("statusLabel.textContent = 'LIVE CSI';", "statusLabel.textContent = 'EXPERIMENTAL CSI';");
        }
        if (name === 'components/ModelPanel.js') {
          const start=text.indexOf('      st.innerHTML = '),end=text.indexOf('\n',start);
          if(start<0||end<0)throw Error('Pinned model statistics renderer changed');
          text=text.slice(0,start)+`      st.appendChild(this._el('span', 'mp-stat-label', 'Inference: '));
      st.appendChild(this._el('span', 'mp-stat-value', (Number.isFinite(am.avg_inference_ms) ? am.avg_inference_ms.toFixed(1) : '—') + ' ms'));
      st.appendChild(this._el('span', 'mp-stat-sep', ' | '));
      st.appendChild(this._el('span', 'mp-stat-label', 'Frames: '));
      st.appendChild(this._el('span', 'mp-stat-value', am.frames_processed ?? 0));`+text.slice(end);
        }
        if (name === 'observatory/js/main.js') {
          text=patch(text,'this._liveData = JSON.parse(evt.data);','this._liveData = JSON.parse(evt.data); this._auragoFrameAt = performance.now();');
          text=patch(text,"WebSocket closed, falling back to demo","WebSocket closed; waiting for live data");
          text=patch(text,"this.settings.dataSource = 'demo';","this._liveData = null; this.settings.dataSource = 'ws';");
          text=patch(text,`    if (this.settings.dataSource === 'ws' && this._liveData) {
      this._currentData = this._liveData;
    } else {
      this._currentData = this._demoData.update(dt);
    }`, `    const demoAllowed = window.auragoRuViewSimulationAllowed?.() === true;
    const frame = this._liveData;
    const real = frame && !frame._simulated && (frame.source === 'esp32' || frame.source === 'wifi' || String(frame.source).startsWith('wifi:'));
    if (this.settings.dataSource === 'ws' && frame && performance.now() - this._auragoFrameAt < 10000 && (real || demoAllowed)) {
      this._currentData = frame;
    } else if (demoAllowed) {
      this._currentData = this._demoData.update(dt);
    } else {
      this._currentData = null;
      this._renderer.clear();
      this._hud.updateHUD({}, this._demoData);
      this._hud.updateSourceBadge('unavailable', null);
      for (const id of ['hr-value','br-value','conf-value','rssi-value','var-value','motion-value','persons-value','presence-label']) {
        const el=document.getElementById(id);if(el)el.textContent='—';
      }
      return;
    }
    this._hud.updateSourceBadge(this._currentData === frame && real ? 'ws' : 'demo', this._ws);`);
        }
        if (name === 'observatory/js/hud-controller.js') {
          text=patch(text,"    if (dataSource === 'ws' && ws?.readyState === WebSocket.OPEN) {",`    if (dataSource === 'unavailable') {
      dot.className = 'dot'; label.textContent = 'NO LIVE DATA';
    } else if (dataSource === 'ws' && ws?.readyState === WebSocket.OPEN) {`);
        }
        if (name === 'viz.html') {
          text=patch(text,'    function mainUpdate(delta, elapsed) {',`    function mainUpdate(delta, elapsed) {
      const demoAllowed = window.auragoRuViewSimulationAllowed?.() === true;
      const fresh = Number.isFinite(state.auragoFrameAt) && performance.now() - state.auragoFrameAt < 10000;
      state.scene.renderer.domElement.style.visibility = demoAllowed || fresh ? '' : 'hidden';
      if (!demoAllowed && !fresh) {
        state.hud?.updateState({connectionStatus:'disconnected',isRealData:false,personCount:0,confidence:0,sensingMode:'Unavailable'});
        return;
      }`);
          text=patch(text,'      if (state.isDemoMode) {\n        // Generate demo data','      if (state.isDemoMode && demoAllowed) {\n        // Generate demo data');
          text=patch(text,'      if (!processed) return;',`      if (!processed?.metadata?.isRealData) return;
      state.auragoFrameAt = performance.now();`);
          text=patch(text,"      } else if (newState === 'error' || newState === 'disconnected') {",`      } else if (newState === 'error' || newState === 'disconnected') {
        state.auragoFrameAt = undefined;`);
        }
        bytes = Buffer.from(text);
      }
      if (/\.(js|html)$/.test(name)) bytes = Buffer.from(patchControls(name, bytes.toString('utf8'), patch));
      await put(name, bytes);
    }
  }
  await copy(path.join(source, 'ui'));
  for (const name of ['LICENSE', 'NOTICE']) { try { await put(`${name}.txt`, await readFile(path.join(source, name))); } catch (e) { if (name === 'LICENSE') throw e; } }
  await put('aurago-adapter.js', await readFile('ui/js/ruview/upstream-adapter.js'));
  await put('control-contracts.js', await readFile('ui/js/ruview/control-contracts.mjs'));
  await put('deps/three/build/three.module.js',await readFile('node_modules/three-ruview/build/three.module.js'));
  // Copy the five pinned entry points and their relative imports, not unused loaders/decoders.
  const seen=new Set();
  async function addon(name){
    if(name.startsWith('.')||!name.endsWith('.js'))throw Error('Invalid Three.js dependency');
    if(seen.has(name))return;seen.add(name);
    const bytes=await readFile('node_modules/three-ruview/examples/jsm/'+name);
    await put('deps/three/examples/jsm/'+name,bytes);
    for(const match of bytes.toString('utf8').matchAll(/(?:from|import)\s*['"](\.[^'"]+\.js)['"]/g))await addon(path.posix.normalize(path.posix.join(path.posix.dirname(name),match[1])));
  }
  for(const name of ['controls/OrbitControls.js','postprocessing/EffectComposer.js','postprocessing/RenderPass.js','postprocessing/UnrealBloomPass.js','postprocessing/ShaderPass.js'])await addon(name);
  await put('deps/three-global.js', await readFile('node_modules/three-ruview/build/three.min.js'));
  await put('deps/three/LICENSE.txt', await readFile('node_modules/three-ruview/LICENSE'));
  await put('deps/stats.min.js', await readFile('node_modules/stats-ruview/build/stats.min.js'));
  await put('deps/stats-LICENSE.txt', await readFile('node_modules/stats-ruview/LICENSE'));
  for(const name of Object.keys(previous.files)){if(name in files)continue;const old=path.resolve(target,name);if(!old.startsWith(path.resolve(target)+path.sep))throw Error('Invalid old manifest path');try{await unlink(old);}catch(error){if(error.code!=='ENOENT')throw error;}}
  await writeFile(`${target}/provenance.json`, JSON.stringify({revision, files},null,2)+'\n');
  console.log(`Vendored ${Object.keys(files).length} RuView assets`);
}
