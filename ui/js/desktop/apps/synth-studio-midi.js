(function (root, factory) {
    const api = factory(root);
    if (typeof module === 'object' && module.exports) module.exports = api;
    else root.SynthStudioMIDI = api;
})(typeof globalThis !== 'undefined' ? globalThis : window, function (root) {
    'use strict';

    const MAX_MIDI_BYTES = 10 * 1024 * 1024;
    const CHANNELS = 16;
    const DRUM_CHANNEL = 9;

    function fail(code, message, cause) {
        const error = new Error(message);
        error.code = code;
        if (cause) error.cause = cause;
        return error;
    }

    function midiLibrary() {
        const library = root.SynthStudioMidiLib || (root.Midi ? { Midi: root.Midi } : null);
        if (!library || typeof library.Midi !== 'function') throw fail('E_MIDI_UNAVAILABLE', 'The local MIDI file library is unavailable.');
        return library.Midi;
    }

    function bytesFor(buffer) {
        if (buffer instanceof ArrayBuffer) return new Uint8Array(buffer);
        if (ArrayBuffer.isView(buffer)) return new Uint8Array(buffer.buffer, buffer.byteOffset, buffer.byteLength);
        throw fail('E_MIDI_TYPE', 'Choose a MIDI file to import.');
    }

    function checkHeader(bytes) {
        if (bytes.length < 14 || bytes[0] !== 0x4d || bytes[1] !== 0x54 || bytes[2] !== 0x68 || bytes[3] !== 0x64) {
            throw fail('E_MIDI_HEADER', 'This file does not have a valid Standard MIDI header.');
        }
        const length = (bytes[4] * 0x1000000) + (bytes[5] << 16) + (bytes[6] << 8) + bytes[7];
        if (length < 6 || bytes.length < 8 + length) throw fail('E_MIDI_HEADER', 'The MIDI header is incomplete.');
        const format = (bytes[8] << 8) | bytes[9];
        if (format === 2) throw fail('E_MIDI_FORMAT_2', 'Type 2 MIDI files have independent timelines and cannot be imported.');
        if (format !== 0 && format !== 1) throw fail('E_MIDI_FORMAT', 'Only Type 0 and Type 1 MIDI files are supported.');
        const division = (bytes[12] << 8) | bytes[13];
        if (division & 0x8000) throw fail('E_MIDI_SMPTE', 'SMPTE-timed MIDI files cannot be imported; use quarter-note timing.');
        if (division === 0) throw fail('E_MIDI_PPQ', 'The MIDI file has an invalid zero PPQ timing value.');
        return format;
    }

    function scaledTick(tick, sourcePPQ) {
        if (!Number.isFinite(tick) || tick < 0 || !Number.isFinite(sourcePPQ) || sourcePPQ < 1) {
            throw fail('E_MIDI_EVENT', 'The MIDI file contains an invalid event time.');
        }
        const result = Math.round(tick * 480 / sourcePPQ);
        if (result > 10_000_000) throw fail('E_MIDI_LIMIT', 'The MIDI file is longer than Synth Studio can import.');
        return result;
    }

    function trackInstrument(track) {
        if (track.channel === DRUM_CHANNEL || (track.instrument && track.instrument.percussion)) return 'gm-drums';
        const number = track.instrument && track.instrument.number;
        if (!Number.isInteger(number) || number < 0 || number > 127) throw fail('E_MIDI_INSTRUMENT', 'A MIDI track has an invalid GM instrument number.');
        return `gm-${number}`;
    }

    function importFile(arrayBuffer) {
        const bytes = bytesFor(arrayBuffer);
        if (bytes.length < 14 || bytes.length > MAX_MIDI_BYTES) throw fail('E_MIDI_SIZE', 'MIDI files must be between 14 bytes and 10 MiB.');
        checkHeader(bytes);

        let midi;
        try { midi = new (midiLibrary())(bytes); }
        catch (cause) { throw fail('E_MIDI_PARSE', 'The MIDI file could not be parsed.', cause); }
        const tempos = midi.header.tempos || [];
        if (tempos.length > 1 || (tempos.length === 1 && tempos[0].ticks !== 0)) {
            throw fail('E_MIDI_TEMPO_CHANGES', 'MIDI tempo changes are unsupported; export a file with one constant tempo.');
        }
        const signatures = midi.header.timeSignatures || [];
        if (signatures.length > 1 || (signatures.length === 1 && (signatures[0].ticks !== 0 || signatures[0].timeSignature[0] !== 4 || signatures[0].timeSignature[1] !== 4))) {
            throw fail('E_MIDI_TIME_SIGNATURE', 'Only a constant 4/4 time signature is supported.');
        }

        const model = root.SynthStudioModel;
        if (!model) throw fail('E_MODEL_UNAVAILABLE', 'The Synth Studio project model is unavailable.');
        const tempo = tempos.length ? tempos[0].bpm : 120;
        if (!Number.isFinite(tempo) || tempo < 40 || tempo > 240) throw fail('E_MIDI_TEMPO', 'MIDI tempo must be between 40 and 240 BPM.');
        const project = model.create((midi.name || 'Imported MIDI').trim().slice(0, 96) || 'Imported MIDI');
        project.tempo = tempo;
        const sourcePPQ = midi.header.ppq;
        if (!Number.isInteger(sourcePPQ) || sourcePPQ < 1 || sourcePPQ > 0x7fff) throw fail('E_MIDI_PPQ', 'The MIDI file has an invalid PPQ timing value.');
        if (!Array.isArray(midi.tracks) || midi.tracks.length > 16) throw fail('E_MIDI_TRACK_LIMIT', 'Synth Studio supports at most 15 melodic tracks and one drum rack.');

        let eventCount = 0;
        for (const sourceTrack of midi.tracks) {
            const instrument = trackInstrument(sourceTrack);
            let track;
            try { track = model.addTrack(project, instrument); }
            catch (cause) { throw fail('E_MIDI_TRACK_LIMIT', 'Synth Studio supports at most 15 melodic tracks and one drum rack.', cause); }
            const trackName = typeof sourceTrack.name === 'string' ? sourceTrack.name.trim() : '';
            if (trackName.length > 96) throw fail('E_MIDI_TRACK_NAME', 'A MIDI track name is longer than 96 characters.');
            if (trackName) track.name = trackName;
            const clip = model.createClip(instrument, 0);
            clip.name = track.name;
            const noteSource = Array.isArray(sourceTrack.notes) ? sourceTrack.notes : [];
            if (noteSource.length > 50_000 || (eventCount += noteSource.length) > 50_000) throw fail('E_MIDI_LIMIT', 'The MIDI file contains too many note events.');
            clip.notes = noteSource.map(note => {
                const start = scaledTick(note.ticks, sourcePPQ);
                const duration = Math.max(1, scaledTick(note.durationTicks, sourcePPQ));
                if (!Number.isInteger(note.midi) || note.midi < 0 || note.midi > 127 || !Number.isFinite(note.velocity) || note.velocity < 0 || note.velocity > 1) {
                    throw fail('E_MIDI_EVENT', 'The MIDI file contains a note with invalid pitch or velocity.');
                }
                return { id: randomId('note'), pitch: note.midi, start, duration, velocity: note.velocity };
            });
            const controlChanges = sourceTrack.controlChanges || {};
            const controllerNumbers = Object.keys(controlChanges).filter(key => /^\d+$/.test(key)).map(Number);
            for (const number of controllerNumbers) {
                if (number !== 1 && number !== 64) throw fail('E_MIDI_CONTROLLER_UNSUPPORTED', `MIDI controller ${number} is unsupported; only modulation, sustain, and pitch bend can be imported.`);
                const type = number === 1 ? 'modulation' : 'sustain';
                for (const cc of controlChanges[number] || []) {
                    if ((eventCount += 1) > 50_000) throw fail('E_MIDI_LIMIT', 'The MIDI file contains too many note and controller events.');
                    if (!Number.isFinite(cc.value) || cc.value < 0 || cc.value > 1) throw fail('E_MIDI_EVENT', 'The MIDI file contains an invalid controller value.');
                    clip.controllers.push({ tick: scaledTick(cc.ticks, sourcePPQ), type, value: cc.value });
                }
            }
            for (const bend of sourceTrack.pitchBends || []) {
                if ((eventCount += 1) > 50_000) throw fail('E_MIDI_LIMIT', 'The MIDI file contains too many note and controller events.');
                if (!Number.isFinite(bend.value) || bend.value < -1 || bend.value > 1) throw fail('E_MIDI_EVENT', 'The MIDI file contains an invalid pitch bend value.');
                clip.controllers.push({ tick: scaledTick(bend.ticks, sourcePPQ), type: 'bend', value: bend.value });
            }
            const noteEndTick = Math.max(0, ...clip.notes.map(note => note.start + note.duration));
            const controllerEndTick = Math.max(0, ...clip.controllers.map(event => event.tick + 1));
            const requiredLength = Math.max(noteEndTick, controllerEndTick);
            clip.length = Math.max(model.BAR, Math.ceil(requiredLength / model.BAR) * model.BAR);
            track.clips.push(clip);
        }
        try { return model.validate(project); }
        catch (cause) { throw fail('E_MIDI_PROJECT', `The MIDI file cannot fit the Synth Studio project limits: ${cause.message}`, cause); }
    }

    function categoryFor(instrument) {
        if (instrument === 'drums-kit' || instrument === 'gm-drums') return 'drums';
        if (instrument.startsWith('gm-')) return 'keys';
        const catalog = root.SynthStudioPresets;
        const preset = catalog && typeof catalog.get === 'function'
            ? catalog.get(instrument)
            : catalog && Array.isArray(catalog.presets) ? catalog.presets.find(item => item && item.id === instrument) : null;
        if (preset && ['bass', 'leads', 'pads', 'keys', 'drums'].includes(preset.category)) return preset.category;
        if (instrument.startsWith('bass-')) return 'bass';
        if (instrument.startsWith('lead-')) return 'leads';
        if (instrument.startsWith('pad-')) return 'pads';
        if (instrument.startsWith('key-')) return 'keys';
        if (instrument.startsWith('drum-')) return 'drums';
        return 'keys';
    }

    function approximateGM(instrument) {
        const match = /^gm-(\d{1,3})$/.exec(instrument);
        if (match) return Math.min(127, Number(match[1]));
        return { bass: 38, leads: 81, pads: 89, keys: 4, drums: 0 }[categoryFor(instrument)];
    }

    function exportFile(inputProject) {
        const model = root.SynthStudioModel;
        if (!model) throw fail('E_MODEL_UNAVAILABLE', 'The Synth Studio project model is unavailable.');
        const project = model.validate(inputProject);
        const Midi = midiLibrary();
        const midi = new Midi();
        midi.name = project.name;
        midi.header.setTempo(project.tempo);
        midi.header.timeSignatures = [{ ticks: 0, timeSignature: [4, 4] }];
        midi.header.update();

        let nextMelodicChannel = 0;
        for (const sourceTrack of project.tracks) {
            const category = categoryFor(sourceTrack.instrument);
            const drums = category === 'drums';
            const channel = drums ? DRUM_CHANNEL : nextMelodicChannel++ + (nextMelodicChannel > DRUM_CHANNEL ? 1 : 0);
            if (channel >= CHANNELS) throw fail('E_MIDI_TRACK_LIMIT', 'MIDI export supports 15 melodic tracks and one drum rack.');
            const track = midi.addTrack();
            track.name = sourceTrack.name;
            track.channel = channel;
            track.instrument.number = approximateGM(sourceTrack.instrument);
            for (const clip of sourceTrack.clips) {
                for (const note of clip.notes) {
                    track.addNote({ midi: note.pitch, ticks: clip.start + note.start, durationTicks: note.duration, velocity: note.velocity });
                }
                for (const event of clip.controllers) {
                    const ticks = clip.start + event.tick;
                    if (event.type === 'bend') track.addPitchBend({ ticks, value: Math.max(-8192, Math.min(8191, Math.round(event.value * 8192))) });
                    else track.addCC({ number: event.type === 'sustain' ? 64 : 1, ticks, value: event.value });
                }
            }
        }
        return midi.toArray();
    }

    function randomId(prefix) {
        const cryptoApi = root.crypto;
        const random = cryptoApi && typeof cryptoApi.randomUUID === 'function'
            ? cryptoApi.randomUUID()
            : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
        return `${prefix}-${random}`;
    }

    function eventTime(event) {
        if (event && Number.isFinite(event.receivedTime)) return event.receivedTime;
        return root.performance && typeof root.performance.now === 'function' ? root.performance.now() : Date.now();
    }

    function create(options = {}) {
        const onNoteOn = typeof options.onNoteOn === 'function' ? options.onNoteOn : () => {};
        const onNoteOff = typeof options.onNoteOff === 'function' ? options.onNoteOff : () => {};
        const onController = typeof options.onController === 'function' ? options.onController : () => {};
        const onState = typeof options.onState === 'function' ? options.onState : () => {};
        let access = null;
        let selected = null;
        let disposed = false;
        let generation = 0;
        let selectionGeneration = 0;
        let accessStateHandler = null;
        let messageHandler = null;
        let propertyPreviousHandler = null;
        const activeNotes = new Map();

        function setState(status, extra = {}) {
            onState({ status, ...extra });
        }

        function inputs() {
            if (!access || !access.inputs || typeof access.inputs.values !== 'function') return [];
            return Array.from(access.inputs.values()).map(input => ({ id: input.id, name: input.name || input.manufacturer || input.id }));
        }

        function releaseNotes() {
            const time = root.performance && typeof root.performance.now === 'function' ? root.performance.now() : Date.now();
            for (const [key, note] of activeNotes) {
                for (let count = 0; count < note.count; count++) onNoteOff({ pitch: note.pitch, velocity: 0, time, channel: note.channel, released: true });
                activeNotes.delete(key);
            }
        }

        function detachInput() {
            if (!selected || !messageHandler) return;
            const input = selected;
            if (typeof input.removeEventListener === 'function') input.removeEventListener('midimessage', messageHandler);
            else if (input.onmidimessage === messageHandler) input.onmidimessage = propertyPreviousHandler || null;
            messageHandler = null;
            propertyPreviousHandler = null;
            releaseNotes();
            if (typeof input.close === 'function') Promise.resolve(input.close()).catch(() => {});
        }

        function detachAccess() {
            if (!access || !accessStateHandler) return;
            if (typeof access.removeEventListener === 'function') access.removeEventListener('statechange', accessStateHandler);
            else if (access.onstatechange === accessStateHandler) access.onstatechange = null;
            accessStateHandler = null;
        }

        function handleMessage(event) {
            if (disposed || !selected || !event || !event.data || event.data.length < 2) return;
            const data = event.data;
            const status = data[0] & 0xf0;
            const channel = data[0] & 0x0f;
            const first = data[1] & 0x7f;
            const second = (data[2] || 0) & 0x7f;
            const time = eventTime(event);
            if (status === 0x90 && second > 0) {
                const key = `${channel}:${first}`;
                const current = activeNotes.get(key) || { pitch: first, channel, count: 0 };
                current.count++;
                activeNotes.set(key, current);
                onNoteOn({ pitch: first, velocity: second / 127, time, channel });
            } else if (status === 0x80 || (status === 0x90 && second === 0)) {
                const key = `${channel}:${first}`;
                const current = activeNotes.get(key);
                if (current) {
                    current.count--;
                    if (current.count <= 0) activeNotes.delete(key);
                }
                onNoteOff({ pitch: first, velocity: second / 127, time, channel });
            } else if (status === 0xb0 && data.length >= 3 && (first === 1 || first === 64)) {
                onController({ type: first === 1 ? 'modulation' : 'sustain', value: second / 127, time, channel });
            } else if (status === 0xe0 && data.length >= 3) {
                const value = ((second << 7) | first) - 8192;
                onController({ type: 'bend', value: value / 8192, time, channel });
            }
        }

        function handleStateChange(event) {
            if (disposed || !selected) return;
            const port = event && (event.port || event.target);
            if (port && port.id === selected.id && port.state === 'disconnected') {
                const lost = { id: selected.id, name: selected.name || selected.id };
                detachInput();
                selected = null;
                setState('disconnected', { reason: 'device-lost', device: lost, inputs: inputs() });
            }
        }

        async function connect() {
            if (disposed) throw fail('E_MIDI_DISPOSED', 'The MIDI input has been disposed.');
            if (access) return inputs();
            const navigatorApi = root.navigator;
            if (!navigatorApi || typeof navigatorApi.requestMIDIAccess !== 'function') {
                setState('unsupported');
                throw fail('E_MIDI_UNSUPPORTED', 'This browser does not support Web MIDI input.');
            }
            const requestGeneration = ++generation;
            let midiAccess;
            try { midiAccess = await navigatorApi.requestMIDIAccess({ sysex: false }); }
            catch (cause) {
                if (requestGeneration !== generation || disposed) return [];
                const denied = cause && ['NotAllowedError', 'SecurityError'].includes(cause.name);
                setState(denied ? 'denied' : 'disconnected', { code: denied ? 'E_MIDI_DENIED' : 'E_MIDI_ACCESS', message: denied ? 'MIDI input permission was denied.' : 'MIDI input could not be opened.' });
                throw fail(denied ? 'E_MIDI_DENIED' : 'E_MIDI_ACCESS', denied ? 'MIDI input permission was denied.' : 'MIDI input could not be opened.', cause);
            }
            if (requestGeneration !== generation || disposed) return [];
            access = midiAccess;
            accessStateHandler = handleStateChange;
            if (typeof access.addEventListener === 'function') access.addEventListener('statechange', accessStateHandler);
            else access.onstatechange = accessStateHandler;
            const available = inputs();
            setState('disconnected', { inputs: available });
            return available;
        }

        async function select(id) {
            if (disposed) throw fail('E_MIDI_DISPOSED', 'The MIDI input has been disposed.');
            if (!access) throw fail('E_MIDI_NOT_CONNECTED', 'Connect to MIDI inputs before selecting a device.');
            if (id === '' || id == null) {
                selectionGeneration++;
                detachInput();
                selected = null;
                setState('disconnected', { inputs: inputs() });
                return;
            }
            const input = access.inputs && typeof access.inputs.get === 'function' ? access.inputs.get(id) : null;
            if (!input || input.state === 'disconnected') throw fail('E_MIDI_DEVICE_MISSING', 'The selected MIDI input is no longer available.');
            if (selected === input) return;
            const requestGeneration = ++selectionGeneration;
            detachInput();
            selected = null;
            try { if (typeof input.open === 'function') await input.open(); }
            catch (cause) {
                if (requestGeneration !== selectionGeneration || disposed) return;
                setState('disconnected', { code: 'E_MIDI_DEVICE_OPEN', message: 'The selected MIDI input could not be opened.' });
                throw fail('E_MIDI_DEVICE_OPEN', 'The selected MIDI input could not be opened.', cause);
            }
            if (requestGeneration !== selectionGeneration || disposed || !access || input.state === 'disconnected') {
                if (typeof input.close === 'function') Promise.resolve(input.close()).catch(() => {});
                return;
            }
            selected = input;
            messageHandler = handleMessage;
            if (typeof selected.addEventListener === 'function') selected.addEventListener('midimessage', messageHandler);
            else {
                propertyPreviousHandler = selected.onmidimessage || null;
                selected.onmidimessage = event => {
                    handleMessage(event);
                    if (typeof propertyPreviousHandler === 'function') propertyPreviousHandler.call(selected, event);
                };
                messageHandler = selected.onmidimessage;
            }
            setState('connected', { device: { id: selected.id, name: selected.name || selected.id }, inputs: inputs() });
        }

        function disconnect() {
            generation++;
            selectionGeneration++;
            detachInput();
            selected = null;
            detachAccess();
            access = null;
            if (!disposed) setState('disconnected', { inputs: [] });
        }

        function dispose() {
            if (disposed) return;
            disconnect();
            disposed = true;
        }

        return { connect, select, inputs, dispose, disconnect };
    }

    return { importFile, exportFile, create };
});
