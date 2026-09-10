import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import vm from 'node:vm';
const handlers = new Map(), calls = new Map();
let delayed = null, failOverview = false, interval = null;
const response = data => ({ ok: true, json: async () => data });
const fixture = {
    '/api/dashboard/overview': { agent: { busy: false }, integrations: { enabled: true, disabled: false, undecided: {} }, missions: { total: 0 } },
    '/api/dashboard/system': { cpu: { usage_percent: 12 }, memory: { used_percent: 24 }, uptime_seconds: 60 },
    '/api/dashboard/memory': { vectordb_entries: 0, core_memory_facts: 0, journal_entries: 0, notes_count: 0 },
    '/api/knowledge-graph/nodes?limit=300': { nodes: [{ id: 'n1', label: '<img onerror=attack()>', type: 'person' }] },
    '/api/knowledge-graph/edges?limit=500': { edges: [] },
};
const context = vm.createContext({
    window: { AuraSSE: {
        on(type, fn) { handlers.set(type, fn); },
        off(type, fn) { if (handlers.get(type) === fn) handlers.delete(type); },
    } },
    document: { hidden: false }, AbortController, Date, console,
    setTimeout, clearTimeout, setInterval(fn) { interval = fn; return 1; }, clearInterval() { interval = null; },
    fetch: async (path, options) => {
        calls.set(path, (calls.get(path) || 0) + 1);
        if (failOverview && path === '/api/dashboard/overview') return { ok: false };
        if (delayed && path === '/api/dashboard/system') return new Promise((resolve,reject) => {
            delayed.resolve=resolve; options.signal.addEventListener('abort', () => reject(Error('Abort')));
        });
        return response(fixture[path] || { items: [] });
    },
});
await vm.runInContext(await fs.readFile('ui/js/desktop/apps/sysworld-data.js', 'utf8'), context);
const api = context.window.SysWorld.data;
let snapshot;
const flush = async () => { for(let n=0;n<5;n++)await new Promise(resolve=>setImmediate(resolve)); };
const off1 = api.subscribe(value => { snapshot=value; }), off2 = api.subscribe(() => {});
await flush();
assert.equal(calls.get('/api/dashboard/system'),1,'Two windows must share OS bootstrap');
assert.ok(interval);
let entities=api.entities(snapshot,key=>key);
const entity=id=>entities.find(e=>e.id===id);
assert.equal(entity('integration:enabled').state,'configured');
assert.equal(entity('integration:disabled').state,'disabled');
assert.equal(entity('integration:undecided').state,'unknown');
assert.equal(entity('agent').state,'idle');
assert.equal(entity('memory').rows[0].value,0,'A measured zero must survive');
assert.equal(entity('infra').rows.find(r=>r.key==='sysworld.city.disk').value,undefined,'Missing disk is not zero');
assert.equal(entity('node:n1').label,'<img onerror=attack()>','Data remains plain text for the DOM renderer');
failOverview=true;api.refresh();await flush();
entities=api.entities(snapshot,key=>key);
assert.equal(entity('agent').stale,true,'Failed source retains a stale value');
assert.equal(entity('integration:disabled').state,'disabled');
handlers.get('tool_call_preview')({ tool_name:'read_file',arguments:'NEVER COPY ME',message:'NEVER COPY ME' });
assert.ok(!JSON.stringify(snapshot.events).includes('NEVER COPY ME'));
assert.equal(snapshot.events[0].district,'agent','Unknown destinations do not choose a random integration');
for(let i=0;i<100;i++)handlers.get('tool_call_preview')({tool_name:'t'+i});
assert.equal(snapshot.events.length,60,'Live feed is bounded');
// A late REST bootstrap cannot replace a newer SSE sample.
delayed={};api.refresh();await flush();
handlers.get('system_metrics')({cpu:{usage_percent:81},memory:{used_percent:42}});
delayed.resolve(response({cpu:{usage_percent:1}}));await flush();
assert.equal(api.normalizeSystemMetrics(snapshot.sources.system.data).cpu,81);
delayed={};api.refresh();await flush();
handlers.get('system_metrics')({cpu:{usage_percent:82}});
delayed.resolve({ok:false});await flush();
assert.equal(snapshot.sources.system.failed,false,'Late REST failure cannot stale a newer SSE sample');
assert.equal(api.normalizeSystemMetrics(snapshot.sources.system.data).cpu,82);
off1();assert.ok(handlers.size>0,'First close keeps the second window alive');
off2();assert.equal(handlers.size,0);assert.equal(interval,null);
assert.equal(Object.keys(snapshot.sources).length,0,'Last close releases retained data');
// All city labels exist and retain placeholders in each supported language.
const base=JSON.parse(await fs.readFile('ui/lang/desktop/en.json','utf8'));
const keys=Object.keys(base).filter(k=>k.startsWith('sysworld.city.'));
assert.equal(keys.length,51);
for(const lang of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']) {
    const data=JSON.parse(await fs.readFile('ui/lang/desktop/'+lang+'.json','utf8'));
    for(const key of keys) { assert.ok(data[key],lang+':'+key);
        assert.deepEqual((data[key].match(/{{\w+}}/g)||[]).sort(),(base[key].match(/{{\w+}}/g)||[]).sort(),key+' placeholders'); }
}
console.log('System World: truthful states, shared polling, stale retention, SSE ordering, cleanup and 16 locales passed.');

// Exercise the real exported geometry and resident transforms without decoding textures in Node.
const THREE = await import('three');
const { createCityLife } = await import('../ui/js/desktop/apps/sysworld-life.js');
const robotBytes = await fs.readFile('ui/3d/system-world/white-robot.glb');
const jsonLength = robotBytes.readUInt32LE(12);
const robotDoc = JSON.parse(robotBytes.subarray(20,20+jsonLength));
for (const key of ['images','textures','materials','samplers']) delete robotDoc[key];
for (const mesh of robotDoc.meshes) for (const primitive of mesh.primitives) delete primitive.material;
const jsonBytes = Buffer.from(JSON.stringify(robotDoc));
const padded = Buffer.concat([jsonBytes,Buffer.alloc((4-jsonBytes.length%4)%4,32)]);
const header = Buffer.from(robotBytes.subarray(0,20));
const geometryGLB = Buffer.concat([header,padded,robotBytes.subarray(20+jsonLength)]);
geometryGLB.writeUInt32LE(geometryGLB.length,8);geometryGLB.writeUInt32LE(padded.length,12);
const nativeFetch = globalThis.fetch;
globalThis.fetch = async () => new Response(geometryGLB);
const scene = new THREE.Scene(), life = createCityLife(scene,[],{robotURL:'robot-test.glb'});
try {
    for(let i=0;i<100&&life.stats().robots!==5&&!life.stats().robotError;i++) await new Promise(r=>setTimeout(r,10));
    assert.equal(life.stats().robots,5,'All five real robot meshes must load');
    const residents=scene.getObjectByName('city-life').children.filter(n=>n.name.startsWith('city-white-robot-'));
    for(let frame=0;frame<3600;frame++) {
        const before=residents.map(r=>r.position.clone());
        life.update(1/30,true);scene.updateMatrixWorld(true);
        residents.forEach((resident,i)=>{
            const travel=resident.position.clone().sub(before[i]).setY(0).normalize();
            // Four cardinal Blender renders establish +X as the exported mesh's face.
            const face=new THREE.Vector3(1,0,0).transformDirection(resident.children[0].children[0].matrixWorld).setY(0).normalize();
            assert.ok(face.dot(travel)>.9,`${resident.name} must face its travel direction (frame ${frame})`);
        });
    }
    console.log('System World: all five robot faces follow straight streets and rounded turns.');
} finally {life.dispose();globalThis.fetch=nativeFetch;}
