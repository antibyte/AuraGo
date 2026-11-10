// Phase 3 lobby registry: design numbers, ground, step rule, walls, solids, doors and the residents' graph.
import assert from 'node:assert/strict';
import * as L from '../ui/js/desktop/apps/sysworld-lobbies.js';
const {PODIUMS,LOBBIES,DOORS,PODIUM,FLIGHT,RISER,TREAD,STEPS,DOOR_TIME,SOLID_TOP}=L;
const near=(a,b,eps=1e-9)=>Math.abs(a-b)<=eps;
// Design table (plans/2026-11-10-system-world-beauty-phase-3-design.md) and the kit v3 stair profile.
assert.ok(near(FLIGHT,1.6)&&near(RISER,.88/6)&&STEPS===5&&TREAD===.32&&PODIUM===.88,'stair profile');
assert.deepEqual(PODIUMS.map(p=>p.id),['agent','memory','integrations','missions','infra','graph','operations']);
assert.deepEqual(LOBBIES.map(l=>l.id),['agent','memory','integrations-west','integrations-east','missions','infra','graph']);
assert.equal(DOORS.length,12);assert.equal(new Set(DOORS.map(d=>d.key)).size,12);
L.clearRoom(LOBBIES[0]).forEach((v,i)=>assert.ok(near(v,[-6.9,-18.9,6.9,-5.1][i],1e-9),'agent clear room'));
const atrium=LOBBIES.find(l=>l.id==='graph');assert.ok(near(atrium.front,45.13,.005)&&atrium.open&&atrium.ceiling===null,'atrium');
assert.ok(LOBBIES.every(l=>l.doors.every(d=>d.height>=2.9)),'every door clears the visitor (eye 2.4 m)');
assert.equal(DOORS.find(d=>d.key==='lobby:agent:0').z,-4.75);
// Ground: podium top, treads, nothing beyond the flight or beside it; the beacon has stairs too.
assert.equal(L.lobbyGround(0,-12),PODIUM);
assert.ok(near(L.lobbyGround(0,-.5+.16),PODIUM-RISER));assert.ok(near(L.lobbyGround(0,-.5+FLIGHT-.01),RISER));
assert.equal(L.lobbyGround(0,-.5+FLIGHT+.01),null);assert.equal(L.lobbyGround(5,.3),null);assert.equal(L.lobbyGround(0,5),null);
assert.ok(near(L.lobbyGround(0,40.6),PODIUM-RISER),'beacon flight');
// Step rule: up the stairs passes, the podium edge and the side of a flight do not; 0.14/0.30/0.147 pass, 0.36 not.
const h=(x,z)=>L.lobbyGround(x,z)??0;
assert.ok(L.stepFree({x:0,z:3},{x:0,z:-3},h),'up the agent stairs');
assert.equal(L.stepFree({x:10,z:1},{x:10,z:-1},h),false,'podium edge');
assert.equal(L.stepFree({x:3.6,z:.3},{x:4.2,z:.3},h),false,'off the side of a flight');
for(const [rise,ok]of[[.14,true],[.30,true],[RISER,true],[.36,false]])assert.equal(L.stepFree({x:12,z:-10},{x:12.4,z:-10},x=>x>12.2?rise:0),ok,'step '+rise);
// Rooms, walls, gaps, corner piers, the atrium polygon, the integration passage.
assert.equal(L.lobbyAt(0,-12)?.id,'agent');assert.equal(L.lobbyAt(0,-4.9),null);
assert.equal(L.lobbyWall(0,-4.9),false,'door gap');assert.equal(L.lobbyWall(3,-4.9),true);assert.equal(L.lobbyWall(-6.6,-5.4),true,'corner pier');
assert.equal(L.lobbyWall(0,-3),false);assert.equal(L.lobbyAt(-43,36)?.id,'graph');
assert.equal(L.lobbyWall(-43,45),false);assert.equal(L.lobbyWall(-40,45),true);assert.equal(L.lobbyWall(43,-10),false,'integration passage');
assert.ok(L.lobbyAirspace(0,3.3,-12)&&!L.lobbyAirspace(0,9,-12)&&!L.lobbyAirspace(0,3.3,3),'airspace');
// Passage: closed door, open door, wall.
assert.equal(L.lobbyPassage({x:0,z:-4},{x:0,z:-5.5},()=>false),false);
assert.ok(L.lobbyPassage({x:0,z:-4},{x:0,z:-5.5},k=>k==='lobby:agent:0'));
assert.equal(L.lobbyPassage({x:3,z:-4},{x:3,z:-5.5},()=>true),false);
// Solids: counts per building, lintel and ceiling heights, atrium lid, nothing for the beacon.
const agent=L.lobbySolids('agent');assert.equal(agent.length,15);
const lintel=agent.find(b=>near(b.min[1],PODIUM+3.5));assert.ok(lintel&&near(lintel.min[0],-2)&&near(lintel.max[0],2)&&lintel.max[1]===SOLID_TOP,'agent lintel');
assert.ok(agent.some(b=>b.min[1]===5&&b.max[1]===SOLID_TOP),'agent ceiling');
assert.equal(L.lobbySolids('memory').length,13);assert.equal(L.lobbySolids('integrations').length,22);assert.equal(L.lobbySolids('missions').length,17);
assert.equal(L.lobbySolids('infra').length,19);assert.equal(L.lobbySolids('graph').length,15);assert.deepEqual(L.lobbySolids('operations'),[]);
assert.ok(L.lobbySolids('graph').some(b=>near(b.min[1],4.7+.4)),'atrium lid');
// Doors.
{
  const doors=L.createLobbyDoors(),key='lobby:agent:0',at={x:0,z:-3.5};
  doors.update(.1,at,[]);assert.equal(doors.value(key),0,'no interior shown: the visitor waits');assert.equal(doors.usable(key),false);
  doors.update(DOOR_TIME,null,[at]);assert.equal(doors.value(key),1,'residents open doors');assert.equal(doors.passable(key),false);
  doors.update(DOOR_TIME,null,[]);assert.equal(doors.value(key),0);
  doors.setReady('agent',true);assert.equal(doors.update(DOOR_TIME/2,at,[]).length,1,'a door reports when it starts to move');
  assert.ok(near(doors.value(key),.5)&&!doors.passable(key));doors.update(DOOR_TIME*.4,at,[]);assert.ok(doors.passable(key));
  doors.update(1,null,[],true);assert.equal(doors.value(key),0,'reduced motion snaps');
  assert.ok(doors.hold(key));doors.update(0,null,[],true);assert.equal(doors.value(key),1,'held open');
  doors.setTier('low');doors.update(0,at,[],true);assert.equal(doors.value(key),0,'low: shut for the visitor');assert.equal(doors.usable(key),false);
  doors.update(0,null,[at],true);assert.equal(doors.value(key),1,'low: residents still pass');assert.equal(doors.passable(key),false);
}
// Graph.
{
  const g=L.lobbyGraph(null),kind=k=>g.nodes.filter(n=>n.kind===k).length;
  assert.deepEqual([kind('foot'),kind('landing'),kind('porch'),kind('entry')],[10,10,12,12]);
  for(const n of g.nodes)assert.equal(L.lobbyAt(n.x,n.z)?.id||null,n.lobby,'node on the wrong side: '+JSON.stringify(n));
  for(const l of LOBBIES)assert.ok(g.nodes.some(n=>n.lobby===l.id&&n.kind==='grid'),l.id+' grid');
  assert.ok(g.links.every(([a,b])=>g.nodes[a]&&g.nodes[b]&&a!==b));
  const adj=g.nodes.map(()=>[]);for(const [a,b]of g.links){adj[a].push(b);adj[b].push(a);}
  g.nodes.forEach((n,i)=>{if(n.kind!=='entry')return;const seen=new Set([i]),queue=[i];let out=false;
    while(queue.length&&!out){const j=queue.pop();if(g.nodes[j].kind==='foot')out=true;for(const k of adj[j])if(!seen.has(k)){seen.add(k);queue.push(k);}}
    assert.ok(out,'entry without a way down to the pavement: '+JSON.stringify(n));});
  const fake=new Map([['lobby-agent',{navigation:{workpoints:[{id:'desk',position:[-1.35,.9,1.5]},{id:'outside',position:[0,.9,9]}]}}]]);
  assert.deepEqual(L.lobbyGraph(fake).nodes.filter(n=>n.kind==='work').map(n=>[n.x,n.z]),[[-1.35,-10.5]]);
  const places=L.lobbyPlaces(fake);assert.equal(places.length,1);
  assert.deepEqual(places[0].place,{id:'lobby-agent',district:'agent',x:0,z:-12,role:'technician',lobby:true});
  console.log(`System World lobbies: ${LOBBIES.length} halls, ${DOORS.length} doors, ${g.nodes.length} graph nodes / ${g.links.length} links; ground, step rule, walls, solids and doors verified.`);
}
