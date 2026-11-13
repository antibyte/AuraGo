import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as THREE from 'three';
import { loadSkeleton, reset } from './pet-skeleton.mjs';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const pet=process.argv[2]?.replace(/^--pet=/,'');
assert.ok(process.argv.length===3&&['rexi','purple-tentacle'].includes(pet),'Choose a supported creature pet');
const out=path.join(root,'ui/3d/pets',pet),dinosaur=pet==='rexi';
const target=await loadSkeleton(path.join(out,'model.glb'));
const bones=new Map(target.skeleton.bones.map(b=>[b.name.replace(/[^a-z0-9]/gi,'').toLowerCase(),b]));
const bone=name=>{const value=bones.get(name.replace(/[^a-z0-9]/gi,'').toLowerCase());assert.ok(value,name);return value;};
assert.equal(bones.size,dinosaur?21:13);
const pelvis=bone('DEF-spine');
const restWorld=new Map(target.skeleton.bones.map(b=>[b,b.getWorldQuaternion(new THREE.Quaternion())]));
const meshes=[];target.gltf.scene.traverse(o=>{if(o.isSkinnedMesh)meshes.push(o);});
assert.equal(meshes.length,1);
const mesh=meshes[0],sole=[];
for(let i=0;i<mesh.geometry.attributes.position.count;i++)if(mesh.geometry.attributes.position.getY(i)<.07)sole.push(i);
assert.ok(sole.length>20);
const samples=sole.filter((_,i)=>i%Math.ceil(sole.length/64)===0);
const motions=JSON.parse(await readFile(path.join(root,'assets/desktop-pets/mesh2motion/source.json'),'utf8')).animations;
const clips=[];

function pose(id,u){
    const p=u*Math.PI*2,s=Math.sin(p),c=Math.cos(p),pulse=Math.sin(Math.PI*u);
    const rotations=new Map();let lift=0;
    const add=(name,x=0,y=0,z=0)=>{const item=rotations.get(name)||[0,0,0];rotations.set(name,[item[0]+x,item[1]+y,item[2]+z]);};
    const arms=(left,right)=>{add('DEF-upper_arm.L',0,0,left);add('DEF-upper_arm.R',0,0,-right);};
    const crouch=amount=>{
        if(dinosaur)for(const side of ['L','R']){add('DEF-thigh.'+side,amount);add('DEF-shin.'+side,-amount*1.5);add('DEF-foot.'+side,amount*.5);}
        else{add('DEF-spine.002',amount*.3);add('DEF-spine.003',amount*.25);add('DEF-spine.004',amount*.2);}
    };
    add('DEF-spine.003',.012*s,0,.012*c);add('DEF-spine.006',0,.045*s,0);
    if(dinosaur){for(let i=1;i<=4;i++)add('Tail.'+String(i).padStart(2,'0'),.018*Math.cos(p-i*.6),.045*Math.sin(p-i*.6),0);}
    else{add('DEF-spine.001',0,0,.018*s);add('DEF-spine.002',0,0,.018*Math.sin(p-.5));add('TentacleTip',.035*s,.025*c,.025*s);}
    switch(id){
        case 'idle-subtle':break;
        case 'idle-a':add('DEF-spine.003',0,.035*s,.015*c);arms(.025*s,-.025*s);break;
        case 'idle-listening':add('DEF-spine.006',-.04,.07*s,.035);break;
        case 'idle-foldarms':arms(-.10,-.10);add('DEF-forearm.L',0,-.25,0);add('DEF-forearm.R',0,.25,0);break;
        case 'idle-talking':arms(.08+.06*s,.08-.06*s);if(dinosaur)add('Jaw',.025*(1+c));else add('DEF-spine.003',.04*s);break;
        case 'tired-hunched':add('DEF-spine.003',.16);add('DEF-spine.004',.10);crouch(.12);break;
        case 'shiver':add('DEF-spine.002',0,.025*Math.sin(p*4),.018*Math.cos(p*4));arms(.06,.06);break;
        case 'greeting':arms(.4+.14*Math.sin(p*2),.08);add('DEF-forearm.L',0,.12*Math.sin(p*2),.12*s);break;
        case 'interact':arms(.13+.10*s,.13-.10*s);add('DEF-spine.003',.04, .04*s);break;
        case 'walk-female':case 'walk-happy':case 'jog':case 'run-female':{
            const speed=id==='jog'||id==='run-female'?1.5:1,amplitude=(id==='walk-happy'?.36:.25)*speed;
            if(dinosaur)for(const [side,sign] of [['L',1],['R',-1]]){
                const step=s*sign;add('DEF-thigh.'+side,amplitude*step);add('DEF-shin.'+side,-.24*Math.max(0,step));add('DEF-foot.'+side,-amplitude*step+.24*Math.max(0,step));
            }else{add('DEF-spine.001',0,.035*s,.055*s);add('DEF-spine.002',.03*c,0,-.025*s);}
            arms(.06+.08*s,.06-.08*s);add('DEF-spine.003',id==='run-female'?.12:.03);break;
        }
        case 'jump-2':lift=.24*pulse;crouch(.3*(1-pulse));arms(.35*pulse,.35*pulse);break;
        case 'victory-fist-pump':arms(.48+.12*s,.05);lift=.07*pulse;add('DEF-spine.006',-.05);break;
        case 'reject':add('DEF-spine.006',0,.20*Math.sin(p*2)*pulse,0);arms(.12,.12);break;
        case 'confused':add('DEF-spine.006',.03,.08*s,.14*pulse);add('DEF-forearm.L',.1,0,.1*pulse);break;
        case 'dance-simple':case 'dance-arm-pump':case 'dab':{
            const pump=id==='dance-arm-pump';add('DEF-spine.002',.04*c,.08*s,.09*s);arms(pump?.28+.17*s:.18+.15*s,pump?.28-.17*s:.18-.15*s);crouch(.12+.07*c);
            if(id==='dab'){arms(.30,-.12);add('DEF-spine.006',.14,0,-.10);}
            break;
        }
        case 'sit-ground-idle':crouch(.65);add('DEF-spine.003',-.08);arms(-.08,-.08);break;
        case 'sleeping':crouch(.42);add('DEF-spine.003',.18);add('DEF-spine.004',.25);add('DEF-spine.006',.12+.012*s);break;
        case 'head-nod':add('DEF-spine.006',.18*Math.sin(p)*pulse);break;
        case 'yes':add('DEF-spine.006',.12*Math.sin(p*2)*pulse);break;
        case 'clapping':arms(.22+.12*s,.22+.12*s);add('DEF-forearm.L',0,-.3+.12*s);add('DEF-forearm.R',0,.3-.12*s);break;
        case 'guitar-playing':arms(.13+.08*s,.08);add('DEF-forearm.R',.13*Math.sin(p*3));add('DEF-spine.003',0,0,.045*s);break;
        case 'fixing-kneeling':crouch(.4);add('DEF-spine.003',.14);arms(.16+.07*s,.16-.07*s);break;
        case 'ledge-hang':arms(.35,.35);crouch(.2);add('DEF-spine.006',-.05,.035*s);break;
        case 'freefall':arms(.48+.05*s,.48-.05*s);crouch(.3);add('DEF-spine.003',-.12);break;
        case 'jump-land':crouch(.40*pulse);add('DEF-spine.003',.08*pulse);arms(.10*pulse,.10*pulse);break;
        default:throw Error('Unmapped creature motion '+id);
    }
    return {rotations,lift};
}

for(const motion of motions){
    reset(target);
    const duration=Math.max(1.2,Math.min(4,motion.duration)),frames=Math.ceil(duration*24);
    const times=[],positions=[],values=new Map(target.skeleton.bones.map(b=>[b,[]]));
    for(let frame=0;frame<=frames;frame++){
        reset(target);
        const u=frame/frames,{rotations,lift}=pose(motion.id,u);
        for(const [name,angles] of rotations){
            const b=bone(name),world=restWorld.get(b);
            const delta=new THREE.Quaternion().setFromEuler(new THREE.Euler(...angles));
            b.quaternion.multiply(world.clone().invert().multiply(delta).multiply(world)).normalize();
        }
        target.gltf.scene.updateMatrixWorld(true);mesh.skeleton.update();
        let floor=Infinity;
        for(const i of samples){const v=new THREE.Vector3().fromBufferAttribute(mesh.geometry.attributes.position,i);mesh.applyBoneTransform(i,v).applyMatrix4(mesh.matrixWorld);floor=Math.min(floor,v.y);}
        assert.ok(Number.isFinite(floor));pelvis.position.y+=lift-floor;
        times.push(u*duration);positions.push(...pelvis.position.toArray());
        for(const b of target.skeleton.bones)values.get(b).push(...b.quaternion.toArray());
    }
    const tracks=target.skeleton.bones.map(b=>new THREE.QuaternionKeyframeTrack(b.name+'.quaternion',times,values.get(b)));
    tracks.push(new THREE.VectorKeyframeTrack(pelvis.name+'.position',times,positions));
    const clip=new THREE.AnimationClip(motion.id,duration,tracks).optimize();
    assert.equal(clip.tracks.length,bones.size+1);
    assert.ok(clip.tracks.every(t=>Array.from(t.values).every(Number.isFinite)));
    assert.ok(clip.tracks.some(t=>Array.from(t.values).some((v,i)=>i>=t.getValueSize()&&Math.abs(v-t.values[i%t.getValueSize()])>.0001)));
    clips.push({id:motion.id,name:motion.name,loop:motion.loop||motion.id.startsWith('idle-'),clip:THREE.AnimationClip.toJSON(clip)});
}
const code=await readFile(fileURLToPath(import.meta.url)),rig=await readFile(path.join(root,'assets/desktop-pets',pet,'rig.json'));
const sourceHash=createHash('sha256').update(code).update(rig).digest('hex');
const body=JSON.stringify({version:1,procedural:true,license:'GPL-3.0-only',repository:'AuraGo',source_sha256:sourceHash,model_sha256:target.hash,clips},(key,value)=>typeof value==='number'?Math.round(value*1e5)/1e5:value)+'\n';
assert.ok(Buffer.byteLength(body)<10*1024*1024);
await writeFile(path.join(out,'motions.json'),body);
await writeFile(path.join(out,'motions-manifest.json'),JSON.stringify({procedural:true,source_sha256:sourceHash,license:'GPL-3.0-only',clips:clips.map(({id,name,loop,clip})=>({id,name,loop,duration:clip.duration})),bytes:Buffer.byteLength(body),sha256:createHash('sha256').update(body).digest('hex')},null,2)+'\n');
console.log(`${pet}: ${clips.length} anatomical movements, ${Buffer.byteLength(body)} bytes`);
