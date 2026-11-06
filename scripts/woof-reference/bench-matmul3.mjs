/* Third probe: how close can plain JS get to the machine's float peak, and which
 * kernel shape wins? The machine matters: the port runs in a browser on the
 * player's laptop, not on the build server. */

function calibrate() {
    const a = new Float32Array(1 << 20).map(() => Math.random());
    const b = new Float32Array(1 << 20).map(() => Math.random());
    let s = 0;
    const t0 = performance.now();
    for (let r = 0; r < 20; r++) for (let i = 0; i < a.length; i++) s += a[i] * b[i];
    const dt = (performance.now() - t0) / 1000;
    console.log(`scalar fma loop: ${(20 * a.length / dt / 1e9).toFixed(2)} GFLOP/s  (sum ${s.toFixed(1)})`);
}

/* Eight outputs per input load, the weights interleaved so each of the eight
 * dot products walks its own contiguous row. */
function dot8(A, W, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        let n = 0;
        for (; n + 8 <= N; n += 8) {
            let s0 = 0, s1 = 0, s2 = 0, s3 = 0, s4 = 0, s5 = 0, s6 = 0, s7 = 0;
            const w0 = (n) * K, w1 = (n + 1) * K, w2 = (n + 2) * K, w3 = (n + 3) * K;
            const w4 = (n + 4) * K, w5 = (n + 5) * K, w6 = (n + 6) * K, w7 = (n + 7) * K;
            for (let k = 0; k < K; k++) {
                const a = A[ao + k];
                s0 += a * W[w0 + k]; s1 += a * W[w1 + k]; s2 += a * W[w2 + k]; s3 += a * W[w3 + k];
                s4 += a * W[w4 + k]; s5 += a * W[w5 + k]; s6 += a * W[w6 + k]; s7 += a * W[w7 + k];
            }
            out[oo + n] = s0; out[oo + n + 1] = s1; out[oo + n + 2] = s2; out[oo + n + 3] = s3;
            out[oo + n + 4] = s4; out[oo + n + 5] = s5; out[oo + n + 6] = s6; out[oo + n + 7] = s7;
        }
        for (; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += A[ao + k] * W[wo + k];
            out[oo + n] = s;
        }
    }
    return out;
}

/* The input row is copied into a plain array first: V8 then keeps it in the
 * fast elements kind instead of a typed array. */
function dot8plain(A, W, M, K, N) {
    const out = new Float32Array(M * N);
    const row = new Array(K);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        for (let k = 0; k < K; k++) row[k] = A[ao + k];
        let n = 0;
        for (; n + 8 <= N; n += 8) {
            let s0 = 0, s1 = 0, s2 = 0, s3 = 0, s4 = 0, s5 = 0, s6 = 0, s7 = 0;
            const w0 = n * K, w1 = (n + 1) * K, w2 = (n + 2) * K, w3 = (n + 3) * K;
            const w4 = (n + 4) * K, w5 = (n + 5) * K, w6 = (n + 6) * K, w7 = (n + 7) * K;
            for (let k = 0; k < K; k++) {
                const a = row[k];
                s0 += a * W[w0 + k]; s1 += a * W[w1 + k]; s2 += a * W[w2 + k]; s3 += a * W[w3 + k];
                s4 += a * W[w4 + k]; s5 += a * W[w5 + k]; s6 += a * W[w6 + k]; s7 += a * W[w7 + k];
            }
            out[oo + n] = s0; out[oo + n + 1] = s1; out[oo + n + 2] = s2; out[oo + n + 3] = s3;
            out[oo + n + 4] = s4; out[oo + n + 5] = s5; out[oo + n + 6] = s6; out[oo + n + 7] = s7;
        }
        for (; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += row[k] * W[wo + k];
            out[oo + n] = s;
        }
    }
    return out;
}

/* Same shape but with double weights: V8 loads doubles without the float32
 * rounding step on the way in. */
function dot8f64(A, W, M, K, N) {
    const out = new Float32Array(M * N);
    for (let m = 0; m < M; m++) {
        const ao = m * K, oo = m * N;
        let n = 0;
        for (; n + 8 <= N; n += 8) {
            let s0 = 0, s1 = 0, s2 = 0, s3 = 0, s4 = 0, s5 = 0, s6 = 0, s7 = 0;
            const w0 = n * K, w1 = (n + 1) * K, w2 = (n + 2) * K, w3 = (n + 3) * K;
            const w4 = (n + 4) * K, w5 = (n + 5) * K, w6 = (n + 6) * K, w7 = (n + 7) * K;
            for (let k = 0; k < K; k++) {
                const a = A[ao + k];
                s0 += a * W[w0 + k]; s1 += a * W[w1 + k]; s2 += a * W[w2 + k]; s3 += a * W[w3 + k];
                s4 += a * W[w4 + k]; s5 += a * W[w5 + k]; s6 += a * W[w6 + k]; s7 += a * W[w7 + k];
            }
            out[oo + n] = s0; out[oo + n + 1] = s1; out[oo + n + 2] = s2; out[oo + n + 3] = s3;
            out[oo + n + 4] = s4; out[oo + n + 5] = s5; out[oo + n + 6] = s6; out[oo + n + 7] = s7;
        }
        for (; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += A[ao + k] * W[wo + k];
            out[oo + n] = s;
        }
    }
    return out;
}

function bench(name, fn, M, K, N, seconds, f64) {
    const A = new Float32Array(M * K).map(() => Math.random() * 0.1);
    const W = (f64 ? new Float64Array(K * N) : new Float32Array(K * N)).map(() => Math.random() * 0.1);
    fn(A, W, M, K, N);
    let iters = 0;
    const t0 = performance.now();
    while (performance.now() - t0 < seconds * 1000) { fn(A, W, M, K, N); iters++; }
    const dt = (performance.now() - t0) / 1000;
    console.log(`${name.padEnd(18)} M=${String(M).padStart(5)} K=${String(K).padStart(4)} N=${String(N).padStart(5)}  ${(iters * M * K * N / dt / 1e9).toFixed(2)} GMAC/s`);
}

calibrate();
console.log('');
bench('dot8', dot8, 1, 448, 1344, 1.2, false);
bench('dot8 plain row', dot8plain, 1, 448, 1344, 1.2, false);
bench('dot8 f64 weights', dot8f64, 1, 448, 1344, 1.2, true);
console.log('');
bench('dot8', dot8, 3600, 192, 960, 1.2, false);
bench('dot8 plain row', dot8plain, 3600, 192, 960, 1.2, false);
bench('dot8 f64 weights', dot8f64, 3600, 192, 960, 1.2, true);
console.log('');
bench('dot8', dot8, 18000, 96, 384, 1.2, false);
bench('dot8 plain row', dot8plain, 18000, 96, 384, 1.2, false);
