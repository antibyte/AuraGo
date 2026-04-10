/* AuraGo – File Sync Status (Knowledge Center Tab) */
/* global t, showToast, switchKCTab */

// ═══════════════════════════════════════════════════════════════
// FILE SYNC STATUS
// ═══════════════════════════════════════════════════════════════

/**
 * Formats an ISO timestamp to a locale-friendly relative or absolute string.
 * @param {string|null} iso - ISO 8601 timestamp
 * @returns {string} Formatted date or placeholder
 */
function fsFormatTime(iso) {
    if (!iso) return '—';
    try {
        const d = new Date(iso);
        if (isNaN(d.getTime())) return '—';
        const now = new Date();
        const diffMs = now - d;
        const diffMin = Math.floor(diffMs / 60000);
        if (diffMin < 1) return t('knowledge.filesync_just_now') || 'just now';
        if (diffMin < 60) return (t('knowledge.filesync_minutes_ago') || '{n} min ago').replace('{n}', diffMin);
        const diffH = Math.floor(diffMin / 60);
        if (diffH < 24) return (t('knowledge.filesync_hours_ago') || '{n}h ago').replace('{n}', diffH);
        return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    } catch (_) {
        return '—';
    }
}

/**
 * Renders a stat row: label + value.
 */
function fsStatRow(label, value, icon) {
    return `<div class="kc-sync-stat">
        ${icon ? `<span class="kc-sync-stat-icon">${icon}</span>` : ''}
        <span class="kc-sync-stat-label">${label}</span>
        <span class="kc-sync-stat-value">${value}</span>
    </div>`;
}

/**
 * Loads and renders the full file sync operative status.
 */
async function loadFileSyncStatus() {
    const indexerBody = document.getElementById('fs-indexer-body');
    const kgBody = document.getElementById('fs-kg-body');
    const collBody = document.getElementById('fs-collections-body');
    const lastSyncBody = document.getElementById('fs-lastsync-body');
    const indexerDot = document.getElementById('fs-indexer-dot');
    const indexerStatusLabel = document.getElementById('fs-indexer-status');

    // Show loading state
    const loadingHtml = `<div class="kc-sync-loading">${t('knowledge.filesync_loading') || 'Loading status…'}</div>`;
    if (indexerBody) indexerBody.innerHTML = loadingHtml;
    if (kgBody) kgBody.innerHTML = loadingHtml;
    if (collBody) collBody.innerHTML = loadingHtml;
    if (lastSyncBody) lastSyncBody.innerHTML = loadingHtml;

    // Fetch all data in parallel
    const [syncStatus, kgStats, lastRun] = await Promise.allSettled([
        fetch('/api/debug/file-sync-status').then(r => r.ok ? r.json() : null).catch(() => null),
        fetch('/api/debug/kg-file-sync-stats').then(r => r.ok ? r.json() : null).catch(() => null),
        fetch('/api/debug/file-sync-last-run').then(r => r.ok ? r.json() : null).catch(() => null),
    ]);

    const status = syncStatus.status === 'fulfilled' ? syncStatus.value : null;
    const kg = kgStats.status === 'fulfilled' ? kgStats.value : null;
    const last = lastRun.status === 'fulfilled' ? lastRun.value : null;

    // ── Indexer Status ──
    if (indexerBody && status) {
        const idx = status.indexer || {};
        const isRunning = !!idx.running;
        if (indexerDot) indexerDot.textContent = isRunning ? '🟢' : '🔴';
        if (indexerStatusLabel) {
            indexerStatusLabel.textContent = isRunning
                ? (t('knowledge.filesync_running') || 'Running')
                : (t('knowledge.filesync_stopped') || 'Stopped');
            indexerStatusLabel.className = 'kc-sync-status-label ' + (isRunning ? 'kc-sync-ok' : 'kc-sync-err');
        }

        let html = '';
        html += fsStatRow(t('knowledge.filesync_total_files') || 'Total Files', idx.total_files ?? '—', '📄');
        html += fsStatRow(t('knowledge.filesync_indexed_files') || 'Indexed Files', idx.indexed_files ?? '—', '🔗');
        html += fsStatRow(t('knowledge.filesync_last_scan') || 'Last Scan', fsFormatTime(idx.last_scan_at), '⏱️');
        if (idx.last_scan_duration) {
            html += fsStatRow(t('knowledge.filesync_scan_duration') || 'Scan Duration', idx.last_scan_duration, '⏳');
        }
        if (idx.directories && idx.directories.length > 0) {
            html += `<div class="kc-sync-stat kc-sync-stat-full">
                <span class="kc-sync-stat-label">📂 ${t('knowledge.filesync_directories') || 'Directories'}</span>
            </div>`;
            idx.directories.forEach(dir => {
                html += `<div class="kc-sync-dir-item"><span class="kc-sync-dir-path">${escapeHtml(dir)}</span></div>`;
            });
        }
        if (idx.errors && idx.errors.length > 0) {
            html += `<div class="kc-sync-errors">
                <span class="kc-sync-errors-title">⚠️ ${idx.errors.length} ${t('knowledge.filesync_errors') || 'Error(s)'}</span>
                <ul class="kc-sync-errors-list">${idx.errors.map(e => `<li>${escapeHtml(String(e))}</li>`).join('')}</ul>
            </div>`;
        }
        indexerBody.innerHTML = html;
    } else if (indexerBody) {
        if (indexerDot) indexerDot.textContent = '⚫';
        if (indexerStatusLabel) {
            indexerStatusLabel.textContent = t('knowledge.filesync_unavailable') || 'Unavailable';
            indexerStatusLabel.className = 'kc-sync-status-label kc-sync-warn';
        }
        indexerBody.innerHTML = `<div class="kc-sync-na">${t('knowledge.filesync_no_data') || 'No indexer data available.'}</div>`;
    }

    // ── KG Stats ──
    if (kgBody && kg) {
        let html = '';
        html += fsStatRow(t('knowledge.filesync_kg_nodes') || 'Nodes (file_sync)', kg.node_count ?? 0, '🔵');
        html += fsStatRow(t('knowledge.filesync_kg_edges') || 'Edges (file_sync)', kg.edge_count ?? 0, '🔗');

        if (kg.by_entity_type && Object.keys(kg.by_entity_type).length > 0) {
            html += `<div class="kc-sync-stat kc-sync-stat-full">
                <span class="kc-sync-stat-label">${t('knowledge.filesync_entity_types') || 'Entity Types'}</span>
            </div>`;
            html += '<div class="kc-sync-chip-row">';
            for (const [etype, count] of Object.entries(kg.by_entity_type)) {
                html += `<span class="kc-sync-chip">${escapeHtml(etype)} <strong>${count}</strong></span>`;
            }
            html += '</div>';
        }

        if (kg.by_collection && Object.keys(kg.by_collection).length > 0) {
            html += `<div class="kc-sync-stat kc-sync-stat-full">
                <span class="kc-sync-stat-label">${t('knowledge.filesync_by_collection') || 'By Collection'}</span>
            </div>`;
            html += '<div class="kc-sync-chip-row">';
            for (const [coll, count] of Object.entries(kg.by_collection)) {
                html += `<span class="kc-sync-chip kc-sync-chip-coll">${escapeHtml(coll)} <strong>${count}</strong></span>`;
            }
            html += '</div>';
        }
        kgBody.innerHTML = html;
    } else if (kgBody) {
        kgBody.innerHTML = `<div class="kc-sync-na">${t('knowledge.filesync_kg_unavailable') || 'Knowledge graph not available.'}</div>`;
    }

    // ── Collections Table ──
    if (collBody && status && status.collections && status.collections.length > 0) {
        let html = '<div class="kc-table-wrap"><table class="kc-table kc-sync-table"><thead><tr>';
        html += `<th>${t('knowledge.filesync_col_collection') || 'Collection'}</th>`;
        html += `<th>${t('knowledge.filesync_col_files') || 'Files'}</th>`;
        html += `<th>${t('knowledge.filesync_col_nodes') || 'Nodes'}</th>`;
        html += `<th>${t('knowledge.filesync_col_edges') || 'Edges'}</th>`;
        html += `<th>${t('knowledge.filesync_col_last_sync') || 'Last Sync'}</th>`;
        html += '</tr></thead><tbody>';
        status.collections.forEach(c => {
            html += `<tr>
                <td class="kc-name">${escapeHtml(c.collection || '—')}</td>
                <td>${c.file_count ?? '—'}</td>
                <td>${c.node_count ?? '—'}</td>
                <td>${c.edge_count ?? '—'}</td>
                <td>${fsFormatTime(c.last_sync_at)}</td>
            </tr>`;
        });
        html += '</tbody></table></div>';
        collBody.innerHTML = html;
    } else if (collBody) {
        collBody.innerHTML = `<div class="kc-sync-na">${t('knowledge.filesync_no_collections') || 'No collections synchronized yet.'}</div>`;
    }

    // ── Last Sync ──
    if (lastSyncBody && last) {
        let html = '';
        html += fsStatRow(t('knowledge.filesync_global_last_sync') || 'Global Last Sync', fsFormatTime(last.global), '🌐');
        if (last.per_collection && last.per_collection.length > 0) {
            last.per_collection.forEach(c => {
                html += fsStatRow(c.collection, fsFormatTime(c.last_sync_at), '📂');
            });
        }
        lastSyncBody.innerHTML = html;
    } else if (lastSyncBody) {
        lastSyncBody.innerHTML = `<div class="kc-sync-na">${t('knowledge.filesync_no_sync_data') || 'No sync data available.'}</div>`;
    }
}

/**
 * Triggers a manual rescan of the file indexer.
 */
async function triggerFileSyncRescan() {
    const btn = document.getElementById('btn-fs-rescan');
    if (btn) {
        btn.disabled = true;
        btn.querySelector('span').textContent = t('knowledge.filesync_rescanning') || 'Scanning…';
    }
    try {
        const resp = await fetch('/api/indexing/rescan', { method: 'POST' });
        if (resp.ok) {
            showToast(t('knowledge.filesync_rescan_started') || 'Rescan triggered successfully.', 'success');
            // Reload status after a delay to allow scan to start
            setTimeout(() => loadFileSyncStatus(), 2000);
        } else {
            const text = await resp.text();
            showToast(text || t('common.error') || 'Error', 'error');
        }
    } catch (e) {
        showToast((t('common.error') || 'Error') + ': ' + e.message, 'error');
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.querySelector('span').textContent = t('knowledge.filesync_rescan') || 'Rescan';
        }
    }
}

/**
 * Simple HTML escaping utility.
 */
function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// ── Hook into tab switching ──
// Patch switchKCTab to auto-load data when the filesync tab is activated
const _origSwitchKCTab = typeof switchKCTab === 'function' ? switchKCTab : null;
if (_origSwitchKCTab) {
    window.switchKCTab = function (tab) {
        _origSwitchKCTab(tab);
        if (tab === 'filesync') {
            loadFileSyncStatus();
        }
    };
}
