// Looping music with a 1.8 s crossfade, ducking and at most two decoded tracks kept in memory.
const FADE = 1.8;
const MAX_TRACK_BYTES = 4 * 1024 * 1024;

export function createMusic(engine, { onTrack, onError } = {}) {
  let tracks = [];
  let desired = null;
  let active = null;
  let duck = null;
  let generation = 0;
  let reported = false;
  const cache = new Map();
  const fading = new Set();

  function fail(error) {
    if (reported || typeof onError !== 'function') return;
    reported = true;
    try { onError(error instanceof Error ? error : new Error(String(error))); } catch (_) {}
  }

  function setTracks(list) {
    tracks = (Array.isArray(list) ? list : []).filter(track => typeof track?.id === 'string' && typeof track.file === 'string');
    cache.clear();
  }

  function load(track) {
    let promise = cache.get(track.id);
    if (!promise) {
      promise = engine.fetchBuffer(track.file, MAX_TRACK_BYTES);
      cache.set(track.id, promise);
      promise.catch(() => { if (cache.get(track.id) === promise) cache.delete(track.id); });
    }
    return promise;
  }

  // Only the playing track and the one being loaded stay decoded.
  function prune(...keep) {
    for (const id of [...cache.keys()]) if (!keep.includes(id)) cache.delete(id);
  }

  function release(entry, now) {
    const param = entry.level.gain;
    const value = param.value;
    param.cancelScheduledValues(now);
    param.setValueAtTime(value, now);
    param.linearRampToValueAtTime(0, now + FADE);
    entry.source.stop(now + FADE + 0.1);
    fading.add(entry);
  }

  function play(track, buffer) {
    const ctx = engine.context();
    const out = engine.bus('music');
    if (!ctx || !out) return false;
    try {
      const now = ctx.currentTime;
      if (!duck) {
        duck = ctx.createGain();
        duck.gain.setValueAtTime(1, now);
        duck.connect(out);
      }
      const source = ctx.createBufferSource();
      const level = ctx.createGain();
      source.buffer = buffer;
      source.loop = true;
      level.gain.setValueAtTime(0, now);
      level.gain.linearRampToValueAtTime(1, now + FADE);
      source.connect(level);
      level.connect(duck);
      const entry = { id: track.id, track, source, level, startedAt: now, duration: buffer.duration };
      source.onended = () => {
        fading.delete(entry);
        source.disconnect();
        level.disconnect();
      };
      source.start(now);
      if (active) release(active, now);
      active = entry;
      prune(track.id);
      if (typeof onTrack === 'function') {
        try { onTrack({ id: track.id, title: track.title, author: track.author }); } catch (_) {}
      }
      return true;
    } catch (error) {
      fail(error);
      return false;
    }
  }

  // Resolves true once the track plays, false on failure and null when a newer request replaced it.
  async function start(track) {
    const token = ++generation;
    prune(active?.id, track.id);
    let buffer;
    try { buffer = await load(track); }
    catch (error) {
      if (token !== generation) return null;
      fail(error);
      return false;
    }
    return token === generation ? play(track, buffer) : null;
  }

  function select(id) {
    const track = tracks.find(item => item.id === id);
    if (!track) return Promise.resolve(false);
    if (id === desired) return Promise.resolve(true);
    desired = id;
    if (active?.id === id) {
      generation++; // already playing: just drop any pending load of another track
      return Promise.resolve(true);
    }
    return start(track).then(result => {
      if (result === false && desired === id) desired = null;
      return result !== false;
    });
  }

  // Manual override: cycles the list without changing the context that select() tracks.
  function next() {
    if (!tracks.length) return false;
    const index = tracks.findIndex(track => track.id === (active?.id ?? desired));
    void start(tracks[(index + 1) % tracks.length]);
    return true;
  }

  function setDuck(seconds, level) {
    const ctx = engine.context();
    if (!ctx || !duck) return;
    const now = ctx.currentTime;
    duck.gain.cancelScheduledValues(now);
    duck.gain.setTargetAtTime(level, now, 0.05);
    duck.gain.setTargetAtTime(1, now + seconds, 0.35);
  }

  function state() {
    const ctx = engine.context();
    const position = ctx && active && active.duration > 0 ? Math.max(0, ctx.currentTime - active.startedAt) % active.duration : 0;
    return {
      playing: !!active, trackId: active?.id ?? null, track: active?.track.title ?? null,
      trackIndex: active ? tracks.findIndex(track => track.id === active.id) : -1, position
    };
  }

  function stop() {
    generation++;
    desired = null;
    for (const entry of [active, ...fading]) {
      if (!entry) continue;
      try { entry.source.stop(); } catch (_) {}
      entry.source.disconnect();
      entry.level.disconnect();
    }
    active = null;
    fading.clear();
    cache.clear();
    duck?.disconnect();
    duck = null;
  }

  return { setTracks, select, next, duck: setDuck, state, stop };
}
