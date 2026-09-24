const test = require('node:test');
const assert = require('node:assert/strict');
const { parseAllowedOrigins, validateBrowserTarget } = require('./egress_policy');

const lookup = async (host) => [{ address: host === 'rebinding.test' ? '169.254.169.254' : '93.184.215.14' }];

test('blocks local files and metadata even when private origins are allowed', async () => {
  const allowed = parseAllowedOrigins('["http://169.254.169.254"]');
  await assert.rejects(validateBrowserTarget('file:///etc/passwd', allowed, lookup));
  await assert.rejects(validateBrowserTarget('http://169.254.169.254/latest/meta-data', allowed, lookup));
  await assert.rejects(validateBrowserTarget('http://rebinding.test/latest', allowed, lookup));
});

test('allows an exact configured home lab origin only', async () => {
  const allowed = parseAllowedOrigins('["http://192.168.1.10:8123"]');
  await validateBrowserTarget('http://192.168.1.10:8123/state', allowed, lookup);
  await assert.rejects(validateBrowserTarget('http://192.168.1.10:8124/state', allowed, lookup));
  await assert.rejects(validateBrowserTarget('http://192.168.1.10:8123.evil.test/', allowed, lookup));
});

test('rejects any DNS answer on a restricted network', async () => {
  await assert.rejects(validateBrowserTarget('https://example.test/', new Set(), async () => [
    { address: '93.184.215.14' }, { address: '10.0.0.5' },
  ]));
});
