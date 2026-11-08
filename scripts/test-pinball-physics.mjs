import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { createSimulation, segmentBox, validatePhysics } from '../ui/js/desktop/apps/pinball/physics.js';
import { STEP, BALL_MASS, BALL_RADIUS, TABLES } from '../ui/js/desktop/apps/pinball/tables.js';

assert.ok(segmentBox([-1,0,0],[1,0,0],[0,0,0],[.01,.01,.01]));
assert.equal(segmentBox([-1,1,0],[1,1,0],[0,0,0],[.01,.01,.01]),false);
for(const table of TABLES){
    const data=JSON.parse(await fs.readFile(new URL('../ui/3d/pinball/v1/'+table.id+'-physics.json',import.meta.url)));
    assert.throws(()=>validatePhysics({...data,meshes:[{surface:'metal',vertices:[Infinity,0,0],indices:[0,0,0]}]}));
    const sim=await createSimulation(data);
    try{
        sim.feed(1);for(let i=0;i<960;i++)sim.step({plunger:i<480?1:0});
        assert.ok(sim.events().some(e=>e.type==='launch'),'spring launcher event');
        assert.ok(sim.snapshot().balls.some(b=>b.position[2]>.4),'spring transfers the ball to the upper playfield');
        sim.clearBalls();
        sim.setGate(true);for(let i=0;i<160;i++)sim.step();
        sim.placeBall(1,[0,.014,1.10]);
        for(let i=0;i<160;i++)sim.step();
        assert.equal(sim.events().filter(e=>e.type==='lock').length,1,'pocket only triggers after the ball physically falls below the playfield');
        sim.eject(1);for(let i=0;i<120;i++)sim.step();
        assert.ok(sim.snapshot().balls[0]?.position[1]>-.01,'physical pocket ejector clears the opening');
        sim.clearBalls();sim.setGate(false);
        for(const [side,x] of [['left',-.08],['right',.08]]){
            for(let i=0;i<120;i++)sim.step({[side]:true});
            sim.placeBall(1,[x,.014,.27],[0,0,-.4]);
            for(let i=0;i<480;i++)sim.step({[side]:true});
            const caught=sim.snapshot().balls[0];
            assert.ok(caught&&Math.hypot(...caught.velocity)<.03&&Math.abs(caught.position[0])>.06&&Math.abs(caught.position[0])<.12,'raised '+side+' flipper physically cradles the ball');
            const caughtPosition=[...caught.position];
            for(let i=0;i<144;i++)sim.step();
            const released=sim.snapshot().balls[0];assert.ok(!released||Math.hypot(...released.position.map((n,i)=>n-caughtPosition[i]))>.012,'lowering the '+side+' flipper releases its cradle');
            sim.clearBalls();
        }
        if(data.magnet){
            const field=await createSimulation(data);
            try{
                field.placeBall(1,[.03,.014,.76],[-.1,0,0]);
                for(let i=0;i<120;i++)field.step();
                const ball=field.snapshot().balls[0];assert.ok(ball&&Math.hypot(ball.position[0],ball.position[2]-.76)<.025,'bounded magnet captures a slow ball');
                assert.equal(field.events().filter(e=>e.type==='magnet').length,1);
            }finally{field.dispose();}
        }
        for(const side of [-1,1]){
            sim.placeBall(1,[0,.014,.46],[side*25,0,0]);
            for(let i=0;i<96;i++){sim.step();const ball=sim.snapshot().balls[0];assert.ok(ball,table.id+': fast ball stays inside');assert.ok(Math.abs(ball.position[0])<.35,table.id+': CCD wall containment');}
            sim.clearBalls();
        }
        for(const timing of [0,4,8,16]){
            sim.placeBall(1,[-.07,.02,.34],[0,0,-8]);
            for(let i=0;i<72;i++)sim.step({left:i>=timing});
            const ball=sim.snapshot().balls[0];assert.ok(ball&&ball.position.every(Number.isFinite),'moving flipper contact remains finite');sim.clearBalls();
        }
        sim.placeBall(1,[-.06,.12,.55]);
        const gravity=[0,9.81*Math.cos(6.5*Math.PI/180),9.81*Math.sin(6.5*Math.PI/180)];
        const energy=b=>.5*BALL_MASS*b.velocity.reduce((a,n)=>a+n*n,0)+.2*BALL_MASS*BALL_RADIUS**2*b.angularVelocity.reduce((a,n)=>a+n*n,0)+BALL_MASS*b.position.reduce((a,n,i)=>a+n*gravity[i],0);
        const initial=energy(sim.snapshot().balls[0]);
        for(let i=0;i<120;i++){sim.step();const b=sim.snapshot().balls[0];assert.ok(b);assert.ok(energy(b)<=initial+.0003,'passive surfaces do not add energy');}
        sim.clearBalls();
        for(let id=1;id<=3;id++)sim.placeBall(id,[(id-2)*.04,.015,.52],[0,0,-1]);
        for(let i=0;i<240;i++)sim.step({left:true,right:true});
        assert.ok(sim.snapshot().balls.every(b=>b.position.every(Number.isFinite)),'multiball remains finite');
        sim.clearBalls();sim.feed(10);sim.feed(11,true);sim.setCoils(false);sim.step();
        assert.equal(sim.stats().pending,0,'tilt returns queued balls to the trough');sim.clearBalls();
        sim.setCoils(true);
        for(let id=10;id<=12;id++)sim.feed(id,true);
        for(let i=0;i<4800;i++)sim.step();
        assert.equal(new Set(sim.events().filter(e=>e.type==='launch').map(e=>e.ball)).size,3,'three queued multiball balls launch through the real shooter');
        sim.clearBalls();const returned=sim.placeBall(20,data.shooter);returned.shot=true;
        for(let i=0;i<1500;i++)sim.step();
        assert.ok(sim.events().some(e=>e.type==='launch'&&e.ball===20),'a returned shooter ball cannot block automatic multiball feeding');
    }finally{sim.dispose();sim.dispose();}
    const replays=[];
    for(const fps of [30,60,144]){
        const replay=await createSimulation(data);replay.feed(1);let accumulator=0,tick=0;
        for(let frame=0;frame<fps*2;frame++){
            accumulator+=1/fps;
            while(accumulator+1e-10>=STEP&&tick<960){replay.step({plunger:tick<480?1:0,left:tick>=650&&tick<740,right:tick>=750});tick++;accumulator-=STEP;}
        }
        assert.equal(tick,960);replays.push(replay.snapshot());replay.dispose();
    }
    assert.deepEqual(replays[0],replays[1]);assert.deepEqual(replays[1],replays[2]);
    console.log('Pinball physics '+table.id+': physical spring, 25 m/s CCD, flipper timing, passive energy, multiball, tilt and 30/60/144 Hz parity passed');
}
