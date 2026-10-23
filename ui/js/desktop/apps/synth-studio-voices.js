(function (root) {
  "use strict";

  // Synth Studio voice engine: patch resolution, the per-track mix graph,
  // voice envelopes and controllers. synth-studio-audio.js drives these same
  // nodes for live playback and offline WAV rendering.

  const TINY = 0.0001;
  const DEFAULT_PARAMS = { tone: 0.5, resonance: 0, drive: 0, attack: 0.01, release: 0.3, chorus: 0, phaser: 0, reverb: 0, delay: 0 };
  const GM = root.SynthStudioGMData;

  const DRIVE_CURVES = Array.from({ length: 33 }, (_, step) => {
    const curve = new Float32Array(513);
    const amount = step / 32;
    const k = 1 + amount * 14;
    const norm = Math.tanh(k) || 1;
    for (let i = 0; i < 513; i++) {
      const x = (i / 256) - 1;
      curve[i] = amount <= 0 ? x : (Math.tanh(x * k) / norm) * (1 - amount * 0.14);
    }
    return curve;
  });

  function clamp(value, low, high, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(high, Math.max(low, number)) : fallback;
  }

  function trackParams(track) {
    return track.params || DEFAULT_PARAMS;
  }

  function customDrumPatch(note) {
    switch (note) {
      case 35: // Sub Kick (808 style)
        return [
          { w: "sine", t: 0, f: 48, v: 0.92, a: 0.001, d: 0.38, s: 0, r: 0.18, p: 3.2, q: 0.065 },
          { w: "triangle", t: 0, f: 110, v: 0.28, a: 0.001, d: 0.04, s: 0, r: 0.03, p: 2.4, q: 0.02 }
        ];
      case 36: // Punch Kick (909 style)
        return [
          { w: "sine", t: 0, f: 56, v: 0.88, a: 0.001, d: 0.24, s: 0, r: 0.12, p: 3.8, q: 0.045 },
          { w: "n0", t: 0, f: 400, v: 0.18, a: 0.001, d: 0.018, s: 0, r: 0.015 }
        ];
      case 37: // Rimshot
        return [
          { w: "triangle", t: 0, f: 480, v: 0.55, a: 0.001, d: 0.045, s: 0, r: 0.03, p: 1.6, q: 0.015 },
          { w: "square", t: 0, f: 1620, v: 0.25, a: 0.001, d: 0.025, s: 0, r: 0.02 }
        ];
      case 38: // Snare
        return [
          { w: "triangle", t: 0, f: 185, v: 0.52, a: 0.001, d: 0.11, s: 0, r: 0.06, p: 1.8, q: 0.03 },
          { w: "n0", t: 0, f: 900, v: 0.42, a: 0.001, d: 0.18, s: 0, r: 0.09 }
        ];
      case 39: // Clap
        return [
          { w: "n0", t: 0, f: 1200, v: 0.48, a: 0.002, h: 0.012, d: 0.16, s: 0, r: 0.08 },
          { w: "triangle", t: 0, f: 860, v: 0.16, a: 0.001, d: 0.04, s: 0, r: 0.03 }
        ];
      case 40: // 909 Snare
        return [
          { w: "sine", t: 0, f: 210, v: 0.58, a: 0.001, d: 0.13, s: 0, r: 0.07, p: 2.1, q: 0.035 },
          { w: "n0", t: 0, f: 1400, v: 0.48, a: 0.001, d: 0.22, s: 0, r: 0.11 }
        ];
      case 42: // Closed Hi-Hat
        return [
          { w: "n1", t: 0, f: 6800, v: 0.34, a: 0.001, d: 0.048, s: 0, r: 0.025 },
          { w: "square", t: 0, f: 4200, v: 0.12, a: 0.001, d: 0.03, s: 0, r: 0.02 }
        ];
      case 45: // Low Tom
      case 47: // Mid Tom
      case 50: // High Tom
        return [
          { w: "sine", t: 0, f: note === 45 ? 98 : note === 47 ? 138 : 188, v: 0.72, a: 0.001, d: 0.22, s: 0, r: 0.11, p: 1.9, q: 0.055 },
          { w: "triangle", t: 0, f: note === 45 ? 196 : note === 47 ? 276 : 376, v: 0.18, a: 0.001, d: 0.07, s: 0, r: 0.04 }
        ];
      case 46: // Open Hi-Hat
        return [
          { w: "n1", t: 0, f: 6800, v: 0.36, a: 0.001, d: 0.28, s: 0.05, r: 0.16 },
          { w: "square", t: 0, f: 5100, v: 0.11, a: 0.001, d: 0.18, s: 0, r: 0.1 }
        ];
      case 49: // Crash
        return [
          { w: "n1", t: 0, f: 5200, v: 0.38, a: 0.002, d: 0.65, s: 0.08, r: 0.42 },
          { w: "bell", t: 0, f: 3400, v: 0.16, a: 0.001, d: 0.38, s: 0, r: 0.25 }
        ];
      case 51: // Ride Cymbal
        return [
          { w: "bell", t: 0, f: 2850, v: 0.28, a: 0.001, d: 0.45, s: 0.04, r: 0.32 },
          { w: "n1", t: 0, f: 6200, v: 0.22, a: 0.001, d: 0.36, s: 0, r: 0.22 }
        ];
      case 56: // 808 Cowbell
        return [
          { w: "pulse25", t: 0, f: 540, v: 0.38, a: 0.001, d: 0.18, s: 0.02, r: 0.09 },
          { w: "pulse25", t: 0, f: 800, v: 0.34, a: 0.001, d: 0.16, s: 0.02, r: 0.08 }
        ];
      case 63: // Electro Conga
        return [
          { w: "sine", t: 0, f: 245, v: 0.65, a: 0.001, d: 0.16, s: 0, r: 0.08, p: 1.55, q: 0.035 },
          { w: "triangle", t: 0, f: 490, v: 0.16, a: 0.001, d: 0.05, s: 0, r: 0.03 }
        ];
      case 70: // Shaker
        return [
          { w: "n0", t: 0, f: 7500, v: 0.28, a: 0.008, d: 0.065, s: 0, r: 0.035 }
        ];
      default:
        return null;
    }
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
    const wave = p.wave || "sawtooth";
    const baseLevel = clamp(p.level, 0, 1.5, 0.24);
    const secondLevel = clamp(p.second, 0, 1, 0);
    const unisonCount = p.unison >= 5 ? 5 : p.unison >= 3 ? 3 : 1;
    const totalBudget = baseLevel + secondLevel;
    const mainLevel = unisonCount === 5 ? totalBudget * 0.42 : unisonCount === 3 ? totalBudget * 0.52 : baseLevel;
    const attack = clamp(p.attack, 0, 5, 0.01);
    const decay = clamp(p.decay, 0, 12, 0.25);
    const sustain = clamp(p.sustain, 0, 1.5, 0.65);
    const release = clamp(p.release, 0, 30, 0.15);
    const pitchSweep = p.pitchEnv ? clamp(p.pitchEnv, 0.2, 4, 1) : p.glide ? clamp(p.glide, 0.25, 4, 1) : 1;
    const pitchTime = clamp(p.pitchTime, 0.015, 0.6, p.glide ? 0.085 : 0.045);

    const mainOp = { w: wave, v: mainLevel, a: attack, d: decay, s: sustain, r: release, t: 1 };
    if (pitchSweep !== 1) {
      mainOp.p = pitchSweep;
      mainOp.q = pitchTime;
    }
    const operators = [mainOp];

    if (unisonCount >= 3) {
      const cents = clamp(p.detune, 1, 60, 10);
      const sideLevel = unisonCount === 5 ? totalBudget * 0.17 : totalBudget * 0.24;
      for (const sign of [1, -1]) {
        const op = { w: wave, v: sideLevel, t: Math.pow(2, (sign * cents) / 1200), a: attack, d: decay, s: sustain, r: release };
        if (pitchSweep !== 1) { op.p = pitchSweep; op.q = pitchTime; }
        operators.push(op);
      }
      if (unisonCount >= 5) {
        const outerLevel = totalBudget * 0.12;
        for (const sign of [1, -1]) {
          const op = { w: wave, v: outerLevel, t: Math.pow(2, (sign * cents * 0.52) / 1200), a: attack, d: decay, s: sustain, r: release };
          if (pitchSweep !== 1) { op.p = pitchSweep; op.q = pitchTime; }
          operators.push(op);
        }
      }
    } else if (p.second) {
      const op = { w: wave, v: clamp(p.second, 0, 1, 0.12), t: Math.pow(2, clamp(p.detune, -1200, 1200, 8) / 1200), a: attack, d: decay, s: sustain, r: release };
      if (pitchSweep !== 1) { op.p = pitchSweep; op.q = pitchTime; }
      operators.push(op);
    }

    if (p.sub) {
      const subOp = {
        w: p.subWave || "sine",
        v: clamp(p.sub, 0, 0.6, 0.14),
        t: p.subOctave === 2 ? 0.25 : 0.5,
        a: attack,
        d: decay,
        s: sustain,
        r: release
      };
      if (pitchSweep !== 1) { subOp.p = pitchSweep; subOp.q = pitchTime; }
      operators.push(subOp);
    }

    if (p.mod) {
      operators.push({ w: p.mod, v: clamp(p.depth, 0, 24, 2), t: clamp(p.ratio, 0.01, 32, 2), a: attack, d: decay, s: sustain, r: release, g: 1 });
    }
    if (p.mod2) {
      operators.push({
        w: p.mod2,
        v: clamp(p.depth2, 0, 24, 1.4),
        t: clamp(p.ratio2, 0.01, 32, 4),
        a: attack * 0.5,
        d: clamp(p.decay2, 0.02, 8, decay * 0.6),
        s: p.decay2 ? 0.06 : sustain * 0.45,
        r: release,
        g: 1
      });
    }
    if (p.noise) operators.push({ w: "n0", v: clamp(p.noise, 0, 1, 0.02), a: 0.001, d: 0.2, s: 0, r: 0.04 });
    return operators;
  }

  function operatorsFor(track, pitch) {
    const instrument = track.instrument;
    if (instrument === "gm-drums") return getGMPatch(pitch);
    if (instrument === "drums-kit") return customDrumPatch(pitch) || getGMPatch(pitch);
    const preset = root.SynthStudioPresets?.get(instrument);
    if (preset?.category === "drums") return customDrumPatch(preset.drumNote) || getGMPatch(preset.drumNote);
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
    const rate = context.sampleRate;
    const length = Math.max(1, Math.floor(rate * 2.2));
    const buffer = context.createBuffer(2, length, rate);
    const reflections = [
      [0.011, 0.32, -0.25], [0.019, -0.26, 0.31], [0.029, 0.22, 0.18],
      [0.043, -0.18, -0.21], [0.059, 0.15, -0.14], [0.079, -0.12, 0.13]
    ];
    let seed = 0x7f4a7c15;
    for (let channel = 0; channel < 2; channel++) {
      const data = buffer.getChannelData(channel);
      let lp = 0;
      for (let i = 0; i < length; i++) {
        seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5;
        const noise = ((seed >>> 0) / 0x80000000) - 1;
        lp = lp * 0.62 + noise * 0.38;
        const decay = Math.pow(1 - i / length, 3.2);
        data[i] = lp * decay * 0.24;
      }
      for (const [sec, gainL, gainR] of reflections) {
        const idx = Math.floor(sec * rate);
        if (idx < length) data[idx] += channel === 0 ? gainL : gainR;
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
    return { master, output, noise: makeNoiseBuffers(context), impulse: makeReverbBuffer(context), periodic: {} };
  }

  function createTrackBus(context, mix, track) {
    const input = context.createGain();
    const filter = context.createBiquadFilter();
    filter.type = "lowpass";
    const drive = context.createWaveShaper();
    drive.curve = DRIVE_CURVES[0];
    drive.oversample = "2x";

    const phaser1 = context.createBiquadFilter();
    const phaser2 = context.createBiquadFilter();
    const phaser3 = context.createBiquadFilter();
    const phaser4 = context.createBiquadFilter();
    const phaserFreqs = [680, 1180, 1920, 3100];
    [phaser1, phaser2, phaser3, phaser4].forEach((stage, idx) => {
      stage.type = "allpass";
      stage.frequency.value = phaserFreqs[idx];
      stage.Q.value = 0.85;
    });
    const phaserWet = context.createGain();
    phaserWet.gain.value = 0;
    const phaserLfo = context.createOscillator();
    phaserLfo.type = "sine";
    phaserLfo.frequency.value = 0.42;
    const phaserLfoGain = context.createGain();
    phaserLfoGain.gain.value = 650;
    phaserLfo.connect(phaserLfoGain);
    [phaser1, phaser2, phaser3, phaser4].forEach(stage => phaserLfoGain.connect(stage.detune));
    phaserLfo.start(0);

    const postFx = context.createGain();
    const pan = context.createStereoPanner ? context.createStereoPanner() : context.createPanner();
    const gain = context.createGain();

    input.connect(filter);
    filter.connect(drive);
    drive.connect(postFx);
    drive.connect(phaser1);
    phaser1.connect(phaser2);
    phaser2.connect(phaser3);
    phaser3.connect(phaser4);
    phaser4.connect(phaserWet);
    phaserWet.connect(postFx);

    postFx.connect(pan);
    pan.connect(gain);
    gain.connect(mix.master);

    const chorusDelayL = context.createDelay(0.1);
    const chorusDelayR = context.createDelay(0.1);
    chorusDelayL.delayTime.value = 0.014;
    chorusDelayR.delayTime.value = 0.019;
    const chorusLfo = context.createOscillator();
    chorusLfo.type = "sine";
    chorusLfo.frequency.value = 0.68;
    const chorusDepthL = context.createGain();
    const chorusDepthR = context.createGain();
    chorusDepthL.gain.value = 0.0026;
    chorusDepthR.gain.value = -0.0026;
    chorusLfo.connect(chorusDepthL);
    chorusLfo.connect(chorusDepthR);
    chorusDepthL.connect(chorusDelayL.delayTime);
    chorusDepthR.connect(chorusDelayR.delayTime);
    chorusLfo.start(0);

    const chorusPanL = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    const chorusPanR = context.createStereoPanner ? context.createStereoPanner() : context.createGain();
    if (chorusPanL.pan) chorusPanL.pan.value = -0.75;
    if (chorusPanR.pan) chorusPanR.pan.value = 0.75;
    const chorusGain = context.createGain();
    chorusGain.gain.value = 0;
    postFx.connect(chorusDelayL);
    postFx.connect(chorusDelayR);
    chorusDelayL.connect(chorusPanL);
    chorusDelayR.connect(chorusPanR);
    chorusPanL.connect(chorusGain);
    chorusPanR.connect(chorusGain);
    chorusGain.connect(gain);

    const reverb = context.createConvolver();
    reverb.buffer = mix.impulse;
    const reverbGain = context.createGain();
    postFx.connect(reverb);
    reverb.connect(reverbGain);
    reverbGain.connect(mix.master);

    const delay = context.createDelay(1.5);
    delay.delayTime.value = 0.24;
    const delayFilter = context.createBiquadFilter();
    delayFilter.type = "lowpass";
    delayFilter.frequency.value = 2800;
    const feedback = context.createGain();
    feedback.gain.value = 0.28;
    const delayGain = context.createGain();
    postFx.connect(delay);
    delay.connect(delayFilter);
    delayFilter.connect(delayGain);
    delayGain.connect(mix.master);
    delayFilter.connect(feedback);
    feedback.connect(delay);

    const bus = {
      input, filter, drive, phaser1, phaser2, phaser3, phaser4, phaserWet, phaserLfo, phaserLfoGain,
      postFx, pan, gain, chorusDelayL, chorusDelayR, chorusLfo, chorusDepthL, chorusDepthR,
      chorusPanL, chorusPanR, chorusGain, reverb, reverbGain, delay, delayFilter, delayGain, feedback
    };
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

  function updateTrackBus(bus, track, at, smooth = false) {
    const p = trackParams(track);
    const now = at ?? bus.input.context.currentTime;
    const set = (param, value) => {
      if (!param) return;
      if (smooth && param.setTargetAtTime) { param.cancelScheduledValues(now); param.setTargetAtTime(value, now, 0.015); }
      else param.setValueAtTime(value, now);
    };
    const presetTone = root.SynthStudioPresets?.get(track.instrument)?.patch?.cutoff;
    const tone = Number.isFinite(presetTone) ? p.tone * 0.65 + presetTone * 0.35 : p.tone;
    const resonance = clamp(p.resonance, 0, 1, 0);
    const driveAmount = clamp(p.drive, 0, 1, 0);
    const chorusAmount = clamp(p.chorus, 0, 1, 0);
    const phaserAmount = clamp(p.phaser, 0, 1, 0);
    const qComp = 1 / (1 + resonance * 0.22);
    set(bus.input.gain, track.mute ? 0 : 1);
    set(bus.gain.gain, track.volume * qComp);
    set(bus.filter.frequency, 240 + tone * 18500);
    set(bus.filter.Q, 0.7 + tone * 0.8 + Math.pow(resonance, 1.35) * 11.5);
    if (bus.drive) bus.drive.curve = DRIVE_CURVES[Math.round(driveAmount * 32)] || DRIVE_CURVES[0];
    if (bus.phaserWet) set(bus.phaserWet.gain, phaserAmount * 0.55);
    if (bus.chorusGain) set(bus.chorusGain.gain, chorusAmount * 0.46);
    if (bus.pan.pan) set(bus.pan.pan, track.pan);
    else if (bus.pan.setPosition) bus.pan.setPosition(track.pan, 0, 1 - Math.abs(track.pan));
    set(bus.reverbGain.gain, p.reverb * 0.36 * track.volume);
    set(bus.delayGain.gain, p.delay * 0.4 * track.volume);
    if (bus.feedback) set(bus.feedback.gain, 0.24 + clamp(p.delay, 0, 1, 0) * 0.18);
  }

  const CUSTOM_HARMONICS = {
    w9999: [0, 1, 0.48, 0.32, 0.22, 0.14, 0.09, 0.05, 0.08],
    supersaw: [0, 1, 0.62, 0.44, 0.33, 0.26, 0.21, 0.17, 0.14, 0.11, 0.09, 0.07, 0.05],
    pulse25: [0, 1, 0.71, 0.33, 0, -0.2, -0.24, -0.14, 0, 0.11, 0.14],
    pulse12: [0, 1, 0.92, 0.8, 0.64, 0.46, 0.28, 0.12, 0, -0.09, -0.13, -0.12],
    formant: [0, 1, 0.72, 0.85, 0.48, 0.62, 0.3, 0.18, 0.24, 0.12, 0.06],
    bell: [0, 1, 0.15, 0.68, 0.08, 0.42, 0.05, 0.28, 0.03, 0.19, 0.12]
  };

  function waveFor(context, mix, wave) {
    if (wave === "n0" || wave === "n1") return "noise";
    const harmonics = CUSTOM_HARMONICS[wave];
    if (!harmonics) return wave;
    if (!mix.periodic) mix.periodic = {};
    if (!mix.periodic[wave]) {
      const real = new Float32Array(harmonics.length);
      const imag = new Float32Array(harmonics);
      mix.periodic[wave] = context.createPeriodicWave(real, imag, { disableNormalization: false });
    }
    return mix.periodic[wave];
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
    const patch = root.SynthStudioPresets?.get(track.instrument)?.patch;
    const params = trackParams(track);
    const pitchHz = 440 * Math.pow(2, (pitch - 69) / 12);
    let voiceFilter = null;
    if (patch && (patch.filterEnv || patch.filterQ)) {
      voiceFilter = context.createBiquadFilter();
      voiceFilter.type = "lowpass";
      const cutoffRatio = clamp(patch.cutoff ?? params.tone, 0.05, 1, 0.65);
      const baseCutoff = clamp(Math.max(pitchHz * 1.35, 180 + Math.pow(cutoffRatio, 1.7) * 9500), 120, 16000, 1800);
      const envAmt = clamp(patch.filterEnv, 0, 1, 0.25) * (0.55 + velocity * 0.45);
      const peakCutoff = clamp(baseCutoff * (1 + envAmt * 5.2), baseCutoff, 18000, 6500);
      const fAttack = Math.max(0.003, clamp(patch.attack, 0, 5, 0.008), params.attack);
      const fDecay = clamp(patch.filterDecay, 0.04, 2.5, clamp(patch.decay, 0.05, 2.5, 0.22));
      voiceFilter.frequency.setValueAtTime(baseCutoff, at);
      voiceFilter.frequency.linearRampToValueAtTime(peakCutoff, at + fAttack);
      voiceFilter.frequency.exponentialRampToValueAtTime(Math.max(90, baseCutoff * (1 + envAmt * 0.45)), at + fAttack + fDecay);
      voiceFilter.Q.setValueAtTime(clamp((patch.filterQ || 1.0) + clamp(params.resonance, 0, 1, 0) * 6, 0.7, 15, 1.2), at);
      voiceGain.connect(voiceFilter);
      voiceFilter.connect(bus.input);
    } else {
      voiceGain.connect(bus.input);
    }
    const sources = [];
    const envelopes = [];
    const operatorsOut = [];
    const velocityScale = Math.pow(velocity, 2);
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
        if (op.p && op.p !== 1 && (tune !== 0 || fixed > 0) && baseFrequency > 0) {
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
    const voice = { trackId: track.id, sources, envelopes, releaseSeconds, startTime: at, endTime: Infinity, voiceGain, voiceFilter, vibrato: null, releaseRequested: false, disposed: false, liveInput: false };
    attachVibrato(context, voice, state.mod, at);
    return voice;
  }

  function disposeVoice(voice) {
    if (voice.disposed) return;
    voice.disposed = true;
    try { voice.voiceGain.disconnect(); } catch (_) {}
    if (voice.voiceFilter) try { voice.voiceFilter.disconnect(); } catch (_) {}
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
