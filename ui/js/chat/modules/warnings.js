// warnings.js — System Warnings UI module
// Loads warnings from /api/warnings, renders the modal, and listens for SSE updates.

(function () {
    'use strict';

    const btn = document.getElementById('warnings-btn');
    const badge = document.getElementById('warnings-badge');
    const overlay = document.getElementById('warnings-overlay');
    const listEl = document.getElementById('warnings-list');
    const closeBtn = document.getElementById('warnings-close');
    const ackAllBtn = document.getElementById('warnings-ack-all');

    if (!btn || !overlay) return;

    let warningsData = [];
    let blinkInterval = null;

    // ── Severity helpers ──────────────────────────────────────
    function severityIcon(sev) {
        switch (sev) {
            case 'critical': return '🔴';
            case 'warning':  return '🟡';
            default:         return 'ℹ️';
        }
    }

    function severityOrder(sev) {
        switch (sev) {
            case 'critical': return 0;
            case 'warning':  return 1;
            default:         return 2;
        }
    }

    // ── Badge / blink ─────────────────────────────────────────
    function updateBadge(unack) {
        if (unack > 0) {
            btn.classList.remove('is-hidden');
            badge.textContent = unack;
            badge.classList.remove('is-hidden');
            startBlink();
        } else {
            badge.classList.add('is-hidden');
            stopBlink();
            // Still show button if there are any warnings at all
            if (warningsData.length === 0) {
                btn.classList.add('is-hidden');
            }
        }
    }

    function startBlink() {
        if (blinkInterval) return;
        blinkInterval = setInterval(function () {
            btn.classList.toggle('warnings-blink');
        }, 800);
    }

    function stopBlink() {
        if (blinkInterval) {
            clearInterval(blinkInterval);
            blinkInterval = null;
        }
        btn.classList.remove('warnings-blink');
    }

    // ── Render ────────────────────────────────────────────────
    function renderWarnings() {
        listEl.innerHTML = '';
        if (warningsData.length === 0) {
            const empty = document.createElement('div');
            empty.className = 'warnings-empty';
            empty.textContent = typeof t === 'function' ? t('chat.warnings_empty') : 'No active warnings.';
            listEl.appendChild(empty);
            return;
        }

        // Sort: critical > warning > info, then by timestamp desc
        warningsData.sort(function (a, b) {
            const so = severityOrder(a.severity) - severityOrder(b.severity);
            if (so !== 0) return so;
            return new Date(b.timestamp) - new Date(a.timestamp);
        });

        warningsData.forEach(function (w) {
            const item = document.createElement('div');
            item.className = 'warnings-item' + (w.acknowledged ? ' acknowledged' : '');
            item.dataset.id = w.id;

            const header = document.createElement('div');
            header.className = 'warnings-item-header';

            const left = document.createElement('span');
            left.innerHTML = severityIcon(w.severity) + ' <strong>' + escapeHtml(w.title) + '</strong>';

            const right = document.createElement('span');
            right.className = 'warnings-item-meta';
            const catLabel = typeof t === 'function' ? t('chat.warnings_cat_' + w.category) || w.category : w.category;
            right.textContent = catLabel;
            if (w.acknowledged) {
                right.textContent += ' ✓';
            }

            header.appendChild(left);
            header.appendChild(right);
            item.appendChild(header);

            const desc = document.createElement('div');
            desc.className = 'warnings-item-desc';
            desc.textContent = w.description;
            item.appendChild(desc);

            if (!w.acknowledged) {
                const ackBtn = document.createElement('button');
                ackBtn.className = 'btn-small warnings-ack-btn';
                ackBtn.textContent = typeof t === 'function' ? t('chat.warnings_acknowledge') : 'Acknowledge';
                ackBtn.addEventListener('click', function () {
                    acknowledgeWarning(w.id);
                });
                item.appendChild(ackBtn);
            }

            listEl.appendChild(item);
        });
    }

    function escapeHtml(str) {
        const d = document.createElement('div');
        d.textContent = str;
        return d.innerHTML;
    }

    // ── API calls ─────────────────────────────────────────────
    function loadWarnings() {
        fetch('/api/warnings', { credentials: 'same-origin' })
            .then(function (r) { return r.json(); })
            .then(function (data) {
                warningsData = data.warnings || [];
                updateBadge(data.unacknowledged || 0);
                if (overlay.classList.contains('active')) {
                    renderWarnings();
                }
            })
            .catch(function (err) {
                console.warn('[Warnings] fetch failed:', err);
            });
    }

    function acknowledgeWarning(id) {
        fetch('/api/warnings/acknowledge', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        })
            .then(function () { loadWarnings(); })
            .catch(function (err) { console.warn('[Warnings] ack failed:', err); });
    }

    function acknowledgeAll() {
        fetch('/api/warnings/acknowledge', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ all: true })
        })
            .then(function () { loadWarnings(); })
            .catch(function (err) { console.warn('[Warnings] ack-all failed:', err); });
    }

    // ── Modal toggle ──────────────────────────────────────────
    function openWarnings() {
        renderWarnings();
        overlay.classList.add('active');
    }

    function closeWarnings() {
        overlay.classList.remove('active');
    }

    btn.addEventListener('click', function () {
        if (overlay.classList.contains('active')) {
            closeWarnings();
        } else {
            loadWarnings();
            openWarnings();
        }
    });

    closeBtn.addEventListener('click', closeWarnings);
    ackAllBtn.addEventListener('click', acknowledgeAll);

    overlay.addEventListener('click', function (e) {
        if (e.target === overlay) closeWarnings();
    });

    // ── SSE listener ──────────────────────────────────────────
    if (typeof AuraSSE !== 'undefined') {
        AuraSSE.on('system_warning', function (payload) {
            // Reload full list to stay in sync
            loadWarnings();
        });
    }

    // ── Initial load ──────────────────────────────────────────
    loadWarnings();
})();
