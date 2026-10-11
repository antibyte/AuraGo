import { readFile, writeFile, readdir, mkdir, unlink } from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';

const revision = 'a71634ae8a606bba5463489450c69e8b42de114e';
const source = process.argv.find(x => x.startsWith('--source='))?.slice(9);
const target = 'ui/js/vendor/ruview';
const check = process.argv.includes('--check');
const digest = b => crypto.createHash('sha256').update(b).digest('hex');
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
      if (!/\.(js|css|html|svg|png|ico|woff2?|jpg|json)$/.test(name) || /(?:test|spec)\.js$/.test(name)) continue;
      let bytes = await readFile(full);
      if (/\.(js|css|html)$/.test(name)) {
        let text = bytes.toString('utf8');
        text = text.replaceAll('https://unpkg.com/three@0.160.0/', './deps/three/').replaceAll('https://unpkg.com/stats.js@0.17.0/build/stats.min.js', './deps/stats.min.js').replaceAll('https://cdnjs.cloudflare.com/ajax/libs/three.js/r128/three.min.js', './deps/three-global.js');
        if (name.endsWith('.html')) {
          text = text.replace(/<img\b[^>]*\bsrc="https:[^"]*"[^>]*>/g, '');
          text = text.replace(/<head([^>]*)>/i, '<head$1><script src="./aurago-adapter.js"></script>');
        }
        if (name === 'app.js') text = text.replace("if ('serviceWorker' in navigator)", 'if (false)');
        if (name === 'services/sensing.service.js') text = text.replace(/function clientSimulationAllowed\([^)]*\)\s*\{[\s\S]*?\n\}/, 'function clientSimulationAllowed() { return false; }');
        bytes = Buffer.from(text);
      }
      await put(name, bytes);
    }
  }
  await copy(path.join(source, 'ui'));
  for (const name of ['LICENSE', 'NOTICE']) { try { await put(`${name}.txt`, await readFile(path.join(source, name))); } catch (e) { if (name === 'LICENSE') throw e; } }
  await put('aurago-adapter.js', await readFile('ui/js/ruview/upstream-adapter.js'));
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
