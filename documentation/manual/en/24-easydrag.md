# 24 · EasyDrag – automations by drag and drop

EasyDrag is the visual editor for flows. A flow is a mission built from building blocks: one or more **triggers** (Schedule, Date & time, Webhook, Email received, MQTT message, Home Assistant state, Device, Phone call on the Fritz!Box, Planner, AuraGo start, Budget alert, Mission finished or Manual start) followed by **steps** such as Web search, AI step, Create PDF, Telegram message, Send email, Home Assistant or any permitted tool. Flows bypass the agent queue and appear in Mission Control with their history.

## Your first flow

1. Open **EasyDrag** on the desktop. Choose **New flow** and give it a name, or start from a **template**.
2. Click **Choose a trigger** and pick one.
3. Select the trigger and press **Tab**: quick search adds the next step right behind it and connects it. You can also drag building blocks from the left panel onto the canvas or onto a connection.
4. Double-click a step (or press **Enter**) to edit it. The left column shows the data of earlier steps, from the last test or as an example. Drag a value into a field, or click it to insert it where the cursor was. References appear as coloured chips with step and field, such as "Web search › results". In the text they read `{{web_search.results}}`.
5. **Test** (Ctrl+Enter) saves the draft and runs it. Steps light up one after another, and connections show how many items passed. A test has real effects: the test dialog names each kind (such as "Sends messages" or "Writes files") with its steps, and **Run test** confirms them. It shows stored test data with `[redacted]` in place of secret values; left unchanged, the test uses the real stored data.
6. **Publish** checks the flow, lists what it does outside AuraGo and can activate it right away. Only published flows run on their own. Your draft stays editable; the published version runs.

## Good to know

- **Saved automatically.** EasyDrag saves one second after your last change. The chip at the bottom right shows the state: *Saved*, *Saving…*, *Offline – retrying* (EasyDrag tries again by itself) or *Not saved* with **Try again**. When the flow was changed somewhere else, EasyDrag asks which version should win. Until AuraGo has your changes, a copy stays in the browser and is offered the next time you open the flow. **Ctrl+S** saves at once and never opens the browser's "Save page". While an EasyDrag dialog is open, Ctrl+S does not save; automatic saving goes on.
- **Runs.** The **Runs** button lists tests and live runs, filtered by *All*, *Errors*, *Tests* or *Live*. Clicking one opens the run view: the flow version of that run with every input and output, read-only. **Back to draft** or **Esc** returns.
- **Error path.** Under *If this step fails* in a step's *Settings* you choose *Stop the run*, *Continue* (the next steps get the error as data) or *Error path* (an extra *Error* output for steps that handle the failure). Retries, seconds between them and a time limit sit next to it.
- **Secrets.** Store API keys for HTTP requests as secrets: **New secret** in the step's *Secret* field. They live in the Vault. Flows can use them; the agent cannot read them.
- **Mission Control.** Flows carry the *EasyDrag* badge there. **Open in EasyDrag** replaces *Edit* and jumps into the editor. Pause, lock, run, delete and the history work as for other missions, but *Run now* and *Resume* stay disabled until the flow is published ("Not published yet").
- **Settings.** *Config → Agent Tools → EasyDrag flows* limits parallel runs, parallel steps per run and the run history. There you also pick, from your provider list, the AI provider for AI steps set to "Default model".

## Keyboard shortcuts

Ctrl+S, Ctrl+Enter and Ctrl+K work anywhere in the editor; the other keys act on the canvas. On macOS, Ctrl is ⌘.

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
