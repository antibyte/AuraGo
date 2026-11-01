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
    function create(storage) {
        const backend = () => storage || window.localStorage;
        let prefs;
        try {
            const raw = backend().getItem(KEY);
            prefs = normalize(raw ? JSON.parse(raw) : {});
        } catch (error) {
            prefs = normalize({});
        }
        let timer = 0;
        function save() {
            try { backend().setItem(KEY, JSON.stringify(prefs)); } catch (error) { /* blocked or full storage: keep going in memory */ }
        }
        // Keystrokes persist after a short pause; everything else saves at once.
        function saveSoon() {
            clearTimeout(timer);
            timer = setTimeout(() => { timer = 0; save(); }, 250);
        }
        function flush() {
            if (timer) { clearTimeout(timer); timer = 0; save(); }
        }
        return {
            prefs: () => prefs,
            set(key, value) {
                prefs = normalize(Object.assign({}, prefs, { [key]: value }));
                save();
            },
            isFavorite: id => prefs.favorites.includes(id),
            toggleFavorite(id) {
                const on = !prefs.favorites.includes(id);
                prefs.favorites = on ? textList(prefs.favorites.concat([id]), MAX_FAVORITES) : prefs.favorites.filter(entry => entry !== id);
                save();
                return on;
            },
            pushRecent(id) {
                prefs.recents = [id].concat(prefs.recents.filter(entry => entry !== id)).slice(0, MAX_RECENTS);
                prefs.last = id;
                save();
            },
            unitOf: (calcId, varId) => prefs.units[calcId + '.' + varId],
            setUnit(calcId, varId, unit) {
                if (unit) prefs.units[calcId + '.' + varId] = unit; else delete prefs.units[calcId + '.' + varId];
                save();
            },
            calcState: id => prefs.values[id] || null,
            saveCalc(id, state) {
                prefs.values[id] = { inputs: textMap(state.inputs), recency: textList(state.recency, 50), locked: textList(state.locked, 50), choices: textMap(state.choices) };
                saveSoon();
            },
            flush
        };
    }

    window.ECalcStore = { KEY, SERIES, create, normalize };
})();
