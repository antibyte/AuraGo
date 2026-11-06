/* Aura Drift - mode checks (sweeper, drift attack, elimination, hot potato, cup points).
 * Run: node scripts/test-aura-drift-modes.mjs
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim();
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}

console.log('Aura Drift modes');
const DT = AD.STEP;
const track = AD.buildTrack(AD.TRACKS[0]);

function make(mode, extra) {
    const race = AD.createRace(Object.assign({ track: track, mode: mode, seed: 7 }, extra || {}));
    race.events = [];
    return race;
}
function events(race, type) {
    return race.events.filter(e => e.type === type);
}
/* Steps until the countdown is over; the player slot idles. */
function runCountdown(race) {
    let guard = 0;
    while (race.state === 'countdown' && guard++ < 2000) AD.stepRace(race, DT);
}
/* An AI-only race: slot 0 is driven by the AI and cannot end the race itself, so
   the mode rule is observed through AD.modeFinished. */
function runAi(race, limit) {
    race.cars[0].isPlayer = false;
    runCountdown(race);
    const startTime = race.time;
    let steps = 0;
    const max = Math.ceil(limit / DT);
    while (!AD.modeFinished(race) && steps++ < max) {
        AD.stepRace(race, DT);
        if (race.mode === 'hot_potato' && !AD.modeFinished(race)) {
            const b = race.bomb;
            if (b.holder < 0 ? !(b.delay > 0) : !race.cars[b.holder].v.hasBomb) {
                check('hot potato bomb has a holder or a pending delay', false, 't=' + race.time.toFixed(2));
                break;
            }
            if (race.cars.filter(c => c.v.hasBomb).length > 1) {
                check('hot potato never has two bombs', false, 't=' + race.time.toFixed(2));
                break;
            }
        }
    }
    return race.time - startTime;
}

/* Cup points. */
check('cup points 25/18/15/12/10/8 then 0', [1, 2, 3, 4, 5, 6, 7].map(AD.cupPoints).join(',') === '25,18,15,12,10,8,0');

/* Mode table. */
check('mode table has the seven modes', AD.MODES.map(m => m.id).join(',') ===
    'quick,time_trial,cup,daily,drift_attack,elimination,hot_potato');
check('rules: hot potato pool and drift attack toggles',
    AD.modeRules('hot_potato').weapons.join(',') === 'air_horn,shield,overdrive' &&
    AD.modeRules('hot_potato').animals === false &&
    AD.modeRules('drift_attack').crates === false && AD.modeRules('drift_attack').noSmashPenalty === true &&
    AD.modeRules('drift_attack').opponents === 0);

/* Elimination: AI only, ends within 200 s with five cars out, first out at 45 s then every 30 s. */
{
    const race = make('elimination');
    const spent = runAi(race, 200);
    const out = race.cars.filter(c => c.v.out);
    check('elimination ends within 200 s', AD.modeFinished(race) && spent <= 200, 'spent ' + spent.toFixed(1));
    check('elimination ends with five out and one winner', out.length === 5 && race.winner >= 0 && !race.cars[race.winner].v.out,
        'out ' + out.length + ' winner ' + race.winner);
    const times = events(race, 'eliminated').map(e => e.t);
    check('eliminations at 45 s then every 30 s', times.length === 5 && times.every((t, i) => Math.abs(t - (45 + 30 * i)) < 0.1),
        times.map(t => t.toFixed(2)).join(','));
    check('eliminated positions count down from 6', events(race, 'eliminated').map(e => e.position).join(',') === '6,5,4,3,2');
    check('elimination warns 5 s before each out', events(race, 'elimination_warn').length === 5);
    check('standings put the winner first and the first out last',
        race.standings[0].index === race.winner && race.standings[5].eliminatedAt === times[0]);
}

/* Elimination through the real path: an idling player is last and goes out first. */
{
    const race = make('elimination');
    let steps = 0;
    while (race.state !== 'finished' && steps++ < 120 * 120) AD.stepRace(race, DT);
    const res = race.result;
    check('elimination: player out ends the race with a result', race.state === 'finished' && !!res);
    const slot = res ? Math.round((res.eliminatedAt - 45) / 30) : -1;
    check('elimination result: out on schedule, position counts down from 6, field 6',
        res && slot >= 0 && Math.abs(res.eliminatedAt - (45 + 30 * slot)) < 0.1 && res.position === 6 - slot &&
        res.field === 6 && res.won === false,
        res && JSON.stringify([res.position, res.eliminatedAt, res.field]));
    check('elimination result keeps the contract fields',
        res && res.mode === 'elimination' && Array.isArray(res.tape) && res.tapeComplete === true && res.title === null &&
        res.tricks && res.tally && typeof res.carId === 'string');
}

/* Hot potato: AI only, ends within 150 s, bomb invariant checked every step. */
{
    const race = make('hot_potato');
    const spent = runAi(race, 150);
    check('hot potato ends within 150 s', AD.modeFinished(race) && spent <= 150, 'spent ' + spent.toFixed(1));
    check('hot potato leaves one car alive', race.cars.filter(c => !c.v.out).length === 1 && race.winner >= 0);
    const given = events(race, 'bomb_given'), blasts = events(race, 'bomb_blast');
    check('hot potato: first bomb at 5 s, one blast per out car',
        given.length === 5 && blasts.length === 5 && Math.abs(given[0].t - 5) < 0.1 && given[0].fuse === 15,
        'given ' + given.length + ' blasts ' + blasts.length);
    check('hot potato: fuse shrinks to the 10 s floor', given.map(e => e.fuse).join(',') === '15,14,13,12,11');
    check('hot potato: next bomb 3 s after a blast', blasts.slice(0, 4).every((b, i) => Math.abs(given[i + 1].t - b.t - 3) < 0.1));
    console.log('  info hot potato passes: ' + events(race, 'bomb_pass').length + ', ticks: ' + events(race, 'bomb_tick').length +
        ', ended after ' + spent.toFixed(1) + ' s');
}

/* Hot potato pass rule: contact moves the bomb, the giver is immune for 1.5 s. */
{
    const race = make('hot_potato');
    race.cars[0].isPlayer = false;
    runCountdown(race);
    while (race.bomb.holder < 0) AD.stepRace(race, DT);
    const holder = race.cars[race.bomb.holder];
    const other = race.cars.find(c => c !== holder);
    race.events.length = 0;
    other.v.x = holder.v.x + 1.5; other.v.y = holder.v.y; other.v.z = holder.v.z;
    AD.updateMode(race, DT);
    const pass = events(race, 'bomb_pass')[0];
    check('contact passes the bomb', !!pass && race.bomb.holder === other.index && other.v.hasBomb && !holder.v.hasBomb);
    AD.updateMode(race, DT);
    check('the giver cannot take it straight back', race.bomb.holder === other.index && race.bomb.immuneFor > 1.4);
    race.bomb.immuneFor = 0;
    AD.updateMode(race, DT);
    check('after the immunity the bomb can move again', race.bomb.holder === holder.index);
}

/* Drift attack: 90 s, no opponents, ends at the clock. */
{
    const race = make('drift_attack');
    check('drift attack has no opponents and a 90 s clock', race.cars.length === 1 && race.clock === 90);
    runCountdown(race);
    const input = race.playerInput;
    const start = race.time;
    let steps = 0;
    while (race.state !== 'finished' && steps++ < 120 * 120) {
        input.throttle = 1;
        input.steer = Math.sin(race.time * 0.9) * 0.8;
        input.handbrake = (Math.sin(race.time * 0.9) > 0.6) ? 1 : 0;
        AD.stepRace(race, DT);
    }
    const spent = race.time - start;
    check('drift attack ends at 90 s', race.state === 'finished' && Math.abs(spent - 90) <= DT * 1.5, 'spent ' + spent.toFixed(3));
    const res = race.result;
    const st = AD.styleSummary(race.cars[0]);
    check('drift attack score is banked plus pending', res && res.score === st.banked + st.pending);
    const medal = res.score >= 50000 ? 'gold' : res.score >= 28000 ? 'silver' : res.score >= 12000 ? 'bronze' : null;
    check('drift attack medal follows 12000/28000/50000', res.medal === medal, res.score + ' ' + res.medal);
    check('drift attack: last ten seconds emit the clock', events(race, 'clock').map(e => e.seconds).join(',') === '10,9,8,7,6,5,4,3,2,1',
        events(race, 'clock').map(e => e.seconds).join(','));
    console.log('  info drift attack scripted score: ' + res.score);
}

/* Sweeper: the first finisher starts 45 s of grace, the player at the back is cut off. */
{
    const race = make('quick', { laps: 1 });
    runCountdown(race);
    let first = null, steps = 0;
    while (race.state !== 'finished' && steps++ < 120 * 400) {
        AD.stepRace(race, DT);
        if (first === null && race.cars.some(c => c.finished)) first = race.time;
    }
    check('sweeper race ends with a result', race.state === 'finished' && !!race.result, 'time ' + race.time.toFixed(1));
    const sw = events(race, 'sweeper');
    check('sweeper fires once at the first finish', sw.length === 1 && first !== null && Math.abs(sw[0].t - first) <= DT * 2);
    check('race ends 45 s after the first finisher', first !== null && Math.abs(race.time - first - 45) <= DT * 2,
        'first ' + first + ' end ' + race.time.toFixed(2));
    const res = race.result;
    check('sweeper result: player did not finish', res && res.dnf === true && res.position >= 1 && res.field === 6);
}

/* Normal finish: player crossing the line ends quick, daily and cup results carry points. */
{
    const race = make('cup', { laps: 1 });
    race.cars[0].finished = true;
    race.cars[0].finishTime = 61.5;
    race.state = 'running';
    AD.stepRace(race, DT);
    check('cup: player finish ends the race', race.state === 'finished' && race.result.mode === 'cup');
    check('cup: result carries position points', race.result.points === AD.cupPoints(race.result.position));
}

if (failures.length) {
    console.log('\n' + failures.length + ' failure(s)');
    process.exit(1);
}
console.log('\nAll mode checks passed');
