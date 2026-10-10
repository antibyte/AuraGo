type Executor = (action: string, params: Record<string, any>) => Promise<unknown>;
let executor: Executor | null = null;
let loader: Executor | null = null;
let port: MessagePort | null = null;
let capture: ((file: { name: string; bytes: ArrayBuffer; type: string }) => void) | null = null;
let readonly = false;
let attached = false;
let loaded: (() => void) | null = null;
export function waitForEditor() { return new Promise<void>(resolve => { loaded = resolve; }); }
export const isReadOnly = () => readonly;
export function notify(event: string, data: unknown = null) {
  port?.postMessage({ type: "aurago.layerling.event", event, data });
}
export function setLoader(value: Executor) { loader = value; }
export function bindEditor(value: Executor) {
  executor = value;
  if (loaded) { const resolve = loaded; loaded = null; resolve(); }
  if (!attached && typeof window !== "undefined") {
    attached = true;
    const channel = (window as any).__AURAGO_DESKTOP_SDK_CHANNEL__;
    channel?.onPort((next: MessagePort) => {
      port = next;
      let pending = Promise.resolve();
      next.addEventListener("message", event => {
        const msg = event.data;
        if (msg?.type !== "aurago.layerling.request") return;
        pending = pending.then(async () => {
          try {
            if (msg.action === "policy") readonly = !!msg.params.readonly;
            const handler = msg.action === "load" || msg.action === "init" ? loader : executor;
            if (msg.action !== "policy" && !handler) throw new Error("Editor is not ready");
            const data = msg.action === "policy" ? {} : await handler!(msg.action, msg.params || {});
            next.postMessage({ type: "aurago.layerling.response", id: msg.id, ok: true, data });
          } catch (error) {
            next.postMessage({ type: "aurago.layerling.response", id: msg.id, ok: false, error: error instanceof Error ? error.message : "Layerling command failed" });
          }
        });
      });
      next.start();
      notify("ready");
    });
  } else if (port) notify("ready");
  return () => { if (executor === value) executor = null; };
}
export async function deliverFile(name: string, blob: Blob) {
  const file = { name, bytes: await blob.arrayBuffer(), type: blob.type };
  if (capture) { capture(file); return; }
  notify("export", file);
}
export async function captureFile(run: () => unknown): Promise<unknown> {
  if (capture) throw new Error("An export is already running");
  let timer: ReturnType<typeof setTimeout>;
  try {
    return await new Promise((resolve, reject) => {
      let delivered = false;
      capture = file => { delivered = true; resolve(file); };
      timer = setTimeout(() => reject(new Error("Layerling export did not complete")), 90000);
      Promise.resolve().then(run).then(() => { if(!delivered) reject(new Error("Layerling export did not complete; check the editor's message")); }).catch(reject);
    });
  } finally { clearTimeout(timer!); capture = null; }
}
