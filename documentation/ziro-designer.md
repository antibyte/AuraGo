# Ziro Designer

Ziro Designer is an optional Software Store application for KiCad-compatible
schematic and PCB editing. Install it through the Virtual Desktop Software Store.
Docker is required; Node and pnpm are needed only when building the image.

## Projects

Use the AuraGo toolbar to create or open a project. Open accepts a `.kicad_pro`
inside a project folder below `Shared`. The default Save As destination is
`Shared/Ziro Designer/<name>/`. Save As creates a separate folder and preserves
the project's filenames and internal references. Use the File Manager's Open
With action for existing projects. If Ziro is absent, that action opens the Store.
Once a project is open, New and Open launch another Desktop window. A window
keeps its own editor instance and draft throughout that project's lifetime.

Save or Ctrl+S publishes the current files of every mounted editor. Browser
autosave is draft recovery only. The Shared status confirms server publication;
the upstream local-save indicator alone does not. Unsaved window closure offers
Save, Discard and Cancel. Session restore keeps each window's draft identity.
Recovery is explicit and never overwrites Shared without Save.
The upstream browser live-sync channel is isolated per Desktop window; it cannot
merge another window's edits or mix unrelated projects with identical names.

Files are published individually with their observed ETags, or create-only when
new. A conflict offers Replace, a complete project copy, or Cancel. Each completed
file advances only this window's version; a partial failure retains the remaining
draft and completed progress. There is no transaction across an entire project.
Files absent from the editor snapshot are not deleted. Manage project folders
and deletions in the File Manager. Exports use Ziro's regular downloads.

The adapter accepts at most 200 MiB and 2000 project files, additionally subject
to AuraGo's configured per-file limit. Relative paths cannot escape the selected
Shared folder. Project files, subfolders and binary assets retain their bytes.
All server calls retain Desktop authentication, live readonly and publication
revocation. The container mounts neither Shared nor the Docker socket.

## Local operation and updates

The image builds Ziro commit `7b4597005e4801d4fb19661fcac6fc3380547cf2` plus the
reviewed adapter. It serves static files on port 8080 as UID 1000. Libraries,
fonts, workers and WASM come from the image or the selected project's files.
Only the bundled KiCad library subset is included. Missing library entries must
be supplied with the project. Supabase, telemetry, support chat, external AI and
online project import are disabled, with CSP denying external runtime requests.

The Desktop uses the existing Store address on a separate origin. The bridge is
bound to that origin, the specific iframe document and a random one-use challenge
using MessageChannel. Navigation and disposal revoke access and pending work.
The upstream manager is replaced by editor launch buttons in embedded mode;
the parent toolbar owns project opening, creation and Shared save decisions.
Narrow windows scroll the editor at its usable minimum size.

Build locally:

```sh
docker build -t aurago-ziro-designer:ziro-7b45970-1 internal/desktopstore/ziro_designer_assets
node scripts/test-ziro.mjs
node scripts/test-ziro-bridge.mjs
```

For real-browser acceptance, start the reviewed image with
`-p 127.0.0.1:14174:8080`, set `ZIRO_TEST_URL` if a different port is used,
and run `node scripts/test-ziro-browser.mjs`. Use an installed Playwright package
(`AURAGO_PLAYWRIGHT_PATH`) and browser (`AURAGO_BROWSER_PATH`) if needed.
Set `ZIRO_TEST_CONTAINER` to that disposable test container's name to include
an actual container restart before reopening the saved project.

The Docker publication workflow accepts `image=ziro-designer` and builds
linux/amd64 and linux/arm64 with provenance, SBOM and digest signing. A local
build does not publish the GHCR catalog reference. Publish the image before
installing on a server; installed apps change through Store Update. Restart,
update and uninstall preserve Shared projects, including Delete app data.

The separate GPL-3.0-or-later image serves `/LICENSE`, `/NOTICE.md` and
`/source.tar.gz`. The source archive includes the pinned source and AuraGo
adapter, Dockerfile and locked dependency references. KiCad library attribution
and the library exception remain in the upstream notice. AuraGo wrapper labels
are translated in all sixteen locales; the editor retains its upstream language.
