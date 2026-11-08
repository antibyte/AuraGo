// AuraPinball spoken callouts via speechSynthesis. Only on-device voices (localService === true) whose language
// matches the locale are used; otherwise the callout is silently skipped. Text comes from tr('dmd_' + key).

export const MIN_GAP_MS = 1200;

// Pure: pick the best local voice for a locale (exact lang match, then same primary language), or null.
export function pickVoice(voices, locale) {
    const norm = v => String(v || '').toLowerCase().replace('_', '-');
    const want = norm(locale), primary = want.split('-')[0];
    if (!primary) return null;
    const local = (voices || []).filter(v => v && v.localService === true && norm(v.lang).split('-')[0] === primary);
    return local.find(v => norm(v.lang) === want && v.default) || local.find(v => norm(v.lang) === want) || local.find(v => v.default) || local[0] || null;
}

// Pure: what to do with a new callout. 'speak' | 'interrupt' (cancel current, then speak) | 'drop'.
export function calloutDecision({ now, lastAt, lastPriority, speaking, priority }) {
    const recent = lastAt !== null && now - lastAt < MIN_GAP_MS;
    if (!recent && !speaking) return 'speak';
    return priority > lastPriority ? 'interrupt' : 'drop';
}

export function createVoice({ locale = 'en', tr = k => k, enabled = true, synth = globalThis.speechSynthesis, Utterance = globalThis.SpeechSynthesisUtterance, now = () => (globalThis.performance ? performance.now() : Date.now()) } = {}) {
    let loc = locale, on = enabled, lastAt = null, lastPriority = 0, volume = 1;
    const voice = () => { try { return synth && synth.getVoices ? pickVoice(synth.getVoices(), loc) : null; } catch { return null; } };
    return {
        available: () => !!Utterance && !!voice(),
        setLocale(l) { loc = l; },
        setEnabled(v) { on = !!v; if (!on) this.cancel(); },
        setVolume(v) { volume = Math.min(1, Math.max(0, +v || 0)); },
        say(key, priority = 1) {
            const v = on && Utterance ? voice() : null;
            if (!v) return false;
            let text = '';
            try { text = String(tr('dmd_' + key) || ''); } catch { text = ''; }
            if (!text || text === 'dmd_' + key) return false;
            const t = now(), decision = calloutDecision({ now: t, lastAt, lastPriority, speaking: !!synth.speaking, priority });
            if (decision === 'drop') return false;
            try {
                if (decision === 'interrupt') synth.cancel();
                const u = new Utterance(text);
                u.voice = v; u.lang = v.lang; u.volume = volume; u.rate = 1.05;
                synth.speak(u);
            } catch { return false; }
            lastAt = t; lastPriority = priority;
            return true;
        },
        cancel() { try { if (synth) synth.cancel(); } catch { /* ignore */ } },
        dispose() { this.cancel(); on = false; }
    };
}
