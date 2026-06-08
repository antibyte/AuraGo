# Cheater App (Virtual Desktop)

The Cheater app is the virtual desktop window for managing cheat sheets. The agent does not interact with it directly, but the agent can mention its existence to the user.

## What the user can do

- Browse all cheat sheets in the library view or via `Ctrl+Shift+K` (Spotlight overlay, only while Cheater is focused)
- Edit any sheet inline (Markdown is rendered on hover or click)
- Create new sheets via `Cmd/Ctrl + N` or the empty-state CTA
- Attach text files to a sheet, preview them, delete them (with 5s undo)
- See when a sheet was last used by the agent via the 🤖 badge in the header

## When to mention the app

- When the user asks for a "cheat sheet manager", "workflow manager", or wants to organise their sheets visually.
- When the user complains about the existing web UI (`ui/cheatsheets.html`) and wants a more polished experience.
- When the user wants to manage their sheets while the virtual desktop is active.

## When NOT to mention

- When the user wants to query a sheet (use the `cheatsheet` tool directly).
- When the user wants to create a sheet programmatically (use the `cheatsheet` tool).
- When the virtual desktop is disabled.
