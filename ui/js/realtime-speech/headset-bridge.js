(function () {
    'use strict';

    // HeadsetBridge connects one Live Speech session to a Bluetooth headset on
    // the AuraGo server. Microphone frames arrive as PCM16 (16 kHz, 512
    // samples); provider audio leaves as [stream byte] + PCM16 (24 kHz).
    class HeadsetBridge extends EventTarget {
        constructor(options) {
            super();
            this.sessionId = String(options.sessionId || '');
            this.clientId = String(options.clientId || '');
            this.device = String(options.device || '');
            this.WebSocketClass = options.WebSocketClass || window.WebSocket;
            this.retryDelays = options.retryDelays || [2000, 5000, 10000];
            this.socket = null;
            this.deviceReady = false;
            this.closed = false;
            this.retryIndex = 0;
            this.retryTimer = null;
            this.permanentError = false;
        }

        emit(type, detail) {
            this.dispatchEvent(new CustomEvent(type, { detail: detail || {} }));
        }

        url() {
            const scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const params = new URLSearchParams({ session: this.sessionId, client: this.clientId, device: this.device });
            const path = '/api/realtime-speech/headset?' + params.toString();
            const common = window.AuraRealtimeProviderCommon;
            return scheme + '//' + location.host + (common && common.apiURL ? common.apiURL(path) : path);
        }

        open() {
            if (this.closed || this.socket) return;
            const socket = new this.WebSocketClass(this.url());
            socket.binaryType = 'arraybuffer';
            this.socket = socket;
            socket.addEventListener('message', event => {
                if (socket === this.socket) this.handleMessage(event.data);
            });
            socket.addEventListener('close', () => this.handleClose(socket));
        }

        handleMessage(data) {
            if (data instanceof ArrayBuffer) {
                if (this.deviceReady) this.emit('frame', { frame: pcm16ToFloat(data) });
                return;
            }
            let message = null;
            try { message = JSON.parse(String(data)); } catch (_) { return; }
            if (!message || typeof message.type !== 'string') return;
            if (message.type === 'ready') {
                this.retryIndex = 0;
                this.emit('open', {});
            } else if (message.type === 'device_ready') {
                this.deviceReady = true;
                this.emit('device', { ready: true });
            } else if (message.type === 'device_lost') {
                this.setLost('device', '');
            } else if (message.type === 'error') {
                const code = String(message.code || 'headset_audio_unavailable');
                if (code === 'headset_unknown') this.permanentError = true;
                this.setLost('error', code);
            }
        }

        setLost(reason, error) {
            const wasReady = this.deviceReady;
            this.deviceReady = false;
            this.emit('device', { ready: false, wasReady, reason, error: error || '' });
        }

        handleClose(socket) {
            if (socket !== this.socket) return;
            this.socket = null;
            if (this.closed) return;
            this.setLost('bridge', '');
            if (this.permanentError) return;
            const delay = this.retryDelays[Math.min(this.retryIndex, this.retryDelays.length - 1)];
            this.retryIndex += 1;
            this.retryTimer = window.setTimeout(() => this.open(), delay);
        }

        // sendOutput forwards one chunk of 24 kHz float samples; inaudible
        // chunks are skipped to keep the idle stream quiet.
        sendOutput(stream, samples) {
            const socket = this.socket;
            if (!this.deviceReady || !socket || socket.readyState !== 1 || isSilent(samples)) return;
            const payload = new Uint8Array(1 + samples.length * 2);
            payload[0] = stream === 1 ? 1 : 0;
            const view = new DataView(payload.buffer);
            for (let i = 0; i < samples.length; i += 1) {
                const value = Math.max(-1, Math.min(1, samples[i]));
                view.setInt16(1 + i * 2, value < 0 ? value * 0x8000 : value * 0x7fff, true);
            }
            socket.send(payload.buffer);
        }

        flush(stream) {
            const socket = this.socket;
            if (!socket || socket.readyState !== 1) return;
            socket.send(JSON.stringify({ type: 'flush', stream: stream === 1 ? 1 : 0 }));
        }

        close() {
            this.closed = true;
            window.clearTimeout(this.retryTimer);
            const socket = this.socket;
            this.socket = null;
            this.deviceReady = false;
            if (socket) {
                try { socket.close(); } catch (_) { }
            }
        }
    }

    function pcm16ToFloat(buffer) {
        const view = new DataView(buffer);
        const samples = new Float32Array(Math.floor(buffer.byteLength / 2));
        for (let i = 0; i < samples.length; i += 1) samples[i] = view.getInt16(i * 2, true) / 32768;
        return samples;
    }

    function isSilent(samples) {
        for (let i = 0; i < samples.length; i += 1) {
            if (Math.abs(samples[i]) > 1e-4) return false;
        }
        return true;
    }

    // listDevices returns {devices: [{id, name, connected, busy}], reason}.
    async function listDevices() {
        const response = await window.AuraRealtimeProviderCommon.apiFetch('/api/realtime-speech/audio-devices', { credentials: 'same-origin', cache: 'no-store' });
        if (!response.ok) return { devices: [], reason: 'HTTP ' + response.status };
        return response.json();
    }

    window.AuraRealtimeHeadsetBridge = { HeadsetBridge, listDevices, pcm16ToFloat };
})();
