# Chapter 24: EasyDrag – automations by drag and drop

EasyDrag is the visual editor for flows. A flow is a mission built from building blocks: one or more **triggers** (Schedule, Date & time, Webhook, Email received, MQTT message, Home Assistant state, Device, Phone call on the Fritz!Box, Planner, AuraGo start, Budget alert, Mission finished or Manual start) followed by **steps** such as Web search, AI step, Create PDF, Telegram message, Send email, Home Assistant or any permitted tool. Flows bypass the agent queue and appear in Mission Control with their history.

This chapter covers the editor. How flow runs, limits, failures and secrets behave on the server is in [Chapter 11: Flow missions (EasyDrag)](11-missions.md#flow-missions-easydrag).

## Your first flow

1. Open **EasyDrag** on the desktop. Choose **New flow** and give it a name, or start from a **template**.
2. Click **Choose a trigger** and pick one.
3. Select the trigger and press **Tab**: quick search adds the next step right behind it and connects it. You can also drag building blocks from the left panel onto the canvas or onto a connection.
4. Double-click a step (or press **Enter**) to edit it. The left column shows the data of earlier steps: from the last test or run, or as an example. Drag a value into a field, or click it to insert it where the cursor was. References appear as coloured chips with step and field, such as "Web search › results". In the text they read `{{web_search.results}}`.
5. **Test** (Ctrl+Enter) saves the draft and runs it. Steps light up one after another, and connections show how many items passed. A test has real effects; see [Testing](#testing).
6. **Publish** checks the flow, lists what it does outside AuraGo and can activate it right away. Only published flows run on their own. Your draft stays editable; the published version runs.

## The editor

- The header shows the flow's name and state, in the same words as Mission Control and the missions page: *Not published yet* (never published), *Published* or *Unpublished changes*, and *Paused* next to it while a published flow is switched off. It holds **Runs**, **Test**, **Publish**, the **Active** switch and the **⋯** menu: *Run now*, *Flow settings*, *Duplicate*, *Export*, *Show in Mission Control* and *Delete*. The footer shows the last run, problems and hints, and the save state. "Draft" always means the version you edit, as opposed to the published one.
- The start page lists your flows with the same states and filters them by *All*, *Active*, *Paused* (published and switched off) and *With errors*.
- The editor follows changes made elsewhere (another window, Mission Control, the missions page or the agent): when the flow is paused or switched on there, the state, the **Active** switch and *Run now* follow; when it is deleted there, EasyDrag says so and shows the start page.
- A flow opens at a readable size: a small flow is shown whole, a larger one starts at its trigger at 80 %. In windows wider than 560 px, EasyDrag remembers on this device where you left each flow. Moving around the canvas and zooming are no changes to the flow: they are neither saved to it nor counted as *Unpublished changes*. **Show everything** (Shift+1) fits the whole flow; when that zooms out far, the cards show only their names, in larger type.
- In narrow windows and on phones the building blocks panel starts closed. Opened, it floats over the canvas and closes once you add a step or tap the canvas. The detail view of a step then uses tabs.
- *Flow settings* hold the name, the description, what happens when a trigger fires during a run (*Queue*, *Run in parallel* or *Skip*), the maximum run time and where failed runs are reported.
- When the desktop is read-only, EasyDrag opens flows read-only; only **Stop** still works for runs that have not ended (unless missions are read-only too, `tools.missions.read_only`). Switching the desktop to read-only does not stop flows that are already running: stop them under **Runs**.

## Saving

- EasyDrag saves one second after your last change. The chip at the bottom right shows the state:
  - *Saved*, *Unsaved* or *Saving…*: the normal states.
  - *Offline – retrying*: AuraGo is not reachable or answered with a server error. EasyDrag tries again by itself.
  - *Not saved* with **Try again**: AuraGo refused the change, for example because the flow is too large or you may not change flows. The tooltip names the reason.
  - *Not saved – fix the errors*: the draft has errors that block saving. The next change is sent again.
  - *Conflict*: the flow was changed somewhere else. EasyDrag asks whether to keep your version or load the other one.
- Until AuraGo has your changes, a copy stays in this browser for up to 30 days. When you open the flow again, EasyDrag offers to restore it, unless the flow was changed elsewhere in the meantime. Deleting a flow, here or elsewhere, removes this copy and everything else this browser remembers for the flow (where you left it, confirmed test effects, the last test trigger).
- **Ctrl+S** saves at once. Inside the editor it never opens the browser's "Save page"; in a run view and while a dialog is open it does not save, and automatic saving goes on.

## Testing

- **Test** saves the draft first; a draft that cannot be saved is not tested. The flow needs a trigger that is turned on.
- The test dialog shows the trigger data the test starts with. With several triggers, pick one under *Start from*. Change the JSON to try other cases; *Remember this data for later tests* keeps your edit.
- Stored test data shows secret values as `[redacted]`. Left unchanged, the test uses the real stored data. Edited data that still contains `[redacted]` runs, but is not remembered.
- A test has real effects. The dialog lists each kind (*Sends messages*, *Writes files*, *Controls devices*, …) with the steps that may cause it. Each step counts with what its building block does by default and with what its actual settings do, so an HTTP request with POST counts as sending, although the building block alone does not. If AuraGo cannot check the settings, or a building block cannot name its effects, the dialog says that the effects could not be fully checked. **Run test** confirms the listed kinds while the flow stays open; *Don't ask again for this flow* remembers them on this device. A kind you have not confirmed yet is asked again.
- **Test step** in a step's detail view (also *Test this step* on the selected card and in its context menu) runs that step together with the steps before it. Its dialog therefore asks about the effects of every step that runs: the step itself and the steps before it that the chosen trigger reaches. A turned-off step does not run, nor do the steps that only it leads to.
- **Stop** in the header cancels the test or run you started in this window, without asking (also a run started with *Run now*). Other runs that have not ended are stopped under **Runs** (see [Runs and notifications](#runs-and-notifications)).

## Publishing

- The publish dialog lists blocking problems, what the flow does outside AuraGo, data from outside that reaches sensitive steps, hints and the changes since the last publication. With blocking problems it can only be closed; a click on a problem opens its step.
- *Activate now so the triggers start working* switches the flow on with the publication. Later, the **Active** switch pauses and resumes it. An unpublished flow cannot be switched on; EasyDrag offers to publish it first.
- The building blocks panel marks blocks with real effects outside AuraGo. The mark looks at a block's default settings (for a generic tool, at its most far-reaching operation) and is no safety guarantee. The publish dialog lists the effects of your actual settings; the test dialog adds those of the blocks' default settings.
- **Published with a problem** means the new version is live, but Mission Control or the timers could not be updated. The header then shows *Publish not finished*, and **Publish** stays available: publish again to finish.
- When the flow's Mission Control entry is missing, publishing again does not help. Export the flow, delete it and import it again.

## Runs and notifications

- **Runs** lists the latest 50 runs, filtered by *All*, *Errors*, *Tests* or *Live*. Clicking one opens the run view: the flow version of that run with every input and output, read-only. A banner names the run; **Back to draft** or **Esc** returns. The building blocks panel stays hidden in the run view, and a failed run opens centred on the failed step.
- A run that has not ended yet (*Queued*, which includes waiting for a free slot, or *Running*) shows **Stop** in the list and in the banner of its run view, also when a trigger or another window started it. Stopping any run but a test asks first; steps that already ran are not undone. A read-only desktop offers Stop too, because stopping changes no flow; read-only missions refuse it.
- **Run now** (⋯ menu, the *Flow* menu, or a card's menu on the start page) starts the published version. The flow must be **Active**: while it is paused, *Run now* is turned off and its tooltip says to switch the flow on first, as Mission Control refuses to run a paused mission. **Test** works either way.
- A failed live run notifies as the flow settings say: on the desktop (the default), as a push notification, on Telegram or not at all. A flow notifies once when it starts failing, then at most once an hour while it keeps failing. Clicking the desktop notification opens EasyDrag on that run.
- After a desktop reload, an open flow is opened again.

## When a step fails

- Under *If this step fails* in a step's *Settings* you choose *Stop the run*, *Continue* (the next steps get the error as data) or *Error path* (an extra *Error* output for steps that handle the failure). *Retries*, *Seconds between* and *Time limit (s)* sit next to it.
- *Data name* in the same tab is the name later steps use in references (`{{data_name.field}}`).

## Secrets

- Store API keys for HTTP requests as secrets: **New secret** in the step's *Secret* field. A value may be up to 4 KiB, and AuraGo accepts at most 30 secret changes per minute. A name that exists already asks before its value is replaced.
- Secrets live in the Vault as `easydrag_<name>`. Flows can use them; the agent cannot list, read or change them. Run data and the run view show their values redacted, with a limit for very short values (see [Chapter 11](11-missions.md#flow-secrets)).
- The bin next to **New secret** deletes the chosen secret after a confirmation and clears the field. Flows that use the secret fail until it is set again; if published flows still use it, a notification names them.

## Templates, import and export

- The start page offers six templates: *AI news as PDF via Telegram*, *Summarise a webhook by email*, *Appointment reminder*, *Lights off when leaving*, *Budget guard* and *Daily RSS digest*. In the editor, steps whose integration is not set up yet carry a badge, and their detail view links to the settings (*Open settings*); in the building blocks panel such blocks offer *Set up*.
- **Export** downloads the draft as `<name>.easydrag.json`. **Import** on the start page creates a new flow from such a file (at most 2 MiB). A flow file carries secret names, never their values; what you type into ordinary fields (a generic tool's header map, for example) is exported as typed.

## Mission Control, the missions page and the dashboard

- In Mission Control, flows carry the *EasyDrag* badge, and the filter *Flows* shows only them. **Open in EasyDrag** replaces *Edit*; **New flow** opens EasyDrag's start page. Schedules read like "weekdays at 07:00 AM" (times follow your language's format), and a mission that never ran shows *Never run*.
- Runs that only wait for a free slot cannot be cancelled in Mission Control; stop them here under **Runs**.
- Running, pausing and deleting flow missions, the missions page and the dashboard are described in [Chapter 11: Flow missions](11-missions.md#flow-missions-easydrag).

## Settings

*Config → Agent Tools → EasyDrag flows*:
- *Enable flows*, *Parallel runs (1–32)*, *Parallel steps per run (1–16)*, *Keep run history (days, 1–365)* and *Runs kept per flow (10–5000)*. Changes to these take effect after a restart; switching *Enable flows* off turns EasyDrag off at once (see [When EasyDrag is switched off](#when-easydrag-is-switched-off)), but published flows keep running until the restart.
- *AI provider for AI steps*: pick a provider from your provider list. AI steps set to "Default model" use it; without one they use the main model. It applies at once.
- The agent options (*Agent may only read flows*, *Agent may publish flows*) are locked. They take effect once the agent can work with flows.
- *Open EasyDrag* opens the app on the desktop.

## When EasyDrag is switched off

EasyDrag needs *Enable flows* and the missions tool (`tools.missions.enabled`, see [Chapter 11](11-missions.md#prerequisites)). When either is off, the EasyDrag icon disappears the next time the desktop loads its app list. When EasyDrag loads while it is off (a new window, **Try again**, or opening a flow), it shows *EasyDrag is switched off* with **Open settings** and **Try again**. The start page shows the same card, with both buttons, in place of your flows the next time it loads its list; New flow, Import and the templates are then disabled. A flow that is already open stays in the editor, and its next save shows *Offline – retrying* until EasyDrag is on again. Flow missions stay in Mission Control but do not run.

## Keyboard shortcuts

Ctrl+S, Ctrl+Enter and Ctrl+K work anywhere in the editor, but not while a dialog or a context menu is open; the other keys act on the canvas. On macOS, Ctrl is ⌘. The canvas is one stop for **Tab** from outside, and **Tab** on the canvas adds a step: the zoom buttons and the tool buttons of the selected step are reached with **Shift+Tab** from the footer. Every tool button also has a key: D turns the step off or on, Ctrl+D duplicates, Del deletes, and *Test step* is in the detail view (Enter). Screen readers also find a list of the steps.

| Key | Action |
|---|---|
| Tab | Add a step (after the selected step) |
| Enter | Open the step |
| C | Connect the selected step |
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
| Ctrl+K | Search building blocks |
| ? | Show these shortcuts |
| Esc | Close or clear the selection; in a run view, back to the draft |

---

**See also:** [Chapter 11: Mission Control](11-missions.md) · [Chapter 4: Web UI](04-webui.md) · [Chapter 14: Security](14-security.md)
