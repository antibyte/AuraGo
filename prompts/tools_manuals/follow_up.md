## Tool: Follow Up (`follow_up`)

Schedule an independent background prompt to be executed by yourself immediately after responding to the user. Use this for sequential, multi-step work without forcing the user to wait or re-prompt you.

> ⛔ **CRITICAL RULE — INFINITE LOOP PREVENTION:**  
> `follow_up` must **only** be used when you have all required information and will do the work **yourself**.  
> **NEVER** use `follow_up` to relay a question back to the user (e.g. "Bitte gib mir den Pfad an" / "Please provide the path").  
> If you are missing information required to complete a task, **respond directly to the user** with your question in plain text — do NOT use `follow_up`. Using it to ask questions creates an infinite loop where each invocation re-asks the same unanswerable question.

### When to use

✅ You have all the information and will continue working immediately.  
✅ A long multi-step task should continue asynchronously in the background.  
✅ You want to chain sequential autonomous steps (e.g. Phase 1 → follow_up → Phase 2).

### When NOT to use

❌ You are missing information from the user (path, interval, name, content, etc.) — ask directly instead.  
❌ You want to prompt the user with a question — respond with plain text instead.  
❌ `task_prompt` ends with `?` — that is almost always a user-directed question.

### Constraints

- Maximum 10 sequential follow-ups before the system forces a pause.
- You CAN include a natural-language response to the user AND append this tool call at the end. The user sees your text; the follow-up triggers immediately afterward.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task_prompt` | string | yes | A self-contained task the agent will perform autonomously. Must NOT be a question for the user. |

### Example

```json
{"action": "follow_up", "task_prompt": "Continue with Phase 3 of the refactoring plan now."}
```