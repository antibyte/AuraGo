// SPDX-License-Identifier: AGPL-3.0-or-later
// Strudel runs only in this opaque-origin, network-disabled frame.
(async function () {
    'use strict';
    const origin = document.querySelector('meta[name="strudel-parent"]').content;
    const send = (type, extra = {}) => parent.postMessage({ strudel: 1, type, ...extra }, origin);
    const sound = document.getElementById('sound');
    const noiseBuffers = new WeakMap();
    let request = 0, pending = false;

    const clampDecay = (value, fallback) => {
        const d = Number(value?.decay);
        return Number.isFinite(d) && d > 0 ? Math.max(0.015, d) : fallback;
    };

    function getNoise(ac) {
        let buffer = noiseBuffers.get(ac);
        if (buffer) return buffer;
        buffer = ac.createBuffer(1, ac.sampleRate * 2, ac.sampleRate);
        const data = buffer.getChannelData(0);
        for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1;
        noiseBuffers.set(ac, buffer);
        return buffer;
    }

    function makeNoiseSource(ac, time) {
        const src = ac.createBufferSource();
        src.buffer = getNoise(ac);
        src.loop = true;
        src.start(time);
        return src;
    }

    function finalizeVoice(out, anchor, sources, nodes, ended) {
        let done = false;
        const cleanup = () => {
            if (done) return;
            done = true;
            for (const n of nodes) { try { n.disconnect(); } catch (_) {} }
            ended?.();
        };
        anchor.onended = cleanup;
        return {
            node: out,
            nodes: { source: sources },
            stop(endTime) {
                for (const s of sources) { try { s.stop(endTime); } catch (_) {} }
            }
        };
    }

    function registerDrumKit() {
        const sbd = strudel.getSound('sbd');
        strudel.registerSound('bd', (time, value, ended) => sbd.onTrigger(time, {
            attack: .001, sustain: 0, release: .02, ...value,
            decay: value?.decay ?? .26, pdecay: value?.pdecay ?? .06, penv: value?.penv ?? 30
        }, ended), { type: 'synth' });

        strudel.registerSound('sd', (time, value, ended) => {
            const ac = strudel.getAudioContext(), t = Math.max(time, ac.currentTime), d = clampDecay(value, .15), end = t + Math.max(d, .09) + .02;
            const out = ac.createGain(); out.gain.value = .6;
            const body = ac.createOscillator(), bodyGain = ac.createGain();
            body.type = 'triangle';
            body.frequency.setValueAtTime(320, t);
            body.frequency.exponentialRampToValueAtTime(165, t + .04);
            bodyGain.gain.setValueAtTime(.55, t);
            bodyGain.gain.exponentialRampToValueAtTime(.0001, t + .09);
            body.connect(bodyGain).connect(out);
            body.start(t); body.stop(end);

            const noise = makeNoiseSource(ac, t);
            const hp = ac.createBiquadFilter(); hp.type = 'highpass'; hp.frequency.setValueAtTime(350, t);
            const bp = ac.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.setValueAtTime(1800, t); bp.Q.value = .7;
            const wireGain = ac.createGain();
            wireGain.gain.setValueAtTime(.75, t);
            wireGain.gain.exponentialRampToValueAtTime(.0001, t + d);
            noise.connect(hp).connect(bp).connect(wireGain).connect(out);
            noise.stop(end);

            return finalizeVoice(out, noise, [body, noise], [out, body, bodyGain, noise, hp, bp, wireGain], ended);
        }, { type: 'synth' });

        strudel.registerSound('cp', (time, value, ended) => {
            const ac = strudel.getAudioContext(), t = Math.max(time, ac.currentTime), d = clampDecay(value, .16), end = t + .022 + d + .02;
            const out = ac.createGain(); out.gain.value = .62;
            const noise = makeNoiseSource(ac, t);
            const bp = ac.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.setValueAtTime(1250, t); bp.Q.value = 1.5;
            const env = ac.createGain();
            env.gain.setValueAtTime(.95, t);
            env.gain.exponentialRampToValueAtTime(.18, t + .0105);
            env.gain.setValueAtTime(.95, t + .011);
            env.gain.exponentialRampToValueAtTime(.22, t + .0215);
            env.gain.setValueAtTime(1, t + .022);
            env.gain.exponentialRampToValueAtTime(.0001, t + .022 + d);
            noise.connect(bp).connect(env).connect(out);
            noise.stop(end);

            return finalizeVoice(out, noise, [noise], [out, noise, bp, env], ended);
        }, { type: 'synth' });

        const registerHat = (name, defaultDecay) => strudel.registerSound(name, (time, value, ended) => {
            const ac = strudel.getAudioContext(), t = Math.max(time, ac.currentTime), d = clampDecay(value, defaultDecay), end = t + d + .02;
            const out = ac.createGain(); out.gain.value = .55;
            const hp = ac.createBiquadFilter(); hp.type = 'highpass'; hp.frequency.setValueAtTime(7500, t); hp.Q.value = .9;
            const env = ac.createGain();
            env.gain.setValueAtTime(1, t);
            env.gain.exponentialRampToValueAtTime(.0001, t + d);
            hp.connect(env).connect(out);

            const noise = makeNoiseSource(ac, t), noiseGain = ac.createGain();
            noiseGain.gain.value = .65;
            noise.connect(noiseGain).connect(hp);
            noise.stop(end);

            const osc1 = ac.createOscillator(), osc2 = ac.createOscillator();
            const g1 = ac.createGain(), g2 = ac.createGain();
            osc1.type = osc2.type = 'square';
            osc1.frequency.setValueAtTime(5400, t); osc2.frequency.setValueAtTime(7900, t);
            g1.gain.value = g2.gain.value = .12;
            osc1.connect(g1).connect(hp); osc2.connect(g2).connect(hp);
            osc1.start(t); osc2.start(t); osc1.stop(end); osc2.stop(end);

            return finalizeVoice(out, noise, [noise, osc1, osc2], [out, hp, env, noise, noiseGain, osc1, osc2, g1, g2], ended);
        }, { type: 'synth' });

        registerHat('hh', .045);
        registerHat('oh', .28);

        strudel.registerSound('rim', (time, value, ended) => {
            const ac = strudel.getAudioContext(), t = Math.max(time, ac.currentTime), d = clampDecay(value, .035), end = t + d + .02;
            const out = ac.createGain(); out.gain.value = .55;

            const tri = ac.createOscillator(), triGain = ac.createGain();
            tri.type = 'triangle'; tri.frequency.setValueAtTime(1750, t);
            triGain.gain.setValueAtTime(.55, t);
            triGain.gain.exponentialRampToValueAtTime(.0001, t + Math.min(d, .035));
            tri.connect(triGain).connect(out);
            tri.start(t); tri.stop(end);

            const sine = ac.createOscillator(), sineGain = ac.createGain();
            sine.type = 'sine'; sine.frequency.setValueAtTime(460, t);
            sineGain.gain.setValueAtTime(.65, t);
            sineGain.gain.exponentialRampToValueAtTime(.0001, t + Math.min(d, .03));
            sine.connect(sineGain).connect(out);
            sine.start(t); sine.stop(end);

            const noise = makeNoiseSource(ac, t), hp = ac.createBiquadFilter(), noiseGain = ac.createGain();
            hp.type = 'highpass'; hp.frequency.setValueAtTime(2600, t); hp.Q.value = 1;
            noiseGain.gain.setValueAtTime(.4, t);
            noiseGain.gain.exponentialRampToValueAtTime(.0001, t + d);
            noise.connect(hp).connect(noiseGain).connect(out);
            noise.stop(end);

            return finalizeVoice(out, noise, [tri, sine, noise], [out, tri, triGain, sine, sineGain, noise, hp, noiseGain], ended);
        }, { type: 'synth' });

        strudel.registerSound('cb', (time, value, ended) => {
            const ac = strudel.getAudioContext(), t = Math.max(time, ac.currentTime), d = clampDecay(value, .22), end = t + d + .02;
            const out = ac.createGain(); out.gain.value = .55;
            const bp = ac.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.setValueAtTime(820, t); bp.Q.value = 2.8;
            const env = ac.createGain();
            env.gain.setValueAtTime(.9, t);
            env.gain.exponentialRampToValueAtTime(.22, t + Math.min(.03, d * .4));
            env.gain.exponentialRampToValueAtTime(.0001, t + d);
            bp.connect(env).connect(out);

            const osc1 = ac.createOscillator(), osc2 = ac.createOscillator();
            osc1.type = osc2.type = 'square';
            osc1.frequency.setValueAtTime(540, t); osc2.frequency.setValueAtTime(800, t);
            osc1.connect(bp); osc2.connect(bp);
            osc1.start(t); osc2.start(t); osc1.stop(end); osc2.stop(end);

            return finalizeVoice(out, osc1, [osc1, osc2], [out, bp, env, osc1, osc2], ended);
        }, { type: 'synth' });

        for (const [name, baseFreq] of [['lt', 95], ['mt', 140], ['ht', 205]]) {
            strudel.registerSound(name, (time, value, ended) => {
                const ac = strudel.getAudioContext(), t = Math.max(time, ac.currentTime), d = clampDecay(value, .22), end = t + d + .02;
                const out = ac.createGain(); out.gain.value = .62;
                const osc = ac.createOscillator(), env = ac.createGain();
                osc.type = 'triangle';
                osc.frequency.setValueAtTime(baseFreq * 2.1, t);
                osc.frequency.exponentialRampToValueAtTime(baseFreq, t + .055);
                env.gain.setValueAtTime(.95, t);
                env.gain.exponentialRampToValueAtTime(.0001, t + d);
                osc.connect(env).connect(out);
                osc.start(t); osc.stop(end);

                return finalizeVoice(out, osc, [osc], [out, osc, env], ended);
            }, { type: 'synth' });
        }
    }

    document.addEventListener(strudel.logKey, event => {
        const detail = event.detail;
        if (request && typeof detail?.message === 'string' && (detail.type === 'error' || /^\[[^\]]+\] error:/.test(detail.message))) send('error', { request, message: detail.message.slice(0, 500) });
    });
    try {
        const repl = await strudel.initStrudel({ onEvalError: error => send('error', { request, message: String(error.message).slice(0, 500) }) });
        registerDrumKit();
        const context = strudel.getAudioContext();
        const origAddModule = context.audioWorklet?.addModule?.bind(context.audioWorklet);
        if (origAddModule) {
            context.audioWorklet.addModule = async (url, opts) => {
                if (typeof url !== 'string' || !url.startsWith('data:text/javascript;base64,')) return origAddModule(url, opts);
                const blobUrl = URL.createObjectURL(new Blob([atob(url.slice(28))], { type: 'text/javascript' }));
                try { return await origAddModule(blobUrl, opts); }
                finally { URL.revokeObjectURL(blobUrl); }
            };
        }
        try {
            await strudel.initAudio({ maxPolyphony: 96 });
        } finally {
            if (origAddModule && context.audioWorklet) context.audioWorklet.addModule = origAddModule;
        }
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
