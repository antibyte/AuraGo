import assert from 'node:assert/strict';
import * as THREE from 'three';

// Layer original tentacle motion over the retained CC0 body clips, never the human fingers.
export function addVampireTentacles(clip, target, id, restCurl) {
    assert.equal(restCurl.length,8);
    const z=new THREE.Vector3(0,0,1),restWorld=new THREE.Quaternion();
    for(const [side,sign] of [['L',1],['R',-1]])for(let segment=0;segment<8;segment++){
        const bone=target.skeleton.bones.find(b=>b.name===`Tentacle${String(segment+1).padStart(2,'0')}${side}`);
        assert.ok(bone,'Missing vampire tentacle segment');
        bone.getWorldQuaternion(restWorld);
        const axis=z.clone().applyQuaternion(restWorld.invert()),times=[],values=[];
        const frames=Math.ceil(clip.duration*24);
        for(let frame=0;frame<=frames;frame++){
            const time=frame*clip.duration/frames,phase=time/clip.duration*Math.PI*2;
            let angle=-restCurl[segment];
            if(id==='ledge-hang')angle=segment===0?-1.12:angle*.35;
            else if(id==='greeting'&&side==='L')angle+=(segment===0?.66:.10)*Math.sin(Math.PI*time/clip.duration)**2;
            else if(/sit-|sleeping/.test(id))angle+=segment===0?-.22:0;
            const amplitude=/dance-|victory|guitar-playing|clapping|dab/.test(id)?.13:/run-|jog|jump-|walk-/.test(id)?.07:.035;
            angle+=Math.sin(phase-segment*.55+(side==='L'?0:.7))*amplitude*(.5+segment/12);
            const rotation=bone.quaternion.clone().multiply(new THREE.Quaternion().setFromAxisAngle(axis,sign*angle)).normalize();
            times.push(time);rotation.toArray(values,values.length);
        }
        clip.tracks.push(new THREE.QuaternionKeyframeTrack(bone.name+'.quaternion',times,values));
    }
    clip.optimize();
}
