(function () {
    'use strict';

    const MAX_ACTIONS = 20;
    const FILE_UNDO_ENDPOINT = '/api/desktop/file-undo';
    const state = {
        api: null,
        refresh: null,
        notify: null,
        isReadonly: null,
        isSessionActive: null,
        t: null,
        undoStack: [],
        redoStack: [],
        pendingRecords: [],
        listeners: new Set(),
        busy: false,
        epoch: 0,
        revision: 0,
        nextActionId: 1
    };

    function text(key, fallback, vars) {
        if (typeof state.t === 'function') {
            try {
                return state.t(key, fallback, vars) || fallback;
            } catch (_) {
                return fallback;
            }
        }
        return fallback;
    }

    function readonly() {
        try {
            return typeof state.isReadonly === 'function' && !!state.isReadonly();
        } catch (_) {
            return true;
        }
    }

    function sessionActive() {
        try {
            return typeof state.isSessionActive !== 'function' || !!state.isSessionActive();
        } catch (_) {
            return false;
        }
    }

    function snapshot() {
        const eligible = sessionActive();
        const undo = state.undoStack[state.undoStack.length - 1] || null;
        const redo = state.redoStack[state.redoStack.length - 1] || null;
        const canUndo = eligible && !state.busy && !readonly() && !!undo && undo.undoMoves.length > 0;
        const canRedo = eligible && !state.busy && !readonly() && !!redo && redo.redoMoves.length > 0;
        return {
            canUndo,
            canRedo,
            undoLabel: canUndo ? undo.label : '',
            redoLabel: canRedo ? redo.label : '',
            undoCount: eligible ? state.undoStack.length : 0,
            redoCount: eligible ? state.redoStack.length : 0,
            busy: state.busy,
            revision: state.revision,
            undoId: canUndo ? undo.id : ''
        };
    }

    function publish() {
        state.revision++;
        const current = snapshot();
        for (const listener of [...state.listeners]) {
            try { listener(current); } catch (err) { console.warn('File history subscriber failed', err); }
        }
        return current;
    }

    function collectMoves(value) {
        if (Array.isArray(value)) return value;
        if (!value || typeof value !== 'object') return [];
        if (Array.isArray(value.moves)) return value.moves;
        return value.undo_receipt ? [value] : [];
    }

    function allKnownReceiptTokens() {
        const tokens = new Set();
        for (const action of new Set([...state.undoStack, ...state.redoStack])) {
            for (const move of action.undoMoves) if (move.undo_receipt) tokens.add(move.undo_receipt);
            for (const move of action.redoMoves) if (move.undo_receipt) tokens.add(move.undo_receipt);
        }
        for (const pending of state.pendingRecords) {
            for (const move of pending.moves) if (move.undo_receipt) tokens.add(move.undo_receipt);
        }
        return tokens;
    }

    function clearRedo() {
        for (const action of state.redoStack) action.redoMoves = [];
        state.redoStack = [];
    }

    function trimActions() {
        const actions = [...new Set([...state.undoStack, ...state.redoStack])]
            .sort((a, b) => a.createdAt - b.createdAt);
        while (actions.length > MAX_ACTIONS) {
            const oldest = actions.shift();
            state.undoStack = state.undoStack.filter(action => action !== oldest);
            state.redoStack = state.redoStack.filter(action => action !== oldest);
            oldest.undoMoves = [];
            oldest.redoMoves = [];
        }
    }

    function newActionId() {
        const cryptoApi = typeof crypto !== 'undefined' ? crypto : null;
        if (cryptoApi && typeof cryptoApi.randomUUID === 'function') return cryptoApi.randomUUID();
        return 'file-action-' + Date.now().toString(36) + '-' + (state.nextActionId++).toString(36);
    }

    function normalizeRecord(resultOrMoves, label) {
        const known = allKnownReceiptTokens();
        const moves = [];
        for (const raw of collectMoves(resultOrMoves)) {
            if (!raw || typeof raw.undo_receipt !== 'string' || !raw.undo_receipt || known.has(raw.undo_receipt)) continue;
            known.add(raw.undo_receipt);
            moves.push({
                undo_receipt: raw.undo_receipt,
                from: typeof raw.from === 'string' ? raw.from : '',
                path: typeof raw.path === 'string' ? raw.path : ''
            });
        }
        if (!moves.length) return null;
        return {
            moves,
            label: typeof label === 'string' && label ? label : text('desktop.file_history_move', 'Move')
        };
    }

    function applyRecord(pending) {
        clearRedo();
        state.undoStack.push({
            id: newActionId(),
            label: pending.label,
            createdAt: Date.now(),
            undoMoves: pending.moves,
            redoMoves: []
        });
        trimActions();
        publish();
    }

    function record(resultOrMoves, label) {
        if (!sessionActive()) return false;
        const pending = normalizeRecord(resultOrMoves, label);
        if (!pending) return false;
        if (state.busy) {
            state.pendingRecords.push(pending);
            return true;
        }
        applyRecord(pending);
        return true;
    }

    function flushPendingRecords() {
        const pending = state.pendingRecords.splice(0);
        if (!sessionActive()) return;
        for (const record of pending) applyRecord(record);
    }

    function containsAction(stack, action) {
        return stack.includes(action);
    }

    function syncActionStacks(action) {
        if (action.undoMoves.length) {
            if (!containsAction(state.undoStack, action)) state.undoStack.push(action);
        } else {
            state.undoStack = state.undoStack.filter(item => item !== action);
        }
        if (action.redoMoves.length) {
            if (!containsAction(state.redoStack, action)) state.redoStack.push(action);
        } else {
            state.redoStack = state.redoStack.filter(item => item !== action);
        }
    }

    function errorDetails(err) {
        const body = err && err.body && typeof err.body === 'object' ? err.body : {};
        const code = String(body.code || body.error || err?.code || '').toLowerCase();
        const status = Number(err && err.status) || 0;
        const message = String((err && err.message) || body.error || 'File operation failed');
        const unavailable = code.includes('undo_unavailable') || code.includes('receipt_unavailable') || code.includes('expired');
        const readOnly = code.includes('readonly') || code.includes('read_only') || code.includes('read-only') || status === 423;
        const conflict = code.includes('conflict') || status === 409 || status === 412;
        const retryable = !unavailable && (conflict || readOnly);
        const conflictObject = body.conflict && typeof body.conflict === 'object' ? body.conflict : {};
        const conflictPath = conflictObject.path || conflictObject.Path || body.path || '';
        return { code, status, message, retryable, unavailable, readOnly, conflict, conflictPath };
    }

    function reportFailure(direction, err) {
        if (typeof state.notify !== 'function') return;
        const details = errorDetails(err);
        let message;
        if (details.unavailable) {
            message = text('desktop.action_unavailable', 'This action is no longer available.');
        } else if (details.readOnly) {
            message = text('desktop.file_dialog_readonly', 'The desktop is read-only.');
        } else if (details.conflict) {
            message = text('desktop.file_conflict_title', 'The destination changed or is occupied.') +
                (details.conflictPath ? ' ' + details.conflictPath : '');
        } else if (!details.retryable) {
            message = text('desktop.file_history_unknown', 'The result is unknown. AuraGo will not retry this action automatically.');
        } else {
            const key = direction === 'undo' ? 'desktop.fm.undo_error' : 'desktop.fm.redo_error';
            message = text(key, details.message, { error: details.message });
        }
        try {
            state.notify({
                title: text('desktop.notification', 'Notification'),
                message
            });
        } catch (notifyErr) {
            console.warn('File history notification failed', notifyErr);
        }
    }

    async function postUndoReceipt(receipt) {
        if (typeof state.api !== 'function') throw new Error('desktop_api_unavailable');
        return state.api(FILE_UNDO_ENDPOINT, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ receipt })
        });
    }

    async function run(direction) {
        if (state.busy) return { status: 'busy' };
        if (!sessionActive()) return { status: 'inactive' };
        if (readonly()) return { status: 'readonly' };
        const sourceStack = direction === 'undo' ? state.undoStack : state.redoStack;
        const targetMovesKey = direction === 'undo' ? 'undoMoves' : 'redoMoves';
        const oppositeMovesKey = direction === 'undo' ? 'redoMoves' : 'undoMoves';
        const action = sourceStack[sourceStack.length - 1];
        if (!action || !action[targetMovesKey].length) return { status: 'empty' };

        const epoch = state.epoch;
        state.busy = true;
        publish();
        let changed = false;
        let completed = 0;
        let stopped = false;
        let failedError = null;
        try {
            const moves = action[targetMovesKey];
            for (let index = moves.length - 1; index >= 0; index--) {
                if (epoch !== state.epoch) return { status: 'cleared' };
                if (!sessionActive()) return { status: 'inactive' };
                if (state.pendingRecords.length) {
                    stopped = true;
                    break;
                }
                const current = moves[index];
                try {
                    const result = await postUndoReceipt(current.undo_receipt);
                    if (epoch !== state.epoch) return { status: 'cleared' };
                    if (!sessionActive()) return { status: 'inactive' };
                    if (!result || typeof result.undo_receipt !== 'string' || !result.undo_receipt) {
                        moves.splice(index, 1);
                        changed = true;
                        stopped = true;
                        failedError = new Error('Undo response did not include a redo receipt');
                        failedError.status = 0;
                        break;
                    }
                    moves.splice(index, 1);
                    action[oppositeMovesKey].push({
                        undo_receipt: result.undo_receipt,
                        from: typeof result.from === 'string' ? result.from : current.path,
                        path: typeof result.path === 'string' ? result.path : current.from
                    });
                    changed = true;
                    completed++;
                } catch (err) {
                    if (epoch !== state.epoch) return { status: 'cleared' };
                    if (!sessionActive()) return { status: 'inactive' };
                    const details = errorDetails(err);
                    if (!details.retryable) {
                        // Semantic invalidation and uncertain transport outcomes must not be replayed.
                        moves.splice(index, 1);
                        changed = true;
                    }
                    failedError = err;
                    stopped = true;
                    break;
                }
            }
            syncActionStacks(action);
            trimActions();
            if (epoch === state.epoch && sessionActive() && changed && typeof state.refresh === 'function') {
                try { await state.refresh(); } catch (refreshError) {
                    if (epoch === state.epoch && sessionActive()) reportFailure(direction, refreshError);
                }
            }
            if (epoch !== state.epoch) return { status: 'cleared' };
            if (!sessionActive()) return { status: 'inactive' };
            if (failedError) reportFailure(direction, failedError);
            return {
                status: failedError ? (completed ? 'partial' : 'failed') : 'completed',
                actionId: action.id,
                label: action.label,
                changed,
                completed,
                stopped
            };
        } finally {
            state.busy = false;
            if (epoch === state.epoch) flushPendingRecords();
            publish();
        }
    }

    function configure(options) {
        options = options && typeof options === 'object' ? options : {};
        state.api = typeof options.api === 'function' ? options.api : state.api;
        state.refresh = typeof options.refresh === 'function' ? options.refresh : state.refresh;
        state.notify = typeof options.notify === 'function' ? options.notify : state.notify;
        state.isReadonly = typeof options.isReadonly === 'function' ? options.isReadonly : state.isReadonly;
        if (Object.prototype.hasOwnProperty.call(options, 'isSessionActive')) {
            state.isSessionActive = typeof options.isSessionActive === 'function' ? options.isSessionActive : null;
        }
        state.t = typeof options.t === 'function' ? options.t : state.t;
        publish();
        return snapshot();
    }

    function subscribe(callback) {
        if (typeof callback !== 'function') return () => {};
        state.listeners.add(callback);
        try { callback(snapshot()); } catch (err) { console.warn('File history subscriber failed', err); }
        return () => state.listeners.delete(callback);
    }

    function clear() {
        state.epoch++;
        state.undoStack = [];
        state.redoStack = [];
        state.pendingRecords = [];
        publish();
    }

    window.AuraDesktopFileHistory = {
        configure,
        record,
        undo: () => run('undo'),
        redo: () => run('redo'),
        snapshot,
        subscribe,
        clear
    };
})();
