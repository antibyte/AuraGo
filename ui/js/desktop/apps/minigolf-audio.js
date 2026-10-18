const MUSIC_PATH = '/3d/minigolf/music/';
const MAX_EFFECT_VOICES = 24;
const ROLLING_ID = 'minigolf-rolling-loop';
const ROLL_REFRESH_MS = 560;
const ROLL_LIFETIME_MS = 950;
const AMBIENT_GAP = [4500, 14000];
const EFFECTS = {
  strike: [[115, 76, 0.7, 130], [10, 92, 0.3, 320]],
  impact: {
    grass: [[115, 62, 0.42, 110]],
    sand: [['drums', 41, 0.5, 300], [116, 47, 0.3, 180, 30]],
    wood: [[115, 72, 0.58, 140], ['drums', 47, 0.24, 110, 20]],
    stone: [['drums', 41, 0.62, 280], [14, 55, 0.36, 320, 40]],
    water: [[99, 84, 0.44, 260], [10, 96, 0.26, 420, 70]]
  },
  rolling: {
    grass: [[115, 53, 0.25, 155]],
    sand: [[116, 45, 0.24, 180]],
    wood: [[115, 64, 0.28, 150]],
    stone: [[14, 52, 0.2, 165]],
    water: [[99, 74, 0.22, 200]]
  },
  splash: [[99, 79, 0.48, 320], [10, 94, 0.24, 420, 70], [99, 72, 0.2, 750, 280]],
  cup: [[14, 79, 0.55, 380], [10, 91, 0.3, 500, 150], [112, 84, 0.22, 900, 330]],
  win: [[10, 72, 0.42, 600], [10, 76, 0.4, 600, 90], [11, 79, 0.36, 700, 180], [10, 84, 0.32, 850, 270], ['drums', 49, 0.5, 1400, 0]]
};
const FX_PARAMS = {
  strike: { tone: 0.62, drive: 0.16 },
  impact_stone: { tone: 0.5, drive: 0.3 },
  impact_wood: { drive: 0.22 },
  impact_sand: { tone: 0.42 },
  rolling: { tone: 0.5 },
  splash: { reverb: 0.35 },
  cup: { reverb: 0.45, tone: 0.55 },
  win: { reverb: 0.3, delay: 0.12 },
  ambient: { reverb: 0.3 }
};
const AMBIENT = {
  garden: [
    [[123, 92, 0.2, 220]],
    [[123, 88, 0.16, 200], [123, 93, 0.18, 240, 220]],
    [[123, 95, 0.14, 180], [123, 91, 0.16, 220, 190], [123, 87, 0.13, 260, 400]]
  ],
  coast: [
    [[122, 55, 0.12, 2600]],
    [[123, 78, 0.15, 260], [123, 74, 0.13, 300, 320]],
    [[122, 50, 0.1, 3000], [123, 76, 0.1, 240, 1500]]
  ],
  night: [
    [[123, 100, 0.09, 90], [123, 100, 0.09, 90, 150], [123, 101, 0.08, 90, 300]],
    [[123, 97, 0.07, 120], [123, 97, 0.07, 120, 170], [123, 96, 0.06, 140, 340]]
  ]
};

const clamp = (value, min, max, fallback) => {
  const number = Number(value);
  return Number.isFinite(number) ? Math.max(min, Math.min(max, number)) : fallback;
};

const nowMs = () => globalThis.performance?.now?.() ?? Date.now();

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
  let lastRollSpeed = 0;
  let ambientWorld = null;
  let ambientTimer = 0;
  let duckTimer = 0;
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

  function restoreMusicVolume() {
    if (duckTimer) { clearTimeout(duckTimer); duckTimer = 0; }
    music?.setVolume?.(musicVolume);
  }

  function duckMusic(milliseconds) {
    if (!music || duckTimer || musicVolume <= 0) return;
    music.setVolume?.(clamp(musicVolume * 0.3, 0, 1, musicVolume));
    duckTimer = setTimeout(() => {
      duckTimer = 0;
      if (!disposed) music?.setVolume?.(musicVolume);
    }, milliseconds);
  }

  function stopAmbient() {
    if (ambientTimer) { clearTimeout(ambientTimer); ambientTimer = 0; }
  }

  function scheduleAmbient() {
    stopAmbient();
    if (disposed || paused || !unlocked || !ambientWorld || effectsVolume <= 0) return;
    const gap = AMBIENT_GAP[0] + Math.random() * (AMBIENT_GAP[1] - AMBIENT_GAP[0]);
    ambientTimer = setTimeout(() => {
      ambientTimer = 0;
      if (disposed || paused || !ambientWorld) return;
      const events = AMBIENT[ambientWorld];
      const notes = events && events[Math.floor(Math.random() * events.length)];
      if (notes) playNotes(notes, { kind: 'ambient', pan: (Math.random() - 0.5) * 1.2, reserve: true });
      scheduleAmbient();
    }, gap);
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
      scheduleAmbient();
      return true;
    })().finally(() => {
      if (disposed) unlockPromise = null;
    });
    return unlockPromise;
  }

  function setMusicVolume(value) {
    musicVolume = clamp(value, 0, 1, musicVolume);
    restoreMusicVolume();
    return musicVolume;
  }

  function setEffectsVolume(value) {
    const wasAudible = effectsVolume > 0;
    effectsVolume = clamp(value, 0, 1, effectsVolume);
    effects?.setVolume?.(effectsVolume);
    if (!wasAudible && effectsVolume > 0) scheduleAmbient();
    else if (effectsVolume <= 0) stopAmbient();
    return effectsVolume;
  }

  function setAmbient(world) {
    const next = AMBIENT[world] ? world : null;
    if (next === ambientWorld) return ambientWorld;
    ambientWorld = next;
    scheduleAmbient();
    return ambientWorld;
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

  function paramsFor(type, options) {
    const surface = String(options.surface || '').toLowerCase();
    const params = (type === 'impact' && surface ? FX_PARAMS[`${type}_${surface}`] : null) || FX_PARAMS[type] || null;
    return options.params ? { ...(params || {}), ...options.params } : params;
  }

  function noteInstrument(program) {
    return program === 'drums' ? 'gm-drums' : `gm-${program}`;
  }

  function speedPitch(program, basePitch, speed) {
    if (program === 'drums') return basePitch;
    return Math.round(clamp(basePitch + speed * 0.35, 0, 127, basePitch));
  }

  function playNotes(notes, options = {}) {
    const speed = clamp(options.speed, 0, 18, 0);
    const pan = clamp(options.pan, -1, 1, 0);
    const kind = options.kind || 'effect';
    const scale = kind === 'strike' || kind === 'impact'
      ? 0.7 + Math.min(1, speed / 14) * 0.35
      : 1;
    const params = paramsFor(kind, options);
    const defaultParams = globalThis.SynthStudioVoices?.DEFAULT_PARAMS || {};
    for (const [index, [program, basePitch, velocity, duration, delay = 0]] of notes.entries()) {
      schedule(() => {
        if (disposed || paused || !effects) return;
        if (effectNotes.size >= MAX_EFFECT_VOICES) {
          if (options.reserve) return;
          const oldest = effectNotes.entries().next().value;
          if (oldest) releaseEffect(...oldest);
          if (disposed || paused) return;
        }
        const id = `${kind}-${generation}-${Date.now()}-${index}-${effectNotes.size}`;
        const track = {
          id: `minigolf-sfx-${kind}`, name: 'Minigolf effect', instrument: noteInstrument(program),
          volume: 0.8, pan, mute: false, solo: false, params: { ...defaultParams, ...(params || {}) }, clips: []
        };
        try {
          const pitched = speedPitch(program, basePitch, speed);
          const loudness = clamp(velocity * scale, 0.03, 1, velocity);
          const release = effects.noteOn(track, pitched, loudness);
          if (typeof release !== 'function') return;
          const note = { release, timer: null };
          note.timer = schedule(() => releaseEffect(id, note), duration);
          effectNotes.set(id, note);
        } catch (error) { report(error); }
      }, delay);
    }
    return true;
  }

  function playEffect(type, options = {}) {
    if (disposed || paused || !effects || effectsVolume <= 0) return false;
    const notes = soundNotes(type, options);
    if (!notes) return false;
    if (type === 'win') duckMusic(2400);
    return playNotes(notes, { ...options, kind: type });
  }

  function effect(type, options = {}) {
    if (disposed || paused) return false;
    if (!unlocked) {
      void unlock().then(ready => { if (ready) playEffect(type, options); });
      return true;
    }
    return playEffect(type, options);
  }

  function rollingPitch(surface, speed) {
    const entry = (EFFECTS.rolling[surface] || EFFECTS.rolling.grass)[0];
    const [program, basePitch] = entry;
    return program === 'drums' ? basePitch : Math.round(clamp(basePitch + Math.min(10, speed * 0.7), 0, 127, basePitch));
  }

  function startRollingNote(surface, speed, pan) {
    if (disposed || paused || !effects || effectsVolume <= 0) return false;
    const entry = EFFECTS.rolling[surface] || EFFECTS.rolling.grass;
    const [program, basePitch, velocity] = entry;
    const existing = effectNotes.get(ROLLING_ID);
    if (existing) releaseEffect(ROLLING_ID, existing);
    if (effectNotes.size >= MAX_EFFECT_VOICES) {
      const oldest = effectNotes.entries().next().value;
      if (oldest) releaseEffect(...oldest);
    }
    const params = { ...(globalThis.SynthStudioVoices?.DEFAULT_PARAMS || {}), ...(FX_PARAMS.rolling || {}) };
    const track = {
      id: 'minigolf-sfx-rolling', name: 'Minigolf rolling', instrument: noteInstrument(program),
      volume: 0.8, pan: clamp(pan, -1, 1, 0), mute: false, solo: false, params, clips: []
    };
    try {
      const loudness = clamp(velocity * (0.55 + Math.min(1, speed / 14) * 0.5), 0.04, 1, velocity);
      const release = effects.noteOn(track, rollingPitch(surface, speed), loudness);
      if (typeof release !== 'function') return false;
      const note = { release, timer: null, surface, pitch: rollingPitch(surface, speed), startedAt: nowMs() };
      note.timer = schedule(() => releaseEffect(ROLLING_ID, note), ROLL_LIFETIME_MS);
      effectNotes.set(ROLLING_ID, note);
      return true;
    } catch (error) { report(error); return false; }
  }

  function rolling(speedValue, surface = 'grass', pan = 0) {
    if (disposed || paused) return false;
    const speed = clamp(speedValue, 0, 18, 0);
    lastRollSpeed = speed;
    if (speed < 0.15) {
      const note = effectNotes.get(ROLLING_ID);
      if (note) releaseEffect(ROLLING_ID, note);
      return false;
    }
    const key = String(surface).toLowerCase();
    const note = effectNotes.get(ROLLING_ID);
    const desired = rollingPitch(key, speed);
    const at = nowMs();
    if (note && note.surface === key && note.pitch === desired && at - note.startedAt < ROLL_REFRESH_MS) return true;
    return startRollingNote(key, speed, pan);
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
    stopAmbient();
    restoreMusicVolume();
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
    scheduleAmbient();
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
    stopAmbient();
    if (duckTimer) { clearTimeout(duckTimer); duckTimer = 0; }
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
      rollingSpeed: lastRollSpeed, ambient: ambientWorld
    };
  }

  return { unlock, setMusicVolume, setEffectsVolume, setAmbient, nextTrack, pause, resume, effect, rolling, dispose, getState };
}
