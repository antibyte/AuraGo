import {footprints,upperSolids} from './sysworld-exploration.js';

// Ground apertures are authored separately from upper architecture. Using one
// facade box would close the integration gate and every accessible pavilion.
export function installCityColliders(traffic,catalog,districts,placements) {
  for(const d of districts) {
    const bounds=catalog.get(d.asset).lods[0].bounds;
    const ground=footprints[d.id]||(d.id==='graph'?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]]);
    ground.forEach(([x0,z0,x1,z1],i)=>traffic.solid('district:'+d.id+':'+i,{x:d.x,z:d.z,min:[x0,0,z0],max:[x1,8,z1]}));
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
}
