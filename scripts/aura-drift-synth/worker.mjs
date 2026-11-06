// Worker thread: renders one job id at a time and posts the channel data back (transferred, no copies).
import { parentPort } from 'node:worker_threads';
import { renderJob } from './registry.mjs';

const f32 = list => list.map(ch => Float32Array.from(ch));
parentPort.on('message', ({ id }) => {
    try {
        const out = renderJob(id);
        const payload = Array.isArray(out) ? f32(out) : Object.fromEntries(Object.entries(out).map(([k, v]) => [k, f32(v)]));
        const transfer = (Array.isArray(payload) ? payload : Object.values(payload).flat()).map(a => a.buffer);
        parentPort.postMessage({ id, payload }, transfer);
    } catch (error) {
        parentPort.postMessage({ id, error: String(error && error.stack || error) });
    }
});
