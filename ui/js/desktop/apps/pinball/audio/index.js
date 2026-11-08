// AuraPinball audio facade (plan 3.12): engine + SFX + per-table music + voice callouts behind one object.
// Nothing sounds before unlock(), which must run from a user gesture (Start, Training, resume). Safe to import in Node.
import { createEngine } from './engine.js';
import { createSfx } from './sfx.js';
import { createMusic } from './music.js';
import { createVoice } from './voice.js';
import neon from './songs/neon.js';
import leviathan from './songs/leviathan.js';
import temple from './songs/temple.js';

export const SONGS = Object.freeze({ neon, leviathan, temple });

export function createAudio({ tableId = 'neon', settings = {}, assetURL = path => path, locale = 'en', tr = key => key, parts = {} } = {}) {
    const engine = (parts.createEngine || createEngine)({ tableId, assetURL, settings, ...(parts.engine || {}) });
    const voice = (parts.createVoice || createVoice)({ locale, tr, enabled: settings.callouts !== false, ...(parts.voice || {}) });
    voice.setVolume(settings.effects ?? .65);
    let sfx = null, music = null, wanted = 'attract', unlocked = false, paused = false, disposed = false, loading = null, current = settings;

    function build() {
        if (sfx || !engine.ctx) return;
        sfx = createSfx(engine);
        const song = SONGS[tableId] || SONGS.neon;
        music = createMusic(engine.ctx, engine.buses.music, song, { reduced: !!current.reduced });
        engine.startAmbience(tableId);
        loading = engine.loadSamples().catch(() => 0);
        music.section(wanted);
    }

    const api = {
        async unlock() {
            if (disposed) return false;
            const ok = await engine.unlock();
            if (ok && !disposed) { unlocked = true; build(); }
            return ok;
        },
        // Resolves true once the context runs and the samples are loaded; false when audio cannot start.
        async ready() {
            if (disposed || !unlocked) return false;
            if (loading) await loading;
            return engine.inspect().unlocked;
        },
        async resume() { paused = false; if (disposed || !unlocked) return false; return engine.resume(); },
        async pause() { paused = true; voice.cancel(); if (!disposed && unlocked) await engine.suspend(); },
        setSettings(next) {
            current = { ...current, ...next };
            engine.setSettings(current);
            voice.setEnabled(current.callouts !== false);
            voice.setVolume(current.effects ?? .65);
        },
        event(ev) { if (sfx && !paused && ev) sfx.event(ev); },
        cue(name) { if (!sfx || paused) return; sfx.cue(name); if (music) music.stinger(name); },
        music(section) { wanted = section; if (music && !paused) music.section(section); },
        roll(balls) { if (sfx && !paused) sfx.roll(balls); },
        plunger(pull) { if (sfx && !paused) sfx.plunger(pull); },
        magnet(on) { if (sfx && !paused) sfx.magnet(on); },
        say(key, priority) { return !paused && voice.say(key, priority); },
        voiceAvailable: () => voice.available(),
        setLocale(value) { voice.setLocale(value); },
        inspect() {
            const e = engine.inspect();
            return { unlocked: e.unlocked, state: e.state, voices: sfx ? sfx.voices() : 0, buffers: e.buffers, paused, music: music ? music.state.section : 'off', wanted, voice: voice.available() };
        },
        dispose() {
            if (disposed) return;
            disposed = true;
            voice.dispose();
            if (sfx) sfx.dispose();
            if (music) music.dispose();
            engine.dispose();
        }
    };
    return api;
}

export default createAudio;
