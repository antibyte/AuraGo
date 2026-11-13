/* Test-only reachable journeys and real-app save/reopen setup. Never shipped by the app loader. */
(function () {
    'use strict';
    const C = window.GoldfishContent, S = window.GoldfishState;
    const clone = value => JSON.parse(JSON.stringify(value));
    const rooms = Object.fromEntries(C.rooms.map(room => [room.id, []]));
    const actions = {}, signatures = new Set();
    let manifest, coverage;

    function path(state, destination) {
        const queue = [[state.room]], seen = new Set([state.room]);
        while (queue.length) {
            const route = queue.shift(), current = route.at(-1);
            if (current === destination) return route.slice(1);
            for (const exit of C.rooms.find(room => room.id === current).exits) {
                if (seen.has(exit.to) || !S.conditions(state, exit)) continue;
                seen.add(exit.to); queue.push(route.concat(exit.to));
            }
        }
        return null;
    }

    function capture(state) {
        const room = C.rooms.find(room => room.id === state.room), art = manifest.rooms[state.room];
        // Branches matter only in their own room; unrelated route history cannot multiply geometry cases.
        const routeKey = {R07: 'customs', R09: 'garden', R12: 'hall'}[state.room];
        const signature = JSON.stringify([state.room, (art.actors || []).filter(actor => S.conditions(state, actor)),
            (art.layers || []).filter(layer => S.conditions(state, layer)),
            room.exits.map(exit => S.conditions(state, exit)), routeKey && state.routes[routeKey], state.room === 'R16' && state.ending]);
        if (!signatures.has(signature)) {
            signatures.add(signature);
            rooms[state.room].push(S.validate(clone(state)));
        }
        for (const action of C.actions) if (!actions[action.id] && S.available(state, action)) actions[action.id] = clone(state);
    }

    async function build() {
        manifest = await (await fetch('/img/goldfish-adventure/manifest.json')).json();
        let journeys = 0;
        for (const customs of ['public', 'maintenance']) for (const garden of ['old_path', 'new_path'])
            for (const hall of ['assembly', 'diagnosis']) for (const ending of ['keys', 'custody', 'purge']) {
                const routes = {customs, garden, hall};
                let state = S.create(); capture(state);
                for (let pass = 0; pass < C.actions.length + 5 && !state.ending; pass++) {
                    let changed = false;
                    for (const action of C.actions) {
                        if (action.repeatable || state.applied.includes(action.id)) continue;
                        if (action.effects?.route && routes[action.effects.route.key] !== action.effects.route.value) continue;
                        if (action.effects?.ending && ending !== action.effects.ending) continue;
                        const key = {'pre-customs': 'customs', 'pre-garden': 'garden', 'pre-hall': 'hall'}[action.checkpoint];
                        const token = {public: 'public', maintenance: 'maintenance', old_path: 'old', new_path: 'new', assembly: 'assembly', diagnosis: 'diagnosis'}[routes[key]];
                        if (key && !action.id.includes(token)) continue;
                        const route = path(state, action.room);
                        if (route === null || !S.available({...state, room: action.room}, action)) continue;
                        for (const room of route) { state = S.move(state, room).state; capture(state); }
                        const result = S.apply(state, action.id, {item: action.item, answer: action.challenge?.answer});
                        if (!result.changed) throw new Error('Unreachable action ' + action.id);
                        state = result.state; capture(state); changed = true;
                        if (state.ending) break;
                    }
                    if (!changed) break;
                }
                if (state.ending !== ending) throw new Error('Journey did not reach ' + ending);
                for (const room of path(state, 'R16') || []) { state = S.move(state, room).state; capture(state); }
                journeys++;
            }
        const unreachableLocked = [], exitCases = [];
        for (const room of C.rooms) {
            if (!rooms[room.id].length) throw new Error('No reachable fixture ' + room.id);
            for (const exit of room.exits) {
                if (!rooms[room.id].some(state => S.conditions(state, exit))) throw new Error('No open exit fixture ' + exit.id);
                const locked = rooms[room.id].some(state => !S.conditions(state, exit));
                if ((exit.requires || []).length && !locked) {
                    // Every entry into the ship needs p04_done, awarded together with ship_ready.
                    const grantors = C.actions.filter(a => a.effects?.flags?.includes('p04_done'));
                    const reached = new Set(['R01']), queue = ['R01'];
                    while (queue.length) {
                        const current = queue.shift();
                        for (const edge of C.rooms.find(r => r.id === current).exits) {
                            if (edge.requires?.includes('p04_done') || reached.has(edge.to)) continue;
                            reached.add(edge.to); queue.push(edge.to);
                        }
                    }
                    if (room.id !== 'R05' || JSON.stringify(exit.requires) !== '["ship_ready"]' || reached.has('R05') ||
                        !grantors.length || !grantors.every(a => a.effects.flags.includes('ship_ready'))) throw new Error('Missing locked fixture ' + exit.id);
                    unreachableLocked.push(exit.id);
                }
                exitCases.push({id:exit.id, open:true, locked, requires:exit.requires || []});
            }
        }
        coverage = {journeys, variants: Object.fromEntries(Object.entries(rooms).map(([id, values]) => [id, values.length])), actions: Object.keys(actions).length, unreachableLocked, exitCases};
        return coverage;
    }

    const frames = () => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    async function until(test) {
        const start = performance.now();
        while (!test()) { if (performance.now() - start > 15000) throw new Error('Fixture readiness timeout'); await frames(); }
    }
    function dismiss() {
        const root = document.querySelector('.agfish-app');
        for (let n = 0; n < 150 && !document.querySelector('.agfish-speech').hidden; n++)
            root.dispatchEvent(new KeyboardEvent('keydown', {key: ' ', code: 'Space', bubbles: true}));
    }
    async function open(state, profile) {
        for (const id of [...aurora.state.windows.keys()]) aurora.closeWindow(id);
        await until(() => !document.querySelector('.agfish-app'));
        const next = clone(state);
        next.settings.sound = false;
        next.seenLines = [...new Set(next.seenLines.concat(S.pendingResults(next).map(line => line.text), S.enter(next).map(line => line.text)))];
        const store = await GoldfishSave.open(), previous = await store.read('auto');
        await store.write('auto', S.validate(next), {expectedRevision: previous?.revision || 0}); store.close();
        fixtureTheme(profile.theme);
        await fixtureOpen('goldfish-adventure');
        await until(() => document.querySelector('.agfish-start .agfish-primary') && !document.querySelector('.agfish-art-loading'));
        const root = document.querySelector('.agfish-app'), win = root.closest('.vd-window');
        Object.assign(win.style, {left: '8px', top: '48px', width: profile.width + 'px', height: profile.height + 'px'});
        await frames(); await frames();
        return state.room;
    }
    async function ready() {
        await until(() => document.querySelector('.agfish-start').hidden && !document.querySelector('.agfish-busy, .agfish-in-moment, .agfish-art-loading'));
        dismiss(); await frames();
    }
    function geometry() {
        const root = document.querySelector('.agfish-app'), scene = document.querySelector('.agfish-scene');
        const canvas = document.querySelector('.agfish-canvas'), win = root.closest('.vd-window');
        const box = node => {const r = node.getBoundingClientRect(); return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};};
        return {window:box(win), root:box(root), scene:box(scene), canvas:box(canvas), backing:[canvas.width,canvas.height],
            clientWidth:canvas.clientWidth, dpr:devicePixelRatio, narrow:root.classList.contains('agfish-narrow')};
    }
    window.GoldfishHotspotFixture = {build, rooms, actions, open, ready, geometry, dismiss, frames,
        coverage: () => coverage, moments: () => Object.keys(manifest.moments)};
}());
