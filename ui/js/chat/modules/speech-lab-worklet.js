class AuraGoSpeechLabProcessor extends AudioWorkletProcessor {
    process(inputs) {
        const channels = inputs[0];
        if (!channels || !channels.length || !channels[0].length) return true;
        const frames = channels[0].length;
        const mono = new Float32Array(frames);
        for (let channelIndex = 0; channelIndex < channels.length; channelIndex++) {
            const channel = channels[channelIndex];
            for (let i = 0; i < frames; i++) mono[i] += channel[i] / channels.length;
        }
        this.port.postMessage(mono, [mono.buffer]);
        return true;
    }
}

registerProcessor('aurago-speech-lab-recorder', AuraGoSpeechLabProcessor);
