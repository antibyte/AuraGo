import {addFilter} from './phaser-filter.js';
// MIT. Camera-relative atmosphere canvases are uploaded through Phaser's texture manager.
export {createPresentation} from './presentation.js';
export function createPhaserAdapter({scene,view='top',report=console.warn}) {
    const id='aura-fx-'+Math.random().toString(36).slice(2),w=scene.scale.width,h=scene.scale.height;
    const back=scene.textures.createCanvas(id+'-sky',w,h),front=scene.textures.createCanvas(id+'-fx',w,h);
    const sky=scene.add.image(0,0,back.key).setOrigin(0).setScrollFactor(0).setDepth(-10000);
    const layer=scene.add.image(0,0,front.key).setOrigin(0).setScrollFactor(0).setDepth(900);
    const b=back.context,c=front.context,settings=new Map(),particles=[],decals=[],surfaces=new Map(),objects=[];
    let post,hudCamera;const cameraFlags=new Map(),webgl=scene.game.renderer.type!==1;
    let level=2,reduced=false,seed=1337,blood=true,time=0,carry=0,disposed=false,stormAt=6;
    const random=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed/4294967296};
    const limits=[250,750,1500];
    const intensity=id=>Math.min(2,Math.max(0,settings.get(id)?.intensity??(settings.has(id)?1:0)));
    function set(id,p){
        if(['bloom','color-grade','vignette','film-grain','heat-haze','underwater','day-night'].includes(id)&&!post){
            if(webgl){post=addFilter(scene,scene.cameras.main);hudCamera=scene.cameras.add(0,0,w,h).setBackgroundColor('rgba(0,0,0,0)')}
            else report('presentation: Canvas fallback uses color overlays; distortion and object shaders require WebGL');
        }
        if(id.startsWith('sky-'))for(const k of settings.keys())if(k.startsWith('sky-'))settings.delete(k);settings.set(id,p)}
    function screen(position){const cam=scene.cameras.main;return [(position?.[0]||0)-cam.scrollX,(position?.[1]||0)-cam.scrollY]}
    function floor(x){const cam=scene.cameras.main;let y=h-5;for(const object of surfaces.keys()){if(object.active===false)continue;const r=object.getBounds?.()||object;if(x+cam.scrollX>=r.x&&x+cam.scrollX<=r.x+r.width)y=Math.min(y,r.y-cam.scrollY)}return y}
    function spawn(id,p,at){
        if(particles.length>=limits[level])return;
        const rain=id.startsWith('rain'),fog=id.startsWith('fog')||id==='smoke',snow=id==='snow';
        const color=id.startsWith('blood')?p.color||'#9f162a':rain?'#bee1f1':snow?'#edf7ff':id==='water-splash'?'#b3e8f4':id==='stone-debris'?'#a69b89':id==='wind-leaves'?'#9ca952':fog?'#afc6d0':['magic','teleport','pickup-glow','engine-trail'].includes(id)?'#78dcff':'#ffc067';
        const pos=at||p.position||[w/2,h/2,0];
        particles.push({id,x:pos[0],y:pos[1],vx:(random()-.5)*(rain?30:90),vy:rain?550:snow?28:fog?-4:(random()-.6)*150,age:0,life:rain?2:snow?15:fog?6:p.lifetime||1.3,size:(rain?14:snow?2:fog?45:id.startsWith('blood')?2:3)*(p.scale||1),color:p.color&&p.color!=='#ffffff'?p.color:color,screen:!!at});
        const e=particles[particles.length-1];if(id==='fire'){e.size=12*(p.scale||1);e.vx*=.15;e.vy=-45;e.life=.8}if(id==='muzzle-flash'||id==='hit-flash'){e.size=15;e.life=.12;e.vx=e.vy=0}
    }
    function emit(id,p){
        if(!['blood-pool','blood-decal','blood-spray','hit-flash','metal-sparks','stone-debris','fire','smoke','embers','explosion','muzzle-flash','engine-trail','magic','teleport','pickup-glow','water-splash','water-ripple'].includes(id))return;
        if(id.startsWith('blood')&&!blood)return;
        if(reduced&&(id==='hit-flash'||id==='muzzle-flash'))return;
        if(id==='blood-decal'||id==='blood-pool'||id==='water-ripple'){
            while(decals.length>=[24,48,96][level])decals.shift();const [x,y]=p.position||[w/2,h/2,0];const shape=Array.from({length:18},(_,i)=>{const a=i/18*Math.PI*2,r=10+random()*13;return[Math.cos(a)*r,Math.sin(a)*r]});
            decals.push({id,x,y,shape,color:p.color||'#881628',age:0,life:p.lifetime||30,scale:p.scale||1});return;
        }
        for(let i=0;i<Math.round((id==='explosion'?65:id==='muzzle-flash'?6:20)*(p.intensity??1));i++)spawn(id,p);
    }
    function drawSky(){
        const skyID=[...settings.keys()].find(k=>k.startsWith('sky-')),cycle=settings.get('day-night');
        b.clearRect(0,0,w,h);if(!skyID&&!cycle)return;
        const hour=cycle?(cycle.fixed?cycle.hour:(cycle.hour+time*24/Math.max(1,cycle.cycle))%24):skyID==='sky-night'||skyID==='sky-space'?0:skyID==='sky-sunset'?17.4:11;
        const day=Math.max(0,Math.sin((hour-6)/24*Math.PI*2)),night=1-Math.min(1,day*3),storm=skyID==='sky-storm';
        const g=b.createLinearGradient(0,0,0,h);g.addColorStop(0,night>.6?'#060d24':storm?'#263647':day<.25?'#524668':'#376b9a');g.addColorStop(1,night>.6?'#172a45':storm?'#7a8991':day<.25?'#efac76':'#bcdde4');b.fillStyle=g;b.fillRect(0,0,w,h);
        const orbX=w*(.18+hour/24*.6),orbY=h*.2;b.globalAlpha=night>.6?.8:1;
        const glow=b.createRadialGradient(orbX,orbY,5,orbX,orbY,70);glow.addColorStop(0,night>.6?'#c9dbed66':'#fff5cc99');glow.addColorStop(1,'#ffffff00');b.fillStyle=glow;b.fillRect(orbX-70,orbY-70,140,140);b.fillStyle=night>.6?'#d6e0e7':'#fff6d4';b.beginPath();b.arc(orbX,orbY,night>.6?13:19,0,7);b.fill();
        if(night>.4){for(let i=0;i<130;i++){const x=((i*773)%w),y=((i*131)%Math.floor(h*.75));b.globalAlpha=.2+((i%7)/10);b.fillStyle='#cde3ff';b.fillRect(x,y,i%11===0?2:1,1)}}
        b.globalAlpha=1;
        if(skyID==='sky-space'){const n=b.createRadialGradient(w*.35,h*.35,20,w*.35,h*.35,w*.6);n.addColorStop(0,'#70438744');n.addColorStop(.45,'#203b7833');n.addColorStop(1,'#00000000');b.fillStyle=n;b.fillRect(0,0,w,h)}
        if(skyID!=='sky-space'){
            const cover=skyID==='sky-cloudy'||storm?18:8;
            for(let i=0;i<cover;i++){const x=((i*173+time*(reduced?0:3))%(w+220))-110,y=40+i%4*32;
                const cloud=b.createRadialGradient(0,0,4,0,0,90);cloud.addColorStop(0,night>.6?'#61738c22':storm?'#10213377':'#eef5fc77');cloud.addColorStop(1,'#ffffff00');b.fillStyle=cloud;b.save();b.translate(x,y);b.scale(1.8,.4);b.fillRect(-95,-95,190,190);b.restore();
            }
        }
    }
    function drawWater(){
        const c=b; // Water surfaces sit behind actors; splashes remain in the foreground.
        const id=[...settings.keys()].find(k=>['water-lake','water-river','water-ocean'].includes(k));if(!id)return;const p=settings.get(id),at=p.position?screen(p.position):[w/2,(view==='side'?h*.72:h*.55)+h*(p.y||0)/100],width=w*(p.width??80)/100,depth=h*(p.depth??80)/100,base=at[1];
        c.save();c.beginPath();c.rect(at[0]-width/2,base-7,width,depth+7);c.clip();
        const g=c.createLinearGradient(0,base,0,h);g.addColorStop(0,'#2eafc5bb');g.addColorStop(1,'#123d68ef');c.fillStyle=g;c.beginPath();
        for(let x=0;x<=w;x+=8){const y=base+Math.sin(x*.025+time*(p.speed||.6))*4+Math.cos(x*.053-time)*2;x?c.lineTo(x,y):c.moveTo(x,y)}c.lineTo(w,h);c.lineTo(0,h);c.fill();
        for(let j=0;j<Math.min(50,Math.ceil(depth/14));j++){c.strokeStyle=j===0?'#c6f4ec99':'#a5e4e22b';c.lineWidth=j===0?2:1;c.beginPath();for(let x=0;x<w;x+=10){const y=base+j*14+Math.sin(x*.03+time*(p.speed??.6)*(id==='water-river'?3:1)-j)*(id==='water-ocean'?4:2);x?c.lineTo(x,y):c.moveTo(x,y)}c.stroke()}c.restore();
    }
    function update(dt,t){
        time=t;let thunder=false;const cycle=settings.get('day-night'),hour=cycle?(cycle.fixed?cycle.hour:(cycle.hour+time*24/Math.max(1,cycle.cycle))%24):12;
        const darkness=cycle?1-(.28+.72*Math.max(0,Math.sin((hour-6)/24*Math.PI*2))):0;
        if(post){
            const main=scene.cameras.main;
            for(const object of cameraFlags.keys())if(!object.scene)cameraFlags.delete(object);
            for(const object of scene.children.list){if(!cameraFlags.has(object))cameraFlags.set(object,object.cameraFilter);object.cameraFilter=cameraFlags.get(object)|(object.depth>=1000?main.id:hudCamera.id)}
            post.uniforms.auraTime=time;post.uniforms.auraFlags=[intensity('vignette'),level>0?intensity('film-grain'):0,!reduced?intensity('heat-haze'):0,!reduced?intensity('underwater'):0];post.uniforms.auraMore=[level>0?intensity('bloom'):0,intensity('color-grade'),darkness,0];
        }
        drawSky();c.clearRect(0,0,w,h);drawWater();
        const weather=[...settings.keys()].find(k=>['rain-light','rain-heavy','snow','wind-dust','wind-leaves'].includes(k));
        if(weather){carry+=dt*(weather==='rain-heavy'?330:weather==='rain-light'?110:30)*[.3,.6,1][level]*Math.min(3,settings.get(weather).intensity??1);let n=Math.min(32,Math.floor(carry));carry=Math.min(1,carry-n);while(n-->0)spawn(weather,settings.get(weather),[random()*w,-20])}
        for(const id of ['fire','smoke','embers','engine-trail','fog-ground','fog-zone'])if(settings.has(id)&&(id.startsWith('fog')||settings.get(id).position)&&random()<dt*12*Math.min(3,settings.get(id).intensity??1)){const p=settings.get(id);if(id==='fog-zone'){const at=screen(p.position||[w/2,h*.65,0]),radius=p.radius||120;spawn(id,p,[at[0]+(random()-.5)*radius*2,at[1]+(random()-.5)*radius])}else spawn(id,p,id==='fog-ground'?[random()*w,h*.6+random()*h*.35]:null)}
        for(let i=decals.length-1;i>=0;i--){const d=decals[i];d.age+=dt;if(d.age>d.life){decals.splice(i,1);continue}c.save();c.translate(d.x-scene.cameras.main.scrollX,d.y-scene.cameras.main.scrollY);c.scale(d.scale,d.scale);c.globalAlpha=Math.min(1,(d.life-d.age)/3);c.fillStyle=d.color;
            if(d.id==='water-ripple'){c.strokeStyle='#b9f0ef';c.lineWidth=1.4;c.beginPath();c.ellipse(0,0,4+d.age*18,2+d.age*8,0,0,7);c.stroke()}else{c.beginPath();d.shape.forEach(([x,y],i)=>i?c.lineTo(x,y):c.moveTo(x,y));c.closePath();c.fill()}c.restore();
        }
        for(let i=particles.length-1;i>=0;i--){const p=particles[i];p.age+=dt;p.x+=p.vx*dt;p.y+=p.vy*dt;const x=p.x-(p.screen?0:scene.cameras.main.scrollX),y=p.y-(p.screen?0:scene.cameras.main.scrollY);if(p.age>p.life||y>floor(x)){if(p.id.startsWith('rain')&&level>0&&random()<.1)emit('water-ripple',{position:[x+scene.cameras.main.scrollX,floor(x)+scene.cameras.main.scrollY,0],lifetime:.5,scale:.3});particles.splice(i,1);continue}
            c.globalAlpha=(p.id.startsWith('rain')?Math.min(1,p.age*12):1)*(1-p.age/p.life);c.fillStyle=p.color;c.strokeStyle=p.color;
            if(p.id.startsWith('rain')){c.lineWidth=.8;c.beginPath();c.moveTo(x,y);c.lineTo(x-p.vx*.02,y-p.size);c.stroke()}
            else if(p.id==='fire'){const g=c.createRadialGradient(x,y,0,x,y,p.size);g.addColorStop(0,'#fff1b0ee');g.addColorStop(.3,'#ffac33bb');g.addColorStop(1,'#ef391000');c.fillStyle=g;c.fillRect(x-p.size,y-p.size,p.size*2,p.size*2)}
            else if(p.id.startsWith('fog')||p.id==='smoke'){const r=p.size*(1+p.age*.2),g=c.createRadialGradient(x,y,0,x,y,r);g.addColorStop(0,p.color+'30');g.addColorStop(1,p.color+'00');c.fillStyle=g;c.fillRect(x-r,y-r,r*2,r*2)}
            else{c.save();c.translate(x,y);c.rotate(p.age);c.beginPath();c.ellipse(0,0,p.size,p.id==='wind-leaves'?p.size*.4:p.size,0,0,7);c.fill();c.restore()}
        }
        c.globalAlpha=1;
        if(settings.has('fog-distance')){c.globalAlpha=Math.min(2,(settings.get('fog-distance').density??.018)/.018)/2;const g=c.createLinearGradient(0,0,0,h);g.addColorStop(0,'#adc8d080');g.addColorStop(1,'#adc8d010');c.fillStyle=g;c.fillRect(0,0,w,h);c.globalAlpha=1}
        if(!post&&settings.has('vignette')){const g=c.createRadialGradient(w/2,h/2,h*.1,w/2,h/2,w*.65);g.addColorStop(0,'#00000000');g.addColorStop(1,'#020617bb');c.fillStyle=g;c.fillRect(0,0,w,h)}
        if(!post&&settings.has('color-grade')){c.fillStyle='#f7b36308';c.fillRect(0,0,w,h)}
        if(!post&&settings.has('underwater')){c.fillStyle='#14658d45';c.fillRect(0,0,w,h)}
        if(!post&&darkness){c.fillStyle='rgba(6,14,35,'+darkness+')';c.fillRect(0,0,w,h)}
        if(!post&&settings.has('film-grain')&&level>0){c.fillStyle='#d5eeff';c.globalAlpha=.06;for(let i=0;i<180;i++)c.fillRect(random()*w,random()*h,1,1);c.globalAlpha=1}
        if(settings.has('thunderstorm')&&time>stormAt){thunder=true;stormAt=time+6+random()*10;if(!reduced){c.fillStyle='#dfedff55';c.fillRect(0,0,w,h)}}
        for(const e of [...objects]){if(!e.object.scene){e.release();continue}e.age+=dt;if(e.id==='hit-flash'&&e.age>.13){e.release();continue}if(e.filter){e.filter.uniforms.auraTime=reduced?0:time;e.filter.uniforms.auraObject=[e.id==='hologram'?1:e.id==='dissolve'?2:3,e.id==='dissolve'?Math.min(1,e.age/e.life):e.id==='hit-flash'?Math.max(0,1-e.age*8):0,0,0];continue}if(e.id==='dissolve')e.object.setAlpha(Math.max(0,1-e.age/e.life));else if(e.id==='hologram'){e.object.setAlpha(.5+(reduced?0:Math.sin(time*8)*.15));e.object.setTint?.(0x66ddff)}else{e.object.setTint?.(e.age<.1?0xffffff:e.tint)}}
        back.refresh();front.refresh();return {thunder};
    }
    return {dimension:'2d',set,emit,update,
        clear(){settings.clear();particles.length=decals.length=0;},
        quality(n,m){level=n;reduced=m;const ratio=[.5,.75,1][n];for(const tex of [back,front]){tex.setSize(Math.round(w*ratio),Math.round(h*ratio));tex.context.setTransform(ratio,0,0,ratio,0,0)}sky.setSize(back.width,back.height).setDisplaySize(w,h);layer.setSize(front.width,front.height).setDisplaySize(w,h)},blood(v){blood=v;if(!v)for(const d of decals)if(d.id.startsWith('blood'))d.life=0},
        registerSurface(object,kind='ground'){surfaces.set(object,kind);return()=>surfaces.delete(object)},
        applyObject(object,id,p={}){if(!['hologram','dissolve','hit-flash'].includes(id))throw Error('presentation: unsupported object effect');if(reduced&&id==='hit-flash')return()=>{};objects.find(e=>e.object===object)?.release();while(objects.length>=128)objects[0].release();const e={object,id,age:0,life:p.lifetime||2,alpha:object.alpha,tint:object.tintTopLeft};if(webgl){object.enableFilters();e.filter=addFilter(scene,object.filterCamera)}objects.push(e);let done=false;e.release=()=>{if(done)return;done=true;if(e.filter)object.filterCamera?.filters?.internal?.remove(e.filter);if(object.scene){object.setAlpha(e.alpha);object.setTint?.(e.tint)}objects.splice(objects.indexOf(e),1)};return e.release},
        reset(){for(const e of objects){e.age=0;e.object.setAlpha(e.alpha);e.object.setTint?.(e.tint)}particles.length=decals.length=0;carry=0;stormAt=6;seed=1337;c.clearRect(0,0,w,h);front.refresh()},
        stats(){return {particles:particles.length,decals:decals.length,surfaces:surfaces.size,objects:objects.length,canvas_fallback:scene.game.renderer.type===1?1:0}},
        dispose(){if(disposed)return;disposed=true;for(const e of [...objects])e.release();particles.length=decals.length=0;surfaces.clear();if(post)scene.cameras.main.filters.internal.remove(post);if(hudCamera)scene.cameras.remove(hudCamera);for(const [object,flags]of cameraFlags)if(object.active)object.cameraFilter=flags;cameraFlags.clear();sky.destroy();layer.destroy();scene.textures.remove(back.key);scene.textures.remove(front.key)},
    };
}
