// SPDX-License-Identifier: GPL-3.0-or-later
import { readFile, writeFile, copyFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const source = path.resolve(process.argv[2] || '.');
const adapter = path.dirname(fileURLToPath(import.meta.url));
async function change(file, replacements) {
    let text = (await readFile(path.join(source, file), 'utf8')).replaceAll('\r\n', '\n');
    for (const [before, after, all = false] of replacements) {
        if (!text.includes(before)) throw new Error('Pinned upstream changed: ' + file + ': ' + before.slice(0, 70));
        text = all ? text.replaceAll(before, after) : text.replace(before, after);
    }
    await writeFile(path.join(source, file), text);
}
await copyFile(path.join(adapter, 'aurago_bridge.ts'), path.join(source, 'designer/src/aurago_bridge.ts'));
await change('designer/src/App.tsx', [
    ["import { OUTPUT_DOWNLOADER }", "import { flushSync } from 'react-dom';\nimport { installAuraGoBridge, auraGoProjectId, auraGoChanged, auraGoSave, auraGoEmbedded } from './aurago_bridge.js';\nimport { OUTPUT_DOWNLOADER }"],
    [': { name: f.name, text: dec.decode(f.bytes) };', ': { name: f.name, text: dec.decode(f.bytes), bytes: f.bytes };'],
    ['files.map((f) => ({ name: f.name, bytes: enc.encode(f.text) }))', 'files.map((f) => ({ name: f.name, bytes: f.bytes ?? enc.encode(f.text) }))', true],
    ['const cur = projectFilesRef.current;', 'const cur = projectFilesRef.current;\n    const storeId = auraGoProjectId();', true],
    ['.find((p) => p.name === projectNameOf(cur))', '.find((p) => storeId ? p.id === storeId : p.name === projectNameOf(cur))', true],
    ['if (fresh.size === 0) return;', 'if (fresh.size === 0) return;\n      auraGoChanged([...fresh].map(([name, text]) => ({ name, text })));'],
    ['const full = fullByBase.get(pcbBasename(f.name));', 'const full = cur.some(file => file.name === f.name) ? f.name : fullByBase.get(pcbBasename(f.name));'],
    ['const flushEditors = useCallback(() => {', 'const flushEditors = useCallback((strict = false) => {'],
    ["console.warn('Editor flush failed:', e);", "if (strict) throw e;\n        console.warn('Editor flush failed:', e);"],
    ["if (loaded) await recordSnapshot(rec.id, loaded.files, 'save', rec.name);", "if (loaded) await recordSnapshot(rec.id, loaded.files, 'save', rec.name);\n      auraGoChanged(files);\n      await auraGoSave();"],
    ['/* storage disabled */', 'if (auraGoEmbedded) throw new Error("ziro.save_failed");\n      /* storage disabled */'],
    ['const persistFilesNow = useCallback((files: PickedFile[]) => {', 'const persistFilesNow = useCallback((files: PickedFile[]) => {\n    auraGoChanged(files);'],
    ['<ProjectSyncProvider projectName={projectName}', '<ProjectSyncProvider projectName={auraGoEmbedded ? auraGoProjectId() : projectName}'],
    ["view === 'home' ? warmFrames(mountFor)", "view === 'home' && !auraGoEmbedded ? warmFrames(mountFor)"],
    ['if (shown) last.current = children;', 'if (shown || auraGoEmbedded) last.current = children;'],
    ['<Frozen shown={', '<Frozen key={openNonce} shown={', true],
    ['const key = JSON.stringify(route);', 'if (auraGoEmbedded) { setRestoring(false); return; }\n    const key = JSON.stringify(route);'],
    ['if (restoring) return;', 'if (restoring || auraGoEmbedded) return;', true],
    ["const registerPcbFlush = useMemo(() => registerFlushFor('pcb'), [registerFlushFor]);", "const registerPcbFlush = useMemo(() => registerFlushFor('pcb'), [registerFlushFor]);\n  const registerSymbolFlush = useMemo(() => registerFlushFor('symbols'), [registerFlushFor]);\n  const registerFootprintFlush = useMemo(() => registerFlushFor('footprints'), [registerFlushFor]);"],
    ['<SymbolEditor\n                onExitToHome', '<SymbolEditor\n                registerAutosaveFlush={auraGoEmbedded ? registerSymbolFlush : undefined}\n                onExitToHome'],
    ['<FootprintEditor\n                onExitToHome', '<FootprintEditor\n                registerAutosaveFlush={auraGoEmbedded ? registerFootprintFlush : undefined}\n                onExitToHome'],
    ["setView('schematic');\n  }, []);\n\n  const showSymbolEditor", "if (!auraGoEmbedded) setView('schematic');\n  }, []);\n\n  const showSymbolEditor"],
    ["  // The views without an editor frame", `  useEffect(() => installAuraGoBridge({
    open: (files) => {
      clearTimeout(saveTimer.current); pendingWrite.current.clear();
      editorFlush.current.clear();
      setSymFromSchematic(null); setEditedSymbol(null); editTargetId.current = null;
      openProjectFiles(openedFromStored(files));
      projectFilesRef.current = openedFromStored(files);
      setStandalonePcb(null); setStartFile(null); setDemoProject(false);
      setSchMounted(true); setView('schematic');
    },
    snapshot: () => {
      flushSync(() => flushEditors(true));
      flushSync(() => flushEditors(true));
      const files = new Map((projectFilesRef.current || []).map(file => [file.name, file.bytes || enc.encode(file.text)]));
      for (const [name, text] of liveEdits.current) files.set(name, enc.encode(text));
      for (const [name, bytes] of pendingWrite.current) files.set(name, bytes);
      return [...files].map(([name, bytes]) => ({ name, bytes }));
    },
  }), [openProjectFiles, flushEditors]);

  // The views without an editor frame`],
    ['  return (\n    <ProjectSyncProvider', `  if (auraGoEmbedded && view === 'home') manager = <div style={{ padding: 24 }}>
    <h2>Ziro Designer</h2>
    <p>Open or create a Shared project in the AuraGo toolbar.</p>
    {projectFiles?.length ? <><button onClick={showSchematic}>Schematic Editor</button>{' '}
    <button onClick={showPcb}>PCB Editor</button>{' '}
    <button onClick={showSymbolEditor}>Symbol Editor</button>{' '}
    <button onClick={showFootprintEditor}>Footprint Editor</button>{' '}</> : null}
    <button onClick={showCalculator}>PCB Calculator</button>
  </div>;
  return (
    <ProjectSyncProvider`],
]);
await change('designer/src/editors/footprint/footprint_edit_frame_app.tsx', [
    ['export function FootprintEditor(props: {', 'export function FootprintEditor(props: {\n  registerAutosaveFlush?: ((fn: (() => void) | null) => void) | undefined;'],
]);
await change('external/wx/filefn.ts', [
    ['export function wxFileExists(aPath: string): boolean {', `export function wxFileExists(aPath: string): boolean {
  const reader = Reflect.get(globalThis, '__auragoZiroProjectRead');
  if (typeof reader === 'function') {
    const bytes: Uint8Array | null | undefined = reader(wxNormalizePath(aPath));
    if (bytes !== undefined) return bytes !== null;
  }`],
    ['export function wxReadFileSync(aPath: string): Uint8Array | null {', `export function wxReadFileSync(aPath: string): Uint8Array | null {
  const reader = Reflect.get(globalThis, '__auragoZiroProjectRead');
  if (typeof reader === 'function') {
    const bytes: Uint8Array | null | undefined = reader(wxNormalizePath(aPath));
    if (bytes !== undefined) return bytes;
  }`],
]);
await change('eeschema/symbol_editor/symbol_edit_frame_ui.tsx', [
    ['  onWriteProjectFiles,\n}: {', '  onWriteProjectFiles,\n  registerAutosaveFlush,\n}: {'],
    ['  app: SYMBOL_EDIT_FRAME_APP;', '  registerAutosaveFlush?: ((fn: (() => void) | null) => void) | undefined;\n  app: SYMBOL_EDIT_FRAME_APP;'],
    ['  useUnsavedGuard(manager.current.hasModifications());', `  useEffect(() => {
    if (!registerAutosaveFlush) return;
    const flush = () => {
      let wrote = false;
      if (editingSchematicSymbol && modified && workSymbol && onSaveToSchematic) {
        onSaveToSchematic(workSymbol);
        manager.current.saveLibraryText(SCHEMATIC_LIB); wrote = true;
      }
      for (const name of manager.current.libraryNames()) {
        if (name === SCHEMATIC_LIB || !manager.current.isLibraryModified(name)) continue;
        const lib = manager.current.library(name);
        if (lib?.scope !== 'project' || !lib.fileName || !onWriteProjectFiles) throw new Error('ziro.save_failed');
        const text = manager.current.saveLibraryText(name);
        if (text === undefined) throw new Error('ziro.save_failed');
        if (projectSymLibTable(initialProject ?? []).some(row => row.name === name)) onWriteProjectFiles([{ name: lib.fileName, text }]);
        else {
          const added = AddSymbolLibraryToProject(initialProject ?? [], { name: lib.fileName, text });
          if (!added.ok) throw new Error(added.error);
          onWriteProjectFiles(added.files);
        }
        wrote = true;
      }
      if (wrote) bump();
    };
    registerAutosaveFlush(flush);
    try { flush(); } catch { /* Shared Save reports a library that needs Save As. */ }
    return () => registerAutosaveFlush(null);
  }, [registerAutosaveFlush, revision, modified, workSymbol, editingSchematicSymbol, onSaveToSchematic, onWriteProjectFiles, initialProject, bump]);
  useUnsavedGuard(manager.current.hasModifications());`],
]);
await change('pcbnew/footprint_edit_frame_ui.tsx', [
    ['  onWriteProjectFiles,\n}: {', '  onWriteProjectFiles,\n  registerAutosaveFlush,\n}: {'],
    ['  app: FOOTPRINT_EDIT_FRAME_APP;', '  registerAutosaveFlush?: ((fn: (() => void) | null) => void) | undefined;\n  app: FOOTPRINT_EDIT_FRAME_APP;'],
    ['  FOOTPRINT_LIBRARY_STORE,', '  FOOTPRINT_LIBRARY_STORE,\n  SAVE_T,'],
    ['  useUnsavedGuard(modified);', `  useEffect(() => {
    if (!registerAutosaveFlush) return;
    const flush = () => {
      if (!frame.IsContentModified()) return;
      const footprint = frame.GetBoard()?.GetFirstFootprint();
      if (!footprint || !writeRef.current || store.SaveFootprint(footprint.GetFPID().GetLibNickname(), footprint) !== SAVE_T.SAVE_OK) throw new Error('ziro.save_failed');
      frame.GetScreen()?.SetContentModified(false);
      repaint();
    };
    registerAutosaveFlush(flush);
    try { flush(); } catch { /* Shared Save reports a footprint that needs Save As. */ }
    return () => registerAutosaveFlush(null);
  }, [registerAutosaveFlush, modified, frame, store]);
  useUnsavedGuard(modified);`],
]);
await change('eeschema/sch_edit_frame_ui.tsx', [
    ['return wxMountFileSystem(`/${name}`, fs);', 'const unmount = wxMountFileSystem(`/${name}`, fs);\n    Pgm().GetLibraryManager().LoadProjectTables(`/${name}`);\n    return unmount;'],
    ['docs.set(base, { ...readSchematic(parse(f.text)), fileName: base });', 'docs.set(f.name, { ...readSchematic(parse(f.text)), fileName: f.name });'],
    ["const startBase = startFile?.split('/').pop()?.split('\\\\').pop();", "const startBase = startFile?.replaceAll('\\\\', '/');"],
]);
await change('eeschema/project.ts', [
    ['export function sheetFile(sheet: SchSheet): string {', "export function sheetFile(sheet: SchSheet, parentFile = ''): string {"],
    ["  const idx = Math.max(raw.lastIndexOf('/'), raw.lastIndexOf('\\\\'));\n  return idx === -1 ? raw : raw.slice(idx + 1);", `  const path = raw.replaceAll('\\\\', '/');
  if (!path || path.startsWith('/') || path.includes(':')) return '';
  const dir = parentFile.slice(0, parentFile.lastIndexOf('/') + 1);
  const parts: string[] = [];
  for (const part of (dir + path).split('/')) {
    if (part === '..') { if (!parts.length) return ''; parts.pop(); }
    else if (part && part !== '.') parts.push(part);
  }
  return parts.join('/');`],
    ['const child = sheetFile(sh);', 'const child = sheetFile(sh, file);'],
    ['for (const doc of docs.values()) {\n    for (const sh of doc.sheets) referenced.add(sheetFile(sh));', 'for (const [file, doc] of docs) {\n    for (const sh of doc.sheets) referenced.add(sheetFile(sh, file));'],
]);
await change('eeschema/libraries/symbol_library_adapter.ts', [
    ['if (!mgr) return [];\n\n  const entries:', 'if (!mgr) return [];\n  mgr.LoadProjectTables(PgmOrNull()!.GetSettingsManager().Prj().GetProjectDirectory(), [LIBRARY_TABLE_TYPE.SYMBOL]);\n\n  const entries:'],
]);
await change('designer/src/libraryHosts.ts', [
    ["'https://pub-ac941e05e1284f409be2ed74ddb151b3.r2.dev'", "'/local-libraries'"],
    ['`${LIBRARY_HOST}/symbols`', "'/symbols'", true], ['`${LIBRARY_HOST}/footprints`', "'/footprints'", true],
    ['|| LIBRARY_HOST', "|| '/models3d'"], ['`${LIBRARY_HOST}/demos`', "'/demos'", true],
]);
await writeFile(path.join(source, 'ai/ai_flag.ts'), '// SPDX-License-Identifier: GPL-3.0-or-later\n// Copyright (C) 2026 ZiroEDA and contributors.\n// AuraGo offline image does not run an AI service.\nexport function aiEnabled(): boolean { return false; }\nexport const AI_AGENT_URL = "";\n');
await writeFile(path.join(source, 'designer/.env.production'), 'VITE_SUPABASE_URL=\nVITE_SUPABASE_ANON_KEY=\nVITE_SENTRY_DSN=\nVITE_CHAT_WIDGET_URL=\nVITE_MODELS3D_URL=/models3d\nVITE_SYMBOLS_URL=/symbols\nVITE_FOOTPRINTS_URL=/footprints\nVITE_DEMOS_URL=/demos\n');
