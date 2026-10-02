// Resamples Live Speech output to the server headset rate and posts fixed
// chunks to the main thread. It outputs silence; playback happens elsewhere.
class AuraRealtimeOutputTap extends AudioWorkletProcessor {
    constructor(options) {
        super();
        const settings = (options && options.processorOptions) || {};
        this.targetRate = settings.targetRate || 24000;
        this.ratio = sampleRate / this.targetRate;
        this.chunkSamples = settings.chunkSamples || 480;
        this.pending = [];
        this.position = 0;
        this.chunk = new Float32Array(this.chunkSamples);
        this.offset = 0;
    }

    push(value) {
        this.chunk[this.offset++] = Math.max(-1, Math.min(1, value));
        if (this.offset !== this.chunk.length) return;
        this.port.postMessage(this.chunk, [this.chunk.buffer]);
        this.chunk = new Float32Array(this.chunkSamples);
        this.offset = 0;
    }

    process(inputs) {
        const channels = inputs[0];
        if (!channels || channels.length === 0 || !channels[0]) return true;
        const length = channels[0].length;
        for (let i = 0; i < length; i += 1) {
            let sum = 0;
            for (let c = 0; c < channels.length; c += 1) sum += channels[c][i];
            this.pending.push(sum / channels.length);
        }
        while (this.position + 1 < this.pending.length) {
            const left = Math.floor(this.position);
            const fraction = this.position - left;
            this.push(this.pending[left] * (1 - fraction) + this.pending[left + 1] * fraction);
            this.position += this.ratio;
        }
        const consumed = Math.min(Math.floor(this.position), Math.max(0, this.pending.length - 1));
        if (consumed > 0) {
            this.pending.splice(0, consumed);
            this.position -= consumed;
        }
        return true;
    }
}

registerProcessor('aurago-realtime-output-tap', AuraRealtimeOutputTap);
