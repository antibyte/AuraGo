/* Second throughput probe: which memory layout makes the Woof port fast enough?
 *
 * The ikj form of the first probe spends three quarters of its time on the
 * load/store of the accumulator. The dot form keeps the sum in a register and
 * streams the weights, but needs the weights stored output-major. Both are
 * benchmarked here on the shapes the model really uses. */

function ikj(A, W, M, K, N) {
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

/* A(M,K) * Wdot(N,K)^T -> (M,N). Wdot is output-major, so the inner loop walks
 * the weights and the input contiguously and the sum stays in a register. */
function dot(A, Wdot, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        for (let n = 0; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += A[ao + k] * Wdot[wo + k];
            out[oo + n] = s;
        }
    }
    return out;
}

/* Same as dot but the sum is kept in a double, which is what numpy's BLAS does
 * for the blocked part of its kernels. */
function dot64(A, Wdot, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        for (let n = 0; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += A[ao + k] * Wdot[wo + k];
            out[oo + n] = s;
        }
    }
    return out;
}

/* Four outputs at a time, so the input row is read once for four dot products. */
function dot4(A, Wdot, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        let n = 0;
        for (; n + 4 <= N; n += 4) {
            const w0 = n * K, w1 = (n + 1) * K, w2 = (n + 2) * K, w3 = (n + 3) * K;
            let s0 = 0, s1 = 0, s2 = 0, s3 = 0;
            for (let k = 0; k < K; k++) {
                const a = A[ao + k];
                s0 += a * Wdot[w0 + k];
                s1 += a * Wdot[w1 + k];
                s2 += a * Wdot[w2 + k];
                s3 += a * Wdot[w3 + k];
            }
            out[oo + n] = s0; out[oo + n + 1] = s1; out[oo + n + 2] = s2; out[oo + n + 3] = s3;
        }
        for (; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += A[ao + k] * Wdot[wo + k];
            out[oo + n] = s;
        }
    }
    return out;
}

function bench(name, fn, M, K, N, seconds, transpose) {
    const A = new Float32Array(M * K).map(() => Math.random() * 0.1);
    const W = new Float32Array(K * N).map(() => Math.random() * 0.1);
    let Wdot = null;
    if (transpose) {
        Wdot = new Float32Array(K * N);
        for (let k = 0; k < K; k++) for (let n = 0; n < N; n++) Wdot[n * K + k] = W[k * N + n];
    }
    fn(A, transpose ? Wdot : W, M, K, N);
    let iters = 0;
    const t0 = performance.now();
    while (performance.now() - t0 < seconds * 1000) { fn(A, transpose ? Wdot : W, M, K, N); iters++; }
    const dt = (performance.now() - t0) / 1000;
    const gmac = iters * M * K * N / dt / 1e9;
    console.log(`${name.padEnd(26)} M=${String(M).padStart(5)} K=${String(K).padStart(4)} N=${String(N).padStart(5)}  ${gmac.toFixed(2)} GMAC/s`);
    return gmac;
}

console.log('--- speech model, single step (M=1) ---');
bench('ikj', ikj, 1, 448, 1344, 1.2, false);
bench('dot (W output-major)', dot, 1, 448, 1344, 1.2, true);
bench('dot4 (W output-major)', dot4, 1, 448, 1344, 1.2, true);

console.log('--- speech model, prefix block (M=40) ---');
bench('ikj', ikj, 40, 448, 1344, 1.2, false);
bench('dot', dot, 40, 448, 1344, 1.2, true);
bench('dot4', dot4, 40, 448, 1344, 1.2, true);

console.log('--- decoder, transposed conv (M=3600, K=192, N=960) ---');
bench('ikj', ikj, 3600, 192, 960, 1.2, false);
bench('dot', dot, 3600, 192, 960, 1.2, true);
bench('dot4', dot4, 3600, 192, 960, 1.2, true);

console.log('--- decoder, conv at 24 kHz (M=18000, K=96, N=384) ---');
bench('ikj', ikj, 18000, 96, 384, 1.2, false);
bench('dot', dot, 18000, 96, 384, 1.2, true);
bench('dot4', dot4, 18000, 96, 384, 1.2, true);
