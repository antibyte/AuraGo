import assert from 'node:assert/strict';
import {loadPlayer,realCatalog,realFiles,readJSON,plain} from '../storymaker-test-support.mjs';
import {t} from '../storymaker-test-context.mjs';

t.check('raster gestures approach canonical arm targets from every base pose without doubling rotations',()=>{
    const C=loadPlayer().StorymakerCore,catalog=realCatalog(),files=realFiles();
    const built=C.compile({format:{aspect:'1:1',fps:30},cast:[{id:'a',character:'char.it-tech-1'}],scenes:[{id:'s',environment:'env.plain-stage',silent:true,duration:5,actors:[{cast:'a',at:'center'}]}]}, {},catalog);
    const sc=built.timeline.scenes[0],actor=sc.entities.find(e=>e.type==='actor'),rc=C.render.makeContext(built.timeline,files,1080,1080),stage=C.render.stageFor(sc,rc);
    const initial=C.scene.evalEntity(actor,1),cycle=C.rig.sampleKeys(rc.cat.cycles.idle.keys,C.rig.cyclePhase(rc.cat.cycles.idle,1+initial.phase)).bones;
    const sample=(name,gesture,frac)=>{
        const state={...initial,pose:{a:name,b:name,u:1},gesture:gesture?{name:gesture,frac}:null};
        const pose=C.render.rigState(state,1,rc,files.rig);C.rasterRig.adjust(pose,state,actor,1,rc,stage);return pose;
    };
    for(const pose of Object.keys(rc.cat.poses))for(const [name,gesture]of Object.entries(rc.cat.gestures)){
        const actual=sample(pose,name,0.5),target=C.rig.sampleKeys(gesture.keys,0.5).bones;
        for(const side of ['l','r']){
            const upper='upper_arm_'+side,lower='lower_arm_'+side;
            if(!gesture.keys.some(k=>k.bones&&(k.bones[upper]||k.bones[lower])||k.hands&&k.hands[side]))continue;
            for(const bone of [upper,lower])assert.ok(Math.abs(actual.offsets[bone]-((target[bone]||0)+(cycle[bone]||0)))<1e-8,pose+'/'+name+'/'+bone+' doubled the pose');
        }
        const base=sample(pose,null,0);
        for(const frac of [0,1]){
            const restored=sample(pose,name,frac);
            for(const bone of files.rig.bones.map(b=>b.name))assert.ok(Math.abs((restored.offsets[bone]||0)-(base.offsets[bone]||0))<1e-8,pose+'/'+name+' did not return to its base');
        }
    }
    const first=plain(sample('present','present',0.4));
    for(const frac of [1,0.2,0.9,0,0.4])sample('present','present',frac);
    assert.deepEqual(plain(sample('present','present',0.4)),first,'reverse/random seeking changed the pose');
    assert.equal(sample('present','present',0.5).handViews.r,'edge');
    assert.equal(sample('present','point',0.5).handViews.r,'edge');
    assert.equal(sample('present','wave',0.5).handViews.r,'palm');
    assert.equal(sample('present','wave',0.5).handViews.l,'edge');
});

t.check('raster cast has rounded shoulder caps and alternate hand orientations; reported identity uses authored hand backs',()=>{
    for(const character of realCatalog().entries.filter(e=>e.kind==='character')){
        const descriptor=readJSON('ui/img/storymaker/'+character.file);
        for(const slots of Object.values(descriptor.views)){
            const torso=slots.find(s=>s.name==='torso');assert.ok(torso.torsoCap.radius>0&&torso.torsoCap.neck>0);
            for(const slot of slots.filter(s=>s.name.startsWith('upper_arm'))){assert.ok(slot.shoulderCap>0&&slot.shoulderCap<100);}
            for(const slot of slots.filter(s=>s.kind==='hand')){
                assert.deepEqual(Object.keys(slot.edgeVariants).sort(),['fist','hold','open','point']);
                const open=slot.edgeVariants.open;
                if(character.id==='char.it-tech-1')assert.match(open.file,/it-tech-1-hands-back\.png$/);
                else assert.deepEqual(open,descriptor.views.profile.find(s=>s.name===slot.name).variants.open);
            }
        }
    }
});
