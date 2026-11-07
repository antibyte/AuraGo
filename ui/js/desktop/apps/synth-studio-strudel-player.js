// SPDX-License-Identifier: AGPL-3.0-or-later
// Strudel runs only in this opaque-origin, network-disabled frame.
(async function () {
    'use strict';
    const origin = document.querySelector('meta[name="strudel-parent"]').content;
    const send = (type, extra = {}) => parent.postMessage({ strudel: 1, type, ...extra }, origin);
    const sound = document.getElementById('sound');
    let request = 0, pending = false;
    document.addEventListener(strudel.logKey, event => {
        const detail = event.detail;
        if (request && typeof detail?.message === 'string' && (detail.type === 'error' || /^\[[^\]]+\] error:/.test(detail.message))) send('error', { request, message: detail.message.slice(0, 500) });
    });
    try {
        const repl = await strudel.initStrudel({ onEvalError: error => send('error', { request, message: String(error.message).slice(0, 500) }) });
        // Familiar tutorial drums use local synthesis rather than downloaded samples.
        for (const [name, source, decay] of [['bd', 'sbd', .2], ['sd', 'white', .12], ['cp', 'pink', .08], ['hh', 'white', .025]]) {
            const sound = strudel.getSound(source);
            strudel.registerSound(name, (time, value, ended) => sound.onTrigger(time, { attack: .001, decay, sustain: 0, release: .02, ...value }, ended), { type: 'synth' });
        }
        const context = strudel.getAudioContext();
        await strudel.initAudio({ maxPolyphony: 64 });
        sound.onclick = async () => { await context.resume(); sound.hidden = context.state === 'running'; if (sound.hidden) send('playing', { request }); };
        addEventListener('message', async event => {
            const data = event.data;
            if (event.source !== parent || event.origin !== origin || data?.strudel !== 1 || data.type !== 'evaluate' || pending || !Number.isSafeInteger(data.request) || typeof data.code !== 'string' || new TextEncoder().encode(data.code).byteLength > 65536) return;
            request = data.request; pending = true;
            sound.textContent = typeof data.soundLabel === 'string' ? data.soundLabel.slice(0, 160) : '▶';
            try {
                context.resume().catch(() => {});
                await repl.evaluate(data.code);
                if (repl.state.error) return;
                sound.hidden = context.state === 'running';
                send(sound.hidden ? 'playing' : 'sound', { request });
            } catch (error) { send('error', { request, message: String(error.message).slice(0, 500) }); }
            finally { pending = false; }
        });
        send('ready');
    } catch (error) { send('error', { request, message: String(error.message).slice(0, 500) }); }
})();
