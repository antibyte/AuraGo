const http = require('node:http');
const net = require('node:net');
const { parseAllowedOrigins, resolveBrowserTarget } = require('./egress_policy');

const port = Number(process.env.PORT || 7332);
const allowed = parseAllowedOrigins(process.env.AURAGO_BROWSER_ALLOWED_PRIVATE_ORIGINS || '[]');

function cleanHeaders(headers, host) {
  const next = { ...headers, host };
  delete next['proxy-authorization'];
  delete next['proxy-connection'];
  return next;
}

function rejectSocket(socket) {
  socket.end('HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n');
}

async function pinnedTarget(raw) {
  const { url, pinnedAddress } = await resolveBrowserTarget(raw, allowed);
  return { url, pinnedAddress, port: Number(url.port || (url.protocol === 'https:' || url.protocol === 'wss:' ? 443 : 80)) };
}

function connectPinned(target, onConnect, onFailure) {
  const upstream = net.connect({ host: target.pinnedAddress, port: target.port, timeout: 15000 });
  upstream.once('connect', () => onConnect(upstream));
  upstream.once('error', () => onFailure(upstream));
  upstream.once('timeout', () => upstream.destroy(new Error('egress dial timeout')));
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'content-type': 'application/json', 'cache-control': 'no-store' });
    res.end(JSON.stringify({ status: 'success', policy_version: 'egress-v1', dns_pinned: true }));
    return;
  }
  try {
    const target = await pinnedTarget(req.url);
    if (target.url.protocol !== 'http:') throw new Error('unsupported proxy request');
    const upstream = http.request({
      hostname: target.pinnedAddress,
      port: target.port,
      method: req.method,
      path: target.url.pathname + target.url.search,
      headers: cleanHeaders(req.headers, target.url.host),
      agent: false,
      timeout: 30000,
    }, (response) => {
      res.writeHead(response.statusCode || 502, response.headers);
      response.pipe(res);
    });
    upstream.on('error', () => { if (!res.headersSent) res.writeHead(502); res.end(); });
    req.pipe(upstream);
  } catch (_) {
    res.writeHead(403);
    res.end();
  }
});

server.on('connect', async (req, socket, head) => {
  try {
    if (!req.url || !/^(?:\[[0-9a-f:.]+\]|[^:/?#@]+):[0-9]{1,5}$/i.test(req.url)) {
      throw new Error('invalid CONNECT authority');
    }
    const target = await pinnedTarget('https://' + req.url + '/');
    connectPinned(target, (upstream) => {
      socket.write('HTTP/1.1 200 Connection Established\r\n\r\n');
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    }, () => rejectSocket(socket));
  } catch (_) {
    rejectSocket(socket);
  }
});

server.on('upgrade', async (req, socket, head) => {
  try {
    const target = await pinnedTarget(req.url);
    if (target.url.protocol !== 'ws:') throw new Error('unsupported WebSocket proxy request');
    connectPinned(target, (upstream) => {
      const lines = [`GET ${target.url.pathname + target.url.search} HTTP/1.1`];
      const headers = cleanHeaders(req.headers, target.url.host);
      for (const [name, value] of Object.entries(headers)) {
        if (Array.isArray(value)) value.forEach((item) => lines.push(`${name}: ${item}`));
        else if (value !== undefined) lines.push(`${name}: ${value}`);
      }
      upstream.write(lines.join('\r\n') + '\r\n\r\n');
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    }, () => rejectSocket(socket));
  } catch (_) {
    rejectSocket(socket);
  }
});

if (require.main === module) server.listen(port, '0.0.0.0');

module.exports = { server };
