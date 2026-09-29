import * as THREE from 'three';
import {daylight} from './sysworld-exploration.js';

// The visible sun/moon disc sweeps from the harbour side at dawn over the camera side at noon
// to behind the skyline at dusk; at night it carries the moon above the far towers. The key
// light keeps its front-lit path so glass facades never mirror a backlight into the camera.
const sunAzimuth=hour=>.5+(hour-6)/12*3.62;
const skyDirection=new THREE.Vector3();
export function createWeather(scene,{sun,rim,hemisphere,atmosphere,surfaces,onThunder}){
  let timeMode='local',weather='clear',tier='high',indoor=false,last=-100,disposed=false,hemiBase=hemisphere.intensity;
  let nextStrike=14,strike=-1,strikes=0;
  const time={value:0},geometry=new THREE.BufferGeometry(),points=[];
  let seed=317;const random=()=>((seed=Math.imul(seed,1664525)+1013904223>>>0)/4294967296);
  for(let i=0;i<1200;i++)points.push((random()-.5)*170,random()*60,(random()-.5)*174-7);
  geometry.setAttribute('position',new THREE.Float32BufferAttribute(points,3));
  // Rain renders as thin motion-blurred streaks sized by distance.
  const material=new THREE.ShaderMaterial({transparent:true,depthWrite:false,uniforms:{time,flash:{value:0}},
    vertexShader:'uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}',
    fragmentShader:'uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}' });
  const rain=new THREE.Points(geometry,material);rain.frustumCulled=false;scene.add(rain);
  const dayFog=new THREE.Color(0x9bb7c9),nightFog=new THREE.Color(0x08121e),duskFog=new THREE.Color(0x2a2638);
  function light(){
    const phase=daylight(timeMode),d=phase.amount,a=sunAzimuth(phase.hour);
    sun.intensity=.8+d*2.6;sun.color.set(phase.evening>.1?0xffb27a:0xc8deff);
    sun.position.set(Math.cos(phase.hour/24*Math.PI*2)*120,40+d*115,85);
    skyDirection.set(Math.cos(a)*120,40-phase.evening*18+d*115,Math.sin(a)*120);
    rim.intensity=.7+phase.evening*1.6;hemiBase=hemisphere.intensity=.5+d*.65;
    scene.fog.color.copy(nightFog).lerp(dayFog,d).lerp(duskFog,phase.evening*(1-d)*.5);
    scene.fog.density=weather==='fog'?.018:weather==='rain'?.008:.003-d*.0008;
    atmosphere.setLighting?.(d,phase.evening,skyDirection);atmosphere.setWeather?.(weather);
    surfaces?.setLighting(d,phase.evening,weather==='clear'?0:1,scene.fog.color);surfaces?.setWeather(weather);
    rain.visible=weather==='rain'&&!indoor;geometry.setDrawRange(0,tier==='low'?200:1200);
  }
  // Soft, brief lightning (never a strobe) with a distance-derived thunder delay.
  function lightning(dt,animated){
    if(weather!=='rain'||indoor||!animated){if(strike>=0){strike=-1;flash(0);}return;}
    nextStrike-=dt;
    if(strike<0&&nextStrike<=0){strike=0;strikes++;nextStrike=9+random()*16;onThunder?.(.6+random()*3.2);}
    if(strike<0)return;
    strike+=dt;
    const f=strike<.09?1:strike<.16?.25:strike<.24?.75:Math.max(0,1-(strike-.24)/.35)*.5;
    flash(f);if(strike>.6){strike=-1;flash(0);}
  }
  function flash(value){hemisphere.intensity=hemiBase+value*2.4;material.uniforms.flash.value=value;atmosphere.setFlash?.(value);}
  light();
  return {
    set(values={}){if(['local','day','evening','night'].includes(values.time))timeMode=values.time;if(['clear','rain','fog'].includes(values.weather))weather=values.weather;light();},
    setTier(value){tier=value;light();},setIndoor(value){if(indoor!==value){indoor=value;light();}},
    update(dt,elapsed,animated){if(disposed)return;rain.visible=weather==='rain'&&!indoor&&animated;if(animated)time.value+=Math.min(dt,.1);lightning(Math.min(dt,.1),animated);if(elapsed-last>30){last=elapsed;light();return true;}return false;},
    mood(){const phase=daylight(timeMode);return {day:phase.amount,evening:phase.evening,weather,indoor};},
    stats:()=>({time:timeMode,weather,indoor,daylight:daylight(timeMode).amount,strikes}),
    dispose(){disposed=true;rain.removeFromParent();geometry.dispose();material.dispose();},
  };
}
