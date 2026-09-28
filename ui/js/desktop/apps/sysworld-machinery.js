import * as THREE from 'three';
import {livingPlaces} from './sysworld-places.js';

export function createMachinery(scene) {
  const root=new THREE.Group();root.name='living-machinery';scene.add(root);
  const geometries=new Set(),materials=new Set(),stations=new Map();let clock=0,tier='high';
  const ownG=g=>(geometries.add(g),g),ownM=m=>(materials.add(m),m);
  const ring=ownG(new THREE.TorusGeometry(1,.025,6,64));
  const glow=ownG(new THREE.CircleGeometry(1,48));
  const pointGeometry=ownG(new THREE.BufferGeometry()),data=new Float32Array(96*3),seeds=new Float32Array(96);
  for(let i=0;i<96;i++){data[i*3]=Math.cos(i*2.399)*Math.sqrt(i/96);data[i*3+2]=Math.sin(i*2.399)*Math.sqrt(i/96);seeds[i]=i/96;}
  pointGeometry.setAttribute('position',new THREE.BufferAttribute(data,3));pointGeometry.setAttribute('phase',new THREE.BufferAttribute(seeds,1));
  for(const p of livingPlaces) {
    const group=new THREE.Group();group.name=p.id;group.position.set(p.x,.12,p.z);group.visible=false;root.add(group);
    const material=ownM(new THREE.MeshBasicMaterial({color:0x72e4df,transparent:true,opacity:.15,depthWrite:false,blending:THREE.AdditiveBlending,toneMapped:false}));
    const waves=[];
    for(let j=0;j<3;j++){const m=new THREE.Mesh(ring,ownM(material.clone()));m.rotation.x=-Math.PI/2;group.add(m);waves.push(m);}
    const base=new THREE.Mesh(glow,ownM(material.clone()));base.rotation.x=-Math.PI/2;base.scale.setScalar(2.5);base.position.y=.2;group.add(base);
    const motes=new THREE.Points(pointGeometry,ownM(new THREE.ShaderMaterial({transparent:true,depthWrite:false,blending:THREE.AdditiveBlending,
      uniforms:{time:{value:0},strength:{value:0},color:{value:new THREE.Color(0x9eeae0)}},
      vertexShader:'attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}',
      fragmentShader:'uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}',
    })));group.add(motes);
    stations.set(p.id,{group,waves,base,motes,district:p.district,started:0,until:0,source:'ambient',model:null});
  }
  const scan=new THREE.Mesh(ownG(new THREE.PlaneGeometry(3.9,2.3)),ownM(new THREE.ShaderMaterial({transparent:true,depthWrite:false,side:THREE.DoubleSide,blending:THREE.AdditiveBlending,
    uniforms:{time:{value:0},strength:{value:0}},vertexShader:'varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',
    fragmentShader:'varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}',
  })));scan.position.set(0,1.4,.8);stations.get('repair-bay').group.add(scan);
  const fountain=stations.get('kinetic-fountain');
  const water=ownM(new THREE.ShaderMaterial({transparent:true,depthWrite:false,side:THREE.DoubleSide,
    uniforms:{time:{value:0}},
    vertexShader:'varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',
    fragmentShader:'varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}',
  }));
  for(let i=0;i<8;i++) {
    const a=i*Math.PI/4,start=new THREE.Vector3(Math.cos(a)*2.3,.5,Math.sin(a)*2.3),end=new THREE.Vector3(Math.cos(a+.5)*.7,.55,Math.sin(a+.5)*.7);
    const curve=new THREE.QuadraticBezierCurve3(start,new THREE.Vector3(Math.cos(a)*1.5,3.7,Math.sin(a)*1.5),end);
    fountain.group.add(new THREE.Mesh(ownG(new THREE.TubeGeometry(curve,24,.045,5,false)),water));
  }
  const surface=ownM(new THREE.ShaderMaterial({transparent:true,depthWrite:false,
    uniforms:{time:{value:0}},
    vertexShader:'varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}',
    fragmentShader:'varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}',
  }));
  const basin=new THREE.Mesh(ownG(new THREE.CircleGeometry(2.58,64)),surface);basin.rotation.x=-Math.PI/2;basin.position.y=.3;fountain.group.add(basin);
  return {
    attach(id,model){const s=stations.get(id);if(s){s.model=model;s.group.visible=true;}},
    demonstrate(id,duration=6,status=null){const s=stations.get(id);if(!s)return;if(s.until<=clock)s.started=clock;s.until=clock+duration;s.source=status?'live':'ambient';s.status=status;s.model?.play('operate');},
    clear(){stations.forEach(s=>{s.until=0;s.source='ambient';s.status=null;});},
    setTier(value){tier=value;},
    update(dt,animated) {
      if(animated)clock+=dt;water.uniforms.time.value=surface.uniforms.time.value=scan.material.uniforms.time.value=clock;
      for(const [id,s]of stations) {
        const operating=s.until>clock,constant=id==='kinetic-fountain';
        const fade=operating?THREE.MathUtils.smoothstep(clock-s.started,0,.35)*THREE.MathUtils.smoothstep(s.until-clock,0,.65):0;
        const strength=constant?Math.max(.25,fade):fade;
        if(!operating){s.source='ambient';s.status=null;}
        const color=s.status==='failed'?0xff6659:s.source==='live'?({infra:0x79bdff,integrations:0x75ebd6,missions:0x9caeff,graph:0xffd394,operations:0xffb883})[s.district]:0x72e4df;
        s.waves.forEach((m,i)=>{m.visible=operating;const t=(clock*.6+i/3)%1;m.position.y=.4+t*2;m.scale.setScalar(.4+t*2.6);m.material.opacity=(1-t)*.22*fade;m.material.color.setHex(color);});
        if(id==='repair-bay'){scan.visible=operating;scan.material.uniforms.strength.value=fade;}
        if(id==='relay-mast'&&operating){const antenna=s.model?.node.getObjectByName('antenna');if(antenna)antenna.rotation.y=THREE.MathUtils.damp(antenna.rotation.y,Math.atan2(-s.group.position.x,-12-s.group.position.z),3,animated?dt:0);}
        s.base.material.opacity=.08*fade;
        s.motes.visible=tier!=='low'&&strength>0;s.motes.material.uniforms.time.value=clock;s.motes.material.uniforms.strength.value=strength;
        if(s.model?.mixer)s.model.mixer.timeScale=animated&&(operating||constant)?.65:0;
      }
    },
    stats:()=>({places:[...stations].filter(([,s])=>s.model).map(([id,s])=>({id,operating:s.until>clock,source:s.source})),time:clock}),
    dispose(){root.removeFromParent();geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());stations.clear();},
  };
}
