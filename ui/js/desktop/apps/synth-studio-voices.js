(function (root) {
  "use strict";

  // Synth Studio voice engine: patch resolution, the per-track mix graph,
  // voice envelopes and controllers. synth-studio-audio.js drives these same
  // nodes for live playback and offline WAV rendering.

  const TINY = 0.0001;
  const DEFAULT_PARAMS = { tone: 0.5, attack: 0.01, release: 0.3, reverb: 0, delay: 0 };
  const GM = root.SynthStudioGMData;
  const PRESETS = root.SynthStudioPresets;

  function clamp(value, low, high, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(high, Math.max(low, number)) : fallback;
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

  root.SynthStudioVoices = {
    DEFAULT_PARAMS,
    clamp,
    operatorsFor,
    voiceReleaseSeconds,
    makeMix,
    createTrackBus,
    updateTrackBus,
    scheduleMetronome,
    createVoice,
    disposeVoice,
    releaseVoice,
    makeTrackState,
    applyController,
    releaseTrackState
  };
})(typeof window !== "undefined" ? window : globalThis);
