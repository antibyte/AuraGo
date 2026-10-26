import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const content = { innerHTML: '' };
const context = vm.createContext({
    configData: { virtual_desktop: { enabled: true } },
    document: { getElementById: () => content },
    t: key => key, escapeAttr: value => String(value), attachChangeListeners() {}
});
vm.runInContext(fs.readFileSync(new URL('../ui/cfg/virtual_desktop.js', import.meta.url), 'utf8'), context);
context.renderVirtualDesktopSection({ label: 'Desktop', desc: '' });
assert.equal(context.configData.tools.qr_code.enabled, false);
assert.equal(context.configData.tools.qr_code.readonly, true);
assert.match(content.innerHTML, /class="toggle " data-path="tools\.qr_code\.enabled"/);
assert.match(content.innerHTML, /class="toggle on" data-path="tools\.qr_code\.readonly"/);
context.configData.tools.qr_code = { enabled: true, readonly: false };
context.renderVirtualDesktopSection(null);
assert.equal(context.configData.tools.qr_code.enabled, true);
assert.equal(context.configData.tools.qr_code.readonly, false);
assert.match(content.innerHTML, /class="toggle on" data-path="tools\.qr_code\.enabled"/);
assert.match(content.innerHTML, /class="toggle " data-path="tools\.qr_code\.readonly"/);
assert.equal(context.configData.virtual_desktop.readonly, undefined, 'agent settings never change the human editor gate');
console.log('test-qr-config: safe defaults, retained choices and independent human editor access passed');
