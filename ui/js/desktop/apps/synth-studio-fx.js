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

  // Smooth full-wave rectifier for octave-up shimmer harmonics; exact zero at
  // silence so idle tracks never inject DC or noise into the reverb hall.
  const RECTIFY_CURVE = (() => {
    const size = 2048;
    const curve = new Float32Array(size);
    const offset = Math.sqrt(0.0004);
    for (let i = 0; i < size; i++) {
      const x = (i / (size - 1)) * 2 - 1;
      curve[i] = Math.sqrt(x * x + 0.0004) - offset;
    }
    return curve;
  })();

  // West-Coast multi-stage sine wavefolder. Symmetry shifts the folding bias to
  // sweep between pure odd harmonics (0.5) and rich even harmonics while keeping
  // exact zero output at x = 0.
  function wavefoldCurve(drive, symmetry) {
    const size = 2048;
    const curve = new Float32Array(size);
    const folds = 1 + clamp(drive, 0, 1, 0.45) * 6;
    const bias = (clamp(symmetry, 0, 1, 0.5) - 0.5) * 0.9;
    const scale = folds * Math.PI * 0.5;
    const zero = Math.sin(bias * scale);
    for (let i = 0; i < size; i++) {
      const x = (i / (size - 1)) * 2 - 1;
      curve[i] = Math.max(-1, Math.min(1, (Math.sin((x + bias) * scale) - zero) * 0.85));
    }
    return curve;
  }

  // Vocal tract formant frequencies [F1, F2, F3] in Hz across A -> E -> I -> O -> U.
  const VOWEL_FORMANTS = [
    [730, 1090, 2440],
    [530, 1840, 2480],
    [270, 2290, 3010],
    [570, 840, 2410],
    [300, 870, 2240]
  ];

  function vowelFormants(position) {
    const scaled = clamp(position, 0, 1, 0.25) * (VOWEL_FORMANTS.length - 1);
    const low = Math.floor(scaled);
    const high = Math.min(VOWEL_FORMANTS.length - 1, low + 1);
    const frac = scaled - low;
    return [
      VOWEL_FORMANTS[low][0] + (VOWEL_FORMANTS[high][0] - VOWEL_FORMANTS[low][0]) * frac,
      VOWEL_FORMANTS[low][1] + (VOWEL_FORMANTS[high][1] - VOWEL_FORMANTS[low][1]) * frac,
      VOWEL_FORMANTS[low][2] + (VOWEL_FORMANTS[high][2] - VOWEL_FORMANTS[low][2]) * frac
    ];
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

  // Mid/side width matrix: M = (L+R)/2, S = (L-R)/2 * amount, L' = M + S and
  // R' = M - S. Amount 1 passes stereo unchanged, 0 folds to mono and 2 widens
  // hard-panned material. Both channels receive the same side signal, so a
  // centred source never shifts to one side and never gains level.
  function createWidthMatrix(context) {
    const splitter = context.createChannelSplitter(2);
    const merger = context.createChannelMerger(2);
    const midGain = context.createGain();
    midGain.gain.value = 0.5;
    const sideLeft = context.createGain();
    const sideRight = context.createGain();
    const sideSum = context.createGain();
    const sideInvert = context.createGain();
    sideInvert.gain.value = -1;
    splitter.connect(midGain, 0);
    splitter.connect(midGain, 1);
    splitter.connect(sideLeft, 0);
    splitter.connect(sideRight, 1);
    sideLeft.connect(sideSum);
    sideRight.connect(sideSum);
    midGain.connect(merger, 0, 0);
    midGain.connect(merger, 0, 1);
    sideSum.connect(merger, 0, 0);
    sideSum.connect(sideInvert);
    sideInvert.connect(merger, 0, 1);
    return {
      input: splitter,
      output: merger,
      sideLeft,
      sideRight,
      nodes: [splitter, merger, midGain, sideLeft, sideRight, sideSum, sideInvert],
      update(amount, at, smooth) {
        set(sideLeft.gain, 0.5 * amount, at, smooth);
        set(sideRight.gain, -0.5 * amount, at, smooth);
      }
    };
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
    const matrix = createWidthMatrix(context);
    p.wetIn.connect(matrix.input);
    matrix.output.connect(p.wet);
    p.nodes.push(...matrix.nodes);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 1);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      matrix.update(clamp(spec.amount, 0, 2, 1), at, smooth);
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

  // Tape: a short, modulated delay adds wow and flutter, then a soft clipper
  // and a low pass round off the repeats like a worn cassette.
  function createTape(context) {
    const p = makeProcessor(context);
    const delay = context.createDelay(0.05);
    delay.delayTime.value = 0.012;
    const wow = context.createOscillator();
    wow.type = "sine";
    wow.frequency.value = 0.6;
    const flutter = context.createOscillator();
    flutter.type = "triangle";
    flutter.frequency.value = 6.2;
    const wowDepth = context.createGain();
    wowDepth.gain.value = 0.0016;
    const flutterDepth = context.createGain();
    flutterDepth.gain.value = 0.0004;
    wow.connect(wowDepth); wowDepth.connect(delay.delayTime);
    flutter.connect(flutterDepth); flutterDepth.connect(delay.delayTime);
    const shaper = context.createWaveShaper();
    shaper.oversample = "2x";
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 3800;
    tone.Q.value = 0.5;
    p.wetIn.connect(delay);
    delay.connect(shaper);
    shaper.connect(tone);
    tone.connect(p.wet);
    wow.start(0);
    flutter.start(0);
    p.oscillators.push(wow, flutter);
    p.nodes.push(delay, wowDepth, flutterDepth, shaper, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.35);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const rate = clamp(spec.rate, 0.1, 8, 0.6);
      const depth = clamp(spec.depth, 0, 1, 0.4);
      set(wow.frequency, rate, at, smooth);
      set(flutter.frequency, clamp(rate * 9, 5, 18, 6), at, smooth);
      set(wowDepth.gain, depth * 0.0035, at, smooth);
      set(flutterDepth.gain, depth * 0.0007, at, smooth);
      const drive = clamp(spec.drive, 0, 1, 0.25);
      if (shaper.__drive !== drive) {
        shaper.curve = softClipCurve(drive);
        shaper.__drive = drive;
      }
      set(tone.frequency, 1200 + clamp(spec.tone, 0, 1, 0.4) * 9000, at, smooth);
    };
    return p;
  }

  function createAutopan(context) {
    const p = makeProcessor(context);
    const panner = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 0.8;
    const depth = context.createGain();
    depth.gain.value = 0.7;
    if (panner.pan) {
      lfo.connect(depth);
      depth.connect(panner.pan);
    }
    p.wetIn.connect(panner);
    panner.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(panner, depth);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.5);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.05, 12, 0.8), at, smooth);
      set(depth.gain, clamp(spec.depth, 0, 1, 0.7), at, smooth);
    };
    return p;
  }

  // Ring mod blends the dry signal into a carrier-modulated copy, so depth 0
  // passes the signal and depth 1 is the classic metallic product.
  function createRingmod(context) {
    const p = makeProcessor(context);
    const carrier = context.createOscillator();
    carrier.type = "sine";
    carrier.frequency.value = 440;
    const carrierDepth = context.createGain();
    carrierDepth.gain.value = 0.6;
    const ring = context.createGain();
    ring.gain.value = 0.4;
    carrier.connect(carrierDepth);
    carrierDepth.connect(ring.gain);
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 6000;
    p.wetIn.connect(ring);
    ring.connect(tone);
    tone.connect(p.wet);
    carrier.start(0);
    p.oscillators.push(carrier);
    p.nodes.push(carrierDepth, ring, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.35);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(carrier.frequency, clamp(spec.freq, 20, 4000, 440), at, smooth);
      const depth = clamp(spec.depth, 0, 1, 0.6);
      set(ring.gain, 1 - depth, at, smooth);
      set(carrierDepth.gain, depth, at, smooth);
      set(tone.frequency, 800 + clamp(spec.tone, 0, 1, 0.5) * 8000, at, smooth);
    };
    return p;
  }

  // Shimmer reverb: pairs a diffuse stereo hall with a full-wave-rectified
  // octave-up harmonic halo through a slowly drift-modulated cathedral tail.
  function createShimmer(context) {
    const p = makeProcessor(context);
    const hall = context.createConvolver();
    const hallGain = context.createGain();
    hallGain.gain.value = 0.85;
    const octShaper = context.createWaveShaper();
    octShaper.oversample = "2x";
    octShaper.curve = RECTIFY_CURVE;
    const octHp = context.createBiquadFilter();
    octHp.type = "highpass";
    octHp.frequency.value = 260;
    const driftDelay = context.createDelay(0.06);
    driftDelay.delayTime.value = 0.018;
    const driftLfo = context.createOscillator();
    driftLfo.type = "sine";
    driftLfo.frequency.value = 0.35;
    const driftDepth = context.createGain();
    driftDepth.gain.value = 0.0022;
    driftLfo.connect(driftDepth);
    driftDepth.connect(driftDelay.delayTime);
    const sheen = context.createConvolver();
    const sheenGain = context.createGain();
    sheenGain.gain.value = 0.48;
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 9500;

    p.wetIn.connect(hall);
    hall.connect(hallGain);
    hallGain.connect(tone);
    p.wetIn.connect(octShaper);
    octShaper.connect(octHp);
    octHp.connect(driftDelay);
    driftDelay.connect(sheen);
    sheen.connect(sheenGain);
    sheenGain.connect(tone);
    tone.connect(p.wet);
    driftLfo.start(0);
    p.oscillators.push(driftLfo);
    p.nodes.push(hall, hallGain, octShaper, octHp, driftDelay, driftDepth, sheen, sheenGain, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.35);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const size = clamp(spec.size, 0, 1, 0.7);
      const decay = clamp(spec.decay, 0, 1, 0.75);
      const hallBuf = impulseFor(context, size, decay);
      if (hall.buffer !== hallBuf) hall.buffer = hallBuf;
      const sheenBuf = impulseFor(context, Math.min(1, size * 0.8 + 0.2), Math.max(0, decay * 0.55));
      if (sheen.buffer !== sheenBuf) sheen.buffer = sheenBuf;
      const amount = clamp(spec.shimmer, 0, 1, 0.55);
      set(hallGain.gain, 1 - amount * 0.28, at, smooth);
      set(sheenGain.gain, amount * 0.9, at, smooth);
      set(tone.frequency, 1400 + clamp(spec.tone, 0, 1, 0.65) * 14000, at, smooth);
    };
    return p;
  }

  // True stereo ping-pong delay: bounces damped echoes left -> right -> left.
  function createPingpong(context) {
    const p = makeProcessor(context);
    const hp = context.createBiquadFilter();
    hp.type = "highpass";
    hp.frequency.value = 110;
    const delayL = context.createDelay(1.6);
    const delayR = context.createDelay(1.6);
    delayL.delayTime.value = 0.28;
    delayR.delayTime.value = 0.28;
    const toneL = context.createBiquadFilter();
    const toneR = context.createBiquadFilter();
    toneL.type = "lowpass";
    toneR.type = "lowpass";
    toneL.frequency.value = 4800;
    toneR.frequency.value = 4800;
    const panL = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    const panR = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    if (panL.pan) panL.pan.value = -0.85;
    if (panR.pan) panR.pan.value = 0.85;
    const feedback = context.createGain();
    feedback.gain.value = 0.42;

    p.wetIn.connect(hp);
    hp.connect(delayL);
    delayL.connect(toneL);
    toneL.connect(panL);
    panL.connect(p.wet);
    toneL.connect(delayR);
    delayR.connect(toneR);
    toneR.connect(panR);
    panR.connect(p.wet);
    toneR.connect(feedback);
    feedback.connect(delayL);
    p.nodes.push(hp, delayL, delayR, toneL, toneR, panL, panR, feedback);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.35);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const time = clamp(spec.time, 0.04, 1.2, 0.28);
      set(delayL.delayTime, time, at, smooth);
      set(delayR.delayTime, time, at, smooth);
      set(feedback.gain, clamp(spec.feedback, 0, 0.88, 0.42), at, smooth);
      const spread = clamp(spec.spread, 0, 1, 0.85);
      if (panL.pan) set(panL.pan, -spread, at, smooth);
      if (panR.pan) set(panR.pan, spread, at, smooth);
      const cutoff = 700 + clamp(spec.tone, 0, 1, 0.55) * 10500;
      set(toneL.frequency, cutoff, at, smooth);
      set(toneR.frequency, cutoff, at, smooth);
    };
    return p;
  }

  // West-Coast wavefolder: folds peaks inward for evolving Buchla-like harmonics.
  function createWavefold(context) {
    const p = makeProcessor(context);
    const shaper = context.createWaveShaper();
    shaper.oversample = "2x";
    shaper.curve = wavefoldCurve(0.45, 0.5);
    const dcBlock = context.createBiquadFilter();
    dcBlock.type = "highpass";
    dcBlock.frequency.value = 24;
    const tone = context.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 7200;
    p.wetIn.connect(shaper);
    shaper.connect(dcBlock);
    dcBlock.connect(tone);
    tone.connect(p.wet);
    p.nodes.push(shaper, dcBlock, tone);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.55);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const drive = clamp(spec.drive, 0, 1, 0.45);
      const symmetry = clamp(spec.symmetry, 0, 1, 0.5);
      const key = drive.toFixed(3) + ":" + symmetry.toFixed(3);
      if (shaper.__key !== key) {
        shaper.curve = wavefoldCurve(drive, symmetry);
        shaper.__key = key;
      }
      set(tone.frequency, 900 + clamp(spec.tone, 0, 1, 0.65) * 13000, at, smooth);
    };
    return p;
  }

  // Three-band parallel vocal formant filter morphing across A -> E -> I -> O -> U.
  function createFormant(context) {
    const p = makeProcessor(context);
    const weights = [1.0, 0.8, 0.55];
    const spans = [180, 460, 280];
    const makeup = context.createGain();
    makeup.gain.value = 1.6;
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 0.6;
    const filters = [];
    const mods = [];
    for (let i = 0; i < 3; i++) {
      const filter = context.createBiquadFilter();
      filter.type = "bandpass";
      filter.frequency.value = VOWEL_FORMANTS[1][i];
      filter.Q.value = 8;
      const gain = context.createGain();
      gain.gain.value = weights[i];
      const mod = context.createGain();
      mod.gain.value = spans[i] * 0.3;
      lfo.connect(mod);
      mod.connect(filter.frequency);
      p.wetIn.connect(filter);
      filter.connect(gain);
      gain.connect(makeup);
      filters.push(filter);
      mods.push(mod);
      p.nodes.push(filter, gain, mod);
    }
    makeup.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(makeup);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.7);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const freqs = vowelFormants(spec.vowel);
      const q = 2.5 + clamp(spec.resonance, 0, 1, 0.55) * 11.5;
      const depth = clamp(spec.depth, 0, 1, 0.3);
      set(lfo.frequency, clamp(spec.rate, 0.05, 8, 0.6), at, smooth);
      for (let i = 0; i < 3; i++) {
        set(filters[i].frequency, freqs[i], at, smooth);
        set(filters[i].Q, q, at, smooth);
        set(mods[i].gain, depth * spans[i], at, smooth);
      }
    };
    return p;
  }

  // Tuned Karplus-Strong comb resonator: turns transients and synths into
  // pitched metallic tubes, strings and cyberpunk resonant bodies.
  function createResonator(context) {
    const p = makeProcessor(context);
    const delay = context.createDelay(0.05);
    delay.delayTime.value = 1 / 220;
    const damp = context.createBiquadFilter();
    damp.type = "lowpass";
    damp.frequency.value = 4500;
    const feedback = context.createGain();
    feedback.gain.value = 0.72;
    const outGain = context.createGain();
    outGain.gain.value = 0.55;
    p.wetIn.connect(delay);
    delay.connect(damp);
    damp.connect(feedback);
    feedback.connect(delay);
    damp.connect(outGain);
    outGain.connect(p.wet);
    p.nodes.push(delay, damp, feedback, outGain);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.45);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      const freq = clamp(spec.freq, 40, 1000, 220);
      set(delay.delayTime, clamp(1 / freq, 0.001, 0.025, 1 / 220), at, smooth);
      set(feedback.gain, clamp(spec.feedback, 0, 0.92, 0.72), at, smooth);
      set(damp.frequency, 600 + clamp(spec.tone, 0, 1, 0.55) * 11500, at, smooth);
    };
    return p;
  }

  // Resonant LFO auto-filter: modulates filter detune in logarithmic octaves so
  // wobble sweeps stay musical across the whole cutoff range.
  function createAutofilter(context) {
    const p = makeProcessor(context);
    const filter = context.createBiquadFilter();
    filter.type = "lowpass";
    filter.frequency.value = 900;
    filter.Q.value = 6.5;
    const lfo = context.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 1.8;
    const depth = context.createGain();
    depth.gain.value = 1950;
    lfo.connect(depth);
    depth.connect(filter.detune);
    p.wetIn.connect(filter);
    filter.connect(p.wet);
    lfo.start(0);
    p.oscillators.push(lfo);
    p.nodes.push(filter, depth);
    p.update = (spec, at, smooth) => {
      const mix = clamp(spec.mix, 0, 1, 0.75);
      set(p.dry.gain, 1 - mix, at, smooth);
      set(p.wet.gain, mix, at, smooth);
      set(lfo.frequency, clamp(spec.rate, 0.08, 16, 1.8), at, smooth);
      set(depth.gain, clamp(spec.depth, 0, 1, 0.65) * 3000, at, smooth);
      set(filter.frequency, clamp(spec.freq, 120, 6000, 900), at, smooth);
      set(filter.Q, 0.8 + clamp(spec.resonance, 0, 1, 0.5) * 13.2, at, smooth);
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
    reverb: createReverb,
    tape: createTape,
    autopan: createAutopan,
    ringmod: createRingmod,
    shimmer: createShimmer,
    pingpong: createPingpong,
    wavefold: createWavefold,
    formant: createFormant,
    resonator: createResonator,
    autofilter: createAutofilter
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

  // Live playback and WAV export share the master strip, so both use the same
  // -0.3 dBFS ceiling. The limiter stays transparent below its knee; only peaks
  // that would otherwise reach the PCM rails are eased down to the ceiling.
  const MASTER_CEILING = 0.966;
  const LIMITER_HEADROOM = 2;
  const LIMITER_KNEE = 0.7;

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

  // A WaveShaper only sees [-1, 1], so the gains around it stretch that domain
  // to LIMITER_HEADROOM times full scale. Below the knee the curve is exactly
  // linear; above it a tanh shoulder approaches MASTER_CEILING with a continuous
  // slope, so the onset of limiting is not a distortion step of its own. Inputs
  // beyond the domain land on the ceiling instead of hard-clipping at +/-1.
  function limiterCurve() {
    const size = 4097;
    const curve = new Float32Array(size);
    const knee = LIMITER_KNEE / LIMITER_HEADROOM;
    const ceiling = MASTER_CEILING / LIMITER_HEADROOM;
    const span = 1 - knee;
    // Keep the transfer's unity slope at the knee:
    // shape / tanh(shape) = span / (ceiling - knee).
    const target = span / (ceiling - knee);
    let low = 1e-6, high = 40;
    for (let iteration = 0; iteration < 60; iteration++) {
      const middle = (low + high) / 2;
      if (middle / Math.tanh(middle) < target) low = middle;
      else high = middle;
    }
    const shape = (low + high) / 2;
    const norm = Math.tanh(shape);
    for (let i = 0; i < size; i++) {
      const x = (i / (size - 1)) * 2 - 1;
      const magnitude = Math.abs(x);
      const limited = magnitude <= knee
        ? magnitude
        : knee + (ceiling - knee) * Math.tanh(((magnitude - knee) / span) * shape) / norm;
      curve[i] = (x < 0 ? -1 : 1) * limited;
    }
    return curve;
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
    const limiterPre = context.createGain();
    limiterPre.gain.value = 1 / LIMITER_HEADROOM;
    const limiter = context.createWaveShaper();
    limiter.oversample = "2x";
    limiter.curve = limiterCurve();
    const limiterPost = context.createGain();
    limiterPost.gain.value = LIMITER_HEADROOM;
    const width = context.createGain();
    const widthMatrix = createWidthMatrix(context);
    const perfFilter = context.createBiquadFilter();
    perfFilter.type = "lowpass";
    perfFilter.frequency.value = 20000;
    perfFilter.Q.value = 0.7;
    // Parallel momentary Live FX deck: idle routes pure unity dry straight to
    // perfGate so DC transparency, neutral width and offline renders stay exact.
    const perfDry = context.createGain();
    perfDry.gain.value = 1;
    const perfHighpass = context.createBiquadFilter();
    perfHighpass.type = "highpass";
    perfHighpass.frequency.value = 40;
    perfHighpass.Q.value = 0.7;
    const perfRiserWet = context.createGain();
    perfRiserWet.gain.value = 0;
    const perfTapeDelay = context.createDelay(2.0);
    perfTapeDelay.delayTime.value = 0;
    const perfTapeWet = context.createGain();
    perfTapeWet.gain.value = 0;
    const perfDubSend = context.createGain();
    perfDubSend.gain.value = 0;
    const perfDubDelay = context.createDelay(2.0);
    perfDubDelay.delayTime.value = 0.375;
    const perfDubFilter = context.createBiquadFilter();
    perfDubFilter.type = "bandpass";
    perfDubFilter.frequency.value = 1350;
    perfDubFilter.Q.value = 1.6;
    const perfDubFb = context.createGain();
    perfDubFb.gain.value = 0;
    const perfDubReturn = context.createGain();
    perfDubReturn.gain.value = 0.78;
    const perfRollIn = context.createGain();
    perfRollIn.gain.value = 0;
    const perfRollDelay = context.createDelay(2.0);
    perfRollDelay.delayTime.value = 0.25;
    const perfRollFb = context.createGain();
    perfRollFb.gain.value = 0;
    const perfRollWet = context.createGain();
    perfRollWet.gain.value = 0;
    const perfGate = context.createGain();
    perfGate.gain.value = 1;
    const analyser = typeof context.createAnalyser === "function" ? context.createAnalyser() : null;
    if (analyser) { analyser.fftSize = 2048; analyser.smoothingTimeConstant = 0.75; }

    input.connect(gain);
    gain.connect(low);
    low.connect(mid);
    mid.connect(high);
    high.connect(compressor);
    compressor.connect(widthMatrix.input);
    widthMatrix.output.connect(width);
    width.connect(perfFilter);
    perfFilter.connect(perfDry);
    perfDry.connect(perfGate);
    perfFilter.connect(perfHighpass);
    perfHighpass.connect(perfRiserWet);
    perfRiserWet.connect(perfGate);
    perfFilter.connect(perfTapeDelay);
    perfTapeDelay.connect(perfTapeWet);
    perfTapeWet.connect(perfGate);
    perfFilter.connect(perfDubSend);
    perfDubSend.connect(perfDubDelay);
    perfDubDelay.connect(perfDubFilter);
    perfDubFilter.connect(perfDubFb);
    perfDubFb.connect(perfDubDelay);
    perfDubFilter.connect(perfDubReturn);
    perfDubReturn.connect(perfGate);
    perfFilter.connect(perfRollIn);
    perfRollIn.connect(perfRollDelay);
    perfRollDelay.connect(perfRollFb);
    perfRollFb.connect(perfRollDelay);
    perfRollDelay.connect(perfRollWet);
    perfRollWet.connect(perfGate);
    // The ceiling is the last stage before the output: width and the momentary
    // Live FX can both amplify the mix, so nothing after them may exceed it.
    perfGate.connect(limiterPre);
    limiterPre.connect(limiter);
    limiter.connect(limiterPost);
    if (analyser) { limiterPost.connect(analyser); analyser.connect(context.destination); }
    else limiterPost.connect(context.destination);

    const cancelAt = (param, at) => { try { param.cancelScheduledValues(at); } catch (_) {} };
    const clearExclusivePerf = at => {
      cancelAt(perfDry.gain, at);
      cancelAt(perfRiserWet.gain, at);
      cancelAt(perfTapeWet.gain, at);
      cancelAt(perfTapeDelay.delayTime, at);
      cancelAt(perfRollIn.gain, at);
      cancelAt(perfRollWet.gain, at);
      cancelAt(perfRollFb.gain, at);
      perfRiserWet.gain.setValueAtTime(0, at);
      perfTapeWet.gain.setValueAtTime(0, at);
      perfTapeDelay.delayTime.setValueAtTime(0, at);
      perfRollIn.gain.setValueAtTime(0, at);
      perfRollWet.gain.setValueAtTime(0, at);
      perfRollFb.gain.setValueAtTime(0, at);
    };

    const master = {
      input,
      output: analyser || limiterPost,
      gain,
      low,
      mid,
      high,
      compressor,
      limiterPre,
      limiter,
      limiterPost,
      width,
      widthMatrix,
      perfFilter,
      perfHighpass,
      perfTapeDelay,
      perfDubSend,
      perfRollWet,
      perfGate,
      analyser,
      settings,
      perform(kind, secondsPerBeat) {
        const at = context.currentTime;
        const beat = clamp(secondsPerBeat, 0.05, 2, 0.5);
        if (kind === "filter") {
          const frequency = perfFilter.frequency;
          const span = beat * 1.5;
          cancelAt(frequency, at);
          frequency.setValueAtTime(Math.max(18000, Math.min(20000, Number(frequency.value) || 20000)), at);
          if (typeof frequency.exponentialRampToValueAtTime === "function") {
            frequency.exponentialRampToValueAtTime(220, at + span * 0.45);
            frequency.exponentialRampToValueAtTime(18000, at + span);
          } else {
            frequency.setValueAtTime(220, at + span * 0.45);
            frequency.setValueAtTime(18000, at + span);
          }
          set(perfFilter.Q, 7, at, false);
          set(perfFilter.Q, 0.7, at + span, true);
          return true;
        }
        if (kind === "stutter") {
          const step = beat / 4;
          const gainParam = perfGate.gain;
          cancelAt(gainParam, at);
          for (let index = 0; index < 16; index++) gainParam.setValueAtTime(index % 2 ? 0.02 : 1, at + index * step);
          gainParam.setValueAtTime(1, at + 16 * step);
          return true;
        }
        if (kind === "riser") {
          const span = beat * 2;
          clearExclusivePerf(at);
          perfDry.gain.setValueAtTime(1, at);
          perfDry.gain.linearRampToValueAtTime(0, at + 0.01);
          perfDry.gain.setValueAtTime(0, at + span - 0.01);
          perfDry.gain.linearRampToValueAtTime(1, at + span);
          perfRiserWet.gain.setValueAtTime(0, at);
          perfRiserWet.gain.linearRampToValueAtTime(1, at + 0.01);
          perfRiserWet.gain.setValueAtTime(1, at + span - 0.01);
          perfRiserWet.gain.linearRampToValueAtTime(0, at + span);
          const frequency = perfHighpass.frequency;
          cancelAt(frequency, at);
          frequency.setValueAtTime(40, at);
          if (typeof frequency.exponentialRampToValueAtTime === "function") {
            frequency.exponentialRampToValueAtTime(4800, at + span * 0.94);
          } else {
            frequency.setValueAtTime(4800, at + span * 0.94);
          }
          frequency.setValueAtTime(40, at + span);
          cancelAt(perfHighpass.Q, at);
          perfHighpass.Q.setValueAtTime(1.2, at);
          perfHighpass.Q.linearRampToValueAtTime(8.5, at + span * 0.9);
          perfHighpass.Q.setValueAtTime(0.7, at + span);
          return true;
        }
        if (kind === "tapestop") {
          const span = clamp(beat * 1.5, 0.25, 1.5, 0.75);
          clearExclusivePerf(at);
          perfDry.gain.setValueAtTime(1, at);
          perfDry.gain.linearRampToValueAtTime(0, at + 0.008);
          perfDry.gain.setValueAtTime(0, at + span + 0.06);
          perfDry.gain.linearRampToValueAtTime(1, at + span + 0.08);
          perfTapeWet.gain.setValueAtTime(0, at);
          perfTapeWet.gain.linearRampToValueAtTime(1, at + 0.008);
          perfTapeWet.gain.setValueAtTime(1, at + span * 0.85);
          perfTapeWet.gain.linearRampToValueAtTime(0, at + span);
          // Quadratic delay growth ramps effective read speed linearly from 1 -> 0.
          perfTapeDelay.delayTime.setValueAtTime(0, at);
          for (let step = 1; step <= 16; step++) {
            const elapsed = (span * step) / 16;
            const delay = Math.min(1.9, (0.5 * elapsed * elapsed) / span);
            perfTapeDelay.delayTime.linearRampToValueAtTime(delay, at + elapsed);
          }
          perfTapeDelay.delayTime.setValueAtTime(0, at + span + 0.06);
          return true;
        }
        if (kind === "dub") {
          const delayTime = clamp(beat * 0.75, 0.08, 1.5, 0.375);
          cancelAt(perfDubDelay.delayTime, at);
          cancelAt(perfDubSend.gain, at);
          cancelAt(perfDubFb.gain, at);
          perfDubDelay.delayTime.setValueAtTime(delayTime, at);
          perfDubFb.gain.setValueAtTime(0.68, at);
          perfDubFb.gain.setValueAtTime(0.68, at + beat * 4);
          perfDubFb.gain.linearRampToValueAtTime(0, at + beat * 8);
          perfDubSend.gain.setValueAtTime(0, at);
          perfDubSend.gain.linearRampToValueAtTime(0.85, at + 0.01);
          perfDubSend.gain.setValueAtTime(0.85, at + beat);
          perfDubSend.gain.linearRampToValueAtTime(0, at + beat + 0.025);
          return true;
        }
        if (kind === "roll") {
          const slice = clamp(beat * 0.5, 0.06, 0.8, 0.25);
          const total = slice * 6;
          clearExclusivePerf(at);
          cancelAt(perfRollDelay.delayTime, at);
          perfRollDelay.delayTime.setValueAtTime(slice, at);
          // Capture one 1/8 slice, then loop it cleanly for the rest of the roll.
          perfDry.gain.setValueAtTime(1, at);
          perfDry.gain.setValueAtTime(1, at + slice - 0.005);
          perfDry.gain.linearRampToValueAtTime(0, at + slice);
          perfDry.gain.setValueAtTime(0, at + total - 0.008);
          perfDry.gain.linearRampToValueAtTime(1, at + total);
          perfRollIn.gain.setValueAtTime(1, at);
          perfRollIn.gain.setValueAtTime(1, at + slice - 0.005);
          perfRollIn.gain.linearRampToValueAtTime(0, at + slice);
          perfRollWet.gain.setValueAtTime(0, at);
          perfRollWet.gain.setValueAtTime(0, at + slice - 0.005);
          perfRollWet.gain.linearRampToValueAtTime(1, at + slice);
          perfRollWet.gain.setValueAtTime(1, at + total - 0.008);
          perfRollWet.gain.linearRampToValueAtTime(0, at + total);
          perfRollFb.gain.setValueAtTime(0, at);
          perfRollFb.gain.setValueAtTime(0.94, at + slice);
          perfRollFb.gain.setValueAtTime(0, at + total);
          return true;
        }
        return false;
      },
      resetPerformance() {
        const at = context.currentTime;
        cancelAt(perfFilter.frequency, at);
        perfFilter.frequency.setValueAtTime(20000, at);
        cancelAt(perfGate.gain, at);
        perfGate.gain.setValueAtTime(1, at);
        set(perfFilter.Q, 0.7, at, false);
        clearExclusivePerf(at);
        perfDry.gain.setValueAtTime(1, at);
        cancelAt(perfHighpass.frequency, at);
        perfHighpass.frequency.setValueAtTime(40, at);
        cancelAt(perfHighpass.Q, at);
        perfHighpass.Q.setValueAtTime(0.7, at);
        cancelAt(perfDubSend.gain, at);
        perfDubSend.gain.setValueAtTime(0, at);
        cancelAt(perfDubFb.gain, at);
        perfDubFb.gain.setValueAtTime(0, at);
      },
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
        widthMatrix.update(clamp(settings.width, 0, 2, 1), at, smooth);
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
    MASTER_CEILING,
    LIMITER_HEADROOM,
    LIMITER_KNEE,
    SYNC_DIVISIONS,
    types: () => Object.keys(BUILDERS),
    createInsert,
    createMaster,
    createSends,
    limiterCurve,
    syncSeconds
  };
})(typeof window !== "undefined" ? window : globalThis);
