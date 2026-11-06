/* Aura Drift - the voice commentary: Underdog Woof speaks the banter lines.
 *
 * The model (ConwayResearch/Underdog-Woof) runs inside a Web Worker, so a synthesis
 * never blocks the frame loop of the race. Its weights are part of the game and are
 * loaded once through the asset loader; every line that was ever spoken is kept as
 * well, in IndexedDB, so the second time a line comes around it is instant.
 *
 * The voice can be switched off completely. Then no worker is started, no weights
 * are loaded and the radio stays a text radio.
 *
 * The model speaks english only, so the commentary always reads the english original
 * of a line, whatever language the game runs in.
 *
 * The model is a single stateful thing, so only one line is spoken at a time. A line
 * the player wants to hear jumps the queue: the warm up job that is running is
 * thrown away, the worker is restarted and the line goes first.
 *
 * Model: ConwayResearch/Underdog-Woof - Underdog Non-Commercial Licence 1.0.
 * Free for non-commercial use with the credit "Voice by Underdog Woof".
 */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    /* The weights are part of the game: the racer never talks to the internet. */
    const MODEL_PATH = '/3d/aura-drift/voice/woof_int8.npz';
    const MODEL_TIMEOUT_MS = 180000;
    const DB_NAME = 'aura-drift-woof';
    const STORE_NAME = 'clips';
    const RATE = 24000;
    /* A line that arrives long after its event would comment on something that is
     * already over, so it is dropped instead of spoken late. */
    const LATE_SECONDS = 20;

    const script = (typeof document !== 'undefined' && document.currentScript && document.currentScript.src) || '';
    const WORKER_URL = script
        ? script.replace(/aura-drift-woof\.js(\?.*)?$/, 'aura-drift-woof-worker.js$1')
        : '/js/desktop/apps/aura-drift-woof-worker.js';

    /* A stable seed per line and wording: the same remark always sounds the same,
     * and every wording of a line has its own voice of it. */
    function seedFor(key, index) {
        let h = 0x811c9dc5;
        const text = key + '#' + (index | 0);
        for (let i = 0; i < text.length; i++) {
            h ^= text.charCodeAt(i);
            h = Math.imul(h, 0x01000193) >>> 0;
        }
        return h % 100000;
    }

    /* A line has several wordings and every wording is a clip of its own. */
    function clipId(key, index) { return key + '#' + (index | 0); }

    function openStore() {
        return new Promise(resolve => {
            if (!window.indexedDB) { resolve(null); return; }
            let req;
            try { req = window.indexedDB.open(DB_NAME, 1); } catch (_) { resolve(null); return; }
            req.onupgradeneeded = () => {
                const db = req.result;
                if (!db.objectStoreNames.contains(STORE_NAME)) db.createObjectStore(STORE_NAME);
            };
            req.onsuccess = () => resolve(req.result);
            req.onerror = () => resolve(null);
            req.onblocked = () => resolve(null);
        });
    }

    function storePut(db, key, value) {
        return new Promise(resolve => {
            if (!db) { resolve(false); return; }
            let tx;
            try { tx = db.transaction(STORE_NAME, 'readwrite'); } catch (_) { resolve(false); return; }
            tx.objectStore(STORE_NAME).put(value, key);
            tx.oncomplete = () => resolve(true);
            tx.onerror = () => resolve(false);
            tx.onabort = () => resolve(false);
        });
    }

    function storeGet(db, key) {
        return new Promise(resolve => {
            if (!db) { resolve(null); return; }
            let tx;
            try { tx = db.transaction(STORE_NAME, 'readonly'); } catch (_) { resolve(null); return; }
            const req = tx.objectStore(STORE_NAME).get(key);
            req.onsuccess = () => resolve(req.result || null);
            req.onerror = () => resolve(null);
        });
    }

    AD.createVoiceCommentary = function (opts) {
        opts = opts || {};
        const audio = opts.audio || null;
        let enabled = opts.enabled !== false;
        let worker = null;
        let booting = null;                 /* the promise of the running boot */
        let resolveBoot = null;
        let modelReady = false;
        let failed = null;
        let progress = 0;
        let racing = false;
        let nextId = 1;
        let running = null;                 /* the job the worker is busy with */
        const clips = new Map();             /* clip id -> Float32Array */
        const ready = new Map();             /* line key -> Set of the wordings it can speak */
        const queue = [];                    /* synthesis jobs */
        let prewarmAt = 0;
        let store = null;
        let generation = 0;                 /* a switch of the voice invalidates the running boot */

        function status() {
            return {
                enabled: enabled,
                state: !enabled ? 'off' : failed ? 'error' : modelReady ? 'ready' : booting ? 'loading' : 'idle',
                progress: progress,
                error: failed,
                cached: clips.size,
                pending: queue.length
            };
        }

        /* The weights: the asset loader knows the way, the game serves them. */
        async function fetchWeights(gen) {
            if (typeof AD.assetsFetchBytes !== 'function') throw new Error('the asset loader is missing');
            const buffer = await AD.assetsFetchBytes(MODEL_PATH, fraction => {
                if (gen !== generation) return;
                progress = fraction;
            }, MODEL_TIMEOUT_MS);
            progress = 1;
            return buffer;
        }

        /* Start the worker and hand it the weights. The promise ends when the
         * worker says that it is ready. */
        function boot() {
            if (!enabled) return Promise.resolve(false);
            if (modelReady) return Promise.resolve(true);
            if (booting) return booting;
            const gen = generation;
            const promise = new Promise(resolve => { resolveBoot = resolve; });
            booting = promise;
            const w = new Worker(WORKER_URL);
            worker = w;
            w.onmessage = onMessage;
            w.onerror = event => {
                if (gen !== generation) return;
                failed = (event && event.message) || 'worker failed';
                modelReady = false;
                booting = null;
                const r = resolveBoot; resolveBoot = null;
                if (r) r(false);
            };
            fetchWeights(gen).then(buffer => {
                if (gen !== generation) { try { w.terminate(); } catch (_) {} return; }
                w.postMessage({ cmd: 'load', buffer: buffer }, [buffer]);
            }).catch(error => {
                try { w.terminate(); } catch (_) {}
                if (worker === w) worker = null;
                if (gen !== generation) return;
                failed = String((error && error.message) || error);
                booting = null;
                const r = resolveBoot; resolveBoot = null;
                if (r) r(false);
            });
            return promise;
        }

        function onMessage(event) {
            const msg = event.data || {};
            if (msg.type === 'ready') {
                modelReady = true;
                progress = 1;
                booting = null;
                const r = resolveBoot; resolveBoot = null;
                if (r) r(true);
                pump();
                return;
            }
            if (msg.type === 'audio') {
                /* A message of a worker that was thrown away belongs to a job that
                 * is not running any more: it must not touch the current one. */
                const job = running && running.id === msg.id ? running : null;
                if (!job) return;
                running = null;
                const id = clipId(job.key, job.index);
                clips.set(id, msg.samples);
                storePut(store, id, msg.samples);
                markReady(job.key, job.index);
                if (job.demand && !job.superseded) {
                    const age = (performance.now() - job.at) / 1000;
                    if (age <= LATE_SECONDS) play(msg.samples);
                }
                pump();
                return;
            }
            if (msg.type === 'error') {
                const job = running && running.id === msg.id ? running : null;
                if (!job) return;
                running = null;
                if (job.demand) failed = String(msg.message || 'the voice failed');
                pump();
            }
        }

        function play(samples) {
            if (!audio || !samples || !samples.length) return;
            if (typeof audio.voice === 'function') audio.voice(samples, RATE);
        }

        /* One synthesis at a time: the model is a single stateful thing. */
        function pump() {
            if (!modelReady || running || !queue.length) return;
            /* A line the player wants to hear always goes before the warm up. */
            let index = queue.findIndex(job => job.demand);
            if (index < 0) {
                if (racing) return;              /* the warm up waits for the race */
                index = 0;
            }
            const job = queue.splice(index, 1)[0];
            running = job;
            worker.postMessage({ cmd: 'speak', id: job.id, text: job.text, voice: job.voice, seed: job.seed });
        }

        /* A line for the player may not wait for the warm up: the warm up job is
         * thrown away, the worker is restarted and the line goes first. */
        function preempt() {
            if (!running || running.demand) return;
            const job = running;
            running = null;
            if (worker) { try { worker.terminate(); } catch (_) {} worker = null; }
            modelReady = false;
            booting = null;
            /* the line is spoken again later, the warm up is not lost */
            job.superseded = false;
            job.at = performance.now();
            queue.unshift(job);
            boot();
        }

        function enqueue(key, index, text, voice, demand) {
            if (clips.has(clipId(key, index))) return null;
            const job = {
                id: nextId++,
                key: key,
                index: index | 0,
                text: text,
                voice: voice,
                seed: seedFor(key, index),
                demand: !!demand,
                superseded: false,
                at: performance.now()
            };
            if (demand) {
                /* Only the newest remark is heard, the older ones are still cached. */
                for (const other of queue) if (other.demand) other.superseded = true;
                queue.unshift(job);
                if (running && !running.demand) preempt();
            } else {
                queue.push(job);
            }
            boot();
            pump();
            return job;
        }

        /* The wordings of a line the voice can speak right now. */
        function readyVariants(key) {
            const set = ready.get(key);
            if (!set || !set.size) return [];
            return Array.from(set).sort((a, b) => a - b);
        }

        function markReady(key, index) {
            let set = ready.get(key);
            if (!set) { set = new Set(); ready.set(key, set); }
            set.add(index | 0);
        }

        /* A remark of the race. The text of the radio is in the game's language, the
         * voice speaks the english original of the very same wording. */
        function say(msg) {
            if (!enabled || !msg || !msg.key) return;
            const index = typeof msg.variant === 'number' && msg.variant >= 0 ? Math.floor(msg.variant) : 0;
            const text = AD.woofLine ? AD.woofLine(msg.key, index) : null;
            if (!text) return;
            const voice = AD.woofVoice ? AD.woofVoice(msg.driverId) : 'juno';
            const clip = clips.get(clipId(msg.key, index));
            if (clip) { play(clip); return; }
            /* The wording is not spoken yet. Another wording of the same line is
             * better than a remark that arrives half a minute late. */
            const known = readyVariants(msg.key);
            if (known.length) {
                play(clips.get(clipId(msg.key, known[0])));
                enqueue(msg.key, index, text, voice, false);
                return;
            }
            enqueue(msg.key, index, text, voice, true);
        }

        /* The lines that were never spoken yet are spoken in the background, so the
         * next race already has them. The first wording of every line comes first,
         * the alternatives follow. */
        function prewarm() {
            if (!enabled || !AD.woofPrewarm) return;
            const list = AD.woofPrewarm();
            for (let i = prewarmAt; i < list.length; i++) {
                const item = list[i] || {};
                const key = item.key || null;
                const index = typeof item.index === 'number' ? item.index : 0;
                if (!key) continue;
                if (clips.has(clipId(key, index))) continue;
                const text = AD.woofLine ? AD.woofLine(key, index) : null;
                if (!text) continue;
                const driverId = key.indexOf('.marshal.') >= 0 ? 'marshal' : key.split('.')[2];
                enqueue(key, index, text, AD.woofVoice ? AD.woofVoice(driverId) : 'juno', false);
            }
            prewarmAt = list.length;
        }

        /* The clips of a previous session are worth more than speaking them again. */
        async function restore() {
            store = await openStore();
            if (!store) return;
            const list = AD.woofPrewarm ? AD.woofPrewarm() : [];
            for (const item of list) {
                const key = item && item.key ? item.key : null;
                const index = item && typeof item.index === 'number' ? item.index : 0;
                if (!key) continue;
                const clip = await storeGet(store, clipId(key, index));
                if (clip && clip.length) {
                    clips.set(clipId(key, index), clip);
                    markReady(key, index);
                }
            }
        }

        function setEnabled(on) {
            const next = !!on;
            if (next === enabled) return;
            enabled = next;
            generation++;
            if (!enabled) {
                /* Stop everything that is running and forget the worker. */
                if (worker) { try { worker.terminate(); } catch (_) {} worker = null; }
                modelReady = false;
                booting = null;
                resolveBoot = null;
                queue.length = 0;
                running = null;
                failed = null;
                progress = 0;
                prewarmAt = 0;
                return;
            }
            failed = null;
            boot();
            restore().then(prewarm);
        }

        function setRacing(on) {
            racing = !!on;
            if (!racing) pump();
        }

        function dispose() {
            generation++;
            enabled = false;
            if (worker) { try { worker.terminate(); } catch (_) {} worker = null; }
            booting = null;
            resolveBoot = null;
            queue.length = 0;
            running = null;
            clips.clear();
            ready.clear();
        }

        if (enabled) {
            /* The weights come first, the clips of the last session decide what is
             * left to speak. */
            boot();
            restore().then(prewarm);
        }

        return {
            say: say,
            setEnabled: setEnabled,
            setRacing: setRacing,
            /* The wordings of a line the voice can speak right now. The banter uses
             * it to pick a wording that is ready and does not have to wait. */
            readyVariants: readyVariants,
            status: status,
            dispose: dispose
        };
    };
})();
