class AuraRealtimeSpeechProcessor extends AudioWorkletProcessor {
    constructor() {
        super();
        this.targetRate = 16000;
        this.sourceRate = sampleRate;
        this.ratio = this.sourceRate / this.targetRate;
        this.pending = [];
        this.position = 0;
        this.chunk = new Float32Array(512);
        this.chunkOffset = 0;
    }

    pushSample(value) {
        this.chunk[this.chunkOffset++] = Math.max(-1, Math.min(1, value));
        if (this.chunkOffset !== this.chunk.length) return;
        this.port.postMessage(this.chunk, [this.chunk.buffer]);
        this.chunk = new Float32Array(512);
        this.chunkOffset = 0;
    }

    process(inputs) {
        const input = inputs[0] && inputs[0][0];
        if (!input || input.length === 0) return true;

        for (let i = 0; i < input.length; i += 1) this.pending.push(input[i]);
        while (this.position + 1 < this.pending.length) {
            const left = Math.floor(this.position);
            const fraction = this.position - left;
            const value = this.pending[left] * (1 - fraction) + this.pending[left + 1] * fraction;
            this.pushSample(value);
            this.position += this.ratio;
        }

        // Keep the final source sample as the left interpolation boundary for
        // the next AudioWorklet quantum. Dropping it causes long-running
        // 48 kHz → 16 kHz streams to drift by roughly one sample per quantum.
        const consumed = Math.min(Math.floor(this.position), Math.max(0, this.pending.length - 1));
        if (consumed > 0) {
            this.pending.splice(0, consumed);
            this.position -= consumed;
        }
        return true;
    }
}

registerProcessor('aurago-realtime-speech', AuraRealtimeSpeechProcessor);
