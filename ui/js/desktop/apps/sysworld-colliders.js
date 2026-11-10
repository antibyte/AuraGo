import {footprints,upperSolids} from './sysworld-exploration.js';
import {LOBBIES,LOBBY_DISTRICTS,lobbySolids} from './sysworld-lobbies.js';

// Ground apertures are authored separately from upper architecture. Using one
// facade box would close the integration gate and every accessible pavilion.
// One catalog for colliders and tests: entries of later catalogs (kit v3) replace earlier ones
// (kit v1) with the same id. Accepts Maps or manifests ({assets: [...]}).
export function kitCatalog(...catalogs){
  const merged=new Map();
  for(const c of catalogs)for(const [id,a] of c instanceof Map?c:(c?.assets||[]).map(a=>[a.id,a]))merged.set(id,a);
  return merged;
}
export function installCityColliders(traffic,catalog,districts,placements,kit=[]) {
  for(const d of districts) {
    const bounds=catalog.get(d.asset).lods[0].bounds;
    const ground=footprints[d.id]||(LOBBY_DISTRICTS.has(d.id)?[]:[[-3,-3,3,3]]);
    ground.forEach(([x0,z0,x1,z1],i)=>traffic.solid('district:'+d.id+':'+i,{x:d.x,z:d.z,min:[x0,0,z0],max:[x1,8,z1]}));
    // Enterable buildings: walls with door gaps, corner piers, lintels, ceiling slab, towers, struts (registry),
    // plus the interior's fixtures from its kit v3 manifest (building-local glTF boxes at the district origin).
    lobbySolids(d.id).forEach((box,i)=>traffic.solid('district:'+d.id+':lobby:'+i,box));
    for(const asset of new Set(LOBBIES.filter(l=>l.district===d.id).map(l=>l.asset)))
      (catalog.get(asset)?.navigation?.colliders||[]).forEach((b,i)=>traffic.solid('district:'+d.id+':'+asset+':'+i,{x:d.x,z:d.z,min:[b[0],b[1],b[2]],max:[b[3],b[4],b[5]]}));
    if(upperSolids[d.id])upperSolids[d.id].forEach(([half,from,to],i)=>traffic.solid('district:'+d.id+':upper:'+i,{x:d.x,z:d.z,min:[-half,from,-half],max:[half,to,half]}));
    else traffic.solid('district:'+d.id+':upper',{x:d.x,z:d.z,min:[bounds.min[0],8,bounds.min[2]],max:bounds.max});
  }
  placements.forEach((p,i)=>{
    if(p.asset==='street-tile'||p.asset==='street-crossing')return;
    const bounds=catalog.get(p.asset)?.lods[0].bounds;if(!bounds)return;
    const min=bounds.min.map((v,k)=>v*p.scale[k]),max=bounds.max.map((v,k)=>v*p.scale[k]);min[1]+=p.y;max[1]+=p.y;
    if(p.asset==='street-lamp') {
      traffic.solid('city:'+i+':pole',{x:p.x,z:p.z,min:[-.2,p.y,-.2],max:[.2,max[1],.2]});
      min[1]=Math.max(min[1],max[1]-1);
    }
    traffic.solid('city:'+i,{x:p.x,z:p.z,heading:p.angle,min,max});
  });
  // Kit v3 placements (phase 4 furniture; street modules have none): every navigation collider of the design,
  // model-local glTF boxes turned with the placement. The owner lets a probe ignore one piece's own solids.
  kit.forEach((p,i)=>(catalog.get(p.asset)?.navigation?.colliders||[]).forEach((b,k)=>traffic.solid('kit:'+i+':'+k,
    {owner:'kit:'+i,x:p.x,z:p.z,heading:p.angle,min:[b[0],p.y+b[1],b[2]],max:[b[3],p.y+b[4],b[5]]})));
}
