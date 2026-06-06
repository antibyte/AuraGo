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

function currentTarget() {
  try {
    const value = fs.readFileSync(targetFile, 'utf8').trim();
    return value || defaultTarget;
  } catch (_) {
    return defaultTarget;
  }
}

function targetURL(reqURL) {
  const target = new URL(currentTarget());
  return new URL(reqURL, target);
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
</body>
</html>`);
}

const server = http.createServer((req, res) => {
  const upstream = targetURL(req.url || '/');
  const proxyReq = http.request({
    hostname: upstream.hostname,
    port: upstream.port || 80,
    path: upstream.pathname + upstream.search,
    method: req.method,
    headers: Object.assign({}, req.headers, { host: upstream.host })
  }, proxyRes => {
    res.writeHead(proxyRes.statusCode || 502, proxyRes.headers);
    proxyRes.pipe(res);
  });
  proxyReq.on('error', () => placeholder(res));
  req.pipe(proxyReq);
});

server.on('upgrade', (req, socket, head) => {
  const upstream = targetURL(req.url || '/');
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
  console.log(`CommandCode preview listening on ${bindHost}:${bindPort}, target ${currentTarget()}`);
});
