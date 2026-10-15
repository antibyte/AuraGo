# Release notes: runtime and CI fixes (October 2026)

- Missions, including person-detection-triggered missions, may arm detection but
  cannot disable alerts or restore scheduled arming. They also cannot claim that
  a detection picture was displayed in a user chat.
- Saving Telegram settings now replaces the polling runtime without restarting
  the server. Reload and shutdown cancel and drain active message workers. A
  verified bot keeps its process-local update cursor across token rotation;
  active messages are not automatically replayed. Invalid replacement tokens
  remain unavailable until corrected; the previous token is not restored.
- Go tests, targeted race tests (including Telegram), and the vulnerability scan
  run independently in CI. Updater-retention and browser-sidecar tests now include
  their required cleanup helper and egress-proxy prerequisites.

## Decisions fallback and possible provider costs

`llm_router.decisions_fallback` defaults to `true`; the task router itself remains
disabled by default. Automatic classification can use Decisions only when the
router and Decisions are enabled, a usable assignment can change the ordinary
route, local rules/cache remain uncertain, and the configured quota permits it.
When the selected Decisions backend is OpenRouter, this sends bounded, scrubbed
human task text to that provider and may incur charges. A managed local backend
does not use OpenRouter for this classification. Disable **Use Decisions** or set
the classifier quota to zero to prevent these automatic Decisions calls. The
explicit preview button can also incur provider costs. See [Task LLM router](llm-router.md).

This update does not change saved routing defaults or memory persistence behavior.
