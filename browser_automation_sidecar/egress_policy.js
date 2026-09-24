const dns = require('dns').promises;
const net = require('net');

const restricted = new net.BlockList();
for (const [address, prefix] of [
  ['0.0.0.0', 8], ['10.0.0.0', 8], ['100.64.0.0', 10], ['127.0.0.0', 8],
  ['169.254.0.0', 16], ['172.16.0.0', 12], ['192.168.0.0', 16],
  ['224.0.0.0', 4], ['240.0.0.0', 4],
]) restricted.addSubnet(address, prefix, 'ipv4');
for (const [address, prefix] of [
  ['::', 128], ['::1', 128], ['fc00::', 7], ['fe80::', 10],
  ['ff00::', 8],
]) restricted.addSubnet(address, prefix, 'ipv6');

const neverAllowed = new net.BlockList();
neverAllowed.addSubnet('0.0.0.0', 8, 'ipv4');
neverAllowed.addSubnet('169.254.0.0', 16, 'ipv4');
neverAllowed.addSubnet('224.0.0.0', 4, 'ipv4');
neverAllowed.addSubnet('240.0.0.0', 4, 'ipv4');
neverAllowed.addSubnet('::', 128, 'ipv6');
neverAllowed.addSubnet('fe80::', 10, 'ipv6');
neverAllowed.addSubnet('ff00::', 8, 'ipv6');

function blocked(list, ip) {
  if (String(ip).toLowerCase().startsWith('::ffff:')) return true;
  const family = net.isIP(ip);
  return !family || list.check(ip, family === 4 ? 'ipv4' : 'ipv6');
}

function parseAllowedOrigins(raw) {
  const values = JSON.parse(raw || '[]');
  if (!Array.isArray(values)) throw new Error('private origin allowlist must be an array');
  return new Set(values.map((value) => {
    const parsed = new URL(value);
    if (!['http:', 'https:', 'ws:', 'wss:'].includes(parsed.protocol) || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
      throw new Error('private origin allowlist contains an invalid origin');
    }
    return parsed.origin;
  }));
}

async function resolveBrowserTarget(raw, allowedOrigins, lookup = dns.lookup) {
  let parsed;
  try { parsed = new URL(raw); } catch (_) { throw new Error('invalid browser target URL'); }
  if (!['http:', 'https:', 'ws:', 'wss:'].includes(parsed.protocol) || parsed.username || parsed.password) {
    throw new Error('browser target scheme or credentials are blocked');
  }
  const privateAllowed = allowedOrigins.has(parsed.origin);
  const host = parsed.hostname.replace(/^\[|\]$/g, '');
  const addresses = net.isIP(host) ? [{ address: host }] : await lookup(host, { all: true, verbatim: true });
  if (!addresses.length) throw new Error('browser target DNS returned no addresses');
  for (const result of addresses) {
    const ip = result.address;
    if (blocked(neverAllowed, ip) || (blocked(restricted, ip) && !privateAllowed)) {
      throw new Error('browser target resolves to a blocked address');
    }
  }
  return { url: parsed, pinnedAddress: addresses[0].address };
}

async function validateBrowserTarget(raw, allowedOrigins, lookup = dns.lookup) {
  return (await resolveBrowserTarget(raw, allowedOrigins, lookup)).url;
}

module.exports = { parseAllowedOrigins, resolveBrowserTarget, validateBrowserTarget };
