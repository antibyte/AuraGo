import fs from 'node:fs';
import vm from 'node:vm';

// Load the same rendering dependencies as config.html into an existing VM.
// A separate window object needs its exported properties mirrored as globals.
export function loadConfigRuntime(context) {
    context.window ||= {};
    context.setTimeout ||= setTimeout;
    context.clearTimeout ||= clearTimeout;
    context.AbortController ||= AbortController;
    const read = name => fs.readFileSync(new URL('../' + name, import.meta.url), 'utf8');
    const shared = read('ui/js/shared/shared-core.js');
    vm.runInContext(shared.slice(shared.indexOf('function tOr('), shared.indexOf('let _sharedModalOverlay')), context);
    for (const name of ['ui/js/precision/icons.js', 'ui/js/config/feedback.js', 'ui/js/config/icons.js', 'ui/js/config/components.js', 'ui/js/config/utils.js', 'ui/cfg/form-builder.js']) {
        vm.runInContext(read(name), context, { filename: name });
        Object.assign(context, context.window);
    }
    const main = read('ui/js/config/main.js');
    vm.runInContext(main.slice(main.indexOf('function syncToggleA11y('), main.indexOf('function enhanceConfigControls(')), context);
}

export function configTestElement() {
    const attributes = new Map(), classes = new Set();
    return {
        nodeType: 1, dataset: {}, children: [], disabled: false, isConnected: true,
        setAttribute: (name, value) => attributes.set(name, String(value)),
        getAttribute: name => attributes.get(name) ?? null,
        hasAttribute: name => attributes.has(name),
        removeAttribute: name => attributes.delete(name),
        classList: { add: (...names) => names.forEach(name => classes.add(name)), remove: (...names) => names.forEach(name => classes.delete(name)), contains: name => classes.has(name), toggle(name, force) { const on = force ?? !classes.has(name); if (on) classes.add(name); else classes.delete(name); return on; } },
        replaceChildren(...nodes) { this.children = nodes; },
        addEventListener() {}, closest() { return null; }, querySelector() { return null; }, querySelectorAll() { return []; }
    };
}
