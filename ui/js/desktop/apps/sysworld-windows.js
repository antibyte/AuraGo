import * as THREE from 'three';

// Interior-mapped window panes for the kit v3 buildings. Decorative, never telemetry.
// Every `city.window` quad carries room coordinates in TEXCOORD_0: u is one unit per bay, left
// to right seen from outside, +16 per facade; v is one unit per storey, 0 at the first upper
// floor, -1 for the ground-floor lobby, +20 per tier. glTF stores V flipped, so rows are 1 - v.
// One shared MeshStandardMaterial (glossy glass that still reflects the environment) adds, per
// room cell, a ray-cast box room behind the glass as emissive light: walls, floor, a ceiling
// light, a furniture layer, and on high tiers robots and screens. Lit rooms follow the time of
// day and switch slowly. The tangent frame and the room size come from screen-space derivatives
// (the kit has no tangents); every normalisation and division is guarded, because one NaN pixel
// would spread through the bloom chain.
export const WINDOW_MATERIAL = 'city.window';
const LEVELS = {low: 0, medium: 1, high: 2, ultra: 2};
export const windowLevel = tier => LEVELS[tier] ?? 2;
const clamp01 = v => (v > 0 ? (v < 1 ? v : 1) : 0);
const smoothstep = (x, a, b) => { const t = clamp01((x - a) / (b - a)); return t * t * (3 - 2 * t); };

// Same inputs and dusk curve as the pane lighting in sysworld-surfaces.js (day amount and evening
// from sysworld-exploration.js daylight): about 12 % of rooms lit by day, 60 % in the evening and
// 50 % at night; overcast days light a few more. Lobbies are lit from dusk to dawn.
export function windowLighting(day, evening, overcast = 0) {
  const d = clamp01(day), dusk = smoothstep(1 - d, .1, .9);
  return {
    share: Math.min(.9, .12 + .38 * dusk + .1 * clamp01(evening) * dusk + .08 * clamp01(overcast)),
    ambient: .012 + .1 * d,
    lobby: dusk,
    glow: .6 + .9 * dusk,
  };
}

export const WINDOW_SHADER = {
  vertexPars: 'varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;',
  vertexMain: `
#ifdef USE_INSTANCING
mat4 winObject=modelMatrix*instanceMatrix;
#else
mat4 winObject=modelMatrix;
#endif
vWinPos=(winObject*vec4(transformed,1.0)).xyz;
vWinNrm=mat3(winObject)*objectNormal;
// glTF stores V flipped: rows count upwards from the first upper floor again.
vWinRoom=vec2(uv.x,1.0-uv.y);
// One seed per placed building from its world translation, so instanced towers differ.
vec3 winKey=fract(floor(winObject[3].xyz*4.0)*0.1031);
winKey+=dot(winKey,winKey.zyx+31.32);
vWinSeed=fract((winKey.x+winKey.y)*winKey.z);`,
  fragmentPars: `uniform float winTime;uniform vec4 winState;uniform vec3 winSky;
varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;
float winHash(vec2 p){vec3 q=fract(vec3(p.xyx)*0.1031);q+=dot(q,q.yzx+33.33);return fract((q.x+q.y)*q.z);}
float winBox(vec2 p,vec2 lo,vec2 hi){vec2 s=step(lo,p)*step(p,hi);return s.x*s.y;}
// Robot silhouette in metres around its feet: two legs, a torso and a round head.
float winRobot(vec2 r){return min(1.0,winBox(vec2(abs(r.x),r.y),vec2(0.05,0.0),vec2(0.13,0.78))+winBox(r,vec2(-0.2,0.78),vec2(0.2,1.36))+step(length(r-vec2(0.0,1.52)),0.13));}`,
  fragmentMain: `
// Room cell, per-room hashes and the light state (rooms re-roll their light every 20-60 s).
vec2 winCell=floor(vWinRoom),winF=vWinRoom-winCell;
vec2 winAa=max(fwidth(vWinRoom),vec2(1e-4));
vec3 winN=vWinNrm/max(length(vWinNrm),1e-6);
vec3 winV=vWinPos-cameraPosition;winV/=max(length(winV),1e-4);
float winFres=pow(clamp(1.0-abs(dot(winV,winN)),0.0,1.0),5.0);
vec2 winKey=winCell+vWinSeed*vec2(71.3,113.9);
float winH0=winHash(winKey),winH1=winHash(winKey*1.31+3.7),winH2=winHash(winKey*0.71+9.1);
float winLobby=step(winCell.y,-0.5);
#if WIN_LEVEL>0
float winEpoch=floor(winTime/(20.0+40.0*winH0)+winH0);
#else
float winEpoch=0.0;
#endif
float winOn=max(step(winHash(winKey+winEpoch*vec2(3.1,1.7)+vec2(5.3,2.9)),winState.x),winLobby*step(0.5,winState.z));
vec3 winWall=mix(vec3(0.62,0.56,0.48),vec3(0.40,0.48,0.55),step(0.55,winH1));
vec3 winLamp=mix(vec3(1.0,0.8,0.56),vec3(0.8,0.9,1.0),step(0.72,winH1))*mix(vec3(1.0),vec3(1.12,0.95,0.78),winLobby);
vec3 winLit=mix(vec3(winState.y),winLamp*winState.w,winOn);
vec2 winS=vec2(WIN_BAY,WIN_STOREY);
vec3 winRoomCol;
#if WIN_LEVEL>0
// Tangent frame and metres per room from screen-space derivatives; an analytic frame (horizontal
// tangent, rows upwards, default room) replaces them when they are degenerate or implausible.
vec3 winDpx=dFdx(vWinPos),winDpy=dFdy(vWinPos);
vec2 winDux=dFdx(vWinRoom),winDuy=dFdy(vWinRoom);
float winDet=winDux.x*winDuy.y-winDux.y*winDuy.x;
float winOk=step(1e-12,abs(winDet));
float winDiv=mix(1.0,winDet,winOk);
vec3 winPu=(winDuy.y*winDpx-winDux.y*winDpy)/winDiv,winPv=(winDux.x*winDpy-winDuy.x*winDpx)/winDiv;
float winLu=length(winPu),winLv=length(winPv);
winOk*=step(0.25,winLu)*step(winLu,40.0)*step(0.25,winLv)*step(winLv,40.0);
vec3 winC=cross(vec3(0.0,1.0,0.0),winN);
vec3 winT=mix(winC/max(length(winC),1e-4),winPu/max(winLu,1e-4),winOk);
vec3 winB=mix(vec3(0.0,1.0,0.0),winPv/max(winLv,1e-4),winOk);
winS=mix(winS,vec2(winLu,winLv),winOk);
// View ray into a box room one cell wide, one storey high and WIN_DEPTH deep.
vec3 winD=vec3(dot(winV,winT),dot(winV,winB),max(-dot(winV,winN),0.02));
vec3 winSize=vec3(winS,WIN_DEPTH),winP=vec3(winF*winS,0.0);
vec3 winT3=abs(step(0.0,winD)*winSize-winP)/max(abs(winD),vec3(1e-4));
float winHit=min(min(winT3.x,winT3.y),winT3.z);
vec3 winQ=clamp((winP+winD*winHit)/winSize,0.0,1.0);
float winBack=step(winT3.z,min(winT3.x,winT3.y));
float winSide=(1.0-winBack)*step(winT3.x,winT3.y);
float winFlr=(1.0-winBack-winSide)*step(winD.y,0.0);
float winCeil=1.0-winBack-winSide-winFlr;
vec3 winAlb=winWall*(winBack+0.72*winSide+0.9*winCeil)+vec3(0.16,0.15,0.17)*winFlr;
float winPanel=winCeil*winBox(winQ.xz,vec2(0.28,0.3),vec2(0.72,0.62));
winRoomCol=winAlb*winLit*mix(0.9,0.6,winQ.z)+winPanel*winLamp*winOn*2.5*winState.w;
// Furniture at 55 % depth: a desk or counter, sometimes a tall shelf.
float winTf=0.55*WIN_DEPTH/winD.z;
vec2 winPf=(winP+winD*winTf).xy;
float winX0=(0.12+0.3*winH1)*winS.x,winX1=winX0+(0.3+0.2*fract(winH1*7.0))*winS.x;
float winFurn=min(1.0,winBox(winPf,vec2(winX0,0.0),vec2(winX1,0.76))+step(0.5,fract(winH1*13.0))*winBox(winPf,vec2(winS.x-0.42,0.0),vec2(winS.x-0.1,1.9)))*step(winTf,winHit);
winRoomCol=mix(winRoomCol,winWall*0.16*winLit,winFurn);
#if WIN_LEVEL>1
// Life at 30 % depth in a share of the lit rooms: a robot walking across or standing and bobbing
// (two in lobbies), and a flickering screen on the back wall of offices.
float winTl=0.3*WIN_DEPTH/winD.z;
vec2 winPl=(winP+winD*winTl).xy;
float winWalk=step(0.5,fract(winH2*5.3));
float winX=mix(0.2+0.6*fract(winH2*29.0),0.5+0.36*sin(winTime*(0.25+0.25*fract(winH2*17.0))+winH2*40.0),winWalk)*winS.x;
float winBob=0.025*sin(winTime*2.6+winH2*30.0)*(1.0-winWalk);
vec2 winR1=vec2(winPl.x-winX,winPl.y-winBob),winR2=vec2(winPl.x-winS.x+winX,winPl.y+winBob);
float winBots=max(winRobot(winR1),winRobot(winR2)*winLobby)*winOn*step(winH2,mix(0.4,0.85,winLobby))*step(winTl,winHit);
float winVisor=max(winBox(winR1,vec2(-0.09,1.49),vec2(0.09,1.55)),winBox(winR2,vec2(-0.09,1.49),vec2(0.09,1.55))*winLobby)*winBots;
float winScreen=winOn*winBack*(1.0-winLobby)*(1.0-winFurn)*step(fract(winH2*3.1),0.35)*winBox(vec2((winQ.x-0.5)*winS.x,winQ.y*winS.y),vec2(-0.5,1.0),vec2(0.5,1.6));
winRoomCol+=winScreen*vec3(0.3,0.7,1.0)*1.8*(0.8+0.2*sin(winTime*9.0+winH2*50.0)*sin(winTime*2.3+winH2*20.0));
winRoomCol=mix(winRoomCol,vec3(0.02,0.025,0.03),winBots)+winVisor*vec3(0.2,0.9,1.0)*1.5;
#endif
// Blinds in about one room in five, drawn on the glass; slats fade out when they get sub-pixel.
float winBlind=step(fract(winH1*23.0),0.2)*step(0.7-0.6*fract(winH2*41.0),winF.y);
float winSlat=0.8+0.2*step(0.5,fract(winF.y*winS.y*12.0))*(1.0-smoothstep(0.2,0.6,winAa.y*winS.y*12.0));
winRoomCol=mix(winRoomCol,vec3(0.55,0.5,0.42)*winSlat*(winState.y*1.6+winLamp*winOn*0.45*winState.w),winBlind);
#else
// Low tier: a flat lit or unlit cell, brighter under the ceiling; no parallax, no motion.
winRoomCol=winWall*winLit*0.8+winLamp*winOn*winState.w*0.5*smoothstep(0.35,1.0,winF.y);
#endif
winRoomCol*=1.0-winFres;
// A faint frame (mullions on the cell's vertical edges, a spandrel at its foot) so the lowest LOD,
// which has no fins or slabs, still reads as a facade.
vec2 winM=vec2(0.06,0.3)/winS;
float winFrame=max(1.0-smoothstep(winM.x-winAa.x,winM.x+winAa.x,min(winF.x,1.0-winF.x)),1.0-smoothstep(winM.y-winAa.y,winM.y+winAa.y,winF.y));
vec3 winGlow=(max(winRoomCol,vec3(0.0))+winSky*(0.04+0.6*winFres))*(1.0-winFrame);
diffuseColor.rgb=mix(diffuseColor.rgb,vec3(0.035,0.042,0.05),winFrame);`,
  roughness: '\nroughnessFactor=mix(roughnessFactor,0.55,winFrame);',
  emissive: '\ntotalEmissiveRadiance+=winGlow;',
};

export function createWindows() {
  const uniforms = {
    winTime: {value: 0},
    winState: {value: new THREE.Vector4(.12, .112, 0, .6)},   // lit share, unlit ambient, lobby, lit glow
    winSky: {value: new THREE.Color(0x08121e)},
  };
  let level = 2, tier = 'high', reduced = false, meshes = 0, disposed = false;
  const material = new THREE.MeshStandardMaterial({
    name: WINDOW_MATERIAL, color: new THREE.Color(.018, .032, .045), roughness: .08, metalness: 0, vertexColors: true,
  });
  material.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, uniforms);
    const head = `#define WIN_LEVEL ${level}\n#define WIN_DEPTH 3.0\n#define WIN_BAY 1.9\n#define WIN_STOREY 3.05\n`;
    shader.vertexShader = head + shader.vertexShader
      .replace('#include <common>', '#include <common>\n' + WINDOW_SHADER.vertexPars)
      .replace('#include <begin_vertex>', '#include <begin_vertex>' + WINDOW_SHADER.vertexMain);
    shader.fragmentShader = head + shader.fragmentShader
      .replace('#include <common>', '#include <common>\n' + WINDOW_SHADER.fragmentPars)
      .replace('#include <color_fragment>', '#include <color_fragment>' + WINDOW_SHADER.fragmentMain)
      .replace('#include <roughnessmap_fragment>', '#include <roughnessmap_fragment>' + WINDOW_SHADER.roughness)
      .replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>' + WINDOW_SHADER.emissive);
  };
  material.customProgramCacheKey = () => 'sw-windows|' + level;
  return {
    material, uniforms,
    owns: m => m === material,
    // Swaps every city.window slot of a mesh for the shared material and disposes the loaded one.
    adopt(mesh) {
      if (!mesh?.isMesh || disposed) return false;
      const list = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
      let hit = false;
      const next = list.map(m => {
        if (m === material || m?.name !== WINDOW_MATERIAL) return m;
        hit = true; m.dispose(); return material;
      });
      if (!hit) return false;
      mesh.material = Array.isArray(mesh.material) ? next : next[0];
      meshes++;
      return true;
    },
    setLighting(day, evening, overcast = 0, sky = null) {
      const l = windowLighting(day, evening, overcast);
      uniforms.winState.value.set(l.share, l.ambient, l.lobby, l.glow);
      if (sky) uniforms.winSky.value.copy(sky);
    },
    setTier(value) {
      tier = value;
      const next = windowLevel(value);
      if (next !== level) { level = next; material.needsUpdate = true; }
    },
    setReducedMotion(value) { reduced = !!value; },
    // Time drives room switching and the life layer; it stands still without motion.
    update(dt, animated) {
      if (!animated || reduced || disposed) return;
      uniforms.winTime.value = (uniforms.winTime.value + Math.min(.1, Math.max(0, dt || 0))) % 7200;
    },
    stats: () => ({
      level, tier, reduced, meshes,
      share: +uniforms.winState.value.x.toFixed(2), lobby: +uniforms.winState.value.z.toFixed(2),
      glow: +uniforms.winState.value.w.toFixed(2), time: +uniforms.winTime.value.toFixed(1),
    }),
    dispose() { disposed = true; material.dispose(); },
  };
}
