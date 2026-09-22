(function () {
    'use strict';
    const base = '/api/desktop/rtl-sdr/', client = crypto.randomUUID(), listeners = new Set();
    let ctx, snapshot, timer, busy = false, wanted = false, generation = 0, heartbeat = 0, badge, error = '', retry;
    const audio = new Audio(); audio.preload = 'none';
    try { audio.volume = Math.max(0, Math.min(1, Number(localStorage.getItem('rtl-sdr.volume') || .7))); } catch (_) { audio.volume = .7; }
    const tr = key => ctx.t('rtlSdr.' + key);
    const request = (path, method = 'GET', body, signal) => ctx.api(base + path, { method, signal, ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }) });
    function publish() {
        if (!badge && ctx) {
            badge = document.createElement('aside'); badge.className = 'sdr-mini'; badge.hidden = true;
            for (const action of ['open', 'mute', 'stop']) { const b = document.createElement('button'); b.type = 'button'; b.dataset.sdrMini = action; badge.append(b); }
            badge.addEventListener('click', e => { const action = e.target.closest('button')?.dataset.sdrMini; if (action === 'open') ctx.openApp('rtl-sdr'); if (action === 'mute') mute(); if (action === 'stop') stop().catch(report); });
            document.body.append(badge);
        }
        if (badge) {
            badge.hidden = !wanted;
            const tune = currentTuning();
            badge.children[0].textContent = '◉ RTL-SDR · ' + (tune?.label || ((tune?.frequency_hz || 0) / 1e6).toFixed(3) + ' MHz');
            badge.children[1].textContent = audio.muted ? '♪' : '∅'; badge.children[1].setAttribute('aria-label', tr(audio.muted ? 'unmute' : 'mute'));
            badge.children[2].textContent = '■'; badge.children[2].setAttribute('aria-label', tr('stop'));
        }
        listeners.forEach(fn => fn(snapshot, error));
    }
    function report(err) { error = err.message || 'sdr_request_failed'; publish(); }
    function currentTuning() { const state = snapshot?.state; return state?.recordings?.find(r => r.id === state.active_job)?.tuning || state?.tuning; }
    function silence() { wanted = false; generation++; clearTimeout(retry); audio.pause(); audio.removeAttribute('src'); audio.load(); publish(); }
    async function refresh() {
        if (!ctx || busy) return snapshot;
        busy = true;
        try {
            snapshot = await request('state');
            if (wanted && (!snapshot.config.enabled || snapshot.state.read_only)) silence();
            if (wanted && Date.now() - heartbeat > 10000) {
                try { await request('heartbeat', 'POST', { client }); heartbeat = Date.now(); }
                catch (err) { silence(); throw err; }
            }
            publish(); return snapshot;
        } finally { busy = false; }
    }
    function init(context) {
        ctx = context;
        if (!timer) timer = setInterval(() => { if (wanted || listeners.size) refresh().catch(report); }, 2000);
        return refresh();
    }
    async function tune(tuning) {
        const epoch = ++generation; error = '';
        await request('tune', 'POST', { client, tuning });
        if (epoch !== generation) { if (!wanted) await request('stop', 'POST', { client }); return; }
        wanted = true; heartbeat = Date.now();
        // Discard buffered audio from the previous frequency or demodulator.
        clearTimeout(retry);
        audio.pause();
        audio.src = base + 'stream?client=' + encodeURIComponent(client) + '&v=' + epoch;
        audio.load();
        try { await audio.play(); } catch (_) { error = 'sdr_audio_unlock'; }
        if ('mediaSession' in navigator) {
            try { navigator.mediaSession.metadata = new MediaMetadata({ title: tuning.label || (tuning.frequency_hz / 1e6).toFixed(3) + ' MHz', artist: 'RTL-SDR' });
                navigator.mediaSession.setActionHandler('stop', () => stop().catch(report));
                navigator.mediaSession.setActionHandler('pause', () => { audio.muted = true; publish(); });
                navigator.mediaSession.setActionHandler('play', () => { audio.muted = false; audio.play().catch(report); publish(); });
            } catch (_) {}
        }
        publish(); await refresh();
    }
    async function stop() { silence(); error = ''; try { await request('stop', 'POST', { client }); } finally { await refresh(); } }
    function mute() { audio.muted = !audio.muted; publish(); }
    function volume(value) { audio.volume = Math.max(0, Math.min(1, Number(value))); try { localStorage.setItem('rtl-sdr.volume', String(audio.volume)); } catch (_) {} publish(); }
    audio.addEventListener('error', () => {
        if (!wanted) return;
        const epoch = generation; clearTimeout(retry);
        retry = setTimeout(() => { if (!wanted || epoch !== generation) return; audio.src = base + 'stream?client=' + encodeURIComponent(client) + '&v=' + Date.now(); audio.play().catch(() => { error = 'sdr_audio_unlock'; publish(); }); }, 3000);
    });
    window.addEventListener('pagehide', () => { silence(); clearInterval(timer); timer = null; if (ctx) request('stop', 'POST', { client }).catch(() => {}); });
    window.RTLSDRRuntime = { init, request, refresh, tune, stop, mute, volume, currentTuning,
        subscribe(fn) { listeners.add(fn); if (snapshot) fn(snapshot, error); return () => listeners.delete(fn); },
        get playing() { return wanted; }, get muted() { return audio.muted; }, get volumeValue() { return audio.volume; }
    };
})();
