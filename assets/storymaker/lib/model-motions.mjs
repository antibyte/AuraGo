// Author missing explainer poses on the original pet skeleton; existing motion data stays untouched.
import * as T from 'three';
import { loadSkeleton, reset } from '../../../scripts/pet-skeleton.mjs';

export async function authorMotions(file, source) {
  const data = await loadSkeleton(file), model = data.gltf.scene;
  const bones = new Map(data.skeleton.bones.map(b => [b.name, b]));
  const point = name => bones.get(name).getWorldPosition(new T.Vector3());
  const rest = Object.fromEntries(data.skeleton.bones.map(b => [b.name, { p: b.position.toArray(), q: b.quaternion.toArray(), s: b.scale.toArray() }]));
  const round = a => a.map(n => +n.toFixed(7));
  const capture = () => Object.fromEntries(data.skeleton.bones.map(b => [b.name, { p: round(b.position.toArray()), q: round(b.quaternion.toArray()) }]));
  const clip = id => T.AnimationClip.parse(source.clips.find(c => c.id === id).clip);
  const mixer = new T.AnimationMixer(model), idle = mixer.clipAction(clip('idle-a'));
  idle.play(); mixer.setTime(0); const relaxed = capture(); mixer.stopAllAction(); mixer.uncacheRoot(model);
  const set = pose => { reset(data); for (const [name,v] of Object.entries(pose)) { const b=bones.get(name); b.position.fromArray(v.p); b.quaternion.fromArray(v.q); } model.updateMatrixWorld(true); };
  function aim(name, end, direction) {
    const b=bones.get(name), origin=point(name), before=point(end).sub(origin).normalize();
    const desired=new T.Vector3(...direction).normalize(), world=b.getWorldQuaternion(new T.Quaternion());
    world.premultiply(new T.Quaternion().setFromUnitVectors(before,desired));
    b.quaternion.copy(b.parent.getWorldQuaternion(new T.Quaternion()).invert().multiply(world)); model.updateMatrixWorld(true);
  }
  function arm(side, upper, lower) {
    aim('DEF-upper_arm'+side,'DEF-forearm'+side,upper);
    aim('DEF-forearm'+side,'DEF-hand'+side,lower);
  }
  function fingers(side, kind) {
    for(const bone of bones.values()) {
      const name=bone.name;
      if(!/^(DEF-f_|DEF-thumb)/.test(name)||!name.endsWith(side))continue;
      const open=kind==='open'||kind==='point'&&name.includes('index')||kind==='count'&&/index|middle/.test(name)||kind==='thumbs-up'&&name.startsWith('DEF-thumb');
      // The source idle clip already has anatomically curled fingers; preserve its per-joint axes.
      bone.quaternion.fromArray((open?rest:relaxed)[name].q);
      if(kind==='typing')bone.quaternion.fromArray(rest[name].q).slerp(new T.Quaternion().fromArray(relaxed[name].q),.35);
    }
  }
  function sitting() {
    for(const side of ['L','R']) {
      aim('DEF-thigh'+side,'DEF-shin'+side,[side==='L'?.08:-.08,-.12,1]);
      aim('DEF-shin'+side,'DEF-foot'+side,[0,-1,-.03]);
    }
  }
  const poses={};
  for(const name of ['stand','sit','typing','point-left','point-right','wave','think','present','arms-crossed','hands-on-hips','shrug','thumbs-up','celebrate','facepalm','count']) {
    set(relaxed);
    if(name==='sit'||name==='typing')sitting();
    if(name==='typing'){arm('L',[.2,-1,.65],[-.15,-.12,1]);arm('R',[-.2,-1,.65],[.15,-.12,1]);fingers('L','typing');fingers('R','typing');}
    if(name==='present'){arm('L',[.75,-.8,.35],[.7,.18,.65]);fingers('L','open');}
    if(name==='point-left'||name==='point-right'){const side=name==='point-left'?'R':'L',s=side==='L'?1:-1;arm(side,[s*.95,-.28,.2],[s,.08,.3]);fingers(side,'point');}
    if(name==='wave'){arm('L',[.8,-.15,.2],[.1,1,.1]);fingers('L','open');}
    if(name==='think'){arm('R',[-.3,-.9,.3],[.3,.8,.55]);fingers('R','point');}
    if(name==='arms-crossed'){arm('L',[.25,-1,.25],[-1,.4,.4]);arm('R',[-.25,-1,.35],[1,.35,.4]);}
    if(name==='hands-on-hips'){arm('L',[.7,-1,0],[-.5,-1,0]);arm('R',[-.7,-1,0],[.5,-1,0]);}
    if(name==='shrug'){arm('L',[.7,-.9,.2],[.7,.65,.2]);arm('R',[-.7,-.9,.2],[-.7,.65,.2]);fingers('L','open');fingers('R','open');}
    if(name==='thumbs-up'||name==='count'){arm('L',[.4,-1,.2],[.05,1,.8]);fingers('L',name);if(name==='thumbs-up')aim('DEF-handL','DEF-thumb03L',[0,1,0]);}
    if(name==='celebrate'){arm('L',[.65,.8,0],[.1,1,0]);arm('R',[-.65,.8,0],[-.1,1,0]);fingers('L','fist');fingers('R','fist');}
    if(name==='facepalm'){arm('R',[-.35,-.7,.6],[.35,1,-.05]);fingers('R','open');}
    model.updateMatrixWorld(true); poses[name]=capture();
  }
  const clips=source.clips.filter(c=>['idle-a','walk-happy','greeting'].includes(c.id));
  return {schema:'storymaker-model-motions/1',rest,poses,clips,source:source.source||source.provenance||null};
}
