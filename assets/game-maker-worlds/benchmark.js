// Technical benchmark only. One owning loop per engine; no simulated frame data.
const dimension=new URLSearchParams(location.search).get('dimension')||'2d';
window.benchmark={ready:false,complete:false,dimension,mode:'browser requestAnimationFrame sample'};
addEventListener('error',e=>benchmark.error=e.message);addEventListener('unhandledrejection',e=>benchmark.error=String(e.reason));
const durations=[];let elapsed=0,last=0;
function sample(now,stats,gl){
  if(last&&elapsed>2000)durations.push(now-last);if(last)elapsed+=now-last;last=now;
  if(elapsed<12000)return;
  if(benchmark.complete)return;
  const sorted=durations.slice().sort((a,b)=>a-b),debug=gl?.getExtension('WEBGL_debug_renderer_info');
  Object.assign(benchmark,{complete:true,frames:durations.length,median_ms:sorted[Math.floor(sorted.length*.5)],p95_ms:sorted[Math.floor(sorted.length*.95)],p99_ms:sorted[Math.floor(sorted.length*.99)],fps:durations.length*1000/durations.reduce((a,b)=>a+b,0),gpu:debug?gl.getParameter(debug.UNMASKED_RENDERER_WEBGL):'unavailable',viewport:[innerWidth,innerHeight],...stats});
  document.getElementById('stats').textContent=JSON.stringify(benchmark,null,2);
}
if(dimension==='2d'){
  const A=await import('/runtime/aurago-game-1.js'),I=await import('/runtime/isometric.js');
  const meta=await(await fetch('/packs/aurago-isometric/manifest.json')).json();
  const ids=['terrain-grass-flat','people-adventurer'],selected={...meta,assets:meta.assets.filter(a=>ids.includes(a.id)),animations:meta.animations.filter(a=>ids.includes(a.asset_id))};
  const frames=new Set([...selected.assets.flatMap(a=>a.frames),...selected.animations.flatMap(a=>a.frames)]);selected.frames=meta.frames.filter(f=>frames.has(f.id));const pages=new Set(selected.frames.map(f=>f.atlas));selected.atlases=meta.atlases.filter(p=>pages.has(p.file));
  let world;
  new Phaser.Game({type:Phaser.WEBGL,width:1920,height:1080,pixelArt:true,backgroundColor:'#112030',scene:{
    preload(){A.preloadPack(this,selected,'/packs/aurago-isometric/');},
    create(){A.registerAnimations(this,selected);const nodes=[];for(let x=0;x<50;x++)for(let y=0;y<40;y++)nodes.push({id:`floor-${x}-${y}`,position:[x,y,0],properties:{walkable:true}});for(let i=0;i<40;i++)nodes.push({id:'actor-'+i,position:[5+i,20,0]});
      world=I.createIsometricWorld({schema_version:2,dimension:'2d',projection:{kind:'isometric'},levels:[{id:'main',active:true}],nodes},{origin:{x:2600,y:100},create:n=>{const actor=n.id.startsWith('actor'),s=A.createAsset(this,selected,actor?ids[1]:ids[0],0,0);if(actor)A.playAction(s,'walk');return s;},position:(s,p,d)=>s.setPosition(p.x,p.y).setDepth(d),destroy:s=>s.destroy()});
      this.cameras.main.setZoom(.31).centerOn(2900,1560);benchmark.ready=true;
      this.events.once('shutdown',()=>world.dispose());
    },
    update(now,dt){for(let i=0;i<40;i++)world.move('actor-'+i,Math.sin(now*.001+i)*dt*.0003,Math.cos(now*.001+i)*dt*.0003);sample(performance.now(),{static_objects:2000,animated_objects:40,objects:world.inspect().objects},this.game.renderer.gl);}
  }});
}else{
  const A=await import('/runtime/aurago-three-assets-1.js'),T=A.THREE;
  const meta=await(await fetch('/packs/aurago-pirates-3d/manifest.json')).json(),base='/packs/aurago-pirates-3d/';
  const renderer=new T.WebGLRenderer({antialias:true});renderer.setSize(1920,1080);renderer.setPixelRatio(1);document.body.append(renderer.domElement);
  const scene=new T.Scene(),camera=new T.PerspectiveCamera(48,1920/1080,.1,1000);scene.background=new T.Color('#557989');camera.position.set(125,150,190);camera.lookAt(0,0,0);scene.add(new T.HemisphereLight(0xcceeff,0x775544,2));const sun=new T.DirectionalLight(0xffebd3,3);sun.position.set(15,60,25);scene.add(sun);
  const water=new T.Mesh(new T.PlaneGeometry(240,200),new T.MeshStandardMaterial({color:0x326a83,roughness:.3}));water.rotation.x=-Math.PI/2;water.position.y=-1.2;scene.add(water);
  const handles=await Promise.all(['ships-sloop','people-pirate-captain','animals-reef-shark'].map(id=>A.loadAsset(meta,id,base))),instances=[];
  for(let group=0;group<3;group++)for(let i=0;i<12;i++){const instance=A.createInstance(handles[group]);instance.root.position.set((i%4-1.5)*42,0,(Math.floor(i/4)-1)*48+group*8);if(group===1)instance.root.position.y=1;instance.forcedLOD=group===0?1:0;scene.add(instance.root);A.playAction(instance,group===0?'sailing':group===1?'walk':'swim');instances.push(instance);}
  benchmark.ready=true;let previous=performance.now();function frame(now){const dt=Math.min(.1,(now-previous)/1000);previous=now;for(const instance of instances)A.updateInstance(instance,dt,camera);renderer.render(scene,camera);sample(now,{ships:12,animated_objects:24,draw_calls:renderer.info.render.calls,triangles:renderer.info.render.triangles,ship_lod:1},renderer.getContext());if(!benchmark.complete)requestAnimationFrame(frame);}requestAnimationFrame(frame);
  window.disposeBenchmark=()=>{instances.forEach(A.disposeInstance);handles.forEach(A.releaseAsset);water.geometry.dispose();water.material.dispose();renderer.dispose();renderer.forceContextLoss();return A.assetCacheSize();};
}
