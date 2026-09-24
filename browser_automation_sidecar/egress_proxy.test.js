const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');

function listen(server) {
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(server.address().port)));
}

function close(server) {
  return new Promise((resolve) => server.close(resolve));
}

function viaProxy(port, target) {
  return new Promise((resolve, reject) => {
    http.get({ hostname: '127.0.0.1', port, path: target }, (response) => {
      let body = '';
      response.on('data', (chunk) => { body += chunk; });
      response.on('end', () => resolve({ status: response.statusCode, body }));
    }).on('error', reject);
  });
}

function connectViaProxy(port, targetPort) {
  return new Promise((resolve, reject) => {
    const req = http.request({ hostname: '127.0.0.1', port, method: 'CONNECT', path: `127.0.0.1:${targetPort}` });
    req.on('connect', (response, socket) => {
      if (response.statusCode !== 200) { socket.destroy(); resolve(response.statusCode); return; }
      let data = '';
      socket.on('data', (chunk) => { data += chunk; if (data.includes('allowed')) { socket.destroy(); resolve(200); } });
      socket.on('error', reject);
      socket.write('GET / HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n');
    });
    req.on('error', reject);
    req.end();
  });
}

test('egress proxy pins an allowed private origin and rejects other private destinations', async () => {
  const upstream = http.createServer((_, res) => res.end('allowed'));
  const upstreamPort = await listen(upstream);
  process.env.AURAGO_BROWSER_ALLOWED_PRIVATE_ORIGINS = JSON.stringify([`http://127.0.0.1:${upstreamPort}`, `https://127.0.0.1:${upstreamPort}`]);
  const { server } = require('./egress_proxy');
  const proxyPort = await listen(server);
  try {
    const allowed = await viaProxy(proxyPort, `http://127.0.0.1:${upstreamPort}/test`);
    assert.equal(allowed.status, 200);
    assert.equal(allowed.body, 'allowed');
    assert.equal(await connectViaProxy(proxyPort, upstreamPort), 200);
    const blocked = await viaProxy(proxyPort, 'http://127.0.0.1:9/test');
    assert.equal(blocked.status, 403);
    const metadata = await viaProxy(proxyPort, 'http://169.254.169.254/latest/meta-data/');
    assert.equal(metadata.status, 403);
  } finally {
    await close(server);
    await close(upstream);
  }
});
