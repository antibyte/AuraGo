import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { writeContract } from './layerling/contract.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const commit = '25f12513802e322cdccecaf138f874ff526f331d';
const source = path.join(root, 'disposable/_layerling/layerling-' + commit);
const output = path.join(root, 'ui/js/vendor/layerling');
const base = '/api/desktop/layerling/ui';
const hash = data => crypto.createHash('sha256').update(data).digest('hex');
const inputBytes = name => Buffer.from(fs.readFileSync(path.join(root,name),'utf8').replaceAll('\r\n','\n'));
const files = dir => fs.readdirSync(dir, { withFileTypes: true }).flatMap(e => e.isDirectory() ? files(path.join(dir,e.name)) : [path.join(dir,e.name)]);
const inputs = ['scripts/build-layerling-vendor.mjs','scripts/layerling/bridge.ts','scripts/layerling/page.tsx','scripts/layerling/contract.mjs','scripts/layerling/README.md'];
if (process.argv.includes('--check')) {
  const manifest = JSON.parse(fs.readFileSync(path.join(output,'vendor.json')));
  if(manifest.commit !== commit) throw new Error('Wrong Layerling revision');
  for(const name of inputs) if(manifest.inputs?.[name] !== hash(inputBytes(name))) throw new Error('Layerling build input changed: '+name);
  if(JSON.stringify(files(output).map(f=>path.relative(output,f).replaceAll('\\','/')).filter(f=>f!=='vendor.json').sort()) !== JSON.stringify(Object.keys(manifest.files).sort())) throw new Error('Layerling resource list changed');
  for(const [name,digest] of Object.entries(manifest.files)) if(hash(fs.readFileSync(path.join(output,name))) !== digest) throw new Error('Layerling checksum mismatch: '+name);
  console.log('Layerling vendor checksums verified');
  process.exit(0);
}
if(process.version !== 'v24.15.0') throw new Error('Use pinned Node 24.15.0 for the vendor build');
if (!fs.existsSync(source)) throw new Error('Extract the pinned upstream archive into '+source+'; see scripts/layerling/README.md');
const archive = path.join(root,'disposable/_layerling/upstream.zip');
if(hash(fs.readFileSync(archive)) !== '9d81fcb78d13afb07511ec75279c341cce87c4ec7efa10ed4cc6a972c43bc5e8') throw new Error('Upstream archive checksum mismatch');
const { unzipSync, zipSync } = createRequire(path.join(source,'package.json'))('fflate');
const reset = ['apps/web/next.config.ts','apps/web/src/components/LayerlingEditor.tsx','apps/web/src/app/layout.tsx','apps/web/src/components/AppFooter.tsx','scripts/build-guide.mjs','scripts/verify-static-worker-assets.mjs','tests/unit/cadModifierRuntime.test.ts','tests/unit/appTheme.test.ts'];
const originals = unzipSync(fs.readFileSync(archive), {filter:entry=>reset.some(name=>entry.name==='layerling-'+commit+'/'+name)});
const locked = unzipSync(fs.readFileSync(archive), {filter:entry=>entry.name==='layerling-'+commit+'/package-lock.json'});
if(!Buffer.from(locked['layerling-'+commit+'/package-lock.json']).equals(fs.readFileSync(path.join(source,'package-lock.json')))) throw new Error('Upstream lockfile changed');
for(const name of reset) fs.writeFileSync(path.join(source,name),originals['layerling-'+commit+'/'+name]);
function patch(file, before, after) {
  const name = path.join(source,file), text = fs.readFileSync(name,'utf8');
  if(text.includes(after)) return;
  if (!text.includes(before)) throw new Error('Patch anchor missing: '+file);
  fs.writeFileSync(name,text.replace(before,after));
}
patch('apps/web/next.config.ts','  devIndicators: false,',`  devIndicators: false,\n  basePath: "${base}",\n  experimental: { cpus: 2 },`);
fs.writeFileSync(path.join(source,'apps/web/src/app/page.tsx'),inputBytes('scripts/layerling/page.tsx'));
patch('apps/web/next.config.ts','  devIndicators: false,',`  devIndicators: false,\n  generateBuildId: async () => "aurago-${commit}",`);
fs.writeFileSync(path.join(source,'apps/web/src/lib/auragoBridge.ts'),inputBytes('scripts/layerling/bridge.ts'));
patch('apps/web/src/app/layout.tsx','<ServiceWorkerRegistration />','{/* AuraGo owns resource caching and offline behavior. */}');
patch('apps/web/src/app/layout.tsx','  manifest: "/manifest.webmanifest",','  // AuraGo owns installation and offline resources.');
patch('apps/web/src/app/layout.tsx','url: "/favicon.ico"',`url: "${base}/favicon.ico"`);
for(const variable of ['page','indexPage']) patch('scripts/build-guide.mjs',`, ${variable});`,`, ${variable}.replace(/((?:src|href)=")\\/(?!\\/)/g, '$1${base}/'));`);
patch('apps/web/src/components/AppFooter.tsx','${SOURCE_CODE_URL.replace(/\\/+$/, "")}', 'https://github.com/henmedia/layerling');
// Only the corresponding-source link uses the archive; project links retain their upstream destinations.
{
  const name=path.join(source,'apps/web/src/components/AppFooter.tsx');
  fs.writeFileSync(name,fs.readFileSync(name,'utf8').replaceAll('${SOURCE_CODE_URL.replace(/\\/+$/, "")}', 'https://github.com/henmedia/layerling'));
}
{
  const name=path.join(source,'scripts/verify-static-worker-assets.mjs');
  fs.writeFileSync(name,fs.readFileSync(name,'utf8').replaceAll('/_next/',base+'/_next/'));
}
const editor = 'apps/web/src/components/LayerlingEditor.tsx';
patch(editor,'    const localDevelopment = process.env.NODE_ENV', '    return; // AuraGo uses a document-bound channel, never the HTTP MCP bridge.\n    const localDevelopment = process.env.NODE_ENV');
patch(editor,'"use client";','"use client";\nimport { bindEditor, captureFile, deliverFile, isReadOnly, notify } from "@/lib/auragoBridge";');
patch(editor,'onPickFile={() => fileInputRef.current?.click()}', 'onPickFile={() => notify("import")}');
patch(editor,'onPickProjectFile={() => projectFileInputRef.current?.click()}', 'onPickProjectFile={() => notify("open")}');
patch(editor,'onPickInsertProjectFile={() => insertProjectFileInputRef.current?.click()}', 'onPickInsertProjectFile={() => notify("import")}');
patch(editor,'  triggerBrowserDownload(filename, content, type);','  await deliverFile(filename, new Blob([content], { type }));');
// Serializing a recovery draft is not a completed Desktop file save.
patch(editor,'        setNotice(t("status.savedProject"));','        setNotice(""); // AuraGo confirms the actual Desktop save.');
// Return the existing mesh-export promise so completion means the file was delivered.
{
  const name=path.join(source,editor), text=fs.readFileSync(name,'utf8');
  const start=text.indexOf('  const exportDesign ='), end=text.indexOf('  const exportStepDesign =',start);
  const section=text.slice(start,end);
  if((section.match(/void exportBodiesWithLooseHoles/g)||[]).length!==2) throw new Error('Export promise patch drift');
  fs.writeFileSync(name,text.slice(0,start)+section.replaceAll('void exportBodiesWithLooseHoles','return exportBodiesWithLooseHoles')+text.slice(end));
}
{
  const name = path.join(source,editor);
  fs.writeFileSync(name,fs.readFileSync(name,'utf8').replace(/async function downloadBlobFile\(filename: string, blob: Blob\) \{[\s\S]*?(?=function shapeAabb)/,
    'async function downloadBlobFile(filename: string, blob: Blob) { return deliverFile(filename, blob); }\n\n'));
}
patch(editor,'  importFilesRef.current = importFiles;',`  importFilesRef.current = importFiles;
  useEffect(() => bindEditor(async (action, params) => {
    if (action === "serialize") return captureFile(() => exportLylDesign(projectName, "unlimited"));
    if (action === "export_model") return captureFile(() => {
      const name = String(params.name || projectName);
      if (params.format === "step") return exportStepDesign(name);
      if (params.format === "png") return exportViewImage(name, { plate: true, transparent: false });
      if (!["stl", "obj", "3mf"].includes(params.format)) throw new Error("Unsupported export format");
      return exportDesign(params.format, name);
    });
    if (action === "import_model") {
      if (isReadOnly()) throw new Error("Desktop is read-only");
      const bytes = new Uint8Array(params.bytes); let binary = "";
      for (let i = 0; i < bytes.length; i += 8192) binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
      return executeMcpCommand({ id: crypto.randomUUID(), action: "import_file", params: { fileName: params.name, base64: btoa(binary) }, createdAt: Date.now() });
    }
    if (isReadOnly() && !["get_scene","list_objects","list_edges","inspect_errors","capture_image","estimate_print","list_custom_shapes","list_reference_points"].includes(action)) throw new Error("Desktop is read-only");
    return executeMcpCommand({ id: crypto.randomUUID(), action: action as LayerlingMcpCommand["action"], params, createdAt: Date.now() });
  }), [executeMcpCommand, exportLylDesign, exportDesign, exportStepDesign, exportViewImage, importFiles, projectName]);`);
for(const file of [...files(path.join(source,'apps/web/src')), ...reset.filter(f=>f.startsWith('tests/')).map(f=>path.join(source,f))].filter(f=>/\.(tsx?|css)$/.test(f))) {
  const text = fs.readFileSync(file,'utf8');
  fs.writeFileSync(file,text.replace(/(["'`])\/(assets|occt|fonts|guide|anleitung|manifold\.wasm)(?=[/"'`?])/g, '$1'+base+'/$2'));
}
function run(args) {
  const result = spawnSync(process.execPath,args,{cwd:source,stdio:'inherit',env:{...process.env,STATIC_EXPORT:'true',NEXT_TELEMETRY_DISABLED:'1',NEXT_PUBLIC_SOURCE_CODE_URL:base+'/source.zip'}});
  if(result.status !== 0) throw new Error('Layerling build failed: '+args.join(' '));
}
run(['scripts/copy-occt-wasm.mjs']);
run(['scripts/build-guide.mjs']);
for(const file of files(path.join(source,'apps/web/public')).filter(f=>f.endsWith('.html'))) {
  const text=fs.readFileSync(file,'utf8');
  fs.writeFileSync(file,text.replace(/((?:src|href)=")\/(?!\/|api\/desktop\/layerling\/ui\/)/g,'$1'+base+'/'));
}
run(['node_modules/next/dist/bin/next','build','apps/web']);
run(['scripts/verify-static-worker-assets.mjs']);
fs.mkdirSync(output,{recursive:true});
if(path.relative(root,output).replaceAll('\\','/') !== 'ui/js/vendor/layerling' || fs.lstatSync(output).isSymbolicLink()) throw new Error('Unsafe vendor output');
fs.rmSync(output,{recursive:true});
fs.mkdirSync(output,{recursive:true});
fs.cpSync(path.join(source,'apps/web/.next-export'),output,{recursive:true});
// Upstream's optional PHP storage endpoint and public-site sitemap are not browser resources.
for(const name of ['store.php','sitemap.xml']) fs.rmSync(path.join(output,name),{force:true});
fs.copyFileSync(path.join(source,'LICENSE'),path.join(output,'LICENSE.txt'));
const manifest = {version:'1.57.0',commit,base,node:'24.15.0',inputs:Object.fromEntries(inputs.map(name=>[name,hash(inputBytes(name))])),files:{}};
await writeContract(source,path.join(root,'internal/layerling'));
const corresponding = {};
function sourceFiles(dir) {
  for(const entry of fs.readdirSync(dir,{withFileTypes:true}).sort((a,b)=>a.name<b.name?-1:a.name>b.name?1:0)) {
    if (['node_modules','.git','.next','.next-export','.next-export-build','.next-dev','out'].includes(entry.name) || entry.name.endsWith('.tsbuildinfo')) continue;
    const name=path.join(dir,entry.name);
    if(entry.isDirectory()) sourceFiles(name);
    else corresponding['layerling/'+path.relative(source,name).replaceAll('\\','/')]=[fs.readFileSync(name),{mtime:new Date('1980-01-01T00:00:00Z')}];
  }
}
sourceFiles(source);
corresponding['AURAGO-BUILD.txt'] = new TextEncoder().encode('Corresponding source for Layerling 1.57.0 / '+commit+' with AuraGo patches.\nLicense: layerling/LICENSE.\nUse Node 24.15.0, cd layerling, npm ci.\nSet environment STATIC_EXPORT=true, NEXT_TELEMETRY_DISABLED=1, NEXT_PUBLIC_SOURCE_CODE_URL='+base+'/source.zip.\nThen run: node scripts/copy-occt-wasm.mjs; node scripts/build-guide.mjs; node node_modules/next/dist/bin/next build apps/web; node scripts/verify-static-worker-assets.mjs.\nOutput: layerling/apps/web/.next-export. The patched basePath is '+base+'. No Node server is needed at runtime.\nThe aurago/scripts directory contains the reproducible patch generator and its guide.\n');
for(const name of ['build-layerling-vendor.mjs','layerling/bridge.ts','layerling/page.tsx','layerling/contract.mjs','layerling/README.md']) corresponding['aurago/scripts/'+name]=[inputBytes('scripts/'+name),{mtime:new Date('1980-01-01T00:00:00Z')}];
fs.writeFileSync(path.join(output,'source.zip'),zipSync(corresponding,{level:6,mtime:new Date('1980-01-01T00:00:00Z')}));
for(const file of files(output)) if(!file.endsWith('vendor.json')) manifest.files[path.relative(output,file).replaceAll('\\','/')] = hash(fs.readFileSync(file));
fs.writeFileSync(path.join(output,'vendor.json'),JSON.stringify(manifest,null,2)+'\n');
console.log('Layerling static vendor build complete');
