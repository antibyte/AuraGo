import {startGame} from './common';
import * as A from '../vendor/aurago-three-assets-1.js';
const T=A.THREE;
let obstacle:any,goal:any;
const api=startGame({mode:'exploration',objective:'Explore the trail, then the fort',speed:6,goal:10,duration:0,lives:2,objects:[],
  levels:[{id:'trail',title:'The trail'},{id:'fort',title:'The fort'}],worldBounds:{min:[-50,-1,-20],max:[50,30,100]},
  setup(api:any){
    obstacle=new T.Mesh(new T.BoxGeometry(1,1,1),new T.MeshStandardMaterial({color:0xf87171}));obstacle.position.set(0,.5,3);api.scene.add(obstacle);
    goal=new T.Mesh(new T.BoxGeometry(2,2,2),new T.MeshStandardMaterial({color:0xa78bfa}));goal.position.set(0,1,api.levelIndex===0?16:25);api.scene.add(goal);
    (window as any).fixture=api;api.setCheckpoint([0,0,0]);
  },
  step(dt:number,api:any){
    if(obstacle.visible&&Math.abs(api.player.position.z-obstacle.position.z)<.7&&api.damagePlayer(100)){
      api.event('hit',obstacle.position.toArray());if(api.levelIndex===0)obstacle.visible=false;
    }
    if(Math.abs(api.player.position.z-goal.position.z)<1)api.win();
  },
  dispose(){for(const o of [obstacle,goal]){o?.geometry.dispose();o?.material.dispose();}}
});
