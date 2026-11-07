/* Aura Drift - the Underdog Woof voice commentary.
 *
 * Checks the line table against the english locale, the voice assignment, the
 * settings switch, the worker facade and the licence credit.
 *
 * Run: node scripts/test-aura-drift-woof.mjs
 */
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = file => readFileSync(path.join(root, file), 'utf8');
const exists = file => existsSync(path.join(root, file));

let passed = 0;
async function check(name, fn) {
    try {
        await fn();
        passed++;
        console.log('ok   ' + name);
    } catch (error) {
        console.error('FAIL ' + name + '\n' + (error && error.stack || error));
        process.exitCode = 1;
    }
}

const LANGUAGES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const DRIVERS = ['viktor', 'rosi', 'bruno', 'dolores', 'gustav', 'heidi', 'ilse', 'kurt'];
const KINDS = ['pass', 'passed', 'hit', 'got_you', 'wreck', 'win', 'style'];
const MARSHAL_LINES = 16;
/* The drivers of the two voices: Juno is the lower one, Mira the higher one. */
const MALE = ['viktor', 'bruno', 'gustav', 'kurt'];
const FEMALE = ['rosi', 'dolores', 'heidi', 'ilse'];

/* The core, the banter and the line table in one sandbox. */
function loadApp() {
    const sandbox = { window: {}, console };
    vm.createContext(sandbox);
    for (const file of ['aura-drift-core.js', 'aura-drift-banter.js', 'aura-drift-woof-lines.js']) {
        vm.runInContext(read('ui/js/desktop/apps/' + file), sandbox, { filename: file });
    }
    return sandbox.window.AuraDriftCore;
}

const AD = loadApp();
const en = JSON.parse(read('ui/lang/desktop/en.json'));

/* The wordings of a line: one per line of its value. */
const wordings = key => String(en[key]).split('\n').map(text => text.trim()).filter(Boolean);

await check('the line table holds every english line', () => {
    assert.equal(typeof AD.woofLine, 'function');
    assert.equal(typeof AD.woofLines, 'function');
    assert.equal(typeof AD.woofVoice, 'function');
    assert.equal(typeof AD.woofPrewarm, 'function');
    const keys = Object.keys(AD.WOOF_LINES);
    assert.equal(keys.length, DRIVERS.length * KINDS.length + MARSHAL_LINES, 'line count');
    for (const driver of DRIVERS) {
        for (const kind of KINDS) {
            const key = `aura_drift.banter.${driver}.${kind}`;
            assert.equal(JSON.stringify(AD.woofLines(key)), JSON.stringify(wordings(key)), key);
        }
    }
    for (let n = 1; n <= MARSHAL_LINES; n++) {
        const key = `aura_drift.marshal.${n}`;
        assert.equal(JSON.stringify(AD.woofLines(key)), JSON.stringify(wordings(key)), key);
    }
    /* No line is invented and none is missing. */
    const expected = new Set();
    for (const driver of DRIVERS) for (const kind of KINDS) expected.add(`aura_drift.banter.${driver}.${kind}`);
    for (let n = 1; n <= MARSHAL_LINES; n++) expected.add(`aura_drift.marshal.${n}`);
    assert.deepEqual(new Set(keys), expected, 'the table and the locale cover the same lines');
    assert.equal(AD.woofLine('aura_drift.banter.nobody.pass'), null, 'an unknown key has no line');
});

await check('every line has several wordings', () => {
    for (const key of Object.keys(AD.WOOF_LINES)) {
        const texts = AD.woofLines(key);
        assert.ok(texts.length >= 5, key + ' has ' + texts.length + ' wordings');
        assert.equal(new Set(texts).size, texts.length, key + ' has no duplicate wording');
        assert.equal(AD.woofLine(key), texts[0], key + ' speaks its first wording');
        assert.equal(AD.woofLine(key, 2), texts[2], key + ' speaks the wording that is asked for');
        assert.equal(AD.woofLine(key, 99), texts[0], key + ' falls back to the first wording');
    }
});

await check('the warm up speaks every wording, the first wording of every line first', () => {
    const list = AD.woofPrewarm();
    const keys = Object.keys(AD.WOOF_LINES);
    assert.equal(list.length, keys.length * 6, 'every wording of every line');
    for (let i = 0; i < keys.length; i++) {
        assert.equal(JSON.stringify(list[i]), JSON.stringify({ key: keys[i], index: 0 }), 'the first wording comes first');
    }
    assert.equal(JSON.stringify(list[keys.length]), JSON.stringify({ key: keys[0], index: 1 }), 'the alternatives follow');
});

await check('the drivers are split over the two voices', () => {
    for (const driver of MALE) assert.equal(AD.woofVoice(driver), 'juno', driver);
    for (const driver of FEMALE) assert.equal(AD.woofVoice(driver), 'mira', driver);
    assert.equal(AD.woofVoice('marshal'), 'juno', 'marshal');
    assert.equal(AD.woofVoice('nobody'), 'juno', 'an unknown driver falls back');
});

await check('the banter hands the line key to the voice', () => {
    const cars = [{ index: 0, isPlayer: true, driver: null, v: { x: 0, y: 0 }, finished: false, finishTime: 0 }];
    DRIVERS.forEach((id, i) => cars.push({
        index: i + 1, isPlayer: false, finished: false, finishTime: 0,
        driver: { id: id }, v: { x: (i + 1) * 10, y: 0 }
    }));
    const race = { cars: cars, standings: cars.slice(), weapons: true };
    const banter = AD.createBanter({ seed: 5, t: key => 'T:' + key, race: race });
    banter.onEvent({ type: 'hit', car: 0, by: 3, heavy: true }, race);
    let out = null;
    for (let s = 0; s < 1; s += 0.1) out = banter.update(0.1) || out;
    assert.ok(out, 'the banter speaks');
    assert.equal(out.key, `aura_drift.banter.${DRIVERS[2]}.got_you`, 'the key travels with the line');
    assert.equal(AD.woofLine(out.key, out.variant), wordings(out.key)[out.variant], 'the voice speaks the english original');
    banter.dispose();
});

await check('the radio and the voice say the same thing', () => {
    const cars = [{ index: 0, isPlayer: true, driver: null, v: { x: 0, y: 0 }, finished: false, finishTime: 0 }];
    DRIVERS.forEach((id, i) => cars.push({
        index: i + 1, isPlayer: false, finished: false, finishTime: 0,
        driver: { id: id }, v: { x: (i + 1) * 10, y: 0 }
    }));
    const race = { cars: cars, standings: cars.slice(), weapons: true };
    const events = [
        { type: 'hit', car: 0, by: 3, heavy: true },
        { type: 'landing', car: 0, quality: 'crash' },
        { type: 'respawn', car: 0 },
        { type: 'animal_cross' },
        { type: 'style_bank', car: 0, points: 500, multiplier: 3 },
        { type: 'finish', car: 0, time: 90 }
    ];
    for (const lang of LANGUAGES) {
        const locale = JSON.parse(read(`ui/lang/desktop/${lang}.json`));
        const banter = AD.createBanter({
            seed: 21, t: key => (locale[key] || key), race,
            /* every wording is ready, so the radio may pick any of them */
            ready: () => [0, 1, 2, 3, 4, 5]
        });
        const spoken = [];
        for (const evt of events) {
            banter.onEvent(evt, race);
            for (let s = 0; s < 30; s += 0.1) {
                const m = banter.update(0.1);
                if (m) spoken.push(m);
            }
        }
        assert.ok(spoken.length >= 4, lang + ': the race is commented on');
        const win = spoken.filter(m => /^aura_drift\.marshal\.1[3-6]$/.test(m.key));
        assert.equal(win.length, 1, lang + ': the win of the player is announced');
        for (const m of spoken) {
            const local = String(locale[m.key] || '').split('\n');
            assert.equal(local.length, 6, lang + ' ' + m.key + ': the line has six wordings');
            assert.equal(local[m.variant], m.text, lang + ' ' + m.key + ': the radio shows the wording');
            assert.equal(AD.woofLine(m.key, m.variant), wordings(m.key)[m.variant], lang + ' ' + m.key + ': the voice speaks it');
        }
        banter.dispose();
    }
});

await check('the voice is a setting that can be switched off', () => {    const defaults = AD.defaultSettings();
    assert.equal(defaults.voice, true, 'the voice is on by default');
    assert.equal(typeof defaults.voice, 'boolean', 'the setting is a boolean');
    const saved = { ...defaults, voice: false };
    const store = { value: JSON.stringify(saved), getItem() { return this.value; }, setItem(_k, v) { this.value = v; } };
    const sandbox = { window: { AuraDriftCore: null, localStorage: store }, console };
    vm.createContext(sandbox);
    vm.runInContext(read('ui/js/desktop/apps/aura-drift-core.js'), sandbox, { filename: 'aura-drift-core.js' });
    const loaded = sandbox.window.AuraDriftCore.loadSettings();
    assert.equal(loaded.voice, false, 'the voice stays off when it was switched off');
    /* A setting of the wrong type never overwrites the default. */
    store.value = JSON.stringify({ ...defaults, voice: 'yes' });
    assert.equal(sandbox.window.AuraDriftCore.loadSettings().voice, true, 'a broken value is ignored');
});

await check('the facade speaks a line and keeps it', async () => {
    const workers = [];
    class FakeWorker {
        constructor(url) {
            this.url = url;
            this.onmessage = null;
            this.onerror = null;
            this.sent = [];
            this.terminated = false;
            workers.push(this);
        }
        postMessage(msg) {
            this.sent.push(msg);
            if (msg.cmd === 'load') {
                setTimeout(() => this.onmessage && this.onmessage({ data: { type: 'ready', voices: ['juno', 'mira'], rate: 24000 } }), 0);
            } else if (msg.cmd === 'speak') {
                const samples = new Float32Array(2400).fill(0.25);
                setTimeout(() => {
                    if (this.terminated) return;
                    this.onmessage && this.onmessage({ data: { type: 'audio', id: msg.id, rate: 24000, samples } });
                }, 30);
            }
        }
        terminate() { this.terminated = true; }
    }
    const sandbox = {
        /* Without the warm up only the line of the player is spoken. */
        window: { AuraDriftCore: Object.assign({}, AD, {
            woofPrewarm: () => [],
            assetsFetchBytes: (path, onProgress) => {
                if (onProgress) onProgress(1, 1024);
                return Promise.resolve(new ArrayBuffer(1024));
            }
        }) },
        console,
        performance,
        Worker: FakeWorker
    };
    vm.createContext(sandbox);
    vm.runInContext(read('ui/js/desktop/apps/aura-drift-woof.js'), sandbox, { filename: 'aura-drift-woof.js' });
    const played = [];
    const voice = sandbox.window.AuraDriftCore.createVoiceCommentary({
        audio: { voice: (samples, rate) => played.push({ samples: samples, rate: rate }) },
        enabled: true
    });
    const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
    await wait(20);
    assert.equal(voice.status().state, 'ready', 'the worker comes up');
    assert.equal(workers.length, 1, 'one worker');

    voice.say({ key: 'aura_drift.banter.viktor.pass', driverId: 'viktor', variant: 0 });
    await wait(80);
    const speaks = workers[0].sent.filter(m => m.cmd === 'speak');
    assert.equal(speaks.length, 1, 'the line is spoken');
    assert.equal(speaks[0].text, wordings('aura_drift.banter.viktor.pass')[0], 'the english original is spoken');
    assert.equal(speaks[0].voice, 'juno', 'viktor has the lower voice');
    assert.equal(played.length, 1, 'the audio is played');
    assert.equal(played[0].rate, 24000, 'the audio runs at 24 kHz');

    /* A line that was spoken once is cached and comes back without the worker. */
    voice.say({ key: 'aura_drift.banter.viktor.pass', driverId: 'viktor', variant: 0 });
    await wait(20);
    assert.equal(workers[0].sent.filter(m => m.cmd === 'speak').length, 1, 'a known line is not spoken again');
    assert.equal(played.length, 2, 'the cached line is played again');

    /* Another wording of the same line is a clip of its own. */
    voice.say({ key: 'aura_drift.banter.viktor.pass', driverId: 'viktor', variant: 3 });
    await wait(80);
    const speaksAgain = workers[0].sent.filter(m => m.cmd === 'speak');
    assert.equal(speaksAgain.length, 2, 'the other wording is spoken');
    assert.equal(speaksAgain[1].text, wordings('aura_drift.banter.viktor.pass')[3], 'the wording that was asked for');
    assert.equal(JSON.stringify(voice.readyVariants('aura_drift.banter.viktor.pass')), JSON.stringify([0, 3]), 'the voice knows its wordings');
    assert.equal(JSON.stringify(voice.readyVariants('aura_drift.banter.nobody.pass')), JSON.stringify([]), 'a line that was never spoken has no wording');

    voice.setEnabled(false);
    assert.equal(voice.status().state, 'off', 'the voice is off');
    voice.say({ key: 'aura_drift.banter.rosi.pass', driverId: 'rosi' });
    await wait(20);
    assert.equal(workers[0].sent.filter(m => m.cmd === 'speak').length, 2, 'an off voice stays quiet');
    voice.dispose();
});

await check('a line for the player jumps the warm up', async () => {
    const workers = [];
    class SlowWorker {
        constructor(url) {
            this.url = url;
            this.onmessage = null;
            this.onerror = null;
            this.sent = [];
            this.terminated = false;
            workers.push(this);
        }
        postMessage(msg) {
            this.sent.push(msg);
            if (msg.cmd === 'load') {
                setTimeout(() => this.onmessage && this.onmessage({ data: { type: 'ready', voices: ['juno', 'mira'], rate: 24000 } }), 0);
            } else if (msg.cmd === 'speak') {
                const samples = new Float32Array(2400).fill(0.25);
                setTimeout(() => {
                    if (this.terminated) return;
                    this.onmessage && this.onmessage({ data: { type: 'audio', id: msg.id, rate: 24000, samples } });
                }, 400);
            }
        }
        terminate() { this.terminated = true; }
    }
    const sandbox = {
        /* The full line table: the warm up starts on its own. */
        window: { AuraDriftCore: Object.assign({}, AD, {
            assetsFetchBytes: (path, onProgress) => {
                if (onProgress) onProgress(1, 1024);
                return Promise.resolve(new ArrayBuffer(1024));
            }
        }) },
        console,
        performance,
        Worker: SlowWorker
    };
    vm.createContext(sandbox);
    vm.runInContext(read('ui/js/desktop/apps/aura-drift-woof.js'), sandbox, { filename: 'aura-drift-woof.js' });
    const voice = sandbox.window.AuraDriftCore.createVoiceCommentary({ enabled: true });
    const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
    /* The warm up has 432 wordings to offer, so the worker needs a moment. */
    let warm = false;
    for (let i = 0; i < 40 && !warm; i++) {
        await wait(25);
        warm = workers[0].sent.some(m => m.cmd === 'speak');
    }
    assert.ok(warm, 'the warm up speaks');
    voice.say({ key: 'aura_drift.banter.viktor.win', driverId: 'viktor', variant: 0 });
    await wait(60);
    assert.equal(workers.length, 2, 'the worker is restarted for the line of the player');
    const demand = workers[1].sent.filter(m => m.cmd === 'speak');
    assert.equal(demand.length, 1, 'the line of the player goes first');
    assert.equal(demand[0].text, wordings('aura_drift.banter.viktor.win')[0], 'the right line');
    assert.equal(demand[0].voice, 'juno', 'the right voice');
    voice.dispose();
});

await check('the worker is a classic script', () => {
    const worker = read('ui/js/desktop/apps/aura-drift-woof-worker.js');
    const runtime = read('ui/js/desktop/apps/aura-drift-woof-runtime.js');
    const text = read('ui/js/desktop/apps/aura-drift-woof-text.js');
    for (const source of [worker, runtime, text]) {
        assert.ok(!/^\s*(import|export)\s/m.test(source), 'no module syntax');
    }
    assert.ok(runtime.includes('self.AuraDriftWoofRuntime'), 'the runtime is exposed for the node test');
    assert.ok(text.includes('self.AuraDriftWoofText'), 'the text side is exposed');
    assert.ok(worker.includes("cmd === 'speak'"), 'the worker speaks');
    assert.ok(worker.includes('importScripts('), 'the worker pulls the runtime in');
});

await check('the module loader knows the voice modules', () => {
    const loader = read('ui/js/desktop/core/module-loader.js');
    const lines = loader.indexOf("/js/desktop/apps/aura-drift-woof-lines.js");
    const facade = loader.indexOf("/js/desktop/apps/aura-drift-woof.js");
    const shell = loader.indexOf("/js/desktop/apps/aura-drift.js'");
    assert.ok(lines > 0, 'the line table is loaded');
    assert.ok(facade > lines, 'the line table comes before the facade');
    assert.ok(shell > facade, 'the facade comes before the shell');
});

await check('every locale has the voice texts and the credit', () => {
    for (const language of LANGUAGES) {
        const locale = JSON.parse(read(`ui/lang/desktop/${language}.json`));
        assert.ok(locale['aura_drift.voice'], `${language}: the voice label`);
        assert.ok(locale['aura_drift.voice_note'], `${language}: the voice note`);
        assert.ok(String(locale['aura_drift.settings.credits']).includes('Voice by Underdog Woof'),
            `${language}: the licence credit`);
    }
});

await check('the model is only fetched when the voice is on', () => {
    const facade = read('ui/js/desktop/apps/aura-drift-woof.js');
    assert.ok(facade.includes('if (enabled) {'), 'the facade waits for the setting');
    assert.ok(facade.includes('ConwayResearch/Underdog-Woof'), 'the model comes from the model hub');
    assert.ok(facade.includes('/3d/aura-drift/voice/woof_int8.npz'), 'the weights are part of the game');
    assert.ok(exists('ui/js/desktop/apps/aura-drift-woof-worker.js'), 'the worker is there');
    /* The racer stays offline: no voice module may reach out to the internet. */
    for (const file of ['aura-drift-woof.js', 'aura-drift-woof-worker.js', 'aura-drift-woof-runtime.js',
        'aura-drift-woof-text.js', 'aura-drift-woof-lines.js']) {
        const source = read('ui/js/desktop/apps/' + file);
        for (const forbidden of ['https://', 'http://', 'fetch(', 'XMLHttpRequest', 'import(']) {
            assert.ok(!source.includes(forbidden), file + ' must not contain ' + forbidden);
        }
    }
});

await check('an off voice touches neither the network nor a worker', async () => {
    let loads = 0;
    const workers = [];
    class CountingWorker {
        constructor() { workers.push(this); }
        postMessage() {}
        terminate() {}
    }
    const sandbox = {
        window: { AuraDriftCore: Object.assign({}, AD, {
            assetsFetchBytes: () => { loads++; return Promise.reject(new Error('no asset')); }
        }) },
        console,
        performance,
        Worker: CountingWorker
    };
    vm.createContext(sandbox);
    vm.runInContext(read('ui/js/desktop/apps/aura-drift-woof.js'), sandbox, { filename: 'aura-drift-woof.js' });
    const voice = sandbox.window.AuraDriftCore.createVoiceCommentary({ enabled: false });
    await new Promise(resolve => setTimeout(resolve, 30));
    assert.equal(loads, 0, 'nothing is loaded');
    assert.equal(workers.length, 0, 'no worker is started');
    assert.equal(voice.status().state, 'off', 'the voice is off');
    voice.say({ key: 'aura_drift.banter.viktor.pass', driverId: 'viktor' });
    await new Promise(resolve => setTimeout(resolve, 20));
    assert.equal(loads, 0, 'a line does not wake the model');
    assert.equal(workers.length, 0, 'a line does not start a worker');
    voice.dispose();
});

/* The mix of a spoken line: the model speaks quietly, so a line has to be lifted
 * to a level that carries over the race, and the race steps back while it is
 * spoken. The sandbox records what the audio graph is told to do. */
function mixSandbox() {
    const gains = [], sources = [];
    const param = (v = 0) => ({
        value: v, last: undefined,
        setValueAtTime() { return this; },
        linearRampToValueAtTime() { return this; },
        exponentialRampToValueAtTime() { return this; },
        setTargetAtTime(x) { this.last = x; return this; },
        cancelScheduledValues() { return this; }
    });
    class FakeNode {
        constructor(nodeKind = 'node') {
            this.nodeKind = nodeKind;
            this.outs = [];
            for (const k of ['gain', 'frequency', 'detune', 'Q', 'pan', 'playbackRate', 'threshold', 'knee', 'ratio', 'attack', 'release', 'delayTime']) this[k] = param(k === 'playbackRate' ? 1 : 0);
            this.type = 'x';
        }
        connect(n) { this.outs.push(n); return n; }
        disconnect() {}
        start() { this.started = true; }
        stop() { const f = this.onended; this.onended = null; if (f) queueMicrotask(f); }
    }
    const sandbox = { console, setTimeout, clearTimeout, queueMicrotask, Math, Date, JSON, Float32Array, Promise };
    sandbox.window = sandbox;
    sandbox.AudioContext = class {
        constructor() { this.currentTime = 0; this.sampleRate = 48000; this.state = 'running'; this.destination = new FakeNode(); }
        resume() { return Promise.resolve(); }
        suspend() { return Promise.resolve(); }
        close() { this.state = 'closed'; return Promise.resolve(); }
        createBuffer(ch, len, sr) { const data = new Float32Array(len); return { numberOfChannels: ch, length: len, sampleRate: sr, duration: len / sr, getChannelData: () => data }; }
        decodeAudioData(bytes, ok) { const b = this.createBuffer(1, 480, 48000); if (ok) ok(b); return Promise.resolve(b); }
        createBufferSource() { const n = new FakeNode(); sources.push(n); return n; }
        createOscillator() { return new FakeNode('oscillator'); }
        createGain() { const n = new FakeNode('gain'); gains.push(n); return n; }
        createBiquadFilter() { return new FakeNode('filter'); }
        createDynamicsCompressor() { return new FakeNode('compressor'); }
        createConvolver() { return new FakeNode('convolver'); }
        createStereoPanner() { return new FakeNode(); }
        createDelay() { return new FakeNode(); }
        createWaveShaper() { return new FakeNode(); }
        createAnalyser() { return new FakeNode(); }
    };
    vm.createContext(sandbox);
    for (const file of ['aura-drift-core.js', 'aura-drift-audio.js']) {
        vm.runInContext(read('ui/js/desktop/apps/' + file), sandbox, { filename: file });
    }
    return { sandbox, gains, sources };
}

/* A line like the model produces it: quiet on average, with high peaks. */
function modelLine(samples = 96000) {
    const line = new Float32Array(samples);
    for (let i = 0; i < samples; i++) line[i] = 0.363 * ((i % 1000) < 30 ? 1 : 0.03) * Math.sin(i * 0.07);
    return line;
}
function levelOf(samples) {
    let sum = 0, peak = 0;
    for (let i = 0; i < samples.length; i++) {
        sum += samples[i] * samples[i];
        const a = samples[i] < 0 ? -samples[i] : samples[i];
        if (a > peak) peak = a;
    }
    return { rms: Math.sqrt(sum / samples.length), peak: peak };
}

await check('a spoken line is loud enough and the race steps back', async () => {
    const { sandbox, gains, sources } = mixSandbox();
    const audio = sandbox.window.AuraDriftCore.createAudio({ volume: 1, muted: false });
    assert.equal(audio.unlock(), true);
    audio.setView('race');
    /* The master is the gain that feeds the glue compressor (and the compressor that feeds
     * the limiter); the engine, the score and the ambience are the three buses under it. */
    const master = gains.find(g => g.outs.some(o => o.nodeKind === 'compressor' && o.outs.some(p => p.nodeKind === 'compressor')));
    const buses = gains.filter(g => g !== master && g.outs.includes(master) && g.gain.last !== undefined);
    assert.equal(buses.length, 3, 'engine, score and ambience have a level');
    /* The score carries the player's trim, so the buses are told apart by their role and not
     * by their level. */
    const engine = buses.find(g => g.gain.last === 1);
    const ambience = buses.find(g => g.gain.last === 0.8);
    const music = buses.find(g => g !== engine && g !== ambience);
    assert.ok(engine && ambience && music, 'engine, score and ambience are told apart');
    const roles = { engine: engine, music: music, ambience: ambience };
    const before = {};
    for (const [role, g] of Object.entries(roles)) before[role] = g.gain.last;

    /* The score follows the trim of the player: a lower trim lowers the bus. */
    audio.setSettings({ volume: 1, muted: false, musicVolume: 0.5 });
    assert.ok(Math.abs(music.gain.last / before.music - 0.5 / 0.75) < 1e-6,
        `the score follows the trim: ${before.music} -> ${music.gain.last}`);
    audio.setSettings({ volume: 1, muted: false, musicVolume: 0.75 });
    assert.equal(music.gain.last, before.music, 'the score is back at the trim of the player');

    const line = modelLine();
    const quiet = levelOf(line);
    assert.ok(quiet.rms < 0.06, `the model speaks quietly: ${quiet.rms}`);

    const duration = audio.voice(line, 24000);
    assert.equal(duration, 4, 'the line is four seconds long');
    const written = sources[sources.length - 1].buffer.getChannelData(0);
    const loud = levelOf(written);
    assert.ok(loud.rms > quiet.rms * 2, `the line is lifted: ${quiet.rms} -> ${loud.rms}`);
    assert.ok(loud.peak <= 0.96, `the peaks stay under the ceiling: ${loud.peak}`);

    /* While the line is spoken the race steps back: the engine the most, the score a little
     * less, the ambience the least. */
    const during = {};
    for (const [role, g] of Object.entries(roles)) during[role] = g.gain.last;
    const DUCK = { engine: 0.18, music: 0.25, ambience: 0.4 };
    for (const role of Object.keys(roles)) {
        const want = before[role] * DUCK[role];
        assert.ok(Math.abs(during[role] - want) < 1e-6, `${role}: ${before[role]} -> ${during[role]}, want ${want}`);
    }

    /* When the line ends the race comes back. */
    sources[sources.length - 1].stop();
    await new Promise(resolve => setImmediate(resolve));
    const after = {};
    for (const [role, g] of Object.entries(roles)) after[role] = g.gain.last;
    for (const role of Object.keys(roles)) {
        assert.ok(Math.abs(after[role] - before[role]) < 1e-6, `${role} is back: ${after[role]}`);
    }
    audio.dispose();
});

console.log(`\n${passed} checks passed`);
