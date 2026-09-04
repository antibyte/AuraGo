(function () {
    'use strict';

    const MAX_TAGS = 8;

    function mount(state, slot) {
        if (!slot || !state || !state.host) return;
        const t = state.t;
        const esc = state.esc;

        const buttons = [
            { id: 'bold', icon: 'B', label: t('desktop.notes_toolbar_bold'), shortcut: 'Ctrl+B' },
            { id: 'italic', icon: 'I', label: t('desktop.notes_toolbar_italic'), shortcut: 'Ctrl+I' },
            { id: 'inline-code', icon: '`', label: t('desktop.notes_toolbar_inline_code') },
            { id: 'code', icon: '</>', label: t('desktop.notes_toolbar_code') },
            { id: 'link', icon: '🔗', label: t('desktop.notes_toolbar_link'), shortcut: 'Ctrl+K' },
            { id: 'heading', icon: 'H', label: t('desktop.notes_toolbar_heading') },
            { id: 'list-ul', icon: '•', label: t('desktop.notes_toolbar_list') },
            { id: 'list-ol', icon: '1.', label: t('desktop.notes_toolbar_ordered_list') },
            { id: 'quote', icon: '“', label: t('desktop.notes_toolbar_quote') },
            { id: 'divider', icon: '—', label: t('desktop.notes_toolbar_divider') },
            { id: 'table', icon: '▦', label: t('desktop.notes_toolbar_table') }
        ];

        const buttonHtml = buttons.map(btn =>
            `<button type="button" class="vd-notes-tool-btn" data-tool="${esc(btn.id)}" title="${esc(btn.label)}${btn.shortcut ? ' (' + esc(btn.shortcut) + ')' : ''}" aria-label="${esc(btn.label)}"${state.readonly ? ' disabled' : ''}><span aria-hidden="true">${esc(btn.icon)}</span></button>`
        ).join('');

        slot.innerHTML = `<div class="vd-notes-toolbar-inner">
            <div class="vd-notes-tool-group">${buttonHtml}</div>
            <button type="button" class="vd-notes-tool-btn vd-notes-tags-btn" data-tool="tags" title="${esc(t('desktop.notes_toolbar_tags'))}" aria-label="${esc(t('desktop.notes_toolbar_tags'))}" aria-expanded="false"${state.readonly ? ' disabled' : ''}><span aria-hidden="true">#</span></button>
            <div class="vd-notes-tags-popover" data-tags-popover hidden></div>
        </div>`;

        const popover = slot.querySelector('[data-tags-popover]');

        slot.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-tool]');
            if (!btn) return;
            const tool = btn.dataset.tool;
            if (tool === 'tags') {
                togglePopover(state, popover, btn);
                return;
            }
            if (state.readonly) return;
            hidePopover(state, popover);
            applyTool(state, tool);
        });

        const onDocPointerDown = (e) => {
            if (popover.hidden) return;
            if (slot.contains(e.target)) return;
            hidePopover(state, popover);
        };
        document.addEventListener('pointerdown', onDocPointerDown);
        if (typeof state.addCleanup === 'function') state.addCleanup(() => document.removeEventListener('pointerdown', onDocPointerDown));

        state.hideTagsPopover = () => hidePopover(state, popover);
        state.refreshTagsPopover = () => {
            if (popover && !popover.hidden) renderPopover(state, popover);
        };
    }

    function togglePopover(state, popover, btn) {
        if (!popover) return;
        if (popover.hidden) {
            renderPopover(state, popover);
            popover.hidden = false;
            if (btn) btn.setAttribute('aria-expanded', 'true');
            const input = popover.querySelector('[data-tags-input]');
            if (input) input.focus();
        } else {
            hidePopover(state, popover);
        }
    }

    function hidePopover(state, popover) {
        if (!popover) return;
        popover.hidden = true;
        const host = state.host;
        if (host) {
            const btn = host.querySelector('.vd-notes-tags-btn');
            if (btn) btn.setAttribute('aria-expanded', 'false');
        }
    }

    function renderPopover(state, popover) {
        const t = state.t;
        const esc = state.esc;
        const tags = (state.getTags && state.getTags()) || [];
        popover.innerHTML = `
            <div class="vd-notes-tags-chips" data-tags-chips>${tags.map(tag =>
                `<span class="vd-notes-pill">${esc(tag)}<button type="button" class="vd-notes-pill-remove" data-remove-tag="${esc(tag)}" aria-label="${esc(t('desktop.notes_tag_remove', { tag }))}">×</button></span>`
            ).join('')}</div>
            <div class="vd-notes-tags-input-row">
                <input type="text" data-tags-input maxlength="32" placeholder="${esc(t('desktop.notes_tag_placeholder'))}" autocomplete="off"${tags.length >= MAX_TAGS ? ' disabled' : ''}>
                <button type="button" class="vd-notes-tool-btn" data-add-tag title="${esc(t('desktop.notes_tag_add'))}" aria-label="${esc(t('desktop.notes_tag_add'))}"${tags.length >= MAX_TAGS ? ' disabled' : ''}>+</button>
            </div>`;

        const input = popover.querySelector('[data-tags-input]');
        const add = () => {
            const value = (input.value || '').trim().toLowerCase();
            if (!value) return;
            addTag(state, value);
            input.value = '';
            input.focus();
        };

        popover.querySelectorAll('[data-remove-tag]').forEach(btn => {
            btn.addEventListener('click', () => {
                const current = (state.getTags && state.getTags()) || [];
                state.setTags(current.filter(tag => tag !== btn.dataset.removeTag));
                renderPopover(state, popover);
                const next = popover.querySelector('[data-tags-input]');
                if (next) next.focus();
            });
        });
        const addBtn = popover.querySelector('[data-add-tag]');
        if (addBtn) addBtn.addEventListener('click', add);
        if (input) {
            input.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.key === ',') {
                    e.preventDefault();
                    add();
                }
            });
        }
    }

    function addTag(state, value) {
        const current = (state.getTags && state.getTags()) || [];
        const tag = String(value || '').trim().toLowerCase().replace(/\s+/g, '-').slice(0, 32);
        if (!tag || current.includes(tag) || current.length >= MAX_TAGS) return;
        state.setTags(current.concat(tag));
        state.refreshTagsPopover();
    }

    function applyTool(state, tool) {
        const textarea = state.host.querySelector('[data-notes-source]');
        if (!textarea) return;
        switch (tool) {
            case 'bold': wrapSelection(textarea, '**', '**'); break;
            case 'italic': wrapSelection(textarea, '*', '*'); break;
            case 'inline-code': wrapSelection(textarea, '`', '`'); break;
            case 'code': wrapBlock(textarea, '```'); break;
            case 'link': insertLink(textarea); break;
            case 'heading': toggleLinePrefix(textarea, '## '); break;
            case 'list-ul': toggleLinePrefix(textarea, '- '); break;
            case 'list-ol': toggleLinePrefix(textarea, '1. '); break;
            case 'quote': toggleLinePrefix(textarea, '> '); break;
            case 'divider': insertDivider(textarea); break;
            case 'table': insertTable(textarea); break;
        }
        textarea.focus();
        textarea.dispatchEvent(new Event('input'));
    }

    function wrapSelection(textarea, before, after) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const selected = textarea.value.slice(start, end);
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

    function insertTable(textarea) {
        const start = textarea.selectionStart;
        const value = textarea.value;
        const needsNewlineBefore = start > 0 && value[start - 1] !== '\n';
        const needsNewlineAfter = value[start] && value[start] !== '\n';
        const table = (needsNewlineBefore ? '\n' : '') +
            '\n| # | # |\n| --- | --- |\n|  |  |\n' +
            (needsNewlineAfter ? '\n' : '');
        textarea.setRangeText(table, start, start, 'end');
        const cursor = start + table.length - 1;
        textarea.selectionStart = cursor;
        textarea.selectionEnd = cursor;
    }

    function bindShortcuts(state) {
        const textarea = state.host.querySelector('[data-notes-source]');
        if (!textarea) return;
        textarea.addEventListener('keydown', (e) => {
            if (state.readonly) return;
            if (!(e.ctrlKey || e.metaKey)) return;
            let handled = true;
            if (!e.shiftKey && (e.key === 'b' || e.key === 'B')) applyTool(state, 'bold');
            else if (!e.shiftKey && (e.key === 'i' || e.key === 'I')) applyTool(state, 'italic');
            else if (!e.shiftKey && (e.key === 'k' || e.key === 'K')) applyTool(state, 'link');
            else handled = false;
            if (handled) e.preventDefault();
        });
    }

    window.NotesToolbar = { mount, bindShortcuts };
})();
