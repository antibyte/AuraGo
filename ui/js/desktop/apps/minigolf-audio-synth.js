// Locally synthesized AuraGolf sound: seeded noise, reverb impulse responses, UI blips and world ambience.
// Every helper takes the AudioContext and a destination node; nothing here owns global state.
export function mulberry32(seed) {
  let a = seed | 0;
  return () => {
    a = (a + 0x6D2B79F5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function seedOf(text) {
  let hash = 2166136261;
  for (let i = 0; i < text.length; i++) hash = Math.imul(hash ^ text.charCodeAt(i), 16777619);
  return hash >>> 0;
}

// The tail is cross-faded into the head so a looped buffer has no seam click.
export function noiseBuffer(ctx, seconds, color = 'white', seed = 1) {
  const length = Math.max(2, Math.round(seconds * ctx.sampleRate));
  const fade = Math.min(length >> 2, Math.round(0.15 * ctx.sampleRate));
  const raw = new Float32Array(length + fade);
  const random = mulberry32(seed);
  let last = 0;
  for (let i = 0; i < raw.length; i++) {
    const white = random() * 2 - 1;
    if (color === 'brown') {
      last = (last + 0.02 * white) / 1.02;
      raw[i] = last * 3.5;
    } else raw[i] = white;
  }
  const buffer = ctx.createBuffer(1, length, ctx.sampleRate);
  const data = buffer.getChannelData(0);
  for (let i = 0; i < length; i++) {
    const w = i < fade ? (i / fade) * Math.PI / 2 : 0;
    data[i] = i < fade ? raw[i] * Math.sin(w) + raw[length + i] * Math.cos(w) : raw[i];
  }
  return buffer;
}

export const REVERB_PRESETS = {
  menu: { seconds: 0.6, wet: 0.06, damping: 0.3 },
  pirate: { seconds: 1.0, wet: 0.12, damping: 0.15 },
  temple: { seconds: 2.2, wet: 0.24, damping: 0.7 },
  moon: { seconds: 1.5, wet: 0.18, damping: 0.4, flutter: 0.012 }
};
const EARLY_TAPS = [[0.007, 1.4], [0.013, 1.1], [0.021, 0.9], [0.034, 0.6]];

// Stereo impulse: seeded noise, -60 dB exponential decay at `seconds`, darker toward the tail.
export function createImpulse(ctx, id) {
  const preset = REVERB_PRESETS[id] || REVERB_PRESETS.menu;
  const rate = ctx.sampleRate;
  const length = Math.max(2, Math.round(preset.seconds * rate));
  const buffer = ctx.createBuffer(2, length, rate);
  for (let channel = 0; channel < 2; channel++) {
    const data = buffer.getChannelData(channel);
    const random = mulberry32(seedOf(id) + channel * 7919);
    let low = 0;
    for (let i = 0; i < length; i++) {
      const progress = i / length;
      low += Math.max(0.05, 1 - preset.damping * (0.2 + 0.78 * progress)) * ((random() * 2 - 1) - low);
      data[i] = low * Math.exp(-6.9078 * progress);
    }
    for (const [time, amount] of EARLY_TAPS) {
      const at = Math.round((time + channel * 0.0013) * rate);
      if (at < length) data[at] += (random() < 0.5 ? -1 : 1) * amount;
    }
    if (preset.flutter) {
      const period = Math.round(preset.flutter * rate);
      for (let i = period; i < length; i++) data[i] += 0.5 * data[i - period];
    }
  }
  return buffer;
}

// One enveloped oscillator; `path` is [[seconds, hz], ...] with linear glides, `band` adds a bandpass.
function tone(ctx, destination, { type = 'sine', path, start, dur, peak, band }) {
  const osc = ctx.createOscillator();
  const env = ctx.createGain();
  const nodes = [osc, env];
  osc.type = type;
  osc.frequency.setValueAtTime(path[0][1], start + path[0][0]);
  for (const [offset, hz] of path.slice(1)) osc.frequency.linearRampToValueAtTime(hz, start + offset);
  env.gain.setValueAtTime(0, start);
  env.gain.linearRampToValueAtTime(peak, start + Math.min(0.01, dur / 4));
  env.gain.linearRampToValueAtTime(0, start + dur);
  if (band) {
    const filter = ctx.createBiquadFilter();
    filter.type = 'bandpass';
    filter.frequency.setValueAtTime(band, start);
    filter.Q.setValueAtTime(3, start);
    osc.connect(filter);
    filter.connect(env);
    nodes.push(filter);
  } else osc.connect(env);
  env.connect(destination);
  osc.onended = () => { for (const node of nodes) node.disconnect(); };
  osc.start(start);
  osc.stop(start + dur + 0.02);
}

const ticks = new WeakMap();
function tick(ctx, destination, start) {
  let buffer = ticks.get(ctx);
  if (!buffer) ticks.set(ctx, buffer = noiseBuffer(ctx, 0.008, 'white', 11));
  const source = ctx.createBufferSource();
  const env = ctx.createGain();
  source.buffer = buffer;
  env.gain.setValueAtTime(0.12, start);
  env.gain.linearRampToValueAtTime(0, start + 0.008);
  source.connect(env);
  env.connect(destination);
  source.onended = () => { source.disconnect(); env.disconnect(); };
  source.start(start);
}

export function playUi(ctx, destination, kind) {
  const start = ctx.currentTime + 0.005;
  if (kind === 'confirm' || kind === 'back') {
    const [first, second] = kind === 'confirm' ? [660, 990] : [880, 587];
    tone(ctx, destination, { path: [[0, first]], start, dur: 0.07, peak: 0.2 });
    tone(ctx, destination, { path: [[0, second]], start: start + 0.08, dur: 0.09, peak: 0.2 });
  } else {
    tone(ctx, destination, { path: [[0, 1600]], start, dur: 0.025, peak: 0.22 });
    tick(ctx, destination, start);
  }
}

function noiseLoop(ctx, color, seconds, seed) {
  const source = ctx.createBufferSource();
  source.buffer = noiseBuffer(ctx, seconds, color, seed);
  source.loop = true;
  return source;
}

function filterNode(ctx, type, hz, q = 1) {
  const filter = ctx.createBiquadFilter();
  filter.type = type;
  filter.frequency.setValueAtTime(hz, ctx.currentTime);
  filter.Q.setValueAtTime(q, ctx.currentTime);
  return filter;
}

function gainOf(ctx, value) {
  const node = ctx.createGain();
  node.gain.setValueAtTime(value, ctx.currentTime);
  return node;
}

// Sine LFO that modulates an AudioParam around its base value.
function lfo(ctx, hz, depth, param) {
  const osc = ctx.createOscillator();
  const amount = gainOf(ctx, depth);
  osc.frequency.setValueAtTime(hz, ctx.currentTime);
  osc.connect(amount);
  amount.connect(param);
  return [osc, amount];
}

// The cicada noise is generated once per context and shared by every burst.
const cicadaBuffers = new WeakMap();
function cicadaNoise(ctx) {
  let buffer = cicadaBuffers.get(ctx);
  if (!buffer) cicadaBuffers.set(ctx, buffer = noiseBuffer(ctx, 2, 'white', 22));
  const source = ctx.createBufferSource();
  source.buffer = buffer;
  source.loop = true;
  return source;
}

const AMBIENCE = {
  pirate: {
    level: 0.6, gap: [6, 14],
    bed(ctx, out) {
      const source = noiseLoop(ctx, 'brown', 4, 21);
      const filter = filterNode(ctx, 'lowpass', 650);
      const swell = gainOf(ctx, 0.55);
      source.connect(filter);
      filter.connect(swell);
      swell.connect(out);
      return [source, filter, swell, ...lfo(ctx, 0.09, 0.4, swell.gain)];
    },
    event(ctx, out, random) {
      const start = ctx.currentTime + 0.05;
      const cries = 2 + Math.floor(random() * 2);
      for (let i = 0; i < cries; i++) {
        tone(ctx, out, { type: 'sawtooth', path: [[0, 1500], [0.12, 2100], [0.35, 1300]], start: start + i * 0.45, dur: 0.35, peak: 0.1, band: 1600 });
      }
    }
  },
  temple: {
    level: 0.06, gap: [4, 11],
    // Only a very faint, slowly swelling jungle air runs all the time; the cicadas live in `burst`.
    bed(ctx, out) {
      const source = noiseLoop(ctx, 'brown', 4, 22);
      const filter = filterNode(ctx, 'lowpass', 650);
      const swell = gainOf(ctx, 0.55);
      source.connect(filter);
      filter.connect(swell);
      swell.connect(out);
      return [source, filter, swell, ...lfo(ctx, 0.05, 0.4, swell.gain)];
    },
    // Cicada chorus: a few seconds of pulsed, band-passed noise with soft edges, then a long silence.
    burst: {
      gap: [10, 25],
      play(ctx, out, random, start) {
        const dur = 2 + random() * 3;
        const edge = 0.4;
        const source = cicadaNoise(ctx);
        const filter = filterNode(ctx, 'bandpass', 4800 + random() * 800, 8);
        const pulse = gainOf(ctx, 0.5);
        const env = ctx.createGain();
        env.gain.setValueAtTime(0, start);
        env.gain.linearRampToValueAtTime(0.45, start + edge);
        env.gain.setValueAtTime(0.45, start + dur - edge);
        env.gain.linearRampToValueAtTime(0, start + dur);
        source.connect(filter);
        filter.connect(pulse);
        pulse.connect(env);
        env.connect(out);
        return { nodes: [source, filter, pulse, env, ...lfo(ctx, 33 + random() * 10, 0.5, pulse.gain)], dur };
      }
    },
    event(ctx, out, random) {
      let start = ctx.currentTime + 0.05;
      const calls = 2 + Math.floor(random() * 4);
      for (let i = 0; i < calls; i++) {
        const dur = 0.07 + random() * 0.09;
        tone(ctx, out, { path: [[0, 2400], [dur, 3600]], start, dur, peak: 0.08 });
        start += dur + 0.06 + random() * 0.09;
      }
    }
  },
  moon: {
    level: 0.03, gap: [5, 15],
    bed(ctx, out) {
      // A quiet low drone that slowly breathes, so it never reads as a static tone.
      const filter = filterNode(ctx, 'lowpass', 220);
      const breath = gainOf(ctx, 0.6);
      filter.connect(breath);
      breath.connect(out);
      const nodes = [filter, breath];
      for (const [hz, level] of [[55, 0.5], [110.4, 0.15]]) {
        const osc = ctx.createOscillator();
        const amount = gainOf(ctx, level);
        osc.type = 'sine';
        osc.frequency.setValueAtTime(hz, ctx.currentTime);
        osc.connect(amount);
        amount.connect(filter);
        nodes.push(osc, amount);
      }
      return [...nodes, ...lfo(ctx, 0.06, 0.4, breath.gain)];
    },
    event(ctx, out, random) {
      const start = ctx.currentTime + 0.05;
      const beeps = 2 + Math.floor(random() * 2);
      for (let i = 0; i < beeps; i++) tone(ctx, out, { path: [[0, i % 2 ? 1760 : 1320]], start: start + i * 0.15, dur: 0.06, peak: 0.06 });
    }
  }
};

// Single use: start() builds the bed and the event timers, stop() fades and tears everything down.
// A world has short `event` sounds and may add `burst`: longer sounds with their own gap, tracked until they end.
export function createAmbience(ctx, destination, world, { random = Math.random } = {}) {
  const spec = AMBIENCE[world];
  let phase = 0;
  let nodes = [];
  let out = null;
  const timers = new Set();
  const live = new Set();

  function schedule([low, high], run) {
    const id = setTimeout(() => {
      timers.delete(id);
      if (phase !== 1) return;
      run();
      schedule([low, high], run);
    }, (low + random() * (high - low)) * 1000);
    timers.add(id);
  }

  function release(burst) {
    if (!live.delete(burst)) return;
    for (const node of burst) {
      try { node.stop?.(); } catch (_) {}
      node.disconnect();
    }
  }

  function playBurst() {
    const start = ctx.currentTime + 0.05;
    const { nodes: burst, dur } = spec.burst.play(ctx, out, random, start);
    live.add(burst);
    for (const node of burst) { node.start?.(start); node.stop?.(start + dur + 0.05); }
    burst[0].onended = () => release(burst);
  }

  function start() {
    if (phase !== 0 || !spec) return;
    phase = 1;
    const now = ctx.currentTime;
    out = gainOf(ctx, 0);
    out.gain.linearRampToValueAtTime(1, now + 1.2);
    out.connect(destination);
    const bed = gainOf(ctx, spec.level);
    bed.connect(out);
    nodes = [out, bed, ...spec.bed(ctx, bed)];
    for (const node of nodes) node.start?.(now);
    schedule(spec.gap, () => spec.event(ctx, out, random));
    if (spec.burst) schedule(spec.burst.gap, playBurst);
  }

  function stop(fade = 0.6) {
    if (phase !== 1) return;
    phase = 2;
    for (const id of timers) clearTimeout(id);
    timers.clear();
    const finish = () => {
      for (const burst of [...live]) release(burst);
      for (const node of nodes) {
        try { node.stop?.(); } catch (_) {}
        node.disconnect();
      }
      nodes = [];
    };
    if (fade <= 0) return finish();
    out.gain.setTargetAtTime(0, ctx.currentTime, fade / 4);
    setTimeout(finish, fade * 1000 + 60);
  }

  return { start, stop };
}
