(function () {
    'use strict';

    const DEFAULT_WORKER_URL = '/js/vendor/stockfish/stockfish-18-lite-single.js';

    function createChessEngine(options = {}) {
        const workerUrl = options.workerUrl || DEFAULT_WORKER_URL;
        let worker = null;
        let ready = false;
        let readyWaiters = [];
        let activeSearch = null;
        let disposed = false;

        function postMessage(message) {
            if (worker && !disposed) worker.postMessage(message);
        }

        function resolveReady() {
            ready = true;
            const waiters = readyWaiters.slice();
            readyWaiters = [];
            waiters.forEach(resolve => resolve());
        }

        function rejectSearch(error) {
            if (!activeSearch) return;
            const search = activeSearch;
            activeSearch = null;
            window.clearTimeout(search.timeoutId);
            search.reject(error);
        }

        function handleMessage(event) {
            const line = String(event && event.data != null ? event.data : '').trim();
            if (!line) return;
            if (line === 'uciok') {
                postMessage('isready');
                return;
            }
            if (line === 'readyok') {
                resolveReady();
                return;
            }
            if (!activeSearch || !line.startsWith('bestmove ')) return;
            const move = line.split(/\s+/)[1] || '';
            const search = activeSearch;
            activeSearch = null;
            window.clearTimeout(search.timeoutId);
            if (!move || move === '(none)') {
                search.reject(new Error('Stockfish did not return a move.'));
                return;
            }
            search.resolve(move);
        }

        function ensureWorker() {
            if (disposed) return Promise.reject(new Error('Chess engine disposed.'));
            if (!worker) {
                worker = new Worker(workerUrl);
                worker.addEventListener('message', handleMessage);
                worker.addEventListener('error', event => {
                    rejectSearch(new Error((event && event.message) || 'Stockfish worker failed.'));
                });
                postMessage('uci');
            }
            if (ready) return Promise.resolve();
            return new Promise(resolve => readyWaiters.push(resolve));
        }

        function strengthDepth(strength) {
            const value = Math.max(1, Math.min(20, Number(strength) || 8));
            return Math.max(1, Math.min(14, Math.round(2 + value * 0.55)));
        }

        async function findMove({ fen, legalMoves, strength = 8, timeoutMs = 15000 } = {}) {
            if (!fen) throw new Error('FEN is required.');
            await ensureWorker();
            if (activeSearch) rejectSearch(new Error('Superseded by a new search.'));
            const legal = new Set((legalMoves || []).map(move => String(move || '').toLowerCase()));
            return new Promise((resolve, reject) => {
                const timeoutId = window.setTimeout(() => {
                    rejectSearch(new Error('Stockfish timed out.'));
                }, timeoutMs);
                activeSearch = {
                    timeoutId,
                    resolve(move) {
                        const normalized = String(move || '').toLowerCase();
                        if (legal.size && !legal.has(normalized)) {
                            reject(new Error('Stockfish returned an illegal move.'));
                            return;
                        }
                        resolve(normalized);
                    },
                    reject
                };
                postMessage('setoption name Skill Level value ' + Math.max(1, Math.min(20, Number(strength) || 8)));
                postMessage('position fen ' + fen);
                postMessage('go depth ' + strengthDepth(strength));
            });
        }

        function newGame() {
            ensureWorker().then(() => postMessage('ucinewgame')).catch(() => {});
        }

        function dispose() {
            disposed = true;
            rejectSearch(new Error('Chess engine disposed.'));
            readyWaiters = [];
            if (worker) {
                try { postMessage('quit'); } catch (e) {}
                worker.terminate();
                worker = null;
            }
        }

        return { findMove, newGame, dispose };
    }

    window.createChessEngine = createChessEngine;
})();
