// Assemble catalog v2 from accepted bitmap masters and the existing motion/font/audio contracts.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { buildOutputs, versionOf } from './catalog.mjs';
import { FOREGROUND_SPOTS } from './environments.mjs';
import { decodePNG, encodePNG, cropPNG, cellBounds, cellContentBounds, cellRect, cellSubjectBounds, thumbnailPNG } from './png.mjs';
import { NAMES } from './names.mjs';
import { MODEL_CHARACTERS } from './model-characters.mjs';

const root=path.resolve(import.meta.dirname,'../../..');
const FRAME={w:1920,h:1080};
// The foreground atlas holds one near-camera accent per environment, painted into a cell of the 16:9 picture. An accent
// that reaches the cell edge belongs to that picture edge and has to bleed off it: the layer is anchored there, so its cut
// edge falls outside the frame instead of floating inside it with a visible seam. An accent that reaches both ends of an
// axis is scaled to cover the frame on that axis. 2.5 % of a cell counts as reaching its edge.
const FOREGROUND_EDGE=0.025;

// foregroundBox(cell, rect, content): the placement of a foreground accent in the design frame. The cell is mapped onto the
// frame, the drawn pixels are anchored to the frame edge they reach and the crop keeps its own margin.
function foregroundBox(cell,rect,content){
    const [cx,cy,cw,ch]=cell;
    const axis=(from,to,span,size)=>{
        const atFrom=from<=span*FOREGROUND_EDGE,atTo=to>=span*(1-FOREGROUND_EDGE);
        if(atFrom&&atTo){const scale=size/(to-from);return{scale,origin:-from*scale};}
        const scale=size/span;
        if(atFrom)return{scale,origin:-from*scale};
        if(atTo)return{scale,origin:size-to*scale};
        return{scale,origin:0};
    };
    const x=axis(content[0]-cx,content[2]-cx+1,cw,FRAME.w),y=axis(content[1]-cy,content[3]-cy+1,ch,FRAME.h);
    return [(rect[0]-cx)*x.scale+x.origin,(rect[1]-cy)*y.scale+y.origin,rect[2]*x.scale,rect[3]*y.scale];
}
export const PROP_PACKS={
    tech:['laptop','phone','monitor','server','router','cloud','database','tablet','keyboard','mouse'],
    security:['lock','key','shield','firewall','usb-stick','hard-drive','backup-box','fingerprint','camera','security-token'],
    office:['mail','document','coffee-cup','desk','chair','calendar','notebook','pen','folder','printer'],
    education:['book','backpack','blackboard','pencil','ruler','globe','microscope','test-tube','graduation-cap','atom'],
    health:['stethoscope','first-aid-kit','medicine','thermometer','heart','lungs','wheelchair','bandage','apple','water-bottle'],
    everyday:['plant','sofa','lamp','clock','shopping-bag','bicycle','house','car','recycling-bin','trash-bin'],
    nature:['sun','moon','tree','leaf','flower','water-drop','wind-turbine','solar-panel','battery','earth'],
    symbols:['check','cross','warning','idea','question','arrow-right','gear','chart-bar','chart-line','puzzle']
};
export const CHARACTER_IDS=MODEL_CHARACTERS.map(c=>c.id);
export const ENVIRONMENT_IDS=['plain-stage','office','home-office','abstract-tech','server-room','classroom','laboratory','workshop','medical-office','living-room','city-street','park'];
export function buildRasterOutputs({prototype=false}={}){
    const legacy=buildOutputs(),files=new Map(), images=new Map(),entries=[];
    const put=(name,value)=>files.set(name,Buffer.isBuffer(value)?value:JSON.stringify(value)+'\n');
    const source=rel=>path.join(root,'assets/storymaker/masters',rel);
    const master=rel=>fs.existsSync(source(rel.replace(/\.png$/,'-clean.png')))?rel.replace(/\.png$/,'-clean.png'):rel;
    function image(rel,delivered){
        if(images.has(delivered))return images.get(delivered);
        const bytes=fs.readFileSync(source(rel)),parsed=decodePNG(bytes);
        if(parsed.width>3840||parsed.height>2160)throw new Error(`Oversized master: ${rel}`);
        put(delivered,bytes);images.set(delivered,parsed);return parsed;
    }
    function thumb(id,bitmap,rect){
        const file=`thumbs/${id}.png`,preview=source(`previews/${id}.png`);
        const image=fs.existsSync(preview)?decodePNG(fs.readFileSync(preview)):cropPNG(bitmap,rect);
        put(file,encodePNG(thumbnailPNG(image)));return file;
    }
    const original=id=>legacy.manifest.entries.find(e=>e.id===id);
    function entry(id,kind,extra){
        const old=original(id),name=id.slice(id.indexOf('.')+1);
        const base=old||{id,kind,name:id,name_key:`storymaker.catalog.${id.replace('.','_')}`,tags:name.split('-'),usage_example:{}};
        return {...base,tags:[...new Set([...(base.tags||[]),...(NAMES[id]||[])])],...extra};
    }
    const rig={...legacy.rig,bodies:structuredClone(legacy.rig.bodies),variants:{},slots:[]};
    put('rig/human.json',rig); // Retained motion vocabulary and v1 project compatibility.
    for(const character of MODEL_CHARACTERS){
        const {id,aliases,tags}=character,model='models/'+id+'.glb',motions='models/'+id+'-motions.json';
        for(const file of [model,motions,'models/'+id+'-provenance.json'])put(file,fs.readFileSync(path.join(root,'assets/storymaker',file)));
        for(const name of ['NOTICE.txt','LICENSE-CC0.txt','LICENSE-MIT.txt'])put('models/'+id+'-'+name,fs.readFileSync(path.join(root,'ui/3d/pets',id,name)));
        const file='characters/'+id+'.json';
        put(file,{schema:'storymaker-character-3d/1',model,motions});
        const bitmap=decodePNG(fs.readFileSync(path.join(root,'ui/3d/pets',id,'thumbnail.png')));
        entries.push(entry('char.'+id,'character',{params:{...original('char.office-man-1').params},file,resources:[model,motions],aliases:aliases.map(a=>'char.'+a),tags:[...tags,...NAMES['char.'+id]],
            thumb:thumb('char.'+id,bitmap,[0,0,bitmap.width,bitmap.height]),usage_example:{character:'char.'+id}}));
    }
    for(const [pack,ids]of Object.entries(PROP_PACKS)){
        if(!fs.existsSync(source(`props/${pack}.png`))){if(prototype)continue;throw new Error(`Missing prop sheet: ${pack}`);}
        const texture=`textures/props/${pack}.png`,bitmap=image(`props/${pack}.png`,texture);
        if(bitmap.width>2048||bitmap.height>2048)throw new Error('Prop atlas exceeds 2048');
        ids.forEach((id,i)=>{
            const rect=cellBounds(bitmap,i,5,2),file=`props/${id}.json`;
            put(file,{viewbox:[0,0,rect[2],rect[3]],image:{file:texture,rect}});
            entries.push(entry('prop.'+id,'prop',{file,resources:[texture],thumb:thumb('prop.'+id,bitmap,rect),anchor:original('prop.'+id)?.anchor||{x:0.5,y:1},size:original('prop.'+id)?.size||{h:0.15},usage_example:{do:'show',item:'prop.'+id,at:'right',entry:'pop'}}));
        });
    }
    for(const id of ENVIRONMENT_IDS){
        if(!fs.existsSync(source(`environments/${id}-day.png`))){if(prototype)continue;throw new Error(`Missing environment: ${id}`);}
        const variants={},resources=[];let day;
        for(const variant of ['day','night']){
            const rel=`environments/${id}-${variant}.png`;
            if(!fs.existsSync(source(rel))&&!prototype)throw new Error(`Missing environment variant: ${rel}`);
            const file=`textures/environments/${id}-${variant}.png`,bitmap=image(fs.existsSync(source(rel))?rel:`environments/${id}-day.png`,file);
            if(variant==='day')day=bitmap;
            variants[variant]=[{image:{file},box:[0,0,1920,1080],parallax:0.65}];resources.push(file);
            const foreground=master(`environments/foreground-${variant}.png`);
            if(fs.existsSync(source(foreground))){
                const nearFile=`textures/environments/foreground-${variant}.png`,near=image(foreground,nearFile);
                const index=ENVIRONMENT_IDS.indexOf(id),cell=cellRect(near,index,3,4),rect=cellBounds(near,index,3,4);
                const content=cellContentBounds(near,index,3,4);
                variants[variant].push({image:{file:nearFile,rect},box:foregroundBox(cell,rect,content),parallax:1.1,foreground:true});
                resources.push(nearFile);
            }else if(!prototype)throw new Error('Missing foreground production layer');
        }
        const old=original('env.'+id)||original('env.plain-stage');
        const baseTokens=JSON.parse(legacy.files.get(old.file)).tokens;
        const tokens={day:{text:'#1f2a44',bg:'#fbf3e4',bg2:'#e8dcc4',...baseTokens.day},night:{text:'#f4f5eb',bg:'#152232',bg2:'#263749',...baseTokens.night}};
        // The raster-only environments have no metadata of their own: they take the base spots and the board of the plain
        // stage. The near-camera foreground layer of an environment moves its left pair of spots to the right of itself.
        const spots={...old.spots,...(FOREGROUND_SPOTS[id]||{})};
        const file=`environments/${id}.json`;put(file,{viewbox:[0,0,1920,1080],variants,tokens});
        entries.push(entry('env.'+id,'environment',{file,resources,thumb:thumb('env.'+id,day,[0,0,day.width,day.height]),spots,floor:old.floor,variants:['day','night'],aspects:old.aspects,usage_example:{environment:'env.'+id}}));
    }
    const reference=images.values().next().value;
    if(!reference)throw new Error('No completed character available');
    for(const old of legacy.manifest.entries){
        if(['character','prop','environment'].includes(old.kind))continue;
        const e={...old};
        if(e.kind==='text_style'&&e.name==='title')e.box={fill:'$bg2',radius:0.012,pad:0.012};
        if(e.thumb){
            const bitmap=decodePNG(fs.readFileSync(path.join(root,'ui/3d/pets/hoodie/thumbnail.png')));
            e.thumb=thumb(e.id,bitmap,[0,0,bitmap.width,bitmap.height]);
        }
        entries.push(e);
        if(e.kind==='sfx')for(const take of e.takes)put(take.file,legacy.files.get(take.file));
    }
    put('glyphs.json',Buffer.from(legacy.files.get('glyphs.json')));
    const manifest={...legacy.manifest,schema:'storymaker-catalog/2',entries,files:[]};
    for(const [file,value]of files){
        const bytes=Buffer.isBuffer(value)?value:Buffer.from(value),ext=path.extname(file),type=ext==='.png'?'image/png':ext==='.wav'?'audio/wav':ext==='.glb'?'model/gltf-binary':ext==='.txt'?'text/plain':'application/json';
        const meta={path:file,type,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex')};
        if(ext==='.png'){meta.width=bytes.readUInt32BE(16);meta.height=bytes.readUInt32BE(20);}
        manifest.files.push(meta);
    }
    manifest.files.sort((a,b)=>a.path.localeCompare(b.path,'en'));
    const total=manifest.files.reduce((n,f)=>n+f.bytes,0);
    if(total>256*1048576)throw new Error(`Delivered catalog exceeds 256 MiB: ${total}`);
    for(const file of manifest.files)if(file.bytes>8*1048576)throw new Error(`Delivery file exceeds 8 MiB: ${file.path}`);
    manifest.version=versionOf(files);put('catalog.json',manifest);
    return {files,manifest,entries,rig};
}
