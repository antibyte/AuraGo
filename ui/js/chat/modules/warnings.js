// warnings.js — System Warnings UI module
// Loads warnings from /api/warnings, renders the modal, and listens for SSE updates.
//
// Badge behaviour:
//   RED  + blink  — one or more warnings whose IDs have never been acknowledged
//   GRAY + no blink — all current warnings were previously acknowledged (known issues)
//   hidden          — no warnings at all
//
// Acknowledged IDs are persisted in localStorage so the state survives page reloads
// and server restarts. Only truly NEW warning IDs cause the red/blink state again.

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

    // ── localStorage persistence ──────────────────────────────
    const LS_KEY = 'aurago-warnings-acked';

    function getAckedIds() {
        try {
            return new Set(JSON.parse(localStorage.getItem(LS_KEY) || '[]'));
        } catch (e) {
            return new Set();
        }
    }

    function persistAckedId(id) {
        const set = getAckedIds();
        set.add(id);
        try { localStorage.setItem(LS_KEY, JSON.stringify([...set])); } catch (e) {}
    }

    function persistAckedIds(warnings) {
        const set = getAckedIds();
        warnings.forEach(function (w) { set.add(w.id); });
        try { localStorage.setItem(LS_KEY, JSON.stringify([...set])); } catch (e) {}
    }

    // ── Severity helpers ──────────────────────────────────────
    function severityIcon(sev) {
        const markup = window.chatUiIconMarkup || (() => '');
        switch (sev) {
            case 'critical': return markup('error');
            case 'warning':  return markup('warning');
            default:         return markup('info');
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
    // warnings: full array from server.
    function updateBadge(warnings) {
        const total = warnings.length;
        if (total === 0) {
            badge.classList.add('is-hidden');
            badge.classList.remove('seen');
            stopBlink();
            return;
        }

        const ackedIds = getAckedIds();
        // "New" = present on server AND never acknowledged by this user
        const newCount = warnings.filter(function (w) { return !ackedIds.has(w.id); }).length;

        if (newCount > 0) {
            // Genuinely new problems — alert the user
            badge.textContent = newCount;
            badge.classList.remove('is-hidden', 'seen');
            startBlink();
        } else {
            // All problems are known / previously acknowledged — show softly
            badge.textContent = total;
            badge.classList.remove('is-hidden');
            badge.classList.add('seen');
            stopBlink();
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

        const ackedIds = getAckedIds();

        // Sort: new (unacked locally) first, then critical > warning > info, then by timestamp desc
        warningsData.sort(function (a, b) {
            const aNew = !ackedIds.has(a.id);
            const bNew = !ackedIds.has(b.id);
            if (aNew !== bNew) return aNew ? -1 : 1;
            const so = severityOrder(a.severity) - severityOrder(b.severity);
            if (so !== 0) return so;
            return new Date(b.timestamp) - new Date(a.timestamp);
        });

        warningsData.forEach(function (w) {
            const isLocallyAcked = ackedIds.has(w.id);
            const item = document.createElement('div');
            item.className = 'warnings-item' + (isLocallyAcked ? ' acknowledged' : '');
            item.dataset.id = w.id;

            const header = document.createElement('div');
            header.className = 'warnings-item-header';

            const left = document.createElement('span');
            left.innerHTML = severityIcon(w.severity) + ' <strong>' + escapeHtml(w.title) + '</strong>';

            const right = document.createElement('span');
            right.className = 'warnings-item-meta';
            const catLabel = typeof t === 'function' ? t('chat.warnings_cat_' + w.category) || w.category : w.category;
            right.textContent = catLabel;
            if (isLocallyAcked) {
                right.insertAdjacentHTML('beforeend', ' ' + (window.chatUiIconMarkup ? window.chatUiIconMarkup('complete') : ''));
            }

            header.appendChild(left);
            header.appendChild(right);
            item.appendChild(header);

            const desc = document.createElement('div');
            desc.className = 'warnings-item-desc';
            desc.textContent = w.description;
            item.appendChild(desc);

            if (!isLocallyAcked) {
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
                updateBadge(warningsData);
                if (overlay.classList.contains('active')) {
                    renderWarnings();
                }
            })
            .catch(function (err) {
                console.warn('[Warnings] fetch failed:', err);
            });
    }

    function acknowledgeWarning(id) {
        // Persist locally first so the badge updates instantly even if the server call is slow
        persistAckedId(id);
        updateBadge(warningsData);
        renderWarnings();

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
        // Persist all current IDs locally
        persistAckedIds(warningsData);
        updateBadge(warningsData);
        renderWarnings();

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
