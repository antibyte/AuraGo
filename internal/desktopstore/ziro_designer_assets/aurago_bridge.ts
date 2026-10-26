// SPDX-License-Identifier: GPL-3.0-or-later
// AuraGo host adapter for ZiroEDA. Shared remains owned by the parent desktop.
import { loadProject, saveProject, updateProjectFiles } from './home/projectStore.js';
import { newProjectFiles } from './home/new_project.js';
import { MEMORY_FILESYSTEM, wxMountFileSystem } from '@ziroeda/wx/filefn.js';

type File = { name: string; bytes: Uint8Array };
type Hooks = { open(files: File[]): void; snapshot(): File[] };
const fragment = new URLSearchParams(location.hash.slice(1));
const challenge = fragment.get('aurago_ziro');
const parentOrigin = fragment.get('parent_origin');
export const auraGoEmbedded = !!(challenge && parentOrigin && window.parent !== window);
if (auraGoEmbedded) history.replaceState(null, '', location.pathname + location.search);
let port: MessagePort | null = null;
let currentId: string | null = null;
let projectKey = '';
let draftKey = '';
let generation = 0;
let requestId = 0;
let clean = new Map<string, Uint8Array>();
let nativeRoot = '';
const changedFiles = new Map<string, Uint8Array>();
let nativeFiles: MEMORY_FILESYSTEM | null = null;
let unmountFiles: (() => void) | null = null;
const encoder = new TextEncoder();
function readProjectFile(path: string): Uint8Array | null | undefined {
  if (!nativeRoot || !path.startsWith(nativeRoot)) return undefined;
  return clean.get(path.slice(nativeRoot.length))?.slice() ?? null;
}
const waiting = new Map<number, { resolve(): void; reject(error: Error): void; timer: ReturnType<typeof setTimeout> }>();
export function auraGoProjectId(): string | null { return currentId; }
const marker = () => `aurago-ziro-draft:${currentId}`;
function mark(value: boolean): void {
  try { if (value) localStorage.setItem(marker(), '1'); else localStorage.removeItem(marker()); } catch { /* IndexedDB reports its own health. */ }
}
function remember(files: readonly { name: string; text: string; bytes?: Uint8Array }[]): void {
  if (!port || !currentId) return;
  let changed = false;
  nativeFiles?.SetWriteListener(null);
  for (const file of files) {
    const bytes = file.bytes ?? encoder.encode(file.text);
    const previous = clean.get(file.name);
    if (previous && previous.length === bytes.length && previous.every((value, i) => value === bytes[i])) continue;
    clean.set(file.name, bytes.slice()); changedFiles.set(file.name, bytes.slice()); changed = true;
    nativeFiles?.Write(file.name, bytes);
  }
  nativeFiles?.SetWriteListener(nativeWrite);
  if (!changed) return;
  generation++;
  mark(true);
  port.postMessage({ ziro: 1, type: 'dirty', generation });
}
function nativeWrite(name: string, bytes: Uint8Array): void {
  if (!port || !currentId) return;
  const id = currentId;
  remember([{ name, bytes, text: '' }]);
  void updateProjectFiles(id, [{ name, bytes }]).catch(() => {});
}
export const auraGoChanged = remember;
export function auraGoSave(): Promise<void> {
  if (!auraGoEmbedded) return Promise.resolve();
  if (!port) return Promise.reject(new Error('ziro.disconnected'));
  const id = ++requestId;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { waiting.delete(id); reject(new Error('ziro.disconnected')); }, 30000);
    waiting.set(id, { resolve, reject, timer });
    port!.postMessage({ ziro: 1, type: 'save-request', id });
  });
}
export function installAuraGoBridge(hooks: Hooks): () => void {
  if (!auraGoEmbedded) return () => {};
  Reflect.set(globalThis, '__auragoZiroProjectRead', readProjectFile);
  let disposed = false;
  let incoming: { key: string; windowKey: string; id: string; files: File[]; draft: File[] } | null = null;
  const displayName = () => projectKey.split('/').pop() || 'Project';
  const snapshot = () => {
    const files = new Map(hooks.snapshot().map(file => [file.name, file.bytes]));
    const unsaved = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(unsaved);
    if (unsaved.defaultPrevented) throw new Error('ziro.save_failed');
    for (const [name, bytes] of changedFiles) files.set(name, bytes);
    return [...files].map(([name, bytes]) => ({ name, bytes }));
  };
  const open = async (files: File[]) => {
    await saveProject(displayName(), files, currentId!);
    if (disposed) return;
    clean = new Map(files.map(file => [file.name, file.bytes.slice()])); changedFiles.clear();
    nativeFiles?.SetWriteListener(null); unmountFiles?.();
    nativeFiles = new MEMORY_FILESYSTEM();
    for (const file of files) nativeFiles.Write(file.name, file.bytes);
    const name = files.find(file => /\.kicad_pro$/i.test(file.name))?.name.split('/').pop()?.replace(/\.kicad_pro$/i, '') || 'Project';
    nativeRoot = '/' + name + '/';
    // Keep files available while the editors replace their own temporary mounts.
    unmountFiles = wxMountFileSystem('/' + name, nativeFiles);
    nativeFiles.SetWriteListener(nativeWrite);
    generation++;
    hooks.open(files);
  };
  const connected = (event: MessageEvent) => {
    if (disposed || port || event.source !== window.parent || event.origin !== parentOrigin
      || event.data?.ziro !== 1 || event.data.type !== 'connect' || event.data.challenge !== challenge || event.ports.length !== 1) return;
    port = event.ports[0]!;
    port.onmessage = async event => {
      const data = event.data;
      if (disposed || data?.ziro !== 1) return;
      if (data.reply) {
        const waiter = waiting.get(data.reply);
        if (waiter) { waiting.delete(data.reply); clearTimeout(waiter.timer); data.ok ? waiter.resolve() : waiter.reject(new Error('ziro.save_failed')); }
        return;
      }
      const respond = (payload: unknown = {}) => { if (!disposed) port?.postMessage({ ziro: 1, reply: data.id, payload }); };
      try {
        switch (data.type) {
          case 'new': {
            projectKey = ''; draftKey = String(data.payload.draftKey);
            currentId = 'aurago-new:' + draftKey;
            const name = String(data.payload.name);
            const files = newProjectFiles(name).map(file => ({ name: file.name.slice(name.length + 1), bytes: file.bytes ?? encoder.encode(file.text) }));
            await open(files); mark(true); respond(); break;
          }
          case 'open': {
            const key = String(data.payload.projectKey), windowKey = String(data.payload.draftKey);
            const id = 'aurago:' + key + ':' + windowKey;
            const saved = await loadProject(id);
            let recover = false;
            try { recover = localStorage.getItem('aurago-ziro-draft:' + id) === '1'; } catch { /* no stored draft */ }
            if (recover && saved) {
              incoming = { key, windowKey, id, files: data.payload.files, draft: saved.files };
              respond({ draft: true }); break;
            }
            projectKey = key; draftKey = windowKey; currentId = id;
            await open(data.payload.files); mark(false); respond({ draft: false }); break;
          }
          case 'draft': {
            const windowKey = String(data.payload.draftKey), id = 'aurago-new:' + windowKey;
            const saved = await loadProject(id);
            let recover = false;
            try { recover = localStorage.getItem('aurago-ziro-draft:' + id) === '1'; } catch { /* no stored draft */ }
            if (recover && saved) incoming = { key: '', windowKey, id, files: [], draft: saved.files };
            respond({ draft: !!incoming }); break;
          }
          case 'recover':
            if (incoming) {
              projectKey = incoming.key; draftKey = incoming.windowKey; currentId = incoming.id;
              await open(incoming.draft); mark(true); incoming = null;
            } respond(); break;
          case 'discard':
            if (incoming) {
              projectKey = incoming.key; draftKey = incoming.windowKey; currentId = incoming.id;
              await open(incoming.files); incoming = null;
            }
            mark(false); respond(); break;
          case 'cancel-open': incoming = null; respond(); break;
          case 'snapshot': {
            // Snapshot every editor before returning bytes, including pending debounce writes.
            const files = snapshot();
            if (currentId) await saveProject(displayName(), files, currentId);
            respond({ files, generation }); break;
          }
          case 'saved': {
            if (data.payload.projectKey !== projectKey) {
              mark(false);
              projectKey = String(data.payload.projectKey);
              currentId = 'aurago:' + projectKey + ':' + draftKey;
            }
            const files = snapshot();
            await saveProject(displayName(), files, currentId!);
            mark(data.payload.generation !== generation);
            respond(); break;
          }
          default: throw new Error('ziro.invalid_project');
        }
      } catch (error) {
        if (!disposed) port?.postMessage({ ziro: 1, reply: data.id, error: error instanceof Error ? error.message : 'ziro.save_failed' });
      }
    };
  };
  const ready = () => setTimeout(() => {
    if (!disposed) window.parent.postMessage({ ziro: 1, type: 'ready', challenge }, parentOrigin!);
  }, 0);
  window.addEventListener('message', connected);
  if (document.readyState === 'complete') ready(); else window.addEventListener('load', ready, { once: true });
  const key = (event: KeyboardEvent) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault(); event.stopImmediatePropagation(); void auraGoSave().catch(() => {});
    }
  };
  window.addEventListener('keydown', key, true);
  return () => {
    disposed = true; port?.close(); port = null;
    nativeFiles?.SetWriteListener(null); unmountFiles?.(); nativeFiles = null; unmountFiles = null;
    nativeRoot = ''; Reflect.deleteProperty(globalThis, '__auragoZiroProjectRead');
    window.removeEventListener('message', connected); window.removeEventListener('load', ready); window.removeEventListener('keydown', key, true);
    for (const waiter of waiting.values()) { clearTimeout(waiter.timer); waiter.reject(new Error('ziro.disconnected')); }
    waiting.clear();
  };
}
