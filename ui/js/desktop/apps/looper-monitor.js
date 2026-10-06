(function () {
    'use strict';

    const SVG_NS = 'http://www.w3.org/2000/svg';
    const STEP_ORDER = ['work', 'evaluate', 'finish'];
    const GAUGE_R = 34;
    const GAUGE_C = 2 * Math.PI * GAUGE_R;
    const FOLLOW_SLACK = 48;

    function h(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text != null) node.textContent = text;
        return node;
    }

    function clamp(value, min, max) {
        return Math.max(min, Math.min(max, value));
    }

    function formatClock(ms) {
        const total = Math.max(0, Math.floor((Number(ms) || 0) / 1000));
        const hours = Math.floor(total / 3600);
        const minutes = Math.floor((total % 3600) / 60);
        const seconds = total % 60;
        const pad = n => (n < 10 ? '0' : '') + n;
        return hours ? hours + ':' + pad(minutes) + ':' + pad(seconds) : minutes + ':' + pad(seconds);
    }

    // The status stream sends only log entries appended since the last message.
    // Each entry has an absolute index (logs_from + position), so the client keeps
    // its own window and merges by index. A message without logs_from (older
    // servers, test fixtures) is a full replacement.
    function newLogCache() {
        return { runId: null, base: 0, logs: [] };
    }

    function mergeStatus(cache, msg) {
        const incoming = Array.isArray(msg.logs) ? msg.logs : [];
        const from = Number.isFinite(msg.logs_from) ? msg.logs_from : 0;
        if (cache.runId !== msg.run_id) {
            cache.runId = msg.run_id;
            cache.base = from;
            cache.logs = incoming.slice();
        } else {
            const at = from - cache.base;
            if (at >= 0 && at <= cache.logs.length) {
                cache.logs = cache.logs.slice(0, at).concat(incoming);
            } else {
                cache.base = from;
                cache.logs = incoming.slice();
            }
        }
        return Object.assign({}, msg, { logs: cache.logs, logs_base: cache.base });
    }

    function highlightCode(esc, text) {
        if (!text) return '';
        return esc(text)
            .replace(/```([\s\S]*?)```/g, '<pre class="vd-looper-code-block"><code>$1</code></pre>')
            .replace(/`([^`]+)`/g, '<code class="vd-looper-code-inline">$1</code>');
    }

    function svg(tag, attrs) {
        const node = document.createElementNS(SVG_NS, tag);
        Object.keys(attrs || {}).forEach(key => node.setAttribute(key, attrs[key]));
        return node;
    }

    function buildGauge() {
        const root = svg('svg', { viewBox: '0 0 88 88', width: '88', height: '88', class: 'vd-looper-gauge-svg', 'aria-hidden': 'true' });
        root.appendChild(svg('circle', { class: 'vd-looper-gauge-track', cx: '44', cy: '44', r: String(GAUGE_R) }));
        const value = svg('circle', { class: 'vd-looper-gauge-value', cx: '44', cy: '44', r: String(GAUGE_R), transform: 'rotate(-90 44 44)' });
        value.style.strokeDasharray = String(GAUGE_C);
        value.style.strokeDashoffset = String(GAUGE_C);
        root.appendChild(value);
        const tick = svg('line', { class: 'vd-looper-gauge-tick' });
        root.appendChild(tick);
        return { root: root, value: value, tick: tick };
    }

    function setGauge(gauge, score, target) {
        const pct = clamp(Number(score) || 0, 0, 100) / 100;
        gauge.value.style.strokeDashoffset = String(GAUGE_C * (1 - pct));
        const angle = clamp(Number(target) || 0, 0, 100) / 100 * 2 * Math.PI - Math.PI / 2;
        const at = radius => (44 + radius * Math.cos(angle)).toFixed(2) + ',' + (44 + radius * Math.sin(angle)).toFixed(2);
        const inner = at(GAUGE_R - 6).split(',');
        const outer = at(GAUGE_R + 6).split(',');
        gauge.tick.setAttribute('x1', inner[0]);
        gauge.tick.setAttribute('y1', inner[1]);
        gauge.tick.setAttribute('x2', outer[0]);
        gauge.tick.setAttribute('y2', outer[1]);
    }

    // Score per round with the target as a dashed line. All numbers are computed
    // here; the only text is the translated per-point title, which is escaped.
    function chartHTML(esc, t, scores, target, width) {
        const W = width, H = 92, L = 26, R = 10, T = 10, B = 18;
        const slots = Math.max(scores.length, 5);
        const xStep = (W - L - R) / Math.max(slots - 1, 1);
        const x = i => L + i * xStep;
        const y = s => T + (1 - clamp(s, 0, 100) / 100) * (H - T - B);
        let grid = '';
        [0, 50, 100].forEach(level => {
            grid += '<line class="vd-looper-chart-grid" x1="' + L + '" x2="' + (W - R) + '" y1="' + y(level).toFixed(1) + '" y2="' + y(level).toFixed(1) + '"></line>' +
                '<text class="vd-looper-chart-axis" x="' + (L - 5) + '" y="' + (y(level) + 3).toFixed(1) + '" text-anchor="end">' + level + '</text>';
        });
        const goal = '<line class="vd-looper-chart-target" x1="' + L + '" x2="' + (W - R) + '" y1="' + y(target).toFixed(1) + '" y2="' + y(target).toFixed(1) + '"></line>';
        const points = scores.map((score, i) => x(i).toFixed(1) + ',' + y(score).toFixed(1));
        const line = points.length > 1 ? '<polyline class="vd-looper-chart-line" points="' + points.join(' ') + '"></polyline>' : '';
        const area = points.length > 1
            ? '<polygon class="vd-looper-chart-area" points="' + x(0).toFixed(1) + ',' + y(0).toFixed(1) + ' ' + points.join(' ') + ' ' + x(scores.length - 1).toFixed(1) + ',' + y(0).toFixed(1) + '"></polygon>'
            : '';
        const best = scores.reduce((acc, s, i) => (s > scores[acc] ? i : acc), 0);
        const dots = scores.map((score, i) => {
            const cls = 'vd-looper-chart-dot' + (i === scores.length - 1 ? ' is-last' : '') + (i === best ? ' is-best' : '') + (score >= target ? ' is-reached' : '');
            return '<circle class="' + cls + '" cx="' + x(i).toFixed(1) + '" cy="' + y(score).toFixed(1) + '" r="' + (i === scores.length - 1 ? 4 : 3) + '"><title>' +
                esc(t('desktop.looper_round', { n: i + 1 }) + ': ' + score) + '</title></circle>';
        }).join('');
        const every = scores.length > 12 ? Math.ceil(scores.length / 8) : 1;
        const labels = scores.map((_, i) => (i % every === 0
            ? '<text class="vd-looper-chart-axis" x="' + x(i).toFixed(1) + '" y="' + (H - 4) + '" text-anchor="middle">' + (i + 1) + '</text>'
            : '')).join('');
        return '<svg class="vd-looper-chart" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="' + esc(t('desktop.looper_chart_label')) + '">' +
            grid + goal + area + line + dots + labels + '</svg>';
    }

    function statusLabel(t, data) {
        const status = data.status || (data.running ? 'running' : (data.paused ? 'paused' : 'idle'));
        const key = 'desktop.looper_status_' + status;
        if (status === 'paused') {
            return t(key, { n: data.resume_from || data.round || 0 });
        }
        return t(key);
    }

    function verdictText(t, data, target) {
        const best = data.best_score || 0;
        const rounds = data.round || 0;
        const scores = data.score_history || [];
        const last = scores.length ? scores[scores.length - 1] : 0;
        switch (data.status) {
        case 'completed':
            return { tone: 'ok', text: t('desktop.looper_verdict_completed', { score: last, target: target, rounds: rounds }) };
        case 'stalled':
            return { tone: 'warn', text: t('desktop.looper_verdict_stalled', { best: best, target: target }) };
        case 'max_rounds':
            return { tone: 'warn', text: t('desktop.looper_verdict_max_rounds', { best: best, target: target }) };
        case 'stopped':
            return { tone: 'neutral', text: t('desktop.looper_verdict_stopped', { rounds: rounds, best: best }) };
        default:
            return null;
        }
    }

    // A run view owns the DOM of one live (or stored) run. It is built once and
    // updated in place: new log entries are appended, nothing is rebuilt, so text
    // selection, scroll position and open/closed rounds survive every update.
    function createRunView(host, helpers) {
        const t = helpers.t;
        const esc = helpers.esc;
        const scroller = helpers.scroller || null;
        const live = helpers.live !== false;

        host.textContent = '';
        const view = h('div', 'vd-looper-run');
        host.appendChild(view);

        const gauge = buildGauge();
        const scoreNum = h('span', 'vd-looper-score-num', '–');
        const gaugeBox = h('div', 'vd-looper-gauge');
        gaugeBox.appendChild(gauge.root);
        gaugeBox.appendChild(scoreNum);
        const targetLabel = h('div', 'vd-looper-gauge-label');
        const gaugeCol = h('div', 'vd-looper-gauge-col');
        gaugeCol.appendChild(gaugeBox);
        gaugeCol.appendChild(targetLabel);

        const statusEl = h('span', 'vd-looper-status');
        const bestEl = h('span', 'vd-looper-best');
        const head = h('div', 'vd-looper-run-head');
        head.appendChild(statusEl);
        head.appendChild(bestEl);

        const roundLine = h('div', 'vd-looper-roundline');
        const fill = h('span', 'vd-looper-progress-fill');
        const progress = h('div', 'vd-looper-progress');
        progress.setAttribute('role', 'progressbar');
        progress.setAttribute('aria-valuemin', '0');
        progress.setAttribute('aria-valuemax', '100');
        progress.appendChild(fill);

        const steps = h('ol', 'vd-looper-steps');
        const stepEls = {};
        STEP_ORDER.forEach(step => {
            const li = h('li', 'vd-looper-step', t('desktop.looper_step_' + step));
            li.dataset.step = step;
            stepEls[step] = li;
            steps.appendChild(li);
        });
        const pauseNote = h('p', 'vd-looper-pausenote');
        pauseNote.setAttribute('role', 'status');
        const meta = h('div', 'vd-looper-run-meta');

        const info = h('div', 'vd-looper-hero-info');
        [head, roundLine, progress, steps, pauseNote, meta].forEach(node => info.appendChild(node));
        const hero = h('section', 'vd-looper-hero');
        hero.appendChild(gaugeCol);
        hero.appendChild(info);

        const verdict = h('div', 'vd-looper-verdict');
        const chartWrap = h('div', 'vd-looper-chartwrap');
        const feedback = h('blockquote', 'vd-looper-feedback');
        const errorEl = h('p', 'vd-looper-error');
        errorEl.setAttribute('role', 'alert');
        const empty = h('div', 'vd-looper-empty');
        const emptyArt = h('img', 'vd-looper-empty-art');
        emptyArt.src = '/img/looper-empty.png';
        emptyArt.alt = '';
        emptyArt.width = 320;
        emptyArt.height = 320;
        emptyArt.draggable = false;
        empty.appendChild(emptyArt);
        empty.appendChild(h('p', 'vd-looper-log-empty', t('desktop.looper_no_run')));
        empty.appendChild(h('p', 'vd-looper-help', t('desktop.looper_how_it_works')));
        const timeline = h('div', 'vd-looper-timeline');
        const noLogs = h('div', 'vd-looper-log-empty', t('desktop.looper_no_logs'));

        [hero, verdict, chartWrap, feedback, errorEl, empty, noLogs, timeline].forEach(node => view.appendChild(node));

        // timeline bookkeeping
        const rounds = new Map();
        const touched = new Set();
        const pending = h('div', 'vd-looper-log vd-looper-log--pending vd-looper-log--active');
        const pendingStep = h('span', 'vd-looper-log-step');
        pending.appendChild(h('div', 'vd-looper-log-header'));
        pending.firstChild.appendChild(pendingStep);
        pending.firstChild.appendChild(h('span', 'vd-looper-log-spinner'));
        pending.firstChild.lastChild.setAttribute('aria-hidden', 'true');
        let lastRunId;
        let renderedTo = 0;
        let latestRoundKey = null;
        let lastScore = null;
        let following = true;
        let chartKey = '';
        let chartData = { scores: [], target: 85 };

        // Drawn at the real pixel width, so axis text keeps its size in any window.
        function drawChart() {
            const scores = chartData.scores;
            if (!scores.length || chartWrap.hidden) {
                chartKey = '';
                chartWrap.textContent = '';
                return;
            }
            const width = clamp(Math.round(chartWrap.clientWidth - 20), 240, 960);
            const key = JSON.stringify([scores, chartData.target, width]);
            if (key === chartKey) return;
            chartKey = key;
            chartWrap.innerHTML = chartHTML(esc, t, scores, chartData.target, width);
        }

        // Redraw on the next frame: changing the observed box inside its own
        // callback would make the browser report undelivered notifications.
        const chartObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(() => requestAnimationFrame(drawChart)) : null;
        if (chartObserver) chartObserver.observe(chartWrap);
        let timer = null;
        let clockBase = 0;
        let clockAt = 0;
        let clockRunning = false;
        let clockEl = null;
        let lastData = null;

        function nearBottom() {
            return !scroller || scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight <= FOLLOW_SLACK;
        }

        function scrollToBottom() {
            if (scroller) scroller.scrollTop = scroller.scrollHeight;
        }

        function setFollowing(value) {
            if (following === value) return;
            following = value;
            if (helpers.onFollowChange) helpers.onFollowChange(following);
        }

        function onScroll() {
            setFollowing(nearBottom());
        }

        if (scroller && live) scroller.addEventListener('scroll', onScroll, { passive: true });

        function roundTitle(round) {
            return round > 0 ? t('desktop.looper_round', { n: round }) : t('desktop.looper_step_finish');
        }

        function setExpanded(state, expanded) {
            state.expanded = expanded;
            state.el.classList.toggle('is-collapsed', !expanded);
            state.head.setAttribute('aria-expanded', String(expanded));
            state.head.title = t(expanded ? 'desktop.looper_collapse_log' : 'desktop.looper_expand_log');
        }

        function ensureRound(round) {
            const key = round > 0 ? 'r' + round : 'finish';
            let state = rounds.get(key);
            if (state) return state;
            const el = h('section', 'vd-looper-round');
            const headBtn = h('button', 'vd-looper-round-head');
            headBtn.type = 'button';
            const title = h('span', 'vd-looper-round-title', roundTitle(round));
            const score = h('span', 'vd-looper-round-score');
            const delta = h('span', 'vd-looper-round-delta');
            const time = h('span', 'vd-looper-round-time');
            const chevron = h('span', 'vd-looper-chevron');
            chevron.setAttribute('aria-hidden', 'true');
            [title, score, delta, time, chevron].forEach(node => headBtn.appendChild(node));
            const body = h('div', 'vd-looper-round-body');
            el.appendChild(headBtn);
            el.appendChild(body);
            state = { key: key, round: round, el: el, head: headBtn, body: body, score: score, delta: delta, time: time, ms: 0, expanded: true };
            headBtn.dataset.roundKey = key;
            rounds.set(key, state);
            if (round > 0 && latestRoundKey && latestRoundKey !== key && !touched.has(latestRoundKey)) {
                setExpanded(rounds.get(latestRoundKey), false);
            }
            if (round > 0) latestRoundKey = key;
            setExpanded(state, true);
            timeline.appendChild(el);
            return state;
        }

        function entryNode(log, abs) {
            const step = log.step || 'work';
            const node = h('div', 'vd-looper-log' + (log.failed ? ' vd-looper-log--failed' : ''));
            node.dataset.step = step;
            node.dataset.logKey = 'e' + abs;
            const header = h('button', 'vd-looper-log-header');
            header.type = 'button';
            header.setAttribute('aria-expanded', 'true');
            header.appendChild(h('span', 'vd-looper-log-step', t('desktop.looper_step_' + step)));
            if (step === 'evaluate') {
                if (log.failed) {
                    const failed = h('span', 'vd-looper-log-failed', t('desktop.looper_eval_failed'));
                    failed.title = t('desktop.looper_eval_failed_help');
                    header.appendChild(failed);
                } else {
                    header.appendChild(h('span', 'vd-looper-log-score', String(Number(log.score) || 0)));
                }
            }
            header.appendChild(h('span', 'vd-looper-log-time', helpers.formatDuration(log.duration_ms || log.duration || 0)));
            node.appendChild(header);
            if (log.feedback || log.response) {
                const body = h('div', 'vd-looper-log-body');
                if (log.feedback) body.appendChild(h('div', 'vd-looper-log-reason', log.feedback));
                if (log.response) {
                    const response = h('div', 'vd-looper-log-response');
                    response.innerHTML = highlightCode(esc, log.response);
                    body.appendChild(response);
                }
                node.appendChild(body);
            }
            return node;
        }

        function applyEntry(log, abs) {
            const round = Number(log.round || 0);
            const state = ensureRound(round);
            state.body.appendChild(entryNode(log, abs));
            state.ms += Number(log.duration_ms || log.duration || 0);
            state.time.textContent = helpers.formatDuration(state.ms);
            if (log.step === 'evaluate') {
                if (log.failed) {
                    state.score.textContent = '–';
                    state.score.title = t('desktop.looper_eval_failed_help');
                    state.score.classList.add('is-failed');
                } else {
                    const score = Number(log.score) || 0;
                    state.score.textContent = String(score);
                    state.score.title = '';
                    state.score.classList.remove('is-failed');
                    if (lastScore != null && score !== lastScore) {
                        const diff = score - lastScore;
                        state.delta.textContent = (diff > 0 ? '+' : '') + diff;
                        state.delta.className = 'vd-looper-round-delta ' + (diff > 0 ? 'is-up' : 'is-down');
                    } else {
                        state.delta.textContent = '';
                    }
                    lastScore = score;
                }
            }
        }

        function resetTimeline(base) {
            timeline.textContent = '';
            rounds.clear();
            touched.clear();
            latestRoundKey = null;
            lastScore = null;
            renderedTo = base;
        }

        function syncTimeline(data) {
            const logs = data.logs || [];
            const base = Number.isFinite(data.logs_base) ? data.logs_base : 0;
            if (data.run_id !== lastRunId || renderedTo < base || renderedTo > base + logs.length) {
                lastRunId = data.run_id;
                resetTimeline(base);
            }
            for (let abs = Math.max(renderedTo, base); abs < base + logs.length; abs++) {
                applyEntry(logs[abs - base], abs);
            }
            renderedTo = base + logs.length;

            const step = data.running ? data.current_step : '';
            const lastLog = logs.length ? logs[logs.length - 1] : null;
            const duplicate = lastLog && lastLog.step === step && (step === 'finish' || Number(lastLog.round) === Number(data.round));
            if (step && STEP_ORDER.indexOf(step) >= 0 && !duplicate) {
                pendingStep.textContent = t('desktop.looper_step_' + step) + (step === 'finish' ? '' : ' · ' + t('desktop.looper_round', { n: data.round || 1 }));
                timeline.appendChild(pending);
            } else if (pending.parentNode) {
                pending.parentNode.removeChild(pending);
            }
        }

        // clock
        function clockNow() {
            return clockBase + (clockRunning ? performance.now() - clockAt : 0);
        }

        function tickClock() {
            if (clockEl) clockEl.textContent = t('desktop.looper_elapsed', { time: formatClock(clockNow()) });
        }

        function setClock(data, logs) {
            const reported = Number(data.elapsed_ms);
            clockBase = Number.isFinite(reported) && reported > 0
                ? reported
                : logs.reduce((sum, log) => sum + Number(log.duration_ms || log.duration || 0), 0);
            clockAt = performance.now();
            clockRunning = !!data.running && Number.isFinite(reported);
            if (clockRunning && !timer && live) timer = setInterval(tickClock, 1000);
            if (!clockRunning && timer) { clearInterval(timer); timer = null; }
        }

        // update
        function update(data) {
            lastData = data;
            const wasFollowing = live ? (following || nearBottom()) : false;
            const logs = data.logs || [];
            const running = !!data.running;
            const paused = !!data.paused || data.status === 'paused';
            const status = data.status || (running ? 'running' : (paused ? 'paused' : 'idle'));
            const scores = (data.score_history || []).map(Number).filter(Number.isFinite);
            const target = Number(data.target_score) || Number(helpers.fallbackTarget && helpers.fallbackTarget()) || 85;
            const last = scores.length ? scores[scores.length - 1] : null;
            const isEmpty = !running && !paused && !logs.length && status === 'idle';

            view.dataset.state = status;
            hero.hidden = isEmpty;
            empty.hidden = !isEmpty;
            timeline.hidden = isEmpty;
            noLogs.hidden = isEmpty || running || logs.length > 0;
            gaugeBox.setAttribute('role', 'img');

            scoreNum.textContent = last == null ? '–' : String(last);
            gaugeBox.setAttribute('aria-label', t('desktop.looper_score_label') + ' ' + scoreNum.textContent);
            setGauge(gauge, last == null ? 0 : last, target);
            gaugeBox.classList.toggle('is-reached', last != null && last >= target);
            targetLabel.textContent = t('desktop.looper_target_short', { score: target });

            statusEl.className = 'vd-looper-status vd-looper-status--' + status;
            statusEl.textContent = statusLabel(t, data);
            bestEl.hidden = !data.best_score;
            bestEl.textContent = data.best_score ? t('desktop.looper_best_score', { score: data.best_score }) : '';

            const maxRounds = Number(data.max_rounds) || 0;
            roundLine.hidden = !(data.round > 0 && maxRounds > 0);
            if (!roundLine.hidden) roundLine.textContent = t('desktop.looper_round_of', { n: data.round, max: data.max_rounds });
            const stepFraction = { work: 0.3, evaluate: 0.75, finish: 1 }[data.current_step] || 0;
            const doneRounds = running ? Math.max(0, (data.round || 1) - 1) + stepFraction : (data.round || 0);
            const pct = maxRounds > 0 ? Math.round(clamp(doneRounds / maxRounds, 0, 1) * 100) : 0;
            fill.style.width = pct + '%';
            progress.hidden = maxRounds <= 0 || isEmpty;
            progress.setAttribute('aria-valuenow', String(pct));

            const current = running ? data.current_step : '';
            const currentIdx = STEP_ORDER.indexOf(current);
            const showFinish = !!data.has_finish || logs.some(log => log.step === 'finish');
            STEP_ORDER.forEach((step, idx) => {
                const li = stepEls[step];
                li.hidden = step === 'finish' && !showFinish;
                li.classList.toggle('is-active', step === current);
                li.classList.toggle('is-done', currentIdx > idx);
            });
            steps.hidden = isEmpty || !running;

            let note = '';
            if (running && data.pause_requested) note = t('desktop.looper_pause_pending');
            else if (paused && data.pause_reason === 'budget') note = t('desktop.looper_budget_paused');
            else if (paused && data.pause_reason === 'interrupted') note = t('desktop.looper_interrupted', { n: (Number(data.resume_from) || 0) + 1 });
            pauseNote.hidden = !note;
            pauseNote.textContent = note;

            const metaParts = [];
            const tokens = (data.input_tokens || 0) + (data.output_tokens || 0);
            if (tokens) metaParts.push(t('desktop.looper_tokens', { count: tokens }));
            const cost = helpers.formatCost(data.estimated_cost_usd);
            if (cost) metaParts.push(data.cost_approximate ? t('desktop.looper_cost_approx', { cost: cost }) : cost);
            meta.textContent = '';
            clockEl = h('span', 'vd-looper-clock');
            meta.appendChild(clockEl);
            if (metaParts.length) meta.appendChild(document.createTextNode(' · ' + metaParts.join(' · ')));
            setClock(data, logs);
            tickClock();

            const outcome = running || paused ? null : verdictText(t, data, target);
            verdict.hidden = !outcome;
            verdict.className = 'vd-looper-verdict' + (outcome ? ' vd-looper-verdict--' + outcome.tone : '');
            verdict.textContent = outcome ? outcome.text : '';

            chartData = { scores: scores, target: target };
            chartWrap.hidden = !scores.length;
            drawChart();

            feedback.hidden = !data.last_feedback;
            feedback.textContent = data.last_feedback || '';
            errorEl.hidden = !data.error;
            errorEl.textContent = data.error ? t('desktop.looper_error_detail', { message: data.error }) : '';

            syncTimeline(data);
            if (live && wasFollowing) scrollToBottom();
        }

        timeline.addEventListener('click', ev => {
            const roundHead = ev.target.closest('.vd-looper-round-head');
            if (roundHead) {
                const state = rounds.get(roundHead.dataset.roundKey);
                if (state) {
                    touched.add(state.key);
                    setExpanded(state, !state.expanded);
                }
                return;
            }
            const header = ev.target.closest('.vd-looper-log-header');
            if (header && header.parentNode && header.parentNode.classList.contains('vd-looper-log')) {
                const entry = header.parentNode;
                const collapsed = entry.classList.toggle('vd-looper-log--collapsed');
                header.setAttribute('aria-expanded', String(!collapsed));
            }
        });

        function followLatest() {
            following = true;
            scrollToBottom();
            if (helpers.onFollowChange) helpers.onFollowChange(true);
        }

        function destroy() {
            if (chartObserver) chartObserver.disconnect();
            if (timer) { clearInterval(timer); timer = null; }
            if (scroller && live) scroller.removeEventListener('scroll', onScroll);
        }

        return {
            update: update,
            destroy: destroy,
            followLatest: followLatest,
            isFollowing: () => following,
            last: () => lastData
        };
    }

    function renderHistoryList(root, runs, helpers) {
        if (!root || !helpers) return;
        const esc = helpers.esc;
        const t = helpers.t;
        const formatCost = helpers.formatCost;
        if (!runs || !runs.length) {
            root.innerHTML = '<div class="vd-looper-log-empty">' + esc(t('desktop.looper_history_empty')) + '</div>';
            return;
        }
        root.innerHTML = '<ul class="vd-looper-history-list">' + runs.map(run => {
            const when = run.finished_at || run.started_at || '';
            const cost = formatCost(run.cost_usd);
            const best = clamp(Number(run.best_score) || 0, 0, 100);
            const goal = clamp(Number(run.target_score) || 0, 0, 100);
            return '<li>' +
                '<button type="button" class="vd-looper-history-item" data-run-id="' + esc(String(run.id)) + '">' +
                '<span class="vd-looper-history-name">' + esc(run.preset_name || t('desktop.looper_untitled')) + '</span>' +
                '<span class="vd-looper-status vd-looper-status--' + esc(run.status || 'idle') + '">' + esc(t('desktop.looper_status_' + (run.status || 'idle'))) + '</span>' +
                '<span class="vd-looper-history-meter" aria-hidden="true"><span class="vd-looper-history-meter-fill" style="width:' + best + '%"></span>' +
                (goal ? '<span class="vd-looper-history-meter-goal" style="left:' + goal + '%"></span>' : '') + '</span>' +
                '<span class="vd-looper-history-meta">' + esc([
                    t('desktop.looper_round_of', { n: run.rounds || 0, max: run.max_rounds || 0 }),
                    run.best_score ? t('desktop.looper_best_score', { score: run.best_score }) : '',
                    cost,
                    when ? String(when).slice(0, 16).replace('T', ' ') : ''
                ].filter(Boolean).join(' · ')) + '</span>' +
                '<span class="vd-looper-history-excerpt">' + esc(run.goal_excerpt || '') + '</span>' +
                '</button>' +
                '<button type="button" class="vd-looper-history-delete" data-run-id="' + esc(String(run.id)) + '" title="' + esc(t('desktop.looper_history_delete')) + '">' + esc(t('desktop.looper_delete')) + '</button>' +
                '</li>';
        }).join('') + '</ul>' +
            '<button type="button" class="vd-looper-history-clear">' + esc(t('desktop.looper_history_clear')) + '</button>';
    }

    // Stored runs reuse the live view, fed once with the saved data. Returns the
    // view so the caller can dispose it.
    function renderHistoryDetail(root, run, helpers) {
        if (!root || !helpers || !run) return null;
        const esc = helpers.esc;
        const t = helpers.t;
        // A run that kept its settings can be loaded into the editor or started
        // again; older runs only say why they cannot.
        const reuse = run.config
            ? '<button type="button" class="vd-looper-history-load">' + esc(t('desktop.looper_history_load')) + '</button>' +
                '<button type="button" class="vd-looper-history-rerun">' + esc(t('desktop.looper_history_rerun')) + '</button>'
            : '<span class="vd-looper-help">' + esc(t('desktop.looper_history_no_config')) + '</span>';
        root.innerHTML = '<div class="vd-looper-history-detail">' +
            '<div class="vd-looper-history-actions">' +
            '<button type="button" class="vd-looper-history-back">' + esc(t('desktop.looper_history_back')) + '</button>' + reuse +
            '</div></div>';
        const host = document.createElement('div');
        root.querySelector('.vd-looper-history-detail').appendChild(host);
        const logs = run.logs || [];
        const view = createRunView(host, Object.assign({}, helpers, { live: false, scroller: null }));
        view.update({
            status: run.status,
            running: false,
            paused: false,
            round: run.rounds,
            max_rounds: run.max_rounds,
            target_score: run.target_score,
            score_history: logs.filter(l => l.step === 'evaluate' && !l.failed).map(l => l.score).filter(n => Number.isFinite(Number(n))),
            best_score: run.best_score,
            last_feedback: logs.reduce((acc, log) => log.feedback || acc, ''),
            error: run.error,
            logs: logs,
            logs_base: 0,
            run_id: 'history-' + run.id,
            input_tokens: run.input_tokens,
            output_tokens: run.output_tokens,
            estimated_cost_usd: run.cost_usd
        });
        return view;
    }

    window.LooperMonitor = {
        newLogCache: newLogCache,
        mergeStatus: mergeStatus,
        createRunView: createRunView,
        renderHistoryList: renderHistoryList,
        renderHistoryDetail: renderHistoryDetail,
        formatClock: formatClock
    };
})();
