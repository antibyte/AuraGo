(function () {
    'use strict';
    const BASE = '/api/desktop/personal-radio/';
    const device = (window.crypto && crypto.randomUUID) ? crypto.randomUUID() : 'radio-' + Date.now() + '-' + Math.random().toString(36).slice(2);
    const listeners = new Set();
    let revision = 0, mutations = 0;
    let startingStation = '';
    let ctx, snapshot, player, pollTimer, heartbeatAt = 0, inFlight = false, badge, wanted = false, events = Promise.resolve(), lastError = '';
    const owned = () => snapshot && snapshot.state && snapshot.state.owner === device && snapshot.state.status !== 'stopped';
    const tr = key => ctx && ctx.t ? ctx.t('personalRadio.' + key) : key;
    async function request(path, method, body, signal) {
        const options = { method: method || 'GET', signal };
        if (body != null) { options.headers = { 'Content-Type': 'application/json' }; options.body = JSON.stringify(body); }
        return ctx.api(BASE + path, options);
    }
    function publish() {
        for (const listener of listeners) listener(snapshot, lastError);
        if (!badge && ctx) {
            badge = document.createElement('div'); badge.className = 'pr-mini'; badge.hidden = true;
            badge.innerHTML = '<button type="button" data-pr-mini="open"></button><button type="button" data-pr-mini="pause">Ⅱ</button><button type="button" data-pr-mini="stop">■</button>';
            badge.addEventListener('click', e => {
                const button = e.target.closest('[data-pr-mini]'); if (!button) return;
                if (button.dataset.prMini === 'open') ctx.openApp('personal-radio');
                else control(button.dataset.prMini).catch(showError);
            });
            document.body.appendChild(badge);
        }
        if (badge) {
            badge.hidden = !owned();
            if (owned()) {
                const station = snapshot.stations.find(x => x.id === snapshot.state.station_id);
                badge.querySelector('[data-pr-mini="open"]').textContent = '◉ ' + (station ? station.name : 'Personal Radio');
                const pause = badge.querySelector('[data-pr-mini="pause"]');
                pause.textContent = snapshot.state.status === 'paused' ? '▶' : 'Ⅱ'; pause.title = tr(snapshot.state.status === 'paused' ? 'resume' : 'pause');
                pause.setAttribute('aria-label', pause.title);
                pause.disabled = snapshot.state.status === 'preparing';
                const stop = badge.querySelector('[data-pr-mini="stop"]'); stop.title = tr('stop'); stop.setAttribute('aria-label', tr('stop'));
            }
        }
    }
    function showError(err) { lastError = err.message || 'radio_request_failed'; publish(); }
    function event(id, kind) {
        const state = snapshot && snapshot.state; if (!state || !owned()) return;
        const epoch = state.epoch;
        events = events.then(async () => {
            if (!owned() || snapshot.state.epoch !== epoch) return;
            const next = await request('playback', 'POST', { device, epoch, current: id, kind });
            if (snapshot.state.epoch === epoch) { revision++; snapshot.state = next; if(kind==='started') lastError=''; player.update(next.queue); publish(); }
        }).catch(async err => {
            showError(err);
            if (err.message === 'radio_device_busy') { wanted = false; player.stop(); }
            await refresh().catch(() => {});
        });
    }
    function init(context) {
        ctx = context;
        if (!player) {
            player = new window.PersonalRadioPlayer({
                audio: async (id, offset, length, signal) => {
                    const response = await fetch(BASE + 'audio/' + encodeURIComponent(id) + '?offset=' + offset + '&length=' + length, { credentials: 'same-origin', cache: 'no-store', signal });
                    if (!response.ok) throw Error('radio_audio_failed');
                    return response.arrayBuffer();
                }, event, error: showError, progress: () => listeners.forEach(fn => fn(snapshot, lastError))
            });
            try { player.setVolume(Number(localStorage.getItem('aurago.personal-radio.volume') || 0.8)); } catch (_) {}
            window.addEventListener('pagehide', () => { wanted = false; player.stop(); clearInterval(pollTimer); });
            window.addEventListener('keydown', e => {
                if (!wanted || !owned() || !['MediaPlayPause', 'MediaStop', 'MediaTrackNext'].includes(e.code)) return;
                e.preventDefault(); e.stopImmediatePropagation();
                control(e.code === 'MediaStop' ? 'stop' : e.code === 'MediaTrackNext' ? 'skip' : 'pause').catch(showError);
            }, true);
        }
        if (!pollTimer) pollTimer = setInterval(() => { if (wanted || listeners.size) refresh().catch(showError); }, 2000);
        return refresh();
    }
    function mediaSession() {
        if (!('mediaSession' in navigator) || !owned()) return;
        const position = player.position();
        const segment = snapshot.state.queue.find(x => x.id === position.current) || snapshot.state.queue[0];
        const station = snapshot.stations.find(x => x.id === snapshot.state.station_id);
        try {
            navigator.mediaSession.metadata = new MediaMetadata({ title: segment ? segment.title : station ? station.name : 'Personal Radio', artist: station ? station.name : 'Personal Radio' });
            navigator.mediaSession.setActionHandler('previoustrack', null);
            navigator.mediaSession.playbackState = snapshot.state.status === 'paused' ? 'paused' : 'playing';
            for (const [key, action] of [['play','resume'],['pause','pause'],['stop','stop'],['nexttrack','skip']]) navigator.mediaSession.setActionHandler(key, () => control(action).catch(showError));
        } catch (_) {}
    }
    async function refresh() {
        if (!ctx || inFlight || mutations) return snapshot;
        inFlight = true;
        const expected = revision;
        try {
            const value = await request('state');
            if (expected !== revision || mutations) return snapshot;
            const oldEpoch = snapshot && snapshot.state && snapshot.state.epoch;
            snapshot = value;
            if (wanted && (!owned() || (oldEpoch && oldEpoch !== value.state.epoch))) { wanted = false; player.stop(); window.dispatchEvent(new CustomEvent('personal-radio-stopped')); }
            if (owned() && wanted) {
                const st = value.state; player.update(st.queue);
                if (st.status === 'paused') { if (!player.paused) await player.pause(); }
                else if (st.status === 'ready' || st.status === 'playing' || st.status === 'buffering' || (st.status === 'preparing' && st.queue.some(x => x.opening))) {
                    if (!player.running || player.paused) { try { await player.play(); } catch (err) { lastError = 'radio_audio_unlock'; } }
                }
                if (Date.now() - heartbeatAt > 15000) {
                    const position = player.position();
                    await request('heartbeat','POST',{ device, epoch: st.epoch, current: position.current, position: position.position }); heartbeatAt = Date.now();
                }
                mediaSession();
            }
            publish(); return snapshot;
        } finally { inFlight = false; }
    }
    async function start(id, takeover) {
        if (startingStation) return;
        startingStation = id; lastError = ''; publish();
        mutations++; try {
        await player.unlock(); lastError = '';
        revision++;
        const state = await request('stations/' + id + '/start', 'POST', { device, takeover: !!takeover });
        revision++;
        if (snapshot.state.epoch !== state.epoch) player.reset();
        snapshot.state = state; wanted = true;
        publish();
        } finally { mutations--; startingStation = ''; publish(); }
        await refresh();
    }
    async function control(action) {
        if (!owned()) return;
        mutations++; try {
        await events;
        const state = snapshot.state;
        revision++;
        if (action === 'pause' && state.status === 'paused') action = 'resume';
        if (action === 'resume') {
            const current = state.queue.find(x => x.id === state.current), expires = current && Date.parse(current.expires);
            if (expires > 0 && expires <= Date.now()) player.reset(true);
            await player.unlock();
        }
        if (action === 'pause') await player.pause();
        if (action === 'stop') {
            wanted = false; player.stop();
            window.dispatchEvent(new CustomEvent('personal-radio-stopped'));
        }
        if (action === 'skip') player.reset(true);
        const next = await request('stations/' + state.station_id + '/' + action, 'POST', { device, epoch: state.epoch, current: state.current || (state.queue[0] && state.queue[0].id) || '' });
        revision++; snapshot.state = next; lastError = '';
        if (action === 'skip' || action === 'resume') {
            player.update(next.queue); if(next.status==='paused') await player.pause(); else await player.play();
        }
        publish();
        } finally { mutations--; }
    }
    function volume(value) { player.setVolume(value); try { localStorage.setItem('aurago.personal-radio.volume', String(player.volume)); } catch (_) {} }
    window.PersonalRadioRuntime = { init, request, refresh, start, control, volume, owned,
        subscribe(fn) { listeners.add(fn); if (snapshot) fn(snapshot, lastError); return () => listeners.delete(fn); },
        position: () => player ? player.position() : { position: 0, duration: 0 },
        get volumeValue() { return player ? player.volume : 0.8; },
        get state() { return snapshot; },
        get startingStation() { return startingStation; },
        get active() { return !!(wanted && owned()); }
    };
})();
