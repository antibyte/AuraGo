# Chapter 24: EasyDrag – automations by drag and drop

EasyDrag is the visual editor for flows. A flow is a mission built from building blocks: one or more **triggers** (Schedule, Date & time, Webhook, Email received, MQTT message, Home Assistant state, Device, Person detected (a network camera sees a person), Phone call on the Fritz!Box, Planner, AuraGo start, Budget alert, Mission finished or Manual start) followed by **steps** such as Web search, AI step, Create PDF, Telegram message, WhatsApp message, Send email, Home Assistant or any permitted tool. Flows bypass the agent queue and appear in Mission Control with their history.

This chapter covers the editor. How flow runs, limits, failures and secrets behave on the server is in [Chapter 11: Flow missions (EasyDrag)](11-missions.md#flow-missions-easydrag).

## Table of Contents

1. [Your First Flow](#your-first-flow)
2. [The Editor](#the-editor)
   - [Visual groups](#visual-groups)
3. [Saving](#saving)
4. [Testing](#testing)
5. [Publishing](#publishing)
6. [Runs and Notifications](#runs-and-notifications)
   - [Resuming local runs](#resuming-local-runs)
   - [Human approval](#human-approval-for-a-local-run)
7. [Decisions Node](#decisions-node)
8. [When a Step Fails](#when-a-step-fails)
9. [Secrets](#secrets)
10. [Templates, Import and Export](#templates-import-and-export)
11. [Mission Control, the Missions Page and the Dashboard](#mission-control-the-missions-page-and-the-dashboard)
12. [Settings](#settings)
13. [When EasyDrag Is Switched Off](#when-easydrag-is-switched-off)
14. [Keyboard Shortcuts](#keyboard-shortcuts)
15. [Reusable Local Blocks](#reusable-local-blocks)

---

## Your First Flow

The start page offers **Cards** and a **Compact list** for the same flows. Use the star
on a flow to mark it as a favorite, then choose **Favorites only** to narrow the list.
Under **Saved filters on this device**, save the current search, status filter, sort
order and favorites selection with a name. Selecting that name restores those choices;
saving the same name updates the filter. Deleting a saved filter leaves its flows intact.
You can keep up to 20 named filters. These preferences belong to this browser on this
device and are excluded from flow exports. If the browser refuses storage, EasyDrag
tells you and keeps the choices only until the window closes. Other open EasyDrag windows
in the same browser follow the change. Clearing browser storage removes these preferences.

**Folders and tags** organizes normal flows on AuraGo across devices. Use a flow's **⋯ →
Folder and tags**, choose one folder and enter up to 16 tags. Tags distinguish upper and
lower case. **Manage folders** creates or renames flat folders; move their flows before
deleting an occupied folder. Folder and tag filters can also be included in a saved local
filter. Organization does not change the flow, its published version or its test results.
If another window saved first, your input stays visible; **Reload stored values** asks before
discarding it. Hidden tags remain on the server during a folder move. **Replace hidden
content** starts a fresh tag list. Keep secrets out of these names and tags.

**Pause several flows:** select up to 50 published flows in the library, then choose
**Review pause**. Selection does not change your flows; changing the search, folder or
filter clears it. The review checks every target. Only flows shown as active in that
review can receive a pause request, after another current-state check.

Choose **Pause active flows** to proceed. Future scheduled and timer runs stop, and
durable recovery waits while paused. Ordinary runs already accepted may continue;
pausing does not undo completed actions. Each flow gets its own result. **Currently
paused; saving unconfirmed** means the current switch is off but the write was not
acknowledged; this does not confirm that the pause will survive a restart. No failed
or unanswered pause request is retried automatically. An uncertain write or permission
loss stops remaining requests; check the results before starting a new reviewed action.
Closing the dialog stops unsent pause requests, but a request already sent may still finish.

For a first result without an account, choose **Your first local result** in the recipe
gallery. Its three steps read a sample name, make a greeting and check that the name
is present. Open the greeting step's output after testing. Recipe search and category
filters help you find other examples; the cards distinguish local processing from
possible external effects. A setup reminder does not mean an integration is missing
or that its connection has been tested.

**Describe your automation** creates an AI proposal from your goal. Review its steps,
changes and possible effects before **Apply**. Only then is a new flow created; in an
existing flow, applying is one undoable edit. Publishing, activation and execution
remain separate actions. Your configured AI provider receives the goal you enter and
the available building-block descriptions, without stored test data or run outputs.
Keep secrets out of the goal text too. Templates and manual editing remain available
when the provider is unavailable.

1. Open **EasyDrag** on the desktop. Choose **New flow** and give it a name, or start from a **template**.
2. Click **Choose a trigger** and pick one.
3. Select the trigger and press **Ctrl+K**: quick search adds the next step right behind it and connects it. **Tab** only moves focus. You can also use **Add** on the selected card, or drag building blocks from the left panel onto the canvas or onto a connection.
4. Double-click a step (or press **Enter**) to edit it. The inspector opens in the editor, not as a dialog. It has Input, Parameters and Output tabs. Parameters has three parts: *Values*, *Settings* and *Note*. The Input tab shows data from earlier steps: from the last test or run, or as an example. Drag a value over the Parameters tab and then into a field, or click it to insert it where the cursor was. References appear as coloured chips with step and field, such as "Web search › results". In the text they read `{{web_search.results}}`.
5. **Test** (Ctrl+Enter) opens the test dialog. Choose **Simulation** with sample outputs or an explicitly selected **Real test**, which can have real effects. The draft is saved before execution. Inspect the result and the steps it followed; see [Testing](#testing).
6. **Publish** checks the flow, lists what it does outside AuraGo and can activate it right away. Only published flows run on their own. Your draft stays editable; the published version runs.

## Reusable Local Blocks

Open **Reusable blocks** from the start page and choose **New block**. A block starts with
an Input and an Output boundary. Double-click each boundary to add named fields and choose
Text, Number, Boolean, Object or List. Mark a field required or nullable independently.
An omitted optional value is different from explicit `null`; `false` and `0` remain real values.
The Output boundary binds each declared output to a fixed value or a value from an earlier step.
Use the Input tab's target selector and data tree to insert a reference at the saved cursor.
For example, `{{block_input.data.name}}` reads the scaffold's input field `name`.
If stored field declarations are invalid, a JSON repair view preserves them until you explicitly
apply a corrected list. A contract allows up to 64 fields and 32 KiB of declarations.

**Simulate block** saves and tests the draft before its first publication. Enter sample inputs
with the typed controls; objects and lists use JSON. Simulation performs only local logic and
creates no run history, timer or mission. Samples stay in the open dialog. **Preview notes** lists
the sample limits: 64 KiB in total, 10,000 values and 32 nested levels. Closing the dialog cancels
its pending request. Review the declared contract and then **Publish** to create an immutable
version. Blocks support Set fields, If, Switch, Merge and calls to published pure block versions.
The maximum call depth is eight; circular references are refused. Each call remains bound
to its chosen version. Blocks cannot contain a batch, wait, stop, use AI, access secrets or
invoke external tools. Completed child steps can be inspected in the simulation result.

In a flow or another block, add **Call block**, select a block and a published version, then choose
**Use selected version**. Review the old and new contracts before applying the reference. Fill the
named input fields or map upstream data to them. Outputs are direct fields, for example
`{{greet.greeting}}` for a call whose data name is `greet`. Publishing a new block version leaves
every existing caller unchanged. Upgrade each caller explicitly; matching bindings remain,
removed fields are listed, and Undo restores the previous reference. Incompatible values stay
visible until corrected. A call has no automatic retries; imported retry settings must be
explicitly reset to zero before publishing or running.

Under **More**, inspect published versions or review a block's dependents before deletion.
Published parent versions and retained runs can keep a block version in use; deletion then
remains blocked. **Load more** retrieves older versions or additional dependencies. Version
inspection is read-only; **Open current draft** returns to editing. **Restore as draft** asks
for confirmation and replaces the current block draft with the selected version. The live block
changes only when you publish; callers keep their pinned version until explicitly upgraded.
The parent run shows the call's block ID and version; recorded child steps remain part of that run.

### Inspect recorded block steps

Open a run or simulation result, select its call or batch step and choose **Output → Inspect
block steps**. Choose a call, then a recorded step to inspect its resolved parameters or
output as a tree, table or JSON. Breadcrumbs and Back navigate nested calls. Versions,
item numbers and attempts identify repeated invocations; the parent flow stays unchanged.
The view shows completed records, not live child progress or separate child runs.
Selecting a record replaces its list; **Back** returns to it. On small windows the same
viewer opens as a dialog so its values and navigation remain usable.

The call list and step list use local pages. Their retained counts differ from totals when
recording was shortened; unknown totals appear as a dash. Incomplete, shortened and older
history are identified explicitly. A missing record does not prove that a step was skipped.
Only complete values up to 16 KiB are shown; larger or already-shortened values are omitted
entirely. After an error, smaller values recorded later may also be omitted because they
could come from shortened error text; execution is unchanged.
The final attempt's times describe the whole node lifetime, including retries.

Document labels come from the exact retained block version. If it is unavailable, protected
or too large, records remain readable without substituting a current version. The same
viewer is available through the run list even when the historical parent document is missing.
Simulation uses only the snapshots returned with that simulation and never fetches live files.

### Extract selected steps into a block

Select a connected, linear sequence of enabled **Set fields** steps and choose **Extract
selection as block** from Edit or the selection menu. The first version supports fixed output
types and **Keep incoming data** turned off; the review explains when a selection or reference cannot be
extracted safely. Pending draft edits are saved first. Name the block and inspect the proposed
inputs, outputs, group changes and extra work before applying.

Apply creates and publishes the block and replaces the selection in the flow draft together.
It does not publish or run the flow. Undo restores the flow draft; it does not delete the new
block. If the response is lost, use **Check status**. An unknown result is not proof that
nothing happened: reload and review the current flow and block library before trying again.

### Process a list with a block

In a normal flow, add **For each item** and explicitly choose a published block version.
Under **Block input for each item**, select the typed field that receives each whole list
entry. Fill the other **Shared inputs** once. **List** accepts a JSON list or one mapped
expression, such as `{{trigger.data.items}}`. Mapping replaces the fixed list; it does not
append an expression to its JSON. Text such as `{{example.field}}` inside an entry stays
literal data, including when another step is renamed or copied.

Set **Maximum items** deliberately (1–1,000; initially 100). A longer list is rejected
before the first item and is never shortened. An empty list succeeds with zero results;
null is not a list. The direct JSON editor allows up to 2 MiB, 200,000 values and 64 levels;
the whole flow must still fit its document limit. The field's information control retains
these limits. If the work-budget check fails, reduce **Maximum items**; clicking the
problem takes you directly to that field.

Items run one at a time in their original order. **Stop at first failure** reports the
failed zero-based index and error code without a partial result. **Collect item errors**
retains ordered success/error records, with successful output or a stable error code.
Cancellation, time limits and resource limits always stop the whole batch. Outputs offer
`items`, `count`, `succeeded` and `failed`; the result view distinguishes processed totals
from retained entries and shows up to 50 item summaries, with the existing tree/table/JSON
views below. A shortened or omitted retained result does not prove that missing entries
succeeded or failed. **Inspect block steps** opens retained item and nested-call evidence;
stop mode still provides no partial structured batch output.

Simulation uses the same pinned pure block. Publishing a later block version never
upgrades this step: review the version change explicitly. If its per-item field disappears,
the previous choice stays visibly unavailable until you choose another one. The batch's
retry count remains zero. Its chosen block may call other pure blocks; external tools and durable
waits remain unavailable.

## The Editor

The step list in the left toolbar provides a visible outline of the flow. Select a
step there to navigate without dragging. **Focus selected path** frames its connected
path. You can resize the library and inspector with their separators, using a pointer
or the keyboard. These device-local view preferences do not modify the flow.
Panel-header width menus offer the same sizes by click or tap.

When inserting input data, choose **Insert data into** first. EasyDrag never guesses
the first available field. It retains the selected field and insertion position;
a wide inspector can show the source data beside the parameter form.

To transform a source value before inserting it, use its **Transform value** button or
press **Shift+F10** on the focused source row. Choose and arrange up to eight operations;
**Insert into chosen field** replaces the selection at the remembered cursor position
as one undoable edit.
The source label identifies where the preview data came from. Without an actual sample,
you can still build a valid expression; an absent sample and explicit JSON `null` are
different. You may enter a fresh JSON sample for this preview only; it is not saved or
inserted into the flow. Local previews limit input and intermediate JSON to 64 KiB and
displayed results to 8 KiB. Expand **Preview limits and notes** for details. Browser
formatting can differ from execution for dates, HTML, Unicode case and rounding; date
previews require an explicit offset in the input and an explicit timezone. Test the flow
to check execution with its real data.

- The header shows the flow's name and state, in the same words as Mission Control and the missions page: *Not published yet* (never published), *Published* or *Unpublished changes*, and *Paused* next to it while a published flow is switched off. It holds **Undo** and **Redo**, **Runs**, **Test**, **Publish**, the **Active** switch and the **⋯** menu: *Run now*, *Version history*, *Flow settings*, *Duplicate*, *Export*, *Show in Mission Control* and *Delete*. In an editor 560 px wide or narrower (a phone), **Runs** moves into the ⋯ menu; **Test** remains directly accessible, and *Paused* stays next to the name; so do *Republish needed* and the warning about stopped Date & time triggers. The footer shows the last run, problems and hints, and the save state. "Draft" always means the version you edit, as opposed to the published one.
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

### Visual groups

Use **Visual groups** in the left toolbar to open the group list. Select steps on the canvas,
then choose **Group selected steps**. Groups organize the canvas only: they do not add a step
or change connections, settings or execution. A step can belong to at most one group. A flow
or block can have up to 100 groups; a title has 1–80 characters and a note up to 2,048.

Click a group header to select its steps; double-click it or press Enter to open the group
inspector. There you can change the title, note and color, add or remove members, move the
group with position controls, or ungroup while keeping its steps. Deleting the last member
also removes the empty group. Each change can be undone. Copying a whole group carries its
group metadata; copying only some members copies those steps without the group.

Collapse and layout lock are local view choices on this device. A lock prevents moving and
aligning the group's steps, not editing their settings or running the flow. The outline still
lists members of a collapsed group; choosing a member or an error expands the group before
focusing it. In a read-only run or version, expanding a group changes only your view.
Group edits change the draft revision, so test the new revision before publishing.

### Collection expressions

These filters use the same expression language in the preview and during execution.
Field names refer to direct fields, not nested paths. For example,
`{{items | where("status", "eq", "ready") | sum("amount")}}` totals ready items.

| Filter | Effect |
| --- | --- |
| `pick("name", "value")` | Keep 1–16 named object fields; absent fields are omitted and `null` is retained. |
| `rename("title", "name")` | Rename a present field; an existing different destination is an error. |
| `where("status", "eq", "ready")` | Keep matching objects. Operators are `eq`, `ne`, `lt`, `le`, `gt`, `ge`; missing fields never match. Equality preserves types; ordering requires matching number or text types. |
| `sort("score", "desc")` | Stable numeric or Unicode code-point text ordering; missing and `null` values stay last. Use `sort()` for primitive values or `sort("", "desc")` for descending order. Mixed types and booleans are errors. |
| `unique("id")` | Keep the first item for each typed field value; missing differs from `null`. Use `unique()` for primitive values. |
| `sum("amount")` | Add finite numbers; empty lists total zero. Missing, `null` and text are errors. Use `sum()` for a list of numbers. |

The filters preserve their input. Each collection operation is bounded to 10,000 items
per list, 256 fields per object, 32 nested levels, 100,000 values and 2 MiB of text/key
bytes. Oversized data fails clearly. Existing flow limits still apply; the local visual
preview can impose smaller limits to stay responsive.

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

The dialog separates **Real test** from **Simulation**. A real test can perform the
listed actions. A simulation runs pure processing steps locally and replaces other
supported steps with the sample outputs you provide. Missing or incompatible samples
stop the simulation; the simulation engine has no tool, model, device or secret access.
Sample outputs are JSON keyed by step ID, with `output` and optional `ports`, or a
supported `error_code` to simulate failure. Unless saved, they stay in the open window.
Results are explicitly marked as simulated and are not live-run
history. Edited real-test data with redacted or omitted values must be completed first.
File references in simulated outputs remain text: they do not open files or load media
previews. A successful simulated step does not mean an external action occurred.

Edit simple text, number and boolean fields directly; nested values remain accessible
in the advanced JSON section. In Simulation, save inputs, fixture outputs and
**Expected results** as a named private test scenario. Each check compares a step's
exact JSON output, its output ports, or both, and reports differences after simulation.

Stored scenarios use their original data snapshot. Renaming preserves it;
**Start a fresh scenario** starts with empty inputs. Execution, parameter or connection
changes require a compatible new scenario; position, label and note changes do not.
Limits are 20 scenarios per flow and 256 KiB per scenario. Their data is encrypted on
the AuraGo server and excluded from flow exports. A complete backup also needs the
password-protected Vault export; the matching key is required to recover these data.

- **Test** saves the draft first; a draft that cannot be saved is not tested. The flow needs a trigger that is turned on.
- The test dialog shows the trigger data the test starts with. With several triggers, pick one under *Start from*. Change the JSON to try other cases; *Remember this data for later tests* keeps your edit.
- Stored test data shows secret values as `[redacted]`. Left unchanged, the test uses the real stored data. Replace exact redaction and omission placeholders in edited data before a real test; incomplete edited data is refused.
- A test has real effects. The dialog lists each kind (*Sends messages*, *Writes files*, *Controls devices*, …) with the steps that may cause it. Each step counts with what its building block does by default and with what its actual settings do, so an HTTP request with POST counts as sending, although the building block alone does not. If AuraGo cannot check the settings, or a building block cannot name its effects, the dialog says that the effects could not be fully checked. **Run test** confirms the listed kinds once the test has started, while the flow stays open; *Don't ask again for this flow* remembers them on this device. A test that does not start (AuraGo refuses it or cannot be reached) confirms nothing. A kind you have not confirmed yet is asked again.
- **Test step** in a step's detail view (also *Test this step* on the selected card and in its context menu) runs that step together with the steps before it. Its dialog therefore asks about the effects of every step that runs: the step itself and the steps before it that the chosen trigger reaches. A turned-off step does not run, nor do the steps that only it leads to.
- **Stop** in the header cancels the test or run you started in this window, without asking (also a run started with *Run now*). Other runs that have not ended are stopped under **Runs** (see [Runs and notifications](#runs-and-notifications)). While that test or live run is still starting or still going, EasyDrag does not start another and does not send an uncertain start again. Stopping does not undo steps that already ran. Automatic saving is unchanged. The runs list opens in the editor body, under the header and any run banner.
- A test runs the draft you see: if it was changed in another window meanwhile, EasyDrag refuses the test (*The flow was changed elsewhere*). Reload the flow and test again.

## Publishing

The review includes named step and parameter changes, with before/after values where
safe, and the test evidence for the exact draft in this editor. A later edit invalidates
that evidence. Credentials remain hidden and large comparisons may be marked as partial;
a successful test does not guarantee that a future live action will succeed.

Add an optional **Version note** when publishing a new flow or block version. The note is
saved with that version. **Version history** offers **Read note** and **Edit note** separately;
an empty saved note removes its text. Editing a note changes no draft, published graph or
test result. Notes preserve whitespace and allow up to 2,048 characters and 4,096 UTF-8 bytes.
Hidden notes require explicit replacement with fresh text. Keep secrets out of notes.
If publication reports a Mission Control or timer problem, the version and its note are
already saved. Publishing the same draft again keeps that note; use **Edit note** to change it.
Conflicts retain your input and never automatically repeat a write. If saving is blocked,
the dialog retains your text while read access remains available; reopen after access is restored.

- The publish dialog lists blocking problems, what the flow does outside AuraGo, data from outside that reaches sensitive steps, hints and the changes since the last publication. With blocking problems it can only be closed; a click on a problem opens its step.
- **Version history** (⋯ menu or the *Flow* menu) lists the published versions, newest first, with their date, number of steps and connections, and *Live* on the version that runs. **Restore as draft** replaces your draft with that version after a question; the live version stays until you publish. A draft that cannot be saved first (offline, invalid) is not replaced. EasyDrag keeps the last 50 published versions; the run view of an older run whose version was cleaned up says so.
- *Activate now so the triggers start working* switches the flow on with the publication. Later, the **Active** switch pauses and resumes it. An unpublished flow cannot be switched on; EasyDrag offers to publish it first.
- The building blocks panel marks blocks with real effects outside AuraGo. The mark looks at a block's default settings (for a generic tool, at its most far-reaching operation) and is no safety guarantee. The publish dialog lists the effects of your actual settings; the test dialog adds those of the blocks' default settings.
- **Published with a problem** means the new version is live, but Mission Control or the timers could not be updated. The header then shows *Publish not finished*, and **Publish** stays available: publish again to finish.
- When the flow's Mission Control entry is missing, use **Repair Mission Control connection**. It restores the existing link disabled and preserves the flow, versions and run history. Review the restored flow before explicitly activating it.
- *Data from outside reaches a sensitive step* (for example a mail subject in a shell command, a file path or a recipient) is a hint, not a blocking problem: AuraGo cannot tell whether the step checks the data, and many flows pass outside text on on purpose. Such flows can be published and run; make sure the step cannot be misused. Outside data that selects a web address's scheme, host, port or user name, or where a secret is sent, blocks publishing and fails that step in tests and older published versions too. Write the trusted address up to the first `/`, `?` or `#` after the host directly in the step; data may still fill the path, query, headers and body. A step that reads outside data is treated as untrusted as a whole, including its fixed output fields.
- A header such as *Authorization*, *X-Api-Key* or *Cookie* with a typed-in value is a hint as well: the value sits in clear text in the flow and its versions. Store it as a secret and use the *Secret* field. Exports and the run view show such values as `[redacted]`.

## Runs and Notifications

**Compare runs** compares two completed runs from the loaded, filtered history
(up to 200 choices). It highlights changed inputs, outputs, branches and states.
Truncated or redacted data cannot establish equality and is marked incomplete.
**Download diagnostics** exports technical metadata only, without names, payloads or
error-message text. Comparing never replays a step.

- **Runs** initially lists the latest 50 runs, filtered by *All*, *Errors*, *Tests* or *Live*. **Load older runs** retrieves another retained page; search covers the loaded rows. Refresh explicitly to restart at the newest page. Clicking a run opens its exact flow version and recorded input/output, read-only. A timeline and failure explanation help locate the affected step. **Back to draft** or **Esc** returns. The building blocks panel stays hidden in the run view, and a failed run opens centred on the failed step. A remaining save warning is labelled **Draft: …** and belongs to your editable draft, not the recorded run.
- In the run view, **Test again with this data** opens the test dialog with that run's trigger data. Replace redacted or omitted values before a real test. The dialog lists the real effects and asks before anything runs; data is remembered only when you tick *Remember this data for later tests*. The button is missing while a run is going and when the original trigger data was not retained in full.
- A run that has not ended yet (*Queued*, which includes waiting for a free slot, or *Running*) shows **Stop** in the list and in the banner of its run view, also when a trigger or another window started it. Stopping any run but a test asks first; steps that already ran are not undone. A read-only desktop offers Stop too, because stopping changes no flow; read-only missions refuse it.
- **Run now** (⋯ menu, the *Flow* menu, or a card's menu on the start page) starts the published version. The flow must be **Active**: while it is paused, *Run now* is turned off and its tooltip says to switch the flow on first, as Mission Control refuses to run a paused mission. **Test** works either way.
- A flow set to *Skip* in its settings does not start a new run while one is still going, so **Run now** starts nothing and says so; such flows skip new runs until the running one has ended.
- A failed live run notifies as the flow settings say: on the desktop (the default), as a push notification, on Telegram, on WhatsApp or not at all. A flow notifies once when it starts failing, then at most once an hour while it keeps failing. Clicking the desktop notification opens EasyDrag on that run.
- After a desktop reload, an open flow is opened again.
- When a *Stop* step ends a run while other steps still run, they get up to 30 seconds to finish; then they are stopped and show *Effect may have happened*: what they send or change may already have happened. If one of them fails after a stop with *Success*, the run counts as failed; after a stop with *Error*, the stop's message stays.
- A *Wait* step frees its run slot while it waits, so waiting flows do not hold up others; afterwards the run queues for a free slot. At most 64 runs wait like this at a time; beyond that a wait keeps its slot.
- A flow set to *parallel* uses at most half of the parallel runs at once. At most 200 runs (and 64 MiB of their trigger data) wait for a start in total; beyond that new triggers are dropped, and Mission Control notes it on the flow.
- Live and test runs are kept apart: *Runs kept per flow* counts live runs, and the newest 20 test runs are kept on their own. Very large runs keep the inputs and outputs of their first steps only (4 MiB per run); later steps show *Shortened*.

### Resuming local runs

**Resume local runs after restarts** is off by default in **Settings → EasyDrag flows**.
Switching it on requires a restart. Switching it off takes effect without a restart and keeps
saved work. The observed processing status and counts describe the server; changing an unsaved
checkbox does not change them. Unknown counts are shown as unknown.

This initial mode supports a manual trigger, Set fields, If, Switch, Merge, Stop and
**Persistent wait** and **Request approval**. The complete graph is checked, including disabled or unreachable steps.
Calls, batches, tools, models and external effects are excluded. Persistent wait accepts 1–86,400
seconds and is distinct from ordinary **Wait**, whose behavior stays unchanged. Publish and
explicitly start the supported live flow; draft tests and simulations never start recovery.
The wait must fit within the run's original total time limit. Date-dependent expressions are
refused when the original timezone cannot be restored safely.

The existing run view shows **Waiting** separately from **Recovery blocked**, with an observed
wake time and original deadline when available. A blocked run has not been declared failed or
completed. Its explanation identifies disabled processing, missing authority, an unavailable key
or storage, or incompatible saved state. Use the matching Vault/backup when repairing saved data;
the UI never replaces an unavailable key or discards the checkpoint automatically.

An explicitly followed active run keeps its run ID across reconnect and window restore. Its
exact stored graph opens read-only; your current draft remains separate. Recovery retains
completed committed steps. Pure computation that was not committed before a crash may repeat.
The original run deadline still applies, and retained output previews can remain shortened.
Publishing v2 does not replace an already admitted v1 run or extend its original deadline.

**Stop** requests cancellation of the observed saved state. **Stopping…** is not proof that the
run ended; the view waits for server confirmation. If that state changed, refresh and review it
before stopping again. Uncertain requests are never sent again automatically. Current Stop
permission is required. For these resumable runs, authorized Stop remains available when missions
or the desktop are read-only, or the app is locked; it does not enable processing. Ordinary
non-resumable runs keep their existing mission read-only restriction. An incompatible old checkpoint can be
stopped separately even when its historical run already ended; its original result is preserved.
Mission Control refuses a mission-wide Stop when saved runs are involved, before stopping any
run. Open the run you want to stop in EasyDrag and stop that individual run there.

This mode does not start dependent missions, Planner follow-ups or completion notifications.
Mission history can show the run once its source is available; the mission's cumulative run
counter is unchanged in this initial mode. No child invocation history is provided.
An active history entry whose source cannot be verified shows **Unknown**; its last recorded
state does not confirm current progress.

### Human approval for a local run

**Request approval** pauses an eligible local durable flow without holding a worker.
Add a review summary and, optionally, review data. The summary is limited to 2 KiB and
the JSON data to 16 KiB with at most 32 nesting levels. They are resolved once when the
run reaches this step. Oversized reviews and registered secret values are refused, not
shortened. Represent integer identifiers above 9,007,199,254,740,991 or below its negative
as strings to preserve their exact digits. The original run deadline also applies to approval.

Open **Approval inbox** in the EasyDrag library, then **Review request**. Check the exact
flow revision, step, complete summary/data and deadline. **Approve** explicitly allows
that same run to continue through its normal queue; it does not prove that it has already
continued or finished. **Reject** stops the waiting run. Expiry also stops it once the
stored state can be verified. The existing **Stop** remains available for a blocked run.

A usable administrator login and the default-off durable processing setting are required.
Only a signed-in administrator browser session can decide; API keys and deep links cannot.
The single administrator account records the local administrator as the decision maker;
it cannot distinguish people sharing its credentials. Read-only mode prevents decisions.

If the decision reply is lost, use **Check status**. This reads the stored result without
sending the decision again. Closing the view does not undo an accepted decision. Once
the encrypted checkpoint no longer retains the review, the view shows decision metadata
and explicitly says that the complete review is no longer retained.
This local approval step cannot authorize external actions.

## Decisions Node

*Decision* uses AuraGo’s optional managed Decisions service, which can run natively or in Docker. It accepts a state value and up to eight registered image references; templates can supply both. The server resolves image references within AuraGo’s media boundary before the model sees them. A missing or unavailable service marks the node as needing setup.

- *Route one* asks one `choice` or yes/no (`noul`) question. Each choice has a stable ID and its own `choice_<id>` output, plus *Uncertain*. Reordering options keeps their wires; changing an ID creates a different output. A confidence threshold (default 0.70) sends lower-confidence answers to *Uncertain*.
- *Evaluate all* accepts up to 16 ordered questions and returns an `answers` object keyed by question ID. Question types are *Choice*, *Yes / no* and *Score*. It uses *Uncertain* if any answer is below the threshold, otherwise `out`.
- Each question has templated instructions. Choice IDs are stable identifiers; labels are display text. Score options have distinct numeric values (their zero-based order is the default); the result includes the expected score. Yes/no always uses the fixed IDs `no` and `yes`.
- Model output is untrusted data. Technical failures stop the run or use the node’s *Error path*; *Continue* is unavailable because a failed evaluation has no valid decision result.
- GPU/iGPU inference is required by default. CPU fallback must be explicitly enabled in **Config → Decisions** and can be revoked there. The service status and any missing setup are shown on the node.

## When a Step Fails

For **Write file**, keep the folder fixed when the filename comes from email,
a webhook, a file read or AI output. For example, `reports/{{trigger.data.name}}.txt`
accepts the name `daily`. A whole external path, `../config`, subfolders or a
Windows drive/stream in the name stops the step with `FLOW_PARAM_INVALID` before
any file access. This also applies to tests and previously published versions.
The filesystem tool's permissions and protected paths still apply.

- Under *If this step fails* in a step's *Settings* you choose *Stop the run*, *Continue* (the next steps get the error as data; on an *If* step they get it along *No*, on a *Switch* step along *Otherwise*, never along *Yes* or a case) or *Error path* (an extra *Error* output for steps that handle the failure). *Retries*, *Seconds between* and *Time limit (s)* sit next to it.
- *Data name* in the same tab is the name later steps use in references (`{{data_name.field}}`).

## Secrets

- Store API keys for HTTP requests as secrets: **New secret** in the step's *Secret* field. A value may be up to 4 KiB, and AuraGo accepts at most 30 secret changes per minute. A name that exists already asks before its value is replaced.
- Secrets live in the Vault as `easydrag_<name>`. Flows can use them; the agent cannot list, read or change them. Run data and the run view show their values redacted, with a limit for very short values (see [Chapter 11](11-missions.md#flow-secrets)).
- The bin next to **New secret** deletes the chosen secret after a confirmation and clears the field. Flows that use the secret fail until it is set again; if published flows still use it, a notification names them.
- While a step uses a secret, outside data (a mail, a webhook, another web step) must not decide where the secret goes. Write the address yourself up to the first `/`, `?` or `#` after the host, for example `https://crm.example/api/contacts?email={{trigger.data.from}}`: data may then fill the path and the query, but not the scheme, the host, the port or a user name. `https://api.example.com{{…}}` or `https://{{…}}/` is refused when publishing, and a version published before this rule fails that step instead of sending the secret. Headers may contain data. *Header name* and *Value prefix* must be written out while a secret is set.

## Templates, Import and Export

- The start page offers twelve templates, including *Your first local result*, email and device notifications, a camera detection notification, an MQTT temperature warning and a manual note as PDF. In the editor, steps whose integration is not set up yet carry a badge, and their detail view links to the settings (*Open settings*); in the building blocks panel such blocks offer *Set up*.
- The new notification recipes select Web Push by default; configure it or choose another channel before publishing. For the temperature warning, select an MQTT topic and review the threshold. Missing or nonnumeric temperatures stop the run with an error; a successful simulation with sample values does not prove that the sensor or notification channel is working. The camera example sends text without attaching an image. Simulated notification and PDF results do not send a message or create a file.
- The email, device and camera examples start with broad filters: they match eligible messages from the watched inbox, device disconnects or armed camera detections from configured sources. Set the desired subject/sender, device, camera or zone filters before publishing if the flow should respond more narrowly.
- **Export** downloads the draft as `<name>.easydrag.json`. **Import** on the start page creates a new flow from such a file (at most 2 MiB). Vault references carry secret names. Credential headers (*Authorization*, *X-Api-Key*, *Cookie*, ...) are exported as `[redacted]`; registered secrets in group titles and notes are masked. If masking exceeds a group text limit, the text is shortened with an ellipsis. Other ordinary fields are exported as typed and may contain sensitive values. An export with visual groups requires a current AuraGo version that supports them; older versions refuse it instead of silently dropping the groups.

### Share a local flow with its blocks

**Export local pack** downloads a `.easydrag-pack.json` file containing the flow and
the exact pure block versions it uses, including nested calls. This format supports
manually started flows with local Set, If, Switch, Merge and Call block steps. Waits, approvals,
batches, AI, tools, integrations and secret references are not supported. Ordinary
`.easydrag.json` import and export remain available.

Import the pack from the library, review the flow, block versions and included content,
then choose **Import as inactive draft**. You get a new unpublished flow and independent
local block copies; existing flows are not overwritten, and nothing runs or activates.
Open the new draft and enter sample input explicitly before using **Simulation**. Keep
its imported blocks while the draft uses them; deleting an unpinned block can make the
draft invalid.

Run history and private scenarios are excluded. Export clears manual sample input and
redacts known secrets, but ordinary literal text can still contain private data. Review
the contents before sharing or importing. Files are limited to 32 MiB, each flow document
to 2 MiB and combined flow/block content to 18 MiB; dependency limits also apply.

If the import reply is lost, use **Check import status**. This only checks existing state;
it does not repeat the import. An incomplete or changed result stays uncertain. Check the
library and Mission Control before starting another import. A request already sent may
finish even if you close the dialog.
The check is kept in the current EasyDrag window. **Finish status check** requires your
confirmation and only discards that local check; it does not change server data or prove
that the import failed. After reloading the page, check the library and Mission Control
before importing the same pack again.

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
