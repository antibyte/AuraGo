(function (root, factory) {
    const api = factory(root);
    if (typeof module === 'object' && module.exports) module.exports = api;
    else root.SynthStudioModel = api;
})(typeof globalThis !== 'undefined' ? globalThis : window, function (root) {
    'use strict';

    const PPQ = 480;
    const BAR = PPQ * 4;
    const MAX_TICK = 10_000_000;
    const MAX_BYTES = 5 * 1024 * 1024;
    const MAX_TRACKS = 16;
    const MAX_CLIPS_PER_TRACK = 256;
    const MAX_EVENTS = 50_000;
    const ID_PATTERN = /^[A-Za-z0-9_-]{1,100}$/;
    const trackKeys = ['id', 'name', 'instrument', 'volume', 'pan', 'mute', 'solo', 'params', 'clips'];
    const clipKeys = ['id', 'name', 'start', 'length', 'notes', 'controllers'];
    const noteKeys = ['id', 'pitch', 'start', 'duration', 'velocity'];
    const controllerKeys = ['tick', 'type', 'value'];
    const paramKeys = ['tone', 'attack', 'release', 'reverb', 'delay'];
    const presetKeys = ['drums-kit', 'gm-drums'];

    function error(code, message) {
        const result = new Error(message);
        result.code = code;
        return result;
    }

    function utf8ByteLength(value) {
        const Encoder = root.TextEncoder || (typeof TextEncoder === 'function' ? TextEncoder : null);
        if (Encoder) return new Encoder().encode(value).byteLength;
        let bytes = 0;
        for (const character of value) {
            const code = character.codePointAt(0);
            bytes += code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4;
        }
        return bytes;
    }

    function randomId(prefix) {
        const cryptoApi = root.crypto;
        const random = cryptoApi && typeof cryptoApi.randomUUID === 'function'
            ? cryptoApi.randomUUID()
            : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
        return `${prefix}-${random}`;
    }

    function plainObject(value, label, allowedKeys) {
        if (!value || typeof value !== 'object' || Array.isArray(value)) {
            throw error('E_PROJECT_TYPE', `${label} must be an object.`);
        }
        const proto = Object.getPrototypeOf(value);
        const constructor = proto && Object.getOwnPropertyDescriptor(proto, 'constructor');
        const isPlainPrototype = proto === null || (Object.getPrototypeOf(proto) === null && constructor && typeof constructor.value === 'function' && constructor.value.name === 'Object');
        if (!isPlainPrototype) {
            throw error('E_PROJECT_TYPE', `${label} must be a plain JSON object.`);
        }
        for (const key of Reflect.ownKeys(value)) {
            if (typeof key !== 'string' || !allowedKeys.includes(key)) {
                throw error('E_PROJECT_FIELD_UNKNOWN', `${label} contains unsupported field ${String(key)}.`);
            }
            const descriptor = Object.getOwnPropertyDescriptor(value, key);
            if (!descriptor || !Object.prototype.hasOwnProperty.call(descriptor, 'value')) {
                throw error('E_PROJECT_TYPE', `${label}.${key} must be a JSON value.`);
            }
        }
        return value;
    }

    function required(object, key, label) {
        if (!Object.prototype.hasOwnProperty.call(object, key)) {
            throw error('E_PROJECT_FIELD_MISSING', `${label}.${key} is required.`);
        }
        return object[key];
    }

    function text(value, label, max = 96) {
        if (typeof value !== 'string' || !value.trim() || value.length > max || /[\u0000-\u001f\u007f]/.test(value)) {
            throw error('E_PROJECT_TEXT', `${label} must be a non-empty string of at most ${max} characters.`);
        }
        return value.trim();
    }

    function id(value, label, ids) {
        if (typeof value !== 'string' || !ID_PATTERN.test(value)) {
            throw error('E_PROJECT_ID', `${label} must use only letters, numbers, underscores, and hyphens.`);
        }
        if (ids.has(value)) throw error('E_PROJECT_ID_DUPLICATE', `${label} duplicates ID ${value}.`);
        ids.add(value);
        return value;
    }

    function number(value, label, min, max, integer = false) {
        if (typeof value !== 'number' || !Number.isFinite(value) || value < min || value > max || (integer && !Number.isInteger(value))) {
            throw error(integer ? 'E_PROJECT_TICK' : 'E_PROJECT_NUMBER', `${label} must be ${integer ? 'an integer' : 'a finite number'} from ${min} to ${max}.`);
        }
        return value;
    }

    function bool(value, label) {
        if (typeof value !== 'boolean') throw error('E_PROJECT_TYPE', `${label} must be true or false.`);
        return value;
    }

    function list(value, label, max) {
        if (!Array.isArray(value) || value.length > max) {
            throw error('E_PROJECT_LIMIT', `${label} must be an array with at most ${max} items.`);
        }
        if (Reflect.ownKeys(value).length !== value.length + 1) {
            throw error('E_PROJECT_TYPE', `${label} must be a dense JSON array.`);
        }
        for (let index = 0; index < value.length; index++) {
            const descriptor = Object.getOwnPropertyDescriptor(value, String(index));
            if (!descriptor || !Object.prototype.hasOwnProperty.call(descriptor, 'value')) {
                throw error('E_PROJECT_TYPE', `${label} must be a dense JSON array.`);
            }
        }
        return value;
    }

    function instrumentIds() {
        const catalog = root.SynthStudioPresets;
        if (!catalog || !Array.isArray(catalog.presets) || !Array.isArray(catalog.gm)) return new Set();
        const known = new Set(presetKeys);
        for (const preset of catalog.presets.concat(catalog.gm)) {
            if (preset && typeof preset.id === 'string' && ID_PATTERN.test(preset.id)) known.add(preset.id);
        }
        return known;
    }

    function assertInstrument(value) {
        if (typeof value !== 'string' || !ID_PATTERN.test(value) || !instrumentIds().has(value)) {
            throw error('E_PROJECT_INSTRUMENT', `Unknown Synth Studio instrument: ${String(value)}.`);
        }
        return value;
    }

    function isDrumInstrument(value) {
        if (value === 'drums-kit' || value === 'gm-drums') return true;
        const catalog = root.SynthStudioPresets;
        const preset = catalog && typeof catalog.get === 'function'
            ? catalog.get(value)
            : catalog && Array.isArray(catalog.presets) ? catalog.presets.find(item => item && item.id === value) : null;
        return !!preset && preset.category === 'drums';
    }

    function normalizedClip(source, ids, totals) {
        const clip = plainObject(source, 'clip', clipKeys);
        const result = {
            id: id(required(clip, 'id', 'clip'), 'clip.id', ids),
            name: text(required(clip, 'name', 'clip'), 'clip.name'),
            start: number(required(clip, 'start', 'clip'), 'clip.start', 0, MAX_TICK, true),
            length: number(required(clip, 'length', 'clip'), 'clip.length', 1, MAX_TICK, true),
            notes: [],
            controllers: []
        };
        if (result.start + result.length > MAX_TICK) throw error('E_PROJECT_TICK', 'clip.start + clip.length exceeds the timeline limit.');

        const notes = list(required(clip, 'notes', 'clip'), 'clip.notes', MAX_EVENTS);
        totals.events += notes.length;
        if (totals.events > MAX_EVENTS) throw error('E_PROJECT_LIMIT', `A project may contain at most ${MAX_EVENTS} notes and controllers.`);
        result.notes = notes.map(sourceNote => {
            const note = plainObject(sourceNote, 'note', noteKeys);
            const normalized = {
                id: id(required(note, 'id', 'note'), 'note.id', ids),
                pitch: number(required(note, 'pitch', 'note'), 'note.pitch', 0, 127, true),
                start: number(required(note, 'start', 'note'), 'note.start', 0, MAX_TICK, true),
                duration: number(required(note, 'duration', 'note'), 'note.duration', 1, MAX_TICK, true),
                velocity: number(required(note, 'velocity', 'note'), 'note.velocity', 0, 1)
            };
            if (normalized.start + normalized.duration > MAX_TICK) throw error('E_PROJECT_TICK', 'note.start + note.duration exceeds the timeline limit.');
            if (normalized.start >= result.length || normalized.start + normalized.duration > result.length) {
                throw error('E_PROJECT_TICK', 'Notes must start and end within their clip.');
            }
            return normalized;
        });

        const controllers = list(required(clip, 'controllers', 'clip'), 'clip.controllers', MAX_EVENTS);
        totals.events += controllers.length;
        if (totals.events > MAX_EVENTS) throw error('E_PROJECT_LIMIT', `A project may contain at most ${MAX_EVENTS} notes and controllers.`);
        result.controllers = controllers.map(sourceController => {
            const controller = plainObject(sourceController, 'controller', controllerKeys);
            const type = required(controller, 'type', 'controller');
            if (!['sustain', 'bend', 'modulation'].includes(type)) {
                throw error('E_PROJECT_CONTROLLER', `Unsupported controller type: ${String(type)}.`);
            }
            const min = type === 'bend' ? -1 : 0;
            return {
                tick: number(required(controller, 'tick', 'controller'), 'controller.tick', 0, result.length - 1, true),
                type,
                value: number(required(controller, 'value', 'controller'), 'controller.value', min, 1)
            };
        });
        return result;
    }

    function validate(value) {
        let source = value;
        if (typeof source === 'string') {
            if (source.length > MAX_BYTES) throw error('E_PROJECT_SIZE', 'Project JSON exceeds the 5 MiB limit.');
            if (utf8ByteLength(source) > MAX_BYTES) throw error('E_PROJECT_SIZE', 'Project JSON exceeds the 5 MiB limit.');
            try { source = JSON.parse(source); }
            catch (_) { throw error('E_PROJECT_JSON', 'Project file is not valid JSON.'); }
        }
        const project = plainObject(source, 'project', ['version', 'name', 'tempo', 'ppq', 'beatsPerBar', 'loop', 'tracks']);
        if (required(project, 'version', 'project') !== 1) throw error('E_PROJECT_VERSION', 'Only Synth Studio project version 1 is supported.');
        const result = {
            version: 1,
            name: text(required(project, 'name', 'project'), 'project.name'),
            tempo: number(required(project, 'tempo', 'project'), 'project.tempo', 40, 240),
            ppq: number(required(project, 'ppq', 'project'), 'project.ppq', PPQ, PPQ, true),
            beatsPerBar: number(required(project, 'beatsPerBar', 'project'), 'project.beatsPerBar', 4, 4, true),
            loop: null,
            tracks: []
        };
        const loop = plainObject(required(project, 'loop', 'project'), 'project.loop', ['enabled', 'start', 'end']);
        result.loop = {
            enabled: bool(required(loop, 'enabled', 'project.loop'), 'project.loop.enabled'),
            start: number(required(loop, 'start', 'project.loop'), 'project.loop.start', 0, MAX_TICK, true),
            end: number(required(loop, 'end', 'project.loop'), 'project.loop.end', 0, MAX_TICK, true)
        };
        if (result.loop.enabled && result.loop.end <= result.loop.start) {
            throw error('E_PROJECT_LOOP', 'An enabled loop must end after it starts.');
        }
        const tracks = list(required(project, 'tracks', 'project'), 'project.tracks', MAX_TRACKS);
        const ids = new Set();
        const totals = { events: 0 };
        result.tracks = tracks.map(sourceTrack => {
            const track = plainObject(sourceTrack, 'track', trackKeys);
            const params = plainObject(required(track, 'params', 'track'), 'track.params', paramKeys);
            const clips = list(required(track, 'clips', 'track'), 'track.clips', MAX_CLIPS_PER_TRACK);
            return {
                id: id(required(track, 'id', 'track'), 'track.id', ids),
                name: text(required(track, 'name', 'track'), 'track.name'),
                instrument: assertInstrument(required(track, 'instrument', 'track')),
                volume: number(required(track, 'volume', 'track'), 'track.volume', 0, 1),
                pan: number(required(track, 'pan', 'track'), 'track.pan', -1, 1),
                mute: bool(required(track, 'mute', 'track'), 'track.mute'),
                solo: bool(required(track, 'solo', 'track'), 'track.solo'),
                params: {
                    tone: number(required(params, 'tone', 'track.params'), 'track.params.tone', 0, 1),
                    attack: number(required(params, 'attack', 'track.params'), 'track.params.attack', 0, 10),
                    release: number(required(params, 'release', 'track.params'), 'track.params.release', 0, 30),
                    reverb: number(required(params, 'reverb', 'track.params'), 'track.params.reverb', 0, 1),
                    delay: number(required(params, 'delay', 'track.params'), 'track.params.delay', 0, 1)
                },
                clips: clips.map(sourceClip => normalizedClip(sourceClip, ids, totals))
            };
        });
        const drumTracks = result.tracks.filter(track => isDrumInstrument(track.instrument)).length;
        if (drumTracks > 1 || result.tracks.length - drumTracks > 15) {
            throw error('E_PROJECT_LIMIT', 'A project supports up to 15 melodic tracks and one drum rack.');
        }
        const serialized = JSON.stringify(result);
        if (serialized.length > MAX_BYTES || utf8ByteLength(serialized) > MAX_BYTES) {
            throw error('E_PROJECT_SIZE', 'Project JSON exceeds the 5 MiB limit.');
        }
        return result;
    }

    function create(name = 'Untitled') {
        return {
            version: 1,
            name: text(name || 'Untitled', 'project.name'),
            tempo: 120,
            ppq: PPQ,
            beatsPerBar: 4,
            loop: { enabled: false, start: 0, end: BAR * 4 },
            tracks: []
        };
    }

    function createClip(instrument, start = 0) {
        assertInstrument(instrument);
        return { id: randomId('clip'), name: 'Clip', start: number(start, 'clip.start', 0, MAX_TICK - BAR, true), length: BAR, notes: [], controllers: [] };
    }

    function addTrack(project, instrument) {
        validate(project);
        assertInstrument(instrument);
        if (project.tracks.length >= MAX_TRACKS) throw error('E_PROJECT_LIMIT', `A project may contain at most ${MAX_TRACKS} tracks.`);
        const drums = project.tracks.filter(track => isDrumInstrument(track.instrument)).length;
        if (isDrumInstrument(instrument) ? drums >= 1 : project.tracks.length - drums >= 15) {
            throw error('E_PROJECT_LIMIT', 'A project supports up to 15 melodic tracks and one drum rack.');
        }
        const index = project.tracks.length + 1;
        const track = {
            id: randomId('track'),
            name: instrument === 'drums-kit' || instrument === 'gm-drums' ? `Drums ${index}` : `Track ${index}`,
            instrument,
            volume: 0.8,
            pan: 0,
            mute: false,
            solo: false,
            params: { tone: 0.5, attack: 0.01, release: 0.3, reverb: 0, delay: 0 },
            clips: []
        };
        project.tracks.push(track);
        return track;
    }

    function cloneClip(clip, start = clip && clip.start || 0) {
        const safe = normalizedClip(clip, new Set(), { events: 0 });
        safe.id = randomId('clip');
        safe.start = number(start, 'clip.start', 0, MAX_TICK - safe.length, true);
        for (const note of safe.notes) note.id = randomId('note');
        return safe;
    }

    function clone(project) {
        return validate(project);
    }

    function events(clip) {
        const safe = normalizedClip(clip, new Set(), { events: 0 });
        const output = [];
        for (const note of safe.notes) {
            output.push({ tick: note.start, type: 'noteOn', pitch: note.pitch, velocity: note.velocity, noteId: note.id });
            output.push({ tick: note.start + note.duration, type: 'noteOff', pitch: note.pitch, velocity: 0, noteId: note.id });
        }
        for (const controller of safe.controllers) output.push({ tick: controller.tick, type: 'controller', controller: controller.type, value: controller.value });
        const order = { noteOff: 0, controller: 1, noteOn: 2 };
        return output.sort((a, b) => a.tick - b.tick || order[a.type] - order[b.type]);
    }

    function endTick(clip) {
        const safe = normalizedClip(clip, new Set(), { events: 0 });
        return safe.start + Math.max(safe.length, ...safe.notes.map(note => note.start + note.duration), ...safe.controllers.map(event => event.tick));
    }

    function presetFor(category, index) {
        const catalog = root.SynthStudioPresets;
        const presets = catalog && Array.isArray(catalog.presets) ? catalog.presets.filter(item => item && item.category === category) : [];
        return presets[index] || (catalog && Array.isArray(catalog.gm) ? catalog.gm[0] : null) || { id: 'gm-0' };
    }

    function makePattern(id, name, category, instrument, notes, length = BAR) {
        return { id, name, category, instrument, clip: { id: `pattern-${id}`, name, start: 0, length, notes: notes.map((note, i) => ({ id: `${id}-n${i + 1}`, pitch: note[0], start: note[1], duration: note[2], velocity: note[3] })), controllers: [] } };
    }

    const patterns = (() => {
        const drumA = [
            [36, 0, 120, 0.92], [42, 0, 60, 0.42], [38, 480, 120, 0.82], [42, 480, 60, 0.42],
            [36, 960, 120, 0.88], [42, 960, 60, 0.42], [38, 1440, 120, 0.82], [42, 1440, 60, 0.42]
        ];
        const drumB = [
            [36, 0, 120, 0.9], [46, 240, 60, 0.5], [38, 480, 120, 0.82], [42, 720, 60, 0.45],
            [36, 960, 120, 0.9], [46, 1200, 60, 0.5], [38, 1440, 120, 0.82], [42, 1680, 60, 0.45]
        ];
        const specs = [
            makePattern('drums-rock', 'Rock beat', 'drums', 'drums-kit', drumA),
            makePattern('drums-offbeat', 'Offbeat groove', 'drums', 'drums-kit', drumB),
            makePattern('bass-octaves', 'Octave bass', 'bass', presetFor('bass', 0).id, [[36, 0, 300, .82], [43, 480, 300, .62], [36, 960, 300, .82], [43, 1440, 300, .62]]),
            makePattern('bass-walk', 'Walking bass', 'bass', presetFor('bass', 1).id, [[36, 0, 180, .78], [38, 240, 180, .64], [41, 480, 180, .7], [43, 720, 180, .65], [36, 960, 180, .78], [43, 1200, 180, .65], [41, 1440, 180, .7], [38, 1680, 180, .64]]),
            makePattern('leads-steps', 'Stepped melody', 'leads', presetFor('leads', 0).id, [[72, 0, 220, .75], [76, 240, 220, .68], [79, 480, 420, .8], [76, 960, 220, .72], [74, 1200, 220, .66], [72, 1440, 420, .8]]),
            makePattern('leads-echo', 'Echo melody', 'leads', presetFor('leads', 1).id, [[67, 0, 180, .72], [74, 360, 180, .62], [79, 720, 300, .78], [74, 1200, 180, .62], [67, 1560, 180, .7]]),
            makePattern('pads-open', 'Open chord', 'pads', presetFor('pads', 0).id, [[60, 0, BAR, .54], [64, 0, BAR, .46], [67, 0, BAR, .5], [72, 0, BAR, .42]]),
            makePattern('pads-rise', 'Rising chord', 'pads', presetFor('pads', 1).id, [[57, 0, BAR, .5], [60, 0, BAR, .44], [64, 0, BAR, .48], [69, 0, BAR, .4]]),
            makePattern('keys-chords', 'Chord rhythm', 'keys', presetFor('keys', 0).id, [[60, 0, 420, .76], [64, 0, 420, .66], [67, 0, 420, .7], [62, 960, 420, .72], [65, 960, 420, .64], [69, 960, 420, .68]]),
            makePattern('keys-arpeggio', 'Arpeggio', 'keys', presetFor('keys', 1).id, [[60, 0, 180, .72], [64, 240, 180, .62], [67, 480, 180, .68], [72, 720, 180, .76], [60, 960, 180, .72], [64, 1200, 180, .62], [67, 1440, 180, .68], [72, 1680, 180, .76]])
        ];
        return specs;
    })();

    function demo() {
        const project = create('Neon Sketch');
        project.tempo = 112;
        for (const pattern of [patterns[0], patterns[2], patterns[4], patterns[6], patterns[8]]) {
            const track = addTrack(project, pattern.instrument);
            track.name = pattern.name;
            for (let bar = 0; bar < 4; bar++) track.clips.push(cloneClip(pattern.clip, bar * BAR));
        }
        return validate(project);
    }

    function history(initial, limit = 100) {
        const cap = number(limit, 'history.limit', 2, 500, true);
        const snapshots = [validate(initial)];
        let index = 0;
        return {
            current() { return clone(snapshots[index]); },
            commit(project) {
                const next = validate(project);
                if (JSON.stringify(next) === JSON.stringify(snapshots[index])) return clone(snapshots[index]);
                snapshots.splice(index + 1);
                snapshots.push(next);
                index = snapshots.length - 1;
                if (snapshots.length > cap) {
                    snapshots.shift();
                    index--;
                }
                return clone(snapshots[index]);
            },
            undo() { if (index === 0) return null; index--; return clone(snapshots[index]); },
            redo() { if (index >= snapshots.length - 1) return null; index++; return clone(snapshots[index]); },
            canUndo() { return index > 0; },
            canRedo() { return index < snapshots.length - 1; }
        };
    }

    return { PPQ, BAR, newID: randomId, create, validate, addTrack, createClip, patterns, demo, events, endTick, clone, cloneClip, history };
});
