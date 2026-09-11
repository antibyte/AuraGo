(function () {
    'use strict';

    const SAMPLE_RATE = 48000;
    const MAX_SECONDS = 1.2;
    const TARGET_PEAK = 0.707; // -3 dBFS

    const EVENTS = [
        'window.open', 'window.close', 'window.minimize', 'window.restore', 'window.maximize',
        'window.snap', 'window.deny', 'notify.info', 'notify.message', 'notify.error',
        'menu.open', 'menu.close', 'space.switch', 'dialog.open', 'dialog.confirm', 'dialog.cancel',
        'file.trash', 'file.delete', 'file.drop'
    ];

    function clamp(v, lo, hi) {
        return Math.min(hi, Math.max(lo, v));
    }

    function midiToHz(n) {
        return 440 * Math.pow(2, (n - 69) / 12);
    }

    function adsr(gain, t0, attack, decay, sustain, release, peak, ctx) {
        const a = Math.max(0.001, attack || 0.005);
        const d = Math.max(0.001, decay || 0.08);
        const s = clamp(sustain == null ? 0 : sustain, 0, 1);
        const r = Math.max(0.001, release || 0.12);
        const p = peak == null ? 0.5 : peak;
        gain.gain.setValueAtTime(0.0001, t0);
        gain.gain.linearRampToValueAtTime(p, t0 + a);
        gain.gain.linearRampToValueAtTime(Math.max(0.0001, p * s), t0 + a + d);
        gain.gain.setValueAtTime(Math.max(0.0001, p * s), t0 + a + d + 0.0001);
        gain.gain.exponentialRampToValueAtTime(0.0001, t0 + a + d + r);
    }

    function connectFilter(input, spec, ctx) {
        if (!spec) return input;
        const f = ctx.createBiquadFilter();
        f.type = spec.type || 'lowpass';
        f.frequency.value = spec.freq || spec.frequency || 2000;
        if (spec.q != null) f.Q.value = spec.q;
        input.connect(f);
        return f;
    }

    function makeImpulse(ctx, seconds, decay) {
        const len = Math.max(1, Math.floor(ctx.sampleRate * seconds));
        const buf = ctx.createBuffer(2, len, ctx.sampleRate);
        for (let c = 0; c < 2; c++) {
            const ch = buf.getChannelData(c);
            for (let i = 0; i < len; i++) {
                ch[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / len, decay || 2.2);
            }
        }
        return buf;
    }

    function makeReverb(ctx, mix, spec) {
        const wet = clamp(mix == null ? 0.25 : mix, 0, 1);
        if (wet <= 0.001) return { input: null, output: null, wet: 0 };
        const conv = ctx.createConvolver();
        conv.buffer = makeImpulse(ctx, spec && spec.seconds || 0.35, spec && spec.decay || 2.4);
        const dry = ctx.createGain();
        const wetGain = ctx.createGain();
        dry.gain.value = 1 - wet * 0.85;
        wetGain.gain.value = wet;
        return { conv, dry, wetGain };
    }

    function wireReverb(source, reverb, destination) {
        if (!reverb || !reverb.conv) {
            source.connect(destination);
            return;
        }
        source.connect(reverb.dry);
        source.connect(reverb.conv);
        reverb.conv.connect(reverb.wetGain);
        reverb.dry.connect(destination);
        reverb.wetGain.connect(destination);
    }

    function scheduleTone(ctx, dest, layer, t0) {
        const at = t0 + (layer.at || 0);
        const dur = layer.duration || 0.35;
        const gainNode = ctx.createGain();
        const peak = (layer.gain == null ? 0.35 : layer.gain) * (layer.velocity == null ? 1 : layer.velocity);
        adsr(gainNode, at, layer.attack, layer.decay, layer.sustain, layer.release, peak, ctx);
        const partials = layer.partials || [{ ratio: 1, gain: 1, type: layer.type || 'sine' }];
        const merger = ctx.createGain();
        merger.gain.value = 1;
        partials.forEach(p => {
            const osc = ctx.createOscillator();
            osc.type = p.type || layer.type || 'sine';
            const base = layer.freq || midiToHz(layer.midi || 69);
            osc.frequency.setValueAtTime(base * (p.ratio || 1), at);
            if (layer.glide) {
                osc.frequency.exponentialRampToValueAtTime(layer.glide, at + (layer.glideTime || 0.2));
            }
            if (layer.detune) osc.detune.setValueAtTime(layer.detune, at);
            const pg = ctx.createGain();
            pg.gain.value = p.gain == null ? 1 : p.gain;
            osc.connect(pg);
            pg.connect(merger);
            osc.start(at);
            osc.stop(at + dur + (layer.release || 0.2) + 0.05);
        });
        let node = connectFilter(merger, layer.filter, ctx);
        if (layer.pan != null) {
            const pan = ctx.createStereoPanner();
            pan.pan.value = clamp(layer.pan, -1, 1);
            node.connect(pan);
            node = pan;
        }
        wireReverb(node, layer.reverbNode, gainNode);
        gainNode.connect(dest);
    }

    function scheduleFM(ctx, dest, layer, t0, reverbPool) {
        const at = t0 + (layer.at || 0);
        const dur = layer.duration || 0.25;
        const carrier = ctx.createOscillator();
        const mod = ctx.createOscillator();
        const modGain = ctx.createGain();
        const out = ctx.createGain();
        carrier.type = layer.carrierType || 'sine';
        mod.type = layer.modType || 'sine';
        const freq = layer.freq || midiToHz(layer.midi || 60);
        carrier.frequency.setValueAtTime(freq, at);
        mod.frequency.setValueAtTime(layer.modFreq || freq * (layer.modRatio || 2.5), at);
        const index = layer.index || 40;
        modGain.gain.setValueAtTime(index, at);
        if (layer.indexDecay) {
            modGain.gain.exponentialRampToValueAtTime(0.5, at + layer.indexDecay);
        }
        mod.connect(modGain);
        modGain.connect(carrier.frequency);
        adsr(out, at, layer.attack, layer.decay, layer.sustain, layer.release, layer.gain == null ? 0.3 : layer.gain, ctx);
        let node = connectFilter(carrier, layer.filter, ctx);
        if (!layer.reverbNode && layer.reverb != null) {
            layer.reverbNode = reverbPool[layer.reverbKey || 'default'] || null;
        }
        wireReverb(node, layer.reverbNode, out);
        out.connect(dest);
        carrier.start(at);
        mod.start(at);
        carrier.stop(at + dur + 0.1);
        mod.stop(at + dur + 0.1);
    }

    function scheduleNoise(ctx, dest, layer, t0, reverbPool) {
        const at = t0 + (layer.at || 0);
        const dur = layer.duration || 0.15;
        const len = Math.max(1, Math.ceil(ctx.sampleRate * (dur + 0.05)));
        const buf = ctx.createBuffer(1, len, ctx.sampleRate);
        const data = buf.getChannelData(0);
        for (let i = 0; i < len; i++) {
            const t = i / len;
            const pinkish = (Math.random() * 2 - 1) * (layer.color === 'pink' ? (1 - t * 0.4) : 1);
            data[i] = pinkish * Math.pow(1 - t, layer.decayPower || 1.8);
        }
        const src = ctx.createBufferSource();
        src.buffer = buf;
        const out = ctx.createGain();
        adsr(out, at, layer.attack || 0.001, layer.decay || dur * 0.4, 0, layer.release || dur * 0.5, layer.gain == null ? 0.25 : layer.gain, ctx);
        let node = connectFilter(src, layer.filter, ctx);
        if (layer.sweep) {
            const f = ctx.createBiquadFilter();
            f.type = layer.filter && layer.filter.type || 'bandpass';
            f.frequency.setValueAtTime(layer.sweep.from || 400, at);
            f.frequency.exponentialRampToValueAtTime(layer.sweep.to || 2400, at + (layer.sweep.time || dur));
            if (layer.filter && layer.filter.q != null) f.Q.value = layer.filter.q;
            src.connect(f);
            node = f;
        }
        if (!layer.reverbNode && layer.reverb != null) {
            layer.reverbNode = reverbPool[layer.reverbKey || 'default'] || null;
        }
        wireReverb(node, layer.reverbNode, out);
        out.connect(dest);
        src.start(at);
    }

    function schedulePluck(ctx, dest, layer, t0) {
        const at = t0 + (layer.at || 0);
        const freq = layer.freq || midiToHz(layer.midi || 72);
        const dur = layer.duration || 0.45;
        const len = Math.max(2, Math.floor(ctx.sampleRate / freq));
        const periods = clamp(Math.floor(layer.periods || 8), 2, 64);
        const bufLen = len * periods;
        const buf = ctx.createBuffer(1, bufLen, ctx.sampleRate);
        const d = buf.getChannelData(0);
        for (let i = 0; i < bufLen; i++) {
            d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / bufLen, layer.decay || 3);
        }
        const src = ctx.createBufferSource();
        src.buffer = buf;
        src.playbackRate.value = 1;
        const out = ctx.createGain();
        adsr(out, at, 0.001, 0.05, 0, dur, layer.gain == null ? 0.35 : layer.gain, ctx);
        let node = connectFilter(src, Object.assign({ type: 'lowpass', freq: layer.filterFreq || freq * 6 }, layer.filter), ctx);
        wireReverb(node, layer.reverbNode, out);
        out.connect(dest);
        src.start(at);
        src.stop(at + dur + 0.05);
    }

    function scheduleSweep(ctx, dest, layer, t0) {
        scheduleTone(ctx, dest, Object.assign({}, layer, {
            partials: [{ ratio: 1, gain: 1, type: layer.type || 'sawtooth' }],
            glide: layer.toFreq || (layer.freq || 440) * (layer.ratio || 2),
            glideTime: layer.sweepTime || layer.duration || 0.25
        }), t0);
    }

    function scheduleArp(ctx, dest, layer, t0) {
        const notes = layer.notes || [72, 76, 79];
        const step = layer.step || 0.06;
        notes.forEach((n, i) => {
            scheduleTone(ctx, dest, {
                midi: typeof n === 'number' ? n : 72,
                duration: layer.noteDur || 0.12,
                at: (layer.at || 0) + i * step,
                attack: 0.002,
                decay: 0.04,
                sustain: 0,
                release: 0.08,
                gain: (layer.gain == null ? 0.22 : layer.gain) * (layer.velocity == null ? 1 : layer.velocity),
                type: layer.type || 'triangle',
                partials: layer.partials,
                filter: layer.filter,
                reverbNode: layer.reverbNode,
                pan: layer.pan
            }, t0);
        });
    }

    function normalizeBuffer(buffer) {
        let peak = 0;
        for (let c = 0; c < buffer.numberOfChannels; c++) {
            const ch = buffer.getChannelData(c);
            for (let i = 0; i < ch.length; i++) {
                peak = Math.max(peak, Math.abs(ch[i]));
            }
        }
        if (peak <= 0.00001) return { peak: 0, rms: 0 };
        const scale = TARGET_PEAK / peak;
        let sumSq = 0;
        let count = 0;
        for (let c = 0; c < buffer.numberOfChannels; c++) {
            const ch = buffer.getChannelData(c);
            for (let i = 0; i < ch.length; i++) {
                ch[i] *= scale;
                sumSq += ch[i] * ch[i];
                count++;
            }
        }
        return { peak: TARGET_PEAK, rms: Math.sqrt(sumSq / Math.max(1, count)) };
    }

    function renderLayers(layers, seconds) {
        const dur = clamp(seconds || MAX_SECONDS, 0.05, MAX_SECONDS);
        const ctx = new OfflineAudioContext(2, Math.ceil(SAMPLE_RATE * dur), SAMPLE_RATE);
        const master = ctx.createGain();
        master.gain.value = 1;
        master.connect(ctx.destination);
        const reverbPool = {
            default: makeReverb(ctx, 0.22),
            long: makeReverb(ctx, 0.42, { seconds: 0.55, decay: 3.2 }),
            short: makeReverb(ctx, 0.12, { seconds: 0.18, decay: 1.6 }),
            plate: makeReverb(ctx, 0.28, { seconds: 0.4, decay: 2.8 })
        };
        (layers || []).forEach(layer => {
            if (!layer || !layer.kind) return;
            const copy = Object.assign({}, layer);
            if (copy.reverb === 'long') copy.reverbNode = reverbPool.long;
            else if (copy.reverb === 'short') copy.reverbNode = reverbPool.short;
            else if (copy.reverb === 'plate') copy.reverbNode = reverbPool.plate;
            else if (copy.reverb === true || copy.reverb === 'default') copy.reverbNode = reverbPool.default;
            switch (copy.kind) {
                case 'tone': scheduleTone(ctx, master, copy, 0); break;
                case 'fm': scheduleFM(ctx, master, copy, 0, reverbPool); break;
                case 'noise': scheduleNoise(ctx, master, copy, 0, reverbPool); break;
                case 'pluck': schedulePluck(ctx, master, copy, 0); break;
                case 'sweep': scheduleSweep(ctx, master, copy, 0); break;
                case 'arp': scheduleArp(ctx, master, copy, 0); break;
                default: break;
            }
        });
        return ctx.startRendering().then(buffer => {
            const stats = normalizeBuffer(buffer);
            return { buffer, stats, duration: buffer.duration };
        });
    }

    function layer(kind, props) {
        return Object.assign({ kind }, props || {});
    }

    window.DesktopSoundSynth = {
        EVENTS,
        SAMPLE_RATE,
        MAX_SECONDS,
        midiToHz,
        layer,
        renderLayers,
        normalizeBuffer
    };
})();
