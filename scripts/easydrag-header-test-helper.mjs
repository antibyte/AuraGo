// Helpers of the Node checks that act on the editor header's pill (W1c). The name does not match test-easydrag*.mjs: the T0 run
// imports it from the checks and does not run it alone.

// pillText is what the pill says.
export const pillText = editor => editor.el.querySelector('[data-ed-pill-text]').textContent;

// pillKind is the state the pill shows: running, past, partial, link, outdated, timers, paused, changed, live or draft.
export const pillKind = editor => editor.el.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind');

// popEntry opens the pill's popover when it is closed and returns its entry data-ed-pop="name" (active, run-now, runs,
// publish-finish, repair), or null when the popover has none. The popover stays open until an entry closes it.
export function popEntry(editor, name) {
    if (!editor.el.querySelector('[data-ed-pill-popover]')) editor.el.querySelector('[data-ed-pill]').fire('click');
    return editor.el.querySelector('[data-ed-pill-popover] [data-ed-pop="' + name + '"]');
}
