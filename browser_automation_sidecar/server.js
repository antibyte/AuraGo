const http = require('http');
const crypto = require('crypto');
const fs = require('fs/promises');
const path = require('path');
const { chromium } = require('playwright');

const PORT = parseInt(process.env.PORT || '7331', 10);
const WORKSPACE_ROOT = process.env.WORKSPACE_ROOT || '/workspace';
const DOWNLOAD_ROOT = process.env.DOWNLOAD_ROOT || '/downloads';
const HEADLESS = process.env.HEADLESS !== 'false';
const ALLOW_FILE_UPLOADS = process.env.ALLOW_FILE_UPLOADS !== 'false';
const ALLOW_FILE_DOWNLOADS = process.env.ALLOW_FILE_DOWNLOADS !== 'false';
const READ_ONLY = process.env.READ_ONLY === 'true';
const SIDECAR_TOKEN = String(process.env.AURAGO_BROWSER_AUTOMATION_TOKEN || '').trim();
const SESSION_TTL_MS = Math.max(1, parseInt(process.env.SESSION_TTL_MINUTES || '30', 10)) * 60 * 1000;
const MAX_SESSIONS = Math.max(1, parseInt(process.env.MAX_SESSIONS || '3', 10));
const VIEWPORT_WIDTH = Math.max(320, parseInt(process.env.VIEWPORT_WIDTH || '1280', 10));
const VIEWPORT_HEIGHT = Math.max(240, parseInt(process.env.VIEWPORT_HEIGHT || '720', 10));
const HTTP_TIMEOUT_MS = Math.max(1000, parseInt(process.env.HTTP_TIMEOUT_MS || '60000', 10));
const BODY_READ_TIMEOUT_MS = Math.max(1000, parseInt(process.env.BODY_READ_TIMEOUT_MS || String(Math.min(HTTP_TIMEOUT_MS, 15000)), 10));
const SHUTDOWN_GRACE_MS = Math.max(1000, parseInt(process.env.SHUTDOWN_GRACE_MS || '10000', 10));
const MAX_DOWNLOAD_BYTES = Math.max(1024 * 1024, parseInt(process.env.MAX_DOWNLOAD_BYTES || String(100 * 1024 * 1024), 10));
const MAX_SCREENSHOT_BYTES = Math.max(256 * 1024, parseInt(process.env.MAX_SCREENSHOT_BYTES || String(25 * 1024 * 1024), 10));
const MAX_DOWNLOAD_RECORDS = Math.max(1, parseInt(process.env.MAX_DOWNLOAD_RECORDS || '25', 10));
const FILE_RETENTION_MS = Math.max(SESSION_TTL_MS, parseInt(process.env.FILE_RETENTION_MINUTES || String(Math.ceil((SESSION_TTL_MS * 2) / 60000)), 10) * 60 * 1000);
const DEFAULT_SCREENSHOT_DIR = path.join(WORKSPACE_ROOT, 'browser_screenshots');

const sessions = new Map();
let browserPromise = null;
let shuttingDown = false;

function json(res, statusCode, payload) {
  res.writeHead(statusCode, { 'Content-Type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify(payload));
}

function hasValidSidecarToken(req) {
  if (!SIDECAR_TOKEN) return true;
  const provided = typeof req.headers['x-aurago-sidecar-token'] === 'string'
    ? req.headers['x-aurago-sidecar-token'].trim()
    : '';
  if (!provided) return false;
  const expectedBuf = Buffer.from(SIDECAR_TOKEN, 'utf8');
  const providedBuf = Buffer.from(provided, 'utf8');
  if (expectedBuf.length !== providedBuf.length) return false;
  return crypto.timingSafeEqual(expectedBuf, providedBuf);
}

function sanitizeFilename(name) {
  return String(name || 'download.bin').replace(/[^a-zA-Z0-9._-]+/g, '_');
}

function sessionId() {
  return `ba_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

function resolveUnder(root, relPath) {
  if (!relPath || typeof relPath !== 'string') {
    throw new Error('path is required');
  }
  const full = path.resolve(root, relPath);
  const normalizedRoot = path.resolve(root);
  if (full !== normalizedRoot && !full.startsWith(normalizedRoot + path.sep)) {
    throw new Error('path escapes allowed root');
  }
  return full;
}

function relFromRoot(root, fullPath) {
  return path.relative(path.resolve(root), path.resolve(fullPath)).split(path.sep).join('/');
}

async function ensureDir(dirPath) {
  await fs.mkdir(dirPath, { recursive: true });
}

async function ensureFileWithinLimit(filePath, maxBytes, kind) {
  const stat = await fs.stat(filePath);
  if (stat.size > maxBytes) {
    await fs.unlink(filePath).catch(() => {});
    throw new Error(`${kind} exceeds size limit of ${maxBytes} bytes`);
  }
  return stat;
}

async function cleanupOldEntries(root, maxAgeMs) {
  let entries = [];
  try {
    entries = await fs.readdir(root, { withFileTypes: true });
  } catch (_) {
    return;
  }
  const cutoff = Date.now() - maxAgeMs;
  for (const entry of entries) {
    const target = path.join(root, entry.name);
    try {
      const stat = await fs.stat(target);
      if (stat.mtimeMs >= cutoff) {
        continue;
      }
      if (entry.isDirectory()) {
        await fs.rm(target, { recursive: true, force: true });
      } else {
        await fs.unlink(target).catch(() => {});
      }
    } catch (_) {
    }
  }
}

async function cleanupStaleArtifacts() {
  await cleanupOldEntries(DOWNLOAD_ROOT, FILE_RETENTION_MS);
  await cleanupOldEntries(DEFAULT_SCREENSHOT_DIR, FILE_RETENTION_MS);
}

function pushDownloadEntry(session, entry) {
  session.downloads.unshift(entry);
  while (session.downloads.length > MAX_DOWNLOAD_RECORDS) {
    session.downloads.pop();
  }
}

async function getBrowser() {
  if (!browserPromise) {
    browserPromise = chromium.launch({ headless: HEADLESS }).catch((error) => {
      browserPromise = null;
      throw error;
    });
  }
  return browserPromise;
}

function touchSession(session) {
  session.lastUsedAt = Date.now();
}

async function destroySession(id) {
  const session = sessions.get(id);
  if (!session) return;
  sessions.delete(id);
  try {
    await session.context.close();
  } catch (error) {
    console.warn(`Failed to close browser automation session ${id}:`, error && error.message ? error.message : error);
  }
  for (const screenshotPath of session.screenshots || []) {
    await fs.unlink(screenshotPath).catch(() => {});
  }
  await fs.rm(path.join(DOWNLOAD_ROOT, session.id), { recursive: true, force: true }).catch(() => {});
}

function isSessionExpired(session) {
  return Date.now() - session.lastUsedAt > SESSION_TTL_MS;
}

async function getActiveSession(id, operation) {
  const session = sessions.get(id);
  if (!session) {
    return { error: { status: 'error', operation, message: 'session not found', session_id: id || '' } };
  }
  if (isSessionExpired(session)) {
    await destroySession(session.id);
    return { error: { status: 'error', operation, message: 'session expired', session_id: id || '' } };
  }
  if (session.page && session.page.isClosed()) {
    await destroySession(session.id);
    return { error: { status: 'error', operation, message: 'session page was closed or crashed', session_id: id || '' } };
  }
  touchSession(session);
  return { session };
}

async function enforceSessionLimit() {
  if (sessions.size < MAX_SESSIONS) return;
  const oldest = [...sessions.values()].sort((a, b) => a.lastUsedAt - b.lastUsedAt)[0];
  if (oldest) {
    await destroySession(oldest.id);
  }
}

async function maybeWaitForLoad(page, timeoutMs) {
  try {
    await page.waitForLoadState('load', { timeout: Math.min(timeoutMs, 4000) });
  } catch (_) {
  }
}

async function collectState(page, options = {}) {
  const maxElements = Math.max(1, Math.min(parseInt(options.maxElements || 20, 10), 100));
  const includeDOMSnippet = options.domSnippet === true;
  const result = await page.evaluate(({ maxElements: maxItems, includeDOMSnippet: includeSnippet }) => {
    const selectorForElement = (el) => {
      const tag = (el.tagName || '').toLowerCase();
      const esc = (value) => String(value || '').replace(/\\/g, '\\\\').replace(/"/g, '\\"');
      if (el.id) return `#${esc(el.id)}`;
      const testId = el.getAttribute && el.getAttribute('data-testid');
      if (testId) return `[data-testid="${esc(testId)}"]`;
      const name = el.getAttribute && el.getAttribute('name');
      if (name) return `${tag}[name="${esc(name)}"]`;
      const ariaLabel = el.getAttribute && el.getAttribute('aria-label');
      if (ariaLabel) return `${tag}[aria-label="${esc(ariaLabel)}"]`;
      const placeholder = el.getAttribute && el.getAttribute('placeholder');
      if (placeholder && (tag === 'input' || tag === 'textarea')) return `${tag}[placeholder="${esc(placeholder)}"]`;
      if (!el.parentElement) return tag || '*';
      const siblings = Array.from(el.parentElement.children).filter((node) => (node.tagName || '').toLowerCase() === tag);
      const index = Math.max(0, siblings.indexOf(el)) + 1;
      return `${tag}:nth-of-type(${index})`;
    };
    const summarizeText = (text, limit) => {
      const normalized = String(text || '').replace(/\s+/g, ' ').trim();
      return normalized.length > limit ? normalized.slice(0, limit) + '...' : normalized;
    };
    const pick = (nodes, mapper) => {
      return Array.from(nodes).slice(0, maxItems).map(mapper);
    };
    const fieldNodes = document.querySelectorAll('input, textarea, select');
    const buttonNodes = document.querySelectorAll('button, input[type="submit"], input[type="button"], [role="button"]');
    const linkNodes = document.querySelectorAll('a[href]');
    const interactiveNodes = document.querySelectorAll('a, button, input, textarea, select, [role="button"]');

    const mapField = (el) => ({
      selector: selectorForElement(el),
      tag: (el.tagName || '').toLowerCase(),
      type: el.getAttribute('type') || '',
      name: el.getAttribute('name') || '',
      label: summarizeText(el.getAttribute('aria-label') || el.getAttribute('placeholder') || '', 120),
      value: (el.getAttribute('type') || '').toLowerCase() === 'password' ? '' : summarizeText(el.value || '', 120),
      required: !!el.required,
      disabled: !!el.disabled
    });

    const mapAction = (el) => ({
      selector: selectorForElement(el),
      tag: (el.tagName || '').toLowerCase(),
      text: summarizeText(el.innerText || el.textContent || el.value || '', 140),
      href: el.href || '',
      disabled: !!el.disabled
    });

    return {
      url: window.location.href,
      title: document.title || '',
      visible_text_summary: summarizeText(document.body ? document.body.innerText : '', 1200),
      form_fields: pick(fieldNodes, mapField),
      buttons: pick(buttonNodes, mapAction),
      links: pick(linkNodes, mapAction),
      elements: pick(interactiveNodes, mapAction),
      dom_snippet: includeSnippet ? summarizeText(document.documentElement ? document.documentElement.outerHTML : '', 1600) : ''
    };
  }, { maxElements, includeDOMSnippet });
  return result;
}

async function registerDownload(session, download) {
  if (!ALLOW_FILE_DOWNLOADS) return;
  const sessionDir = path.join(DOWNLOAD_ROOT, session.id);
  await ensureDir(sessionDir);
  const baseName = sanitizeFilename(await download.suggestedFilename());
  let dest = path.join(sessionDir, baseName);
  let counter = 1;
  while (true) {
    try {
      await fs.access(dest);
      const ext = path.extname(baseName);
      const stem = baseName.slice(0, baseName.length - ext.length);
      dest = path.join(sessionDir, `${stem}_${counter}${ext}`);
      counter += 1;
    } catch (_) {
      break;
    }
  }
  try {
    await download.saveAs(dest);
    const stat = await ensureFileWithinLimit(dest, MAX_DOWNLOAD_BYTES, 'download');
    pushDownloadEntry(session, {
      name: path.basename(dest),
      rel_path: relFromRoot(DOWNLOAD_ROOT, dest),
      size: stat.size,
      created_at: new Date().toISOString(),
      status: 'success'
    });
  } catch (error) {
    await fs.unlink(dest).catch(() => {});
    pushDownloadEntry(session, {
      name: path.basename(dest),
      created_at: new Date().toISOString(),
      status: 'error',
      message: error && error.message ? error.message : 'download failed'
    });
  }
}

async function createSession(targetURL) {
  await enforceSessionLimit();
  const browser = await getBrowser();
  const context = await browser.newContext({
    acceptDownloads: ALLOW_FILE_DOWNLOADS,
    viewport: { width: VIEWPORT_WIDTH, height: VIEWPORT_HEIGHT }
  });
  const page = await context.newPage();
  const id = sessionId();
  const session = {
    id,
    context,
    page,
    downloads: [],
    screenshots: [],
    createdAt: Date.now(),
    lastUsedAt: Date.now()
  };
  page.on('download', (download) => {
    registerDownload(session, download).catch(() => {});
  });
  sessions.set(id, session);
  if (targetURL) {
    await page.goto(targetURL, { waitUntil: 'load', timeout: 30000 });
  }
  const state = await collectState(page, { maxElements: 12, domSnippet: false });
  return {
    status: 'success',
    operation: 'create_session',
    session_id: id,
    url: state.url,
    title: state.title,
    message: targetURL ? `Session created and navigated to ${state.url}` : 'Session created',
    page_summary: state.visible_text_summary
  };
}

async function handleAutomation(body) {
  const operation = String(body.operation || '').trim();
  if (!operation) {
    return { status: 'error', message: 'operation is required' };
  }
  if (READ_ONLY && ['click', 'type', 'select', 'press', 'upload_file'].includes(operation)) {
    return { status: 'error', operation, message: 'browser automation sidecar is in read-only mode' };
  }

  if (operation === 'create_session') {
    return createSession(body.url || '');
  }

  const resolved = await getActiveSession(String(body.session_id || ''), operation);
  if (resolved.error) {
    return resolved.error;
  }
  const { session } = resolved;
  const page = session.page;
  const timeoutMs = Math.max(1000, parseInt(body.timeout_ms || '15000', 10));
  const selector = typeof body.selector === 'string' ? body.selector : '';

  switch (operation) {
    case 'close_session':
      await destroySession(session.id);
      return { status: 'success', operation, session_id: session.id, message: 'session closed' };
    case 'navigate':
      await page.goto(String(body.url || ''), { waitUntil: 'load', timeout: timeoutMs });
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `navigated to ${page.url()}` };
    case 'click':
      await page.locator(selector).click({ timeout: timeoutMs });
      await maybeWaitForLoad(page, timeoutMs);
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `clicked ${selector}` };
    case 'type':
      await page.locator(selector).fill(String(body.text || ''), { timeout: timeoutMs });
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `typed into ${selector}` };
    case 'select':
      await page.locator(selector).selectOption(String(body.value || ''));
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `selected ${body.value || ''} on ${selector}` };
    case 'press':
      if (selector) {
        await page.locator(selector).press(String(body.key || 'Enter'), { timeout: timeoutMs });
      } else {
        await page.keyboard.press(String(body.key || 'Enter'));
      }
      await maybeWaitForLoad(page, timeoutMs);
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `pressed ${body.key || 'Enter'}` };
    case 'wait_for': {
      const waitState = String(body.wait_for || 'visible');
      if (waitState === 'load' || waitState === 'networkidle') {
        await page.waitForLoadState(waitState, { timeout: timeoutMs });
      } else {
        if (!selector) throw new Error('selector is required for this wait_for state');
        await page.locator(selector).waitFor({ state: waitState, timeout: timeoutMs });
      }
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `wait_for ${waitState} completed` };
    }
    case 'extract': {
      const state = await collectState(page, { maxElements: body.max_elements, domSnippet: body.dom_snippet === true });
      return { status: 'success', operation, session_id: session.id, ...state, message: 'page state extracted' };
    }
    case 'current_state': {
      const state = await collectState(page, { maxElements: 10, domSnippet: false });
      return { status: 'success', operation, session_id: session.id, url: state.url, title: state.title, page_summary: state.visible_text_summary, message: 'current page state retrieved' };
    }
    case 'screenshot': {
      const relPath = String(body.output_path || '');
      const target = resolveUnder(WORKSPACE_ROOT, relPath);
      await ensureDir(path.dirname(target));
      await page.screenshot({ path: target, fullPage: body.full_page === true });
      await ensureFileWithinLimit(target, MAX_SCREENSHOT_BYTES, 'screenshot');
      session.screenshots.push(target);
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), screenshot_rel_path: relFromRoot(WORKSPACE_ROOT, target), message: `screenshot saved to ${relPath}` };
    }
    case 'upload_file': {
      if (!ALLOW_FILE_UPLOADS) return { status: 'error', operation, session_id: session.id, message: 'file uploads are disabled', http_status: 400 };
      const relPath = String(body.file_path || '');
      const target = resolveUnder(WORKSPACE_ROOT, relPath);
      await page.locator(selector).setInputFiles(target, { timeout: timeoutMs });
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `uploaded file ${relPath}` };
    }
    case 'list_downloads':
      return { status: 'success', operation, session_id: session.id, downloads: session.downloads, message: `listed ${session.downloads.length} downloads` };
    case 'get_download': {
      if (!ALLOW_FILE_DOWNLOADS) return { status: 'error', operation, session_id: session.id, message: 'file downloads are disabled', http_status: 400 };
      let chosen = null;
      const requestedName = String(body.download_name || '').trim();
      if (requestedName) {
        chosen = session.downloads.find((entry) => entry.name === requestedName);
      } else if (session.downloads.length > 0) {
        chosen = session.downloads[0];
      }
      if (!chosen) {
        return { status: 'error', operation, session_id: session.id, message: 'download not found' };
      }
      if (chosen.status === 'error') {
        return { status: 'error', operation, session_id: session.id, message: chosen.message || `download failed: ${chosen.name}` };
      }
      return { status: 'success', operation, session_id: session.id, download_name: chosen.name, download_rel_path: chosen.rel_path, message: `download ready: ${chosen.name}` };
    }
    default:
      return { status: 'error', operation, session_id: session.id, message: `unsupported operation: ${operation}` };
  }
}

async function readBody(req) {
  return new Promise((resolve, reject) => {
    let data = '';
    let settled = false;
    const timeout = setTimeout(() => {
      cleanup();
      settled = true;
      reject(new Error('request body timed out'));
      req.destroy();
    }, BODY_READ_TIMEOUT_MS);
    const cleanup = () => {
      clearTimeout(timeout);
      req.removeListener('data', onData);
      req.removeListener('end', onEnd);
      req.removeListener('error', onError);
    };
    const onData = (chunk) => {
      if (settled) return;
      data += chunk;
      if (data.length > 1024 * 1024) {
        settled = true;
        cleanup();
        reject(new Error('request body too large'));
      }
    };
    const onEnd = () => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(data);
    };
    const onError = (error) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(error);
    };
    req.on('data', onData);
    req.on('end', onEnd);
    req.on('error', onError);
  });
}

setInterval(() => {
  const now = Date.now();
  for (const [id, session] of sessions.entries()) {
    if (now - session.lastUsedAt > SESSION_TTL_MS) {
      destroySession(id).catch(() => {});
    }
  }
}, 30 * 1000);

setInterval(() => {
  cleanupStaleArtifacts().catch(() => {});
}, 10 * 60 * 1000);

const server = http.createServer({ requestTimeout: HTTP_TIMEOUT_MS }, async (req, res) => {
  try {
    if ((req.url === '/health' || req.url === '/automation') && !hasValidSidecarToken(req)) {
      return json(res, 401, { status: 'error', message: 'unauthorized' });
    }

    if (req.method === 'GET' && req.url === '/health') {
      try {
        const browser = await getBrowser();
        const version = await browser.version();
        return json(res, 200, {
          status: 'success',
          message: 'browser automation sidecar is healthy',
          sessions: sessions.size,
          headless: HEADLESS,
          read_only: READ_ONLY,
          browser_version: version
        });
      } catch (error) {
        return json(res, 503, {
          status: 'error',
          retryable: false,
          message: error && error.message ? `browser unavailable: ${error.message}` : 'browser unavailable'
        });
      }
    }

    if (req.method === 'POST' && req.url === '/automation') {
      const contentType = String(req.headers['content-type'] || '').toLowerCase();
      if (!contentType.includes('application/json')) {
        return json(res, 415, { status: 'error', message: 'content-type must be application/json' });
      }
      const raw = await readBody(req);
      let body;
      try {
        body = raw ? JSON.parse(raw) : {};
      } catch (parseErr) {
        return json(res, 400, { status: 'error', message: 'invalid JSON: ' + (parseErr.message || String(parseErr)) });
      }
      const result = await handleAutomation(body);
      const statusCode = Number.isInteger(result.http_status) ? result.http_status : (result.status === 'error' ? 400 : 200);
      delete result.http_status;
      return json(res, statusCode, result);
    }

    return json(res, 404, { status: 'error', message: 'not found' });
  } catch (error) {
    return json(res, 500, { status: 'error', message: error && error.message ? error.message : String(error) });
  }
});

async function shutdown(signal) {
  if (shuttingDown) return;
  shuttingDown = true;
  try {
    await Promise.race([
      new Promise((resolve) => server.close(() => resolve())),
      new Promise((resolve) => setTimeout(resolve, SHUTDOWN_GRACE_MS))
    ]);
    const sessionIds = [...sessions.keys()];
    for (const id of sessionIds) {
      await destroySession(id);
    }
    if (browserPromise) {
      const browser = await browserPromise.catch(() => null);
      if (browser) {
        await browser.close().catch(() => {});
      }
      browserPromise = null;
    }
  } finally {
    if (signal) {
      process.exit(0);
    }
  }
}

async function boot() {
  await ensureDir(WORKSPACE_ROOT);
  await ensureDir(DOWNLOAD_ROOT);
  await ensureDir(DEFAULT_SCREENSHOT_DIR);
  await cleanupStaleArtifacts();
  server.listen(PORT, '0.0.0.0');
}

process.on('SIGTERM', () => {
  shutdown('SIGTERM').catch((error) => {
    console.error('Failed to shut down browser automation sidecar cleanly:', error);
    process.exit(1);
  });
});

process.on('SIGINT', () => {
  shutdown('SIGINT').catch((error) => {
    console.error('Failed to shut down browser automation sidecar cleanly:', error);
    process.exit(1);
  });
});

boot().catch((error) => {
  console.error('Failed to start browser automation sidecar:', error);
  process.exit(1);
});
