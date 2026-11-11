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

// Room wall tones (phase 6): linear albedo and the cumulative share of rooms (the last entry ends at 1). Warm white,
// cool grey-blue, sage, terracotta, teal and charcoal, so large facades read as rooms, not as rows of pale boxes.
export const WINDOW_TONES = Object.freeze([
  Object.freeze({ upTo: .28, color: Object.freeze([.62, .56, .48]) }),
  Object.freeze({ upTo: .5, color: Object.freeze([.40, .48, .55]) }),
  Object.freeze({ upTo: .64, color: Object.freeze([.42, .50, .40]) }),
  Object.freeze({ upTo: .76, color: Object.freeze([.55, .38, .30]) }),
  Object.freeze({ upTo: .88, color: Object.freeze([.22, .36, .38]) }),
  Object.freeze({ upTo: 1, color: Object.freeze([.26, .25, .27]) }),
]);
const glslFloat = v => (Number.isInteger(v) ? v.toFixed(1) : String(v));
const TONE_GLSL = 'vec3 winTone(float h){return ' + WINDOW_TONES.slice(0, -1).map(t => 'h<' + glslFloat(t.upTo) + '?vec3(' + t.color.map(glslFloat).join(',') + '):').join('')
  + 'vec3(' + WINDOW_TONES.at(-1).color.map(glslFloat).join(',') + ');}';

// Layout of the resident sprite atlas (ui/3d/system-world/textures/v1/window-life.png), kept in sync with window-life.json by
// scripts/test-system-world-windows.mjs. Cell i sits in column i % columns of row floor(i / columns), row 0 is the top row of
// the image; a cell is metres[0] x metres[1] m, the figure's feet centre sits at its horizontal middle, feet m above its lower edge.
// Channels are data (R sqrt lamp luminance, G sqrt daylight luminance, B accent mask, A coverage); stride is metres per gait cycle.
export const WINDOW_LIFE_ATLAS = Object.freeze({
  columns: 6, rows: 4, pixels: Object.freeze([288, 432]), metres: Object.freeze([1.6, 2.4]), feet: .06, stride: 1.4348,
  sets: Object.freeze({
    walk: Object.freeze({first: 0, count: 8, kind: 'technician'}),
    carry: Object.freeze({first: 8, count: 8, kind: 'courier'}),
    idleWalk: Object.freeze({first: 16, count: 1, kind: 'technician'}),
    idleCarry: Object.freeze({first: 17, count: 1, kind: 'courier'}),
    idle: Object.freeze({first: 18, count: 1, kind: 'archivist'}),
    greet: Object.freeze({first: 19, count: 3, kind: 'archivist'}),
    work: Object.freeze({first: 22, count: 2, kind: 'technician'}),
  }),
});
// Relighting of the sprites (baked into the shader as constants): lamp and day scale the two light channels, glowDay and glowLit
// the glowing accent parts (eyes, strips) by day and in lit rooms, shadow the strength of the contact shadow on the floor.
export const WINDOW_LIFE_LOOK = Object.freeze({ lamp: 1, day: 1.8, glowDay: .35, glowLit: .8, shadow: .5 });

const LF = WINDOW_LIFE_ATLAS, LK = WINDOW_LIFE_LOOK, LFS = LF.sets;
const LIFE_GLSL = `
#if WIN_LEVEL>1
uniform sampler2D winLife;uniform float winLifeOn;
const float WIN_LK_LAMP=${glslFloat(LK.lamp)};const float WIN_LK_DAY=${glslFloat(LK.day)};const float WIN_LK_GD=${glslFloat(LK.glowDay)};const float WIN_LK_GL=${glslFloat(LK.glowLit)};const float WIN_LK_SH=${glslFloat(LK.shadow)};
// Atlas layout; reciprocals are baked so the shader never divides.
const float WIN_LF_COLS=${glslFloat(LF.columns)};const float WIN_LF_ICOLS=${glslFloat(1 / LF.columns)};const float WIN_LF_IROWS=${glslFloat(1 / LF.rows)};
const float WIN_LF_IW=${glslFloat(1 / LF.metres[0])};const float WIN_LF_IH=${glslFloat(1 / LF.metres[1])};const float WIN_LF_FEET=${glslFloat(LF.feet)};
const vec2 WIN_LF_HALF=vec2(${glslFloat(.5 / LF.pixels[0])},${glslFloat(.5 / LF.pixels[1])});
const float WIN_LF_ISTRIDE=${glslFloat(1 / LF.stride)};const float WIN_LF_SPEED=0.55;const float WIN_LF_ISPEED=${glslFloat(1 / .55)};
const float WIN_LF_WALK=${LFS.walk.first}.0;const float WIN_LF_CARRY=${LFS.carry.first}.0;const float WIN_LF_IDLEW=${LFS.idleWalk.first}.0;const float WIN_LF_IDLEC=${LFS.idleCarry.first}.0;
const float WIN_LF_IDLE=${LFS.idle.first}.0;const float WIN_LF_GREET=${LFS.greet.first}.0;const float WIN_LF_WORK=${LFS.work.first}.0;
vec3 winAccent(float h){return h<0.25?vec3(0.2,0.9,1.0):h<0.5?vec3(0.25,0.9,0.5):h<0.75?vec3(1.0,0.62,0.2):vec3(1.0,0.85,0.6);}
// One atlas cell: r = metres around the feet, g = screen derivatives of r. Zero outside the cell, clamped half a texel inside it;
// explicit gradients, so cell jumps and mirrored cards cause no mip seams.
vec4 winSprite(float cell,vec2 r,vec2 gx,vec2 gy){
  float row=floor((cell+0.5)*WIN_LF_ICOLS);
  float col=cell-row*WIN_LF_COLS;
  vec2 q=vec2(r.x*WIN_LF_IW+0.5,(r.y+WIN_LF_FEET)*WIN_LF_IH);
  float inside=step(0.0,q.x)*step(q.x,1.0)*step(0.0,q.y)*step(q.y,1.0);
  vec2 qc=clamp(q,WIN_LF_HALF,vec2(1.0)-WIN_LF_HALF);
  vec2 uv=(vec2(col,row)+vec2(qc.x,1.0-qc.y))*vec2(WIN_LF_ICOLS,WIN_LF_IROWS);
  vec2 sc=vec2(WIN_LF_IW*WIN_LF_ICOLS,-WIN_LF_IH*WIN_LF_IROWS);
  vec4 t=textureGrad(winLife,uv,gx*sc,gy*sc);
  return vec4(t.rgb,t.a*inside);
}
// Pose of one resident on its timeline: x metres from the left wall, cell A, cell B, blend of B over A. role 0 stroller, 1 courier,
// 2 greeter, 3 worker. flip is -1 for a card mirrored (walking towards -x), bobK scales the standing bob. Nothing depends on time
// advancing: a frozen stroller is caught mid-timeline, a frozen greeter is never in the middle of a wave. A range under 0.3 m means standing still.
vec4 winPose(float role,float h,float tm,float lo,float hi,float W,float deskX,out float flip,out float bobK){
  flip=mix(1.0,-1.0,step(0.5,fract(h*17.9)));bobK=1.0;
  if(role<1.5){
    float can=step(0.3,hi-lo),hiE=lo+(hi-lo)*can,len=max(hiE-lo,0.3),tw=len*WIN_LF_ISPEED;
    float ts=3.0+5.0*fract(h*13.7),per=2.0*(ts+tw);
    // The phase is a uniform draw over the whole cycle, so neighbouring rooms start, turn and arrive at unrelated times.
    float u=mod(tm+per*fract(h*3.3),per);
    float leg=step(ts+tw,u)*can;
    float tseg=u-leg*(ts+tw)-ts;
    float dist=clamp(tseg,0.0,tw)*WIN_LF_SPEED*can;
    float walking=can*step(0.0,tseg)*smoothstep(0.0,0.25,tseg)*(1.0-smoothstep(tw-0.25,tw,tseg));
    float frame=clamp(floor(fract(dist*WIN_LF_ISTRIDE)*8.0),0.0,7.0);
    flip=mix(1.0,-1.0,leg);bobK=1.0-walking;
    return vec4(mix(lo+dist,hiE-dist,leg),mix(WIN_LF_IDLEW,WIN_LF_IDLEC,role),mix(WIN_LF_WALK,WIN_LF_CARRY,role)+frame,walking);
  }
  if(role<2.5){
    float per=14.0+16.0*fract(h*3.9);
    float u=mod(tm+2.6+(per-2.6)*fract(h*5.7),per);
    float w=smoothstep(0.0,0.2,u)*(1.0-smoothstep(2.2,2.4,u))*step(u,2.4);
    float idx=mod(floor(u*6.0),4.0);
    return vec4(lo+(hi-lo)*fract(h*29.0),WIN_LF_IDLE,WIN_LF_GREET+min(idx,4.0-idx),w);
  }
  bobK=0.0;
  return vec4(clamp(deskX,lo,hi),WIN_LF_WORK,WIN_LF_WORK+1.0,0.5+0.5*sin(tm*(8.0+2.4*fract(h*5.1))+h*50.0));
}
// A resident relit for this room: rgb = colour, a = coverage.
vec4 winFigure(vec4 p,float flip,float bob,vec2 pl,vec2 gx,vec2 gy,vec3 accent,vec3 lampL,vec3 fill,float on){
  vec2 r=vec2((pl.x-p.x)*flip,pl.y-bob),fx=vec2(gx.x*flip,gx.y),fy=vec2(gy.x*flip,gy.y);
  vec4 t=winSprite(p.y,r,fx,fy);
  if(p.w>0.001)t=mix(t,winSprite(p.z,r,fx,fy),p.w);
  float lamp=t.r*t.r,day=t.g*t.g,am=min(2.0*t.b,1.0),gl=max(2.0*t.b-1.0,0.0);
  vec3 c=mix(vec3(1.0),accent,am)*(lamp*lampL*(1.0-0.3*0.3)*WIN_LK_LAMP+day*fill*(0.12+0.88*0.49)*WIN_LK_DAY)+accent*gl*(WIN_LK_GD+WIN_LK_GL*on);
  return vec4(c,t.a);
}
#endif`;

export const WINDOW_SHADER = {
  vertexPars: 'attribute vec3 winAxU;attribute vec3 winAxV;varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;varying vec3 vWinAxU;varying vec3 vWinAxV;',
  vertexMain: `
#ifdef USE_INSTANCING
mat4 winObject=modelMatrix*instanceMatrix;
#else
mat4 winObject=modelMatrix;
#endif
vWinPos=(winObject*vec4(transformed,1.0)).xyz;
vWinNrm=mat3(winObject)*objectNormal;
vWinAxU=mat3(winObject)*winAxU;vWinAxV=mat3(winObject)*winAxV;
// glTF stores V flipped: rows count upwards from the first upper floor again.
vWinRoom=vec2(uv.x,1.0-uv.y);
// One seed per placed building from its world translation, so instanced towers differ.
vec3 winKey=fract(floor(winObject[3].xyz*4.0)*0.1031);
winKey+=dot(winKey,winKey.zyx+31.32);
vWinSeed=fract((winKey.x+winKey.y)*winKey.z);`,
  fragmentPars: `uniform float winTime;uniform vec4 winState;uniform vec3 winSky;
varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;varying vec3 vWinAxU;varying vec3 vWinAxV;
float winHash(vec2 p){vec3 q=fract(vec3(p.xyx)*0.1031);q+=dot(q,q.yzx+33.33);return fract((q.x+q.y)*q.z);}
// Box coverage with a smooth edge about one pixel wide (aa = pixel footprint in the same units).
float winBox(vec2 p,vec2 lo,vec2 hi,vec2 aa){vec2 s=smoothstep(lo-aa,lo+aa,p)*(1.0-smoothstep(hi-aa,hi+aa,p));return s.x*s.y;}
// Light on a room surface: lamp light falls off a little with depth, daylight a lot (to 12 % at the back).
vec3 winLightAt(vec3 lamp,vec3 fill,float z){return min(lamp*(1.0-0.3*z)+fill*(0.12+0.88*(1.0-z)*(1.0-z)),vec3(1.2));}
${TONE_GLSL}${LIFE_GLSL}`,
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
float winH3=winHash(winKey*1.73+5.9);
vec3 winWall=winTone(winH3);
vec3 winLamp=mix(vec3(1.0,0.8,0.56),vec3(0.8,0.9,1.0),step(0.72,winH1))*mix(vec3(1.0),vec3(1.0,0.92,0.78),winLobby);
// Light budget (linear radiance before tone mapping): lit walls stay below about 0.85, only the
// ceiling panel (1.2), screens (1.15) and the glowing accent parts of the figures pass the bloom threshold. Daylight (read back
// from the unlit ambient) fills every room, brightest at the glass and a third of it at the back.
float winDusk=clamp((winState.w-0.6)*1.1111,0.0,1.0);
float winDay=clamp((winState.y-0.012)*10.0,0.0,1.0);
vec3 winLampL=winLamp*(mix(0.08,0.7,winDusk)*winOn);
vec3 winFill=vec3(0.94,1.0,1.06)*(0.012+0.2*winDay);
float winAlive=max(winOn,winDay*step(fract(winH0*7.7),0.45));
vec2 winS=vec2(WIN_BAY,WIN_STOREY);
vec3 winRoomCol;
#if WIN_LEVEL>0
// Tangent frame and metres per room from exact per-pane data: adopt() stores, per vertex, the object-space vector of one room
// unit along u and along v (from positions and uvs); the vertex shader carries them to world space, so panes of any width and
// any object scale are right at every distance, with no screen-space derivative. A degenerate vector falls back to the analytic
// frame (horizontal tangent, rows upwards) and the default room.
vec3 winPu=vWinAxU,winPv=vWinAxV;
float winLu=length(winPu),winLv=length(winPv);
float winOk=step(0.25,winLu)*step(winLu,40.0)*step(0.25,winLv)*step(winLv,40.0);
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
vec3 winAlb=winWall*(winBack+0.72*winSide+0.9*winCeil)+vec3(0.22,0.2,0.21)*winFlr;
vec2 winAq=min(max(fwidth(winQ.xz),vec2(1e-3)),vec2(0.2));
float winPanel=winCeil*winBox(winQ.xz,vec2(0.28,0.3),vec2(0.72,0.62),winAq*0.6);
winRoomCol=winAlb*winLightAt(winLampL,winFill,winQ.z)+winPanel*winLamp*winOn*mix(0.5,1.2,winDusk);
float winAh=max(fwidth(winHit),1e-3);
// Furniture at 55 % depth: a desk or counter, sometimes a tall shelf.
float winTf=0.55*WIN_DEPTH/winD.z;
vec2 winPf=(winP+winD*winTf).xy;
float winX0=(0.12+0.3*winH1)*winS.x,winX1=winX0+(0.3+0.2*fract(winH1*7.0))*winS.x;
vec2 winAf=min(max(fwidth(winPf),vec2(1e-3)),vec2(0.2))*0.6;
float winFurn=min(1.0,winBox(winPf,vec2(winX0,0.0),vec2(winX1,0.76),winAf)+step(0.5,fract(winH1*13.0))*winBox(winPf,vec2(winS.x-0.42,0.0),vec2(winS.x-0.1,1.9),winAf))*smoothstep(winTf-winAh,winTf+winAh,winHit);
winRoomCol=mix(winRoomCol,vec3(0.07,0.065,0.06)*(0.8+0.4*fract(winH1*5.0))*winLightAt(winLampL,winFill,0.55),winFurn);
#if WIN_LEVEL>1
// Life at 30 % depth in a share of the lit rooms: sprites of the resident robots (a stroller, a courier, a greeter or a worker at
// the desk; two in lobbies), drawn as a flat card from the atlas and relit per room, plus a flickering screen on the back wall of
// offices. Without the atlas on the GPU (winLifeOn 0) no figure is drawn.
float winTl=0.3*WIN_DEPTH/winD.z;
vec2 winPl=(winP+winD*winTl).xy;
// Continuous card coordinate (no fract at the cell edge) for the sprite gradients.
vec2 winPc=vWinRoom*winS+winD.xy*winTl;
vec2 winGx=dFdx(winPc),winGy=dFdy(winPc);
float winPres=winAlive*step(winH2,mix(0.4,0.85,winLobby))*winLifeOn;
vec2 winSp=vec2((winQ.x-0.5)*winS.x,winQ.y*winS.y);
float winScreen=winOn*winBack*(1.0-winLobby)*(1.0-winFurn)*step(fract(winH2*3.1),0.35)*winBox(winSp,vec2(-0.5,1.0),vec2(0.5,1.6),min(max(fwidth(winSp),vec2(1e-3)),vec2(0.2))*0.6);
winRoomCol+=winScreen*vec3(0.3,0.7,1.0)*1.15*(0.8+0.2*sin(winTime*9.0+winH2*50.0)*sin(winTime*2.3+winH2*20.0));
// Figures stand in front of the back-wall screen.
if(winPres>0.001){
  // winHash collides for some neighbouring cells in 32-bit floats (seen as two rooms walking in step), so the
  // cell index is added in golden-ratio steps: neighbours always differ, whatever the hash returns.
  float winH4=fract(winHash(winKey*1.91+7.7)+0.6180339*winCell.x+0.7548777*winCell.y),winHr=winHash(winKey*2.17+1.3);
  // Roles follow the cell, so horizontal, vertical and diagonal neighbours never share one (lobbies: three roles, no worker).
  float winOff=floor(vWinSeed*4.0);
  float winRole1=mix(mod(winCell.x+2.0*winCell.y+winOff,4.0),mod(winCell.x+winOff,3.0),winLobby),winRole2=mod(winRole1+1.0,3.0);
  // Lobby pair: needs 1.15 m between the figure centres and 0.45 m to each side wall (2.05 m of true room width, exact pane data, so
  // it cannot flip while the camera moves); figure 1 keeps to the left half, figure 2 (mirrored) to the right. Narrower lobbies show one.
  float winTwo=winLobby*step(2.05,winS.x);
  float winLo=0.45,winHi=mix(winS.x-0.45,max(0.5*winS.x-0.575,winLo),winTwo);
  float winFl1,winBk1,winFl2,winBk2;
  vec4 winPo1=winPose(winRole1,winH4,winTime,winLo,winHi,winS.x,0.5*(winX0+winX1),winFl1,winBk1);
  vec4 winPo2=winPose(winRole2,fract(winH4*3.7+0.41),winTime,winLo,winHi,winS.x,0.5*(winX0+winX1),winFl2,winBk2);
  winPo2.x=winS.x-winPo2.x;winFl2=-winFl2;
  float winBob1=0.025*sin(winTime*2.6+winH2*30.0)*winBk1,winBob2=0.025*sin(winTime*2.6+winH2*30.0+2.1)*winBk2;
  // Contact shadow: a soft ellipse on the floor around each figure's feet.
  vec2 winFq=vec2(winQ.x*winS.x,winQ.z*WIN_DEPTH);
  float winFlrK=winFlr*(1.0-winFurn)*WIN_LK_SH*winPres;
  vec2 winSd1=(winFq-vec2(winPo1.x,0.3*WIN_DEPTH))*vec2(2.381,3.333),winSd2=(winFq-vec2(winPo2.x,0.3*WIN_DEPTH))*vec2(2.381,3.333);
  winRoomCol*=1.0-winFlrK*max(1.0-smoothstep(0.0,1.0,length(winSd1)),(1.0-smoothstep(0.0,1.0,length(winSd2)))*winTwo);
  float winCard=smoothstep(winTl-winAh,winTl+winAh,winHit);
  vec4 winF1=winFigure(winPo1,winFl1,winBob1,winPl,winGx,winGy,winAccent(fract(winH4*5.3+0.1)),winLampL,winFill,winOn);
  winRoomCol=mix(winRoomCol,winF1.rgb,winF1.a*winPres*winCard);
  if(winTwo>0.5){
    vec4 winF2=winFigure(winPo2,winFl2,winBob2,winPl,winGx,winGy,winAccent(fract(winH4*9.1+0.6)),winLampL,winFill,winOn);
    winRoomCol=mix(winRoomCol,winF2.rgb,winF2.a*winPres*winCard);
  }
}
#endif
// Blinds in about one room in five, drawn on the glass; slats fade out when they get sub-pixel.
float winBlindTop=0.7-0.6*fract(winH2*41.0);
float winBlind=step(fract(winH1*23.0),0.2)*smoothstep(winBlindTop-winAa.y,winBlindTop+winAa.y,winF.y);
// Slats: anti-aliased triangle wave; they fade to their average colour below about two pixels.
float winSlatPx=winAa.y*WIN_STOREY*12.0;
float winSlatW=smoothstep(0.5-min(winSlatPx,0.5),0.5+min(winSlatPx,0.5),abs(fract(winF.y*WIN_STOREY*12.0)-0.5)*2.0);
float winSlat=mix(0.8+0.2*winSlatW,0.9,smoothstep(0.12,0.25,winSlatPx));
winRoomCol=mix(winRoomCol,vec3(0.55,0.5,0.42)*winSlat*winLightAt(winLampL,winFill,0.0)*0.8,winBlind);
#else
// Low tier: a flat lit or unlit cell, brighter under the ceiling; no parallax, no motion.
winRoomCol=winWall*winLightAt(winLampL,winFill,0.4)*0.8+winLamp*winOn*mix(0.1,0.2,winDusk)*smoothstep(0.35,1.0,winF.y);
#endif
winRoomCol=min(winRoomCol,vec3(WIN_CAP));
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

// Own environment intensity for the glass (the scene environment texture is handed over with setEnvironment): by day the
// reflection no longer outshines the rooms behind it, at night the glass keeps a faint sheen.
export const WINDOW_ENV = Object.freeze({ day: .12, night: .08 });

// Exact room axes of a pane geometry: for every vertex of a window triangle the object-space vector of one room unit along u
// (uv.x) and along v (rows upwards, 1 - uv.y), solved from the triangle's positions and uvs and stored as the attributes winAxU
// and winAxV (the geometry owner disposes them). Done once per geometry; returns whether anything was added.
export function addPaneAxes(geometry, windowSlot = () => true) {
  const pos = geometry?.getAttribute?.('position'), uv = geometry?.getAttribute?.('uv');
  if (!pos || !uv || geometry.getAttribute('winAxU')) return false;
  const index = geometry.index, n = pos.count, axU = new Float32Array(n * 3), axV = new Float32Array(n * 3);
  const groups = geometry.groups.length ? geometry.groups : [{ start: 0, count: index ? index.count : n, materialIndex: 0 }];
  const vertex = i => (index ? index.getX(i) : i);
  const p = [0, 0, 0].map(() => [0, 0, 0]);
  for (const g of groups) {
    if (!windowSlot(g.materialIndex)) continue;
    for (let i = g.start; i + 2 < g.start + g.count; i += 3) {
      const j = [vertex(i), vertex(i + 1), vertex(i + 2)];
      for (let k = 0; k < 3; k++) { p[k][0] = pos.getX(j[k]); p[k][1] = pos.getY(j[k]); p[k][2] = pos.getZ(j[k]); }
      const du1 = uv.getX(j[1]) - uv.getX(j[0]), dv1 = uv.getY(j[0]) - uv.getY(j[1]);   // v rows count upwards: v = 1 - uv.y
      const du2 = uv.getX(j[2]) - uv.getX(j[0]), dv2 = uv.getY(j[0]) - uv.getY(j[2]);
      const det = du1 * dv2 - du2 * dv1;
      if (!(Math.abs(det) > 1e-12)) continue;
      for (let c = 0; c < 3; c++) {
        const e1 = p[1][c] - p[0][c], e2 = p[2][c] - p[0][c];
        const u = (e1 * dv2 - e2 * dv1) / det, v = (e2 * du1 - e1 * du2) / det;
        for (const k of j) { axU[k * 3 + c] = u; axV[k * 3 + c] = v; }
      }
    }
  }
  geometry.setAttribute('winAxU', new THREE.BufferAttribute(axU, 3));
  geometry.setAttribute('winAxV', new THREE.BufferAttribute(axV, 3));
  return true;
}

export function createWindows() {
  const uniforms = {
    winTime: {value: 0},
    winState: {value: new THREE.Vector4(.12, .112, 0, .6)},   // lit share, unlit ambient, lobby, lit glow
    winSky: {value: new THREE.Color(0x08121e)},
    winLife: {value: null},   // resident sprite atlas; a 1x1 transparent placeholder keeps the program valid until it arrives
    winLifeOn: {value: 0},    // 0 until the atlas is on the GPU, then eased to 1: no atlas, no figures
  };
  const placeholder = new THREE.DataTexture(new Uint8Array(4), 1, 1, THREE.RGBAFormat);
  placeholder.colorSpace = THREE.NoColorSpace; placeholder.needsUpdate = true;
  uniforms.winLife.value = placeholder;
  let life = null, lifeFailures = 0;
  const releaseLife = () => { if (!life) return; const image = life.image; life.dispose(); image?.close?.(); life = null; };
  let level = 2, tier = 'high', reduced = false, meshes = 0, disposed = false;
  const material = new THREE.MeshStandardMaterial({
    name: WINDOW_MATERIAL, color: new THREE.Color(.018, .032, .045), roughness: .08, metalness: 0, vertexColors: true,
  });
  material.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, uniforms);
    const head = `#define WIN_LEVEL ${level}\n#define WIN_DEPTH 3.0\n#define WIN_BAY 1.9\n#define WIN_STOREY 3.05\n#define WIN_CAP 1.1\n`;
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
      addPaneAxes(mesh.geometry, i => list[i]?.name === WINDOW_MATERIAL || list[i] === material);
      mesh.material = Array.isArray(mesh.material) ? next : next[0];
      meshes++;
      return true;
    },
    setLighting(day, evening, overcast = 0, sky = null) {
      const l = windowLighting(day, evening, overcast);
      uniforms.winState.value.set(l.share, l.ambient, l.lobby, l.glow);
      if (sky) uniforms.winSky.value.copy(sky);
      material.envMapIntensity = WINDOW_ENV.night + (WINDOW_ENV.day - WINDOW_ENV.night) * clamp01(day);
    },
    // The same PMREM texture the scene uses; null leaves the glass on the scene environment.
    setEnvironment(texture) {
      const next = texture || null;
      if (material.envMap === next) return;
      material.envMap = next; material.needsUpdate = true;
    },
    setTier(value) {
      tier = value;
      const next = windowLevel(value);
      if (next !== level) { level = next; material.needsUpdate = true; }
    },
    setReducedMotion(value) { reduced = !!value; },
    // Hands over the loaded sprite atlas (NoColorSpace, flipY false, mipmapped); the windows own it from here on and dispose it.
    // A falsy texture means the load failed: the figures stay off and the failure is counted.
    setLife(texture) {
      if (!texture) { lifeFailures++; return false; }
      if (disposed) { const image = texture.image; texture.dispose(); image?.close?.(); return false; }
      if (texture === life) return true;
      releaseLife(); life = texture; uniforms.winLife.value = texture; uniforms.winLifeOn.value = 0;
      return true;
    },
    // Time drives room switching and the life layer; it stands still without motion.
    update(dt, animated) {
      if (life && uniforms.winLifeOn.value < 1 && !disposed) {
        // The figures fade in over about a second; with frozen time or reduced motion they appear at once.
        const step = animated && !reduced && dt > 0 ? Math.min(.1, dt) : 1;
        uniforms.winLifeOn.value = Math.min(1, uniforms.winLifeOn.value + step);
      }
      if (!animated || reduced || disposed) return;
      uniforms.winTime.value = (uniforms.winTime.value + Math.min(.1, Math.max(0, dt || 0))) % 7200;
    },
    stats: () => ({
      level, tier, reduced, meshes, life: life ? 1 : 0, lifeFailures,
      share: +uniforms.winState.value.x.toFixed(2), lobby: +uniforms.winState.value.z.toFixed(2),
      glow: +uniforms.winState.value.w.toFixed(2), time: +uniforms.winTime.value.toFixed(1),
    }),
    dispose() { disposed = true; releaseLife(); uniforms.winLife.value = null; placeholder.dispose(); material.dispose(); },
  };
}
