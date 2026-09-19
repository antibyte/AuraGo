// Browser-only acceptance harness. Not part of the shipped resource set.
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';

export async function reviewSystemWorld(page=0,animations=false){
    const manifest=await(await fetch('/3d/system-world/v2/manifest.json')).json(),loader=new GLTFLoader();
    const entries=animations?manifest.assets.filter(a=>a.animations.length):manifest.assets;
    const selected=animations?entries.slice(page,page+1):entries.slice(page*8,page*8+8);
    const renderer=new THREE.WebGLRenderer({antialias:true,alpha:false,preserveDrawingBuffer:true});
    renderer.setSize(300,200);renderer.setPixelRatio(1);renderer.outputColorSpace=THREE.SRGBColorSpace;
    const sheet=document.createElement('canvas'),ctx=sheet.getContext('2d');sheet.width=animations?1200:900;
    const rows=animations?selected[0].animations.length:selected.length;sheet.height=rows*232;
    ctx.fillStyle='#102027';ctx.fillRect(0,0,sheet.width,sheet.height);ctx.font='15px sans-serif';
    const scene=new THREE.Scene();scene.background=new THREE.Color('#162a34');
    scene.add(new THREE.HemisphereLight(0xecf7ff,0x526169,2.5));const sun=new THREE.DirectionalLight(0xffeed3,3);sun.position.set(5,10,8);scene.add(sun);
    const camera=new THREE.PerspectiveCamera(40,1.5,.01,1000),report=[];
    try{
        for(let r=0;r<selected.length;r++){
            const entry=selected[r];
            for(const lod of (animations?[entry.lods[0]]:entry.lods)){
                const gltf=await loader.loadAsync('/3d/system-world/v2/'+lod.file),root=gltf.scene;scene.add(root);
                const box=new THREE.Box3().setFromObject(root),size=box.getSize(new THREE.Vector3()),center=box.getCenter(new THREE.Vector3());
                const radius=Math.max(size.x,size.y,size.z)*1.9;
                camera.position.copy(center).add(new THREE.Vector3(radius*.7,radius*.5,radius*.8));camera.lookAt(center);
                const mixer=new THREE.AnimationMixer(root),clips=animations?gltf.animations:[null];
                for(let row=0;row<clips.length;row++)for(let frame=0;frame<(animations?4:1);frame++){
                    if(clips[row]){mixer.stopAllAction();mixer.clipAction(clips[row]).reset().play();mixer.setTime(clips[row].duration*[0,.19,.43,.69][frame]);}
                    renderer.render(scene,camera);const x=(animations?frame:lod.level)*300,y=(animations?row:r)*232;
                    ctx.drawImage(renderer.domElement,x,y);ctx.fillStyle='#dbe8ef';ctx.fillText(entry.id+' / '+(animations?clips[row].name+' '+frame:('LOD '+lod.level)),x+8,y+220);
                }
                report.push({asset:entry.id,lod:lod.level,clips:gltf.animations.map(c=>c.name),triangles:renderer.info.render.triangles});
                mixer.stopAllAction();mixer.uncacheRoot(root);root.removeFromParent();root.traverse(n=>{if(n.isMesh){n.geometry.dispose();for(const m of(Array.isArray(n.material)?n.material:[n.material]))m.dispose();}});
            }
        }
        return {png:sheet.toDataURL('image/png'),report,total:entries.length};
    }finally{renderer.dispose();renderer.forceContextLoss();}
}
