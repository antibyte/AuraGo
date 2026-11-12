// T0: the fast EasyDrag check. One command, no browser.
//
//   node scripts/test-easydrag-t0.mjs              Node scripts and Go steps
//   node scripts/test-easydrag-t0.mjs --node-only  Node scripts only (the CI Node job)
//
// Every scripts/test-easydrag*.mjs runs in a process of its own, at most one per CPU. The Go steps in
// GO_STEPS start together with them, one process each, because the slowest Go package (internal/flows,
// about 35 s) would otherwise be added to the Node time. The first failure stops everything that is
// still running and prints its output. The script sets no Go temporary directory: the caller's
// environment, GOTMPDIR included, reaches every child unchanged.
//
// Several scripts (extra*.mjs, flagship, pack, ux-batch5, render-baseline, ...) only export a run()
// that scripts/test-easydrag.mjs calls with its harness. Alone they check that the module imports and
// parses, and finish in well under a second.
import { spawn, spawnSync } from 'node:child_process';
import { readdirSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SELF = path.basename(fileURLToPath(import.meta.url));

// Part C appends './cmd/flowsdoc' here. Each entry: { label, args, env } (env is merged over the caller's).
export const GO_STEPS = [
    { label: 'go internal/flows', args: ['test', './internal/flows/...'], env: {} },
    { label: 'go internal/server (flow)', args: ['test', './internal/server', '-run', '(?i)flow'], env: {} },
    { label: 'go internal/tools (flow)', args: ['test', './internal/tools', '-run', '(?i)flow'], env: {} },
    { label: 'go ui (easydrag)', args: ['test', './ui', '-run', '(?i)easydrag'], env: {} },
];

const TAIL_LINES = 60;

function nodeJobs() {
    return readdirSync(path.join(ROOT, 'scripts'))
        .filter(name => /^test-easydrag.*\.mjs$/.test(name) && name !== SELF)
        .sort()
        .map(name => ({ label: 'node scripts/' + name, cmd: process.execPath, args: ['scripts/' + name], env: {} }));
}

function goJobs() {
    return GO_STEPS.map(step => ({ label: step.label, cmd: 'go', args: step.args, env: step.env || {} }));
}

function kill(child) {
    if (child.exitCode !== null || child.pid === undefined) return;
    if (process.platform === 'win32') {
        spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
        try { process.kill(-child.pid, 'SIGKILL'); } catch { child.kill('SIGKILL'); }
    }
}

function seconds(ms) { return (ms / 1000).toFixed(1) + ' s'; }

export async function main(argv) {
    const nodeOnly = argv.includes('--node-only');
    const started = Date.now();
    const queue = nodeJobs();
    const goQueue = nodeOnly ? [] : goJobs();
    const total = queue.length + goQueue.length;
    const running = new Set();
    let failed = null;
    let done = 0;

    const launch = job => new Promise(resolve => {
        const t0 = Date.now();
        const child = spawn(job.cmd, job.args, {
            cwd: ROOT,
            env: { ...process.env, ...job.env },
            detached: process.platform !== 'win32',
            windowsHide: true,
        });
        running.add(child);
        const chunks = [];
        child.stdout.on('data', d => chunks.push(d));
        child.stderr.on('data', d => chunks.push(d));
        child.on('error', err => { chunks.push(Buffer.from(String(err))); finish(-1); });
        child.on('close', code => finish(code));
        let over = false;
        function finish(code) {
            if (over) return;
            over = true;
            running.delete(child);
            if (failed) return resolve();
            done++;
            if (code === 0) {
                console.log('ok   ' + job.label + '  (' + seconds(Date.now() - t0) + ', ' + done + '/' + total + ')');
            } else {
                failed = { job, code, output: Buffer.concat(chunks).toString('utf8') };
                for (const other of running) kill(other);
            }
            resolve();
        }
    });

    // Go steps start at once; the Node scripts fill the remaining CPUs.
    const goRuns = goQueue.map(launch);
    const width = Math.max(1, os.availableParallelism() - goQueue.length);
    const worker = async () => {
        while (queue.length && !failed) await launch(queue.shift());
    };
    await Promise.all([...goRuns, ...Array.from({ length: width }, worker)]);

    const elapsed = seconds(Date.now() - started);
    if (failed) {
        const lines = failed.output.trimEnd().split('\n');
        console.log('\nFAIL ' + failed.job.label + ' (exit ' + failed.code + ')');
        console.log(lines.slice(-TAIL_LINES).join('\n'));
        console.log('\nT0 FAILED after ' + elapsed + ': ' + failed.job.label + ' (' + done + ' of ' + total + ' passed first)');
        return 1;
    }
    console.log('\nT0 passed: ' + total + ' steps' + (nodeOnly ? ' (Node only)' : '') + ' in ' + elapsed);
    return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
    process.exit(await main(process.argv.slice(2)));
}
