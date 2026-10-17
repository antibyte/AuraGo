const MUSIC_PATH = '/3d/minigolf/music/';
const MAX_EFFECT_VOICES = 24;
const EFFECTS = {
  strike: [[115, 76, 0.7, 130], [10, 92, 0.26, 260]],
  impact: {
    grass: [[115, 62, 0.42, 100]],
    sand: [[116, 49, 0.52, 190]],
    wood: [[115, 72, 0.58, 140]],
    stone: [[14, 58, 0.42, 200]],
    water: [[99, 84, 0.44, 260], [10, 96, 0.22, 360]]
  },
  rolling: {
    grass: [[115, 53, 0.25, 155]],
    sand: [[116, 45, 0.24, 180]],
    wood: [[115, 64, 0.28, 150]],
    stone: [[14, 52, 0.2, 165]],
    water: [[99, 74, 0.22, 200]]
  },
  splash: [[99, 79, 0.48, 320], [10, 94, 0.2, 420]],
  cup: [[14, 79, 0.55, 380], [10, 91, 0.26, 500]],
  win: [[10, 72, 0.42, 600], [10, 76, 0.38, 600, 90], [11, 79, 0.34, 700, 180], [10, 84, 0.3, 850, 270]]
};

const clamp = (value, min, max, fallback) => {
  const number = Number(value);
  return Number.isFinite(number) ? Math.max(min, Math.min(max, number)) : fallback;
};

export function createAudio({ assetURL = value => value, onTrack, onError } = {}) {
  const engine = globalThis.SynthStudioAudio;
  const music = engine?.create?.({ lookAhead: 0.75 }) || null;
  const effects = engine?.create?.() || null;
  let musicVolume = 0.2;
  let effectsVolume = 0.65;
  music?.setVolume?.(musicVolume);
  effects?.setVolume?.(effectsVolume);

  let disposed = false;
  let unlocked = false;
  let paused = false;
  let playing = false;
  let unlockPromise = null;
  let trackIndex = -1;
  let savedTick = 0;
  let generation = 0;
  let playlist = [];
  let lastRollAt = -Infinity;
  let lastRollSpeed = 0;
  const effectNotes = new Map();
  const timers = new Set();

  function report(error) {
    if (disposed || typeof onError !== 'function') return;
    try { onError(error instanceof Error ? error : new Error(String(error))); } catch (_) {}
  }

  function schedule(callback, delay) {
    const timer = setTimeout(() => {
      timers.delete(timer);
      if (!disposed) callback();
    }, delay);
    timers.add(timer);
    return timer;
  }

  async function fetchJSON(file, maxBytes) {
    const response = await fetch(assetURL(`${MUSIC_PATH}${file}`), { cache: 'force-cache' });
    if (!response.ok) throw new Error(`Minigolf audio asset could not be loaded (${response.status}).`);
    const text = await response.text();
    if (text.length > maxBytes) throw new Error('Minigolf audio asset exceeds its size limit.');
    return JSON.parse(text);
  }

  async function loadPlaylist() {
    const manifest = await fetchJSON('manifest.json', 128 * 1024);
    if (manifest?.version !== 1 || !Array.isArray(manifest.tracks) || !manifest.tracks.length || manifest.tracks.length > 16) {
      throw new Error('Minigolf music playlist is invalid.');
    }
    const loaded = await Promise.all(manifest.tracks.map(async track => {
      if (!/^[a-z0-9-]+\.project\.json$/.test(track?.project || '') || typeof track.title !== 'string') return null;
      try {
        const project = await fetchJSON(track.project, 5 * 1024 * 1024);
        if (!Array.isArray(project?.tracks) || project.tracks.length > 16) throw new Error('Minigolf music project is invalid.');
        return { ...track, projectData: project };
      } catch (error) {
        report(error);
        return null;
      }
    }));
    if (disposed) return;
    playlist = loaded.filter(Boolean);
    if (!playlist.length) throw new Error('No minigolf music tracks are available.');
  }

  function currentTick() {
    return playing && music ? clamp(music.currentTick(), 0, 1e9, savedTick) : savedTick;
  }

  function notifyTrack(track) {
    if (typeof onTrack !== 'function') return;
    const metadata = { ...track };
    delete metadata.projectData;
    try { onTrack(metadata); } catch (_) {}
  }

  function playSelected(tick = 0) {
    if (disposed || paused || !unlocked || !music || trackIndex < 0 || trackIndex >= playlist.length) return false;
    const track = playlist[trackIndex];
    const token = ++generation;
    savedTick = clamp(tick, 0, 1e9, 0);
    playing = true;
    notifyTrack(track);
    let result;
    try {
      result = music.play(track.projectData, savedTick, undefined, () => {
        if (disposed || paused || token !== generation) return;
        playing = false;
        savedTick = 0;
        advanceTrack();
      });
    } catch (error) {
      playing = false;
      report(error);
      return false;
    }
    Promise.resolve(result).catch(error => {
      if (disposed || token !== generation) return;
      playing = false;
      report(error);
    });
    return true;
  }

  function stopMusic() {
    if (!music) return savedTick;
    const tick = currentTick();
    generation++;
    playing = false;
    try { savedTick = clamp(music.stop(), 0, 1e9, tick); }
    catch (_) { savedTick = tick; }
    return savedTick;
  }

  function advanceTrack() {
    if (disposed || !playlist.length) return false;
    trackIndex = (trackIndex + 1) % playlist.length;
    savedTick = 0;
    return playSelected(0);
  }

  async function unlock() {
    if (disposed) return false;
    if (unlockPromise) return unlockPromise;
    unlockPromise = (async () => {
      if (!music || !effects) {
        report(new Error('Synth Studio audio is unavailable.'));
        return false;
      }
      const results = await Promise.allSettled([music.unlock(), effects.unlock()]);
      const audioReady = results.some(result => result.status === 'fulfilled' && result.value !== false);
      for (const result of results) if (result.status === 'rejected') report(result.reason);
      if (!audioReady || disposed) return false;
      unlocked = true;
      try { await loadPlaylist(); }
      catch (error) { report(error); }
      if (disposed) return false;
      if (!paused && playlist.length && trackIndex < 0) {
        trackIndex = 0;
        playSelected(0);
      }
      return true;
    })().finally(() => {
      if (disposed) unlockPromise = null;
    });
    return unlockPromise;
  }

  function setMusicVolume(value) {
    musicVolume = clamp(value, 0, 1, musicVolume);
    music?.setVolume?.(musicVolume);
    return musicVolume;
  }

  function setEffectsVolume(value) {
    effectsVolume = clamp(value, 0, 1, effectsVolume);
    effects?.setVolume?.(effectsVolume);
    return effectsVolume;
  }

  function releaseEffect(id, note) {
    if (note.timer) {
      clearTimeout(note.timer);
      timers.delete(note.timer);
    }
    effectNotes.delete(id);
    try { note.release(); } catch (_) {}
  }

  function soundNotes(type, options) {
    if (type === 'impact' || type === 'rolling') {
      const surface = String(options.surface || 'grass').toLowerCase();
      return EFFECTS[type][surface] || EFFECTS[type].grass;
    }
    return EFFECTS[type];
  }

  function playEffect(type, options = {}) {
    if (disposed || paused || !effects || effectsVolume <= 0) return false;
    const notes = soundNotes(type, options);
    if (!notes) return false;
    const speed = clamp(options.speed, 0, 18, 0);
    const pan = clamp(options.pan, -1, 1, 0);
    const defaultParams = globalThis.SynthStudioVoices?.DEFAULT_PARAMS || {};
    for (const [index, [program, basePitch, velocity, duration, delay = 0]] of notes.entries()) {
      schedule(() => {
        if (disposed || paused || effectNotes.size >= MAX_EFFECT_VOICES) {
          const oldest = effectNotes.entries().next().value;
          if (oldest) releaseEffect(...oldest);
          if (disposed || paused) return;
        }
        const id = `${type}-${generation}-${Date.now()}-${index}-${effectNotes.size}`;
        const track = {
          id: `minigolf-sfx-${program}`, name: 'Minigolf effect', instrument: `gm-${program}`,
          volume: 0.8, pan, mute: false, solo: false, params: { ...defaultParams }, clips: []
        };
        try {
          const release = effects.noteOn(track, Math.round(clamp(basePitch + speed * 0.35, 0, 127, basePitch)), velocity);
          if (typeof release !== 'function') return;
          const note = { release, timer: null };
          note.timer = schedule(() => releaseEffect(id, note), duration);
          effectNotes.set(id, note);
        } catch (error) { report(error); }
      }, delay);
    }
    return true;
  }

  function effect(type, options = {}) {
    if (disposed || paused) return false;
    if (!unlocked) {
      void unlock().then(ready => { if (ready) playEffect(type, options); });
      return true;
    }
    return playEffect(type, options);
  }

  function rolling(speedValue, surface = 'grass') {
    if (disposed || paused) return false;
    const speed = clamp(speedValue, 0, 18, 0);
    lastRollSpeed = speed;
    if (speed < 0.15) return false;
    const now = globalThis.performance?.now?.() ?? Date.now();
    if (now - lastRollAt < 120) return false;
    lastRollAt = now;
    return effect('rolling', { speed, surface });
  }

  function releaseEffects() {
    for (const [id, note] of effectNotes) releaseEffect(id, note);
    for (const timer of timers) clearTimeout(timer);
    timers.clear();
  }

  function pause() {
    if (disposed || paused) return savedTick;
    paused = true;
    savedTick = stopMusic();
    releaseEffects();
    lastRollSpeed = 0;
    return savedTick;
  }

  async function resume() {
    if (disposed) return false;
    if (!unlocked) {
      paused = false;
      return false;
    }
    if (!paused) return playing;
    paused = false;
    if (trackIndex < 0 && playlist.length) trackIndex = 0;
    return playSelected(savedTick);
  }

  function nextTrack() {
    if (disposed || !playlist.length) return false;
    if (playing) stopMusic();
    return advanceTrack();
  }

  function dispose() {
    if (disposed) return;
    disposed = true;
    generation++;
    paused = true;
    releaseEffects();
    try { music?.stop(); } catch (_) {}
    try { music?.dispose(); } catch (_) {}
    try { effects?.dispose(); } catch (_) {}
    playlist = [];
    playing = false;
    unlocked = false;
  }

  function getState() {
    return {
      unlocked, playing, paused, disposed, trackIndex,
      track: trackIndex >= 0 ? playlist[trackIndex]?.title || null : null,
      tick: currentTick(), musicVolume, effectsVolume, activeEffects: effectNotes.size,
      rollingSpeed: lastRollSpeed
    };
  }

  return { unlock, setMusicVolume, setEffectsVolume, nextTrack, pause, resume, effect, rolling, dispose, getState };
}
