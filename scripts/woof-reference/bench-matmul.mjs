/* Throughput probe for the Woof port: how many MAC/s does a plain Float32Array matmul
 * reach in this V8? The decoder needs about 3.5 GMAC per three seconds of audio, so
 * this number decides whether the browser port is viable at all. */
const K = 448, N = 1344;

function matmul(A, W, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        for (let k = 0; k < K; k++) {
            const a = A[ao + k];
            if (a === 0) continue;
            const bo = k * N;
            for (let n = 0; n < N; n++) out[oo + n] += a * W[bo + n];
        }
    }
    return out;
}

function matmulUnrolled(A, W, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        for (let k = 0; k < K; k++) {
            const a = A[ao + k];
            if (a === 0) continue;
            const bo = k * N;
            let n = 0;
            for (; n + 4 <= N; n += 4) {
                out[oo + n] += a * W[bo + n];
                out[oo + n + 1] += a * W[bo + n + 1];
                out[oo + n + 2] += a * W[bo + n + 2];
                out[oo + n + 3] += a * W[bo + n + 3];
            }
            for (; n < N; n++) out[oo + n] += a * W[bo + n];
        }
    }
    return out;
}

function bench(name, fn, M, K2, N2, seconds) {
    const A = new Float32Array(M * K2).map(() => Math.random() * 0.1);
    const W = new Float32Array(K2 * N2).map(() => Math.random() * 0.1);
    fn(A, W, M, K2, N2);                     /* warm up the JIT */
    let iters = 0;
    const t0 = performance.now();
    while (performance.now() - t0 < seconds * 1000) {
        fn(A, W, M, K2, N2);
        iters++;
    }
    const dt = (performance.now() - t0) / 1000;
    const macs = iters * M * K2 * N2;
    console.log(`${name.padEnd(22)} M=${String(M).padStart(5)} K=${String(K2).padStart(4)} N=${String(N2).padStart(5)}` +
        `  ${(macs / dt / 1e9).toFixed(2)} GMAC/s`);
    return macs / dt;
}

/* The shapes the real model needs. */
const speech = bench('speech step (ikj)', matmul, 1, 448, 1344, 1.5);
bench('speech step (unroll4)', matmulUnrolled, 1, 448, 1344, 1.5);
bench('prefix block (M=40)', matmul, 40, 448, 1344, 1.5);
bench('decoder convt (M=600)', matmul, 600, 384, 6144, 1.5);
bench('decoder conv (M=18000)', matmul, 18000, 192, 960, 1.5);

/* What the whole pipeline needs: 3.5 GMAC of decoder work plus 1.1 GMAC of speech work
 * for a three second line. */
const rate = speech / 1e9;
console.log(`\n3.5 GMAC decoder + 1.1 GMAC speech at ${rate.toFixed(2)} GMAC/s -> ${(4.6 / rate).toFixed(2)} s per 3 s line`);
