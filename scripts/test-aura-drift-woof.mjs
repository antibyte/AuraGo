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
const KINDS = ['pass', 'passed', 'hit', 'got_you', 'wreck', 'win'];
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

await check('the line table holds every english line', () => {
    assert.equal(typeof AD.woofLine, 'function');
    assert.equal(typeof AD.woofVoice, 'function');
    assert.equal(typeof AD.woofPrewarm, 'function');
    const keys = AD.woofPrewarm();
    assert.equal(keys.length, DRIVERS.length * KINDS.length + 12, 'line count');
    for (const driver of DRIVERS) {
        for (const kind of KINDS) {
            const key = `aura_drift.banter.${driver}.${kind}`;
            assert.equal(AD.woofLine(key), en[key], key);
        }
    }
    for (let n = 1; n <= 12; n++) {
        const key = `aura_drift.marshal.${n}`;
        assert.equal(AD.woofLine(key), en[key], key);
    }
    /* No line is invented and none is missing. */
    const expected = new Set();
    for (const driver of DRIVERS) for (const kind of KINDS) expected.add(`aura_drift.banter.${driver}.${kind}`);
    for (let n = 1; n <= 12; n++) expected.add(`aura_drift.marshal.${n}`);
    assert.deepEqual(new Set(keys), expected, 'the table and the locale cover the same lines');
    assert.equal(AD.woofLine('aura_drift.banter.nobody.pass'), null, 'an unknown key has no line');
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
    assert.equal(AD.woofLine(out.key), en[out.key], 'the voice speaks the english original');
    banter.dispose();
});

await check('the voice is a setting that can be switched off', () => {
    const defaults = AD.defaultSettings();
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

    voice.say({ key: 'aura_drift.banter.viktor.pass', driverId: 'viktor' });
    await wait(80);
    const speaks = workers[0].sent.filter(m => m.cmd === 'speak');
    assert.equal(speaks.length, 1, 'the line is spoken');
    assert.equal(speaks[0].text, en['aura_drift.banter.viktor.pass'], 'the english original is spoken');
    assert.equal(speaks[0].voice, 'juno', 'viktor has the lower voice');
    assert.equal(played.length, 1, 'the audio is played');
    assert.equal(played[0].rate, 24000, 'the audio runs at 24 kHz');

    /* A line that was spoken once is cached and comes back without the worker. */
    voice.say({ key: 'aura_drift.banter.viktor.pass', driverId: 'viktor' });
    await wait(20);
    assert.equal(workers[0].sent.filter(m => m.cmd === 'speak').length, 1, 'a known line is not spoken again');
    assert.equal(played.length, 2, 'the cached line is played again');

    voice.setEnabled(false);
    assert.equal(voice.status().state, 'off', 'the voice is off');
    voice.say({ key: 'aura_drift.banter.rosi.pass', driverId: 'rosi' });
    await wait(20);
    assert.equal(workers[0].sent.filter(m => m.cmd === 'speak').length, 1, 'an off voice stays quiet');
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
    await wait(40);
    assert.ok(workers[0].sent.some(m => m.cmd === 'speak'), 'the warm up speaks');
    voice.say({ key: 'aura_drift.banter.viktor.win', driverId: 'viktor' });
    await wait(60);
    assert.equal(workers.length, 2, 'the worker is restarted for the line of the player');
    const demand = workers[1].sent.filter(m => m.cmd === 'speak');
    assert.equal(demand.length, 1, 'the line of the player goes first');
    assert.equal(demand[0].text, en['aura_drift.banter.viktor.win'], 'the right line');
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

console.log(`\n${passed} checks passed`);
