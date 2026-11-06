import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const base = '/img/goldfish-adventure/';
const sources = ['content', 'content-act1', 'content-act2', 'content-act3', 'content-act4', 'state', 'renderer']
    .map(name => readFileSync(resolve(root, 'ui/js/desktop/apps/goldfish-adventure', name + '.js'), 'utf8'));
const freeze = value => {
    if (value && typeof value === 'object') { Object.values(value).forEach(freeze); Object.freeze(value); }
    return value;
};

async function fixture(reduced = false) {
    const frames = new Map(), draws = [], scales = [], loaded = [], sounds = [], activity = [];
    let frameID = 0, time = 0, ticks = 0, writes = 0;
    const sprite = id => ({ src: base + id + '.png', height: 0.3,
        frames: [{ rect: [0, 0, 100, 100], pivot: [0.5, 1] },
            { src: base + id + '-acting.png', rect: [0, 0, 100, 100], pivot: [0.6, 1] }],
        clips: { idle: { frames: [0] }, 'idle-right': { frames: [0] }, wake: { frames: [1] },
            listen: { frames: [1] }, use: { frames: [1] }, talk: { frames: [1] }, react: { frames: [1] } } });
    const manifest = { version: 1, rooms: { R01: { background: base + 'R01.png', audio: base + 'room.ogg',
        actors: [{ id: 'bruno', position: [0.7, 0.83] }] } }, sprites: { r0st: sprite('r0st'), bruno: sprite('bruno') },
        moments: { r01_stop: [
            { duration: 400, actors: { r0st: { clip: 'wake', position: [0.3, 0.83] }, bruno: { clip: 'listen' } }, camera: { zoom: 1.2, x: 0.5, y: 0.5 }, sfx: base + 'wake.ogg' },
            { duration: 600, actors: { r0st: { clip: 'use', position: [0.5, 0.83] }, bruno: { clip: 'talk', position: [0.75, 0.83] } }, camera: { zoom: 1.3, x: 0.58, y: 0.5 }, sfx: base + 'tool.ogg' },
            { duration: 300, actors: { r0st: { clip: 'idle' }, bruno: { clip: 'react' } }, camera: { zoom: 1, x: 0.5, y: 0.5 } }
        ] } };
    class TestImage {
        naturalWidth = 100; naturalHeight = 100;
        set src(value) { this.source = value; if (value) { loaded.push(value); queueMicrotask(() => this.onload?.()); } }
        get src() { return this.source; }
    }
    class TestAudio {
        constructor(src) { this.src = src; this.paused = true; sounds.push(this); }
        play() { this.paused = false; return Promise.resolve(); }
        pause() { this.paused = true; }
        load() {}
        removeAttribute(key) { if (key === 'src') this.src = ''; }
    }
    const painting = { fillRect() {}, save() {}, restore() {}, translate() {},
        scale: (x, y) => scales.push([x, y]), drawImage: (...args) => draws.push(args) };
    const canvas = { clientWidth: 960, getContext: () => painting };
    const context = { window: { matchMedia: () => ({ matches: reduced }), devicePixelRatio: 1 }, TextEncoder,
        fetch: async () => ({ ok: true, json: async () => manifest }), Image: TestImage, Audio: TestAudio, AbortController,
        ResizeObserver: class { observe() {} disconnect() {} },
        requestAnimationFrame: callback => { frames.set(++frameID, callback); return frameID; },
        cancelAnimationFrame: id => frames.delete(id) };
    vm.createContext(context);
    sources.forEach(source => vm.runInContext(source, context));
    const stateAPI = context.window.GoldfishState;
    const reward = stateAPI.apply(stateAPI.create(), 'r01_stop');
    reward.state.settings.sound = true;
    const state = freeze(reward.state), snapshot = JSON.stringify(state);
    const renderer = context.window.GoldfishRenderer.create(canvas, {
        onMoment: value => activity.push(value), onTick: () => ticks++, onPosition: () => writes++
    });
    await renderer.load('R01', state);
    renderer.setPaused(false);
    const advance = async count => {
        for (let index = 0; index < count; index++) {
            time += 16;
            const pending = [...frames.values()]; frames.clear();
            pending.forEach(callback => callback(time));
            await Promise.resolve();
        }
    };
    const unchanged = () => {
        assert.equal(JSON.stringify(state), snapshot, 'Presentation cannot change committed state');
        assert.equal(stateAPI.apply(state, 'r01_stop').changed, false, 'Interrupted scenes cannot repeat rewards');
        assert.equal(writes, 0, 'Cinematic positions never become checkpoint positions');
    };
    return { renderer, advance, unchanged, frames, draws, scales, loaded, sounds, activity, ticks: () => ticks };
}

{
    const env = await fixture();
    let settled = false;
    const promise = env.renderer.act('r01_stop', 'use').then(value => { settled = true; return value; });
    await env.advance(8);
    assert.equal(env.sounds.length, 0, 'Autoplay stays locked before a user gesture');
    env.renderer.unlock();
    env.renderer.setPaused(true);
    const before = env.ticks(), drawCount = env.draws.length;
    await env.advance(80);
    assert.equal(env.ticks(), before, 'Inactive time does not advance the scene');
    assert.equal(env.draws.length, drawCount);
    assert.equal(settled, false, 'Pause preserves the pending scene');
    assert.ok(env.sounds.every(sound => sound.paused));
    env.renderer.setPaused(false);
    await env.advance(32);
    assert.ok(env.loaded.some(src => src.includes('r0st-acting.png')), 'Alternate acting atlases are loaded');
    assert.ok(env.draws.some(args => args[0].src.includes('r0st-acting.png')), 'Alternate frame sources are drawn');
    assert.ok(env.scales.some(([zoom]) => zoom > 1), 'Authored camera moves use the scene timeline');
    assert.ok(env.sounds.some(sound => sound.src.endsWith('tool.ogg')), 'Beat sound runs only after unlocking');
    assert.equal(env.renderer.skipMoment(), true);
    assert.equal(await promise, true);
    assert.deepEqual(env.activity, [true, false], 'Skip settles once and removes scene controls');
    assert.equal(env.renderer.skipMoment(), false);
    env.unchanged();
    env.renderer.dispose();
    assert.equal(env.frames.size, 0);
    assert.ok(env.sounds.every(sound => sound.paused && !sound.src));
}

{
    const env = await fixture();
    const first = env.renderer.act('r01_stop', 'use');
    await env.advance(5);
    const replacement = env.renderer.act('r01_stop', 'use');
    assert.equal(await first, false, 'A superseded scene settles its original waiter');
    await env.advance(100);
    assert.equal(await replacement, true, 'Every authored beat finishes on the active clock');
    env.unchanged();
    const interrupted = env.renderer.act('r01_stop', 'use');
    await env.advance(3);
    env.renderer.dispose();
    assert.equal(await interrupted, false, 'Closing an app resolves the scene immediately');
    const drawCount = env.draws.length;
    await env.advance(100);
    assert.equal(env.draws.length, drawCount, 'Closed scenes cannot be revived by later frames');
    assert.equal(env.frames.size, 0);
    assert.equal(await env.renderer.act('r01_stop', 'use'), false, 'Disposed renderers cannot start a scene');
    env.unchanged();
}

{
    const env = await fixture(true);
    const promise = env.renderer.act('r01_stop', 'use');
    await env.advance(65);
    assert.equal(await promise, true, 'Reduced-motion pose holds still finish');
    assert.ok(env.scales.every(([x, y]) => x === 1 && y === 1), 'Reduced motion never zooms or pans');
    assert.ok(env.draws.some(args => args[0].src.includes('acting.png')), 'Reduced motion retains authored acting poses');
    env.unchanged();
    env.renderer.dispose();
}

console.log('Goldfish animation: authored beats, multi-atlas poses, pause/resume, skip, replacement, close, reduced motion, gesture-gated sound and single rewards passed.');
