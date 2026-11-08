import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { jointSection } from '../../assets/storymaker/lib/raster-attachments.mjs';
import { decodePNG } from '../../assets/storymaker/lib/png.mjs';
import { BODIES } from '../../assets/storymaker/lib/dims.mjs';
import { readJSON, root, loadPlayer, realCatalog } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

t.check('sprite joint measurement skips empty transparent gutter rows',()=>{
    const bitmap={width:10,height:10,pixels:Buffer.alloc(400)};
    for(let y=3;y<9;y++)for(let x=2;x<8;x++)bitmap.pixels[(y*10+x)*4+3]=255;
    assert.deepEqual(jointSection(bitmap,[0,0,10,10],0.1),{center:0.5,width:0.6,fraction:0.35});
    assert.throws(()=>jointSection({...bitmap,pixels:Buffer.alloc(400)},[0,0,10,10],0.1),/Empty/);
});

const playerChecks=t.pending.slice();
t.check('every raster identity has aligned joints, matching knee widths and anatomically bounded hand swaps',async()=>{
    // PNG measurements must not block the event loop while worker-deadline checks are running.
    await Promise.all(playerChecks);
    const C=loadPlayer().StorymakerCore,catalog=realCatalog(),rig=readJSON('ui/img/storymaker/rig/human.json');
    const point=(m,x,y)=>[m[0]*x+m[2]*y+m[4],m[1]*x+m[3]*y+m[5]];
    const near=(actual,want,label)=>actual.forEach((v,i)=>assert.ok(Math.abs(v-want[i])<1e-7,label+': '+actual+' vs '+want));
    const opaque=(slot,world,p,bitmap)=>{
        const m=C.rasterRig.slotMatrix(world,slot),det=m[0]*m[3]-m[1]*m[2],x=p[0]-m[4],y=p[1]-m[5];
        const u=((m[3]*x-m[2]*y)/det-slot.box[0])/slot.box[2],v=((-m[1]*x+m[0]*y)/det-slot.box[1])/slot.box[3];
        if(u<0||u>=1||v<0||v>=1)return false;
        const r=slot.image.rect,px=r[0]+Math.floor(u*r[2]),py=r[1]+Math.floor(v*r[3]);
        return bitmap.pixels[(py*bitmap.width+px)*4+3]>=64;
    };
    for(const entry of catalog.entries.filter(e=>e.kind==='character')){
        const body=BODIES[entry.params.body],descriptor=readJSON('ui/img/storymaker/'+entry.file),images=new Map();
        const image=file=>{if(!images.has(file))images.set(file,decodePNG(fs.readFileSync(path.join(root,'ui/img/storymaker',file))));return images.get(file);};
        for(const [view,slots]of Object.entries(descriptor.views)){
            const byName=Object.fromEntries(slots.map(s=>[s.name,s])),index=name=>slots.findIndex(s=>s.name===name);
            assert.ok(index('upper_arm_r')>index('torso'),entry.id+'/'+view+' near shoulder was hidden');
            assert.equal(index('upper_arm_l')>index('torso'),view==='front',entry.id+'/'+view+' far arm ordering');
            for(const side of ['l','r'])for(const [upperName,lowerName]of [['upper_leg','lower_leg'],['upper_arm','lower_arm']]){
                const upper=byName[upperName+'_'+side],lower=byName[lowerName+'_'+side];
                const end=upperName==='upper_leg'?0.90:0.87,start=upperName==='upper_leg'?0.12:0.20;
                const a=jointSection(image(upper.image.file),upper.image.rect,start),b=jointSection(image(upper.image.file),upper.image.rect,end);
                const lowerStart=jointSection(image(lower.image.file),lower.image.rect,0.12);
                const lowerEnd=jointSection(image(lower.image.file),lower.image.rect,upperName==='upper_leg'?0.92:0.90);
                const mapped=(slot,section,m)=>point(C.rasterRig.slotMatrix(m,slot),slot.box[0]+section.center*slot.box[2],slot.box[1]+section.fraction*slot.box[3]);
                for(const offsets of [{},{[upper.bone]:-90,[lower.bone]:90},{[upper.bone]:-150,[lower.bone]:-45}]){
                    const worlds=C.rig.boneMatrices(rig,view,C.rig.bodyOf(rig,entry.params.body),offsets);
                    near(mapped(upper,a,worlds[upper.bone]),point(worlds[upper.bone],0,0),'proximal '+entry.id+'/'+view);
                    near(mapped(upper,b,worlds[upper.bone]),point(worlds[lower.bone],0,0),'distal '+entry.id+'/'+view);
                    near(mapped(lower,lowerStart,worlds[lower.bone]),point(worlds[lower.bone],0,0),'lower '+entry.id+'/'+view);
                    near(mapped(lower,lowerEnd,worlds[lower.bone]),point(worlds[lower.bone],0,upperName==='upper_leg'?205:135),'ankle/wrist '+entry.id+'/'+view);
                    const center=point(worlds[lower.bone],0,0),radius=b.width*upper.box[2]*0.2;
                    for(let angle=0;angle<32;angle++){
                        const p=[center[0]+Math.cos(angle*Math.PI/16)*radius,center[1]+Math.sin(angle*Math.PI/16)*radius];
                        assert.ok(opaque(upper,worlds[upper.bone],p,image(upper.image.file))||opaque(lower,worlds[lower.bone],p,image(lower.image.file)),'transparent joint gap '+entry.id+'/'+view+'/'+upperName+'/'+side);
                    }
                }
                assert.ok(Math.abs(b.width*upper.box[2]-lowerStart.width*lower.box[2])<1e-7,'joint width step '+entry.id+'/'+view);
                assert.ok(Math.abs(upper.shear)<1&&Math.abs(lower.shear)<1,'implausible shear '+entry.id+'/'+view);
            }
            for(const slot of slots.filter(s=>s.kind==='hand'))for(const [name,variant]of Object.entries(slot.variants).concat(Object.entries(slot.edgeVariants))){
                const section=jointSection(image(variant.file),variant.rect,0.08),box=variant.box;
                near([box[0]+section.center*box[2],box[1]+section.fraction*box[3]],[0,135],'hand wrist '+entry.id+'/'+view+'/'+name);
                assert.ok(box[3]<=body.forearm*2.2+1e-7,'oversized hand '+entry.id+'/'+view+'/'+name);
                assert.ok(Math.abs(box[2]/box[3]-variant.rect[2]/variant.rect[3])<1e-7,'stretched hand '+entry.id+'/'+view+'/'+name);
            }
        }
    }
});
