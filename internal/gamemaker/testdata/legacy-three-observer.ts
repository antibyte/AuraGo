  function observeTargets(){
    const look=new T.Raycaster();camera.updateMatrixWorld();
    if(config.mode==='fps')look.setFromCamera({x:0,y:0},camera);else look.set(player.position,new T.Vector3(0,0,1));
    let aimed='',nearest=Infinity;
    const targets=objects.slice(0,256).map((o:any)=>{
      if(!observedIDs.has(o))observedIDs.set(o,'object-'+(++observedID));
      const mesh=o.unit.root,id=o.nodeID||observedIDs.get(o),box=new T.Box3().setFromObject(mesh),center=box.getCenter(new T.Vector3()),size=box.getSize(new T.Vector3());
      const projected=center.clone().project(camera),active=mesh.visible&&!!mesh.parent,record=builder?.nodes.get(o.nodeID);
      const solid=builder?!!o.node?.collider&&!o.node.behaviors.some((b:any)=>['collect','reach','projectile'].includes(b.type)):['tree','building','obstacle'].includes(o.role);
      if(active&&(solid||o.role==='enemy'||o.node?.behaviors.some((b:any)=>['destroy','health'].includes(b.type)))){
        const hit=look.intersectObject(mesh,true)[0];if(hit&&hit.distance<nearest){nearest=hit.distance;aimed=id;}
      }
      const asset_ids=[mesh.userData.assetID,...(mesh.userData.sceneVisuals||[]).map((v:any)=>v.unit.root.userData.assetID)].filter(Boolean);
      return {id,roles:[o.role||'',...(o.node?.behaviors||[]).map((b:any)=>b.type)],asset_ids,x:center.x,y:center.z,z:center.y,w:size.x,h:size.z,depth:size.y,active,visible:active&&projected.z>=-1&&projected.z<=1&&Math.abs(projected.x)<=1&&Math.abs(projected.y)<=1,solid,health:record?.health};
    });
    return {kind:'3d',active:active&&!paused&&!disposed,mode:config.mode,player:{x:player.position.x,y:player.position.z,z:player.position.y,w:.6,h:.6,aim,pitch},eye:{x:camera.position.x,y:camera.position.z,z:camera.position.y},targets,aimed,projectiles:[],bounds:{x:sceneBounds.min[0],y:sceneBounds.min[2],w:sceneBounds.max[0]-sceneBounds.min[0],h:sceneBounds.max[2]-sceneBounds.min[2]},truncated:objects.length>256};
  }
