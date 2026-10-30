// Execute the app-owned handoff callbacks with local exporter/download fixtures.
// Native binary encoders have their own suites; this matrix checks selection,
// current editor values, metadata, companion files and immutable revision URLs.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const root='ui/js/desktop/', life=new AbortController(), signal=life.signal;
const mime={txt:'text/plain',md:'text/markdown',html:'text/html',docx:'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  xlsx:'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',xlsm:'application/vnd.ms-excel.sheet.macroenabled.12',csv:'text/csv',
  png:'image/png',jpg:'image/jpeg',jpeg:'image/jpeg',webp:'image/webp',ora:'image/openraster',wav:'audio/wav',mid:'audio/midi',midi:'audio/midi',
  pdf:'application/pdf',json:'application/json',zip:'application/zip',epub:'application/epub+zip',stl:'model/stl',obj:'model/obj',glb:'model/gltf-binary',
  '3mf':'model/3mf',step:'application/step',lyl:'application/octet-stream',svg:'image/svg+xml',dxf:'application/dxf'};
const file=(name,content='current unsaved content',type=mime[name.split('.').pop()]||'application/octet-stream')=>new File([content],name,{type});
const formats=options=>Array.from(options.formats,item=>typeof item==='string'?item:item.id);
let cases=0, outputs=0;
function binding(name,env={}) {
  const source=fs.readFileSync(root+name,'utf8');
  const start=source.search(/(?:window\.AuraDesktopEmail\?\.attach|attachEmailExport)\(/);
  assert(start>=0,'no adapter in '+name);
  // Compile the actual call expression, including nested callbacks, rather than
  // copying app adapters into this test. Internal semicolons are not boundaries.
  let script;
  for(let end=source.indexOf(');',start);end>=0;end=source.indexOf(');',end+2)) {
    try { script=new vm.Script(source.slice(start,end+2));break; } catch(error) { if(!(error instanceof SyntaxError))throw error; }
  }
  assert(script,'unterminated adapter in '+name);
  let getOptions;
  const requests=[];
  const email={attach:(_id,_toolbar,get)=>{getOptions=get;},files:async entries=>{
    requests.push(...entries);return entries.map(entry=>{assert(entry.url&&entry.name,'missing download identity in '+name);return file(entry.name,entry.url);});
  }};
  const context={File,Blob,AbortController,signal,windowId:'fixture',id:'fixture',t:key=>key,tr:key=>key,ctx:{t:key=>key},
    attachEmailExport:email.attach,window:{AuraDesktopEmail:email},...env};
  context.window={AuraDesktopEmail:email,...env.window};
  script.runInNewContext(context);
  return {options:()=>getOptions(),requests};
}
async function check(name,env,expected,verify=()=>{}) {
  const item=binding(name,env),options=item.options();
  assert(!options.disabled,name+' unexpectedly disabled');
  assert.deepEqual(formats(options),expected,name+' format matrix drift');
  for(const format of expected) {
    const files=await options.createFiles(format,signal);
    assert(Array.isArray(files)&&files.length,name+' / '+format+' produced no files');
    for(const result of files)assert(result instanceof File&&result.name&&result.type&&result.size,name+' invalid file metadata');
    await verify(format,files,item.requests);outputs+=files.length;
  }
  cases++;console.log('ok '+name+': '+expected.join(', '));
}
const native=(extension)=>[file('Draft.'+extension)];
await check('apps/writer.js',{documentLife:life,basename:()=> 'Draft.docx',path:'Documents/Draft.docx',editor:{save:async()=>new Uint8Array([80,75,1,2])},
  loading:false,loadFailed:false,fileOperation:false,MIME:mime.docx,exportSimple:async(format,asFile,passed)=>{assert(asFile);assert.equal(passed,signal);return native(format);}},
  ['docx','html','md','txt'],async(format,files)=>{if(format==='docx')assert.deepEqual([...new Uint8Array(await files[0].arrayBuffer())],[80,75,1,2]);});
for(const path of ['Draft.xlsx','Draft.xlsm'])await check('apps/sheets.js',{documentLife:life,basename:p=>p,path,book:{},loading:false,loadingFailed:false,disposed:false,
  download:async(format,asFile,passed)=>{assert(asFile);assert.equal(passed,signal);return native(format);}},[path.split('.').pop(),'csv']);
await check('apps/notes.js',{docLife:life,current:{title:'Draft'},editor:{},busy:false,disposed:false,
  exportNote:async(format,asFile,passed)=>{assert(asFile);assert.equal(passed,signal);return native(format);}},['md','html','txt']);
await check('apps/editor-filemenu.js',{textarea:{value:'unsaved editor'},current:()=>true,loadFailed:false,saving:false,path:'Documents/source.txt',pathBaseName:()=> 'source.txt'},['source'],async(_format,files)=>assert.equal(await files[0].text(),'unsaved editor'));
await check('apps/code-studio/core.js',{instance:{openTabs:[{path:'src/main.go',content:'unsaved code'}],activeTabIndex:0},isLiveInstance:()=>true},['source'],async(_format,files)=>assert.equal(await files[0].text(),'unsaved code'));
await check('apps/cheater.js',{state:{sheet:{id:'one',name:'Commands',content:'saved source'}},host:{querySelector:()=>({value:'# Unsaved Markdown'})}},['md'],async(_format,files)=>assert.equal(await files[0].text(),'# Unsaved Markdown'));
await check('apps/pixel-email.js',{r:{windowId:'fixture',history:[{}],historyIdx:0,imgWidth:2,imgHeight:2,layers:[{}],fileName:'Layers.ora',ctx:{promptDialog:async()=> '88'},t:key=>key,
  oraProjectBlob:async()=>new Blob(['PK native layers']),flattenDocument:surface=>surface},Pixel:{acquireTempCanvas:()=>({toBlob:(done,type,quality)=>{assert(type==='image/png'||quality===.88);done(new Blob(['unsaved pixels'],{type}));}}),releaseTempCanvas:()=>{}}},
  ['png','jpeg','webp','ora'],async(format,files)=>{assert.equal(files[0].name,'Layers.'+(format==='jpeg'?'jpg':format));assert.equal(await files[0].text(),format==='ora'?'PK native layers':'unsaved pixels');});
for(const clip of [false,true])await check('apps/camera.js',{state:{mediaToken:1,videoBlob:new Blob(['recorded clip'],{type:'video/webm'}),capturedDataURL:'data:image/jpeg;base64,AA=='},
  isPreview:()=>true,isClipPreview:()=>clip,videoFileType:()=>({ext:'.webm'}),dataURLtoBlob:()=>new Blob(['current photo'],{type:'image/jpeg'}),timestampName:(kind,ext)=>kind+ext},[clip?'webm':'jpg']);
await check('apps/gallery.js',{targetItems:()=>[{id:'one',name:'a.jpg'},{id:'two',name:'b.webm'}],downloadURL:item=>'/files/'+item.name},['original'],(_format,files)=>assert.equal(files.length,2));
await check('apps/network-cameras.js',{state:{},selectedStream:()=>({id:'camera-one',name:'Door'}),text:()=>'',fetch:async(url,options)=>{
  assert.equal(url,'/api/desktop/integrations/go2rtc/snapshot');assert.deepEqual(JSON.parse(options.body),{stream_id:'camera-one',store:true});assert.equal(options.signal,signal);
  return {ok:true,json:async()=>({web_path:'/files/snapshot.jpg'})};}},['jpg']);
await check('apps/sysworld.js',{root:{},inst:{city:{capture:async()=>new Blob(['current scene'],{type:'image/png'})},load:life,mode:'orbit'}},['png']);
await check('shared/qr-dialog.js',{options:{windowId:'fixture'},node:{},close:()=>{},revision:9,batchResult:{errors:[]},batchRevision:9,previewRevision:9,preview:new Blob(['verified PNG'],{type:'image/png'}),
  current:()=>true,busy:false,resourceBusy:false,catalog:{output_formats:['png','jpeg','svg','ascii','unicode','unicode_half','txt','markdown']},spec:{output:{}},clone:structuredClone,
  batch:async(dry,asFile,passed)=>{assert.equal(dry,false);assert(asFile);assert.equal(passed,signal);return [file('QR.zip')];},
  renderBinary:async snapshot=>({blob:new Blob(['current QR '+snapshot.output.format],{type:mime[snapshot.output.format]||'text/plain'})})},['png','jpeg','svg','ascii','unicode','unicode_half','txt','markdown','zip']);
await check('apps/synth-studio.js',{currentProject:{name:'Arrangement'},project:()=>({name:'Arrangement',notes:['current notes']}),blocked:()=>false,
  M:{clone:structuredClone,validate:value=>value},exportFile:async(format,asFile)=>{assert(asFile);return [file('Arrangement.'+(format==='midi'?'mid':format))];}},['wav','midi','project']);
await check('apps/noisemaker.js',{S:{t:key=>key},menuTargets:()=>[{id:'one',web_path:'/files/track-one.wav'},{id:'two',web_path:'/files/track-two.mp3'}]},['original'],(_format,files)=>assert.equal(files.length,2));
await check('apps/rtl-sdr.js',{s:{id:'fixture',abort:life},box:{querySelector:()=>({getAttribute:()=>'/api/desktop/rtl-sdr/recordings/one/audio'})},audio:'/api/desktop/rtl-sdr/recordings/one/audio',r:{name:'Radio'},id:'one'},['wav']);
await check('apps/video-studio-jobs.js',{s:{id:'fixture',projectEpoch:8,abort:life,project:{name:'Movie'}},visibleJobs:()=>[
  {id:'render-1',kind:'render',status:'succeeded',artifact:{download_url:'/api/desktop/video-studio/jobs/render-1/artifact'}},
  {id:'still-2',kind:'still',status:'succeeded',artifact:{download_url:'/api/desktop/video-studio/jobs/still-2/frame.png'}}],safeSameOrigin:()=>true,exportName:()=> 'Movie.mp4'},['render-1','still-2']);
await check('apps/openscad.js',{state:{result:{job_id:'revision-8',files:[{name:'model.stl',download_url:'/api/openscad/jobs/revision-8/model.stl'}]},editor:{getValue:()=> 'cube(8);'},source:'cube(1);'}},['source','model.stl'],async(format,files)=>{if(format==='source')assert.equal(await files[0].text(),'cube(8);');else assert((await files[0].text()).includes('revision-8'));});
const nasscad={ready:true,formats:['project','stl'],createFiles:async format=>[file('Scene.'+format)]};
await check('apps/nasscad.js',{frame:{contentWindow:{AuraNasscadEmail:nasscad}},state:{}},['project','stl']);
await check('apps/layerling.js',{s:{queue:{},path:'Documents/Shape.lyl',port:{},request:async(action,params)=>({bytes:new TextEncoder().encode('current '+action),name:'Shape.'+params.format,type:mime[params.format]})},life},['stl','obj','3mf','step','png','lyl']);
await check('apps/detective.js',{state:{current:{id:'case-1',title:'Report'}},chosenReport:()=>({revision:7}),controller:life,base:'/api/desktop/detective',v:{formats:()=>['md','docx','pdf'].map(id=>({id,label:id}))}},['md','docx','pdf'],(_format,_files,requests)=>assert(requests.at(-1).url.includes('revision=7')));
await check('apps/newspaper.js',{st:{edition:{id:'edition-2',revision:3,title:'Daily',local_date:'2026-10-30'}},controller:life,base:'/api/desktop/newspaper'},['pdf'],(_format,files,requests)=>{assert(files[0].name.endsWith('-r3.pdf'));assert(requests[0].url.includes('edition-2'));});
let flushed=false;
await check('apps/easydrag-editor.js',{app:{windowId:'fixture'},disposed:false,restoring:false,deleting:false,draftModel:{doc:{name:'Automation'}},ed:{flow:{id:'flow_one'},flushPendingEdits:()=>true,saver:{flush:async()=>{flushed=true;return true;}},api:{exportUrl:()=>{assert(flushed);return '/api/desktop/flows/flow_one/export';}}}},['json']);
await check('apps/game-maker-studio.js',{host:{},state:{project:{id:'game-one',current_revision:8,name:'Game'},api:{exportURL:id=>'/api/game-maker/projects/'+id+'/export'}},window:{GameMakerStudioPreview:{flush:async()=>true}}},['zip']);
await check('apps/cyd-creator-app.js',{host:{},inst:{projectId:'cyd-one',project:{slug:'Panel',name:'Panel'},flushAll:async()=>true},P:{api:{exportUrl:id=>'/api/desktop/cyd-creator/projects/'+id+'/export'}}},['zip']);
await check('file-manager/core-render.js',{instance:{},withInstance:(_inst,fn)=>fn(),getSelectedFiles:()=>[{type:'file',name:'a.txt',path:'Documents/a.txt'},{type:'file',name:'b.pdf',path:'Documents/b.pdf'},{type:'dir',name:'folder'}],isRemote:()=>false,isLiveInstance:()=>true},['original'],(_format,files)=>assert.equal(files.length,2));
await check('file-manager/core-render.js',{instance:{},withInstance:(_inst,fn)=>fn(),getSelectedFiles:()=>[{type:'file',name:'remote.pdf',path:'remote.pdf'}],isRemote:()=>true,isLiveInstance:()=>true,storageLocation:path=>({path}),storageURL:()=>'/api/desktop/storage/content?provider=webdav&path=remote.pdf'},['original'],(_format,_files,requests)=>assert(requests[0].url.endsWith('&download=1')));
for(const name of ['viewer.js','viewer-3d.js'])await check('apps/'+name,{currentPath:'Documents/model.glb',life,fileName:'model.glb',archiveEntry:'models/model.glb',fileUrl:'/api/desktop/archive/entry?path=Archive.zip&entry=models/model.glb',archiveEntryURL:()=>'/api/desktop/archive/entry?path=Archive.zip&entry=models/model.glb'},['original']);
await check('apps/zipper.js',{zipPath:'Documents/Archive.zip',entries:[{name:'a.txt'},{name:'b.pdf'},{name:'directory/',is_dir:true}],selected:new Set(['a.txt','b.pdf','directory/']),baseName:path=>path.split('/').pop(),archiveEntryURL:name=>'/api/desktop/archive/entry?path=Archive.zip&entry='+name},['archive','selection'],(format,files)=>assert.equal(files.length,format==='selection'?2:1));
await check('apps/bookshelf.js',{state:{selected:{id:'book-one',title:'Book',path:'Library/Book.pdf',format:'pdf',renditions:[{format:'epub',available:true}]}},abort:life,
  window:{BookshelfAPI:{create:(_ctx,passed)=>{assert.equal(passed,signal);return {prepare:async(id,format)=>({content_url:'/api/desktop/bookshelf/books/'+id+'/content?format='+format+'&ticket=fixture'})};}}}},['pdf','epub']);

console.log(`desktop-email adapters: ${cases} selection cases, ${outputs} file outputs passed`);
