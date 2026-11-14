package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopAuraDriftPetDecorBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t).CancelTimeout().Timeout(6 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	requireWebGL2(t, page)
	auraDriftReopen(page, map[string]interface{}{"quality": "low"}, "")
	page.MustEval(`async()=>{
        const AD=AuraDriftCore, make=AD.createPetDecor;
        AD.createPetDecor=(...args)=>{const api=make(...args);window.__petDecor=api;return api;};
        await AuraDriftApp.debug.start({mode:'time_trial',trackId:'metropolis'});
        await __petDecor.ready;
    }`)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if !page.MustEval(`()=>{const s=AuraDriftApp.stats()[0];return s.renderer.kind==='three'&&s.renderer.petDecor.loaded===2&&s.renderer.petDecor.total===6&&s.renderer.petDecor.failed===0;}`).Bool() {
		t.Fatal("mascots did not load through the live race renderer: ", page.MustEval(`()=>JSON.stringify(AuraDriftApp.stats())`).Str())
	}
	page.MustEval(`()=>{
        AuraDriftApp.debug.freeze(true);
        const canvas=document.createElement('canvas');canvas.id='mascot-probe';
        canvas.style.cssText='position:fixed;inset:0;width:960px;height:640px;z-index:999999';document.body.append(canvas);
        window.__mascotRenderer=AuraDriftCore.createRenderer3D(canvas,{quality:'low'},window.t);
        __mascotRenderer.setQuality('low');__mascotRenderer.resize(960,640,1);
    }`)
	defer page.MustEval(`()=>{__mascotRenderer.dispose();document.getElementById('mascot-probe').remove();AuraDriftApp.debug.freeze(false);}`)
	artifacts := auraDriftArtifactDir(t, "drift-pets")
	for _, item := range []struct {
		track string
		spot  int
	}{{"metropolis", 0}, {"metropolis", 1}, {"alpenpass", 0}, {"alpenpass", 1}, {"wuesten-canyon", 0}, {"wuesten-canyon", 1}, {"neon-hafen", 0}, {"neon-hafen", 1}, {"salzsee", 0}, {"wald-rallye", 0}, {"wald-rallye", 1}, {"industrie-ring", 0}, {"gletscherpass", 0}} {
		page.MustEval(`async (id,spot)=>{
            const AD=AuraDriftCore;
            if(!window.__mascotRace||__mascotRace.track.id!==id){
                const track=AD.buildTrack(AD.trackById(id));
                window.__mascotRace=AD.createRace({track,mode:'time_trial',opponents:0,weapons:false});
                __mascotRenderer.setTrack(track,__mascotRace);await __petDecor.ready;
            }
            const track=__mascotRace.track;
            const stats=__petDecor.stats();if(stats.loaded!==2||stats.failed||stats.total!==6)throw Error('Mascot loading failed: '+JSON.stringify(stats));
            const p=stats.placements[spot], holder=__mascotRenderer.scene.getObjectByName('mascot-'+p.folder);
            const dir=new AuraDriftThree.Vector3(0,0,1).applyQuaternion(holder.quaternion);
            const target=[p.x,p.z+1.5,p.y];
            window.__mascotPose={eye:[p.x+dir.x*10+dir.z*3,p.z+3.5,p.y+dir.z*10-dir.x*3],target,fov:45,roll:0};
            const car=__mascotRace.cars[0].v,road=AD.pointAt(track,p.s-8);car.x=road.x;car.y=road.y;car.psi=Math.atan2(road.ty,road.tx);
            for(let i=0;i<35;i++)__petDecor.update(__mascotPose,__mascotRace,.03,false);
            __mascotRenderer.render(__mascotRace,__mascotPose,.03,false);
            const state=__petDecor.stats();if(state.visible!==1||state.animated!==1)throw Error('Low-tier mascot budget: '+JSON.stringify(state));
            const root=__mascotRenderer.scene.getObjectByName('pet-decor');
            const instances=root.children.filter(o=>o.name===holder.name);
            const skinned=o=>{let mesh; o.traverse(n=>{if(n.isSkinnedMesh&&!mesh)mesh=n;});return mesh;};
            const a=skinned(instances[0]),b=skinned(instances[1]);
            a.updateWorldMatrix(true,false);a.skeleton.update();
            const point=new AuraDriftThree.Vector3();let lowest=Infinity;
            for(let i=0;i<a.geometry.attributes.position.count;i++){
                point.fromBufferAttribute(a.geometry.attributes.position,i);a.applyBoneTransform(i,point).applyMatrix4(a.matrixWorld);lowest=Math.min(lowest,point.y);
            }
            if(lowest<p.z-.3)throw Error('Mascot feet are buried in the terrain: '+p.folder+' '+lowest+' < '+p.z);
            if(a.skeleton===b.skeleton||a.skeleton.bones[0]===b.skeleton.bones[0]||a.geometry!==b.geometry)throw Error('Mascots must share geometry and own independent skeletons');
            window.__mascotSkin=a;
            const signature=()=>a.skeleton.bones.flatMap(b=>b.quaternion.toArray());
            const before=signature();for(let i=0;i<20;i++)__petDecor.update(__mascotPose,__mascotRace,.03,false);
            if(!signature().some((v,i)=>Math.abs(v-before[i])>.00001))throw Error('Mascot skin did not animate');
            const frozen=signature();for(let i=0;i<10;i++)__petDecor.update(__mascotPose,__mascotRace,.03,true);
            if(signature().some((v,i)=>v!==frozen[i])||__petDecor.stats().animated!==0)throw Error('Reduced motion moved the skin');
            const ticks=__petDecor.stats().ticks;__petDecor.update(__mascotPose,__mascotRace,0,false);
            if(__petDecor.stats().ticks!==ticks)throw Error('Paused mascot advanced');
            __petDecor.update({eye:[100000,0,100000]},__mascotRace,.03,false);
            if(__petDecor.stats().visible!==0||__petDecor.stats().animated!==0)throw Error('Distant mascots did not stop');
            const eye=__mascotPose.eye;
            __petDecor.update({eye,target:[eye[0]*2-target[0],eye[1],eye[2]*2-target[2]]},__mascotRace,.03,false);
            if(holder.visible)throw Error('A mascot behind the camera consumed its visibility budget');
            __petDecor.update({eye:[p.x,p.z+60,p.y-5],target:[p.x,p.z,p.y+15]},__mascotRace,.03,false);
            if(!holder.visible)throw Error('A high camera incorrectly culled a mascot below it');
            __petDecor.setQuality(AD.QUALITY.high.gl);__petDecor.update(__mascotPose,__mascotRace,.03,false);
            if(__petDecor.stats().visible>2)throw Error('High-tier mascot budget exceeded');
            __petDecor.setQuality(AD.QUALITY.low.gl);
            __mascotRenderer.render(__mascotRace,__mascotPose,.03,false);
        }`, item.track, item.spot)
		if artifacts != "" {
			page.MustElement("#mascot-probe").MustScreenshot(filepath.Join(artifacts, fmt.Sprintf("%s-%d.png", item.track, item.spot)))
		}
	}
	page.MustEval(`async()=>{
        const AD=AuraDriftCore,T=AuraDriftThree,track=__mascotRace.track,live=__petDecor;
        const json=AD.assets.fetchJson;
        try{
            AD.assets.fetchJson=()=>Promise.reject(Error('fixture missing mascot pack'));
            const scene=new T.Scene(),api=AD.createPetDecor(T,scene,track,AD.QUALITY.low.gl);await api.ready;
            if(api.stats().failed!==2||api.stats().total!==0)throw Error('Failed optional assets must omit the mascots');
            api.dispose();if(scene.children.length)throw Error('Failed mascots leaked a scene group');
        }finally{AD.assets.fetchJson=json;}
        const bytes=AD.assets.fetchBytes;let release;const gate=new Promise(resolve=>{release=resolve;});
        try{
            AD.assets.fetchBytes=url=>bytes(url).then(value=>gate.then(()=>value));
            const scene=new T.Scene(),api=AD.createPetDecor(T,scene,track,AD.QUALITY.low.gl);api.dispose();release();await api.ready;
            if(scene.children.length||api.stats().total)throw Error('Late model load survived disposal');
        }finally{AD.assets.fetchBytes=bytes;}
        const scene=__mascotRenderer.scene,geometry=__mascotSkin.geometry;let disposed=0;
        geometry.addEventListener('dispose',()=>disposed++);
        live.dispose();live.dispose();
        if(scene.getObjectByName('pet-decor')||disposed!==1)throw Error('Mascot disposal must release shared geometry exactly once');
    }`)
	auraDriftNoErrors(t, page)
	if artifacts != "" {
		if err := os.WriteFile(filepath.Join(artifacts, "checks.txt"), []byte("Live race integration; nine skinned pets across eight tracks; geometry sharing; independent rigs; movement; reduced motion; pause; distance and quality caps; asset failure; late disposal; shared geometry disposal.\n"), 0644); err != nil {
			t.Fatal(err)
		}
		t.Log("Mascot render artifacts: ", artifacts)
	}
}
