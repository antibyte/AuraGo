import * as THREE from 'three';
import {daylight} from './sysworld-exploration.js';

export function createWeather(scene,{sun,rim,hemisphere,atmosphere,ground}){
  let timeMode='local',weather='clear',tier='high',indoor=false,last=-100,disposed=false;
  const time={value:0},geometry=new THREE.BufferGeometry(),points=[];
  let seed=317;const random=()=>((seed=Math.imul(seed,1664525)+1013904223>>>0)/4294967296);
  for(let i=0;i<1200;i++)points.push((random()-.5)*170,random()*60,(random()-.5)*174-7);
  geometry.setAttribute('position',new THREE.Float32BufferAttribute(points,3));
  const material=new THREE.ShaderMaterial({transparent:true,depthWrite:false,uniforms:{time},
    vertexShader:'uniform float time;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.0);gl_PointSize=3.0;}',
    fragmentShader:'void main(){float a=(1.0-abs(gl_PointCoord.x-.5)*2.0)*.35;gl_FragColor=vec4(.6,.78,.86,a);}' });
  const rain=new THREE.Points(geometry,material);rain.frustumCulled=false;scene.add(rain);
  const dayFog=new THREE.Color(0x82999f),nightFog=new THREE.Color(0x08121e);
  function light(){
    const phase=daylight(timeMode),d=phase.amount;
    sun.intensity=.8+d*2.6;sun.color.set(phase.evening>.1?0xffd1a1:0xc8deff);
    sun.position.set(Math.cos(phase.hour/24*Math.PI*2)*120,40+d*115,85);
    rim.intensity=.7+phase.evening*1.6;hemisphere.intensity=.5+d*.65;
    scene.fog.color.copy(nightFog).lerp(dayFog,d);scene.fog.density=weather==='fog'?.018:weather==='rain'?.008:.0035;
    atmosphere.setLighting?.(d,phase.evening,sun.position);
    ground.material.roughness=weather==='rain'?.14:.44;
    rain.visible=weather==='rain'&&!indoor;geometry.setDrawRange(0,tier==='low'?200:1200);
  }
  light();
  return {
    set(values={}){if(['local','day','evening','night'].includes(values.time))timeMode=values.time;if(['clear','rain','fog'].includes(values.weather))weather=values.weather;light();},
    setTier(value){tier=value;light();},setIndoor(value){if(indoor!==value){indoor=value;light();}},
    update(dt,elapsed,animated){if(disposed)return;rain.visible=weather==='rain'&&!indoor&&animated;if(animated)time.value+=Math.min(dt,.1);if(elapsed-last>30){last=elapsed;light();return true;}return false;},
    stats:()=>({time:timeMode,weather,indoor,daylight:daylight(timeMode).amount}),
    dispose(){disposed=true;rain.removeFromParent();geometry.dispose();material.dispose();},
  };
}
