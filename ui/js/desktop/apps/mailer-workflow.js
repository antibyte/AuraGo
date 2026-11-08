// Mailer workflow: keyboard shortcuts, list density, busy feedback and the
// snooze dialog.
//
// The module loads before mailer.js and receives the shell's own
// capability-checked actions as dependencies. It never renders the window and
// owns no lifecycle: all state lives on the shell's instance object. Keeping
// these helpers here keeps mailer.js inside the Desktop JS line budget.
(function () {
    'use strict';

    function create(deps) {
        const {
            t, esc, clamp, savePrefs, localPrefs, localDateTime,
            openCompose, runOneAction, openThread, selectedReadOnly,
            showError, setStatus, refreshCurrent
        } = deps;

        // List density is a per-desktop preference, not a server setting. The
        // compact view only tightens spacing; it never drops message metadata.
        function densityLabel(inst) {
            return t(inst.ctx, 'density_toggle', { mode: t(inst.ctx, inst.density === 'compact' ? 'density_comfortable' : 'density_compact') });
        }

        function syncDensity(inst) {
            inst.mailerRoot.dataset.density = inst.density;
            const button = inst.host.querySelector('[data-density-toggle]');
            if (!button) return;
            const label = densityLabel(inst);
            button.setAttribute('aria-pressed', String(inst.density === 'compact'));
            button.title = label;
            button.setAttribute('aria-label', label);
        }

        function toggleDensity(inst) {
            inst.density = inst.density === 'compact' ? 'comfortable' : 'compact';
            savePrefs(Object.assign(localPrefs(), { density: inst.density }));
            syncDensity(inst);
        }

        function setBusy(inst, busy) {
            const workspace = inst.host.querySelector('[data-mailer-workspace]');
            if (workspace) workspace.setAttribute('aria-busy', String(!!busy));
            const refresh = inst.host.querySelector('[data-refresh]');
            if (refresh) {
                refresh.setAttribute('aria-busy', String(!!busy));
                refresh.disabled = !!busy;
            }
        }

        // Keyboard shortcuts only act outside text inputs, selects, settings and
        // the composer. Every action reuses the existing capability checks, so a
        // read-only account can never be changed by a stray keypress.
        function shortcutRows(inst) {
            const root = inst.host.querySelector('[data-thread-list]');
            if (!root || !root.offsetParent) return [];
            return Array.from(root.querySelectorAll('[data-thread-id]'));
        }

        function shortcutRow(inst) {
            const rows = shortcutRows(inst);
            if (!rows.length) return null;
            const active = rows.indexOf(document.activeElement);
            return active >= 0 ? rows[active] : rows[clamp(inst.keyboardIndex, 0, rows.length - 1)] || null;
        }

        function moveListCursor(inst, delta) {
            const rows = shortcutRows(inst);
            if (!rows.length) return false;
            const active = rows.indexOf(document.activeElement);
            const next = active < 0 ? (delta > 0 ? 0 : rows.length - 1) : clamp(active + delta, 0, rows.length - 1);
            rows[next].focus();
            inst.keyboardIndex = next;
            return true;
        }

        function shortcutKey(inst, event) {
            if (event.ctrlKey || event.metaKey || event.altKey || event.isComposing || !inst.state) return;
            if (!inst.host.querySelector('[data-shortcuts]').hidden) return;
            if (event.target.closest('input, textarea, select, [contenteditable="true"], button, a, [data-settings], .vd-mailer-compose')) return;
            const key = event.key;
            if (key === '?') { event.preventDefault(); openShortcuts(inst); return; }
            if (key === '/') {
                event.preventDefault();
                const search = inst.host.querySelector('[data-search]');
                search.focus(); search.select();
                return;
            }
            if (key === 'c') { event.preventDefault(); openCompose(inst); return; }
            if (key === 'j' || key === 'k' || key === 'ArrowDown' || key === 'ArrowUp') {
                if (!moveListCursor(inst, key === 'j' || key === 'ArrowDown' ? 1 : -1)) return;
                event.preventDefault();
                return;
            }
            if (['s', 'e', 'Enter'].includes(key)) {
                const row = shortcutRow(inst);
                if (!row) return;
                event.preventDefault();
                const threadId = row.dataset.threadId;
                if (key === 'Enter') { openThread(inst, threadId); return; }
                const item = inst.threads.find(entry => entry.id === threadId) || {};
                inst.pendingFocusThread = threadId;
                runOneAction(inst, threadId, key === 's' ? (item.starred ? 'unstar' : 'star') : 'archive');
            }
        }

        function shortcutEscape(inst) {
            const host = inst.host;
            const shortcuts = host.querySelector('[data-shortcuts]');
            if (!shortcuts.hidden) { closeShortcuts(inst); return true; }
            if (host.querySelector('[data-snooze-time]')) { closeSnooze(inst); return true; }
            const settings = host.querySelector('[data-settings]');
            if (!settings.hidden) { settings.hidden = true; return true; }
            const back = host.querySelector('[data-reader-back]');
            if (inst.activeThread && back && back.offsetParent) { back.click(); return true; }
            return false;
        }

        function shortcutsMarkup(inst) {
            const rows = [
                ['J / K', 'shortcuts_move'],
                ['Enter', 'shortcuts_open'],
                ['S', 'shortcuts_star'],
                ['E', 'archive'],
                ['C', 'compose'],
                ['/', 'search'],
                ['?', 'shortcuts_title'],
                ['Esc', 'shortcuts_close'],
                ['Ctrl / ⌘ + F', 'search']
            ];
            return `<section class="vd-mailer-shortcuts-card" role="dialog" aria-modal="true" aria-label="${esc(t(inst.ctx, 'shortcuts_title'))}">
            <header class="vd-mailer-shortcuts-head"><h2>${esc(t(inst.ctx, 'shortcuts_title'))}</h2><button type="button" data-close-shortcuts aria-label="${esc(t(inst.ctx, 'close'))}">×</button></header>
            <p class="vd-mailer-shortcuts-hint">${esc(t(inst.ctx, 'shortcuts_hint'))}</p>
            <dl class="vd-mailer-shortcuts-list">${rows.map(([keys, label]) => `<dt><kbd>${esc(keys)}</kbd></dt><dd>${esc(t(inst.ctx, label))}</dd>`).join('')}</dl>
        </section>`;
        }

        function openShortcuts(inst) {
            const overlay = inst.host.querySelector('[data-shortcuts]');
            if (!overlay || !overlay.hidden) return;
            inst.shortcutReturnFocus = document.activeElement;
            overlay.innerHTML = shortcutsMarkup(inst);
            overlay.hidden = false;
            overlay.querySelector('[data-close-shortcuts]')?.focus();
        }

        function closeShortcuts(inst) {
            const overlay = inst.host.querySelector('[data-shortcuts]');
            if (!overlay || overlay.hidden) return;
            overlay.hidden = true;
            overlay.innerHTML = '';
            const previous = inst.shortcutReturnFocus;
            inst.shortcutReturnFocus = null;
            if (previous && previous.isConnected && typeof previous.focus === 'function') previous.focus();
        }

        // Presets are computed at click time, not when the dialog opens, so a
        // long-open dialog can never snooze a message into the past.
        function snoozePresetDate(kind) {
            const now = new Date();
            if (kind === 'later') return new Date(now.getTime() + 3 * 3600000);
            const date = new Date(now);
            date.setHours(9, 0, 0, 0);
            date.setDate(date.getDate() + (kind === 'week' ? 7 : 1));
            return date;
        }

        function openSnooze(inst) {
            if (inst.state.readonly || !inst.selected.size || selectedReadOnly(inst)) return;
            const until = new Date(Date.now() + 86400000);
            const presets = [
                ['later', 'snooze_preset_later'],
                ['tomorrow', 'snooze_preset_tomorrow'],
                ['week', 'snooze_preset_week']
            ];
            inst.host.querySelector('[data-status]').innerHTML = `<span class="vd-mailer-snooze-dialog" role="group" aria-label="${esc(t(inst.ctx, 'snooze'))}"><span class="vd-mailer-snooze-presets">${presets.map(([kind, key]) => `<button type="button" class="vd-mailer-button" data-snooze-preset="${kind}">${esc(t(inst.ctx, key))}</button>`).join('')}</span><label>${esc(t(inst.ctx, 'snooze_until'))}<input type="datetime-local" data-snooze-time value="${localDateTime(until)}"></label><button type="button" class="vd-mailer-button" data-snooze-submit>${esc(t(inst.ctx, 'snooze'))}</button><button type="button" class="vd-mailer-button" data-close-snooze>${esc(t(inst.ctx, 'cancel'))}</button></span>`;
            inst.host.querySelector('[data-snooze-time]').focus();
        }

        function closeSnooze(inst) { inst.host.querySelector('[data-status]').textContent = ''; }

        async function submitSnooze(inst, presetValue) {
            if (inst.snoozing) return;
            const until = presetValue || inst.host.querySelector('[data-snooze-time]')?.value;
            const target = new Date(until);
            if (!until || !(target > new Date())) {
                // Keep the picker open: an invalid time is a correction, not a
                // reason to throw away the whole dialog.
                const dialog = inst.host.querySelector('.vd-mailer-snooze-dialog');
                let notice = inst.host.querySelector('[data-snooze-error]');
                if (!notice && dialog) {
                    notice = document.createElement('span');
                    notice.className = 'vd-mailer-snooze-error';
                    notice.dataset.snoozeError = '';
                    notice.setAttribute('role', 'alert');
                    dialog.append(notice);
                }
                if (notice) notice.textContent = t(inst.ctx, 'snooze_future_required');
                return;
            }
            const ids = Array.from(inst.selected);
            inst.snoozing = true;
            try {
                for (const threadId of ids) {
                    const detail = await inst.api.thread(threadId);
                    for (const message of detail.messages || []) await inst.api.snooze(message.id, target.toISOString());
                }
                inst.selected.clear(); closeSnooze(inst); await refreshCurrent(inst, true);
                setStatus(inst, t(inst.ctx, 'snoozed'));
            } catch (error) { showError(inst, error); }
            finally { inst.snoozing = false; }
        }

        return {
            densityLabel, syncDensity, toggleDensity, setBusy,
            shortcutKey, shortcutEscape, openShortcuts, closeShortcuts,
            snoozePresetDate, openSnooze, closeSnooze, submitSnooze
        };
    }

    window.MailerWorkflow = { create };
})();
