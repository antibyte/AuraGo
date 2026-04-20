const http = require('http');
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
const SESSION_TTL_MS = Math.max(1, parseInt(process.env.SESSION_TTL_MINUTES || '30', 10)) * 60 * 1000;
const MAX_SESSIONS = Math.max(1, parseInt(process.env.MAX_SESSIONS || '3', 10));
const VIEWPORT_WIDTH = Math.max(320, parseInt(process.env.VIEWPORT_WIDTH || '1280', 10));
const VIEWPORT_HEIGHT = Math.max(240, parseInt(process.env.VIEWPORT_HEIGHT || '720', 10));

const sessions = new Map();
let browserPromise = null;

function json(res, statusCode, payload) {
  res.writeHead(statusCode, { 'Content-Type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify(payload));
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

async function getBrowser() {
  if (!browserPromise) {
    browserPromise = chromium.launch({ headless: HEADLESS });
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
  } catch (_) {
  }
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
  await download.saveAs(dest);
  const stat = await fs.stat(dest);
  const entry = {
    name: path.basename(dest),
    rel_path: relFromRoot(DOWNLOAD_ROOT, dest),
    size: stat.size,
    created_at: new Date().toISOString()
  };
  session.downloads.unshift(entry);
  if (session.downloads.length > 25) {
    session.downloads.length = 25;
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

  const session = sessions.get(String(body.session_id || ''));
  if (!session) {
    return { status: 'error', operation, message: 'session not found', session_id: body.session_id || '' };
  }
  touchSession(session);
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
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), screenshot_rel_path: relFromRoot(WORKSPACE_ROOT, target), message: `screenshot saved to ${relPath}` };
    }
    case 'upload_file': {
      if (!ALLOW_FILE_UPLOADS) throw new Error('file uploads are disabled');
      const relPath = String(body.file_path || '');
      const target = resolveUnder(WORKSPACE_ROOT, relPath);
      await page.locator(selector).setInputFiles(target, { timeout: timeoutMs });
      return { status: 'success', operation, session_id: session.id, url: page.url(), title: await page.title(), message: `uploaded file ${relPath}` };
    }
    case 'list_downloads':
      return { status: 'success', operation, session_id: session.id, downloads: session.downloads, message: `listed ${session.downloads.length} downloads` };
    case 'get_download': {
      if (!ALLOW_FILE_DOWNLOADS) throw new Error('file downloads are disabled');
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
      return { status: 'success', operation, session_id: session.id, download_name: chosen.name, download_rel_path: chosen.rel_path, message: `download ready: ${chosen.name}` };
    }
    default:
      return { status: 'error', operation, session_id: session.id, message: `unsupported operation: ${operation}` };
  }
}

async function readBody(req) {
  return new Promise((resolve, reject) => {
    let data = '';
    req.on('data', (chunk) => {
      data += chunk;
      if (data.length > 1024 * 1024) {
        reject(new Error('request body too large'));
      }
    });
    req.on('end', () => resolve(data));
    req.on('error', reject);
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

const server = http.createServer(async (req, res) => {
  try {
    if (req.method === 'GET' && req.url === '/health') {
      return json(res, 200, {
        status: 'success',
        message: 'browser automation sidecar is healthy',
        sessions: sessions.size,
        headless: HEADLESS,
        read_only: READ_ONLY
      });
    }

    if (req.method === 'POST' && req.url === '/automation') {
      const raw = await readBody(req);
      const body = raw ? JSON.parse(raw) : {};
      const result = await handleAutomation(body);
      const statusCode = result.status === 'error' ? 400 : 200;
      return json(res, statusCode, result);
    }

    return json(res, 404, { status: 'error', message: 'not found' });
  } catch (error) {
    return json(res, 500, { status: 'error', message: error && error.message ? error.message : String(error) });
  }
});

async function boot() {
  await ensureDir(WORKSPACE_ROOT);
  await ensureDir(DOWNLOAD_ROOT);
  server.listen(PORT, '0.0.0.0');
}

boot().catch((error) => {
  console.error('Failed to start browser automation sidecar:', error);
  process.exit(1);
});
