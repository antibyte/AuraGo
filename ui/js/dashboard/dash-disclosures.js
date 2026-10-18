// AuraGo dashboard – disclosure state of the big cards (window.DashDisclosures, P5). Classic script, loaded with defer
// after dash-sections.js and before main.js. The big cards keep their detail blocks in
// <details class="dash-disclosure" data-disclosure="<card-id>:<key>">, closed in the markup; the open ones are stored
// per browser in localStorage 'dash-disclosures' as { "<card-id>:<key>": true }.
//   init(root)    adds one capture-phase toggle listener on document (toggle does not bubble): it stores the state and,
//                 when a block opens, resizes the Chart.js charts inside it (a chart created in a closed block has no
//                 size yet); then restore(root). main.js#initDashboard calls it before the first section loads.
//   restore(root) opens the stored blocks below root (document when omitted) and never closes one: the toggle event of
//                 a block the user just opened can arrive after a section load finished. main.js#loadSectionContent
//                 calls it after every successful load (CardState may have put a card body back from saved markup).
//   setCount(id, count, title)  shows a positive count in the summary's badge and hides the badge otherwise.
//   parse / serialize  pure and garbage-tolerant. Nothing touches the DOM at load time (ui/testdata/dash-disclosures.cjs).
// Without storage (private mode, quota) the state of the current page view is kept in memory.
(function () {
    'use strict';

    const KEY = 'dash-disclosures';
    const ID_PATTERN = /^card-[a-z0-9-]+:[a-z0-9-]+$/;
    let memory = {};
    let storageOK = true;
    let listening = false;

    function isId(id) {
        return typeof id === 'string' && ID_PATTERN.test(id);
    }

    // Stored text → { "<card-id>:<key>": true } for valid ids; anything else (null, broken JSON, arrays, other values)
    // gives {}.
    function parse(raw) {
        const state = {};
        if (typeof raw !== 'string' || raw === '') return state;
        let value;
        try {
            value = JSON.parse(raw);
        } catch (_) {
            return state;
        }
        if (!value || typeof value !== 'object' || Array.isArray(value)) return state;
        Object.keys(value).forEach(id => {
            if (isId(id) && value[id] === true) state[id] = true;
        });
        return state;
    }

    // { id: true } → stored text with the valid open ids only, sorted, so one state always stores the same text.
    function serialize(state) {
        const out = {};
        if (state && typeof state === 'object' && !Array.isArray(state)) {
            Object.keys(state).filter(id => isId(id) && state[id] === true).sort().forEach(id => { out[id] = true; });
        }
        return JSON.stringify(out);
    }

    function read() {
        if (storageOK) {
            try {
                memory = parse(window.localStorage.getItem(KEY));
            } catch (_) {
                storageOK = false;
            }
        }
        return memory;
    }

    function write(id, open) {
        const next = Object.assign({}, read());
        if (open) next[id] = true;
        else delete next[id];
        memory = next;
        if (!storageOK) return;
        try {
            window.localStorage.setItem(KEY, serialize(next));
        } catch (_) {
            storageOK = false;
        }
    }

    function resizeCharts(details) {
        const chart = window.Chart;
        if (!chart || typeof chart.getChart !== 'function') return;
        details.querySelectorAll('canvas').forEach(canvas => {
            const instance = chart.getChart(canvas);
            if (instance) instance.resize();
        });
    }

    function onToggle(event) {
        const details = event.target;
        if (!details || details.tagName !== 'DETAILS') return;
        const id = details.getAttribute('data-disclosure');
        if (!isId(id)) return;
        write(id, details.open);
        if (details.open) resizeCharts(details);
    }

    function restore(root) {
        const scope = root && typeof root.querySelectorAll === 'function' ? root : document;
        const state = read();
        scope.querySelectorAll('details[data-disclosure]').forEach(details => {
            if (state[details.getAttribute('data-disclosure')] === true && !details.open) details.open = true;
        });
    }

    function init(root) {
        if (!listening) {
            document.addEventListener('toggle', onToggle, true);
            listening = true;
        }
        restore(root);
    }

    function setCount(id, count, title) {
        if (!isId(id)) return;
        const badge = document.querySelector('details[data-disclosure="' + id + '"] > summary [data-disclosure-count]');
        if (!badge) return;
        const n = Number(count);
        const show = Number.isFinite(n) && n > 0;
        badge.textContent = show ? n.toLocaleString(document.documentElement.lang || undefined) : '';
        badge.hidden = !show;
        if (show && title) badge.title = String(title);
        else badge.removeAttribute('title');
    }

    window.DashDisclosures = { KEY, init, restore, parse, serialize, setCount };
})();
