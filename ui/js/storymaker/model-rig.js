// Absolute-time 3D sampling: every pose starts from authored data, including reverse and restarted seeks.
(function (root) {
    'use strict';
    var SM=root.StorymakerCore||(root.StorymakerCore={});
    function create(model,pack) {
        var T=root.StorymakerVendor.THREE, bones=new Map(), objects=new Map(), meshes=[];
        model.traverse(function(o){objects.set(o.name,o);if(o.isBone)bones.set(o.name,o);if(o.morphTargetInfluences)meshes.push(o);});
        if(pack.schema!=='storymaker-model-motions/1'||!pack.poses||!pack.rest||!Array.isArray(pack.clips)||pack.clips.length>32)throw new Error('Invalid model motions');
        Object.values(pack.poses).concat([pack.rest]).forEach(function(pose){
            if(Object.keys(pose).length>128)throw new Error('Invalid model pose');
            Object.values(pose).forEach(function(v){if(!v.p||v.p.length!==3||!v.q||v.q.length!==4||!v.p.concat(v.q,v.s||[]).every(Number.isFinite))throw new Error('Invalid model pose joint');});
        });
        var clips={},transforms=new Map();
        pack.clips.forEach(function(item){
            var clip=T.AnimationClip.parse(item.clip);
            if(!(clip.duration>0&&clip.duration<30)||clip.tracks.length>256)throw new Error('Invalid model clip');
            clips[item.id]={duration:clip.duration,tracks:clip.tracks.map(function(track){
                var split=track.name.lastIndexOf('.'),name=track.name.slice(0,split),property=track.name.slice(split+1);
                if(!['position','quaternion'].includes(property)||track.times.length>2048||!Array.from(track.values).every(Number.isFinite))throw new Error('Invalid model track');
                var object=objects.get(name);if(object&&!transforms.has(name))transforms.set(name,{p:object.position.toArray(),q:object.quaternion.toArray()});
                return {name:name,property:property,sample:track.createInterpolant()};
            })};
        });
        var sole=[];
        model.updateMatrixWorld(true);
        model.traverse(function(mesh){if(!mesh.isSkinnedMesh)return;var pos=mesh.geometry.attributes.position,min=Infinity;
            for(var i=0;i<pos.count;i++)min=Math.min(min,pos.getY(i));
            for(var j=0;j<pos.count;j++)if(pos.getY(j)<min+.025)sole.push({mesh:mesh,index:j});
        });
        return {model:model,bones:bones,objects:objects,transforms:transforms,meshes:meshes,pack:pack,clips:clips,sole:sole,vector:new T.Vector3(),quaternion:new T.Quaternion()};
    }
    function smooth(x){x=Math.max(0,Math.min(1,x));return x*x*(3-2*x);}
    function apply(rig,state,time,presentation,entity) {
        var T=root.StorymakerVendor.THREE, poses=rig.pack.poses, a=poses[state.pose.a]||poses.stand,b=poses[state.pose.b]||poses.stand,u=state.pose.u;
        rig.transforms.forEach(function(v,name){var o=rig.objects.get(name);o.position.fromArray(v.p);o.quaternion.fromArray(v.q);});
        rig.bones.forEach(function(bone,name){
            var va=a[name]||rig.pack.rest[name],vb=b[name]||va;
            if(!va||!vb)throw new Error('Missing model pose joint');
            bone.position.fromArray(va.p).lerp(rig.vector.fromArray(vb.p),u);
            bone.quaternion.fromArray(va.q).slerp(rig.quaternion.fromArray(vb.q),u);
            bone.scale.fromArray(rig.pack.rest[name].s);
        });
        var walking=state.walkTime!==null&&state.walkTime!==undefined;
        if(walking){var clip=rig.clips['walk-happy'],at=((state.walkTime%clip.duration)+clip.duration)%clip.duration;
            var walk=entity&&entity.walks&&entity.walks.active(time),blend=walk?smooth((time-walk.t0)/.18)*smooth((walk.t1-time)/.18):1;
            clip.tracks.forEach(function(track){var object=rig.objects.get(track.name);if(!object)return;var v=track.sample.evaluate(at);
                if(track.property==='quaternion')object.quaternion.slerp(rig.quaternion.fromArray(v),blend);else object.position.lerp(rig.vector.fromArray(v),blend);
            });
        }
        if(state.gesture){
            var names={point:'point-right',wave:'wave',shrug:'shrug',think:'think',present:'present','thumbs-up':'thumbs-up',celebrate:'celebrate',facepalm:'facepalm',count:'count'};
            var target=poses[names[state.gesture.name]],f=state.gesture.frac,w=smooth(f/.18)*smooth((1-f)/.22);
            if(target)rig.bones.forEach(function(bone,name){if(/arm|hand|shoulder|thumb|f_/.test(name)){bone.quaternion.slerp(rig.quaternion.fromArray(target[name].q),w);}});
            if(state.gesture.name==='wave'){var hand=rig.bones.get('DEF-handL');if(hand)hand.quaternion.multiply(rig.quaternion.setFromAxisAngle(rig.vector.set(0,1,0),Math.sin(f*Math.PI*8)*.22*w));}
        }
        var chest=rig.bones.get('DEF-spine003');
        if(chest)chest.quaternion.multiply(rig.quaternion.setFromAxisAngle(rig.vector.set(1,0,0),Math.sin((time+(state.phase||0))*Math.PI*.7)*.006));
        var speech=entity&&entity.speech&&entity.speech.active(time),weights={},mouth=presentation.mouth||state.mouth;
        if(speech){
            var block=speech.data.block,track=block.mouths||SM.lipsync.track(block),span=block.duration_ms/1000;
            var x=(time-speech.t0)/span*track.length-.5,index=Math.floor(x),fraction=smooth(x-index);
            [index,index+1].forEach(function(k,i){var name=track[k];if(name&&name!=='rest')weights[name]=(weights[name]||0)+(i?fraction:1-fraction);});
        }else if(mouth&&mouth!=='rest')weights[mouth]=1;
        var expression=state.expression;
        if(expression&&expression!=='neutral')weights[expression]=mouth&&mouth!=='rest'?.35:1;
        var blink=(time+(state.phase||0)*3)%3.6;
        weights.blink=blink<.16?Math.sin(blink/.16*Math.PI):0;
        rig.meshes.forEach(function(mesh){mesh.morphTargetInfluences.fill(0);Object.keys(weights).forEach(function(name){var index=mesh.morphTargetDictionary[name];if(index!==undefined)mesh.morphTargetInfluences[index]=weights[name];});});
        rig.model.updateMatrixWorld(true);
        var floor=Infinity;
        rig.sole.forEach(function(point){var mesh=point.mesh;rig.vector.fromBufferAttribute(mesh.geometry.attributes.position,point.index);mesh.applyBoneTransform(point.index,rig.vector).applyMatrix4(mesh.matrixWorld);floor=Math.min(floor,rig.vector.y);});
        return Number.isFinite(floor)?floor:0;
    }
    SM.modelRig={create:create,apply:apply};
})(typeof globalThis!=='undefined'?globalThis:self);
