// Small Web Audio engine for AuraGolf: bus graph with limiter, decoded sample voices, loops and reverb.
// The AudioContext only exists after ensureContext(); no public method throws.
export const AUDIO_PATH_RE = /^(sfx|music)\/[a-z0-9-]+\.ogg$/;
const BASE = '/3d/minigolf/audio/';
const SAMPLE_LIMIT = 512 * 1024;
const MAX_VOICES = 20;
const COOLDOWN = 0.03;
const LOAD_CONCURRENCY = 6;

export const clamp = (value, min, max, fallback) => {
  const number = Number(value);
  return Number.isFinite(number) ? Math.max(min, Math.min(max, number)) : fallback;
};

export function createEngine({ assetURL = value => value, onError, random = Math.random, AudioContextClass } = {}) {
  let ctx = null;
  let graph = null;
  let reverb = null;
  let retiring = null;
  let disposed = false;
  let queue = Promise.resolve();
  let musicVolume = 1;
  let effectsVolume = 1;
  let loaded = 0;
  let failed = 0;
  const samples = new Map();
  const voices = new Set();
  const loops = new Set();
  const timers = new Set();
  const lastStart = new Map();
  const lastVariant = new Map();
  const reported = new Set();

  function report(kind, error) {
    if (disposed || reported.has(kind) || typeof onError !== 'function') return;
    reported.add(kind);
    try { onError(error instanceof Error ? error : new Error(String(error))); } catch (_) {}
  }

  const ease = (param, value, constant = 0.03) => param.setTargetAtTime(value, ctx.currentTime, constant);
  const gainNode = value => {
    const node = ctx.createGain();
    node.gain.setValueAtTime(value, ctx.currentTime);
    return node;
  };

  function buildGraph() {
    const now = ctx.currentTime;
    const g = {
      effects: gainNode(1), ambience: gainNode(1), ui: gainNode(1), music: gainNode(1),
      fx: gainNode(effectsVolume), musicGain: gainNode(musicVolume), master: gainNode(0.9), reverbIn: gainNode(1),
      limiter: ctx.createDynamicsCompressor()
    };
    for (const [name, value] of [['threshold', -3], ['knee', 0], ['ratio', 20], ['attack', 0.003], ['release', 0.12]]) g.limiter[name].setValueAtTime(value, now);
    for (const bus of [g.effects, g.ambience, g.ui]) bus.connect(g.fx);
    g.effects.connect(g.reverbIn);
    g.ambience.connect(g.reverbIn);
    g.fx.connect(g.master);
    g.music.connect(g.musicGain);
    g.musicGain.connect(g.master);
    g.master.connect(g.limiter);
    g.limiter.connect(ctx.destination);
    return g;
  }

  function ensureContext() {
    if (disposed) return null;
    if (ctx) return ctx;
    const Context = AudioContextClass || globalThis.AudioContext || globalThis.webkitAudioContext;
    if (!Context) {
      report('context', new Error('Web Audio is not available.'));
      return null;
    }
    let created = null;
    try {
      try { created = new Context({ latencyHint: 'balanced' }); } catch (_) { created = new Context(); }
      ctx = created;
      graph = buildGraph();
    } catch (error) {
      try { Promise.resolve(created?.close?.()).catch(() => {}); } catch (_) {}
      ctx = null;
      graph = null;
      report('context', error);
    }
    return ctx;
  }

  // Context state changes run one at a time so a quick pause/resume cannot end up suspended.
  const serial = task => (queue = queue.then(task, task));

  function start() {
    if (!ensureContext()) return Promise.resolve(false);
    return serial(async () => {
      const current = ctx;
      if (!current) return false;
      try { if (current.state !== 'running') await current.resume(); }
      catch (error) { report('context', error); return false; }
      return !disposed && current.state === 'running';
    });
  }

  function suspend() {
    return serial(async () => {
      const current = ctx;
      if (!current || current.state !== 'running') return;
      try { await current.suspend(); } catch (error) { report('context', error); }
    });
  }

  function resume() {
    return serial(async () => {
      const current = ctx;
      if (!current || disposed) return false;
      try { if (current.state !== 'running' && current.state !== 'closed') await current.resume(); }
      catch (error) { report('context', error); }
      return !disposed && current.state === 'running';
    });
  }

  function setMusicVolume(value) {
    musicVolume = clamp(value, 0, 1, musicVolume);
    if (graph) ease(graph.musicGain.gain, musicVolume);
    return musicVolume;
  }

  function setEffectsVolume(value) {
    effectsVolume = clamp(value, 0, 1, effectsVolume);
    if (graph) ease(graph.fx.gain, effectsVolume);
    return effectsVolume;
  }

  async function fetchBuffer(path, maxBytes = SAMPLE_LIMIT) {
    if (!AUDIO_PATH_RE.test(path)) throw new Error('Minigolf audio path is invalid.');
    const current = ctx;
    if (!current) throw new Error('Web Audio is not available.');
    const response = await fetch(assetURL(BASE + path), { cache: 'force-cache' });
    if (!response.ok) throw new Error(`Minigolf audio asset could not be loaded (${response.status}).`);
    const bytes = await response.arrayBuffer();
    if (bytes.byteLength > maxBytes) throw new Error('Minigolf audio asset exceeds its size limit.');
    const buffer = await current.decodeAudioData(bytes);
    if (disposed) throw new Error('Minigolf audio was disposed.');
    return buffer;
  }

  async function loadSamples(sfxMap) {
    const jobs = [];
    for (const [group, paths] of Object.entries(sfxMap || {})) paths.forEach((path, index) => jobs.push([group, path, index]));
    const slots = new Map();
    let next = 0;
    async function worker() {
      while (next < jobs.length && !disposed) {
        const [group, path, index] = jobs[next++];
        try {
          const buffer = await fetchBuffer(path, SAMPLE_LIMIT);
          const slot = slots.get(group) || [];
          slot[index] = buffer;
          slots.set(group, slot);
          samples.set(group, slot.filter(Boolean));
          loaded++;
        } catch (error) {
          failed++;
          report('sample', error);
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(LOAD_CONCURRENCY, jobs.length) }, worker));
    return sampleState();
  }

  function fadeOut({ source, level }, fade) {
    if (!ctx) return;
    const now = ctx.currentTime;
    try {
      level.gain.setTargetAtTime(0, now, Math.max(0.001, fade / 4));
      source.stop(now + fade);
    } catch (_) {}
  }

  function play(group, { gain = 1, rate = 1, pan = 0, delay = 0, priority = 0, bus = 'effects' } = {}) {
    const list = samples.get(group);
    if (!ctx || !graph || !list?.length || !graph[bus]) return null;
    try {
      const now = ctx.currentTime;
      const at = now + Math.max(0, Number(delay) || 0);
      const previousStart = lastStart.get(group);
      if (previousStart !== undefined && Math.abs(at - previousStart) < COOLDOWN) return null;
      if (voices.size >= MAX_VOICES) {
        const oldest = [...voices].find(voice => voice.priority <= priority);
        if (!oldest) return null;
        voices.delete(oldest);
        fadeOut(oldest, 0.015);
      }
      lastStart.set(group, at);
      const previous = lastVariant.get(group);
      const range = list.length > 1 && previous !== undefined ? list.length - 1 : list.length;
      let index = Math.min(range - 1, Math.floor(random() * range));
      if (range < list.length && index >= previous) index++;
      lastVariant.set(group, index);
      const jitter = spread => 1 + (random() * 2 - 1) * spread;
      const source = ctx.createBufferSource();
      const level = ctx.createGain();
      const panner = ctx.createStereoPanner();
      source.buffer = list[index];
      source.playbackRate.setValueAtTime(rate * jitter(0.06), now);
      level.gain.setValueAtTime(Math.max(0, gain) * jitter(0.15), now);
      panner.pan.setValueAtTime(clamp(pan, -1, 1, 0), now);
      source.connect(level);
      level.connect(panner);
      panner.connect(graph[bus]);
      const voice = { group, priority, source, level, panner };
      source.onended = () => {
        voices.delete(voice);
        for (const node of [source, level, panner]) node.disconnect();
      };
      voices.add(voice);
      source.start(at);
      return voice;
    } catch (error) {
      report('context', error);
      return null;
    }
  }

  function loop(group, { bus = 'effects' } = {}) {
    const buffer = samples.get(group)?.[0];
    if (!ctx || !graph || !buffer || !graph[bus]) return null;
    const current = ctx;
    try {
      const now = current.currentTime;
      const source = current.createBufferSource();
      const filter = current.createBiquadFilter();
      const level = current.createGain();
      const panner = current.createStereoPanner();
      source.buffer = buffer;
      source.loop = true;
      filter.type = 'lowpass';
      filter.frequency.setValueAtTime(8000, now);
      level.gain.setValueAtTime(0, now);
      panner.pan.setValueAtTime(0, now);
      source.connect(filter);
      filter.connect(level);
      level.connect(panner);
      panner.connect(graph[bus]);
      const to = (param, value) => param.setTargetAtTime(value, current.currentTime, 0.06);
      const handle = {
        source,
        setRate: value => to(source.playbackRate, value),
        setGain: value => to(level.gain, value),
        setCutoff: value => to(filter.frequency, value),
        setPan: value => to(panner.pan, clamp(value, -1, 1, 0)),
        stop(fade = 0.12) {
          if (loops.delete(handle)) fadeOut({ source, level }, fade);
        }
      };
      source.onended = () => {
        loops.delete(handle);
        for (const node of [source, filter, level, panner]) node.disconnect();
      };
      source.start(now);
      loops.add(handle);
      return handle;
    } catch (error) {
      report('context', error);
      return null;
    }
  }

  function stopVoices(fade = 0.02) {
    for (const voice of voices) fadeOut(voice, fade);
    voices.clear();
  }

  function dropReverb(slot) {
    const detach = () => graph?.reverbIn.disconnect(slot.send);
    for (const disconnect of [detach, () => slot.send.disconnect(), () => slot.convolver.disconnect(), () => slot.out.disconnect()]) {
      try { disconnect(); } catch (_) {}
    }
  }

  // Drops the slot that is still fading out (at most one) before another one retires.
  function dropRetiring() {
    if (!retiring) return;
    clearTimeout(retiring.timer);
    timers.delete(retiring.timer);
    dropReverb(retiring.slot);
    retiring = null;
  }

  // A fresh ConvolverNode per call, each behind its own output gain: the old one's output
  // fades to silence while its tail rings and is only disconnected afterwards.
  function setReverb(buffer, wet) {
    if (!ctx || !graph || !buffer) return false;
    try {
      const now = ctx.currentTime;
      const level = clamp(wet, 0, 1, 0.1);
      const previous = reverb;
      const send = ctx.createGain();
      const convolver = ctx.createConvolver();
      const out = ctx.createGain();
      convolver.buffer = buffer;
      send.gain.setValueAtTime(previous ? 0 : level, now);
      if (previous) send.gain.setTargetAtTime(level, now, 0.2);
      out.gain.setValueAtTime(1, now);
      graph.reverbIn.connect(send);
      send.connect(convolver);
      convolver.connect(out);
      out.connect(graph.fx);
      reverb = { send, convolver, out };
      if (previous) {
        dropRetiring();
        previous.send.gain.setTargetAtTime(0, now, 0.1);
        previous.out.gain.setTargetAtTime(0, now, 0.05);
        const timer = setTimeout(() => {
          timers.delete(timer);
          if (retiring?.slot === previous) retiring = null;
          dropReverb(previous);
        }, 500);
        timers.add(timer);
        retiring = { slot: previous, timer };
      }
      return true;
    } catch (error) {
      report('context', error);
      return false;
    }
  }

  const sampleState = () => ({ loaded, failed });

  function dispose() {
    if (disposed) return;
    stopVoices(0);
    for (const handle of [...loops]) handle.stop(0);
    disposed = true;
    for (const timer of timers) clearTimeout(timer);
    timers.clear();
    voices.clear();
    loops.clear();
    const closing = ctx;
    ctx = null;
    graph = null;
    reverb = null;
    retiring = null;
    try { Promise.resolve(closing?.close()).catch(() => {}); } catch (_) {}
  }

  return {
    ensureContext, start, suspend, resume, setMusicVolume, setEffectsVolume, loadSamples, fetchBuffer,
    has: group => (samples.get(group)?.length || 0) > 0, play, loop, stopVoices, setReverb,
    bus: name => graph?.[name] || null, activeVoices: () => voices.size, sampleState,
    context: () => ctx, now: () => ctx?.currentTime ?? 0, dispose
  };
}
