# Desktop HTTP and SDK contract

Desktop authorization combines the authenticated browser session or token scope,
the current Desktop policy, and the integration's own permissions. An app's SDK
manifest permissions are additional requirements. None overrides readonly.

## Readonly and invocation lifetime

`virtual_desktop.readonly` permits reads, previews and passive playback. It blocks
file changes, settings changes, new controlling SSH/VNC sessions, agent runs and
paid generation. SFTP listing, stat and download remain reads. Stop, cancel and
cleanup remain available to an authenticated caller with the required scope.

A denied mutation returns HTTP 403 with `code: "desktop_readonly"`. Activating
readonly cancels Desktop-owned requests, background runs and controlling
connections. A cancelled generation cannot publish files or late results after
readonly is disabled again. The policy update preserves open read clients and
the Tresor browser binding and encrypted local drafts.

Shared integrations have explicit server-owned entries beneath
`/api/desktop/integrations/`. They retain their original administrative and
integration checks. A client header does not establish Desktop ownership.
Queued local missions and their local followups retain an ephemeral owner through
execution; they are discarded on revocation or process restart. Persistent queue
markers also prevent recovery from an older queued/running status after a crash
between state saves. Scheduled/admin missions have their
own lifecycle. Remote missions cannot be started through this Desktop entry
because their protocol has no acknowledged cancellation.

HTTP bootstrap, WebSocket welcome and later management events use the same scope
projection. `desktop:read` does not expose administrative settings/providers or
grant SDK writes. The obsolete `control_level` field is absent from bootstrap and
configuration UI. Legacy YAML still loads; a normal save removes that key.

## Conditional file publication

`GET /api/desktop/file?path=Documents/example.txt` returns `content`, `entry` and
`version`, with the same strong `ETag` response header. HEAD also returns the
version for binary files. Keep the version with the particular editor's bytes.

| Operation | Preconditions |
| --- | --- |
| PUT `/api/desktop/file` | `If-Match: <observed ETag>` for replacement; `If-None-Match: *` for creation |
| POST `/api/pixel/save` | Same conditions; path must be workspace-relative and match decoded PNG/JPEG bytes |
| POST `/api/desktop/upload` | Conditions apply to the destination; `unique=1` uses a create-only distinct filename |
| PATCH `/api/desktop/file` | Conditions apply to `new_path`; body contains `old_path` and `new_path` |
| POST `/api/desktop/copy` | Conditions apply to `dest_path` |

Missing conditions return 428 (`file_precondition_required`); stale versions or
occupied create-only targets return 412 (`file_conflict`). The JSON `conflict`
object carries the destination `path` and, for a regular file, its current
`version`. A directory target returns 409 (`directory_conflict`); directory merge
or replacement is not implicit. `If-Match: *` and weak/list ETags are unsupported.

The shell shows Replace, Keep a copy, and Cancel. Replace retries with the version
shown in the conflict; another intervening write causes another conflict. Copy
uses a fresh create-only destination. Directory replacement is unavailable;
choose another name. Never automatically retry an uncertain mutation. The server
checks the destination and publishes under one mutation lock; failed publication
retains the previous bytes. Interrupted multipart uploads never replace them.

## SDK file access

Load `/js/desktop/aura-desktop-sdk.js` and declare `files:read`/`files:write` in the
installed app manifest. The parent bridge checks these permissions. Each iframe
keeps its own observed versions; one app's read cannot acknowledge another app's
unsaved content.

```javascript
const file = await AuraDesktop.fs.read('Documents/example.txt');
const saved = await AuraDesktop.request('fs:write', {
    path: file.entry.path,
    content: editedText,
    version: file.version
});
// Keep the returned path: the user may have chosen a distinct copy.
currentPath = saved.path;
```

`AuraDesktop.fs.write(path, content)` uses this iframe's last observed version or
create-only semantics when it has not read the file. It also accepts an explicit
version as its third argument: `AuraDesktop.fs.write(path, content, file.version)`.
A successful write advances only that iframe's observed version, so sequential
writes work while another iframe with an older version receives the normal 412
conflict. The parent owns the shared conflict dialog. Handle rejection/cancellation
without another write. File dialogs, Editor, Pixel and uploads use the same conflict
decisions.

## SDK document channel

The shell grants the SDK channel only to the rooted Apps/Widgets HTML entry it
launches. A random capability is carried in that document's URL fragment, removed
before app scripts run, then checked with a per-load challenge over a dedicated
`MessageChannel`. The sandboxed document keeps its opaque `null` origin; privileged
requests and responses use the bound port, not `window.postMessage` to a reusable
iframe `WindowProxy`. Navigating the frame or closing it revokes its channel and
cancels pending SDK work. A shell-managed app/widget reload creates a fresh document
and channel; an arbitrary in-frame navigation does not inherit the SDK grant.

This transport does not change manifest permissions or add a new permission gate.
Existing SDK v1 context and clipboard behavior remains available to valid apps, and
`files:read`/`files:write` are still checked by the parent for file operations.

## Archives, Notes and printing

Archive HTML/HTM/JS/MJS entries are plaintext with `nosniff`. SVG responses carry a
restrictive HTTP sandbox CSP that also applies to direct navigation. PNG/text,
HEAD and byte-range reads remain available.

Notes trash operations use `Trash/Notes/<uuid>/<original Notes subpath>` and
restore that subpath. Batches reject Notes roots/ancestors and overlapping
selections before moving anything. Old ordinary Trash files keep their original
interpretation. Native agent mutation remains blocked for protected Notes trash.

Notes, Writer, Sheets and Viewer use the shared `AuraDesktopPrint` helper. It
creates a frame with `sandbox="allow-same-origin allow-modals"` before attaching
it, preserves sanitization, waits for images/fonts and invokes print from the
parent. Frame scripts remain disabled. Native printer and hardware acceptance
are separate from browser fixture tests.

## Session compatibility

Session v2 adds stable window keys and an active-window reference without
changing the existing version. Old snapshots fall back to their highest-z
visible window. Compact mode preserves the logical active space. Layout clamping
prioritizes reachability when the viewport is smaller than an app's minimum size.
