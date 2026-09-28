import * as THREE from 'three';
import { ShaderPass } from 'three/addons/postprocessing/ShaderPass.js';

// Final cinematic grade after the output pass (display-referred input): light shafts from the
// sun/moon, anamorphic streaks on bright highlights, edge chromatic aberration, split toning,
// a gentle S-curve, vignette, luminance-aware grain and tour letterboxing. Every pow() base
// is clamped (see sysworld-atmosphere.js) so multisampled edges cannot produce NaN.
const POST = `uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
uniform vec2 lightPos;uniform vec3 shaftColor,shadowTint,highlightTint;varying vec2 vUv;
float hash2(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
float luma(vec3 c){return dot(c,vec3(0.2126,0.7152,0.0722));}
void main(){vec2 d=vUv-0.5;float r2=dot(d,d);
vec2 off=d*aberration*r2*4.0;
vec3 c=vec3(texture2D(tDiffuse,vUv+off).r,texture2D(tDiffuse,vUv).g,texture2D(tDiffuse,vUv-off).b);
if(streak>0.0){vec3 s=vec3(0.0);
for(int i=1;i<=6;i++){float o=float(i)*0.009,w=1.0-float(i)/7.0;
s+=(max(texture2D(tDiffuse,vUv+vec2(o,0.0)).rgb-0.8,0.0)+max(texture2D(tDiffuse,vUv-vec2(o,0.0)).rgb-0.8,0.0))*w;}
c+=vec3(luma(s))*vec3(0.35,0.62,1.0)*streak;}
if(shafts>0.0){vec2 delta=(vUv-lightPos)/16.0;vec2 uv=vUv;float decay=1.0,sum=0.0;
for(int i=0;i<16;i++){uv-=delta;vec2 q=(uv-lightPos)*vec2(aspect,1.0);
float l=luma(texture2D(tDiffuse,clamp(uv,0.0,1.0)).rgb);sum+=max(l-0.72,0.0)*decay*exp(-dot(q,q)*16.0);decay*=0.93;}
c+=shaftColor*sum*shafts;}
float l=luma(c);c=mix(vec3(l),c,saturation);
c+=shadowTint*(1.0-smoothstep(0.0,0.45,l))+highlightTint*smoothstep(0.5,1.0,l);
c=clamp(c,0.0,1.0);c=mix(c,c*c*(3.0-2.0*c),contrast);
c*=1.0-vignette*smoothstep(0.28,1.05,r2*2.4);
c+=(hash2(vUv*vec2(1920.0,1080.0)+fract(time)*7.0)-0.5)*grain*(0.45+0.55*(1.0-l));
float bar=letterbox*clamp((1.0-aspect/2.39)*0.5,0.0,0.12);
c*=smoothstep(bar,bar+0.003,vUv.y)*smoothstep(bar,bar+0.003,1.0-vUv.y);
gl_FragColor=vec4(c,1.0);}`;

export function createCinema(time) {
  const u = {
    tDiffuse: { value: null }, time, vignette: { value: .5 }, grain: { value: .03 }, aberration: { value: .0024 },
    streak: { value: .16 }, shafts: { value: 0 }, letterbox: { value: 0 }, aspect: { value: 16 / 9 },
    contrast: { value: .22 }, saturation: { value: 1.08 }, lightPos: { value: new THREE.Vector2(.5, .5) },
    shaftColor: { value: new THREE.Color(0xbcd8ff) }, shadowTint: { value: new THREE.Color(0x001a1e) }, highlightTint: { value: new THREE.Color(0x160c00) },
  };
  const pass = new ShaderPass(new THREE.ShaderMaterial({ uniforms: u, vertexShader: 'varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}', fragmentShader: POST }));
  pass.enabled = false;
  const projected = new THREE.Vector3(), warmShaft = new THREE.Color(0xffc488), coolShaft = new THREE.Color(0xbcd8ff);
  let cinematic = false;
  return {
    pass,
    setCinematic(value) { cinematic = !!value; },
    // Light shafts follow the sun/moon only while it is in front of the camera and near the frame.
    update(dt, camera, sunDir, { day, dusk, cloud, animated }) {
      u.aspect.value = camera.aspect;
      const target = cinematic ? 1 : 0;
      u.letterbox.value = animated ? u.letterbox.value + (target - u.letterbox.value) * Math.min(1, dt * 2.2) : target;
      projected.copy(sunDir).multiplyScalar(1000).add(camera.position).project(camera);
      const x = (projected.x + 1) / 2, y = (projected.y + 1) / 2;
      const inFrame = projected.z < 1 ? Math.max(0, 1 - Math.max(0, Math.abs(x - .5) - .5, Math.abs(y - .5) - .5) * 4) : 0;
      u.lightPos.value.set(x, y);
      u.shafts.value = inFrame * (.06 + dusk * .14 + (1 - day) * .03) * (1 - cloud * .75);
      u.shaftColor.value.copy(coolShaft).lerp(warmShaft, dusk);
      u.shadowTint.value.setRGB(0, .018 + .01 * (1 - day), .024 + .01 * (1 - day));
      u.highlightTint.value.setRGB(.02 + dusk * .03, .01 + dusk * .006, 0);
    },
    stats: () => ({ enabled: pass.enabled, letterbox: +u.letterbox.value.toFixed(2), shafts: +u.shafts.value.toFixed(2) }),
    dispose() { pass.material.dispose(); pass.dispose?.(); },
  };
}
