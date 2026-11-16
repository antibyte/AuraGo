# Everyday Desktop controls

The browser Desktop keeps its Standard taskbar and Fruity dock. Window menus
provide the same commands as keyboard shortcuts, including when the browser or
operating system reserves a shortcut. Press F1 for the shortcut list.

## Arrange and close windows

Right-click or long-press the maximize button to choose a layout. With keyboard
focus on that button, Shift+F10 opens the same menu. Layouts include all four
quarters, all four halves and maximize. A layout is disabled when the app's
minimum or maximum size does not fit. Restore returns to the last free window bounds even
after several layout changes.
Session restore keeps the layout and the original free bounds across reloads.
On a smaller display, an unavailable layout falls back to a window that fits.

Win/Command plus Left or Right snaps to a half; then Up or Down selects a quarter.
The window menu also offers closing other windows on the current workspace or
all windows of the same app. Close decisions run in order; cancelling keeps that
window and all later targets open.

## Search and default applications

Start and Spotlight search applications, individual Desktop settings, recent
files and shell actions. Settings results focus the matching control. Arithmetic
results use the Calculator parser and open in Calculator. Spotlight also filters
files by type, modification date and byte size. More results load in pages;
partial scans and request failures are reported explicitly.

Settings → Files lists default applications by extension. It uses the same
available choices as Open With. A removed application remains visible until you
reset its association; opening the file uses the normal fallback in the meantime.
Read-only mode disables changes and mutating search actions.

## Notifications and accessibility

The notification center provides Do Not Disturb and separate popup/sound choices
for each app. These preferences belong to this browser origin and synchronize
between its tabs. They remain usable when Desktop settings are read-only.

The history keeps at most 40 entries in the tab's session storage. Reload restores
it only after checking the authenticated session; logout, expiry or a different
session clears it. Apps can mark an entry as memory-only. Opening the center does
not mark anything read: use an entry's action or Mark all read. Remove dismisses
only that history entry. If browser storage is unavailable, the center indicates
that preferences or history can be kept only in memory.

Settings → Appearance offers text size at 100%, 125% or 150% and high contrast.
Shell controls respect forced colors and reduced motion. Embedded applications
retain their own accessibility behavior; use browser/OS zoom and assistive tools
as needed. Sensitive, editable or embedded content and live canvas/media use
replacement cards in window previews.

## Quick settings, calendar and media

The sliders button beside the clock opens Quick settings. Desktop sound volume
controls interface feedback; music keeps its player's volume. Do Not Disturb
uses the same browser-local policy as the notification center. Sound, animation
and theme-mode changes use the server's Desktop settings and remain disabled in
read-only mode. Light/dark choices appear only for themes that support them.

Click the clock to browse months, select a day and view its appointments. Today
returns to the current date; Open Calendar opens the full application. Loading
errors offer Retry instead of presenting an empty agenda.

Quick settings shows the current media owner's title, artist and playback state.
Its transport buttons invoke that player's existing handlers; it creates no
second player. Personal Radio retains priority, followed by the native audio
player, WebAmp, then the most recently activated Radio, TeeVee or RTL-SDR player.

## Undo file moves and deletion

Desktop and File Manager share a history of 20 recent action groups. Normal
deletion moves local files to Trash. Use the Undo button in the operation toast,
the Desktop menu, or Ctrl+Z while the Desktop/File Manager has keyboard focus.
Redo uses Ctrl+Shift+Z or Ctrl+Y. Text editors keep their own Undo shortcuts.

History is temporary and clears on reload or logout. Server receipts expire after
30 minutes; the server keeps at most 256 file identities, so older entries can
expire sooner as new operations complete. Choose at most 256 entries in a single
receipt-backed operation. A partially completed operation keeps its confirmed
steps available; an uncertain network outcome is never retried automatically.

Undo restores a file's location only after checking its identity, current write
permissions and a free destination. It never replaces a file that appeared at the
target. Replaced/deleted source identities cannot be restored through an old
receipt. Notes restore through the Trash service. Permanent deletion, replacement
of existing files, remote-storage operations and moving symbolic links do not
receive file-history receipts; their existing permission and confirmation rules
still apply. Trash restoration remains available after history has been cleared.
