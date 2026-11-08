import assert from 'node:assert/strict';
import { createRound, currentPlayer, handleEvent, tickRound, takeActions, nudgeRound, gateOpen } from '../ui/js/desktop/apps/pinball/rules.js';
import { normalizeSettings, normalizeScores, TABLES } from '../ui/js/desktop/apps/pinball/tables.js';

const hit=(g,type,id=type)=>handleEvent(g,{type,id,ball:g.active[0]});
for(const table of TABLES){
    const game=createRound(table.id,['A','B']);takeActions(game);
    hit(game,'launch');assert.equal(game.saverUntil,10);
    tickRound(game,4);const old=game.active[0];handleEvent(game,{type:'drain',ball:old});
    assert.equal(game.active.length,1);assert.equal(game.turn,0);assert.equal(takeActions(game).filter(a=>a.type==='feed').length,1);
    handleEvent(game,{type:'drain',ball:old});assert.equal(game.active.length,1,'duplicate drain must not duplicate replacement');
    hit(game,'lock');assert.equal(currentPlayer(game).locks,0,'unqualified pocket entry must only eject');takeActions(game);
    for(let lock=0;lock<3;lock++){
        for(let i=0;i<3;i++)hit(game,'target','target-'+i);
        assert.ok(gateOpen(game));const points=currentPlayer(game).score;hit(game,'target','target-0');assert.equal(currentPlayer(game).score,points);
        hit(game,'lock');assert.equal(currentPlayer(game).bank.length,0);
    }
    assert.equal(game.multiball,true);assert.equal(game.active.length,3);
    assert.equal(takeActions(game).filter(a=>a.type==='feed').length,2);
    for(const [index,goal] of table.goals.entries()){
        for(let i=0;i<goal.count;i++)hit(game,goal.type,goal.type==='target'?'mission-target-'+i:'shot-'+i%2);
        assert.ok(currentPlayer(game).completed[index],table.id+': mission '+index);
    }
    assert.ok(currentPlayer(game).finale);
    tickRound(game,30);const ids=[...game.active];
    handleEvent(game,{type:'drain',ball:ids[0]});handleEvent(game,{type:'drain',ball:ids[1]});
    assert.equal(game.phase,'playing');assert.equal(game.turn,0);
    const jackpots=currentPlayer(game).jackpots;hit(game,'ramp','finale-shot');assert.equal(currentPlayer(game).jackpots,jackpots+1,'qualified finale keeps lit jackpots after multiball ends');
    handleEvent(game,{type:'drain',ball:ids[2]});assert.equal(game.phase,'bonus');
    tickRound(game,32);assert.equal(game.turn,1);assert.equal(currentPlayer(game).score,0);assert.equal(currentPlayer(game).locks,0);assert.deepEqual(currentPlayer(game).progress,[0,0,0]);
    assert.equal(game.players[0].played,1);
}
const tilt=createRound('neon',['A']);takeActions(tilt);assert.equal(nudgeRound(tilt),false,'cannot tilt a ball still in the trough');hit(tilt,'launch');
for(let second=0;second<3;second++){tickRound(tilt,second*1.1);for(let n=0;n<4;n++)nudgeRound(tilt);}
assert.equal(currentPlayer(tilt).tilt,true);assert.equal(currentPlayer(tilt).bonus,0);assert.equal(tilt.saverUntil,0);
const before=currentPlayer(tilt).score;hit(tilt,'bumper');assert.equal(currentPlayer(tilt).score,before);
hit(tilt,'drain');assert.equal(tilt.phase,'bonus');
const recovery=createRound('neon',['A']);takeActions(recovery);tickRound(recovery,30);hit(recovery,'drain','search');assert.equal(recovery.phase,'playing');assert.equal(currentPlayer(recovery).played,0);assert.equal(recovery.active.length,1,'failed mechanical search replaces the ball through the shooter');

const complete=createRound('temple',['A','B','C','D']);takeActions(complete);
for(let ball=0;ball<12;ball++){tickRound(complete,ball*4+20);hit(complete,'drain');tickRound(complete,ball*4+22);}
assert.equal(complete.phase,'finished');assert.ok(complete.players.every(p=>p.played===3));
const training=createRound('neon',['A','B'],true);assert.equal(training.players.length,1);takeActions(training);
for(let i=0;i<5;i++){tickRound(training,100+i);hit(training,'drain');assert.equal(training.phase,'playing');assert.equal(training.players[0].played,0);assert.equal(training.active.length,1);}
hit(training,'launch');currentPlayer(training).tilt=true;training.saverUntil=0;hit(training,'drain');assert.equal(currentPlayer(training).tilt,false);assert.equal(training.active.length,1,'training recovers from tilt');
assert.equal(normalizeSettings({effects:0,ambience:0}).effects,0);
assert.equal(normalizeSettings({effects:NaN}).effects,.65);
assert.equal(normalizeSettings({keys:{left:'Space'}}).keys.left,'ShiftLeft');
assert.equal(normalizeScores({neon:[{name:'A',score:Infinity,date:1}]}).neon.length,0);
assert.equal(normalizeScores({neon:Array.from({length:20},(_,i)=>({name:'A',score:i,date:1}))}).neon.length,10);
console.log('Pinball rules: missions, qualified locks, multiball, duplicate drains, hotseat, tilt, training and persistence passed');
