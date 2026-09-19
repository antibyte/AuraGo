(function () {
    'use strict';
    // Only small PCM windows live in memory. All source nodes use the same
    // audio clock; UI timers never decide a track's exact start time.
    class PersonalRadioPlayer {
        constructor(deps) {
            this.deps = deps; this.context = null; this.master = null;
            this.slots = []; this.queue = []; this.generation = 0;
            this.loading = false; this.running = false; this.paused = false; this.completed = new Set();
            this.volume = 0.8; this.timer = null; this.abort = new AbortController();
            this.primed = false;
        }
        async unlock() {
            if (!this.context || this.context.state === 'closed') {
                this.context = new (window.AudioContext || window.webkitAudioContext)();
                this.master = this.context.createGain();
                const limiter = this.context.createDynamicsCompressor();
                limiter.threshold.value = -2; limiter.ratio.value = 16;
                this.master.connect(limiter); limiter.connect(this.context.destination);
                this.setVolume(this.volume);
            }
            await this.context.resume();
            if (this.context.state !== 'running') throw Error('radio_audio_unlock');
        }
        setVolume(value) {
            this.volume = Math.max(0, Math.min(1, Number(value) || 0));
            if (this.master) this.master.gain.setTargetAtTime(this.volume * this.volume, this.context.currentTime, 0.03);
        }
        update(queue) {
            this.queue = queue || [];
            const started = new Set(this.slots.filter(s => s.started && !s.ended).map(s => s.segment.id));
            const future = this.slots.filter(s => !s.started).map(s => s.segment.id);
            const expected = this.queue.filter(s => !started.has(s.id) && !this.completed.has(s.id)).slice(0, future.length).map(s => s.id);
            if (future.some((id, index) => id !== expected[index])) {
                this.slots = this.slots.filter(slot => { if (slot.started) return true; this.release(slot); return false; });
                const current = this.slots[this.slots.length - 1];
                if (current && this.context) { current.gain.gain.cancelScheduledValues(this.context.currentTime); current.gain.gain.setValueAtTime(1, this.context.currentTime); }
            }
            // A removed reserved track must never survive a skip/block/revision.
            const ids = new Set(this.queue.map(x => x.id));
            this.slots = this.slots.filter(slot => {
                if (ids.has(slot.segment.id) || (slot.started && !slot.ended)) return true;
                this.release(slot); return false;
            });
        }
        async play() {
            await this.unlock(); this.paused = false; this.running = true;
            if (!this.timer) this.timer = setInterval(() => this.pump(), 300);
            await this.pump();
        }
        async pause() { this.paused = true; if (this.context) await this.context.suspend(); }
        release(slot) {
            slot.cancelled = true;
            for (const source of slot.nodes) { try { source.stop(); source.disconnect(); } catch (_) {} }
            slot.nodes.clear(); slot.buffers.clear(); slot.gain.disconnect();
        }
        reset(keepPrimed) {
            if (!keepPrimed) this.primed = false;
            this.generation++; this.running = false; this.paused = false;
            this.abort.abort(); this.abort = new AbortController();
            clearInterval(this.timer); this.timer = null;
            this.slots.forEach(s => this.release(s)); this.slots = []; this.queue = []; this.completed.clear();
        }
        stop() { this.reset(); if (this.context) { this.context.close().catch(() => {}); this.context = null; } }
        position() {
            const now = this.context ? this.context.currentTime : 0;
            const slot = this.slots.find(s => s.start != null && now >= s.start && now < s.start + s.duration);
            return slot ? { current: slot.segment.id, position: Math.max(0, Math.round((now - slot.start) * 1000)), duration: slot.segment.duration_ms } : { current: '', position: 0, duration: 0 };
        }
        pcm(data) {
            const view = new DataView(data);
            if (data.byteLength < 44 || view.getUint16(20, true) !== 1 || view.getUint16(34, true) !== 16) throw Error('radio_invalid_audio');
            const channels = view.getUint16(22, true), rate = view.getUint32(24, true), bytes = view.getUint32(40, true);
            if (channels < 1 || channels > 2 || rate < 8000 || rate > 96000 || bytes > data.byteLength - 44) throw Error('radio_invalid_audio');
            const frames = bytes / channels / 2;
            const buffer = this.context.createBuffer(channels, frames, rate);
            for (let channel = 0; channel < channels; channel++) {
                const values = buffer.getChannelData(channel);
                for (let i = 0; i < frames; i++) values[i] = view.getInt16(44 + (i * channels + channel) * 2, true) / 32768;
            }
            return buffer;
        }
        async fetchChunk(slot) {
            if (slot.offset >= slot.segment.duration_ms || slot.cancelled) return;
            const offset = slot.offset, generation = this.generation;
            const data = await this.deps.audio(slot.segment.asset_id, offset, 30000, this.abort.signal);
            if (generation !== this.generation || slot.cancelled || !this.context) return;
            const buffer = this.pcm(data);
            slot.buffers.set(offset, buffer); slot.offset += Math.round(buffer.duration * 1000);
            if (slot.offset <= offset) throw Error('radio_invalid_audio');
        }
        schedule(slot) {
            if (slot.start == null) return;
            for (const [offset, buffer] of slot.buffers) {
                const when = slot.start + offset / 1000;
                if (when + buffer.duration <= this.context.currentTime) { slot.buffers.delete(offset); continue; }
                if (when < this.context.currentTime - 0.05) throw Error('radio_buffering');
                const source = this.context.createBufferSource(); source.buffer = buffer; source.connect(slot.gain);
                slot.nodes.add(source); source.onended = () => { slot.nodes.delete(source); source.disconnect(); source.buffer = null; };
                source.start(when); slot.scheduledTo = offset / 1000 + buffer.duration;
                slot.buffers.delete(offset);
            }
        }
        async pump() {
            if (!this.running || this.paused || this.loading || !this.context || this.context.state !== 'running') return;
            this.loading = true;
            const generation = this.generation;
            let work = null;
            try {
                const now = this.context.currentTime;
                for (const slot of this.slots) {
                    if (slot.start != null && now >= slot.start && !slot.started) {
                        slot.started = true; this.deps.event(slot.segment.id, 'started');
                    }
                    if (slot.start != null && now >= slot.start + slot.duration && !slot.ended) {
                        slot.ended = true; this.completed.add(slot.segment.id); this.deps.event(slot.segment.id, 'ended');
                        if (this.completed.size > 128) this.completed.delete(this.completed.values().next().value);
                    }
                }
                this.slots = this.slots.filter(s => { if (!s.ended) return true; this.release(s); return false; });
                const activeIDs = new Set(this.slots.map(s => s.segment.id));
                for (const segment of this.queue) {
                    // A short spoken segment also reserves the following
                    // music, so its end never depends on a UI timer firing.
                    const limit = this.slots.some(s => s.segment.kind !== 'music') ? 3 : 2;
                    if (this.slots.length >= limit) break;
                    if (activeIDs.has(segment.id) || this.completed.has(segment.id)) continue;
                    const expires = Date.parse(segment.expires);
                    if (expires > 0 && expires <= Date.now()) { this.completed.add(segment.id); this.deps.event(segment.id, 'failed'); continue; }
                    const gain = this.context.createGain(); gain.connect(this.master);
                    this.slots.push({ segment, gain, duration: segment.duration_ms / 1000, start: null, offset: 0, scheduledTo: 0, buffers: new Map(), nodes: new Set() });
                    activeIDs.add(segment.id);
                }
                for (const slot of this.slots) {
                    work = slot;
                    const elapsed = slot.start == null ? 0 : Math.max(0, this.context.currentTime - slot.start);
                    // Ninety seconds of scheduled lookahead tolerates throttled
                    // background timers without retaining a complete long track.
                    while (slot.offset < slot.segment.duration_ms && slot.offset / 1000 < elapsed + 90) {
                        await this.fetchChunk(slot);
                        if (generation !== this.generation || slot.cancelled) return;
                    }
                }
                if (!this.slots.length) return;
                const first = this.slots[0], next = this.slots[1];
                if (first.start == null) {
                    if (!first.buffers.size || (!this.primed && (!next || !next.buffers.size))) return;
                    first.start = this.context.currentTime + 0.15;
                    this.primed = true;
                }
                work = first; this.schedule(first);
                for (let index = 1; index < this.slots.length; index++) {
                    const previous = this.slots[index-1], upcoming = this.slots[index];
                    if (upcoming.start == null && upcoming.buffers.size) {
                        const overlap = previous.segment.kind === 'music' && upcoming.segment.kind === 'music' ? Math.min(2, previous.duration / 8, upcoming.duration / 8) : 0;
                        upcoming.start = previous.start + previous.duration - overlap;
                        previous.gain.gain.setValueAtTime(1, Math.max(this.context.currentTime, upcoming.start));
                        previous.gain.gain.linearRampToValueAtTime(0, previous.start + previous.duration);
                        upcoming.gain.gain.setValueAtTime(overlap ? 0 : 1, upcoming.start);
                        if (overlap) upcoming.gain.gain.linearRampToValueAtTime(1, upcoming.start + overlap);
                    }
                    work = upcoming; this.schedule(upcoming);
                }
                this.deps.progress(this.position());
            } catch (err) {
                if (generation !== this.generation || err.name === 'AbortError') return;
                this.deps.error(err);
                if (work && !work.cancelled) {
                    this.completed.add(work.segment.id); this.release(work);
                    this.slots = this.slots.filter(slot => slot !== work);
                    // Rebuild the unstarted transition after a failed fetch;
                    // its old start time may depend on the removed segment.
                    this.slots = this.slots.filter(slot => { if (slot.started) return true; this.release(slot); return false; });
                    const current = this.slots[this.slots.length - 1];
                    if (current) { current.gain.gain.cancelScheduledValues(this.context.currentTime); current.gain.gain.setValueAtTime(1, this.context.currentTime); }
                    this.deps.event(work.segment.id, 'failed');
                }
            } finally { this.loading = false; }
        }
    }
    window.PersonalRadioPlayer = PersonalRadioPlayer;
})();
