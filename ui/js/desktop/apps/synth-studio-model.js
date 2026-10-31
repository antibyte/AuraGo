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
    const paramKeysV2 = ['tone', 'resonance', 'drive', 'attack', 'release', 'chorus', 'phaser', 'reverb', 'delay'];
    const paramKeysV3 = ['tone', 'resonance', 'drive', 'attack', 'release', 'reverb', 'delay'];
    const presetKeys = ['drums-kit', 'gm-drums'];
    const automationParametersV2 = ['tone', 'reverb'];
    const automationParametersV3 = ['tone', 'resonance', 'drive', 'reverb', 'delay'];
    const masterKeys = ['volume', 'low', 'mid', 'high', 'threshold', 'ratio', 'knee', 'attack', 'release', 'width', 'plateSize', 'plateDecay', 'echoTime', 'echoFeedback', 'echoSync'];
    const MAX_INSERTS = 4;
    const ECHO_DIVISIONS = ['off', '1/4', '1/8', '1/8T', '1/16', '1/16T'];

    // The effect rack schema. The audio implementation lives in
    // synth-studio-fx.js; this table is the single source of truth for
    // validation, for the default chain and for the rack UI.
    const FX_TYPES = [
        { id: 'chorus', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.3 },
            { key: 'rate', min: 0.05, max: 8, step: 0.01, def: 0.7 },
            { key: 'depth', min: 0, max: 1, step: 0.01, def: 0.35 },
            { key: 'feedback', min: 0, max: 0.9, step: 0.01, def: 0 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.3 }
        ] },
        { id: 'phaser', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.3 },
            { key: 'rate', min: 0.05, max: 8, step: 0.01, def: 0.42 },
            { key: 'depth', min: 0, max: 1, step: 0.01, def: 0.5 },
            { key: 'feedback', min: 0, max: 0.9, step: 0.01, def: 0.35 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.35 }
        ] },
        { id: 'tremolo', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.4 },
            { key: 'rate', min: 0.2, max: 20, step: 0.05, def: 4.5 },
            { key: 'depth', min: 0, max: 1, step: 0.01, def: 0.6 }
        ] },
        { id: 'flanger', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.3 },
            { key: 'rate', min: 0.02, max: 8, step: 0.01, def: 0.3 },
            { key: 'depth', min: 0, max: 1, step: 0.01, def: 0.4 },
            { key: 'feedback', min: 0, max: 0.95, step: 0.01, def: 0.4 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.5 }
        ] },
        { id: 'rotary', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.4 },
            { key: 'rate', min: 0.2, max: 12, step: 0.05, def: 1.6 },
            { key: 'depth', min: 0, max: 1, step: 0.01, def: 0.55 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.45 }
        ] },
        { id: 'bitcrush', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.5 },
            { key: 'bits', min: 2, max: 16, step: 1, def: 6 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.35 }
        ] },
        { id: 'softclip', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.6 },
            { key: 'drive', min: 0, max: 1, step: 0.01, def: 0.35 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.6 }
        ] },
        { id: 'eq3', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 1 },
            { key: 'low', min: -18, max: 18, step: 0.5, def: 0 },
            { key: 'mid', min: -18, max: 18, step: 0.5, def: 0 },
            { key: 'high', min: -18, max: 18, step: 0.5, def: 0 },
            { key: 'freq', min: 200, max: 4000, step: 10, def: 1000 }
        ] },
        { id: 'width', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 1 },
            { key: 'amount', min: 0, max: 2, step: 0.01, def: 1 }
        ] },
        { id: 'cabinet', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.6 },
            { key: 'model', min: 0, max: 3, step: 1, def: 0 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.5 }
        ] },
        { id: 'delay', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.3 },
            { key: 'time', min: 0.01, max: 1.5, step: 0.005, def: 0.24 },
            { key: 'feedback', min: 0, max: 0.9, step: 0.01, def: 0.25 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.45 }
        ] },
        { id: 'reverb', params: [
            { key: 'mix', min: 0, max: 1, step: 0.01, def: 0.25 },
            { key: 'size', min: 0, max: 1, step: 0.01, def: 0.5 },
            { key: 'decay', min: 0, max: 1, step: 0.01, def: 0.6 },
            { key: 'tone', min: 0, max: 1, step: 0.01, def: 0.5 }
        ] }
    ];
    const FX_BY_ID = new Map(FX_TYPES.map(type => [type.id, type]));
    const MASTER_DEFAULTS = {
        volume: 0.78, low: 0, mid: 0, high: 0, threshold: -8, ratio: 10,
        knee: 8, attack: 0.003, release: 0.18, width: 1,
        plateSize: 0.5, plateDecay: 0.65, echoTime: 0.375, echoFeedback: 0.28, echoSync: '1/8'
    };

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

    function optionalParam(params, key, min, max, fallback = 0) {
        if (!Object.prototype.hasOwnProperty.call(params, key)) return fallback;
        return number(params[key], `track.params.${key}`, min, max);
    }

    function insertSpec(type, overrides = {}) {
        const definition = FX_BY_ID.get(type);
        if (!definition) throw error('E_PROJECT_FX', `Unsupported effect insert: ${String(type)}.`);
        const spec = { type: definition.id };
        for (const param of definition.params) {
            const value = Object.prototype.hasOwnProperty.call(overrides, param.key) ? overrides[param.key] : param.def;
            spec[param.key] = number(value, `track.fx.insert.${param.key}`, param.min, param.max);
        }
        return spec;
    }

    function normalizedInsert(source) {
        if (!source || typeof source !== 'object' || Array.isArray(source)) throw error('E_PROJECT_TYPE', 'An effect insert must be an object.');
        const definition = typeof source.type === 'string' ? FX_BY_ID.get(source.type) : null;
        if (!definition) throw error('E_PROJECT_FX', `Unsupported effect insert: ${String(source.type)}.`);
        const insert = plainObject(source, 'track.fx.insert', ['type', ...definition.params.map(param => param.key)]);
        const result = { type: definition.id };
        for (const param of definition.params) {
            result[param.key] = number(required(insert, param.key, 'track.fx.insert'), `track.fx.insert.${param.key}`, param.min, param.max);
        }
        return result;
    }

    function normalizedFx(source) {
        const fx = plainObject(source, 'track.fx', ['inserts']);
        const inserts = list(required(fx, 'inserts', 'track.fx'), 'track.fx.inserts', MAX_INSERTS);
        return { inserts: inserts.map(normalizedInsert) };
    }

    function normalizedMaster(source) {
        const master = plainObject(source, 'project.master', masterKeys);
        const echoSync = required(master, 'echoSync', 'project.master');
        if (typeof echoSync !== 'string' || !ECHO_DIVISIONS.includes(echoSync)) throw error('E_PROJECT_TYPE', 'Unsupported echo sync division.');
        return {
            volume: number(required(master, 'volume', 'project.master'), 'project.master.volume', 0, 1.5),
            low: number(required(master, 'low', 'project.master'), 'project.master.low', -18, 18),
            mid: number(required(master, 'mid', 'project.master'), 'project.master.mid', -18, 18),
            high: number(required(master, 'high', 'project.master'), 'project.master.high', -18, 18),
            threshold: number(required(master, 'threshold', 'project.master'), 'project.master.threshold', -60, 0),
            ratio: number(required(master, 'ratio', 'project.master'), 'project.master.ratio', 1, 20),
            knee: number(required(master, 'knee', 'project.master'), 'project.master.knee', 0, 40),
            attack: number(required(master, 'attack', 'project.master'), 'project.master.attack', 0, 1),
            release: number(required(master, 'release', 'project.master'), 'project.master.release', 0, 1),
            width: number(required(master, 'width', 'project.master'), 'project.master.width', 0, 2),
            plateSize: number(required(master, 'plateSize', 'project.master'), 'project.master.plateSize', 0, 1),
            plateDecay: number(required(master, 'plateDecay', 'project.master'), 'project.master.plateDecay', 0, 1),
            echoTime: number(required(master, 'echoTime', 'project.master'), 'project.master.echoTime', 0.01, 2),
            echoFeedback: number(required(master, 'echoFeedback', 'project.master'), 'project.master.echoFeedback', 0, 0.85),
            echoSync
        };
    }

    function presetPatchFor(instrument) {
        const catalog = root.SynthStudioPresets;
        const preset = catalog && typeof catalog.get === 'function' ? catalog.get(instrument) : null;
        return (preset && preset.patch) || {};
    }

    function defaultParamsFor(instrument) {
        const fx = presetPatchFor(instrument).fx || {};
        return {
            tone: typeof fx.tone === 'number' ? fx.tone : 0.5,
            resonance: typeof fx.resonance === 'number' ? fx.resonance : 0,
            drive: typeof fx.drive === 'number' ? fx.drive : 0,
            attack: typeof fx.attack === 'number' ? fx.attack : 0.01,
            release: typeof fx.release === 'number' ? fx.release : 0.3,
            reverb: typeof fx.reverb === 'number' ? fx.reverb : 0,
            delay: typeof fx.delay === 'number' ? fx.delay : 0
        };
    }

    // A preset may ship a starter insert chain. The legacy `chorus`/`phaser`
    // mixer values become the first inserts of the chain.
    function defaultInsertsFor(instrument) {
        const fx = presetPatchFor(instrument).fx || {};
        const inserts = [];
        if (Array.isArray(fx.inserts)) {
            for (const entry of fx.inserts) {
                if (!entry || typeof entry.type !== 'string') continue;
                const { type, ...overrides } = entry;
                inserts.push(insertSpec(type, overrides));
                if (inserts.length >= MAX_INSERTS) break;
            }
        }
        if (typeof fx.chorus === 'number' && fx.chorus > 0) inserts.push(insertSpec('chorus', { mix: fx.chorus }));
        if (typeof fx.phaser === 'number' && fx.phaser > 0) inserts.push(insertSpec('phaser', { mix: fx.phaser }));
        return inserts.slice(0, MAX_INSERTS);
    }

    function validate(value) {
        let source = value;
        if (typeof source === 'string') {
            if (source.length > MAX_BYTES) throw error('E_PROJECT_SIZE', 'Project JSON exceeds the 5 MiB limit.');
            if (utf8ByteLength(source) > MAX_BYTES) throw error('E_PROJECT_SIZE', 'Project JSON exceeds the 5 MiB limit.');
            try { source = JSON.parse(source); }
            catch (_) { throw error('E_PROJECT_JSON', 'Project file is not valid JSON.'); }
        }
        const project = plainObject(source, 'project', ['version', 'name', 'tempo', 'ppq', 'beatsPerBar', 'loop', 'tracks', 'harmony', 'scenes', 'master']);
        const version = required(project, 'version', 'project');
        if (![1, 2, 3].includes(version)) throw error('E_PROJECT_VERSION', 'Only Synth Studio project versions 1, 2 and 3 are supported.');
        if (version === 1 && ('harmony' in project || 'scenes' in project)) throw error('E_PROJECT_FIELD_UNKNOWN', 'Version 1 does not support harmony or scenes.');
        if (version < 3 && 'master' in project) throw error('E_PROJECT_FIELD_UNKNOWN', 'The master strip requires project version 3.');
        const result = {
            version,
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
        if (version >= 2) {
            const h = plainObject(required(project, 'harmony', 'project'), 'harmony', ['root','scale','snap']);
            if (!['major','minor'].includes(h.scale)) throw error('E_PROJECT_TYPE', 'Invalid scale.');
            result.harmony = {root:number(h.root,'harmony.root',0,11,true),scale:h.scale,snap:bool(h.snap,'harmony.snap')};
            result.scenes = list(required(project,'scenes','project'),'scenes',8).map(s => {
                plainObject(s,'scene',['id','name','start','end']);
                const start=number(s.start,'scene.start',0,MAX_TICK,true),end=number(s.end,'scene.end',1,MAX_TICK,true);
                if (start % BAR || end % BAR || end <= start) throw error('E_PROJECT_TICK','Scenes require complete bars.');
                return {id:id(s.id,'scene.id',ids),name:text(s.name,'scene.name'),start,end};
            });
        }
        if (version >= 3) result.master = normalizedMaster(required(project, 'master', 'project'));
        result.tracks = tracks.map(sourceTrack => {
            const trackKeysV = version >= 3 ? [...trackKeys,'automation','fx'] : version === 2 ? [...trackKeys,'automation'] : trackKeys;
            const track = plainObject(sourceTrack, 'track', trackKeysV);
            const params = plainObject(required(track, 'params', 'track'), 'track.params', version >= 3 ? paramKeysV3 : paramKeysV2);
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
                    resonance: optionalParam(params, 'resonance', 0, 1, 0),
                    drive: optionalParam(params, 'drive', 0, 1, 0),
                    attack: number(required(params, 'attack', 'track.params'), 'track.params.attack', 0, 10),
                    release: number(required(params, 'release', 'track.params'), 'track.params.release', 0, 30),
                    reverb: number(required(params, 'reverb', 'track.params'), 'track.params.reverb', 0, 1),
                    delay: number(required(params, 'delay', 'track.params'), 'track.params.delay', 0, 1),
                    ...(version < 3 ? {
                        chorus: optionalParam(params, 'chorus', 0, 1, 0),
                        phaser: optionalParam(params, 'phaser', 0, 1, 0)
                    } : {})
                },
                clips: clips.map(sourceClip => normalizedClip(sourceClip, ids, totals)),
                ...(version >= 2 ? {automation: normalizedAutomation(required(track,'automation','track'),totals,version)} : {}),
                ...(version >= 3 ? {fx: normalizedFx(required(track,'fx','track'))} : {})
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

    function normalizedAutomation(points, totals, version) {
        const allowed = version >= 3 ? automationParametersV3 : automationParametersV2;
        const safe=list(points,'track.automation',MAX_EVENTS), seen=new Set();
        totals.events+=safe.length;
        if(totals.events>MAX_EVENTS)throw error('E_PROJECT_LIMIT','Notes, controllers and automation exceed the event budget.');
        return safe.map(p=>{
            plainObject(p,'automation',['tick','parameter','value']);
            if(!allowed.includes(p.parameter))throw error('E_PROJECT_TYPE','Unsupported automation parameter.');
            const tick=number(p.tick,'automation.tick',0,MAX_TICK,true),key=tick+':'+p.parameter;
            if(seen.has(key))throw error('E_PROJECT_TYPE','Duplicate automation point.');seen.add(key);
            return {tick,parameter:p.parameter,value:number(p.value,'automation.value',0,1)};
        }).sort((a,b)=>a.tick-b.tick||a.parameter.localeCompare(b.parameter));
    }

    function upgrade(project) {
        let p=validate(project);
        if(p.version===1){p.version=2;p.harmony={root:0,scale:'major',snap:false};p.scenes=[];p.tracks.forEach(t=>t.automation=[]);}
        if(p.version===2){
            const tracks=p.tracks.map(t=>{
                const params={...t.params};
                const inserts=[];
                if(params.chorus>0)inserts.push(insertSpec('chorus',{mix:params.chorus}));
                if(params.phaser>0)inserts.push(insertSpec('phaser',{mix:params.phaser}));
                delete params.chorus;delete params.phaser;
                return {...t,params,fx:{inserts}};
            });
            p.version=3;p.master={...MASTER_DEFAULTS};p.tracks=tracks;
        }
        return validate(p);
    }

    function create(name = 'Untitled') {
        return {
            version: 3,
            harmony: {root:0,scale:'major',snap:false},
            scenes: [],
            master: {...MASTER_DEFAULTS},
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
            params: defaultParamsFor(instrument),
            ...(project.version >= 2 ? {automation:[]} : {}),
            ...(project.version >= 3 ? {fx:{inserts:defaultInsertsFor(instrument)}} : {}),
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

    // Prefer a named instrument so the showcase keeps its character when the
    // library grows, but stay usable with a reduced catalogue.
    function instrumentFor(id, category, index) {
        const catalog = root.SynthStudioPresets;
        const known = catalog && typeof catalog.get === 'function' ? catalog.get(id) : null;
        return known ? id : presetFor(category, index).id;
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
        const drumHats = [
            [42, 0, 60, 0.44], [42, 240, 60, 0.5], [42, 480, 60, 0.44], [42, 720, 60, 0.5],
            [42, 960, 60, 0.44], [42, 1200, 60, 0.5], [42, 1440, 60, 0.44], [42, 1680, 60, 0.52]
        ];
        const drumFill = [
            [38, 0, 60, 0.58], [38, 240, 60, 0.64], [38, 480, 60, 0.7], [38, 720, 60, 0.76],
            [38, 960, 60, 0.82], [38, 1140, 60, 0.88], [38, 1200, 60, 0.92], [47, 1680, 120, 0.9], [51, 1740, 180, 0.82]
        ];
        const specs = [
            makePattern('drums-rock', 'Rock beat', 'drums', 'drums-kit', drumA),
            makePattern('drums-offbeat', 'Offbeat groove', 'drums', 'drums-kit', drumB),
            makePattern('drums-hats', 'Hat pattern', 'drums', 'drums-kit', drumHats),
            makePattern('drums-fill', 'Snare fill', 'drums', 'drums-kit', drumFill),
            makePattern('bass-octaves', 'Octave bass', 'bass', presetFor('bass', 0).id, [[36, 0, 300, .82], [43, 480, 300, .62], [36, 960, 300, .82], [43, 1440, 300, .62]]),
            makePattern('bass-walk', 'Walking bass', 'bass', presetFor('bass', 1).id, [[36, 0, 180, .78], [38, 240, 180, .64], [41, 480, 180, .7], [43, 720, 180, .65], [36, 960, 180, .78], [43, 1200, 180, .65], [41, 1440, 180, .7], [38, 1680, 180, .64]]),
            makePattern('bass-pulse', 'Pulsing bass', 'bass', presetFor('bass', 0).id, [[36, 0, 150, .7], [36, 240, 150, .5], [36, 480, 150, .66], [36, 720, 150, .5], [36, 960, 150, .7], [36, 1200, 150, .5], [36, 1440, 150, .66], [36, 1680, 150, .5]]),
            makePattern('leads-steps', 'Stepped melody', 'leads', presetFor('leads', 0).id, [[72, 0, 220, .75], [76, 240, 220, .68], [79, 480, 420, .8], [76, 960, 220, .72], [74, 1200, 220, .66], [72, 1440, 420, .8]]),
            makePattern('leads-echo', 'Echo melody', 'leads', presetFor('leads', 1).id, [[67, 0, 180, .72], [74, 360, 180, .62], [79, 720, 300, .78], [74, 1200, 180, .62], [67, 1560, 180, .7]]),
            makePattern('lead-rise', 'Rising line', 'leads', presetFor('leads', 11).id, [[72, 0, 200, .58], [74, 240, 200, .64], [76, 480, 200, .68], [79, 720, 200, .72], [81, 960, 200, .76], [83, 1200, 200, .8], [84, 1440, 320, .84]]),
            makePattern('pads-open', 'Open chord', 'pads', presetFor('pads', 0).id, [[60, 0, BAR, .54], [64, 0, BAR, .46], [67, 0, BAR, .5], [72, 0, BAR, .42]]),
            makePattern('pads-rise', 'Rising chord', 'pads', presetFor('pads', 1).id, [[57, 0, BAR, .5], [60, 0, BAR, .44], [64, 0, BAR, .48], [69, 0, BAR, .4]]),
            makePattern('keys-chords', 'Chord rhythm', 'keys', presetFor('keys', 0).id, [[60, 0, 420, .76], [64, 0, 420, .66], [67, 0, 420, .7], [62, 960, 420, .72], [65, 960, 420, .64], [69, 960, 420, .68]]),
            makePattern('keys-arpeggio', 'Arpeggio', 'keys', presetFor('keys', 1).id, [[60, 0, 180, .72], [64, 240, 180, .62], [67, 480, 180, .68], [72, 720, 180, .76], [60, 960, 180, .72], [64, 1200, 180, .62], [67, 1440, 180, .68], [72, 1680, 180, .76]]),
            makePattern('keys-arp16', 'Fast arpeggio', 'keys', presetFor('keys', 1).id, [[60, 0, 110, .58], [64, 120, 110, .48], [67, 240, 110, .54], [72, 360, 110, .62], [76, 480, 110, .6], [72, 600, 110, .54], [67, 720, 110, .48], [64, 840, 110, .54], [60, 960, 110, .58], [64, 1080, 110, .48], [67, 1200, 110, .54], [72, 1320, 110, .62], [76, 1440, 110, .6], [72, 1560, 110, .54], [67, 1680, 110, .48], [64, 1800, 110, .54]])
        ];
        return specs;
    })();

    function patternById(id) {
        return patterns.find(pattern => pattern.id === id) || null;
    }

    // A bar range, 0-based and exclusive on the right.
    function span(from, to) {
        const bars = [];
        for (let bar = from; bar < to; bar++) bars.push(bar);
        return bars;
    }

    // Every arrangement — the showcase demo and the three templates — is built
    // from the same declarative plan, so the arrangement stays reviewable data
    // instead of hand-built state.
    function arrangement(name, tempo, plan) {
        const project = create(name);
        project.tempo = tempo;
        const tracks = plan.tracks.map(entry => {
            const track = addTrack(project, entry.instrument);
            track.name = entry.name;
            track.pan = entry.pan || 0;
            track.volume = entry.volume === undefined ? 0.8 : entry.volume;
            track.params = { ...track.params, ...(entry.params || {}) };
            if (entry.inserts) track.fx = { inserts: entry.inserts.map(spec => insertSpec(spec.type, spec)) };
            return track;
        });
        plan.sections.forEach(section => {
            const track = tracks[section.track];
            const source = patternById(section.pattern);
            if (!source) return;
            section.bars.forEach(bar => {
                const clip = cloneClip(source.clip, bar * BAR);
                if (section.velocity) clip.notes.forEach(note => { note.velocity = Math.min(1, note.velocity * section.velocity); });
                track.clips.push(clip);
            });
        });
        project.loop = { enabled: true, start: 0, end: BAR * plan.bars };
        (plan.scenes || []).forEach(scene => {
            project.scenes.push({ id: randomId('scene'), name: scene.name, start: scene.start * BAR, end: scene.end * BAR });
        });
        (plan.automation || []).forEach(entry => {
            const track = tracks[entry.track];
            track.automation = entry.points.map(point => ({ tick: point[0] * BAR, parameter: entry.parameter, value: point[1] }));
        });
        if (plan.master) project.master = { ...project.master, ...plan.master };
        return validate(project);
    }

    // The showcase: a 32 bar arrangement with an intro, a groove, a build, a
    // break and an outro. Two scenes and two automation curves make the
    // scene pads and the automation lanes audible on the very first launch.
    function demo() {
        return arrangement('Neon Sketch', 112, {
            bars: 32,
            tracks: [
                { name: 'Drums', instrument: 'drums-kit', pan: 0, params: { tone: 0.78, resonance: 0.1, drive: 0.22, attack: 0.002, release: 0.12, reverb: 0.04, delay: 0 }, inserts: [{ type: 'softclip', mix: 0.35, drive: 0.3 }] },
                { name: 'Bass', instrument: instrumentFor('bass-sub','bass',0), pan: -0.06, params: { tone: 0.44, resonance: 0.36, drive: 0.28, attack: 0.004, release: 0.14, reverb: 0.03, delay: 0.04 }, inserts: [{ type: 'eq3', mix: 1, low: 2.5, mid: -1.5, high: 0, freq: 900 }] },
                { name: 'Lead', instrument: instrumentFor('lead-saw','leads',0), pan: 0.24, params: { tone: 0.7, resonance: 0.26, drive: 0.22, attack: 0.008, release: 0.2, reverb: 0.2, delay: 0.2 }, inserts: [{ type: 'delay', mix: 0.2, time: 0.375, feedback: 0.3 }] },
                { name: 'Pads', instrument: instrumentFor('pad-warm','pads',0), pan: 0.2, params: { tone: 0.56, resonance: 0.18, drive: 0.05, attack: 0.3, release: 0.7, reverb: 0.38, delay: 0.12 }, inserts: [{ type: 'chorus', mix: 0.42, rate: 0.6, depth: 0.4 }] },
                { name: 'Keys', instrument: instrumentFor('key-electric-piano','keys',0), pan: -0.26, params: { tone: 0.72, resonance: 0.12, drive: 0.08, attack: 0.004, release: 0.24, reverb: 0.22, delay: 0.14 }, inserts: [{ type: 'tremolo', mix: 0.18, rate: 5.5, depth: 0.3 }] }
            ],
            sections: [
                { track: 0, pattern: 'drums-hats', bars: span(2, 4) },
                { track: 0, pattern: 'drums-rock', bars: span(4, 16) },
                { track: 0, pattern: 'drums-offbeat', bars: span(16, 24) },
                { track: 0, pattern: 'drums-rock', bars: span(28, 30) },
                { track: 0, pattern: 'drums-fill', bars: span(30, 32) },
                { track: 1, pattern: 'bass-octaves', bars: span(4, 16) },
                { track: 1, pattern: 'bass-walk', bars: span(16, 24) },
                { track: 1, pattern: 'bass-pulse', bars: span(24, 28) },
                { track: 1, pattern: 'bass-octaves', bars: span(28, 32) },
                { track: 2, pattern: 'leads-steps', bars: span(8, 12) },
                { track: 2, pattern: 'lead-rise', bars: span(12, 16) },
                { track: 2, pattern: 'leads-steps', bars: span(16, 24) },
                { track: 2, pattern: 'leads-echo', bars: span(24, 28) },
                { track: 3, pattern: 'pads-rise', bars: span(0, 16) },
                { track: 3, pattern: 'pads-open', bars: span(16, 28) },
                { track: 3, pattern: 'pads-rise', bars: span(28, 32) },
                { track: 4, pattern: 'keys-arpeggio', bars: span(0, 4) },
                { track: 4, pattern: 'keys-chords', bars: span(4, 12) },
                { track: 4, pattern: 'keys-arp16', bars: span(12, 16) },
                { track: 4, pattern: 'keys-arpeggio', bars: span(16, 24) },
                { track: 4, pattern: 'keys-arp16', bars: span(24, 28) },
                { track: 4, pattern: 'keys-chords', bars: span(28, 32) }
            ],
            scenes: [
                { name: 'Main', start: 4, end: 24 },
                { name: 'Break', start: 24, end: 28 }
            ],
            automation: [
                { track: 2, parameter: 'tone', points: [[0, 0.45], [12, 0.3], [16, 0.95], [17, 0.7], [24, 0.7], [25, 0.45]] },
                { track: 3, parameter: 'reverb', points: [[0, 0.3], [24, 0.32], [28, 0.85], [29, 0.4]] }
            ],
            master: { volume: 0.8, low: 1.5, mid: -1, high: 1, threshold: -9, ratio: 9, width: 1.12, plateSize: 0.58, plateDecay: 0.7, echoFeedback: 0.26, echoSync: '1/8' }
        });
    }

    const TEMPLATES = {
        electronic: {
            tempo: 124,
            bars: 16,
            tracks: [
                { name: 'Drums', instrument: 'drums-kit', params: { tone: 0.82, resonance: 0.14, drive: 0.3, attack: 0.002, release: 0.1, reverb: 0.03, delay: 0 }, inserts: [{ type: 'softclip', mix: 0.45, drive: 0.4 }] },
                { name: 'Bass', instrument: instrumentFor('bass-acid','bass',2), pan: -0.04, params: { tone: 0.34, resonance: 0.62, drive: 0.4, attack: 0.003, release: 0.1, reverb: 0.02, delay: 0.06 }, inserts: [{ type: 'cabinet', mix: 0.5, model: 2 }] },
                { name: 'Lead', instrument: instrumentFor('lead-supersaw','leads',10), pan: 0.2, params: { tone: 0.74, resonance: 0.3, drive: 0.26, attack: 0.01, release: 0.22, reverb: 0.22, delay: 0.22 }, inserts: [{ type: 'eq3', mix: 1, low: -1, mid: 1.5, high: 2, freq: 1400 }] },
                { name: 'Pads', instrument: instrumentFor('pad-nebula','pads',11), pan: 0.22, params: { tone: 0.6, resonance: 0.22, drive: 0.08, attack: 0.4, release: 0.8, reverb: 0.4, delay: 0.16 }, inserts: [{ type: 'phaser', mix: 0.4, rate: 0.5, depth: 0.6 }] },
                { name: 'Keys', instrument: instrumentFor('key-dx7-tines','keys',11), pan: -0.24, params: { tone: 0.8, resonance: 0.1, drive: 0.1, attack: 0.003, release: 0.26, reverb: 0.24, delay: 0.18 }, inserts: [{ type: 'chorus', mix: 0.36, rate: 0.9 }] }
            ],
            sections: [
                { track: 0, pattern: 'drums-rock', bars: span(0, 8) },
                { track: 0, pattern: 'drums-offbeat', bars: span(8, 12) },
                { track: 0, pattern: 'drums-rock', bars: span(12, 16) },
                { track: 1, pattern: 'bass-pulse', bars: span(0, 16) },
                { track: 2, pattern: 'leads-steps', bars: span(8, 12) },
                { track: 2, pattern: 'lead-rise', bars: span(12, 16) },
                { track: 3, pattern: 'pads-rise', bars: span(0, 8) },
                { track: 3, pattern: 'pads-open', bars: span(8, 16) },
                { track: 4, pattern: 'keys-arp16', bars: span(0, 16) }
            ],
            master: { volume: 0.78, low: 1, mid: -1.5, high: 1.5, threshold: -10, ratio: 10, width: 1.15, echoSync: '1/16' }
        },
        chill: {
            tempo: 84,
            bars: 16,
            tracks: [
                { name: 'Drums', instrument: 'drums-kit', volume: 0.6, params: { tone: 0.62, resonance: 0.08, drive: 0.12, attack: 0.004, release: 0.2, reverb: 0.12, delay: 0 }, inserts: [{ type: 'softclip', mix: 0.2, drive: 0.18 }] },
                { name: 'Bass', instrument: instrumentFor('bass-sub','bass',0), pan: -0.02, params: { tone: 0.4, resonance: 0.2, drive: 0.16, attack: 0.02, release: 0.3, reverb: 0.06, delay: 0 }, inserts: [] },
                { name: 'Lead', instrument: instrumentFor('lead-triangle','leads',3), pan: 0.26, params: { tone: 0.7, resonance: 0.14, drive: 0.06, attack: 0.03, release: 0.4, reverb: 0.34, delay: 0.2 }, inserts: [{ type: 'rotary', mix: 0.34, rate: 1.1, depth: 0.5 }] },
                { name: 'Pads', instrument: instrumentFor('pad-dream','pads',6), pan: 0.18, params: { tone: 0.54, resonance: 0.14, drive: 0.04, attack: 0.6, release: 1, reverb: 0.5, delay: 0.16 }, inserts: [{ type: 'chorus', mix: 0.45, rate: 0.35, depth: 0.5 }] },
                { name: 'Keys', instrument: instrumentFor('key-marimba','keys',5), pan: -0.22, params: { tone: 0.78, resonance: 0.1, drive: 0.04, attack: 0.004, release: 0.32, reverb: 0.26, delay: 0.1 }, inserts: [] }
            ],
            sections: [
                { track: 0, pattern: 'drums-offbeat', bars: span(0, 16) },
                { track: 1, pattern: 'bass-octaves', bars: span(0, 16) },
                { track: 2, pattern: 'leads-echo', bars: span(8, 16) },
                { track: 3, pattern: 'pads-open', bars: span(0, 16) },
                { track: 4, pattern: 'keys-arpeggio', bars: span(0, 8) },
                { track: 4, pattern: 'keys-chords', bars: span(8, 16) }
            ],
            master: { volume: 0.82, low: 2, mid: -1, high: 0.5, threshold: -14, ratio: 6, width: 1.08, plateSize: 0.66, plateDecay: 0.76, echoFeedback: 0.2, echoSync: '1/8' }
        },
        retro: {
            tempo: 108,
            bars: 16,
            tracks: [
                { name: 'Drums', instrument: 'drums-kit', params: { tone: 0.74, resonance: 0.2, drive: 0.26, attack: 0.002, release: 0.12, reverb: 0.06, delay: 0 }, inserts: [{ type: 'bitcrush', mix: 0.12, bits: 9 }] },
                { name: 'Bass', instrument: instrumentFor('bass-round','bass',1), pan: -0.05, params: { tone: 0.5, resonance: 0.3, drive: 0.24, attack: 0.01, release: 0.16, reverb: 0.05, delay: 0.04 }, inserts: [{ type: 'cabinet', mix: 0.42, model: 3 }] },
                { name: 'Lead', instrument: instrumentFor('lead-pwm','leads',11), pan: 0.22, params: { tone: 0.62, resonance: 0.34, drive: 0.24, attack: 0.008, release: 0.18, reverb: 0.2, delay: 0.2 }, inserts: [{ type: 'phaser', mix: 0.3, rate: 0.7, depth: 0.55 }] },
                { name: 'Pads', instrument: instrumentFor('pad-analog','pads',7), pan: 0.24, params: { tone: 0.5, resonance: 0.16, drive: 0.1, attack: 0.24, release: 0.6, reverb: 0.32, delay: 0.14 }, inserts: [{ type: 'chorus', mix: 0.52, rate: 0.85, depth: 0.45 }] },
                { name: 'Keys', instrument: instrumentFor('key-wurli','keys',10), pan: -0.2, params: { tone: 0.7, resonance: 0.18, drive: 0.2, attack: 0.004, release: 0.28, reverb: 0.2, delay: 0.12 }, inserts: [{ type: 'tremolo', mix: 0.22, rate: 4.2, depth: 0.35 }] }
            ],
            sections: [
                { track: 0, pattern: 'drums-rock', bars: span(0, 16) },
                { track: 1, pattern: 'bass-octaves', bars: span(0, 16) },
                { track: 2, pattern: 'leads-steps', bars: span(8, 16) },
                { track: 3, pattern: 'pads-rise', bars: span(4, 16) },
                { track: 4, pattern: 'keys-chords', bars: span(0, 8) },
                { track: 4, pattern: 'keys-arpeggio', bars: span(8, 16) }
            ],
            master: { volume: 0.8, low: 1.5, mid: 0.5, high: -1, threshold: -8, ratio: 12, width: 1.04, plateSize: 0.5, plateDecay: 0.68, echoFeedback: 0.24, echoSync: '1/8' }
        }
    };

    function template(style, name) {
        const plan = TEMPLATES[style] || TEMPLATES.electronic;
        return arrangement(name || style, plan.tempo, plan);
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

    return { PPQ, BAR, newID: randomId, create, validate, upgrade, addTrack, defaultParamsFor, defaultInsertsFor, createClip, patterns, patternById, demo, template, events, endTick, clone, cloneClip, history, FX_TYPES, MASTER_DEFAULTS, MAX_INSERTS, insertSpec };
});
