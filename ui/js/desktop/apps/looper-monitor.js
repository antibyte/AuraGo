(function () {
    'use strict';

    function sparklineSVG(scores) {
        const values = (scores || []).map(n => Number(n)).filter(n => Number.isFinite(n));
        if (!values.length) {
            return '';
        }
        const w = 132;
        const h = 36;
        const min = 0;
        const max = 100;
        const step = values.length === 1 ? 0 : (w - 8) / (values.length - 1);
        const points = values.map((score, i) => {
            const x = 4 + i * step;
            const y = h - 4 - ((Math.max(min, Math.min(max, score)) - min) / (max - min)) * (h - 8);
            return x.toFixed(1) + ',' + y.toFixed(1);
        }).join(' ');
        const last = values[values.length - 1];
        const lastX = 4 + (values.length - 1) * step;
        const lastY = h - 4 - ((Math.max(min, Math.min(max, last)) - min) / (max - min)) * (h - 8);
        return '<svg class="vd-looper-sparkline" viewBox="0 0 ' + w + ' ' + h + '" width="' + w + '" height="' + h + '" aria-hidden="true">' +
            '<polyline points="' + points + '" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"></polyline>' +
            '<circle cx="' + lastX.toFixed(1) + '" cy="' + lastY.toFixed(1) + '" r="3" fill="currentColor"></circle>' +
            '</svg>';
    }

    function logKey(log, index) {
        return String(log.round || 0) + '|' + String(log.step || '') + '|' + String(log.duration_ms || log.duration || 0) + '|' + index;
    }

    function highlightCode(esc, text) {
        if (!text) return '';
        return esc(text)
            .replace(/```([\s\S]*?)```/g, '<pre class="vd-looper-code-block"><code>$1</code></pre>')
            .replace(/`([^`]+)`/g, '<code class="vd-looper-code-inline">$1</code>');
    }

    function groupLogs(logs) {
        const groups = [];
        const map = new Map();
        (logs || []).forEach((log, index) => {
            const round = Number(log.round || 0);
            let group = map.get(round);
            if (!group) {
                group = { round: round, logs: [] };
                map.set(round, group);
                groups.push(group);
            }
            group.logs.push({ log: log, index: index });
        });
        return groups;
    }

    function pendingStepHTML(esc, t, currentStep, lastLogStep, running) {
        if (!running || !currentStep || currentStep === lastLogStep) {
            return '';
        }
        if (currentStep === 'idle' || currentStep === 'paused' || currentStep === 'stopped') {
            return '';
        }
        return '<div class="vd-looper-log vd-looper-log--pending vd-looper-log--active">' +
            '<div class="vd-looper-log-header">' +
            '<span class="vd-looper-log-step">' + esc(t('desktop.looper_step_' + currentStep)) + '</span>' +
            '<span class="vd-looper-log-spinner" aria-hidden="true"></span>' +
            '</div></div>';
    }

    function renderTimeline(esc, t, formatDuration, logs, currentStep, running, expandState) {
        if (!logs || !logs.length) {
            return '<div class="vd-looper-log-empty">' + esc(t('desktop.looper_no_logs')) + '</div>';
        }
        const groups = groupLogs(logs);
        const lastRound = groups.length ? groups[groups.length - 1].round : 0;
        const lastLogStep = logs.length ? (logs[logs.length - 1].step || '') : '';
        return groups.slice().reverse().map((group, gi) => {
            const isActive = running && group.round === lastRound && group.round > 0;
            const title = group.round > 0
                ? t('desktop.looper_round', { n: group.round })
                : t('desktop.looper_step_finish');
            const items = group.logs.map(({ log, index }) => {
                const key = logKey(log, index);
                let expanded = expandState.has(key) ? expandState.get(key) : index === logs.length - 1;
                const stepLabel = t('desktop.looper_step_' + (log.step || 'work'));
                const scoreHtml = log.step === 'evaluate'
                    ? '<span class="vd-looper-log-score">' + esc(String(Number(log.score) || 0)) + '</span>'
                    : '';
                const active = running && log.step === currentStep && index === logs.length - 1;
                const body = (log.feedback || log.response || log.prompt)
                    ? '<div class="vd-looper-log-body">' +
                        (log.feedback ? '<div class="vd-looper-log-reason">' + esc(log.feedback) + '</div>' : '') +
                        (log.response ? '<div class="vd-looper-log-response">' + highlightCode(esc, log.response) + '</div>' : '') +
                        '</div>'
                    : '';
                return '<div class="vd-looper-log' + (expanded ? '' : ' vd-looper-log--collapsed') + (active ? ' vd-looper-log--active' : '') + '" data-log-key="' + esc(key) + '">' +
                    '<button type="button" class="vd-looper-log-header" aria-expanded="' + String(expanded) + '">' +
                    '<span class="vd-looper-log-step">' + esc(stepLabel) + '</span>' +
                    scoreHtml +
                    (active ? '<span class="vd-looper-log-spinner" aria-hidden="true"></span>' : '') +
                    '<span class="vd-looper-log-time">' + esc(formatDuration(log.duration_ms || log.duration || 0)) + '</span>' +
                    '</button>' +
                    body +
                    '</div>';
            }).join('');
            const pending = gi === 0 ? pendingStepHTML(esc, t, currentStep, lastLogStep, running) : '';
            return '<section class="vd-looper-round' + (isActive ? ' vd-looper-round--active' : '') + '">' +
                '<h3 class="vd-looper-round-title">' + esc(title) + '</h3>' +
                items + pending +
                '</section>';
        }).join('');
    }

    function statusLabel(t, data) {
        const status = data.status || (data.running ? 'running' : (data.paused ? 'paused' : 'idle'));
        const key = 'desktop.looper_status_' + status;
        if (status === 'paused') {
            return t(key, { n: data.resume_from || data.round || 0 });
        }
        return t(key);
    }

    function renderRun(root, data, helpers) {
        if (!root || !helpers) return;
        const esc = helpers.esc;
        const t = helpers.t;
        const formatCost = helpers.formatCost;
        const formatDuration = helpers.formatDuration;
        const expandState = helpers.expandState || new Map();
        const scores = data.score_history || [];
        const cost = formatCost(data.estimated_cost_usd, t);
        const tokens = (data.input_tokens || 0) + (data.output_tokens || 0);
        const meta = [];
        if (data.round > 0 && data.max_rounds > 0) {
            meta.push(t('desktop.looper_round_of', { n: data.round, max: data.max_rounds }));
        }
        if (tokens) meta.push(t('desktop.looper_tokens', { count: tokens }));
        if (cost) meta.push(cost);
        const empty = !data.running && !data.paused && !(data.logs && data.logs.length);
        root.innerHTML = '<div class="vd-looper-run">' +
            '<div class="vd-looper-run-head">' +
            '<span class="vd-looper-status vd-looper-status--' + esc(data.status || 'idle') + '">' + esc(statusLabel(t, data)) + '</span>' +
            (data.best_score ? '<span class="vd-looper-best">' + esc(t('desktop.looper_best_score', { score: data.best_score })) + '</span>' : '') +
            '</div>' +
            (empty
                ? '<div class="vd-looper-log-empty">' + esc(t('desktop.looper_no_run')) + '</div>'
                : sparklineSVG(scores) +
                    (data.error ? '<p class="vd-looper-error" role="alert">' + esc(t('desktop.looper_error_detail', { message: data.error })) + '</p>' : '') +
                    (data.last_feedback ? '<blockquote class="vd-looper-feedback">' + esc(data.last_feedback) + '</blockquote>' : '') +
                    '<div class="vd-looper-run-meta">' + esc(meta.join(' · ')) + '</div>' +
                    '<div class="vd-looper-timeline">' + renderTimeline(esc, t, formatDuration, data.logs, data.current_step, !!data.running, expandState) + '</div>') +
            '</div>';
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
            const cost = formatCost(run.cost_usd, t);
            return '<li>' +
                '<button type="button" class="vd-looper-history-item" data-run-id="' + esc(String(run.id)) + '">' +
                '<span class="vd-looper-history-name">' + esc(run.preset_name || t('desktop.looper_untitled')) + '</span>' +
                '<span class="vd-looper-status vd-looper-status--' + esc(run.status || 'idle') + '">' + esc(t('desktop.looper_status_' + (run.status || 'idle'))) + '</span>' +
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

    function renderHistoryDetail(root, run, helpers) {
        if (!root || !helpers || !run) return;
        const esc = helpers.esc;
        const t = helpers.t;
        root.innerHTML = '<div class="vd-looper-history-detail">' +
            '<button type="button" class="vd-looper-history-back">' + esc(t('desktop.looper_history_back')) + '</button>' +
            '</div>';
        const host = document.createElement('div');
        root.querySelector('.vd-looper-history-detail').appendChild(host);
        renderRun(host, {
            status: run.status,
            running: false,
            paused: false,
            round: run.rounds,
            max_rounds: run.max_rounds,
            score_history: (run.logs || []).filter(l => l.step === 'evaluate').map(l => l.score).filter(n => Number.isFinite(Number(n))),
            best_score: run.best_score,
            last_feedback: (run.logs || []).reduce((acc, log) => log.feedback || acc, ''),
            error: run.error,
            logs: run.logs || [],
            input_tokens: run.input_tokens,
            output_tokens: run.output_tokens,
            estimated_cost_usd: run.cost_usd
        }, helpers);
    }

    window.LooperMonitor = {
        sparklineSVG: sparklineSVG,
        renderRun: renderRun,
        renderHistoryList: renderHistoryList,
        renderHistoryDetail: renderHistoryDetail,
        logKey: logKey
    };
})();
