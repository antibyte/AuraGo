// MIT. Shared buffers and bounded particles; the host owns the only animation clock.
import * as T from 'three';
import {Sky} from 'three/addons/objects/Sky.js';
import {Water} from 'three/addons/objects/Water.js';
import {EffectComposer} from 'three/addons/postprocessing/EffectComposer.js';
import {RenderPass} from 'three/addons/postprocessing/RenderPass.js';
import {ShaderPass} from 'three/addons/postprocessing/ShaderPass.js';
import {UnrealBloomPass} from 'three/addons/postprocessing/UnrealBloomPass.js';
import {OutputPass} from 'three/addons/postprocessing/OutputPass.js';
export {createPresentation} from './presentation.js';

const vertex=`attribute float size;attribute float alpha;attribute float kind;varying vec3 tint;varying float opacity;varying float shape;
void main(){tint=color;opacity=alpha;shape=kind;vec4 p=modelViewMatrix*vec4(position,1.);gl_Position=projectionMatrix*p;gl_PointSize=clamp(size*650./max(1.,-p.z),1.,kind<.5?34.:160.);}`;
const fragment=`varying vec3 tint;varying float opacity;varying float shape;void main(){vec2 p=gl_PointCoord*2.-1.;float a;
if(shape<.5){p.x*=6.;a=(1.-smoothstep(.35,1.,length(p)))*.5;}
else if(shape<1.5){a=(1.-smoothstep(.05,1.,length(p)))*.5;}
else if(shape<2.5){a=1.-smoothstep(.65,1.,length(p));}
else if(shape<3.5){p.y*=1.8;a=1.-smoothstep(.5,1.,length(p));}
else {p.y*=3.;a=(1.-smoothstep(0.,1.,length(p)))*.16;}
if(a*opacity<.015)discard;gl_FragColor=vec4(tint,a*opacity);}`;
const grade={uniforms:{tDiffuse:{value:null},time:{value:0},vignette:{value:0},grain:{value:0},heat:{value:0},underwater:{value:0},grade:{value:0}},vertexShader:'varying vec2 uv0;void main(){uv0=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',fragmentShader:`uniform sampler2D tDiffuse;uniform float time,vignette,grain,heat,underwater,grade;varying vec2 uv0;
void main(){vec2 p=uv0;p.x+=(sin(p.y*32.+time*2.)*.002+sin(p.y*77.-time)*.001)*(heat+underwater);vec3 c=texture2D(tDiffuse,p).rgb;
c=mix(c,c*vec3(.55,.86,1.04),underwater*.6);c=mix(c,pow(max(c,vec3(0.)),vec3(.97))*vec3(1.035,1.01,.96),grade*.4);
c*=1.-vignette*.6*smoothstep(.15,.8,length(uv0-.5));c+=(fract(sin(dot(uv0+time,vec2(12.9898,78.233)))*43758.5453)-.5)*grain*.035;gl_FragColor=vec4(c,1.);}`};

export function createThreeAdapter({scene,camera,renderer,sun,ambient}) {
    const group=new T.Group();group.name='AuraGo presentation';scene.add(group);
    const settings=new Map(),surfaces=new Map(),owned=[],particles=[],decals=[],objectEffects=[],objectReleases=new Map();
    let level=2,reduced=false,blood=true,time=0,disposed=false,seed=71237,weatherCarry=0,stormAt=6;
    const random=()=>{seed^=seed<<13;seed^=seed>>>17;seed^=seed<<5;return(seed>>>0)/4294967296};
    const center=new T.Vector3(),v=new T.Vector3(),ray=new T.Raycaster(),normal=new T.Vector3();
    const old={background:scene.background,fog:scene.fog,sun:sun&&{color:sun.color.clone(),intensity:sun.intensity,position:sun.position.clone()},ambient:ambient?.intensity};
    const capacity=4000,positions=new Float32Array(capacity*3),colors=new Float32Array(capacity*3),sizes=new Float32Array(capacity),alphas=new Float32Array(capacity),kinds=new Float32Array(capacity);
    const geometry=new T.BufferGeometry();for(const [key,array,n]of [['position',positions,3],['color',colors,3],['size',sizes,1],['alpha',alphas,1],['kind',kinds,1]])geometry.setAttribute(key,new T.BufferAttribute(array,n).setUsage(T.DynamicDrawUsage));
    geometry.setDrawRange(0,0);const material=new T.ShaderMaterial({vertexShader:vertex,fragmentShader:fragment,vertexColors:true,transparent:true,depthWrite:false});
    const points=new T.Points(geometry,material);points.frustumCulled=false;group.add(points);owned.push(geometry,material);
    let sky,stars,clouds,moon,water,waterNormals,composer,bloom,gradePass,outputPass;
    const color=new T.Color(),tint=new T.Color();
    function makeSky(){
        if(sky)return;sky=new Sky();sky.scale.setScalar(210);sky.material.uniforms.cloudCoverage.value=0;
        sky.material.uniforms.auraSkyColor={value:new T.Color(.42,.62,.78)};
        sky.material.fragmentShader='uniform vec3 auraSkyColor;\n'+sky.material.fragmentShader.replace('gl_FragColor = vec4( texColor, 1.0 );','vec3 d=normalize(vWorldPosition-cameraPosition);vec3 atmosphere=mix(auraSkyColor,auraSkyColor*vec3(.1,.28,.52),pow(max(0.,d.y),.45));gl_FragColor = vec4(mix(atmosphere,clamp(texColor*.035,0.,1.),.3),1.0);');
        group.add(sky);owned.push(sky.geometry,sky.material);
        const starGeometry=new T.BufferGeometry(),a=new Float32Array(1200*3);for(let i=0;i<1200;i++){v.set(random()-.5,random()-.5,random()-.5).normalize().multiplyScalar(190);a.set(v.toArray(),i*3)}
        starGeometry.setAttribute('position',new T.BufferAttribute(a,3));const m=new T.PointsMaterial({color:0xc8deff,size:.65,transparent:true,depthWrite:false,fog:false});stars=new T.Points(starGeometry,m);group.add(stars);owned.push(starGeometry,m);
        const mg=new T.SphereGeometry(4,24,16),mm=new T.MeshBasicMaterial({color:0xd7e0e7,fog:false});moon=new T.Mesh(mg,mm);moon.position.set(-80,100,-120);group.add(moon);owned.push(mg,mm);
        const cg=new T.SphereGeometry(205,32,16),cm=new T.ShaderMaterial({side:T.BackSide,transparent:true,depthWrite:false,uniforms:{time:{value:0},coverage:{value:.5},night:{value:0}},vertexShader:'varying vec3 p;void main(){p=position;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',fragmentShader:`varying vec3 p;uniform float time,coverage,night;
float hash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}float noise(vec2 x){vec2 i=floor(x),f=fract(x);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1,0)),f.x),mix(hash(i+vec2(0,1)),hash(i+1.),f.x),f.y);}
void main(){vec3 d=normalize(p);vec2 uv=night>1.1?vec2(atan(d.z,d.x),asin(d.y))*3.:d.xz/max(.12,d.y)*1.7+vec2(time*.015,0);float n=0.,a=.55;for(int i=0;i<5;i++){n+=a*noise(uv);uv=uv*2.03+.71;a*=.5;}float c=smoothstep(1.-coverage,1.15-coverage,n)*(night>1.1?1.:smoothstep(0.,.2,d.y));gl_FragColor=night>1.1?vec4(mix(vec3(.05,.12,.3),vec3(.3,.07,.4),n),c*.45):vec4(mix(vec3(.78,.87,.95),vec3(.09,.12,.18),night),c*.42);}`});clouds=new T.Mesh(cg,cm);group.add(clouds);owned.push(cg,cm);
    }
    function makePost(){
        if(composer)return;composer=new EffectComposer(renderer);composer.addPass(new RenderPass(scene,camera));bloom=new UnrealBloomPass(new T.Vector2(512,512),.3,.35,.8);composer.addPass(bloom);gradePass=new ShaderPass(grade);composer.addPass(gradePass);outputPass=new OutputPass();composer.addPass(outputPass);resize();
    }
    function makeWater(id,p){
        if(water){group.remove(water);water.geometry.dispose();water.material.dispose();water.dispose();waterNormals.dispose()}
        const a=new Uint8Array(64*64*4);for(let i=0;i<64*64;i++){a[i*4]=128+Math.sin(i*.37)*32;a[i*4+1]=128+Math.cos(i*.51)*32;a[i*4+2]=245;a[i*4+3]=255}
        waterNormals=new T.DataTexture(a,64,64);waterNormals.wrapS=waterNormals.wrapT=T.RepeatWrapping;waterNormals.magFilter=waterNormals.minFilter=T.LinearFilter;waterNormals.needsUpdate=true;
        const g=new T.PlaneGeometry(p.width||80,p.depth||80,32,32);
        water=new Water(g,{textureWidth:512,textureHeight:512,waterNormals,sunDirection:new T.Vector3(1,1,1),sunColor:0xffecd1,waterColor:id==='water-river'?0x246d65:0x125f80,distortionScale:id==='water-ocean'?3:1.2,fog:!!scene.fog});
        water.material.fragmentShader=water.material.fragmentShader.replace('vec3 outgoingLight = albedo;','vec3 outgoingLight = mix(albedo,waterColor,.4);').replace('gl_FragColor = vec4( outgoingLight, alpha );', `
            float wave=sin(worldPosition.x*.32+time)*cos(worldPosition.z*.24-time*.6);
            float foam=smoothstep(.94,.995,wave)*(.3+.7*sin(worldPosition.x*8.+worldPosition.z*9.)*sin(worldPosition.x*8.+worldPosition.z*9.));
            outgoingLight=mix(outgoingLight,vec3(.65,.87,.9),foam*.25);
            gl_FragColor = vec4(outgoingLight, alpha);`);
        water.rotation.x=-Math.PI/2;water.position.fromArray(p.position||[0,p.y??-.05,20]);water.userData.base=g.attributes.position.array.slice();water.userData.id=id;water.userData.speed=p.speed||.6;
        const reflection=water.onBeforeRender;water.onBeforeRender=function(...args){if(level===2)reflection.apply(this,args)};group.add(water);
    }
    function set(id,p){
        settings.set(id,p);
        if(id.startsWith('sky-')||id==='day-night'){for(const key of settings.keys())if(id.startsWith('sky-')&&key.startsWith('sky-')&&key!==id)settings.delete(key);makeSky()}
        if(['water-lake','water-river','water-ocean'].includes(id))makeWater(id,p);
        if(['bloom','color-grade','vignette','film-grain','heat-haze','underwater'].includes(id))makePost();
        if(id==='fog-distance')scene.fog=new T.FogExp2(0x9cb0b2,p.density??.018);
    }
    function floorAt(x,z){
        if(!surfaces.size)return 0;
        ray.set(v.set(x,200,z),normal.set(0,-1,0));const hits=ray.intersectObjects([...surfaces.keys()],true);return hits[0]?.point.y??0;
    }
    function particle(id,p,position){
        const max=[500,1500,4000][level];if(particles.length>=max)return;
        const rain=id.startsWith('rain'),snow=id==='snow',smoke=id==='smoke'||id.startsWith('fog'),leaf=id==='wind-leaves',isBlood=id.startsWith('blood'),flame=id==='fire',trail=id==='engine-trail',splash=id==='water-splash',stone=id==='stone-debris';
        const hue=isBlood?p.color||'#9f162a':smoke?'#9caebb':rain||splash?'#b3d7ed':stone?'#a69b89':snow?'#f5faff':leaf?'#9eae55':trail||id==='magic'||id==='teleport'||id==='pickup-glow'?'#76ddff':flame?'#ff792e':'#ffbd57';
        color.set(p.color&&p.color!=='#ffffff'?p.color:hue);
        const at=position||p.position||[0,1,0],scale=p.scale||1;
        particles.push({id,x:at[0],y:at[1],z:at[2],vx:(random()-.5)*(rain?2:6)*scale,vy:rain?-26:snow?-1.5:smoke?.5+random():(random()*5+1)*scale,vz:(random()-.5)*(rain?1:6)*scale,age:0,life:rain?2.5:snow?12:smoke?5:p.lifetime||1.5,size:(rain?.25:snow?.09:smoke?1.8:isBlood?.11:.18)*scale,c:color.toArray(),kind:rain?0:id.startsWith('fog')?4:smoke?1:leaf?3:2,floor:rain||snow?floorAt(at[0],at[2]):0});
        const e=particles[particles.length-1];
        if(flame||trail){e.kind=1;e.size=(flame?.5:.25)*scale;e.life=flame?1:.5;e.vx*=.1;e.vz*=.1;e.vy=flame?2:0;if(trail){const n=p.normal||[0,0,-1];e.vx=n[0]*5;e.vy=n[1]*5;e.vz=n[2]*5}}
        if(id==='muzzle-flash'||id==='hit-flash'){e.life=.12;e.size=.6*scale;e.kind=1;e.vx=e.vy=e.vz=0}
        if(p.normal&&(isBlood||id==='metal-sparks'||stone||splash)){e.vx+=p.normal[0]*3;e.vy+=p.normal[1]*3;e.vz+=p.normal[2]*3}
        if(id==='wind-dust'||leaf){e.vx=3;e.vy=-.15;e.life=8;e.size=leaf?.16:.5;e.kind=leaf?3:1}
    }
    function decal(id,p){
        if(!blood)return;
        if(id==='blood-pool')p={...p,position:[p.position?.[0]||0,floorAt(p.position?.[0]||0,p.position?.[2]||0),p.position?.[2]||0],normal:[0,1,0]};while(decals.length>=[24,48,96][level]){const d=decals.shift();d.mesh.removeFromParent();d.mesh.geometry.dispose();d.mesh.material.dispose();d.tex?.dispose()}
        const c=document.createElement('canvas');c.width=c.height=96;const ctx=c.getContext('2d');ctx.fillStyle=p.color||'#8c1625';ctx.beginPath();
        for(let i=0;i<=24;i++){const angle=i/24*Math.PI*2,r=25+random()*19;const x=48+Math.cos(angle)*r,y=48+Math.sin(angle)*r;i?ctx.lineTo(x,y):ctx.moveTo(x,y)}ctx.fill();
        for(let i=0;i<14;i++){ctx.beginPath();ctx.arc(random()*96,random()*96,1+random()*3,0,7);ctx.fill()}
        const tex=new T.CanvasTexture(c),g=new T.PlaneGeometry((id==='blood-pool'?1.2:.5)*(p.scale||1),(id==='blood-pool'?1:.6)*(p.scale||1));
        const m=new T.MeshBasicMaterial({map:tex,transparent:true,depthWrite:false,polygonOffset:true,polygonOffsetFactor:-2,side:T.DoubleSide});const mesh=new T.Mesh(g,m);normal.fromArray(p.normal||[0,1,0]);if(normal.lengthSq()<.001)normal.set(0,1,0);normal.normalize();mesh.quaternion.setFromUnitVectors(new T.Vector3(0,0,1),normal);mesh.position.fromArray(p.position||[0,0,0]).addScaledVector(normal,.006);group.add(mesh);decals.push({mesh,tex,age:0,life:p.lifetime||30});
    }
    function emit(id,p){
        if(!['blood-pool','blood-decal','blood-spray','hit-flash','metal-sparks','stone-debris','fire','smoke','embers','explosion','muzzle-flash','engine-trail','magic','teleport','pickup-glow','water-splash','water-ripple'].includes(id))return;
        if(id==='blood-pool'||id==='blood-decal'){decal(id,p);return}
        if(reduced&&(id==='hit-flash'||id==='muzzle-flash'))return;
        if(id==='hit-flash'){for(const e of objectEffects)if(e.id==='hit-flash')e.age=0;particle(id,{...p,color:'#efffff',lifetime:.08,scale:2});return}
        const count=Math.round((id==='explosion'?90:id==='muzzle-flash'?8:30)*Math.min(2,p.intensity??1));
        if(id!=='water-ripple')for(let i=0;i<count;i++)particle(id,p);
        if(id==='explosion'){for(let i=0;i<18;i++)particle('smoke',{...p,color:'#515760',scale:1.2,lifetime:3});for(const e of particles.slice(-count-18,-18)){e.vx*=2;e.vz*=2;e.vy*=1.5}}
        if(id==='water-ripple'){
            while(decals.length>=[24,48,96][level]){const d=decals.shift();d.mesh.removeFromParent();d.mesh.geometry.dispose();d.mesh.material.dispose();d.tex?.dispose()}
            const g=new T.RingGeometry(.09,.1,32),m=new T.MeshBasicMaterial({color:0xb3e4f3,transparent:true,opacity:.65,side:T.DoubleSide,depthWrite:false});const mesh=new T.Mesh(g,m);mesh.rotation.x=-Math.PI/2;mesh.position.fromArray(p.position||[0,.02,0]);group.add(mesh);decals.push({mesh,age:0,life:p.lifetime||1.4,ripple:true,scale:p.scale||1});
        }
    }
    function update(dt,elapsed){
        let thunder=false;time=elapsed;camera.getWorldPosition(center);if(sky&&!settings.has('day-night')&&![...settings.keys()].some(k=>k.startsWith('sky-')))sky.visible=stars.visible=clouds.visible=moon.visible=false;
        if(sky&&(settings.has('day-night')||[...settings.keys()].some(k=>k.startsWith('sky-')))){
            const cycle=settings.get('day-night'),preset=[...settings.keys()].find(k=>k.startsWith('sky-'))||'sky-clear';
            const hour=cycle?(cycle.fixed?cycle.hour:(cycle.hour+elapsed*24/Math.max(1,cycle.cycle))%24):preset==='sky-night'||preset==='sky-space'?0:preset==='sky-sunset'?17.4:11;
            const angle=(hour-6)/24*Math.PI*2,day=Math.max(0,Math.sin(angle)),night=1-Math.min(1,day*3);
            sky.position.copy(center);clouds.position.copy(center);stars.position.copy(center);moon.position.copy(center).add(v.set(-80,100,-120));
            sky.material.uniforms.sunPosition.value.set(Math.cos(angle)*100,Math.sin(angle)*100,-30);sky.material.uniforms.turbidity.value=preset==='sky-storm'?18:5;sky.material.uniforms.rayleigh.value=2;
            sky.material.uniforms.auraSkyColor.value.set(preset==='sky-storm'?0x667384:day<.3?0xd8856b:0x7fbee4);
            sky.visible=night<.95;stars.visible=night>.15;stars.material.opacity=night;moon.visible=preset!=='sky-space'&&night>.4;
            if(night>.7)scene.background=tint.set(preset==='sky-space'?0x060b21:0x111c36);
            if(sun){sun.position.set(Math.cos(angle)*35,Math.sin(angle)*40,15);sun.intensity=.12+day*2.7;sun.color.setRGB(1,.65+day*.28,.48+day*.4)}
            if(ambient)ambient.intensity=.3+day*1.3;
            clouds.material.uniforms.time.value=reduced?0:time;clouds.material.uniforms.coverage.value=preset==='sky-cloudy'?.65:preset==='sky-storm'?.85:.38;clouds.material.uniforms.night.value=night;clouds.visible=true;clouds.material.uniforms.night.value=preset==='sky-space'?1.5:night;
            if(scene.fog)scene.fog.color.setRGB(.12+day*.45,.16+day*.48,.24+day*.45);
        }
        const weather=[...settings.keys()].find(k=>['rain-light','rain-heavy','snow','wind-dust','wind-leaves'].includes(k));
        if(weather){weatherCarry+=dt*(weather==='rain-heavy'?650:weather==='rain-light'?220:weather==='snow'?75:25)*([.3,.65,1][level])*Math.min(3,settings.get(weather).intensity??1);let count=Math.min(32,Math.floor(weatherCarry));weatherCarry=Math.min(1,weatherCarry-count);while(count-->0)particle(weather,settings.get(weather),[center.x+(random()-.5)*34,center.y+8+random()*7,center.z+(random()-.5)*34])}
        for(const id of ['fire','smoke','embers','engine-trail','fog-ground','fog-zone'])if(settings.has(id)&&(id.startsWith('fog')||settings.get(id).position)&&random()<dt*18*Math.min(3,settings.get(id).intensity??1)){const p=settings.get(id),isFog=id.startsWith('fog'),at=p.position||[center.x,.5,center.z],radius=id==='fog-zone'?(p.radius||12):18;particle(id,{...p,scale:isFog?5*(p.scale||1):p.scale,lifetime:isFog?8:p.lifetime||2},isFog?[at[0]+(random()-.5)*radius*2,at[1],at[2]+(random()-.5)*radius*2]:at)}
        if(settings.has('thunderstorm')&&time>stormAt){thunder=true;if(sun&&!reduced)sun.intensity=5;stormAt=time+5+random()*12}
        for(let i=particles.length-1;i>=0;i--){const p=particles[i];p.age+=dt;if(p.age>p.life){particles.splice(i,1);continue}p.x+=p.vx*dt;p.y+=p.vy*dt;p.z+=p.vz*dt;if(!p.id.startsWith('rain')&&p.id!=='snow'&&p.kind!==1&&p.kind!==4)p.vy-=6*dt;
            if(p.y<p.floor){if(p.id.startsWith('rain')&&level>0&&random()<.08)emit('water-ripple',{position:[p.x,p.floor+.02,p.z],lifetime:.6,scale:.25});particles.splice(i,1);continue}
        }
        for(let i=0;i<particles.length;i++){const p=particles[i];positions.set([p.x,p.y,p.z],i*3);colors.set(p.c,i*3);sizes[i]=p.size*(p.kind===1?1+p.age*.2:1);alphas[i]=(p.id.startsWith('rain')?Math.min(1,p.age*8):1)*(1-p.age/p.life);kinds[i]=p.kind}
        geometry.setDrawRange(0,particles.length);for(const a of Object.values(geometry.attributes))a.needsUpdate=true;
        for(let i=decals.length-1;i>=0;i--){const d=decals[i];d.age+=dt;d.mesh.material.opacity=Math.min(1,(d.life-d.age)/Math.min(5,d.life));if(d.ripple)d.mesh.scale.setScalar((1+d.age*7)*d.scale);if(d.age>d.life){d.mesh.removeFromParent();d.mesh.geometry.dispose();d.mesh.material.dispose();d.tex?.dispose();decals.splice(i,1)}}
        for(const e of objectEffects){e.age+=dt;e.clock.value=reduced?0:time;e.progress.value=e.id==='dissolve'?Math.min(1,e.age/e.life):e.id==='hit-flash'?Math.max(0,1-e.age*8):0;}
        if(water){water.material.uniforms.time.value=time*water.userData.speed;const a=water.geometry.attributes.position,base=water.userData.base;for(let i=0;i<a.count;i++)a.array[i*3+2]=Math.sin(base[i*3]*.3+time)*Math.cos(base[i*3+1]*.25-time*.6)*(water.userData.id==='water-ocean'?.25:.035);a.needsUpdate=true;water.geometry.computeVertexNormals();water.material.uniforms.sunDirection.value.copy(sun?.position||v.set(1,1,1)).normalize()}
        if(composer){bloom.enabled=level>0&&settings.has('bloom');bloom.strength=.3*(settings.get('bloom')?.intensity??1);const u=gradePass.uniforms;u.time.value=time;for(const [param,id] of [['vignette','vignette'],['grain','film-grain'],['heat','heat-haze'],['underwater','underwater'],['grade','color-grade']])u[param].value=settings.has(id)?(reduced&&['heat','underwater'].includes(param)?0:Math.min(2,settings.get(id).intensity??1)):0}
        return {thunder};
    }
    function resize(){const size=renderer.getSize(new T.Vector2());composer?.setSize(size.x,size.y)}
    function reset(){particles.length=0;for(const d of decals){d.mesh.removeFromParent();d.mesh.geometry.dispose();d.mesh.material.dispose();d.tex?.dispose()}decals.length=0;geometry.setDrawRange(0,0);weatherCarry=0;stormAt=6;seed=71237;for(const e of objectEffects){e.age=0;e.progress.value=0;e.clock.value=0}}
    return {dimension:'3d',set,emit,update,resize,reset,
        clear(){settings.clear();reset();scene.fog=old.fog;if(sky)sky.visible=stars.visible=clouds.visible=moon.visible=false;if(water){water.removeFromParent();water.geometry.dispose();water.material.dispose();water.dispose();waterNormals.dispose();water=null}},
        quality(n,m){level=n;reduced=m;if(composer)composer.setPixelRatio(Math.min(renderer.getPixelRatio(),n===0?.75:n===1?1:1.5))},
        blood(enabled){blood=enabled;if(!blood)for(const d of decals)if(!d.ripple)d.life=0},
        render(){if(composer)composer.render(0);else renderer.render(scene,camera)},
        registerSurface(mesh,kind='ground'){surfaces.set(mesh,kind);return()=>surfaces.delete(mesh)},
        applyObject(object,id,options={}){
            if(!['hologram','dissolve','hit-flash'].includes(id))throw Error('presentation: unsupported object effect');
            if(reduced&&id==='hit-flash')return()=>{};
            objectReleases.get(object)?.();if(objectReleases.size>=128)objectReleases.values().next().value();
            const restore=[];
            object.traverse(node=>{
                if(!node.isMesh)return;const old=node.material;
                const materials=(Array.isArray(old)?old:[old]).map(original=>{
                    const m=original.clone(),entry={material:m,id,age:0,life:options.lifetime||2,progress:{value:0},clock:{value:0}};
                    m.transparent=true;
                    m.onBeforeCompile=shader=>{
                        shader.uniforms.auraProgress=entry.progress;shader.uniforms.auraTime=entry.clock;
                        shader.vertexShader='varying vec3 auraPosition;\n'+shader.vertexShader.replace('#include <begin_vertex>','#include <begin_vertex>\nauraPosition=position;');
                        shader.fragmentShader='uniform float auraProgress,auraTime;varying vec3 auraPosition;\n'+shader.fragmentShader.replace('#include <dithering_fragment>',
                            id==='dissolve'?`float n=fract(sin(dot(floor(auraPosition*35.),vec3(12.9898,78.233,37.719)))*43758.5453);if(n<auraProgress)discard;gl_FragColor.rgb+=vec3(1.,.3,.04)*(1.-smoothstep(0.,.07,n-auraProgress));`:
                            id==='hologram'?`float scan=.6+.4*sin(auraPosition.y*90.-auraTime*5.);gl_FragColor=vec4(vec3(.13,.72,1.)*scan,.38+scan*.3);`:
                            'gl_FragColor.rgb=mix(gl_FragColor.rgb,vec3(1.),auraProgress);');
                    };
                    m.customProgramCacheKey=()=> 'aurago-'+id;objectEffects.push(entry);
                    restore.push(()=>{m.dispose();const i=objectEffects.indexOf(entry);if(i>=0)objectEffects.splice(i,1)});return m;
                });
                node.material=Array.isArray(old)?materials:materials[0];restore.push(()=>{node.material=old});
            });
            let done=false;const release=()=>{if(done)return;done=true;restore.forEach(f=>f());objectReleases.delete(object)};objectReleases.set(object,release);return release;
        },
        stats(){return {particles:particles.length,decals:decals.length,surfaces:surfaces.size,objects:objectEffects.length}},
        dispose(){if(disposed)return;disposed=true;reset();group.removeFromParent();for(const release of objectReleases.values())release();owned.forEach(x=>x.dispose());if(water){water.geometry.dispose();water.material.dispose();water.dispose();waterNormals.dispose()}composer?.dispose();bloom?.dispose();gradePass?.dispose();outputPass?.dispose();surfaces.clear();scene.background=old.background;scene.fog=old.fog;if(sun&&old.sun){sun.color.copy(old.sun.color);sun.intensity=old.sun.intensity;sun.position.copy(old.sun.position)}if(ambient)ambient.intensity=old.ambient},
    };
}
