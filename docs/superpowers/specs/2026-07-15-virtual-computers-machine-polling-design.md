# Virtual Computers Machine Polling Design

## Goal

Keep the machine list in each open Virtual Computers desktop window current without requiring manual refreshes or disrupting an active VNC or terminal session.

## Existing Behavior

The app loads machines when the window opens, when the user explicitly refreshes, and after machine mutations. Active tasks have their own two-second polling loop, but external machine changes are not discovered automatically. The existing render and reconciliation paths already preserve a valid VNC or terminal session across ordinary data refreshes.

## User Experience

- While the Virtual Computers app window is visible, it checks the machine list every five seconds.
- If the response is unchanged, the UI is not redrawn.
- If machines were added, removed, reordered, or changed, the list and detail view update automatically.
- A valid active VNC or terminal session remains mounted and connected.
- If the selected machine disappears or changes in a way that invalidates its active viewer, the existing reconciliation behavior closes that session cleanly and returns the UI to a valid state.
- Polling pauses while the browser tab is hidden or the desktop app window is minimized or otherwise not visible. It resumes through the next scheduled check after visibility returns.
- Background polling failures do not repeatedly notify the user or replace the currently displayed list. The following cycle retries normally; manual refresh remains available for explicit feedback.

## Polling Architecture

The implementation adds a dedicated machine-only polling loop to `ui/js/desktop/apps/virtual-computers.js`. It must not call the full app refresh because that would also reload setup status, templates, tasks, and volumes and could introduce unnecessary loading states.

The loop uses chained `setTimeout` calls with a five-second delay instead of `setInterval`. The next timeout is scheduled after each attempt completes, which prevents overlapping machine requests on slow connections.

Per app-window state tracks:

- the machine polling timer;
- whether a machine request is currently in flight;
- the snapshot of the last accepted machine response.

Each desktop app window owns its own state and timer. Multiple Virtual Computers windows remain independent.

## Change Detection and Rendering

The initial full refresh establishes the baseline machine snapshot. Later polling requests call only the existing machines endpoint and normalize a non-array response to an empty array.

The new machine response is compared with the last accepted snapshot. Array order remains significant because a server-side order change should be reflected in the list. When the snapshots match, the poll returns without mutating app state or calling the render path.

When they differ, the app replaces `state.machines`, records the new snapshot, runs the existing selection and viewer reconciliation logic, and redraws the affected UI through the normal render path. This keeps session lifecycle decisions centralized rather than duplicating them in the poller.

Manual and mutation-triggered full refreshes also update the baseline snapshot so the next background poll does not perform a redundant redraw.

## Visibility and Lifecycle

Before making a request, the poller verifies that:

- the app has not been disposed;
- no machine poll is already in flight;
- the browser document is visible;
- the app host and its desktop window are visible rather than minimized.

A skipped visibility check still schedules the next five-second attempt. No new global visibility listener is required, keeping the lifecycle small and guaranteeing at most a five-second delay after the app becomes visible again.

Disposal clears the polling timeout in addition to the existing task timeout and disconnects existing viewers through their current cleanup paths. A request that completes after disposal must not update or redraw the app.

## Error Handling

Background polling is best-effort. Network or parsing failures retain the last known machine list, do not raise recurring notifications, and do not overwrite explicit resource errors from normal refreshes. The in-flight marker is always cleared and the next attempt is scheduled unless the app has been disposed.

The feature introduces no backend endpoints, configuration fields, credentials, database migrations, dashboard entries, or translation strings.

## Testing

UI regression tests must prove:

- the initial refresh establishes the machine snapshot;
- the poller requests only the machines endpoint after five seconds;
- an unchanged response does not redraw the UI;
- a changed response updates machines and invokes the normal reconciliation/render path;
- browser-tab hiding and desktop-window minimization skip network requests;
- slow requests cannot overlap;
- polling resumes after visibility returns;
- disposal clears the timer and ignores late responses;
- a normal refresh updates the snapshot and prevents a redundant poll redraw;
- an unchanged active VNC or terminal session survives a machine-list update;
- deletion or capability changes still close invalid viewers through existing reconciliation.

Existing Virtual Computers backend, VNC, terminal, asset-order, translation, and Quick Connect regression tests must remain green. Verification includes focused UI tests, `npm run check:ui`, `npm run test:ui-regressions`, relevant Go tests, the full Go suite, and GitNexus change detection before the implementation commit.

## Out of Scope

- Server-sent events or a new WebSocket event stream for machine changes.
- Polling setup status, templates, tasks, volumes, screenshots, or viewer output through this loop.
- User-configurable polling intervals.
- Notifications for transient background polling failures.
