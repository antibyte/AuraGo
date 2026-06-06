#!/usr/bin/env node
'use strict';

const fs = require('fs');
const http = require('http');
const net = require('net');
const { URL } = require('url');

const bindHost = process.env.COMMANDCODE_PREVIEW_HOST || '0.0.0.0';
const bindPort = Number(process.env.COMMANDCODE_PREVIEW_PORT || 80);
const targetFile = process.env.COMMANDCODE_PREVIEW_TARGET_FILE || '/tmp/commandcode-preview-target';
const defaultTarget = process.env.COMMANDCODE_PREVIEW_TARGET || 'http://127.0.0.1:5173';
const candidatePorts = (process.env.COMMANDCODE_PREVIEW_CANDIDATE_PORTS || '5173,4173,3000,3001,5174,8080,8000')
  .split(',')
  .map(value => Number(value.trim()))
  .filter(port => Number.isInteger(port) && port > 0 && port < 65536);

function normalizeTarget(value) {
  try {
    return new URL(value);
  } catch (_) {
    return new URL(defaultTarget);
  }
}

function currentTarget() {
  try {
    const value = fs.readFileSync(targetFile, 'utf8').trim();
    return normalizeTarget(value || defaultTarget);
  } catch (_) {
    return normalizeTarget(defaultTarget);
  }
}

function writeTarget(target) {
  try {
    fs.writeFileSync(targetFile, target.href);
  } catch (_) {
    // The gateway still works with an in-memory target when the file is not writable.
  }
}

function targetURL(reqURL, target) {
  return new URL(reqURL, target);
}

function probeTarget(target) {
  return new Promise(resolve => {
    const req = http.request({
      hostname: target.hostname,
      port: Number(target.port || 80),
      path: '/',
      method: 'GET',
      timeout: 450,
      headers: { host: target.host }
    }, res => {
      res.resume();
      resolve(true);
    });
    req.on('error', () => resolve(false));
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
    req.end();
  });
}

async function resolveTarget(forceDiscover) {
  const configured = currentTarget();
  if (!forceDiscover && await probeTarget(configured)) return configured;
  for (const port of candidatePorts) {
    const target = new URL(`http://127.0.0.1:${port}/`);
    if (await probeTarget(target)) {
      writeTarget(target);
      return target;
    }
  }
  return configured;
}

function placeholder(res) {
  res.writeHead(200, {
    'content-type': 'text/html; charset=utf-8',
    'cache-control': 'no-store'
  });
  res.end(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>CommandCode Preview</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #111827; color: #e5e7eb; }
    main { max-width: 680px; padding: 32px; line-height: 1.55; }
    code { padding: 2px 6px; border-radius: 6px; background: #020617; color: #93c5fd; }
  </style>
</head>
<body>
  <main>
    <h1>CommandCode Preview</h1>
    <p>Start a development server in the terminal and point this preview to it.</p>
    <p><code>npm run dev -- --host 0.0.0.0</code></p>
    <p><code>preview-port 5173</code></p>
  </main>
  <script>setTimeout(() => window.location.reload(), 1500);</script>
</body>
</html>`);
}

function proxyHTTPRequest(req, res, target, allowRetry) {
  const upstream = targetURL(req.url || '/', target);
  const proxyReq = http.request({
    hostname: upstream.hostname,
    port: Number(upstream.port || 80),
    path: upstream.pathname + upstream.search,
    method: req.method,
    headers: Object.assign({}, req.headers, { host: upstream.host })
  }, proxyRes => {
    res.writeHead(proxyRes.statusCode || 502, proxyRes.headers);
    proxyRes.pipe(res);
  });
  proxyReq.on('error', async () => {
    if (allowRetry && !res.headersSent && (req.method === 'GET' || req.method === 'HEAD')) {
      const retryTarget = await resolveTarget(true);
      if (retryTarget.href !== target.href) {
        proxyHTTPRequest(req, res, retryTarget, false);
        return;
      }
    }
    if (!res.headersSent) placeholder(res);
  });
  req.pipe(proxyReq);
}

const server = http.createServer(async (req, res) => {
  const target = await resolveTarget(false);
  proxyHTTPRequest(req, res, target, true);
});

server.on('upgrade', async (req, socket, head) => {
  const upstream = targetURL(req.url || '/', await resolveTarget(false));
  const upstreamSocket = net.connect(Number(upstream.port || 80), upstream.hostname, () => {
    upstreamSocket.write(`${req.method} ${upstream.pathname}${upstream.search} HTTP/${req.httpVersion}\r\n`);
    const headers = Object.assign({}, req.headers, { host: upstream.host });
    for (const [key, value] of Object.entries(headers)) {
      upstreamSocket.write(`${key}: ${value}\r\n`);
    }
    upstreamSocket.write('\r\n');
    if (head && head.length) upstreamSocket.write(head);
    socket.pipe(upstreamSocket).pipe(socket);
  });
  upstreamSocket.on('error', () => socket.destroy());
});

server.listen(bindPort, bindHost, () => {
  console.log(`CommandCode preview listening on ${bindHost}:${bindPort}, target ${currentTarget().href}`);
});
