// Metres, Y up. Roads, plots, transport and walkable surfaces share this layout.
export const streets = {xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80};
export const surfaces = {ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16};
// The waterfront pavilions face north, onto the southern boulevard. Their
// complete footprints (including walls) stay beyond its southern pavement.
export const interiors = ['agent','memory','missions'].map((id,i)=>{
  const x=[0,-43,43][i],z=73;
  return {id,x,z,width:12,depth:12,front:-1,doorZ:z-6,liftX:x+4,liftZ:z+3};
});
export const stations = [
  {id:'infra',x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},
  {id:'memory',x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},
  {id:'graph',x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},
  {id:'operations',x:0,z:59,platformX:0,platformZ:64.5,angle:0},
  {id:'missions',x:43,z:59,platformX:43,platformZ:64.5,angle:0},
  {id:'integrations',x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},
  {id:'agent',x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI},
];
export const tramWaypoints=[[-67,-77],[-67,59],[67,59],[67,-77]];
// Agent tower sky deck: a crown terrace at 72.2 m reached by the open east lift from the
// 0.88 m podium (travel mirrored as SKY_TRAVEL in assets/system-world/build_expansion.py).
// Walkable: the annulus between inner and outer radius plus the bridge while the cab is up.
const deckSpot=(bearing,radius)=>{const b=bearing*Math.PI/180;return {x:Math.sin(b)*radius,z:-12-Math.cos(b)*radius,angle:Math.PI-b};};
export const skyDeck={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,
  lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},
  telescopes:[45,135,225,315].map(b=>({id:'deck-'+b,...deckSpot(b,7.25)})),
  benches:[0,180,270].map(b=>deckSpot(b,6.6))};
export function skyWalkable(x,z,cabUp){
  const r=Math.hypot(x-skyDeck.x,z-skyDeck.z);
  if(r>=skyDeck.inner&&r<=skyDeck.outer)return true;
  return cabUp&&x>=skyDeck.outer-.5&&x<=skyDeck.lift.x+1&&Math.abs(z-skyDeck.lift.z)<skyDeck.bridge;
}
export const dronePad={x:78,z:30};
export const towers=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];
export function streetAt(x,z,halfWidth=streets.halfWidth){
  return x>=streets.minX&&x<=streets.maxX&&z>=streets.minZ&&z<=streets.maxZ&&
    (streets.xs.some(v=>Math.abs(x-v)<=halfWidth)||streets.zs.some(v=>Math.abs(z-v)<=halfWidth));
}
export function streetHeight(x,z){
  if(!streetAt(x,z))return surfaces.ground;
  return streetAt(x,z,streets.laneHalfWidth)?surfaces.road:surfaces.pavement;
}
export function galleryContains(r,x,z,margin=0){
  return x>=r.x-5.85+margin&&x<=r.x+2.4-margin&&z>=r.z+margin&&z<=r.z+5.85-margin;
}
export function liftContains(r,x,z,margin=0){return Math.abs(x-r.liftX)<=1.6-margin&&Math.abs(z-r.liftZ)<=1.6-margin;}
export function roomFloorTiles(){
  const tiles=[],shaft=[2.4,1.4,5.6,4.6];
  const add=(x0,z0,x1,z1)=>{if(x1>x0&&z1>z0)tiles.push({x:(x0+x1)/2,z:(z0+z1)/2,sx:(x1-x0)/4,sz:(z1-z0)/4});};
  for(let x=-4;x<=4;x+=4)for(let z=-4;z<=4;z+=4){
    const [x0,z0,x1,z1]=[x-2,z-2,x+2,z+2];
    const [a,b,c,d]=[Math.max(x0,shaft[0]),Math.max(z0,shaft[1]),Math.min(x1,shaft[2]),Math.min(z1,shaft[3])];
    if(a>=c||b>=d){add(x0,z0,x1,z1);continue;}
    // The parked lift supplies this surface. Leaving a floor underneath its
    // platform would create the same coplanar flicker as the original quay.
    add(x0,z0,a,z1);add(c,z0,x1,z1);add(a,z0,c,b);add(a,d,c,z1);
  }
  return tiles;
}
export function upperWalkable(r,x,z,liftUp){
  // A continuous opening joins the gallery and platform. The other three
  // platform edges and gallery edges remain guarded; no floor exists in the atrium.
  if(galleryContains(r,x,z,.25))return true;
  if(!liftUp)return false;
  return x>=r.x+1.8&&x<=r.liftX+1.25&&Math.abs(z-r.liftZ)<1.25;
}
