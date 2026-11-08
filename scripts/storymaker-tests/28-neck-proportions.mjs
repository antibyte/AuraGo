import assert from 'node:assert/strict';
import fs from 'node:fs';
import {decodePNG} from '../../assets/storymaker/lib/png.mjs';
import {readJSON,realCatalog,root,loadPlayer} from '../storymaker-test-support.mjs';
import {t} from '../storymaker-test-context.mjs';

const earlier=t.pending.slice();
t.check('the rendered cast keeps opaque neck contact through facial swaps and has bounded arm reach',async()=>{
    await Promise.all(earlier);
    const C=loadPlayer().StorymakerCore,rig=readJSON('ui/img/storymaker/rig/human.json');
    const point=(m,x,y)=>[m[0]*x+m[2]*y+m[4],m[1]*x+m[3]*y+m[5]];
    const alpha=(slot,image,world,p,bitmap)=>{
        const m=C.rasterRig.slotMatrix(world,slot),det=m[0]*m[3]-m[1]*m[2],dx=p[0]-m[4],dy=p[1]-m[5];
        const b=image.box||slot.box,x=(m[3]*dx-m[2]*dy)/det,y=(-m[1]*dx+m[0]*dy)/det;
        const u=(x-b[0])/b[2],v=(y-b[1])/b[3],r=image.rect;
        return u>=0&&u<1&&v>=0&&v<1?bitmap.pixels[((r[1]+Math.floor(v*r[3]))*bitmap.width+r[0]+Math.floor(u*r[2]))*4+3]:0;
    };
    for(const entry of realCatalog().entries.filter(e=>e.kind==='character')){
        const descriptor=readJSON('ui/img/storymaker/'+entry.file),body=C.rig.bodyOf(rig,entry.params.body),images=new Map();
        const image=file=>{if(!images.has(file))images.set(file,decodePNG(fs.readFileSync(root+'/ui/img/storymaker/'+file)));return images.get(file);};
        for(const [view,slots]of Object.entries(descriptor.views)){
            const torso=slots.find(s=>s.name==='torso'),head=slots.find(s=>s.kind==='head');
            const worlds=C.rig.boneMatrices(rig,view,body,{}),points=[];
            for(const x of [-0.2,0,0.2].map(f=>f*torso.torsoCap.neck)){
                let y=torso.box[1];
                while(y<torso.torsoCap.y&&alpha(torso,torso.image,worlds.torso,point(worlds.torso,x,y),image(torso.image.file))<192)y++;
                assert.ok(y<torso.torsoCap.y,'No painted neck: '+entry.id+'/'+view);
                points.push(point(worlds.torso,x,y+4));
            }
            for(const tilt of [-12,-8,0,8,14]){
                const state=C.rig.boneMatrices(rig,view,body,{head:tilt});
                for(const [name,variant]of Object.entries(head.variants))for(const p of points){
                    assert.ok(alpha(head,variant,state.head,p,image(variant.file))>=64,'Transparent neck: '+entry.id+'/'+view+'/'+name+'/'+tilt);
                }
            }
            for(const side of ['l','r']){
                const upper=worlds['upper_arm_'+side],lower=worlds['lower_arm_'+side],length=body.bones['lower_arm_'+side].length;
                const wrist=point(lower,0,length),reach=Math.hypot(lower[4]-upper[4],lower[5]-upper[5]);
                assert.ok(reach/length>1.05&&reach/length<1.2,'Unbalanced arm segments: '+entry.id);
                assert.ok(wrist[1]-worlds.hips[5]>-20&&wrist[1]-worlds.hips[5]<35,'Wrist outside hip band: '+entry.id+'/'+view);
            }
        }
    }
});
