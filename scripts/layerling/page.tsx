"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { LayerlingEditor } from "@/components/LayerlingEditor";
import { importLylProject } from "@/lib/lylProject";
import { setLanguage } from "@/lib/i18n";
import { notify, setLoader, isReadOnly, waitForEditor } from "@/lib/auragoBridge";
const empty: [] = [];

export default function AuraGoEditor() {
  const [project, setProject] = useState<Awaited<ReturnType<typeof importLylProject>> | null>(null);
  const [revision, setRevision] = useState(0);
  const [name, setName] = useState("Untitled");
  const [theme, setTheme] = useState<"light" | "dark">("dark");
  const [createdAt, setCreatedAt] = useState(() => Date.now());
  const workspaceSnapshot = useRef({ projectId: "", fingerprint: "" });
  const load = useCallback(async (bytes?: ArrayBuffer) => {
    const restored = bytes ? await importLylProject(bytes) : null;
    const ready = waitForEditor();
    setCreatedAt(restored?.createdAt || Date.now());
    setProject(restored); setName(restored?.projectName || "Untitled"); setRevision(value => value + 1);
    await ready;
    return { loaded: true };
  }, []);
  useEffect(() => {
    setLoader(async (action, params) => {
      if (action === "init") {
        setLanguage(params.language === "de" ? "de" : "en", false);
        setTheme(params.theme === "light" ? "light" : "dark");
        document.documentElement.dataset.theme = params.theme === "light" ? "light" : "dark";
        document.documentElement.lang = params.language === "de" ? "de" : "en";
        return { initialized: true };
      }
      return load(params.bytes);
    });
  }, [load]);
  const changed = useCallback(() => notify("changed"), []);
  const workspaceChanged = useCallback((snapshot: unknown) => {
    const value = snapshot as { projectId: string };
    const fingerprint = JSON.stringify(snapshot);
    const previous = workspaceSnapshot.current;
    workspaceSnapshot.current = { projectId: value.projectId, fingerprint };
    // Upstream emits the initial workspace after mounting and again after hydration.
    if (previous.projectId === value.projectId && previous.fingerprint !== fingerprint) changed();
  }, [changed]);
  return <main style={{height:"100vh"}} onKeyDownCapture={event => { if(isReadOnly()) event.stopPropagation(); }} onPointerDownCapture={event => { if(isReadOnly()) { event.preventDefault(); event.stopPropagation(); } }}>
    <LayerlingEditor key={revision} projectId={"aurago-" + revision} projectName={name}
      projectCreatedAt={createdAt} projectModifiedAt={project?.modifiedAt || createdAt} initialShapes={project?.shapes || empty} initialHistory={project?.history}
      initialHistoryIndex={project?.historyIndex} initialAssets={project?.assets || empty} initialWorkspace={project?.workspace}
      initialSnap={project?.snapGrid} initialPlacementElevation={project?.placementElevation}
      initialPlacementWorkplane={project?.placementWorkplane}
      onProjectShapesChange={changed} onProjectWorkspaceChange={workspaceChanged}
      onProjectNameChange={value => { setName(value); changed(); }}
      onOpenLylProjectFile={async file => { notify("open", {name:file.name,bytes:await file.arrayBuffer()}); }}
      onHome={() => notify("open")} themePreference={theme} resolvedTheme={theme} />
  </main>;
}
