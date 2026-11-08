(function () {
    'use strict';

    const MAX_BYTES = 128 * 1024;
    const SCHEMA = 1;
    // Flags that a content change retired. A save that still carries one loads without it.
    const LEGACY_FLAGS = new Set(['e_scrap_refusal']);
    const copy = value => JSON.parse(JSON.stringify(value));
    const unique = values => [...new Set(values)];
    const fail = code => { const error = new Error(code); error.code = code; throw error; };
    const content = () => window.GoldfishContent;
    const room = id => content().rooms.find(value => value.id === id);
    const action = id => content().actions.find(value => value.id === id);
    const size = value => new TextEncoder().encode(JSON.stringify(value)).byteLength;

    function conditions(state, record) {
        return (record.requires || []).every(id => state.flags.includes(id)) &&
            (!(record.requiresAny || []).length || record.requiresAny.some(id => state.flags.includes(id))) &&
            (record.forbids || []).every(id => !state.flags.includes(id));
    }

    function create() {
        const first = room(content().start);
        return {
            schema: SCHEMA, contentVersion: content().version, room: first.id,
            flags: [], inventory: [], completed: [], applied: [], routes: {}, protocols: [], ending: null,
            seenLines: [], journal: [], visited: [first.id],
            position: { x: first.spawn?.[0] ?? 0.24, y: first.spawn?.[1] ?? 0.83 },
            settings: { sound: false, reducedMotion: false, hotspots: false }, elapsedMs: 0
        };
    }

    function available(state, candidate) {
        const record = typeof candidate === 'string' ? action(candidate) : candidate;
        if (!record || record.room !== state.room || !conditions(state, record)) return false;
        if (state.ending && !record.repeatable) return false;
        if (record.item && !state.inventory.includes(record.item)) return false;
        if (record.effects?.route && state.routes[record.effects.route.key] &&
            state.routes[record.effects.route.key] !== record.effects.route.value) return false;
        if (record.effects?.ending && (!state.protocols.includes(record.effects.ending) || state.ending)) return false;
        if (state.applied.includes(record.id) && !record.repeatable) return false;
        return true;
    }

    function lines(state, records) {
        return (records || []).filter(record => conditions(state, record)).map(record => copy(record));
    }

    function pendingResults(state) {
        const seen = new Set(state.seenLines), pending = [];
        const atAction = { flags: [] };
        for (const id of state.applied) {
            const record = action(id);
            atAction.flags = unique(atAction.flags.concat(record.effects?.flags || []));
            for (const line of lines(atAction, record.lines)) {
                if (!seen.has(line.text)) { pending.push(line); seen.add(line.text); }
            }
        }
        return pending;
    }

    function correct(challenge, answer) {
        if (!challenge) return true;
        if (Array.isArray(challenge.answer)) {
            return Array.isArray(answer) && answer.length === challenge.answer.length &&
                answer.every((value, index) => value === challenge.answer[index]);
        }
        return typeof answer === 'string' && answer === challenge.answer;
    }

    function effects(state, record) {
        const next = copy(state), effect = record.effects || {};
        next.flags = unique(next.flags.concat(effect.flags || []));
        next.inventory = unique(next.inventory.filter(id => !(effect.removeItems || []).includes(id)).concat(effect.items || []));
        if (effect.puzzle) next.completed = unique(next.completed.concat(effect.puzzle));
        if (effect.route) next.routes[effect.route.key] = effect.route.value;
        if (effect.protocol) next.protocols = unique(next.protocols.concat(effect.protocol));
        if (effect.ending) next.ending = effect.ending;
        next.applied.push(record.id);
        next.journal = unique(next.journal.concat(record.journal || []));
        return next;
    }

    function apply(state, id, input = {}) {
        const record = action(id);
        if (!available(state, record)) return { state, changed: false, lines: [], journal: [], error: 'unavailable' };
        if (record.item && record.item !== input.item) return { state, changed: false, lines: [], journal: [], error: 'item_needed' };
        if (!correct(record.challenge, input.answer)) {
            return { state, changed: false, lines: lines(state, record.challenge?.wrongLines), journal: [], error: 'wrong' };
        }
        if (state.applied.includes(id)) return { state, changed: false, lines: lines(state, record.lines), journal: [] };
        const next = effects(state, record);
        return { state: next, changed: true, lines: lines(next, record.lines), journal: record.journal || [] };
    }

    function move(state, target) {
        const exit = room(state.room).exits.find(value => value.to === target);
        if (!exit || !conditions(state, exit)) {
            return { state, changed: false, lines: lines(state, exit?.blocked), error: 'unavailable' };
        }
        const next = copy(state), destination = room(target);
        next.room = target;
        next.visited = unique(next.visited.concat(target));
        next.position = { x: destination.spawn?.[0] ?? 0.24, y: destination.spawn?.[1] ?? 0.83 };
        return { state: next, changed: true, lines: lines(next, destination.entry) };
    }

    function reachable(state, target) {
        const pending = [state.room], visited = new Set(pending);
        while (pending.length) {
            const current = pending.shift();
            if (current === target) return true;
            for (const exit of room(current).exits) {
                if (room(exit.to) && conditions(state, exit) && !visited.has(exit.to)) {
                    visited.add(exit.to);
                    pending.push(exit.to);
                }
            }
        }
        return false;
    }

    function inspectShape(value, depth = 0) {
        if (depth > 8) fail('invalid');
        if (value === null || typeof value === 'boolean') return;
        if (typeof value === 'number') { if (!Number.isFinite(value)) fail('invalid'); return; }
        if (typeof value === 'string') { if (value.length > 4096) fail('invalid'); return; }
        if (typeof value !== 'object') fail('invalid');
        if (Array.isArray(value)) {
            if (value.length > 2048) fail('invalid');
        } else {
            if (Object.prototype.toString.call(value) !== '[object Object]' || Object.keys(value).length > 32) fail('invalid');
            if (Object.keys(value).some(key => ['__proto__', 'prototype', 'constructor'].includes(key))) fail('invalid');
        }
        for (const entry of Object.values(value)) inspectShape(entry, depth + 1);
    }

    function stringArray(value, allowed, limit = 2048) {
        if (!Array.isArray(value) || value.length > limit || unique(value).length !== value.length ||
            value.some(id => typeof id !== 'string' || !allowed.has(id))) fail('invalid');
    }

    function validate(raw) {
        inspectShape(raw);
        if (size(raw) > MAX_BYTES) fail('oversized');
        if (!raw || raw.schema !== SCHEMA || raw.contentVersion !== content().version) fail('version');
        const initial = create();
        if (Object.keys(raw).some(key => !Object.hasOwn(initial, key)) ||
            Object.keys(initial).some(key => !Object.hasOwn(raw, key))) fail('invalid');
        const rooms = new Set(content().rooms.map(value => value.id));
        const actions = new Set(content().actions.map(value => value.id));
        const textKeys = new Set(Object.keys(content().texts || {}));
        for (const value of content().rooms) {
            for (const line of value.entry || []) textKeys.add(line.text);
            for (const hotspot of value.hotspots || []) for (const line of hotspot.look || []) textKeys.add(line.text);
        }
        for (const value of content().actions) {
            for (const line of (value.lines || []).concat(value.challenge?.wrongLines || [])) textKeys.add(line.text);
        }
        if (!rooms.has(raw.room)) fail('invalid');
        stringArray(raw.applied, actions, content().actions.length);
        stringArray(raw.visited, rooms, content().rooms.length);
        stringArray(raw.seenLines, textKeys);
        if (!raw.visited.includes(content().start) || !raw.visited.includes(raw.room)) fail('invalid');
        if (!raw.position || Object.keys(raw.position).sort().join() !== 'x,y' ||
            ![raw.position.x, raw.position.y].every(value => typeof value === 'number' && value >= 0 && value <= 1)) fail('invalid');
        if (!raw.settings || Object.keys(raw.settings).sort().join() !== 'hotspots,reducedMotion,sound' ||
            Object.values(raw.settings).some(value => typeof value !== 'boolean')) fail('invalid');
        if (!raw.routes || Array.isArray(raw.routes) || typeof raw.routes !== 'object' ||
            Object.keys(raw.routes).some(key => !['customs', 'garden', 'hall'].includes(key))) fail('invalid');
        if (!Number.isSafeInteger(raw.elapsedMs) || raw.elapsedMs < 0 || raw.elapsedMs > 365 * 86400000) fail('invalid');
        // Replay the compact witness: room travel must be possible before every rewarded action.
        let replay = initial;
        const witnessed = new Set([initial.room]);
        for (const id of raw.applied) {
            const record = action(id);
            for (const candidate of rooms) if (reachable(replay, candidate)) witnessed.add(candidate);
            if (!reachable(replay, record.room)) fail('impossible');
            replay.room = record.room;
            witnessed.add(record.room);
            if (!available(replay, record)) fail('impossible');
            replay = effects(replay, record);
        }
        if (!reachable(replay, raw.room)) fail('impossible');
        for (const visited of raw.visited) if (!reachable(replay, visited) && !witnessed.has(visited)) fail('impossible');
        const flags = Array.isArray(raw.flags) ? raw.flags.filter(id => !LEGACY_FLAGS.has(id)) : raw.flags;
        for (const field of ['flags', 'inventory', 'completed', 'journal', 'protocols']) {
            const values = field === 'flags' ? flags : raw[field];
            stringArray(values, new Set(replay[field]));
            if (values.length !== replay[field].length) fail('impossible');
        }
        if (JSON.stringify(Object.entries(raw.routes).sort()) !== JSON.stringify(Object.entries(replay.routes).sort()) ||
            raw.ending !== replay.ending) fail('impossible');
        return copy(Array.isArray(raw.flags) ? { ...raw, flags } : raw);
    }

    window.GoldfishState = Object.freeze({
        create, validate, available, apply, move, conditions, reachable, lines, pendingResults,
        enter: state => lines(state, room(state.room).entry),
        MAX_BYTES, SCHEMA
    });
}());
