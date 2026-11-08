// Assemble catalog v2 from accepted bitmap masters and the existing motion/font/audio contracts.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { buildOutputs, versionOf } from './catalog.mjs';
import { decodePNG, encodePNG, cropPNG, cellBounds, cellRect, cellSubjectBounds, thumbnailPNG } from './png.mjs';
import { BODIES } from './dims.mjs';
import { NAMES } from './names.mjs';
import { calibrateAttachments } from './raster-attachments.mjs';

const root=path.resolve(import.meta.dirname,'../../..');
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
export const CHARACTER_IDS=['office-woman-1','office-man-1','it-tech-1','senior-man-1','teacher-1','student-1','doctor-1','nurse-1','craftswoman-1','scientist-1','senior-woman-1','schoolboy-1'];
export const ENVIRONMENT_IDS=['plain-stage','office','home-office','abstract-tech','server-room','classroom','laboratory','workshop','medical-office','living-room','city-street','park'];
const VIEWS=['front','three_quarter','profile'];
const BODY_TYPES={'office-woman-1':'average','office-man-1':'broad','it-tech-1':'slim','senior-man-1':'elder','teacher-1':'average','student-1':'slim','doctor-1':'average','nurse-1':'broad','craftswoman-1':'broad','scientist-1':'slim','senior-woman-1':'elder','schoolboy-1':'child'};
const FACE_STATES=['neutral','happy','laugh','sad','angry','surprised','confused','thinking','worried','determined','mouth.ai','mouth.e','mouth.o','mouth.u','mouth.mbp','mouth.fv','blink','mouth.rest'];
const PARTS=[
    ['head','head',[-84,-136,168,178]],['torso','torso',[-100,-290,200,305]],['hips','hips',[-90,-42,180,110]],
    ['upper_arm_l','upper_arm_l',[-45,-25,90,190]],['lower_arm_l','lower_arm_l',[-34,-18,68,170]],
    ['upper_arm_r','upper_arm_r',[-45,-25,90,190]],['lower_arm_r','lower_arm_r',[-34,-18,68,170]],
    ['hand_l','lower_arm_l',[-33,120,66,104]],['hand_r','lower_arm_r',[-33,120,66,104]],
    ['upper_leg_l','upper_leg_l',[-48,-20,96,250]],['lower_leg_l','lower_leg_l',[-42,-18,84,240]],
    ['upper_leg_r','upper_leg_r',[-48,-20,96,250]],['lower_leg_r','lower_leg_r',[-42,-18,84,240]],
    ['shoe_l','lower_leg_l',[-40,175,126,50]],['shoe_r','lower_leg_r',[-40,175,126,50]]
];
const LEGS=['upper_leg_l','lower_leg_l','shoe_l','upper_leg_r','lower_leg_r','shoe_r','hips','torso','head'];
const ORDER={front:[...LEGS,'upper_arm_l','lower_arm_l','hand_l','upper_arm_r','lower_arm_r','hand_r'],angled:['upper_arm_l','lower_arm_l','hand_l',...LEGS,'upper_arm_r','lower_arm_r','hand_r']};
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
    const rig={...legacy.rig,variants:{},slots:[]};
    put('rig/human.json',rig);
    for(const id of CHARACTER_IDS){
        const available=VIEWS.every(view=>fs.existsSync(source(master(`characters/${id}-${view}.png`))));
        if(!available){if(prototype)continue;throw new Error(`Character production incomplete: ${id}`);}
        if(!prototype&&!fs.existsSync(source(`previews/char.${id}.png`)))throw new Error(`Missing rendered character preview: ${id}`);
        const views={},resources=[],seated={};
        const originalCharacter=original('char.'+id)||original('char.office-woman-1');
        const body=BODIES[BODY_TYPES[id]];
        const facesPath=master(`characters/${id}-faces.png`),handsPath=master(`characters/${id}-hands.png`);
        const faces=fs.existsSync(source(facesPath))?image(facesPath,`textures/characters/${id}-faces.png`):null;
        const hands=fs.existsSync(source(handsPath))?image(handsPath,`textures/characters/${id}-hands.png`):null;
        if(!prototype&&(!faces||!hands))throw new Error(`Facial/hand production incomplete: ${id}`);
        if(faces)resources.push(`textures/characters/${id}-faces.png`);
        if(hands)resources.push(`textures/characters/${id}-hands.png`);
        const seatedPath=master(`characters/${id}-seated.png`),seatedFile=`textures/characters/${id}-seated.png`;
        const bent=fs.existsSync(source(seatedPath))?image(seatedPath,seatedFile):null;
        if(bent)resources.push(seatedFile);else if(!prototype)throw new Error(`Seated production incomplete: ${id}`);
        let head;
        for(const view of VIEWS){
            const rel=master(`characters/${id}-${view}.png`),file=`textures/characters/${id}-${view}.png`;
            const bitmap=image(rel,file);if(bitmap.width>2048||bitmap.height>2048)throw new Error('Character atlas exceeds 2048');resources.push(file);
            const parts=PARTS.map(([name,bone,box],i)=>({name,bone,box:box.slice(),image:{file,rect:cellBounds(bitmap,i,4,4)}}));
            parts.filter(p=>p.name.startsWith('shoe')).forEach(p=>{p.pivot=[0,205];p.upright=true;p.box[1]-=205;});
            const torso=parts.find(p=>p.name==='torso');
            torso.box=[-(body.shoulderX+20),-body.torsoLen-35,(body.shoulderX+20)*2,body.torsoLen+50];
            parts.filter(p=>p.name.startsWith('upper_arm')).forEach(p=>{p.box[3]=body.upperArm+40;});
            parts.filter(p=>p.name.startsWith('upper_leg')).forEach(p=>{p.box[3]=body.upperLeg+35;});
            if(faces){
                const slot=parts[0];slot.kind='head';slot.variants={};
                // Exclude neighboring hair in uneven generated gutters while preserving the shared neck coordinate frame.
                FACE_STATES.forEach((state,i)=>{const index=VIEWS.indexOf(view)*18+i,frame=cellRect(faces,index,9,6),rect=cellSubjectBounds(faces,index,9,6),box=slot.box;
                    slot.variants[state]={file:`textures/characters/${id}-faces.png`,rect,box:[box[0]+(rect[0]-frame[0])/frame[2]*box[2],box[1]+(rect[1]-frame[1])/frame[3]*box[3],rect[2]/frame[2]*box[2],rect[3]/frame[3]*box[3]]};});
                slot.variants.default=slot.variants.neutral;
            }
            if(hands)for(const side of ['l','r']){
                const slot=parts.find(p=>p.name==='hand_'+side);slot.kind='hand';slot.side=side;slot.variants={};
                ['open','fist','point','hold'].forEach((state,i)=>{slot.variants[state]={file:`textures/characters/${id}-hands.png`,rect:cellSubjectBounds(hands,VIEWS.indexOf(view)*8+(side==='l'?0:4)+i,4,6),box:[-33,120,66,state==='fist'||state==='hold'?72:104]};});
            }
            if(hands)calibrateAttachments(parts,bitmap,hands,body,view);
            views[view]=ORDER[view==='front'?'front':'angled'].map(name=>parts.find(p=>p.name===name));
            if(bent){
                const width=view==='front'?body.hipHW*2.8:body.upperLeg+145;
                seated[view]={name:'seated_lower_body',bone:'hips',box:[view==='front'?-width/2:-90,-30,width,255],image:{file:seatedFile,rect:cellBounds(bent,VIEWS.indexOf(view),3,2)}};
            }
            if(view==='three_quarter')head={bitmap,rect:parts[0].image.rect};
        }
        const file=`characters/${id}.json`;put(file,{schema:'storymaker-character/1',views,seated});
        const old=originalCharacter;
        entries.push(entry('char.'+id,'character',{params:{...old.params,body:BODY_TYPES[id]},file,resources,thumb:thumb('char.'+id,head.bitmap,head.rect),usage_example:{character:'char.'+id}}));
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
                variants[variant].push({image:{file:nearFile,rect},box:[(rect[0]-cell[0])/cell[2]*1920,(rect[1]-cell[1])/cell[3]*1080,rect[2]/cell[2]*1920,rect[3]/cell[3]*1080],parallax:1.1,foreground:true});
                resources.push(nearFile);
            }else if(!prototype)throw new Error('Missing foreground production layer');
        }
        const old=original('env.'+id)||original('env.plain-stage');
        const baseTokens=JSON.parse(legacy.files.get(old.file)).tokens;
        const tokens={day:{text:'#1f2a44',bg:'#fbf3e4',bg2:'#e8dcc4',...baseTokens.day},night:{text:'#f4f5eb',bg:'#152232',bg2:'#263749',...baseTokens.night}};
        const file=`environments/${id}.json`;put(file,{viewbox:[0,0,1920,1080],variants,tokens});
        entries.push(entry('env.'+id,'environment',{file,resources,thumb:thumb('env.'+id,day,[0,0,day.width,day.height]),spots:old.spots,floor:old.floor,variants:['day','night'],aspects:old.aspects,usage_example:{environment:'env.'+id}}));
    }
    const reference=images.values().next().value;
    if(!reference)throw new Error('No completed character available');
    for(const old of legacy.manifest.entries){
        if(['character','prop','environment'].includes(old.kind))continue;
        const e={...old};
        if(e.kind==='text_style'&&e.name==='title')e.box={fill:'$bg2',radius:0.012,pad:0.012};
        if(e.thumb){
            if(e.kind==='expression'||e.kind==='mouth'){
                const face=images.get('textures/characters/office-woman-1-faces.png');
                if(face)e.thumb=thumb(e.id,face,cellSubjectBounds(face,18+FACE_STATES.indexOf(e.kind==='mouth'?'mouth.'+e.name:e.name),9,6));
                else e.thumb=thumb(e.id,reference,cellBounds(reference,0,4,4));
            }else{
                if(!prototype&&!fs.existsSync(source(`previews/${e.id}.png`)))throw new Error(`Missing rendered motion preview: ${e.id}`);
                e.thumb=thumb(e.id,reference,cellBounds(reference,0,4,4));
            }
        }
        entries.push(e);
        if(e.kind==='sfx')for(const take of e.takes)put(take.file,legacy.files.get(take.file));
    }
    put('glyphs.json',Buffer.from(legacy.files.get('glyphs.json')));
    const manifest={...legacy.manifest,schema:'storymaker-catalog/2',entries,files:[]};
    for(const [file,value]of files){
        const bytes=Buffer.isBuffer(value)?value:Buffer.from(value),ext=path.extname(file),type=ext==='.png'?'image/png':ext==='.wav'?'audio/wav':'application/json';
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
