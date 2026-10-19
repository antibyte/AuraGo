import { AUDIO_PATH_RE, clamp, createEngine } from './minigolf-audio-engine.js';
import { createMusic } from './minigolf-audio-music.js';
import { REVERB_PRESETS, createAmbience, createImpulse, playUi } from './minigolf-audio-synth.js';

export const AUDIO_MANIFEST_PATH = '/3d/minigolf/audio/manifest.json';
export const WORLD_ALIAS = { garden: 'temple', coast: 'pirate', night: 'moon' };
export const REQUIRED_GROUPS = [
  'strike', 'impact.wood.light', 'impact.wood.medium', 'impact.wood.heavy', 'impact.stone',
  'impact.metal.light', 'impact.metal.medium', 'impact.metal.heavy', 'impact.soft.light', 'impact.soft.heavy',
  'cup.rim', 'cup.rattle', 'splash', 'cannon', 'cannon.boom', 'boost', 'teleport',
  'roll.soft', 'roll.hard', 'win.pirate', 'win.temple', 'win.moon'
];
export const AUDIO_CREDITS = [
  { title: 'Happy Clappy Loop', author: 'OwlishMedia' },
  { title: 'Pirate Adventure', author: 'Eldritch Grim' },
  { title: 'Hidden Temple', author: 'Eldritch Grim' },
  { title: 'Galactic Temple', author: 'yd' },
  { title: 'Impact Sounds · Sci-Fi Sounds · Music Jingles', author: 'Kenney' },
  { title: 'Bowling Ball Rolling', author: 'qubodup' },
  { title: '30 CC0 SFX Loops', author: 'rubberduck' },
  { title: '6 Short Water Splashes', author: 'ezwa (cut by qubodup)' },
  { title: 'Cannon Fire', author: 'Thimras' }
];

const MANIFEST_LIMIT = 128 * 1024;
const IMPACT_MATERIAL = { wood: 'wood', stone: 'stone', metal: 'metal', grass: 'soft', sand: 'soft' };
const STONE_RATE = { light: 1.08, medium: 1, heavy: 0.86 };
const UI_KIND = { start: 'confirm', resume: 'confirm', 'next-hole': 'confirm', close: 'back', menu: 'back', pause: 'back' };
// Rolling bed per surface: which loop is audible, its playback rate, gain and lowpass cutoff at full speed.
const ROLL = {
  grass: { loop: 'soft', rate: 1, gain: 0.55, cutoff: 2600 },
  sand: { loop: 'soft', rate: 0.8, gain: 0.7, cutoff: 1500 },
  wood: { loop: 'hard', rate: 1, gain: 0.5, cutoff: 5000 },
  stone: { loop: 'hard', rate: 1.1, gain: 0.5, cutoff: 6500 },
  metal: { loop: 'hard', rate: 1.25, gain: 0.45, cutoff: 8000 }
};
const EFFECT_TYPES = new Set(['strike', 'impact', 'cup', 'splash', 'cannon', 'boost', 'teleport', 'win']);

function parseManifest(text) {
  if (text.length > MANIFEST_LIMIT) throw new Error('Minigolf audio manifest exceeds its size limit.');
  const data = JSON.parse(text);
  const valid = list => Array.isArray(list) && list.length <= 16 && list.every(path => typeof path === 'string' && AUDIO_PATH_RE.test(path));
  if (data?.version !== 1 || !data.sfx || typeof data.sfx !== 'object' || !Array.isArray(data.music) || data.music.length > 8) {
    throw new Error('Minigolf audio manifest is invalid.');
  }
  if (!Object.values(data.sfx).every(valid)) throw new Error('Minigolf audio manifest lists an invalid sample path.');
  const music = data.music.filter(track => /^[a-z0-9-]+$/.test(track?.id || '') && AUDIO_PATH_RE.test(track.file || '') && typeof track.title === 'string');
  return { sfx: data.sfx, music: music.map(({ id, file, title, author }) => ({ id, file, title, author: String(author ?? '') })) };
}

export function createAudio({ assetURL = value => value, onTrack, onError, random = Math.random } = {}) {
  let disposed = false;
  let unlocked = false;
  let paused = false;
  let unlockPromise = null;
  let samplesReady = Promise.resolve();
  let musicVolume = 0.2;
  let effectsVolume = 0.65;
  let musicContext = 'menu';
  let worldContext = 'pirate';
  let reverbId = null;
  let ambientWorld = null;
  let ambience = null;
  let loops = null;
  let rollingSpeed = 0;
  let pausedPosition = 0;
  let lastUi = -1;
  const impulses = new Map();

  function report(error) {
    if (disposed || typeof onError !== 'function') return;
    try { onError(error instanceof Error ? error : new Error(String(error))); } catch (_) {}
  }

  const engine = createEngine({ assetURL, onError: report, random });
  const music = createMusic(engine, { onTrack, onError: report });
  engine.setMusicVolume(musicVolume);
  engine.setEffectsVolume(effectsVolume);

  async function fetchManifest() {
    const response = await fetch(assetURL(AUDIO_MANIFEST_PATH), { cache: 'force-cache' });
    if (!response.ok) throw new Error(`Minigolf audio manifest could not be loaded (${response.status}).`);
    return parseManifest(await response.text());
  }

  function startAmbience() {
    const ctx = engine.context();
    if (ambience || !ctx || !ambientWorld || !unlocked || paused || disposed || effectsVolume <= 0) return;
    ambience = createAmbience(ctx, engine.bus('ambience'), ambientWorld, { random });
    ambience.start();
  }

  function stopAmbience(fade = 0.6) {
    ambience?.stop(fade);
    ambience = null;
  }

  function stopRolling(fade = 0.12) {
    if (!loops) return;
    for (const loop of Object.values(loops)) loop?.stop(fade);
    loops = null;
  }

  function applyMusic() {
    music.select(musicContext);
    const ctx = engine.context();
    if (!ctx || reverbId === musicContext) return;
    if (!impulses.has(musicContext)) impulses.set(musicContext, createImpulse(ctx, musicContext));
    if (engine.setReverb(impulses.get(musicContext), REVERB_PRESETS[musicContext].wet)) reverbId = musicContext;
  }

  function unlock() {
    if (disposed) return Promise.resolve(false);
    if (unlockPromise) return unlockPromise;
    const attempt = (async () => {
      try {
        const running = await engine.start();
        if (disposed) return false;
        if (!running) throw new Error('Minigolf audio could not start.');
        const manifest = await fetchManifest();
        if (disposed) return false;
        unlocked = true;
        music.setTracks(manifest.music);
        applyMusic();
        startAmbience();
        samplesReady = engine.loadSamples(manifest.sfx);
        if (paused) void engine.suspend();
        return true;
      } catch (error) {
        report(error);
        return false;
      }
    })();
    unlockPromise = attempt;
    // A failed attempt may be retried by the next user gesture.
    void attempt.then(ready => { if (!ready && unlockPromise === attempt) unlockPromise = null; });
    return attempt;
  }

  function setMusicVolume(value) {
    musicVolume = clamp(value, 0, 1, musicVolume);
    engine.setMusicVolume(musicVolume);
    return musicVolume;
  }

  function setEffectsVolume(value) {
    effectsVolume = clamp(value, 0, 1, effectsVolume);
    engine.setEffectsVolume(effectsVolume);
    if (effectsVolume <= 0) {
      stopAmbience();
      stopRolling();
    } else startAmbience();
    return effectsVolume;
  }

  function setMusic(context) {
    const key = WORLD_ALIAS[context] || context;
    musicContext = REVERB_PRESETS[key] ? key : 'menu';
    if (musicContext !== 'menu') worldContext = musicContext;
    if (unlocked && !disposed) applyMusic();
    return musicContext;
  }

  function setAmbient(world) {
    const key = WORLD_ALIAS[world] || world;
    const next = key === 'pirate' || key === 'temple' || key === 'moon' ? key : null;
    if (next === ambientWorld) return ambientWorld;
    ambientWorld = next;
    stopAmbience();
    startAmbience();
    return ambientWorld;
  }

  function play(group, options) {
    return !!engine.play(group, options);
  }

  function fire(type, options) {
    const pan = clamp(options.pan, -1, 1, 0);
    switch (type) {
      case 'strike': {
        const speed = clamp(options.speed, 0, 12, 0);
        const gain = 0.35 + 0.65 * Math.min(1, speed / 12);
        const hit = play('strike', { gain, rate: 1 + speed / 60, pan, priority: 1 });
        return play('cup.rim', { gain: gain * 0.15, rate: 1.6, pan }) || hit;
      }
      case 'impact': {
        const speed = clamp(options.speed, 0, 40, 0);
        const material = IMPACT_MATERIAL[String(options.surface || '').toLowerCase()] || 'wood';
        if (material === 'soft' && speed < 1.2) return false;
        const tier = speed < 2.5 ? 'light' : speed < 6 ? 'medium' : 'heavy';
        const exact = material === 'stone' ? 'impact.stone' : material === 'soft' ? `impact.soft.${tier === 'heavy' ? 'heavy' : 'light'}` : `impact.${material}.${tier}`;
        const group = [exact, `impact.${material}.light`, `impact.${material}`].find(name => engine.has(name));
        return !!group && play(group, { gain: clamp(0.2 + speed / 10, 0.2, 1, 0.2), rate: material === 'stone' ? STONE_RATE[tier] : 1, pan });
      }
      case 'cup': {
        let started = play('cup.rim', { gain: 0.55, pan, priority: 1 });
        const hits = 2 + Math.floor(random() * 3);
        let delay = 0;
        for (let i = 0; i < hits; i++) {
          delay += 0.07 + random() * 0.06;
          started = play('cup.rattle', { gain: 0.45 * 0.72 ** i, rate: 1.15 + random() * 0.2, delay, pan }) || started;
        }
        started = play('impact.soft.light', { gain: 0.5, delay: delay + 0.09, pan }) || started;
        music.duck(1.4, 0.35);
        return started;
      }
      case 'splash': return play('splash', { gain: 0.45 + 0.4 * Math.min(1, clamp(options.speed, 0, 6, 0) / 6), pan, priority: 1 });
      case 'cannon': {
        const bang = play('cannon', { gain: 0.9, priority: 1 });
        const boom = play('cannon.boom', { gain: 0.6, priority: 1 });
        music.duck(1.2, 0.4);
        return bang || boom;
      }
      case 'boost': return play('boost', { gain: 0.55, rate: 1 + clamp(options.speed, 0, 20, 0) / 40, pan, priority: 1 });
      case 'teleport': return play('teleport', { gain: 0.6, pan, priority: 1 });
      case 'win': {
        const started = play(`win.${worldContext}`, { gain: 0.8, priority: 2 });
        music.duck(3, 0.25);
        return started;
      }
      default: return false;
    }
  }

  function effect(type, options = {}) {
    if (disposed || paused || effectsVolume <= 0 || !EFFECT_TYPES.has(type)) return false;
    if (unlocked) return fire(type, options || {});
    void unlock().then(async ready => {
      if (!ready) return;
      await samplesReady;
      if (!disposed && !paused && effectsVolume > 0) fire(type, options || {});
    });
    return true;
  }

  function rolling(speedValue, surface = 'grass', pan = 0) {
    if (disposed || paused) return false;
    const speed = clamp(speedValue, 0, 40, 0);
    rollingSpeed = speed;
    if (speed < 0.15 || effectsVolume <= 0 || !unlocked) {
      stopRolling();
      return false;
    }
    loops = loops || {};
    loops.soft = loops.soft || engine.loop('roll.soft');
    loops.hard = loops.hard || engine.loop('roll.hard');
    if (!loops.soft && !loops.hard) {
      loops = null;
      return false;
    }
    const profile = ROLL[String(surface).toLowerCase()] || ROLL.grass;
    const n = Math.min(1, speed / 14);
    for (const [name, loop] of Object.entries(loops)) {
      if (!loop) continue;
      const active = name === profile.loop;
      loop.setGain(active ? profile.gain * (0.15 + 0.85 * Math.sqrt(n)) : 0);
      loop.setPan(pan);
      if (active) {
        loop.setRate(profile.rate * (0.55 + 0.9 * n));
        loop.setCutoff(profile.cutoff * (0.45 + 0.55 * n));
      }
    }
    return true;
  }

  function ui(action) {
    if (disposed || paused || !unlocked || effectsVolume <= 0) return false;
    const ctx = engine.context();
    if (!ctx || ctx.currentTime - lastUi < 0.04) return false;
    lastUi = ctx.currentTime;
    playUi(ctx, engine.bus('ui'), UI_KIND[action] || 'click');
    return true;
  }

  function pause() {
    if (disposed) return pausedPosition;
    if (paused) return pausedPosition;
    pausedPosition = music.state().position;
    paused = true;
    rollingSpeed = 0;
    engine.stopVoices(0);
    stopRolling(0);
    stopAmbience(0);
    void engine.suspend();
    return pausedPosition;
  }

  async function resume() {
    if (disposed) return false;
    if (!unlocked) {
      paused = false;
      return false;
    }
    if (paused) {
      paused = false;
      startAmbience();
    }
    const running = await engine.resume();
    return !disposed && running && music.state().playing;
  }

  function nextTrack() {
    return !disposed && unlocked && music.next();
  }

  function dispose() {
    if (disposed) return;
    stopAmbience(0);
    stopRolling(0);
    music.stop();
    engine.dispose();
    disposed = true;
    unlocked = false;
    paused = true;
    impulses.clear();
  }

  function getState() {
    const track = music.state();
    return {
      unlocked, playing: track.playing && !paused && !disposed, paused, disposed,
      trackIndex: track.trackIndex, track: track.track, trackId: track.trackId,
      position: paused ? pausedPosition : track.position, musicVolume, effectsVolume,
      activeEffects: engine.activeVoices(), rollingSpeed, ambient: ambientWorld, music: musicContext,
      samples: engine.sampleState(), contextState: engine.context()?.state ?? 'none'
    };
  }

  return { unlock, setMusic, setMusicVolume, setEffectsVolume, setAmbient, nextTrack, pause, resume, effect, rolling, ui, dispose, getState };
}
