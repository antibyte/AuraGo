# WebDAV and Koofr in the Desktop File Manager

Enable the integration, save its credentials in the Vault, then enable **Show
in File Manager**. `webdav.show_in_filemanager` and `koofr.show_in_filemanager`
both default to `false`. Changes take effect without a restart. Cloud locations
are available to Desktop administrators.

WebDAV opens at the configured base URL. Expanding Koofr loads accessible mounts,
including connected storage services, with their names, availability and write
permissions. Opening a location loads its directory. Displaying the File Manager
does not probe either provider. Offline mounts remain visible but disabled.

The existing list/grid views, sorting, filtering, properties, previews,
downloads, uploads, new files/folders, rename, duplicate, copy, move, delete,
keyboard shortcuts and drag-and-drop work with these locations. Hold Ctrl while
dropping files to copy them. Clipboard entries, tabs, history, favorites and
window context retain the provider and Koofr mount separately from the relative
path. Older local path calls and favorites remain supported.

Previews are read-only. Copy a cloud file to the Desktop workspace before editing
it in Writer, Sheets or another local app. Cloud archives, synchronization,
operating-system mounts and saving directly back from Desktop editors are not
provided. Local Notes and managed widget HTML retain their dedicated validating
write paths; a cloud transfer cannot bypass those protections.

## Transfers and conflicts

Transfers run sequentially on the server and stream progress to their window.
Cancel, closing that window, or revoking its grant cancels the request. Each
operation observes the configured Desktop file-size limit, at most 10,000
entries, 64 directory levels and 512 MiB of file content. Remote writes stage one
bounded file privately before uploading; local writes use rooted, conditional
Desktop publication. Temporary files are removed when the operation ends.

Existing destinations are conflicts. **Replace** is offered only for files with
an observed strong version: WebDAV ETags, Koofr hashes, or local content hashes.
The provider or local publication rechecks that version. **Keep copy** chooses a
new name and retries a create-only operation. Existing directories are never
silently merged; use Keep copy for those conflicts.

A move across locations copies the complete source tree, verifies sizes and
SHA-256 content, rechecks the result, and only then attempts conditional source
deletion. Failed or uncertain copies never start source cleanup. Changed or
unversioned source files remain in place. Koofr supports conditional deletion of
empty folders; WebDAV has no equivalent atomic condition, so empty WebDAV source
folders are retained after a cross-location move. The File Manager explicitly
reports retained sources and incomplete transfers. Already created destination
files may remain after cancellation or failure; this is not a multi-file rollback.

Deleting cloud items asks for confirmation naming their storage location.
AuraGo provides no trash or undo for this action. Root deletion and transfers
into the source's own subtree are rejected. Desktop readonly, integration
readonly, visibility/activation and Koofr mount permissions all apply. Revoking
an integration or Desktop grant cancels in-flight storage requests. New requests
recheck current policy; provider permissions are checked before each operation.

## Implementation and verification

Admin-only routes under `/api/desktop/storage/` provide `sources`, `mounts`,
`files`, `info`, `content`, `upload`, `directory`, `entry` and `transfer`.
Locations use `{provider, mount_id, path}`. Credentials never reach the browser.
WebDAV filenames come from origin- and root-validated `href` values, never display
names. Credentialed redirects stay on the configured origin. Existing agent
tool schemas and formatted outputs are unchanged; binary transfers use separate
typed provider methods.

Koofr wire contracts follow its official [mount metadata](https://github.com/koofr/go-koofrclient/blob/master/api_scheme.go)
and [file operations](https://github.com/koofr/go-koofrclient/blob/master/client_files.go),
including `autorename=false`, `overwriteIfHash`, `removeIfHash` and `removeIfEmpty`.

Focused suites: `TestCloudStorage*`, `TestDesktopStorage*`,
`TestCloudFileManagerVisibilityRoundTrip`, and `TestDesktopCloudStorageBrowser`.
The browser suite covers Standard/Fruity at 1280 and 430 pixels, configuration
toggles, lazy mounts, navigation, tabs/history, sibling windows, clipboard,
conflicts, drag-and-drop, previews, cancellation and live revocation. Existing
WebDAV/Koofr agent tests, Desktop conditional streams and File Manager instance
and clipboard regressions remain part of validation. These fixtures do not
constitute live-provider or deployment acceptance.
