# Chapter 24: EasyDrag – automations by drag and drop

EasyDrag is the visual editor for flows. A flow is a mission built from building blocks: one or more **triggers** (Schedule, Date & time, Webhook, Email received, MQTT message, Home Assistant state, Device, Person detected (a network camera sees a person), Phone call on the Fritz!Box, Planner, AuraGo start, Budget alert, Mission finished or Manual start) followed by **steps** such as Web search, AI step, Create PDF, Telegram message, Send email, Home Assistant or any permitted tool. Flows bypass the agent queue and appear in Mission Control with their history.

This chapter covers the editor. How flow runs, limits, failures and secrets behave on the server is in [Chapter 11: Flow missions (EasyDrag)](11-missions.md#flow-missions-easydrag).

## Table of Contents

1. [Your First Flow](#your-first-flow)
2. [The Editor](#the-editor)
3. [Saving](#saving)
4. [Testing](#testing)
5. [Publishing](#publishing)
6. [Runs and Notifications](#runs-and-notifications)
7. [Decisions Node](#decisions-node)
8. [When a Step Fails](#when-a-step-fails)
9. [Secrets](#secrets)
10. [Templates, Import and Export](#templates-import-and-export)
11. [Mission Control, the Missions Page and the Dashboard](#mission-control-the-missions-page-and-the-dashboard)
12. [Settings](#settings)
13. [When EasyDrag Is Switched Off](#when-easydrag-is-switched-off)
14. [Keyboard Shortcuts](#keyboard-shortcuts)

---

## Your First Flow

1. Open **EasyDrag** on the desktop. Choose **New flow** and give it a name, or start from a **template**.
2. Click **Choose a trigger** and pick one.
3. Select the trigger and press **Ctrl+K**: quick search adds the next step right behind it and connects it. **Tab** only moves focus. You can also use **Add** on the selected card, or drag building blocks from the left panel onto the canvas or onto a connection.
4. Double-click a step (or press **Enter**) to edit it. The inspector opens in the editor, not as a dialog. It has Input, Parameters and Output tabs. Parameters has three parts: *Values*, *Settings* and *Note*. The Input tab shows data from earlier steps: from the last test or run, or as an example. Drag a value over the Parameters tab and then into a field, or click it to insert it where the cursor was. References appear as coloured chips with step and field, such as "Web search › results". In the text they read `{{web_search.results}}`.
5. **Test** (Ctrl+Enter) saves the draft and runs it. Steps light up one after another, and connections show how many items passed. A test has real effects; see [Testing](#testing).
6. **Publish** checks the flow, lists what it does outside AuraGo and can activate it right away. Only published flows run on their own. Your draft stays editable; the published version runs.

## The Editor

- The header shows the flow's name and state, in the same words as Mission Control and the missions page: *Not published yet* (never published), *Published* or *Unpublished changes*, and *Paused* next to it while a published flow is switched off. It holds **Undo** and **Redo**, **Runs**, **Test**, **Publish**, the **Active** switch and the **⋯** menu: *Run now*, *Version history*, *Flow settings*, *Duplicate*, *Export*, *Show in Mission Control* and *Delete*. In an editor 560 px wide or narrower (a phone), **Runs** and **Test** move into the ⋯ menu too, and *Paused* stays next to the name; so do *Republish needed* and the warning about stopped Date & time triggers. The footer shows the last run, problems and hints, and the save state. "Draft" always means the version you edit, as opposed to the published one.
- **Undo** and **Redo** in the header take back or repeat the last change, also on touch screens (Ctrl+Z and Ctrl+Shift+Z do the same on the canvas). Deleting steps shows a note such as "Alpha deleted" with **Undo** for a few seconds; deleting asks no question first.
- **Find step in flow** (Ctrl+F, also in the *View* menu and the canvas context menu) searches the steps by name, data name, type or parameter text; Enter jumps to the first match.
- **Arrange automatically** (Alt+Shift+F, also in the *Edit* menu and the canvas context menu) lines the steps up in columns from the trigger to the right. It is one undo step.
- Right-click (or long-press) a card for *Open*, *Test this step*, *Duplicate*, *Copy*, *Turn off* or *Turn on* and *Delete*; with two or more steps selected, *Align horizontally* and *Align vertically* put them on one row or column. Right-click the empty canvas for *Add step here*, *Paste here*, *Find step in flow*, *Arrange automatically*, *Select all* and *Show everything*.
- The overview at the bottom left (windows wider than 900 px) shows the whole flow; click or drag in it to move the view.
- Until a flow has its first connection, a tip explains how to connect steps: drag from the dot on a card's right edge onto the next card, or select a step and press C. When a connection is not possible, a short note next to the pointer says why (for example a loop, or ports that are already connected).
- Quick search adds a step when you click or tap it, not while you scroll its list. A step's *Settings* also hold its *Position on the canvas* (X and Y in pixels), so you can move it without dragging.
- The start page lists your flows with the same states and filters them by *All*, *Active*, *Paused* (published and switched off) and *With errors*. If a later refresh fails, the last list stays on screen and read-only until **Try again** succeeds. A lost sign-in clears that list. The first load, and flows switched off, replace the list with a retry or lock card instead of an empty library. An empty library offers templates, **New flow** and **Import**.
- The editor follows changes made elsewhere (another window, Mission Control, the missions page or the agent): when the flow is paused or switched on there, the state, the **Active** switch and *Run now* follow; when it is deleted there, EasyDrag says so and shows the start page.
- A flow opens at a readable size: a small flow is shown whole, a larger one starts at its trigger at 80 %. In windows wider than 560 px, EasyDrag remembers on this device where you left each flow. Moving around the canvas and zooming are no changes to the flow: they are neither saved to it nor counted as *Unpublished changes*. **Show everything** (Shift+1) fits the whole flow; when that zooms out far, the cards show only their names, in larger type.
- The inspector sits in the editor body. It is not a dialog, and EasyDrag does not open a pinned wide window for it. Input, Parameters and Output are tabs at every width. Below about 760 px the canvas and the inspector take turns. The EasyDrag window can be as narrow as 360 px; a 320 px viewport still fits. In narrow windows and on phones the building blocks panel starts closed. Opened, it floats over the canvas and closes once you add a step or tap the canvas.
- *Flow settings* hold the name, the description, what happens when a trigger fires during a run (*Queue*, *Run in parallel* or *Skip*), the maximum run time and where failed runs are reported. The maximum run time takes minutes and seconds, from 1 second to 24 hours; with both fields empty a run may take 30 minutes, the default. **Apply** leaves it as it is unless you changed it.
- When the desktop is read-only, EasyDrag opens flows read-only; only **Stop** still works for runs that have not ended (unless missions are read-only too, `tools.missions.readonly`). Switching the desktop to read-only does not stop flows that are already running: stop them under **Runs**. Read-only missions are different: while `tools.missions.readonly` is on, published flows start no new runs on their own (schedules, events, Date/Time timers) and Mission Control's *Run now* is refused, at once and without a restart.
- After an update, AuraGo checks every published flow against the current rules once at start-up. A flow whose published version no longer passes carries *Republish needed* on its start-page card and in the header, and Mission Control notes it on the flow mission. Its runs may fail until you open the flow, fix what the problems list shows and publish it again; the mark and the Mission Control note go away with the new publication (or when a later start-up check finds the flow fine).
- Runs that were already waiting in a queue when missions became read-only or EasyDrag was switched off do not start later either: when their turn comes they end as *Stopped* (automatic runs are paused), and Mission Control notes it on the flow mission. A run you started yourself with *Run now* still runs.
- When AuraGo cannot start the flow store (a locked or damaged `flows.db`), it keeps trying in the background. Until it works, the start page shows *Date & time triggers are not running* and the header a chip with the same words; the notice names Date & time triggers because they depend on these timers.
- In a list of keys and values (the headers of an HTTP request, for example), two rows with the same key are marked, and nothing in that list is saved until each key is unique. Numbers and yes/no values from an imported flow keep their type until you edit them.

## Saving

- EasyDrag saves one second after your last change. The chip at the bottom right shows the state:
  - *Saved*, *Unsaved* or *Saving…*: the normal states.
  - *Offline – retrying*: AuraGo is not reachable or answered with a server error. EasyDrag tries again by itself; when AuraGo gave a reason, it is shown next to the state.
  - *Not saved* with **Try again**: AuraGo refused the change, for example because the flow is too large or you may not change flows. The reason is shown next to it.
  - *Not saved – fix the errors*: the draft has errors that block saving. The next change is sent again.
  - *Conflict*: the flow was changed somewhere else. EasyDrag asks whether to keep your version or load the other one.
- Until AuraGo has your changes, a copy stays in this browser for up to 30 days. When you open the flow again, EasyDrag offers to restore it, unless the flow was changed elsewhere in the meantime. Deleting a flow, here or elsewhere, removes this copy and everything else this browser remembers for the flow (where you left it, confirmed test effects, the last test trigger).
- **Ctrl+S** saves at once. Inside the editor it never opens the browser's "Save page". It does not save in a run view or while a real dialog is open. The step inspector does not block it, and automatic saving goes on. A note or field you are still typing is written before save, test, publish or leaving. Invalid JSON, a number outside its range, or a data name the editor refuses stays in the field and blocks that action. Text you are composing with an input method is not cut off.

## Testing

- **Test** saves the draft first; a draft that cannot be saved is not tested. The flow needs a trigger that is turned on.
- The test dialog shows the trigger data the test starts with. With several triggers, pick one under *Start from*. Change the JSON to try other cases; *Remember this data for later tests* keeps your edit.
- Stored test data shows secret values as `[redacted]`. Left unchanged, the test uses the real stored data. Edited data that still contains `[redacted]` runs, but is not remembered.
- A test has real effects. The dialog lists each kind (*Sends messages*, *Writes files*, *Controls devices*, …) with the steps that may cause it. Each step counts with what its building block does by default and with what its actual settings do, so an HTTP request with POST counts as sending, although the building block alone does not. If AuraGo cannot check the settings, or a building block cannot name its effects, the dialog says that the effects could not be fully checked. **Run test** confirms the listed kinds once the test has started, while the flow stays open; *Don't ask again for this flow* remembers them on this device. A test that does not start (AuraGo refuses it or cannot be reached) confirms nothing. A kind you have not confirmed yet is asked again.
- **Test step** in a step's detail view (also *Test this step* on the selected card and in its context menu) runs that step together with the steps before it. Its dialog therefore asks about the effects of every step that runs: the step itself and the steps before it that the chosen trigger reaches. A turned-off step does not run, nor do the steps that only it leads to.
- **Stop** in the header cancels the test or run you started in this window, without asking (also a run started with *Run now*). Other runs that have not ended are stopped under **Runs** (see [Runs and notifications](#runs-and-notifications)). While that test or live run is still starting or still going, EasyDrag does not start another and does not send an uncertain start again. Stopping does not undo steps that already ran. Automatic saving is unchanged. The runs list opens in the editor body, under the header and any run banner.
- A test runs the draft you see: if it was changed in another window meanwhile, EasyDrag refuses the test (*The flow was changed elsewhere*). Reload the flow and test again.

## Publishing

- The publish dialog lists blocking problems, what the flow does outside AuraGo, data from outside that reaches sensitive steps, hints and the changes since the last publication. With blocking problems it can only be closed; a click on a problem opens its step.
- **Version history** (⋯ menu or the *Flow* menu) lists the published versions, newest first, with their date, number of steps and connections, and *Live* on the version that runs. **Restore as draft** replaces your draft with that version after a question; the live version stays until you publish. A draft that cannot be saved first (offline, invalid) is not replaced. EasyDrag keeps the last 50 published versions; the run view of an older run whose version was cleaned up says so.
- *Activate now so the triggers start working* switches the flow on with the publication. Later, the **Active** switch pauses and resumes it. An unpublished flow cannot be switched on; EasyDrag offers to publish it first.
- The building blocks panel marks blocks with real effects outside AuraGo. The mark looks at a block's default settings (for a generic tool, at its most far-reaching operation) and is no safety guarantee. The publish dialog lists the effects of your actual settings; the test dialog adds those of the blocks' default settings.
- **Published with a problem** means the new version is live, but Mission Control or the timers could not be updated. The header then shows *Publish not finished*, and **Publish** stays available: publish again to finish.
- When the flow's Mission Control entry is missing, publishing again does not help. Export the flow, delete it and import it again.
- *Data from outside reaches a sensitive step* (for example a mail subject in a shell command, a file path or a recipient) is a hint, not a blocking problem: AuraGo cannot tell whether the step checks the data, and many flows pass outside text on on purpose. Such flows can be published and run; make sure the step cannot be misused. Only outside data that decides where a secret is sent blocks publishing.
- A header such as *Authorization*, *X-Api-Key* or *Cookie* with a typed-in value is a hint as well: the value sits in clear text in the flow and its versions. Store it as a secret and use the *Secret* field. Exports and the run view show such values as `[redacted]`.

## Runs and Notifications

- **Runs** lists the latest 50 runs, filtered by *All*, *Errors*, *Tests* or *Live*. Clicking one opens the run view: the flow version of that run with every input and output, read-only. A banner names the run; **Back to draft** or **Esc** returns. The building blocks panel stays hidden in the run view, and a failed run opens centred on the failed step.
- In the run view, **Test again with this data** opens the test dialog with the trigger data of that run. Values hidden as `[redacted]` must be replaced, or the test runs with the placeholder text. The dialog lists the real effects and asks before anything runs, as every test does; the data is not remembered unless you tick *Remember this data for later tests*. The button is missing while a run is going, and for a run whose trigger data was too large to be kept in full.
- A run that has not ended yet (*Queued*, which includes waiting for a free slot, or *Running*) shows **Stop** in the list and in the banner of its run view, also when a trigger or another window started it. Stopping any run but a test asks first; steps that already ran are not undone. A read-only desktop offers Stop too, because stopping changes no flow; read-only missions refuse it.
- **Run now** (⋯ menu, the *Flow* menu, or a card's menu on the start page) starts the published version. The flow must be **Active**: while it is paused, *Run now* is turned off and its tooltip says to switch the flow on first, as Mission Control refuses to run a paused mission. **Test** works either way.
- A flow set to *Skip* in its settings does not start a new run while one is still going, so **Run now** starts nothing and says so; such flows skip new runs until the running one has ended.
- A failed live run notifies as the flow settings say: on the desktop (the default), as a push notification, on Telegram or not at all. A flow notifies once when it starts failing, then at most once an hour while it keeps failing. Clicking the desktop notification opens EasyDrag on that run.
- After a desktop reload, an open flow is opened again.
- When a *Stop* step ends a run while other steps still run, they get up to 30 seconds to finish; then they are stopped and show *Effect may have happened*: what they send or change may already have happened. If one of them fails after a stop with *Success*, the run counts as failed; after a stop with *Error*, the stop's message stays.
- A *Wait* step frees its run slot while it waits, so waiting flows do not hold up others; afterwards the run queues for a free slot. At most 64 runs wait like this at a time; beyond that a wait keeps its slot.
- A flow set to *parallel* uses at most half of the parallel runs at once. At most 200 runs (and 64 MiB of their trigger data) wait for a start in total; beyond that new triggers are dropped, and Mission Control notes it on the flow.
- Live and test runs are kept apart: *Runs kept per flow* counts live runs, and the newest 20 test runs are kept on their own. Very large runs keep the inputs and outputs of their first steps only (4 MiB per run); later steps show *Shortened*.

## Decisions Node

*Decision* uses AuraGo’s optional managed Decisions service, which can run natively or in Docker. It accepts a state value and up to eight registered image references; templates can supply both. The server resolves image references within AuraGo’s media boundary before the model sees them. A missing or unavailable service marks the node as needing setup.

- *Route one* asks one `choice` or yes/no (`noul`) question. Each choice has a stable ID and its own `choice_<id>` output, plus *Uncertain*. Reordering options keeps their wires; changing an ID creates a different output. A confidence threshold (default 0.70) sends lower-confidence answers to *Uncertain*.
- *Evaluate all* accepts up to 16 ordered questions and returns an `answers` object keyed by question ID. Question types are *Choice*, *Yes / no* and *Score*. It uses *Uncertain* if any answer is below the threshold, otherwise `out`.
- Each question has templated instructions. Choice IDs are stable identifiers; labels are display text. Score options have distinct numeric values (their zero-based order is the default); the result includes the expected score. Yes/no always uses the fixed IDs `no` and `yes`.
- Model output is untrusted data. Technical failures stop the run or use the node’s *Error path*; *Continue* is unavailable because a failed evaluation has no valid decision result.
- GPU/iGPU inference is required by default. CPU fallback must be explicitly enabled in **Config → Decisions** and can be revoked there. The service status and any missing setup are shown on the node.

## When a Step Fails

- Under *If this step fails* in a step's *Settings* you choose *Stop the run*, *Continue* (the next steps get the error as data; on an *If* step they get it along *No*, on a *Switch* step along *Otherwise*, never along *Yes* or a case) or *Error path* (an extra *Error* output for steps that handle the failure). *Retries*, *Seconds between* and *Time limit (s)* sit next to it.
- *Data name* in the same tab is the name later steps use in references (`{{data_name.field}}`).

## Secrets

- Store API keys for HTTP requests as secrets: **New secret** in the step's *Secret* field. A value may be up to 4 KiB, and AuraGo accepts at most 30 secret changes per minute. A name that exists already asks before its value is replaced.
- Secrets live in the Vault as `easydrag_<name>`. Flows can use them; the agent cannot list, read or change them. Run data and the run view show their values redacted, with a limit for very short values (see [Chapter 11](11-missions.md#flow-secrets)).
- The bin next to **New secret** deletes the chosen secret after a confirmation and clears the field. Flows that use the secret fail until it is set again; if published flows still use it, a notification names them.
- While a step uses a secret, outside data (a mail, a webhook, another web step) must not decide where the secret goes. Write the address yourself up to the first `/`, `?` or `#` after the host, for example `https://crm.example/api/contacts?email={{trigger.data.from}}`: data may then fill the path and the query, but not the scheme, the host, the port or a user name. `https://api.example.com{{…}}` or `https://{{…}}/` is refused when publishing, and a version published before this rule fails that step instead of sending the secret. Headers may contain data. *Header name* and *Value prefix* must be written out while a secret is set.

## Templates, Import and Export

- The start page offers six templates: *AI news as PDF via Telegram*, *Summarise a webhook by email*, *Appointment reminder*, *Lights off when leaving*, *Budget guard* and *Daily RSS digest*. In the editor, steps whose integration is not set up yet carry a badge, and their detail view links to the settings (*Open settings*); in the building blocks panel such blocks offer *Set up*.
- **Export** downloads the draft as `<name>.easydrag.json`. **Import** on the start page creates a new flow from such a file (at most 2 MiB). A flow file carries secret names, never their values; credential headers (*Authorization*, *X-Api-Key*, *Cookie*, ...) are exported as `[redacted]`; other values typed into ordinary fields are exported as typed.

## Mission Control, the Missions Page and the Dashboard

- In Mission Control, flows carry the *EasyDrag* badge, and the filter *Flows* shows only them. **Open in EasyDrag** replaces *Edit*; **New flow** opens EasyDrag's start page. Schedules read like "weekdays at 07:00 AM" (times follow your language's format), and a mission that never ran shows *Never run*.
- Runs that only wait for a free slot cannot be cancelled in Mission Control; stop them here under **Runs**.
- *Run* in Mission Control answers *skipped* when the flow allows one run at a time and one is going.
- Running, pausing and deleting flow missions, the missions page and the dashboard are described in [Chapter 11: Flow missions](11-missions.md#flow-missions-easydrag).

## Settings

*Config → Agent Tools → EasyDrag flows*:
- *Enable flows*, *Parallel runs (1–32)*, *Parallel steps per run (1–16)*, *Keep run history (days, 1–365)* and *Runs kept per flow (10–5000)*. Changes to these take effect after a restart; switching *Enable flows* off turns EasyDrag off at once (see [When EasyDrag is switched off](#when-easydrag-is-switched-off)), and published flows start no new runs on their own (schedules, events, Date/Time timers) from then on; a run that is already going continues until you stop it under **Runs**.
- *AI provider for AI steps*: pick a provider from your provider list. AI steps set to "Default model" use it; without one they use the main model. It applies at once.
- The agent options (*Agent may only read flows*, *Agent may publish flows*) are locked. They take effect once the agent can work with flows.
- *Open EasyDrag* opens the app on the desktop.

## When EasyDrag Is Switched Off

EasyDrag needs *Enable flows* and the missions tool (`tools.missions.enabled`, see [Chapter 11](11-missions.md#prerequisites)). When either is off, the EasyDrag icon disappears the next time the desktop loads its app list. When EasyDrag loads while it is off (a new window, **Try again**, or opening a flow), it shows *EasyDrag is switched off* with **Open settings** and **Try again**. The start page shows the same card, with both buttons, in place of your flows the next time it loads its list; New flow, Import and the templates are then disabled. A flow that is already open stays in the editor, and its next save shows *Offline – retrying* until EasyDrag is on again. Flow missions stay in Mission Control, but their schedules, events and Date/Time timers start no runs while EasyDrag is off, and Mission Control's *Run now* is refused. You can still stop a run that is already going.

## Keyboard Shortcuts

Ctrl+S, Ctrl+Enter and Ctrl+K work in the editor, including while the step inspector is open. They wait while a real dialog or a context menu is open. The other keys act on the canvas, and not while you are typing in a field. On macOS, Ctrl is ⌘. **Tab** moves focus. It does not add a step. The canvas is one tab stop; the zoom buttons follow. A selected step's tool buttons join that order. Those tools are also keys or buttons: **Add** inserts the next step from the card, **Connect** (C) opens a list of compatible steps and ports, D turns the step off or on, Ctrl+D duplicates, and Del deletes. Tapping a port and releasing it without a target opens the same list with that side and port already chosen. *Test step* is on the card and in the inspector (Enter opens the inspector). Screen readers also find a list of the steps, next to the canvas. That list is there for browse mode; it has not been tested with screen-reader users.

| Key | Action |
|---|---|
| Tab | Move focus. Does not add a step |
| Enter | Open the step inspector |
| C | Connect the selected step (choose the step and port) |
| D | Turn steps off or on |
| Del | Delete the selection |
| Arrow keys | Move between steps |
| Shift+arrow keys | Nudge the selection |
| Hold Space | Pan the canvas |
| Shift+1 | Show the whole flow |
| + / − | Zoom in or out |
| Ctrl+0 | Zoom to 100 % |
| Ctrl+Z / Ctrl+Shift+Z | Undo / redo |
| Ctrl+C / X / V / D | Copy / cut / paste / duplicate |
| Ctrl+A | Select all |
| Ctrl+S | Save now |
| Ctrl+Enter | Test |
| Ctrl+K | Add from the one selected step's outgoing port; otherwise search building blocks |
| Ctrl+F | Find a step in the flow |
| Alt+Shift+F | Arrange automatically |
| ? | Show these shortcuts |
| Esc | Close or clear the selection; in a run view, back to the draft |

---

**See also:** [Chapter 11: Mission Control](./11-missions.md) · [Chapter 4: Web UI](./04-webui.md) · [Chapter 14: Security](./14-security.md)

---

**Previous Chapter:** [Chapter 23: Internals](./23-internals.md)
**Next Chapter:** [Chapter 25: CYD Creator](./25-cyd-creator.md)
