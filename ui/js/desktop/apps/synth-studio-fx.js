(function (root) {
  "use strict";

  // Synth Studio effect rack: serial inserts, the shared send buses and the
  // master strip. Every builder works in a live AudioContext and in an
  // OfflineAudioContext, so WAV export renders exactly what playback does.
  // synth-studio-voices.js owns the per-track bus, synth-studio-audio.js owns
  // the project clock.

  const TINY = 0.0001;

  function clamp(value, low, high, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(high, Math.max(low, number)) : fallback;
  }

  function set(param, value, at, smooth) {
    if (!param) return;
    const time = Math.max(0, at ?? 0);
    try {
      if (smooth) param.setTargetAtTime(value, time, 0.02);
      else param.setValueAtTime(value, time);
    } catch (_) {
      try { param.value = value; } catch (__) {}
    }
  }

  function disconnect(node) {
    if (!node) return;
    try { node.disconnect(); } catch (_) {}
  }

  function stop(node) {
    if (!node || typeof node.stop !== "function") return;
    try { node.stop(); } catch (_) {}
  }

  // --- shared curve helpers -------------------------------------------------

  function bitCrushCurve(bits) {
    const size = 2048;
    const curve = new Float32Array(size);
    const levels = Math.max(2, Math.pow(2, Math.round(bits)));
    for (let i = 0; i < size; i++) {
      const x = (i / (size - 1)) * 2 - 1;
      curve[i] = Math.round(x * levels) / levels;
    }
    return curve;
  }

  function softClipCurve(drive) {
    const size = 2048;
    const curve = new Float32Array(size);
    const k = 1 + clamp(drive, 0, 1, 0.35) * 14;
    const norm = Math.tanh(k) || 1;
    for (let i = 0; i < size; i++) {
      const x = (i / (size - 1)) * 2 - 1;
      curve[i] = Math.tanh(x * k) / norm;
    }
    return curve;
  }

  const IMPULSE_CACHE = new WeakMap();

  function impulseFor(context, size, decay) {
    let cache = IMPULSE_CACHE.get(context);
    if (!cache) { cache = new Map(); IMPULSE_CACHE.set(context, cache); }
    const sizeKey = Math.round(clamp(size, 0, 1, 0.5) * 8) / 8;
    const decayKey = Math.round(clamp(decay, 0, 1, 0.6) * 8) / 8;
    const key = sizeKey + ':' + decayKey;
    if (cache.has(key)) return cache.get(key);
    const rate = context.sampleRate;
    const seconds = 0.35 + sizeKey * 3.4;
    const length = Math.max(1, Math.floor(rate * seconds));
    const buffer = context.createBuffer(2, length, rate);
    const reflections = [[0.011, 0.3, -0.24], [0.019, -0.25, 0.3], [0.031, 0.21, 0.17], [0.047, -0.17, -0.2], [0.063, 0.14, -0.13], [0.081, -0.11, 0.12]];
    let seed = 0x7f4a7c15;
    for (let channel = 0; channel < 2; channel++) {
      const data = buffer.getChannelData(channel);
      let low = 0;
      for (let i = 0; i < length; i++) {
        seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5;
        const noise = ((seed >>> 0) / 0x80000000) - 1;
        low = low * 0.6 + noise * 0.4;
        const envelope = Math.pow(1 - i / length, 1.4 + decayKey * 5.5);
        data[i] = low * envelope * 0.26;
      }
      for (const [sec, gainL, gainR] of reflections) {
        const index = Math.floor(sec * rate);
        if (index < length) data[index] += channel === 0 ? gainL : gainR;
      }
    }
    cache.set(key, buffer);
    return buffer;
  }

  // --- insert builders ------------------------------------------------------
  // Every insert exposes the same contract: input, output, update(spec, at,
  // smooth) and dispose(). The wet/dry blend is uniform, so `mix` always means
  // "how much of this processor reaches the output".
  //
  // The processor owns the blend: the dry path runs input -> dry -> output,
  // the wet path runs input -> wetIn -> the builder chain -> wet -> output.
  // The builder only has to route its own chain into `wet`.

  function makeProcessor(context) {
    const processor = {
      input: context.createGain(),
      output: context.createGain(),
      wetIn: context.createGain(),
      dry: context.createGain(),
      wet: context.createGain(),
      nodes: [],
      oscillators: []
    };
    processor.dry.gain.value = 1;
    processor.wet.gain.value = 0;
    processor.input.connect(processor.dry);
    processor.dry.connect(processor.output);
    processor.input.connect(processor.wetIn);
    processor.wet.connect(processor.output);
    return processor;
  }

  function createChorus(context) {
    const p = makeProcessor(context);
    const delayL = context.createDelay(0.08);
    const delayR = context.createDelay(0.08);
    delayL.delayTime.value = 0.014;
    delayR.delayTime.value = 0.019;
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 0.7;
    const depthL = context.createGain();
    const depthR = context.createGain();
    depthL.gain.value = 0.0026;
    depthR.gain.value = -0.0026;
    lfo.connect(depthL); lfo.connect(depthR);
    depthL.connect(delayL.delayTime);
    depthR.connect(delayR.delayTime);
    const panL = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    const panR = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    if (panL.pan) panL.pan.value = -0.75;
    if (panR.pan) panR.pan.value = 0.75;
    const feedback = context.createGain();
    feedback.gain.value = 0;
    const tone = context.createBiquadFilter();
    tone.type = "highpass";
    tone.frequency.value = 120;
    p.wetIn.connect(delayL);
    p.wetIn.connect(delayR);
    delayL.connect(panL); delayR.connect(panR);
    panL.connect(feedback); feedback.connect(delayR);
    panL.connect(tone); panR.connect(tone);
    tone.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(delayL, delayR, depthL, depthR, panL, panR, feedback, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.05, 8, 0.7), at, smooth);
      const depth = clamp(spec.depth, 0, 1, 0.35) * 0.008;
      set(depthL.gain, depth, at, smooth);
      set(depthR.gain, -depth, at, smooth);
      set(feedback.gain, clamp(spec.feedback, 0, 0.9, 0), at, smooth);
      set(tone.frequency, 120 + clamp(spec.tone, 0, 1, 0.3) * 900, at, smooth);
    };
    return p;
  }

  function createPhaser(context) {
    const p = makeProcessor(context);
    const stages = [680, 1180, 1920, 3100];
    const filters = stages.map(frequency => {
      const filter = context.createBiquadFilter();
      filter.type = "allpass";
      filter.frequency.value = frequency;
      filter.Q.value = 0.85;
      return filter;
    });
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 0.42;
    const depth = context.createGain();
    depth.gain.value = 650;
    lfo.connect(depth);
    for (const filter of filters) depth.connect(filter.detune);
    const feedback = context.createGain();
    feedback.gain.value = 0.35;
    // A cycle without a delay node is muted by the Web Audio graph, so the
    // feedback path carries a hair of delay to stay alive.
    const feedbackDelay = context.createDelay(0.05);
    feedbackDelay.delayTime.value = 0.0012;
    p.wetIn.connect(filters[0]);
    for (let i = 0; i < filters.length - 1; i++) filters[i].connect(filters[i + 1]);
    filters[filters.length - 1].connect(feedback);
    feedback.connect(feedbackDelay);
    feedbackDelay.connect(filters[0]);
    filters[filters.length - 1].connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(...filters, depth, feedback, feedbackDelay);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.05, 8, 0.42), at, smooth);
      set(depth.gain, clamp(spec.depth, 0, 1, 0.5) * 1400, at, smooth);
      set(feedback.gain, clamp(spec.feedback, 0, 0.9, 0.35), at, smooth);
      const base = 220 + clamp(spec.tone, 0, 1, 0.35) * 2600;
      filters.forEach((filter, index) => set(filter.frequency, base * (1 + index * 0.85), at, smooth));
    };
    return p;
  }

  function createTremolo(context) {
    const p = makeProcessor(context);
    const amp = context.createGain();
    amp.gain.value = 1;
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 4.5;
    const depth = context.createGain();
    depth.gain.value = 0.6;
    lfo.connect(depth);
    depth.connect(amp.gain);
    p.wetIn.connect(amp);
    amp.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(amp, depth);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.2, 20, 4.5), at, smooth);
      const amount = clamp(spec.depth, 0, 1, 0.6);
      set(depth.gain, amount, at, smooth);
      set(amp.gain, 1 - amount * 0.5, at, smooth);
    };
    return p;
  }

  function createFlanger(context) {
    const p = makeProcessor(context);
    const delay = context.createDelay(0.05);
    delay.delayTime.value = 0.006;
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 0.3;
    const depth = context.createGain();
    depth.gain.value = 0.003;
    lfo.connect(depth);
    depth.connect(delay.delayTime);
    const feedback = context.createGain();
    feedback.gain.value = 0.4;
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 4200;
    p.wetIn.connect(delay);
    delay.connect(tone);
    tone.connect(feedback);
    feedback.connect(delay);
    tone.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(delay, depth, feedback, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.02, 8, 0.3), at, smooth);
      set(depth.gain, clamp(spec.depth, 0, 1, 0.4) * 0.012, at, smooth);
      set(feedback.gain, clamp(spec.feedback, 0, 0.95, 0.4), at, smooth);
      set(tone.frequency, 900 + clamp(spec.tone, 0, 1, 0.5) * 9000, at, smooth);
    };
    return p;
  }

  function createRotary(context) {
    const p = makeProcessor(context);
    const amp = context.createGain();
    amp.gain.value = 1;
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 1.6;
    const tremoloDepth = context.createGain();
    tremoloDepth.gain.value = 0.5;
    const sweepDepth = context.createGain();
    sweepDepth.gain.value = 180;
    lfo.connect(tremoloDepth);
    lfo.connect(sweepDepth);
    tremoloDepth.connect(amp.gain);
    const horn = context.createBiquadFilter();
    horn.type = "bandpass";
    horn.frequency.value = 520;
    horn.Q.value = 1.4;
    sweepDepth.connect(horn.frequency);
    p.wetIn.connect(horn);
    horn.connect(amp);
    amp.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(amp, tremoloDepth, sweepDepth, horn);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.2, 12, 1.6), at, smooth);
      const depth = clamp(spec.depth, 0, 1, 0.55);
      set(tremoloDepth.gain, depth * 0.5, at, smooth);
      set(amp.gain, 1 - depth * 0.25, at, smooth);
      set(sweepDepth.gain, depth * 420, at, smooth);
      set(horn.frequency, 260 + clamp(spec.tone, 0, 1, 0.45) * 900, at, smooth);
    };
    return p;
  }

  function createBitcrush(context) {
    const p = makeProcessor(context);
    const shaper = context.createWaveShaper();
    shaper.oversample = "2x";
    shaper.curve = bitCrushCurve(6);
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 3200;
    p.wetIn.connect(shaper);
    shaper.connect(tone);
    tone.connect(p.wet);
    p.nodes.push(shaper, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const bits = clamp(Math.round(spec.bits), 2, 16, 6);
      if (shaper.curve?.__bits !== bits) { shaper.curve = bitCrushCurve(bits); shaper.curve.__bits = bits; }
      set(tone.frequency, 600 + clamp(spec.tone, 0, 1, 0.35) * 11000, at, smooth);
    };
    return p;
  }

  function createSoftclip(context) {
    const p = makeProcessor(context);
    const shaper = context.createWaveShaper();
    shaper.oversample = "2x";
    shaper.curve = softClipCurve(0.35);
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 8000;
    p.wetIn.connect(shaper);
    shaper.connect(tone);
    tone.connect(p.wet);
    p.nodes.push(shaper, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const drive = clamp(spec.drive, 0, 1, 0.35);
      if (shaper.curve?.__drive !== drive) { shaper.curve = softClipCurve(drive); shaper.curve.__drive = drive; }
      set(tone.frequency, 2200 + clamp(spec.tone, 0, 1, 0.6) * 12000, at, smooth);
    };
    return p;
  }

  function createEq3(context) {
    const p = makeProcessor(context);
    const low = context.createBiquadFilter();
    low.type = "lowshelf";
    low.frequency.value = 120;
    const mid = context.createBiquadFilter();
    mid.type = "peaking";
    mid.frequency.value = 1000;
    mid.Q.value = 0.9;
    const high = context.createBiquadFilter();
    high.type = "highshelf";
    high.frequency.value = 4000;
    p.wetIn.connect(low);
    low.connect(mid);
    mid.connect(high);
    high.connect(p.wet);
    p.nodes.push(low, mid, high);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 1);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(low.gain, clamp(spec.low, -18, 18, 0), at, smooth);
      set(mid.gain, clamp(spec.mid, -18, 18, 0), at, smooth);
      set(high.gain, clamp(spec.high, -18, 18, 0), at, smooth);
      set(mid.frequency, clamp(spec.freq, 200, 4000, 1000), at, smooth);
    };
    return p;
  }

  function createWidth(context) {
    const p = makeProcessor(context);
    const splitter = context.createChannelSplitter(2);
    const merger = context.createChannelMerger(2);
    const midGain = context.createGain();
    midGain.gain.value = 0.5;
    const sideGain = context.createGain();
    sideGain.gain.value = 0.5;
    const sideNeg = context.createGain();
    sideNeg.gain.value = -0.5;
    p.wetIn.connect(splitter);
    splitter.connect(midGain, 0);
    splitter.connect(midGain, 1);
    splitter.connect(sideGain, 0);
    splitter.connect(sideNeg, 1);
    midGain.connect(merger, 0, 0);
    midGain.connect(merger, 0, 1);
    sideGain.connect(merger, 0, 0);
    sideNeg.connect(merger, 0, 1);
    merger.connect(p.wet);
    p.nodes.push(splitter, merger, midGain, sideGain, sideNeg);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 1);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const width = clamp(spec.amount, 0, 2, 1);
      set(sideGain.gain, 0.5 * width, at, smooth);
      set(sideNeg.gain, -0.5 * width, at, smooth);
    };
    return p;
  }

  const CABINETS = [
    { band: 780, q: 0.7, roll: 4600, presence: 2600 },
    { band: 1350, q: 0.8, roll: 5800, presence: 3400 },
    { band: 130, q: 0.6, roll: 1400, presence: 900 },
    { band: 2300, q: 0.9, roll: 6600, presence: 4200 }
  ];

  function createCabinet(context) {
    const p = makeProcessor(context);
    const shaper = context.createWaveShaper();
    shaper.oversample = "2x";
    shaper.curve = softClipCurve(0.28);
    const band = context.createBiquadFilter();
    band.type = "peaking";
    band.frequency.value = 780;
    band.Q.value = 0.7;
    band.gain.value = 5;
    const presence = context.createBiquadFilter();
    presence.type = "peaking";
    presence.frequency.value = 2600;
    presence.Q.value = 1.1;
    presence.gain.value = 4;
    const roll = context.createBiquadFilter();
    roll.type = "lowpass";
    roll.frequency.value = 4600;
    p.wetIn.connect(shaper);
    shaper.connect(band);
    band.connect(presence);
    presence.connect(roll);
    roll.connect(p.wet);
    p.nodes.push(shaper, band, presence, roll);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const cabinet = CABINETS[clamp(Math.round(spec.model), 0, 3, 0)];
      set(band.frequency, cabinet.band, at, smooth);
      set(presence.frequency, cabinet.presence, at, smooth);
      set(roll.frequency, cabinet.roll * (0.5 + clamp(spec.tone, 0, 1, 0.5)), at, smooth);
    };
    return p;
  }

  function createDelay(context) {
    const p = makeProcessor(context);
    const delay = context.createDelay(1.6);
    delay.delayTime.value = 0.24;
    const feedback = context.createGain();
    feedback.gain.value = 0.25;
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 3200;
    p.wetIn.connect(delay);
    delay.connect(tone);
    tone.connect(feedback);
    feedback.connect(delay);
    tone.connect(p.wet);
    p.nodes.push(delay, feedback, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(delay.delayTime, clamp(spec.time, 0.01, 1.5, 0.24), at, smooth);
      set(feedback.gain, clamp(spec.feedback, 0, 0.9, 0.25), at, smooth);
      set(tone.frequency, 700 + clamp(spec.tone, 0, 1, 0.45) * 9000, at, smooth);
    };
    return p;
  }

  function createReverb(context) {
    const p = makeProcessor(context);
    const convolver = context.createConvolver();
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 7000;
    p.wetIn.connect(convolver);
    convolver.connect(tone);
    tone.connect(p.wet);
    p.nodes.push(convolver, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const buffer = impulseFor(context, spec.size, spec.decay);
      if (convolver.buffer !== buffer) convolver.buffer = buffer;
      set(tone.frequency, 1200 + clamp(spec.tone, 0, 1, 0.5) * 13000, at, smooth);
    };
    return p;
  }

  const BUILDERS = {
    chorus: createChorus,
    phaser: createPhaser,
    tremolo: createTremolo,
    flanger: createFlanger,
    rotary: createRotary,
    bitcrush: createBitcrush,
    softclip: createSoftclip,
    eq3: createEq3,
    width: createWidth,
    cabinet: createCabinet,
    delay: createDelay,
    reverb: createReverb
  };

  function createInsert(context, spec) {
    const builder = BUILDERS[spec.type];
    if (!builder) return null;
    const processor = builder(context);
    processor.type = spec.type;
    processor.update(spec, 0, false);
    const baseDispose = processor.dispose;
    processor.dispose = () => {
      if (baseDispose) baseDispose();
      for (const oscillator of processor.oscillators) stop(oscillator);
      for (const node of processor.nodes) disconnect(node);
      disconnect(processor.input);
      disconnect(processor.output);
      disconnect(processor.wetIn);
      disconnect(processor.dry);
      disconnect(processor.wet);
    };
    return processor;
  }

  // --- master strip ---------------------------------------------------------

  const MASTER_DEFAULTS = {
    volume: 0.78,
    low: 0,
    mid: 0,
    high: 0,
    threshold: -8,
    ratio: 10,
    knee: 8,
    attack: 0.003,
    release: 0.18,
    width: 1,
    plateSize: 0.5,
    plateDecay: 0.65,
    echoTime: 0.375,
    echoFeedback: 0.28,
    echoSync: "1/8"
  };

  const SYNC_DIVISIONS = {
    "1/4": 1,
    "1/8": 0.5,
    "1/8T": 1 / 3,
    "1/16": 0.25,
    "1/16T": 1 / 6
  };

  function syncSeconds(division, secondsPerBeat, fallback) {
    const factor = SYNC_DIVISIONS[division];
    if (!factor || !Number.isFinite(secondsPerBeat) || secondsPerBeat <= 0) return fallback;
    return secondsPerBeat * factor;
  }

  function createMaster(context, spec) {
    const settings = { ...MASTER_DEFAULTS, ...(spec || {}) };
    const input = context.createGain();
    const gain = context.createGain();
    const low = context.createBiquadFilter();
    low.type = "lowshelf";
    low.frequency.value = 110;
    const mid = context.createBiquadFilter();
    mid.type = "peaking";
    mid.frequency.value = 1000;
    mid.Q.value = 0.9;
    const high = context.createBiquadFilter();
    high.type = "highshelf";
    high.frequency.value = 5000;
    const compressor = context.createDynamicsCompressor();
    const limiter = context.createWaveShaper();
    limiter.oversample = "2x";
    limiter.curve = softClipCurve(0.12);
    const width = context.createGain();
    const splitter = context.createChannelSplitter(2);
    const merger = context.createChannelMerger(2);
    const midGain = context.createGain();
    midGain.gain.value = 0.5;
    const sideGain = context.createGain();
    sideGain.gain.value = 0.5;
    const sideNeg = context.createGain();
    sideNeg.gain.value = -0.5;
    const analyser = typeof context.createAnalyser === "function" ? context.createAnalyser() : null;
    if (analyser) { analyser.fftSize = 2048; analyser.smoothingTimeConstant = 0.75; }

    input.connect(gain);
    gain.connect(low);
    low.connect(mid);
    mid.connect(high);
    high.connect(compressor);
    compressor.connect(limiter);
    limiter.connect(splitter);
    splitter.connect(sideGain, 0);
    splitter.connect(sideNeg, 1);
    splitter.connect(midGain, 0);
    splitter.connect(midGain, 1);
    midGain.connect(merger, 0, 0);
    midGain.connect(merger, 0, 1);
    sideGain.connect(merger, 0, 0);
    sideNeg.connect(merger, 0, 1);
    merger.connect(width);
    if (analyser) { width.connect(analyser); analyser.connect(context.destination); }
    else width.connect(context.destination);

    const master = {
      input,
      output: analyser || width,
      gain,
      low,
      mid,
      high,
      compressor,
      limiter,
      midGain,
      sideGain,
      sideNeg,
      analyser,
      settings,
      update(next, at, smooth) {
        Object.assign(settings, next || {});
        set(gain.gain, clamp(settings.volume, 0, 1.5, 0.78), at, smooth);
        set(low.gain, clamp(settings.low, -18, 18, 0), at, smooth);
        set(mid.gain, clamp(settings.mid, -18, 18, 0), at, smooth);
        set(high.gain, clamp(settings.high, -18, 18, 0), at, smooth);
        set(compressor.threshold, clamp(settings.threshold, -60, 0, -8), at, smooth);
        set(compressor.ratio, clamp(settings.ratio, 1, 20, 10), at, smooth);
        set(compressor.knee, clamp(settings.knee, 0, 40, 8), at, smooth);
        set(compressor.attack, clamp(settings.attack, 0, 1, 0.003), at, smooth);
        set(compressor.release, clamp(settings.release, 0, 1, 0.18), at, smooth);
        const amount = clamp(settings.width, 0, 2, 1);
        set(sideGain.gain, 0.5 * amount, at, smooth);
        set(sideNeg.gain, -0.5 * amount, at, smooth);
      }
    };
    master.update(settings, 0, false);
    return master;
  }

  // --- shared send buses ----------------------------------------------------

  function createSends(context, spec, secondsPerBeat) {
    const settings = { ...MASTER_DEFAULTS, ...(spec || {}) };
    const reverbIn = context.createGain();
    const delayIn = context.createGain();
    const plate = context.createConvolver();
    const plateTone = context.createBiquadFilter();
    plateTone.type = "lowpass";
    plateTone.frequency.value = 8000;
    const plateGain = context.createGain();
    const echo = context.createDelay(2.5);
    const echoTone = context.createBiquadFilter();
    echoTone.type = "lowpass";
    echoTone.frequency.value = 3200;
    const echoFeedback = context.createGain();
    const echoGain = context.createGain();

    reverbIn.connect(plate);
    plate.connect(plateTone);
    plateTone.connect(plateGain);
    delayIn.connect(echo);
    echo.connect(echoTone);
    echoTone.connect(echoFeedback);
    echoFeedback.connect(echo);
    echoTone.connect(echoGain);

    const sends = {
      reverbIn,
      delayIn,
      plate,
      plateTone,
      plateGain,
      echo,
      echoTone,
      echoFeedback,
      echoGain,
      settings,
      update(next, at, smooth, beatSeconds) {
        Object.assign(settings, next || {});
        const buffer = impulseFor(context, settings.plateSize, settings.plateDecay);
        if (plate.buffer !== buffer) plate.buffer = buffer;
        set(plateTone.frequency, 1500 + clamp(settings.plateDecay, 0, 1, 0.65) * 10000, at, smooth);
        const time = syncSeconds(settings.echoSync, beatSeconds, clamp(settings.echoTime, 0.01, 2, 0.375));
        set(echo.delayTime, time, at, smooth);
        set(echoFeedback.gain, clamp(settings.echoFeedback, 0, 0.85, 0.28), at, smooth);
        set(echoTone.frequency, 900 + clamp(settings.echoFeedback, 0, 0.85, 0.28) * 9000, at, smooth);
      }
    };
    sends.update(settings, 0, false, secondsPerBeat);
    return sends;
  }

  root.SynthStudioFX = {
    MASTER_DEFAULTS,
    SYNC_DIVISIONS,
    types: () => Object.keys(BUILDERS),
    createInsert,
    createMaster,
    createSends,
    syncSeconds
  };
})(typeof window !== "undefined" ? window : globalThis);
