# question_user

Use `question_user` when you need the user to choose from explicit options before you can continue. It blocks until the user answers or the timeout expires.

Do not use it for general chat or open-ended questions unless `allow_free_text` is enabled. For simple clarifying questions without choices, ask in plain text.

## Parameters

- `question` (required): The question shown to the user.
- `options` (required): At least two objects with `label` and `value`. `description` is optional.
- `allow_free_text` (optional): If true, the user may type a custom answer instead of choosing an option.
- `timeout_seconds` (optional): Defaults to 120 seconds in webchat and 20 seconds in text channels.

## Response

Option selection:

```json
{"status":"ok","selected":"option_value"}
```

Free-text answer:

```json
{"status":"ok","selected":"","free_text":"user answer"}
```

Timeout:

```json
{"status":"timeout","selected":""}
```

## Channel Behavior

In webchat, the user sees a modal with option buttons, a timer, and a text input when free text is enabled.

In Telegram, Discord, and SMS, the user sees a numbered list. They can reply with a number or, when free text is enabled, type an answer freely.
