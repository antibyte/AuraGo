(function () {
    'use strict';

    const SHORTCUT_HELP_KEY = '?';

    function mount(state, slot) {
        if (!slot) return;
        const t = state.t;
        const esc = state.esc;
        const readonly = state.readonly;

        const buttons = [
            { id: 'bold', label: t('cheater.toolbar.bold'), icon: 'B', shortcut: 'Ctrl+B', title: t('cheater.toolbar.bold') },
            { id: 'italic', label: t('cheater.toolbar.italic'), icon: 'I', shortcut: 'Ctrl+I', title: t('cheater.toolbar.italic') },
            { id: 'code', label: t('cheater.toolbar.code'), icon: '</>', shortcut: 'Ctrl+Shift+C', title: t('cheater.toolbar.code') },
            { id: 'inline-code', label: t('cheater.toolbar.inline_code'), icon: '`', shortcut: '', title: t('cheater.toolbar.inline_code') },
            { id: 'link', label: t('cheater.toolbar.link'), icon: t('cheater.toolbar.link_icon'), shortcut: 'Ctrl+K', title: t('cheater.toolbar.link') },
            { id: 'heading', label: t('cheater.toolbar.heading'), icon: 'H', shortcut: '', title: t('cheater.toolbar.heading') },
            { id: 'list-ul', label: t('cheater.toolbar.list'), icon: '•', shortcut: '', title: t('cheater.toolbar.list') },
            { id: 'list-ol', label: t('cheater.toolbar.ordered_list'), icon: '1.', shortcut: '', title: t('cheater.toolbar.ordered_list') },
            { id: 'quote', label: t('cheater.toolbar.quote'), icon: '“', shortcut: '', title: t('cheater.toolbar.quote') },
            { id: 'divider', label: t('cheater.toolbar.divider'), icon: '—', shortcut: '', title: t('cheater.toolbar.divider') }
        ];

        const buttonHtml = buttons.map(btn =>
            `<button type="button" class="cheater-tool-btn" data-tool="${esc(btn.id)}" title="${esc(btn.title)}${btn.shortcut ? ' (' + esc(btn.shortcut) + ')' : ''}"${readonly ? ' disabled' : ''} aria-label="${esc(btn.title)}"><span aria-hidden="true">${esc(btn.icon)}</span></button>`
        ).join('');

        slot.innerHTML = `<div class="cheater-toolbar-inner">
            <div class="cheater-toolbar-group">${buttonHtml}</div>
            <button type="button" class="cheater-tool-btn cheater-help-btn" data-tool="help" title="${esc(t('cheater.toolbar.help'))}" aria-label="${esc(t('cheater.toolbar.help'))}">?</button>
        </div>`;

        slot.querySelectorAll('[data-tool]').forEach(btn => {
            btn.addEventListener('click', () => {
                const tool = btn.dataset.tool;
                if (tool === 'help') {
                    showHelp(state);
                    return;
                }
                if (readonly) return;
                applyTool(state, tool);
            });
        });

        bindShortcuts(state);
    }

    function applyTool(state, tool) {
        const textarea = state.host.querySelector('[data-source]');
        if (!textarea) return;
        switch (tool) {
            case 'bold': wrapSelection(textarea, '**', '**'); break;
            case 'italic': wrapSelection(textarea, '_', '_'); break;
            case 'code': wrapBlock(textarea, '```'); break;
            case 'inline-code': wrapSelection(textarea, '`', '`'); break;
            case 'link': insertLink(textarea); break;
            case 'heading': toggleLinePrefix(textarea, '## '); break;
            case 'list-ul': toggleLinePrefix(textarea, '- '); break;
            case 'list-ol': toggleLinePrefix(textarea, '1. '); break;
            case 'quote': toggleLinePrefix(textarea, '> '); break;
            case 'divider': insertDivider(textarea); break;
        }
        textarea.focus();
        textarea.dispatchEvent(new Event('input'));
    }

    function wrapSelection(textarea, before, after) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const value = textarea.value;
        const selected = value.slice(start, end);
        const replacement = before + (selected || '') + after;
        textarea.setRangeText(replacement, start, end, 'end');
        if (!selected) {
            textarea.selectionStart = start + before.length;
            textarea.selectionEnd = start + before.length;
        }
    }

    function wrapBlock(textarea, fence) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const value = textarea.value;
        const before = value.lastIndexOf('\n', start - 1);
        const lineStart = before === -1 ? 0 : before + 1;
        const after = value.indexOf('\n', end);
        const lineEnd = after === -1 ? value.length : after;
        const block = value.slice(lineStart, lineEnd);
        const needsLeadingNewline = lineStart > 0 && value[lineStart - 1] !== '\n';
        const needsTrailingNewline = lineEnd < value.length && value[lineEnd] !== '\n';
        const replacement = (needsLeadingNewline ? '\n' : '') + fence + '\n' + block + '\n' + fence + (needsTrailingNewline ? '\n' : '');
        textarea.setRangeText(replacement, lineStart, lineEnd, 'end');
    }

    function insertLink(textarea) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const selected = textarea.value.slice(start, end) || '';
        const replacement = '[' + selected + '](' + ')';
        textarea.setRangeText(replacement, start, end, 'end');
        const cursor = start + replacement.length - 1;
        textarea.selectionStart = cursor;
        textarea.selectionEnd = cursor;
    }

    function toggleLinePrefix(textarea, prefix) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const value = textarea.value;
        const before = value.lastIndexOf('\n', start - 1);
        const lineStart = before === -1 ? 0 : before + 1;
        const after = value.indexOf('\n', end);
        const lineEnd = after === -1 ? value.length : after;
        const block = value.slice(lineStart, lineEnd);
        const lines = block.split('\n');
        const allHave = lines.every(line => line.startsWith(prefix));
        const transformed = lines.map(line => allHave ? line.slice(prefix.length) : prefix + line).join('\n');
        textarea.setRangeText(transformed, lineStart, lineEnd, 'end');
        textarea.selectionStart = lineStart;
        textarea.selectionEnd = lineStart + transformed.length;
    }

    function insertDivider(textarea) {
        const start = textarea.selectionStart;
        const value = textarea.value;
        const needsNewlineBefore = start > 0 && value[start - 1] !== '\n';
        const needsNewlineAfter = value[start] && value[start] !== '\n';
        const divider = (needsNewlineBefore ? '\n' : '') + '\n---\n' + (needsNewlineAfter ? '\n' : '');
        textarea.setRangeText(divider, start, start, 'end');
    }

    function bindShortcuts(state) {
        if (state._toolbarShortcutBound) return;
        state._toolbarShortcutBound = true;
        state.host.addEventListener('keydown', (e) => {
            if (state.readonly) return;
            if (!(e.ctrlKey || e.metaKey)) {
                if (e.key === SHORTCUT_HELP_KEY && !e.target.matches('input, textarea')) {
                    e.preventDefault();
                    showHelp(state);
                }
                return;
            }
            const textarea = state.host.querySelector('[data-source]');
            if (!textarea || document.activeElement !== textarea) return;
            let handled = true;
            if (e.shiftKey && (e.key === 'c' || e.key === 'C')) applyTool(state, 'code');
            else if (!e.shiftKey && (e.key === 'b' || e.key === 'B')) applyTool(state, 'bold');
            else if (!e.shiftKey && (e.key === 'i' || e.key === 'I')) applyTool(state, 'italic');
            else if (!e.shiftKey && (e.key === 'k' || e.key === 'K')) {
                if (!e.shiftKey) applyTool(state, 'link');
                else handled = false;
            } else {
                handled = false;
            }
            if (handled) {
                e.preventDefault();
            }
        });
    }

    function showHelp(state) {
        const existing = document.querySelector('.cheater-help-modal');
        if (existing) { existing.remove(); return; }
        const t = state.t;
        const esc = state.esc;
        const shortcuts = [
            { keys: 'Ctrl+S', action: t('cheater.help.save') },
            { keys: 'Ctrl+Shift+K', action: t('cheater.help.spotlight') },
            { keys: 'Ctrl+N', action: t('cheater.help.new_sheet') },
            { keys: 'Ctrl+B', action: t('cheater.toolbar.bold') },
            { keys: 'Ctrl+I', action: t('cheater.toolbar.italic') },
            { keys: 'Ctrl+Shift+C', action: t('cheater.toolbar.code') },
            { keys: 'Ctrl+K', action: t('cheater.toolbar.link') },
            { keys: 'Ctrl+Shift+E', action: t('cheater.help.cycle_view') },
            { keys: 'Tab / Shift+Tab', action: t('cheater.help.indent') },
            { keys: 'Esc', action: t('cheater.help.close') }
        ];
        const modal = document.createElement('div');
        modal.className = 'cheater-help-modal';
        modal.setAttribute('role', 'dialog');
        modal.setAttribute('aria-modal', 'true');
        modal.setAttribute('aria-label', t('cheater.toolbar.help'));
        modal.innerHTML = `<div class="cheater-help-backdrop" data-backdrop></div>
            <div class="cheater-help-panel">
                <h2>${esc(t('cheater.toolbar.help'))}</h2>
                <dl class="cheater-help-list">
                    ${shortcuts.map(s => `<div class="cheater-help-row"><dt><kbd>${esc(s.keys)}</kbd></dt><dd>${esc(s.action)}</dd></div>`).join('')}
                </dl>
                <div class="cheater-help-footer"><button type="button" class="cheater-primary" data-close>${esc(t('cheater.close'))}</button></div>
            </div>`;
        document.body.appendChild(modal);
        modal.querySelector('[data-backdrop]').addEventListener('click', () => modal.remove());
        modal.querySelector('[data-close]').addEventListener('click', () => modal.remove());
        modal.addEventListener('keydown', (e) => { if (e.key === 'Escape') modal.remove(); });
        const closeBtn = modal.querySelector('[data-close]');
        if (closeBtn) closeBtn.focus();
    }

    window.CheaterToolbar = window.CheaterToolbar || {};
    window.CheaterToolbar.mount = mount;
})();