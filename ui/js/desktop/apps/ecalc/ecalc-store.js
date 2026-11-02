/* ECalc preferences: the last page, favorites, recents, unit choices, significant digits, the E-series, the symbol
   standard and the inputs of every calculator, kept in localStorage under aurago.ecalc.v1. Every storage access is
   wrapped in try/catch: with blocked or full storage the app works, it just forgets. Publishes window.ECalcStore.
   Persisted shape: {last, favorites[], recents[] (at most 12), units{"calc.var": unitId}, sig (3..6), eseries,
   symbols ("iec"|"ansi"), values{calcId: {inputs{var: raw text}, recency[], locked[], choices{}}}}. */
(function () {
    'use strict';

    const KEY = 'aurago.ecalc.v1';
    const SERIES = ['E6', 'E12', 'E24', 'E48', 'E96', 'E192'];
    const MAX_RECENTS = 12, MAX_FAVORITES = 200, MAX_CALCS = 200;

    const isText = value => typeof value === 'string';
    const textList = (value, limit) => [...new Set((Array.isArray(value) ? value : []).filter(isText))].slice(0, limit);
    const textMap = value => {
        const out = {};
        if (value && typeof value === 'object' && !Array.isArray(value)) for (const [key, entry] of Object.entries(value)) if (isText(entry)) out[key] = entry;
        return out;
    };

    function normalize(raw) {
        const source = raw && typeof raw === 'object' ? raw : {};
        const sig = Number(source.sig);
        const values = {};
        if (source.values && typeof source.values === 'object') {
            for (const [id, entry] of Object.entries(source.values).slice(0, MAX_CALCS)) {
                if (!entry || typeof entry !== 'object') continue;
                values[id] = { inputs: textMap(entry.inputs), recency: textList(entry.recency, 50), locked: textList(entry.locked, 50), choices: textMap(entry.choices) };
            }
        }
        return {
            last: isText(source.last) ? source.last : '',
            favorites: textList(source.favorites, MAX_FAVORITES),
            recents: textList(source.recents, MAX_RECENTS),
            units: textMap(source.units),
            sig: Number.isInteger(sig) && sig >= 3 && sig <= 6 ? sig : 4,
            eseries: SERIES.includes(source.eseries) ? source.eseries : 'E24',
            symbols: source.symbols === 'ansi' ? 'ansi' : 'iec',
            values
        };
    }

    // `storage` is injectable for tests; the default is window.localStorage, looked up inside the try block because
    // merely touching it can throw when the browser blocks storage.
    //
    // Several ECalc windows share one key, so no window may write back what it loaded at the start: every change is a
    // read-modify-write (read and normalize what storage holds now, apply only that change, save). prefs() is the
    // in-memory copy, refreshed by each change and, for the default storage, by the storage event of the other windows.
    function create(storage) {
        const backend = () => storage || window.localStorage;
        // What storage holds now, normalized; null when it cannot be read.
        function stored() {
            try {
                const raw = backend().getItem(KEY);
                return normalize(raw ? JSON.parse(raw) : {});
            } catch (error) {
                return null;
            }
        }
        let prefs = stored() || normalize({});
        // Inputs of calculators wait for the typing pause; they are laid over whatever storage holds when they are written.
        const pending = new Map();
        let timer = 0;
        function overlay(base) {
            for (const [id, entry] of pending) base.values[id] = entry;
            return base;
        }
        function commit(change) {
            const base = overlay(stored() || prefs);
            change(base);
            prefs = base;
            pending.clear();
            clearTimeout(timer);
            timer = 0;
            try { backend().setItem(KEY, JSON.stringify(prefs)); } catch (error) { /* blocked or full storage: keep going in memory */ }
        }
        // Keystrokes persist after a short pause; everything else saves at once.
        function saveSoon() {
            clearTimeout(timer);
            timer = setTimeout(flush, 250);
        }
        function flush() {
            if (pending.size) commit(() => {});
        }
        // Another window changed the shared key.
        function onStorage(event) {
            if (event.key !== null && event.key !== KEY) return;
            const fresh = stored();
            if (fresh) prefs = overlay(fresh);
        }
        const listening = !storage && typeof window.addEventListener === 'function';
        if (listening) window.addEventListener('storage', onStorage);
        return {
            prefs: () => prefs,
            set(key, value) {
                commit(base => {
                    const next = normalize(Object.assign({}, base, { [key]: value }));
                    if (Object.prototype.hasOwnProperty.call(next, key)) base[key] = next[key];
                });
            },
            isFavorite: id => prefs.favorites.includes(id),
            // The direction follows what this window shows; the list it is applied to is the one in storage.
            toggleFavorite(id) {
                const on = !prefs.favorites.includes(id);
                commit(base => { base.favorites = on ? textList(base.favorites.concat([id]), MAX_FAVORITES) : base.favorites.filter(entry => entry !== id); });
                return on;
            },
            pushRecent(id) {
                commit(base => {
                    base.recents = [id].concat(base.recents.filter(entry => entry !== id)).slice(0, MAX_RECENTS);
                    base.last = id;
                });
            },
            unitOf: (calcId, varId) => prefs.units[calcId + '.' + varId],
            setUnit(calcId, varId, unit) {
                commit(base => { if (unit) base.units[calcId + '.' + varId] = unit; else delete base.units[calcId + '.' + varId]; });
            },
            calcState: id => prefs.values[id] || null,
            saveCalc(id, state) {
                const entry = { inputs: textMap(state.inputs), recency: textList(state.recency, 50), locked: textList(state.locked, 50), choices: textMap(state.choices) };
                pending.set(id, entry);
                prefs.values[id] = entry;
                saveSoon();
            },
            flush,
            // Writes what is pending and stops listening to the other windows.
            dispose() {
                flush();
                clearTimeout(timer);
                timer = 0;
                if (listening) window.removeEventListener('storage', onStorage);
            }
        };
    }

    window.ECalcStore = { KEY, SERIES, create, normalize };
})();
