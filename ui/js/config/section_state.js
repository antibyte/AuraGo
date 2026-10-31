// State of a Config section for the sidebar dot and the overview:
// active | configured | error | restart | off | none (precedence in that order
// of urgency: error > restart > active > configured > off). The rules live in
// AuraConfigCatalog.sectionStatePaths; values are read from the DRAFT, so a
// switch that was just toggled changes the state before it is saved.
(function () {
    'use strict';

    const STATUS_KEY = 'aurago.config.sectionStatus.v1';
    const RESTART_KEY = 'aurago.config.restartSections.v1';
    const STATUS_URL = '/api/dashboard/system';
    // The new server counts as restarted when it started this long after the old one.
    const RESTART_TOLERANCE_MS = 10000;
    const NOT_SET = ['disabled', 'none', 'off', 'false'];
    const STATES = Object.freeze(['active', 'configured', 'error', 'restart', 'off', 'none']);

    let sectionKeys = null;

    function text(key, fallback) {
        if (typeof t === 'function') {
            const value = t(key);
            if (value && value !== key) return value;
        }
        return fallback;
    }

    function catalog() {
        return window.AuraConfigCatalog || {};
    }

    function draftValue(path) {
        const state = window.AuraConfigState;
        return state ? state.get(path) : undefined;
    }

    function isMasked(value) {
        if (typeof cfgIsMaskedSecret === 'function') return cfgIsMaskedSecret(value);
        return value === '••••••••';
    }

    // A stored secret reads as the mask and counts as present.
    function hasValue(value) {
        if (isMasked(value)) return true;
        if (Array.isArray(value)) return value.length > 0;
        if (typeof value === 'boolean') return true;
        return value != null && String(value).trim() !== '';
    }

    function isOn(path) {
        if (path.indexOf('set:') === 0) {
            const value = draftValue(path.slice(4));
            if (isMasked(value)) return true;
            return hasValue(value) && NOT_SET.indexOf(String(value).trim().toLowerCase()) < 0;
        }
        return draftValue(path) === true;
    }

    // A nested array lists alternatives: any one of them is enough.
    function present(path) {
        return Array.isArray(path) ? path.some(present) : hasValue(draftValue(path));
    }

    function ruleSatisfied(ruleId) {
        const rule = (catalog().actionRules || {})[ruleId];
        if (!rule) return false;
        const required = rule.requiredPaths || [];
        const credentials = rule.credentialPaths || [];
        if (!required.length && !credentials.length) return false;
        return required.every(present) && credentials.every(present);
    }

    // active | configured | off | none from the draft values alone.
    function baseState(key) {
        const entry = (catalog().sectionStatePaths || {})[key] || {};
        if (entry.none) return 'none';
        const paths = entry.on || [key + '.enabled'];
        if (!entry.on && draftValue(paths[0]) === undefined) return 'none';
        if (paths.some(isOn)) return 'active';
        if (entry.rule && ruleSatisfied(entry.rule)) return 'configured';
        return 'off';
    }

    function readJSON(key) {
        try {
            const value = JSON.parse(sessionStorage.getItem(key) || 'null');
            return value && typeof value === 'object' ? value : null;
        } catch (_) {
            return null;
        }
    }

    function writeJSON(key, value) {
        try {
            if (value == null) sessionStorage.removeItem(key);
            else sessionStorage.setItem(key, JSON.stringify(value));
        } catch (_) { /* private browsing can disable storage */ }
    }

    // {section: {state: 'error'|'degraded', reason, at}} written by feedback.js.
    function readStatus() {
        return readJSON(STATUS_KEY) || {};
    }

    function readRestart() {
        const value = readJSON(RESTART_KEY);
        return value && Array.isArray(value.sections) ? value : null;
    }

    function sectionState(key) {
        const base = baseState(key);
        const status = readStatus();
        const failed = status[key];
        if (failed && (failed.state === 'error' || failed.state === 'degraded')) {
            // A switched-off section is not checked any more, so an old failure is stale.
            if (base === 'off' || base === 'none') {
                delete status[key];
                writeJSON(STATUS_KEY, status);
            } else {
                return { state: 'error', reason: String(failed.reason || '') };
            }
        }
        const restart = readRestart();
        if (restart && restart.sections.indexOf(key) >= 0) {
            return { state: 'restart', reason: text('config.state.restart_reason', 'The change takes effect after a restart.') };
        }
        return { state: base, reason: '' };
    }

    function stateLabel(state) {
        const fallback = { active: 'Active', configured: 'Set up, but off', error: 'Error at the last check', restart: 'Restart needed', off: 'Off', none: '' };
        return text('config.state.' + state, fallback[state] || '');
    }

    // ---- which section owns a path ----------------------------------------------

    function setSections(keys) {
        sectionKeys = new Set(keys || []);
    }

    function activeSection() {
        const state = window.AuraConfigState;
        return state ? state.snapshot().section : '';
    }

    function sectionOfPath(path) {
        const owners = Object.entries(catalog().searchSections || {});
        const owner = owners.find(entry => entry[1].some(prefix => path === prefix || path.indexOf(prefix + '.') === 0));
        if (owner) return owner[0];
        const first = String(path).split('.')[0];
        return !sectionKeys || sectionKeys.has(first) ? first : '';
    }

    function sectionsOfPaths(paths) {
        const found = new Set();
        (paths || []).forEach(path => {
            const key = sectionOfPath(String(path));
            if (key) found.add(key);
        });
        // A path no section claims belongs to the section being edited.
        if (!found.size && paths && paths.length) {
            const active = activeSection();
            if (active && active !== 'overview') found.add(active);
        }
        return [...found];
    }

    // ---- restart marker -----------------------------------------------------------

    function announce() {
        document.dispatchEvent(new CustomEvent('aura-config-section-marker'));
    }

    // Start time (Unix ms) of the running server as the config responses report it.
    let knownBootAt = null;

    function noteServerBoot(value) {
        knownBootAt = typeof value === 'number' && value > 0 ? value : null;
    }

    // Slow fallback for servers that do not report their start time: the dashboard
    // samples the CPU for about a second.
    async function serverBootAt() {
        if (knownBootAt != null) return knownBootAt;
        try {
            const response = await fetch(STATUS_URL, { headers: { Accept: 'application/json' } });
            if (!response.ok) return null;
            const body = await response.json();
            return typeof body.uptime_seconds === 'number' ? Date.now() - body.uptime_seconds * 1000 : null;
        } catch (_) {
            return null;
        }
    }

    // After a save that needs a restart: remember the sections whose paths were
    // changed. The marker keeps the start time of the server that has to restart,
    // taken from the save response (bootAt) or the last config read, and is written
    // together with the marker so a quick restart cannot slip past it.
    async function rememberRestart(paths, bootAt) {
        const sections = sectionsOfPaths(paths);
        if (!sections.length) return;
        const previous = readRestart();
        const merged = new Set(previous ? previous.sections : []);
        sections.forEach(key => merged.add(key));
        const known = typeof bootAt === 'number' && bootAt > 0 ? bootAt : knownBootAt;
        const boot = known != null ? known : (previous && previous.bootAt != null ? previous.bootAt : null);
        writeJSON(RESTART_KEY, { sections: [...merged], bootAt: boot, savedAt: Date.now() });
        announce();
        if (boot != null) return;
        const read = await serverBootAt();
        const current = readRestart();
        if (current && current.bootAt == null && read != null) {
            current.bootAt = read;
            writeJSON(RESTART_KEY, current);
        }
    }

    // True when the marker was cleared because the server has restarted since.
    // A marker without a start time cannot be judged: it is dropped as soon as the
    // current start time is known.
    async function clearRestartIfRestarted() {
        const marker = readRestart();
        if (!marker) return false;
        const bootAt = await serverBootAt();
        if (bootAt == null) return false;
        if (marker.bootAt != null && bootAt <= marker.bootAt + RESTART_TOLERANCE_MS) return false;
        writeJSON(RESTART_KEY, null);
        announce();
        return true;
    }

    window.AuraConfigSectionState = Object.freeze({
        STATES: STATES,
        state: sectionState,
        stateLabel: stateLabel,
        setSections: setSections,
        sectionOfPath: sectionOfPath,
        sectionsOfPaths: sectionsOfPaths,
        readStatus: readStatus,
        rememberRestart: rememberRestart,
        noteServerBoot: noteServerBoot,
        clearRestartIfRestarted: clearRestartIfRestarted
    });
    window.cfgSectionState = sectionState;
})();
