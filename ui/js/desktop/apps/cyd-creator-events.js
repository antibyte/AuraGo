// CYD Creator event stream: one EventSource per open project (GET /projects/{id}/events), resumed
// with ?after=<last seq>, reconnected with backoff. The backlog before stream_ready is coalesced
// (latest state per job/build/device, every message, the steps of each job) and replayed at
// stream_ready, so a window does not repaint its whole history. Everything reaches inst.bus as
// sse:<type>(data, meta); events:state reports connecting | live | reconnecting, events:failed(n)
// the number of failures in a row. stream_ready
// carries {"after":N} (the seq the server continues after); with "reload":true the window's state
// cannot be continued (events after the resume point were pruned, or the database was restored or
// reset): the cursor moves to N and the app must reload project state.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const TYPES = ['project_changed', 'lock_changed', 'build_status', 'build_log', 'job_status', 'job_step', 'job_message', 'device_changed', 'stream_ready'];
    const MAX_BACKOFF_MS = 30000;
    const STEPS_PER_JOB = 100;
    const MESSAGES_MAX = 500;

    // createCursor remembers the highest persisted seq. accept(lastEventId) is false for a seq the
    // window already applied (a reconnect replays from `after`, never before it); events without an
    // id (ephemeral: build_log, stream_ready) are always accepted. reset(seq) moves the cursor to
    // the server's continuation point after a reload, also downwards (a restored database).
    function createCursor(start) {
        let after = Number(start) > 0 ? Number(start) : 0;
        return {
            get after() { return after; },
            accept(id) {
                if (id === undefined || id === null || id === '') return true;
                const seq = Number(id);
                if (!Number.isFinite(seq) || seq <= 0) return true;
                if (seq <= after) return false;
                after = seq;
                return true;
            },
            reset(seq) { after = Number(seq) > 0 ? Number(seq) : 0; }
        };
    }

    // backoff(attempt) is 1 s, 2 s, 4 s … capped at 30 s.
    function backoff(attempt) {
        const n = Math.max(0, Number(attempt) || 0);
        return Math.min(MAX_BACKOFF_MS, 1000 * Math.pow(2, n));
    }

    // jitter spreads the reconnects of many windows: ms ±20 %, kept within 1–30 s. random() is
    // Math.random unless a test passes its own.
    function jitter(ms, random) {
        const r = typeof random === 'function' ? Number(random()) || 0 : Math.random();
        return Math.round(Math.min(MAX_BACKOFF_MS, Math.max(1000, ms * (0.8 + 0.4 * r))));
    }

    // createCatchup coalesces the backlog: drain() returns [type, data] pairs in apply order. Its
    // size is bounded: the server replays at most the project's retained events (5000), messages
    // keep the newest MESSAGES_MAX and each job keeps its newest STEPS_PER_JOB steps.
    function createCatchup() {
        let lock = null;
        const projectChanges = new Map(); // what → data with the highest version
        const builds = new Map();
        const jobs = new Map();
        const devices = new Map();
        const messages = [];
        const steps = new Map();
        let lastStepJob = '';
        const ephemeral = [];
        return {
            add(type, data) {
                const d = data || {};
                switch (type) {
                case 'lock_changed': lock = d; break;
                case 'project_changed': {
                    const what = String(d.what || 'layout');
                    const prev = projectChanges.get(what);
                    if (!prev || Number(d.version) >= Number(prev.version)) projectChanges.set(what, d);
                    break;
                }
                case 'build_status': if (d.build && d.build.id) { builds.delete(d.build.id); builds.set(d.build.id, d); } break;
                case 'job_status': if (d.job && d.job.id) { jobs.delete(d.job.id); jobs.set(d.job.id, d); } break;
                case 'device_changed': if (d.device && d.device.id) { devices.delete(d.device.id); devices.set(d.device.id, d); } break;
                case 'job_message': messages.push(d); if (messages.length > MESSAGES_MAX) messages.shift(); break;
                case 'job_step': {
                    const id = String(d.job_id || '');
                    const list = steps.get(id) || [];
                    list.push(d);
                    if (list.length > STEPS_PER_JOB) list.shift();
                    steps.set(id, list);
                    lastStepJob = id;
                    break;
                }
                default: ephemeral.push([type, d]);
                }
            },
            drain() {
                const out = [];
                if (lock) out.push(['lock_changed', lock]);
                projectChanges.forEach(d => out.push(['project_changed', d]));
                builds.forEach(d => out.push(['build_status', d]));
                jobs.forEach(d => out.push(['job_status', d]));
                devices.forEach(d => out.push(['device_changed', d]));
                messages.forEach(d => out.push(['job_message', d]));
                const jobIds = Array.from(jobs.keys());
                // Steps are shown for one job only: the newest job in the backlog, else the job of the
                // newest step (a reconnect backlog of a running job has steps but no status).
                const stepJob = jobIds.length ? jobIds[jobIds.length - 1] : lastStepJob;
                (steps.get(stepJob) || []).forEach(d => out.push(['job_step', d]));
                ephemeral.forEach(pair => out.push(pair));
                return out;
            }
        };
    }

    // connect(inst) opens the stream for inst.projectId. opts.EventSource overrides the constructor
    // and opts.random the jitter source (tests); opts.after is a resume point. Returns {close(), after}.
    function connect(inst, opts) {
        const o = opts || {};
        const cursor = createCursor(o.after || 0);
        let source = null;
        let attempt = 0;
        let timer = 0;
        let closed = false;
        let catchup = createCatchup();
        let catching = true;
        // The resume point of the running catch-up. A stream cut off before stream_ready is resumed
        // from the same point, not after the buffered seqs, so the server can still tell that events
        // after it were pruned; the replayed seqs are dropped by the cursor. It does not move while
        // backlogs keep being cut off; each retry then re-reads that backlog, which the server's
        // retention (5000 events per project) bounds, and the backoff spaces the retries.
        let catchupFrom = cursor.after;
        // Per source: the last lastEventId seen and the highest seq the server sent. A browser
        // EventSource keeps lastEventId for an event without an id line (build_log, stream_ready),
        // and the server never repeats an id on one stream, so an unchanged lastEventId means "no id".
        let lastId = '';
        let streamSeq = 0;

        function emitState(state) { inst.bus.emit('events:state', state); }

        function deliver(type, data, seq) {
            inst.bus.emit('sse:' + type, data, { seq: seq || 0 });
        }

        function eventSeq(ev) {
            const id = ev.lastEventId == null ? '' : String(ev.lastEventId);
            const fresh = id !== lastId;
            lastId = id;
            const seq = fresh ? Number(id) : 0;
            return Number.isFinite(seq) && seq > 0 ? seq : 0;
        }

        // continuation is stream_ready's data.after, the seq the server's live loop continues after
        // (0 when absent, e.g. an older server or a fixture).
        function continuation(data) {
            const after = Number(data && data.after);
            return Number.isFinite(after) && after > 0 ? after : 0;
        }

        function onEvent(current, type, ev) {
            if (closed || current !== source) return;
            const seq = eventSeq(ev);
            let data = {};
            if (ev.data) {
                // A malformed frame is dropped before it can move the cursor.
                try { data = JSON.parse(ev.data); } catch (_) { return; }
            }
            if (seq > streamSeq) streamSeq = seq;
            if (seq > 0 && !cursor.accept(seq)) return;
            if (type === 'stream_ready') {
                attempt = 0;
                // The server continues after data.after (at least the last event it sent here).
                if (data && data.reload === true) cursor.reset(Math.max(streamSeq, continuation(data)));
                const backlog = catchup.drain();
                catchup = createCatchup();
                catching = false;
                backlog.forEach(pair => deliver(pair[0], pair[1], 0));
                emitState('live');
                deliver('stream_ready', data, 0);
                return;
            }
            if (catching && type !== 'build_log') { catchup.add(type, data); return; }
            deliver(type, data, seq);
        }

        function open() {
            if (closed) return;
            const ES = o.EventSource || window.EventSource;
            if (typeof ES !== 'function') return;
            // A backlog interrupted by an error stays buffered: the next stream resumes from the same point.
            if (!catching) { catching = true; catchup = createCatchup(); catchupFrom = cursor.after; }
            emitState(attempt === 0 ? 'connecting' : 'reconnecting');
            const current = new ES(P.api.eventsUrl(inst.projectId, catchupFrom));
            source = current;
            lastId = '';
            streamSeq = 0;
            TYPES.forEach(type => current.addEventListener(type, ev => onEvent(current, type, ev)));
            current.onerror = () => {
                if (closed || current !== source) return;
                current.close();
                source = null;
                emitState('reconnecting');
                timer = setTimeout(open, jitter(backoff(attempt), o.random));
                attempt++;
                // events:failed(n): n stream failures in a row since the last stream_ready. The app
                // asks the API after a few whether reconnecting can work at all (deleted project,
                // expired session, CYD Creator switched off) and then calls close().
                inst.bus.emit('events:failed', attempt);
            };
        }

        open();
        return {
            close() {
                closed = true;
                clearTimeout(timer);
                if (source) source.close();
                source = null;
            },
            get after() { return cursor.after; }
        };
    }

    P.events = { TYPES, connect, createCursor, createCatchup, backoff };
})();
