import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const context = { window: {}, TextEncoder };
vm.createContext(context);
for (const name of ['content', 'content-act1', 'content-act2', 'content-act3', 'content-act4', 'state']) {
    vm.runInContext(readFileSync(resolve(root, 'ui/js/desktop/apps/goldfish-adventure', name + '.js'), 'utf8'), context);
}
const C = context.window.GoldfishContent;
const S = context.window.GoldfishState;
C.texts = JSON.parse(readFileSync(resolve(root, 'ui/lang/goldfish/de.json'), 'utf8'));
const clone = value => JSON.parse(JSON.stringify(value));
const action = id => C.actions.find(value => value.id === id);
const initial = S.create();

assert.equal(C.rooms.length, 16, 'All sixteen authored rooms must be present');
assert.equal(C.puzzles.length, 18, 'All eighteen puzzle situations must be present');
assert.equal(S.validate(initial).room, 'R01');
assert.equal(S.move(initial, 'R16').error, 'unavailable');
assert.equal(S.apply(initial, 'r01_open').error, 'unavailable');
assert.equal(S.apply(initial, 'unknown').error, 'unavailable');
assert.equal(S.apply(initial, 'r01_stop').state.flags.includes('r01_stopped'), true);
assert.deepEqual(clone(initial), clone(S.create()), 'Rejected actions cannot mutate their input');

const interrupted = S.apply(initial, 'r01_stop').state;
assert.deepEqual(clone(S.pendingResults(interrupted)), clone(action('r01_stop').lines), 'A saved action resumes its result, never its rewards');
const acknowledged = clone(interrupted);
acknowledged.seenLines.push(action('r01_stop').lines[0].text);
S.validate(acknowledged);
assert.equal(S.pendingResults(acknowledged)[0].text, action('r01_stop').lines[1].text, 'Legacy schema-one seen lines retain their meaning');
assert.equal(S.apply(acknowledged, 'r01_stop').changed, false);
const stopped = action('r01_stop'), originalLines = stopped.lines;
try {
    const laterFlag = action('r01_clamp').effects.flags[0];
    stopped.lines = [{ ...originalLines[0], forbids: [laterFlag] }, { ...originalLines[1], requires: [laterFlag] }];
    const later = S.apply(interrupted, 'r01_clamp').state;
    const recovered = S.pendingResults(later).map(value => value.text);
    assert.ok(recovered.includes(originalLines[0].text), 'Recover conditions at the original action, before later flags');
    assert.ok(!recovered.includes(originalLines[1].text), 'Later flags cannot reveal a result that was never eligible');
} finally { stopped.lines = originalLines; }

function path(state, destination) {
    const queue = [[state.room]], visited = new Set([state.room]);
    while (queue.length) {
        const route = queue.shift(), current = route.at(-1);
        if (current === destination) return route.slice(1);
        for (const exit of C.rooms.find(value => value.id === current).exits) {
            if (!S.conditions(state, exit) || visited.has(exit.to)) continue;
            visited.add(exit.to); queue.push(route.concat(exit.to));
        }
    }
    return null;
}

const choices = {
    customs: ['public', 'maintenance'], garden: ['old_path', 'new_path'], hall: ['assembly', 'diagnosis']
};

function excluded(record, routes, ending) {
    if (record.effects?.route && routes[record.effects.route.key] !== record.effects.route.value) return true;
    if (record.effects?.ending && ending !== record.effects.ending) return true;
    if (record.checkpoint) {
        const key = { 'pre-customs': 'customs', 'pre-garden': 'garden', 'pre-hall': 'hall' }[record.checkpoint];
        const token = { public: 'public', maintenance: 'maintenance', old_path: 'old', new_path: 'new', assembly: 'assembly', diagnosis: 'diagnosis' }[routes[key]];
        if (key && !record.id.includes(token)) return true;
    }
    return false;
}

let tested = 0, transitions = 0;
for (const customs of choices.customs) for (const garden of choices.garden) for (const hall of choices.hall) {
    for (const ending of ['keys', 'custody', 'purge']) {
        let state = S.create();
        const routes = { customs, garden, hall };
        const snapshots = [];
        for (let pass = 0; pass < C.actions.length + 5 && !state.ending; pass++) {
            let changed = false;
            for (const record of C.actions) {
                if (state.applied.includes(record.id) || record.repeatable || excluded(record, routes, ending)) continue;
                const route = path(state, record.room);
                if (route === null) continue;
                const candidate = clone(state); candidate.room = record.room;
                if (!S.available(candidate, record)) continue;
                for (const to of route) {
                    const result = S.move(state, to);
                    assert.equal(result.error, undefined, 'Every journey move uses a real unlocked exit');
                    state = result.state;
                }
                const before = clone(state);
                if (record.challenge) {
                    const wrong = S.apply(state, record.id, { item: record.item, answer: '__wrong__' });
                    assert.equal(wrong.error, 'wrong');
                    assert.deepEqual(clone(wrong.state), before, 'Wrong answers cannot consume items or award flags');
                    assert.deepEqual(clone(state), before, 'Reducer input stays immutable');
                }
                if (record.item) {
                    assert.equal(S.apply(state, record.id, { answer: record.challenge?.answer }).error, 'item_needed');
                }
                const result = S.apply(state, record.id, { item: record.item, answer: record.challenge?.answer });
                assert.equal(result.changed, true, record.id + ' should advance this route');
                state = result.state;
                assert.equal(S.apply(state, record.id, { item: record.item, answer: record.challenge?.answer }).changed, false, 'Rewards cannot repeat');
                const validated = S.validate(JSON.parse(JSON.stringify(state)));
                assert.deepEqual(clone(validated), clone(state), record.id + ' round-trip');
                snapshots.push(clone(state));
                transitions++;
                changed = true;
                if (state.ending) break;
            }
            if (!changed) break;
        }
        assert.equal(state.ending, ending, `Reach ending ${ending} on ${JSON.stringify(routes)} (last actions ${state.applied.slice(-8)})`);
        assert.deepEqual(clone(state.routes), routes);
        assert.equal(state.completed.length, 15, 'Exactly fifteen of eighteen puzzles complete per run');
        assert.ok(state.flags.includes('maintenance_authority'));
        assert.ok(state.flags.includes('services_safe'));
        assert.equal(S.move(state, 'R16').changed, true, 'Every ending has a playable observatory');
        for (const record of C.actions.filter(value => !value.repeatable)) {
            const final = clone(state); final.room = record.room;
            assert.equal(S.available(final, record), false, 'Committed endings freeze further puzzle changes');
        }
        const observatory = S.move(state, 'R16').state;
        const epilogue = C.actions.find(value => value.room === 'R16' && S.available(observatory, value));
        assert.ok(epilogue, 'At least one optional epilogue remains playable');
        const farewell = S.apply(observatory, epilogue.id);
        S.validate(farewell.state);
        assert.equal(farewell.state.ending, ending, 'Epilogues preserve the ending');
        const damaged = clone(state); damaged.routes.customs = customs === 'public' ? 'maintenance' : 'public';
        assert.throws(() => S.validate(damaged), /impossible/);
        const omitted = clone(state); omitted.applied = omitted.applied.slice(1);
        assert.throws(() => S.validate(omitted), /impossible|invalid/);
        for (const snapshot of snapshots.filter(value => value.completed.length % 4 === 0)) S.validate(snapshot);
        tested++;
    }
}

// A flag that a content change retired still loads: the run keeps its progress and loses the flag.
let retired = S.apply(initial, 'r01_stop').state;
retired = S.apply(retired, 'r01_clamp').state;
retired = S.apply(retired, 'r01_brake', { answer: ['stop', 'hold', 'release'] }).state;
retired = S.apply(retired, 'r01_open').state;
const withRetired = clone(retired);
withRetired.flags.push('e_scrap_refusal');
const migrated = S.validate(withRetired);
assert.equal(migrated.flags.includes('e_scrap_refusal'), false, 'A retired flag is dropped on load');
assert.deepEqual(clone(migrated.flags), clone(retired.flags), 'The migration keeps every earned flag');
assert.deepEqual(clone(migrated.applied), clone(retired.applied), 'The migration keeps the applied actions');
const unknownFlag = clone(retired);
unknownFlag.flags.push('e_not_a_flag');
assert.throws(() => S.validate(unknownFlag), /invalid|impossible/, 'An unknown flag is still rejected');

for (const mutate of [
    value => { value.schema = 999; },
    value => { value.contentVersion = 999; },
    value => { value.inventory = ['__proto__']; },
    value => { value.flags = ['p18_done']; },
    value => { value.room = 'R16'; value.visited.push('R16'); },
    value => { value.routes = { customs: 'public' }; },
    value => { value.position.x = 4; },
    value => { value.settings.sound = 'yes'; },
    value => { value.elapsedMs = Infinity; },
    value => { value.url = '/files/private'; },
    value => { value.seenLines = ['<script>bad()</script>']; },
    value => { value.applied = Array(2050).fill('r01_stop'); }
]) {
    const damaged = clone(initial); mutate(damaged);
    assert.throws(() => S.validate(damaged), undefined, 'Malformed or impossible import must fail');
}
assert.throws(() => S.validate(JSON.parse('{"schema":1,"__proto__":{"polluted":true}}')), /invalid/);
assert.equal({}.polluted, undefined);
console.log(`Goldfish state: ${tested} route/ending journeys, ${transitions} atomic transitions; invalid imports, item selection, wrong answers and duplicate rewards passed.`);
