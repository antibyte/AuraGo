(function (root) {
  "use strict";

  const TINY = 0.0001;
  const MAX_TRACKS = 64;
  const MAX_NOTES = 12000;
  const MAX_ACTIVE_NOTES = 256;
  const MAX_RENDER_SECONDS = 300;
  const MAX_EFFECT_TAIL_SECONDS = 3.5;
  const LOOKAHEAD = 0.12;
  const TICK_MS = 25;
  const DEFAULT_PARAMS = { tone: 0.5, attack: 0.01, release: 0.3, reverb: 0, delay: 0 };
  const GM = root.SynthStudioGMData;
  const PRESETS = root.SynthStudioPresets;

  function clamp(value, low, high, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(high, Math.max(low, number)) : fallback;
  }

  function codedError(code, message) {
    const error = new Error(message);
    error.code = code;
    return error;
  }

  function budgetError(message) {
    return codedError("SYNTH_STUDIO_RENDER_BUDGET", message);
  }

  function exportLimitError(message) {
    return codedError("export_limit", message);
  }

  function audioContextConstructor() {
    return root.AudioContext || root.webkitAudioContext;
  }

  function offlineContextConstructor() {
    return root.OfflineAudioContext || root.webkitOfflineAudioContext;
  }

  function normalizeProject(project) {
    if (!project || typeof project !== "object" || !Array.isArray(project.tracks)) {
      throw new TypeError("Synth Studio project needs a tracks array.");
    }
    if (project.tracks.length > MAX_TRACKS) throw budgetError(`A project can have at most ${MAX_TRACKS} audio tracks.`);
    const tempo = clamp(project.tempo, 20, 300, 120);
    const ppq = Math.round(clamp(project.ppq, 24, 1920, 480));
    const tracks = [];
    const ids = new Set();
    let noteCount = 0;
    let endTick = 0;
    for (let index = 0; index < project.tracks.length; index++) {
      const source = project.tracks[index] || {};
      const id = String(source.id || `track-${index + 1}`);
      if (ids.has(id)) throw new TypeError(`Duplicate audio track id: ${id}`);
      ids.add(id);
      const track = {
        id,
        name: String(source.name || id),
        instrument: String(source.instrument || "lead-saw"),
        volume: clamp(source.volume, 0, 1.5, 0.8),
        pan: clamp(source.pan, -1, 1, 0),
        mute: Boolean(source.mute),
        solo: Boolean(source.solo),
        params: { ...DEFAULT_PARAMS, ...(source.params || {}) },
        clips: []
      };
      track.params.tone = clamp(track.params.tone, 0, 1, DEFAULT_PARAMS.tone);
      track.params.attack = clamp(track.params.attack, 0, 10, DEFAULT_PARAMS.attack);
      track.params.release = clamp(track.params.release, 0, 30, DEFAULT_PARAMS.release);
      track.params.reverb = clamp(track.params.reverb, 0, 1, DEFAULT_PARAMS.reverb);
      track.params.delay = clamp(track.params.delay, 0, 1, DEFAULT_PARAMS.delay);
      for (let clipIndex = 0; clipIndex < (Array.isArray(source.clips) ? source.clips.length : 0); clipIndex++) {
        const input = source.clips[clipIndex] || {};
        const start = Math.max(0, Math.floor(clamp(input.start, 0, 1e9, 0)));
        const length = Math.max(0, Math.floor(clamp(input.length, 0, 1e9, 0)));
        const clip = { id: String(input.id || `${id}-clip-${clipIndex + 1}`), name: String(input.name || ""), start, length, notes: [], controllers: [] };
        for (let noteIndex = 0; noteIndex < (Array.isArray(input.notes) ? input.notes.length : 0); noteIndex++) {
          const note = input.notes[noteIndex] || {};
          const localStart = Math.floor(clamp(note.start, 0, 1e9, 0));
          const duration = Math.max(1, Math.floor(clamp(note.duration, 1, 1e9, 1)));
          const pitch = Math.round(clamp(note.pitch, 0, 127, -1));
          if (pitch < 0) continue;
          const absoluteStart = start + localStart;
          const absoluteEnd = absoluteStart + duration;
          if (absoluteEnd > Number.MAX_SAFE_INTEGER) throw new TypeError("Audio note tick is outside the safe range.");
          clip.notes.push({
            id: String(note.id || `${clip.id}-note-${noteIndex + 1}`),
            pitch, start: localStart, duration,
            velocity: clamp(note.velocity, 0, 1, 0.8),
            absoluteStart, absoluteEnd,
            key: `${id}/${clip.id}/${noteIndex}`
          });
          noteCount++;
          endTick = Math.max(endTick, absoluteEnd);
        }
        for (const controller of Array.isArray(input.controllers) ? input.controllers : []) {
          if (!controller || !["sustain", "bend", "modulation"].includes(controller.type)) continue;
          const tick = Math.max(0, Math.floor(clamp(controller.tick, 0, 1e9, 0)));
          const value = controller.type === "bend"
            ? clamp(controller.value, -1, 1, 0)
            : clamp(controller.value, 0, 1, 0);
          clip.controllers.push({ tick, type: controller.type, value, absoluteTick: start + tick });
          endTick = Math.max(endTick, start + tick);
        }
        track.clips.push(clip);
        endTick = Math.max(endTick, start + length);
      }
      tracks.push(track);
    }
    if (noteCount > MAX_NOTES) throw budgetError(`A project can contain at most ${MAX_NOTES} notes.`);
    const loop = project.loop && typeof project.loop === "object" ? {
      enabled: Boolean(project.loop.enabled),
      start: Math.max(0, Math.floor(clamp(project.loop.start, 0, 1e9, 0))),
      end: Math.max(0, Math.floor(clamp(project.loop.end, 0, 1e9, 0)))
    } : { enabled: false, start: 0, end: 0 };
    if (loop.enabled && loop.end <= loop.start) throw new TypeError("An enabled loop needs an end tick after its start tick.");
    if (loop.enabled && (loop.end - loop.start) * (60 / tempo / ppq) < 0.05) {
      throw new TypeError("An enabled loop must be at least 50 ms long.");
    }
    return { tracks, tempo, ppq, beatsPerBar: Math.round(clamp(project.beatsPerBar, 1, 16, 4)), loop, endTick, noteCount, metronome: project.metronome === true };
  }

  function isAudible(track, anySolo) {
    return !track.mute && (!anySolo || track.solo);
  }

  function allEvents(info) {
    const events = [];
    for (const track of info.tracks) {
      for (const clip of track.clips) {
        for (const note of clip.notes) {
          events.push({ type: "noteOn", tick: note.absoluteStart, ownerTick: note.absoluteStart, key: note.key, track, note });
          events.push({ type: "noteOff", tick: note.absoluteEnd, ownerTick: note.absoluteStart, key: note.key, track, note });
        }
        for (const controller of clip.controllers) {
          events.push({ type: "controller", tick: controller.absoluteTick, track, controller });
        }
      }
    }
    if (info.metronome) {
      const end = Math.max(info.endTick, info.loop.enabled ? info.loop.end : 0);
      const count = Math.floor(end / info.ppq) + 1;
      if (count > MAX_NOTES * 2) throw budgetError("Metronome output exceeds the event limit.");
      for (let tick = 0; tick <= end; tick += info.ppq) {
        events.push({ type: "metronome", tick, accent: Math.floor(tick / info.ppq) % info.beatsPerBar === 0 });
      }
    }
    const order = { metronome: 0, controller: 1, noteOff: 2, noteOn: 3 };
    events.sort((a, b) => a.tick - b.tick || order[a.type] - order[b.type]);
    return events;
  }

  function seedControllers(events, info, startTick) {
    const seeds = [];
    for (const track of info.tracks) {
      const latest = new Map();
      for (const event of events) {
        if (event.type !== "controller" || event.track.id !== track.id || event.tick >= startTick) continue;
        const previous = latest.get(event.controller.type);
        if (!previous || event.tick >= previous.tick) latest.set(event.controller.type, event);
      }
      for (const event of latest.values()) {
        seeds.push({
          type: "controller",
          tick: startTick,
          track,
          controller: { ...event.controller, absoluteTick: startTick }
        });
      }
    }
    return seeds;
  }

  function loopControllerSeeds(events, info) {
    const defaults = [];
    for (const track of info.tracks) {
      defaults.push(
        { type: "controller", tick: info.loop.start, track, controller: { type: "sustain", value: 0 } },
        { type: "controller", tick: info.loop.start, track, controller: { type: "bend", value: 0 } },
        { type: "controller", tick: info.loop.start, track, controller: { type: "modulation", value: 0 } }
      );
    }
    return [...defaults, ...seedControllers(events, info, info.loop.start)];
  }

  function eventsForRange(events, startTick, endTick, initialStart) {
    return events.filter(event => {
      if (event.type === "noteOff") return event.ownerTick >= initialStart && event.ownerTick < endTick;
      return event.tick >= initialStart && event.tick < endTick;
    }).filter(event => event.tick >= startTick || event.type === "noteOff");
  }

  function trackParams(track) {
    return track.params || DEFAULT_PARAMS;
  }

  function getGMPatch(note) {
    return GM?.drums?.[note - 35]?.operators || fallbackDrumPatch(note);
  }

  function fallbackDrumPatch(note) {
    if (note === 35 || note === 36 || note === 41 || note === 43 || note === 45 || note === 47 || note === 48 || note === 50) {
      return [{ w: "sine", t: 0, f: Math.max(45, 170 - (note - 35) * 10), v: 0.8, d: 0.12, r: 0.08, p: 0.45, q: 0.12 }];
    }
    return [{ w: "n0", t: 0, f: 440, v: 0.25, a: 0.001, d: 0.18, r: 0.08 }];
  }

  function customPatch(patch) {
    const p = patch || {};
    const operators = [{
      w: p.wave || "sawtooth",
      v: clamp(p.level, 0, 1.5, 0.24),
      a: clamp(p.attack, 0, 5, 0.01),
      d: clamp(p.decay, 0, 12, 0.25),
      s: clamp(p.sustain, 0, 1.5, 0.65),
      r: clamp(p.release, 0, 30, 0.15),
      t: 1
    }];
    if (p.second) {
      operators.push({ w: p.wave || "sawtooth", v: clamp(p.second, 0, 1, 0.12), t: Math.pow(2, clamp(p.detune, -1200, 1200, 8) / 1200), a: operators[0].a, d: operators[0].d, s: operators[0].s, r: operators[0].r });
    }
    if (p.mod) {
      operators.push({ w: p.mod, v: clamp(p.depth, 0, 24, 2), t: clamp(p.ratio, 0.01, 32, 2), a: operators[0].a, d: operators[0].d, s: operators[0].s, r: operators[0].r, g: 1 });
    }
    if (p.noise) operators.push({ w: "n0", v: clamp(p.noise, 0, 1, 0.02), a: 0.001, d: 0.2, s: 0, r: 0.04 });
    return operators;
  }

  function operatorsFor(track, pitch) {
    const instrument = track.instrument;
    if (instrument === "gm-drums" || instrument === "drums-kit") return getGMPatch(pitch);
    const preset = PRESETS?.get(instrument);
    if (preset?.category === "drums") return getGMPatch(preset.drumNote);
    const match = /^gm-(\d+)$/.exec(instrument);
    if (match) return GM?.programs?.[Number(match[1])]?.operators || customPatch({ wave: "sine" });
    return customPatch(preset?.patch);
  }

  function maxReleaseSeconds(info) {
    let maximum = 0;
    for (const track of info.tracks) {
      maximum = Math.max(maximum, track.params.release);
      const patches = new Map();
      for (const clip of track.clips) for (const note of clip.notes) {
        const key = `${track.instrument}:${note.pitch}`;
        if (!patches.has(key)) patches.set(key, operatorsFor(track, note.pitch));
      }
      for (const operators of patches.values()) {
        for (const operator of operators || []) maximum = Math.max(maximum, clamp(operator.r, 0, 30, 0));
      }
    }
    return maximum;
  }

  function makeNoiseBuffers(context) {
    const buffers = {};
    const length = Math.max(2048, Math.floor(context.sampleRate * 0.5));
    for (const kind of ["n0", "n1"]) {
      const buffer = context.createBuffer(1, length, context.sampleRate);
      const data = buffer.getChannelData(0);
      let seed = kind === "n0" ? 0x1a2b3c4d : 0x5a17c9e3;
      for (let i = 0; i < length; i++) {
        seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5;
        const noise = ((seed >>> 0) / 0x80000000) - 1;
        data[i] = kind === "n0" ? noise * 0.5 : (noise * 0.32 + Math.sin(i * 0.071) * 0.18);
      }
      buffers[kind] = buffer;
    }
    return buffers;
  }

  function makeReverbBuffer(context) {
    const length = Math.max(1, Math.floor(context.sampleRate * 2.2));
    const buffer = context.createBuffer(2, length, context.sampleRate);
    let seed = 0x7f4a7c15;
    for (let channel = 0; channel < 2; channel++) {
      const data = buffer.getChannelData(channel);
      for (let i = 0; i < length; i++) {
        seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5;
        const noise = ((seed >>> 0) / 0x80000000) - 1;
        data[i] = noise * Math.pow(1 - i / length, 3.5) * 0.22;
      }
    }
    return buffer;
  }

  function makeMix(context) {
    const master = context.createGain();
    master.gain.value = 0.78;
    let output = context.destination;
    if (context.createDynamicsCompressor) {
      const limiter = context.createDynamicsCompressor();
      limiter.threshold.value = -8;
      limiter.knee.value = 8;
      limiter.ratio.value = 10;
      limiter.attack.value = 0.003;
      limiter.release.value = 0.18;
      master.connect(limiter);
      output = limiter;
    } else {
      master.connect(output);
    }
    if (output !== context.destination) output.connect(context.destination);
    return { master, output, noise: makeNoiseBuffers(context), impulse: makeReverbBuffer(context), periodic: null };
  }

  function createTrackBus(context, mix, track) {
    const input = context.createGain();
    const filter = context.createBiquadFilter();
    filter.type = "lowpass";
    const pan = context.createStereoPanner ? context.createStereoPanner() : context.createPanner();
    const gain = context.createGain();
    input.connect(filter);
    filter.connect(pan);
    pan.connect(gain);
    gain.connect(mix.master);
    const reverb = context.createConvolver();
    reverb.buffer = mix.impulse;
    const reverbGain = context.createGain();
    filter.connect(reverb);
    reverb.connect(reverbGain);
    reverbGain.connect(mix.master);
    const delay = context.createDelay(1.5);
    delay.delayTime.value = 0.24;
    const feedback = context.createGain();
    feedback.gain.value = 0.24;
    const delayGain = context.createGain();
    filter.connect(delay);
    delay.connect(delayGain);
    delayGain.connect(mix.master);
    delay.connect(feedback);
    feedback.connect(delay);
    const bus = { input, filter, pan, gain, reverb, reverbGain, delay, delayGain, feedback };
    updateTrackBus(bus, track);
    return bus;
  }

  function scheduleMetronome(context, mix, at, accent) {
    const oscillator = context.createOscillator();
    const gain = context.createGain();
    oscillator.type = "sine";
    oscillator.frequency.setValueAtTime(accent ? 1320 : 880, at);
    gain.gain.setValueAtTime(0.14, at);
    gain.gain.exponentialRampToValueAtTime(0.0001, at + 0.045);
    oscillator.connect(gain);
    gain.connect(mix.master);
    oscillator.start(at);
    oscillator.stop(at + 0.05);
  }

  function updateTrackBus(bus, track) {
    const p = trackParams(track);
    const now = bus.input.context.currentTime;
    const presetTone = PRESETS?.get(track.instrument)?.patch?.cutoff;
    const tone = Number.isFinite(presetTone) ? p.tone * 0.65 + presetTone * 0.35 : p.tone;
    bus.gain.gain.setValueAtTime(track.mute ? 0 : track.volume, now);
    bus.filter.frequency.setValueAtTime(240 + tone * 18500, now);
    bus.filter.Q.setValueAtTime(0.7 + tone * 1.2, now);
    if (bus.pan.pan) bus.pan.pan.setValueAtTime(track.pan, now);
    else if (bus.pan.setPosition) bus.pan.setPosition(track.pan, 0, 1 - Math.abs(track.pan));
    bus.reverbGain.gain.setValueAtTime(p.reverb * 0.32, now);
    bus.delayGain.gain.setValueAtTime(p.delay * 0.38, now);
  }

  function waveFor(context, mix, wave) {
    if (wave !== "w9999") return wave === "n0" || wave === "n1" ? "noise" : wave;
    if (!mix.periodic) {
      const real = new Float32Array([0, 1, 0.45, 0.28, 0.18, 0.12]);
      const imag = new Float32Array(real.length);
      mix.periodic = context.createPeriodicWave(real, imag, { disableNormalization: false });
    }
    return mix.periodic;
  }

  function envelopeLevel(op, peak, age, attack, hold, decay) {
    if (age <= 0) return 0;
    if (attack > TINY && age < attack) return peak * age / attack;
    const afterAttack = Math.max(0, age - attack);
    if (afterAttack < hold) return peak;
    const afterHold = afterAttack - hold;
    if (decay > TINY && afterHold < decay) {
      const sustain = clamp(op.s, 0, 1.5, 0);
      return peak + (peak * sustain - peak) * afterHold / decay;
    }
    return peak * clamp(op.s, 0, 1.5, 0);
  }

  function attachVibrato(context, voice, amount, at) {
    if (!amount || voice.vibrato) {
      if (voice.vibrato) voice.vibrato.gain.gain.setTargetAtTime(amount * 45, at, 0.015);
      return;
    }
    const oscillator = context.createOscillator();
    const gain = context.createGain();
    oscillator.type = "sine";
    oscillator.frequency.value = 5.4;
    gain.gain.setValueAtTime(amount * 45, at);
    oscillator.connect(gain);
    for (const source of voice.sources) if (source.detune) gain.connect(source.detune);
    oscillator.start(at);
    voice.vibrato = { oscillator, gain };
  }

  function createVoice(context, mix, bus, state, track, pitch, velocity, at, operators) {
    const voiceGain = context.createGain();
    voiceGain.gain.value = 1;
    voiceGain.connect(bus.input);
    const sources = [];
    const envelopes = [];
    const operatorsOut = [];
    const pitchHz = 440 * Math.pow(2, (pitch - 69) / 12);
    const velocityScale = Math.pow(velocity, 2);
    const params = trackParams(track);
    for (let index = 0; index < operators.length; index++) {
      const op = operators[index] || {};
      const fixed = Number.isFinite(op.f) ? op.f : 0;
      const tune = Number.isFinite(op.t) ? op.t : 1;
      const baseFrequency = Math.max(0.01, pitchHz * tune + fixed);
      const source = op.w === "n0" || op.w === "n1" ? context.createBufferSource() : context.createOscillator();
      if (source.buffer !== undefined) {
        source.buffer = mix.noise[op.w] || mix.noise.n0;
        source.loop = true;
      } else {
        const wave = waveFor(context, mix, op.w || "sine");
        if (typeof wave === "string") source.type = wave;
        else source.setPeriodicWave(wave);
        source.frequency.setValueAtTime(baseFrequency, at);
        if (op.p && op.p !== 1 && tune !== 0 && baseFrequency > 0) {
          const startFrequency = Math.max(0.01, baseFrequency * clamp(op.p, 0.05, 8, 1));
          source.frequency.setValueAtTime(startFrequency, at);
          source.frequency.exponentialRampToValueAtTime(baseFrequency, at + clamp(op.q, 0.01, 2, 0.12));
        }
      }
      if (source.detune) source.detune.setValueAtTime(state.bend * 200, at);
      const gain = context.createGain();
      const keyScale = Math.pow(2, ((pitch - 60) * clamp(op.k, -2, 2, 0)) / 12);
      const carrier = !op.g;
      const rawLevel = Number.isFinite(op.v) ? Math.abs(op.v) : 0.5;
      const peak = carrier
        ? clamp(rawLevel * velocityScale * keyScale, 0, 2, 0.2)
        : clamp(rawLevel * Math.max(1, baseFrequency) * velocityScale * keyScale, 0, 5000, 1);
      const attack = Math.max(clamp(op.a, 0, 5, 0), params.attack);
      const hold = clamp(op.h, 0, 5, 0);
      const decay = clamp(op.d, 0, 12, 0.08);
      const release = Math.max(clamp(op.r, 0, 30, 0), params.release);
      const sustain = clamp(op.s, 0, 1.5, 0);
      gain.gain.setValueAtTime(0, at);
      gain.gain.linearRampToValueAtTime(peak, at + attack);
      gain.gain.setValueAtTime(peak, at + attack + hold);
      gain.gain.linearRampToValueAtTime(peak * sustain, at + attack + hold + decay);
      source.connect(gain);
      operatorsOut.push({ source, gain, route: Math.floor(Number(op.g) || 0) });
      envelopes.push({ gain, op, peak, attack, hold, decay, release });
    }
    for (const operator of operatorsOut) {
      const destination = operator.route;
      if (destination <= 0) {
        operator.gain.connect(voiceGain);
      } else if (destination > 10) {
        const target = operatorsOut[destination - 11];
        if (target) operator.gain.connect(target.gain.gain);
        else operator.gain.connect(voiceGain);
      } else {
        const target = operatorsOut[destination - 1];
        const parameter = target?.source.frequency || target?.source.playbackRate;
        if (parameter) operator.gain.connect(parameter);
        else operator.gain.connect(voiceGain);
      }
      if (operator.source.start) operator.source.start(at);
      sources.push(operator.source);
    }
    const releaseSeconds = Math.max(params.release, ...envelopes.map(envelope => envelope.release));
    const voice = { trackId: track.id, sources, envelopes, releaseSeconds, startTime: at, endTime: Infinity, voiceGain, vibrato: null, releaseRequested: false, disposed: false, liveInput: false };
    attachVibrato(context, voice, state.mod, at);
    return voice;
  }

  function disposeVoice(voice) {
    if (voice.disposed) return;
    voice.disposed = true;
    try { voice.voiceGain.disconnect(); } catch (_) {}
    for (const envelope of voice.envelopes) try { envelope.gain.disconnect(); } catch (_) {}
    if (voice.vibrato) {
      try { voice.vibrato.gain.disconnect(); } catch (_) {}
    }
  }

  function releaseVoice(voice, at, tailOverride) {
    if (!voice || voice.releaseRequested || voice.disposed) return;
    voice.releaseRequested = true;
    const tail = tailOverride === undefined ? voice.releaseSeconds : clamp(tailOverride, 0, 30, voice.releaseSeconds);
    for (const envelope of voice.envelopes) {
      const age = Math.max(0, at - voice.startTime);
      const held = envelopeLevel(envelope.op, envelope.peak, age, envelope.attack, envelope.hold, envelope.decay);
      envelope.gain.gain.cancelScheduledValues(at);
      envelope.gain.gain.setValueAtTime(held, at);
      envelope.gain.gain.linearRampToValueAtTime(0, at + tail);
    }
    voice.endTime = at + tail + 0.04;
    if (voice.vibrato) {
      voice.vibrato.gain.gain.cancelScheduledValues(at);
      voice.vibrato.gain.gain.setValueAtTime(0, at);
      voice.vibrato.oscillator.stop(voice.endTime);
    }
    for (const source of voice.sources) {
      try { source.stop(voice.endTime); } catch (_) {}
      const previousOnEnded = source.onended;
      source.onended = () => {
        if (previousOnEnded) previousOnEnded();
        disposeVoice(voice);
      };
    }
  }

  function makeTrackState() {
    return { sustain: false, bend: 0, mod: 0, voices: new Set(), pending: [] };
  }

  function applyController(context, state, voiceMap, track, type, value, at) {
    const voiceSet = state.voices;
    if (type === "sustain") {
      const on = value >= 0.5;
      if (state.sustain && !on) {
        for (const voice of state.pending) releaseVoice(voice, at);
        state.pending.length = 0;
      }
      state.sustain = on;
      return;
    }
    if (type === "bend") {
      state.bend = clamp(value, -1, 1, 0);
      for (const voice of voiceSet) for (const source of voice.sources) {
        if (source.detune && voice.endTime > at) source.detune.setValueAtTime(state.bend * 200, Math.max(at, voice.startTime));
      }
      return;
    }
    if (type === "modulation") {
      state.mod = clamp(value, 0, 1, 0);
      for (const voice of voiceSet) {
        if (voice.endTime <= at) continue;
        attachVibrato(context, voice, state.mod, Math.max(at, voice.startTime));
      }
    }
  }

  function releaseTrackState(context, state, track, at) {
    applyController(context, state, null, track, "sustain", 0, at);
    applyController(context, state, null, track, "bend", 0, at);
    applyController(context, state, null, track, "modulation", 0, at);
    for (const voice of state.voices) releaseVoice(voice, at);
    state.pending.length = 0;
  }

  function voiceReleaseSeconds(track, note) {
    let release = trackParams(track).release;
    for (const operator of operatorsFor(track, note.pitch) || []) release = Math.max(release, clamp(operator.r, 0, 30, 0));
    return release + 0.04;
  }

  function activeVoiceLimit(events, startTick, endTick, secPerTick = 0, repeated = null, terminalTick = endTick) {
    let timeline = events.filter(event => event.type === "controller" || event.type === "noteOn" || event.type === "noteOff");
    if (repeated && secPerTick > 0) {
      const span = endTick - startTick;
      const cycles = Math.ceil(repeated.maxReleaseSeconds / (span * secPerTick)) + 2;
      const perCycle = timeline.length + repeated.seeds.length;
      if (cycles * perCycle > 250000) return MAX_ACTIVE_NOTES + 1;
      const expanded = [];
      for (let cycle = 0; cycle < cycles; cycle++) {
        const offset = cycle * span;
        if (cycle > 0) for (const seed of repeated.seeds) expanded.push({ ...seed, tick: startTick + offset });
        for (const event of timeline) {
          if (cycle > 0 && event.tick < startTick) continue;
          const eventTick = event.type === "noteOff" && event.tick > endTick ? endTick : event.tick;
          expanded.push({ ...event, tick: eventTick + offset, key: `${event.key || ""}@${cycle}` });
        }
      }
      timeline = expanded;
      terminalTick = startTick + cycles * span;
    }
    timeline.sort((a, b) => a.tick - b.tick || ({ controller: 0, noteOff: 1, noteOn: 2 }[a.type] - ({ controller: 0, noteOff: 1, noteOn: 2 }[b.type])));
    const states = new Map();
    const notes = new Map();
    const active = [];
    let peak = 0;
    const trackState = id => {
      if (!states.has(id)) states.set(id, { sustain: false, pending: [] });
      return states.get(id);
    };
    const expire = at => {
      for (let index = active.length - 1; index >= 0; index--) if (active[index].releaseAt <= at) active.splice(index, 1);
    };
    const releasePending = (state, at) => {
      for (const voice of state.pending) voice.releaseAt = at + voice.releaseSeconds;
      state.pending.length = 0;
    };
    for (const event of timeline) {
      const at = (event.tick - startTick) * secPerTick;
      expire(at);
      if (event.type === "controller") {
        if (event.controller.type !== "sustain") continue;
        const state = trackState(event.track.id);
        const on = event.controller.value >= 0.5;
        if (state.sustain && !on) releasePending(state, at);
        state.sustain = on;
      } else if (event.type === "noteOn") {
        const voice = { releaseAt: Infinity, releaseSeconds: voiceReleaseSeconds(event.track, event.note) };
        active.push(voice);
        notes.set(event.key, { voice, state: trackState(event.track.id) });
        peak = Math.max(peak, active.length);
        if (peak > MAX_ACTIVE_NOTES) return peak;
      } else {
        const note = notes.get(event.key);
        if (!note) continue;
        notes.delete(event.key);
        if (note.state.sustain) note.state.pending.push(note.voice);
        else note.voice.releaseAt = at + note.voice.releaseSeconds;
      }
    }
    if (secPerTick > 0) {
      const at = (terminalTick - startTick) * secPerTick;
      expire(at);
      for (const state of states.values()) releasePending(state, at);
      for (const voice of active) voice.releaseAt = Math.min(voice.releaseAt, at + voice.releaseSeconds);
    }
    return peak;
  }

  function create() {
    let context = null;
    let mix = null;
    let buses = new Map();
    let currentInfo = null;
    let currentTickValue = 0;
    let playback = null;
    let resolvePlayback = null;
    let rejectPlayback = null;
    let playGeneration = 0;
    let retireBuses = false;
    const voices = new Set();
    const liveStates = new Map();

    function ensureContext() {
      if (context) return context;
      const Constructor = audioContextConstructor();
      if (!Constructor) throw codedError("SYNTH_STUDIO_AUDIO_UNAVAILABLE", "Web Audio is unavailable in this browser.");
      context = new Constructor();
      mix = makeMix(context);
      return context;
    }

    function busFor(track) {
      let bus = buses.get(track.id);
      if (!bus) {
        bus = createTrackBus(ensureContext(), mix, track);
        buses.set(track.id, bus);
      } else {
        updateTrackBus(bus, track);
      }
      return bus;
    }

    function disconnectBuses() {
      for (const bus of buses.values()) {
        for (const node of Object.values(bus)) try { node.disconnect(); } catch (_) {}
      }
      buses.clear();
      retireBuses = false;
    }

    function retireBusesIfIdle() {
      if (!retireBuses || !context) return;
      const now = context.currentTime;
      for (const voice of voices) if (voice.endTime > now) return;
      disconnectBuses();
    }

    function resolveTrack(trackValue) {
      if (trackValue && typeof trackValue === "object") {
        return normalizeProject({ tempo: 120, ppq: 480, tracks: [trackValue] }).tracks[0];
      }
      const id = String(trackValue || "");
      const existing = currentInfo?.tracks.find(track => track.id === id);
      return existing || {
        id: id || "audition",
        name: id || "Audition",
        instrument: "lead-saw",
        volume: 0.8,
        pan: 0,
        mute: false,
        solo: false,
        params: { ...DEFAULT_PARAMS },
        clips: []
      };
    }

    function voiceOn(track, pitch, velocity, at, state, liveInput = false) {
      if (!isAudible(track, Boolean(currentInfo?.tracks.some(item => item.solo)))) return null;
      let activeCount = 0;
      for (const voice of voices) if (voice.endTime > at) activeCount++;
      if (activeCount >= MAX_ACTIVE_NOTES) throw budgetError(`A maximum of ${MAX_ACTIVE_NOTES} voices can overlap.`);
      const operators = operatorsFor(track, pitch);
      const voice = createVoice(ensureContext(), mix, busFor(track), state, track, pitch, velocity, at, operators);
      voice.liveInput = liveInput;
      state.voices.add(voice);
      voices.add(voice);
      for (const source of voice.sources) {
        source.onended = () => {
          state.voices.delete(voice);
          voices.delete(voice);
          if (!voice.releaseRequested) releaseVoice(voice, context.currentTime);
          else disposeVoice(voice);
          retireBusesIfIdle();
        };
      }
      return voice;
    }

    function stateFor(track) {
      if (!liveStates.has(track.id)) liveStates.set(track.id, makeTrackState());
      return liveStates.get(track.id);
    }

    function clearVoices(at) {
      for (const voice of voices) {
        voice.releaseRequested = true;
        for (const source of voice.sources) {
          try {
            source.stop(at + 0.006);
          } catch (_) {}
        }
        if (voice.vibrato) try { voice.vibrato.oscillator.stop(at + 0.006); } catch (_) {}
        try { voice.voiceGain.gain.cancelScheduledValues(at); voice.voiceGain.gain.setValueAtTime(0, at); } catch (_) {}
      }
      voices.clear();
      if (playback) playback.states.clear();
    }

    function finishPlayback(tick, stopped, error) {
      if (!playback) return;
      const finished = playback;
      currentTickValue = tick;
      clearInterval(playback.timer);
      playback = null;
      if (stopped) {
        clearVoices(context?.currentTime || 0);
        finished.states.clear();
        disconnectBuses();
      } else {
        finished.states.clear();
        retireBuses = true;
        retireBusesIfIdle();
      }
      if (resolvePlayback || rejectPlayback) {
        const resolve = resolvePlayback;
        const reject = rejectPlayback;
        resolvePlayback = null;
        rejectPlayback = null;
        if (error && reject) reject(error);
        else if (resolve) resolve();
      }
    }

    async function unlock() {
      const ctx = ensureContext();
      if (ctx.state === "suspended") await ctx.resume();
      return true;
    }

    function play(project, startTick = 0, onTick, onEnd) {
      const info = normalizeProject(project);
      const requestGeneration = ++playGeneration;
      if (playback) finishPlayback(currentTick(), true);
      clearVoices(ensureContext().currentTime);
      disconnectBuses();
      currentInfo = info;
      const anySolo = info.tracks.some(track => track.solo);
      if (anySolo) for (const track of info.tracks) if (!isAudible(track, anySolo)) track.volume = 0;
      for (const track of info.tracks) busFor(track);
      return unlock().then(() => {
        if (requestGeneration !== playGeneration) return;
        return new Promise((resolve, reject) => {
        const all = allEvents(info);
        const secPerTick = 60 / info.tempo / info.ppq;
        const loop = info.loop.enabled;
        const loopStart = loop ? info.loop.start : 0;
        const loopEnd = loop ? info.loop.end : Math.max(info.endTick, 1);
        let tick = Math.max(0, Math.floor(clamp(startTick, 0, 1e9, 0)));
        if (loop) {
          if (tick < loopStart || tick >= loopEnd) tick = loopStart;
        }
        const liveRange = loop
          ? eventsForRange(all, loopStart, loopEnd, loopStart).map(event =>
            event.type === "noteOff" && event.tick > loopEnd ? { ...event, tick: loopEnd } : event)
          : eventsForRange(all, tick, Math.max(info.endTick, tick + 1), tick);
        const loopSeeds = loop ? loopControllerSeeds(all, info) : [];
        const preflightEvents = loop
          ? [...loopSeeds, ...liveRange]
          : [...seedControllers(all, info, tick), ...liveRange];
        const peak = activeVoiceLimit(
          preflightEvents,
          loop ? loopStart : tick,
          loop ? loopEnd : Math.max(info.endTick, tick + 1),
          secPerTick,
          loop ? { seeds: loopSeeds, maxReleaseSeconds: maxReleaseSeconds(info) } : null,
          loop ? loopEnd : Math.max(info.endTick, tick + 1)
        );
        if (peak > MAX_ACTIVE_NOTES) throw budgetError(`The arrangement has more than ${MAX_ACTIVE_NOTES} overlapping notes.`);
        resolvePlayback = resolve;
        rejectPlayback = reject;
        const startTime = context.currentTime + 0.045;
        const states = new Map(info.tracks.map(track => [track.id, makeTrackState()]));
        const tailSeconds = maxReleaseSeconds(info) + MAX_EFFECT_TAIL_SECONDS;
        const initial = seedControllers(all, info, tick);
        for (const event of initial) {
          const value = event.controller.value;
          applyController(context, states.get(event.track.id), null, event.track, event.controller.type, value, startTime);
        }
        const scheduleCycle = (baseTime, cycleStart, firstCycle, cycleIndex) => {
          const anchor = firstCycle ? cycleStart : loopStart;
          const cycleEvents = all.filter(event => {
            if (event.type === "noteOff") return event.ownerTick >= cycleStart && event.ownerTick < loopEnd;
            if (firstCycle) return event.tick >= cycleStart && event.tick < loopEnd;
            return event.tick >= loopStart && event.tick < loopEnd;
          }).map(event => {
            const eventTick = event.type === "noteOff" && event.tick > loopEnd ? loopEnd : event.tick;
            const key = event.type === "noteOn" || event.type === "noteOff" ? `${event.key}@${cycleIndex}` : event.key;
            return { ...event, key, at: baseTime + (eventTick - anchor) * secPerTick };
          });
          if (firstCycle) return cycleEvents;
          return [...loopSeeds.map(event => ({ ...event, at: baseTime })), ...cycleEvents];
        };
        let pending = [];
        let nextCycleTime = startTime;
        let endTime = startTime + Math.max(0, info.endTick - tick) * secPerTick;
        let cycleIndex = 1;
        if (loop) {
          const firstDuration = loopEnd - tick;
          pending = scheduleCycle(startTime, tick, true, 0);
          nextCycleTime = startTime + firstDuration * secPerTick;
        } else {
          pending = all.filter(event => event.tick >= tick).map(event => ({ ...event, at: startTime + (event.tick - tick) * secPerTick }));
          pending.push({ type: "end", tick: info.endTick, at: endTime });
        }
        const priority = { metronome: 0, controller: 1, noteOff: 2, noteOn: 3, end: 4 };
        pending.sort((a, b) => a.at - b.at || priority[a.type] - priority[b.type]);
        for (const track of info.tracks) if (isAudible(track, anySolo)) busFor(track);
        const stateKeys = new Map();
        const enqueue = (newEvents) => {
          pending.push(...newEvents);
          pending.sort((a, b) => a.at - b.at || priority[a.type] - priority[b.type]);
        };
        const process = event => {
          if (event.type === "metronome") {
            scheduleMetronome(context, mix, event.at, event.accent);
            return;
          }
          if (event.type === "end") {
            for (const track of info.tracks) releaseTrackState(context, states.get(track.id), track, event.at);
            return;
          }
          const state = states.get(event.track.id);
          if (event.type === "controller") {
            applyController(context, state, null, event.track, event.controller.type, event.controller.value, event.at);
            return;
          }
          if (event.type === "noteOn") {
            const voice = voiceOn(event.track, event.note.pitch, event.note.velocity, event.at, state);
            if (voice) stateKeys.set(event.key, { voice, track: event.track, state });
            return;
          }
          const record = stateKeys.get(event.key);
          if (!record) return;
          stateKeys.delete(event.key);
          if (record.state.sustain) record.state.pending.push(record.voice);
          else releaseVoice(record.voice, event.at);
        };
        const renderTick = () => {
          try {
            const now = context.currentTime;
            if (loop) {
              while (nextCycleTime <= now + LOOKAHEAD) {
                const baseTime = nextCycleTime;
                enqueue(scheduleCycle(baseTime, loopStart, false, cycleIndex++));
                nextCycleTime += (loopEnd - loopStart) * secPerTick;
              }
            }
            const horizon = now + LOOKAHEAD;
            while (pending.length && pending[0].at <= horizon) process(pending.shift());
            const elapsedTick = tick + Math.max(0, now - startTime) / secPerTick;
            let shownTick = elapsedTick;
            if (loop && elapsedTick >= loopEnd) shownTick = loopStart + ((elapsedTick - loopStart) % (loopEnd - loopStart));
            currentTickValue = shownTick;
            if (typeof onTick === "function") onTick(shownTick);
            if (!loop && now >= endTime + tailSeconds) {
              finishPlayback(info.endTick, false);
              if (typeof onEnd === "function") {
                try { onEnd(); } catch (_) {}
              }
            }
          } catch (error) {
            finishPlayback(currentTickValue, true, error);
          }
        };
        playback = { timer: setInterval(renderTick, TICK_MS), states, startTime, startTick: tick, loop, loopStart, loopEnd, secPerTick, endTime, tailSeconds };
        currentTickValue = tick;
        renderTick();
        });
      });
    }

    function stop() {
      playGeneration++;
      const tick = currentTick();
      if (playback) finishPlayback(tick, true);
      else {
        clearVoices(context?.currentTime || 0);
        disconnectBuses();
      }
      return tick;
    }

    function currentTick() {
      if (!playback || !context) return currentTickValue;
      const elapsed = Math.max(0, context.currentTime - playback.startTime) / playback.secPerTick;
      const raw = playback.startTick + elapsed;
      if (!playback.loop || raw < playback.loopEnd) return raw;
      return playback.loopStart + ((raw - playback.loopStart) % (playback.loopEnd - playback.loopStart));
    }

    function noteOn(trackValue, pitchValue, velocityValue) {
      const track = resolveTrack(trackValue);
      const pitch = Math.round(clamp(pitchValue, 0, 127, 60));
      const velocity = clamp(velocityValue, 0, 1, 0.8);
      if (!context) ensureContext();
      if (!buses.has(track.id)) busFor(track);
      const state = stateFor(track);
      const voice = voiceOn(track, pitch, velocity, context.currentTime + 0.004, state, true);
      let released = false;
      return function release() {
        if (released || !voice) return;
        released = true;
        const at = context.currentTime + 0.005;
        if (state.sustain) state.pending.push(voice);
        else releaseVoice(voice, at);
      };
    }

    function addController(trackValue, type, value) {
      if (!["sustain", "bend", "modulation"].includes(type)) return false;
      const track = resolveTrack(trackValue);
      const state = stateFor(track);
      if (!context) ensureContext();
      const normalized = type === "bend" ? clamp(value, -1, 1, 0) : clamp(value, 0, 1, 0);
      applyController(context, state, null, track, type, normalized, context.currentTime + 0.004);
      return true;
    }

    function panic(trackValue) {
      const targetId = trackValue && typeof trackValue === "object"
        ? String(trackValue.id || "")
        : (trackValue == null ? null : String(trackValue));
      const at = context ? context.currentTime + 0.004 : 0;
      for (const [id, state] of liveStates) {
        if (targetId !== null && id !== targetId) continue;
        for (const voice of state.voices) {
          if (voice.liveInput) releaseVoice(voice, at, 0.06);
        }
        state.pending = state.pending.filter(voice => {
          if (voice.liveInput) {
            releaseVoice(voice, at, 0.06);
            return false;
          }
          return true;
        });
        state.sustain = false;
        state.bend = 0;
        state.mod = 0;
        if (context) {
          const track = resolveTrack(id);
          applyController(context, state, null, track, "bend", 0, at);
          applyController(context, state, null, track, "modulation", 0, at);
        }
      }
      return true;
    }

    function dispose() {
      stop();
      if (context) {
        const ctx = context;
        context = null;
        mix = null;
        try { ctx.close(); } catch (_) {}
      }
    }

    return { unlock, play, stop, currentTick, noteOn, addController, panic, dispose };
  }

  function checkRenderBudget(info, events, startTick, endTick, durationSeconds, secPerTick, seedEvents = events) {
    if (info.noteCount > MAX_NOTES) throw budgetError(`A project can contain at most ${MAX_NOTES} notes.`);
    const seeds = seedControllers(seedEvents, info, startTick);
    const peak = activeVoiceLimit([...seeds, ...events], startTick, endTick, secPerTick, null, endTick);
    if (peak > MAX_ACTIVE_NOTES) throw budgetError(`The arrangement has more than ${MAX_ACTIVE_NOTES} overlapping notes.`);
    if (!Number.isFinite(durationSeconds) || durationSeconds > MAX_RENDER_SECONDS) {
      throw exportLimitError(`WAV rendering is limited to ${MAX_RENDER_SECONDS} seconds per export, including effect tail.`);
    }
  }

  function encodeWav(audioBuffer) {
    const channels = 2;
    const frames = audioBuffer.length;
    const bytes = new ArrayBuffer(44 + frames * channels * 2);
    const view = new DataView(bytes);
    const writeText = (offset, value) => {
      for (let i = 0; i < value.length; i++) view.setUint8(offset + i, value.charCodeAt(i));
    };
    writeText(0, "RIFF");
    view.setUint32(4, 36 + frames * channels * 2, true);
    writeText(8, "WAVE");
    writeText(12, "fmt ");
    view.setUint32(16, 16, true);
    view.setUint16(20, 1, true);
    view.setUint16(22, channels, true);
    view.setUint32(24, audioBuffer.sampleRate, true);
    view.setUint32(28, audioBuffer.sampleRate * channels * 2, true);
    view.setUint16(32, channels * 2, true);
    view.setUint16(34, 16, true);
    writeText(36, "data");
    view.setUint32(40, frames * channels * 2, true);
    const left = audioBuffer.getChannelData(0);
    const right = audioBuffer.getChannelData(1);
    let offset = 44;
    for (let i = 0; i < frames; i++) {
      for (const sample of [left[i], right[i]]) {
        const bounded = Math.max(-1, Math.min(1, sample || 0));
        view.setInt16(offset, bounded < 0 ? bounded * 32768 : bounded * 32767, true);
        offset += 2;
      }
    }
    return new Blob([bytes], { type: "audio/wav" });
  }

  async function renderWav(project) {
    const Constructor = offlineContextConstructor();
    if (!Constructor) throw codedError("SYNTH_STUDIO_AUDIO_UNAVAILABLE", "Offline Web Audio is unavailable in this browser.");
    const info = normalizeProject(project);
    const events = allEvents(info);
    const looping = info.loop.enabled;
    const startTick = looping ? info.loop.start : 0;
    const endTick = looping ? info.loop.end : Math.max(info.endTick, 1);
    const selected = eventsForRange(events, startTick, endTick, startTick).map(event =>
      looping && event.type === "noteOff" && event.tick > endTick ? { ...event, tick: endTick } : event);
    const rate = 44100;
    const secPerTick = 60 / info.tempo / info.ppq;
    let audibleEndTick = endTick;
    for (const event of selected) if (event.type === "noteOff") audibleEndTick = Math.max(audibleEndTick, event.tick);
    const tailSeconds = maxReleaseSeconds(info) + MAX_EFFECT_TAIL_SECONDS;
    const duration = Math.max(0.05, (audibleEndTick - startTick) * secPerTick) + tailSeconds;
    checkRenderBudget(info, selected, startTick, endTick, duration, secPerTick, events);
    const frames = Math.ceil(duration * rate);
    const context = new Constructor(2, frames, rate);
    const mix = makeMix(context);
    const anySolo = info.tracks.some(track => track.solo);
    const buses = new Map();
    const states = new Map();
    for (const track of info.tracks) {
      buses.set(track.id, createTrackBus(context, mix, track));
      states.set(track.id, makeTrackState());
    }
    const initial = seedControllers(events, info, startTick);
    const schedule = [
      ...initial.map(event => ({ ...event, tick: startTick })),
      ...selected,
      { type: "end", tick: audibleEndTick }
    ].sort((a, b) => a.tick - b.tick || ({ metronome: 0, controller: 1, noteOff: 2, noteOn: 3, end: 4 }[a.type] - ({ metronome: 0, controller: 1, noteOff: 2, noteOn: 3, end: 4 }[b.type])));
    const active = new Map();
    for (const event of schedule) {
      const at = Math.max(0, (event.tick - startTick) * secPerTick);
      if (event.type === "metronome") {
        scheduleMetronome(context, mix, at, event.accent);
        continue;
      }
      if (event.type === "end") {
        for (const track of info.tracks) releaseTrackState(context, states.get(track.id), track, at);
        continue;
      }
      const state = states.get(event.track.id);
      if (event.type === "controller") {
        applyController(context, state, null, event.track, event.controller.type, event.controller.value, at);
      } else if (event.type === "noteOn") {
        if (!isAudible(event.track, anySolo)) continue;
        const voice = createVoice(context, mix, buses.get(event.track.id), state, event.track, event.note.pitch, event.note.velocity, at, operatorsFor(event.track, event.note.pitch));
        state.voices.add(voice);
        active.set(event.key, { voice, state, track: event.track });
      } else {
        const record = active.get(event.key);
        if (!record) continue;
        active.delete(event.key);
        if (state.sustain) state.pending.push(record.voice);
        else releaseVoice(record.voice, at);
      }
    }
    const rendered = await context.startRendering();
    return encodeWav(rendered);
  }

  root.SynthStudioAudio = { create, renderWav };
})(typeof window !== "undefined" ? window : globalThis);
