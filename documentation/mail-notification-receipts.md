# Mail delivery receipts

Authenticated incoming-mail events have independent, persistent duplicate protection.
Routine notifications and attachments require explicit relay instructions; urgent
notifications still obey the existing tool and integration grants.

Under **Config → Notifications → Mail delivery receipts**, administrators see only
channel occupancy, attempt time, status and expiry. Message text, subjects,
recipients, event identities and provider errors are never included.

- Each of ntfy, Pushover, Telegram, WhatsApp, Discord, Web Push, Telnyx and CYD has
  2,000 slots. A full channel does not block another channel.
- An active attempt cannot be resolved. A locally rejected or definitely unsent
  attempt releases its reservation. A possible send with no confirmation stays
  uncertain, including after restart; it is never automatically replayed/expired.
- Confirmed deliveries keep the duplicate block for 30 days after confirmation.
  Old string receipts remain effective. Old records without a channel stay in a
  separate legacy pool, without consuming a modern channel's slots.
- **Mark delivered** preserves the block and starts its confirmed expiry.
  **Definitely not sent** removes the reservation and permits a later attempt.
  Resolve only from independent evidence. Neither action sends anything.
- If a response is lost or a revision conflicts, refresh and inspect the current
  state. Do not repeat the mutation automatically. A partial Telegram text/file
  delivery remains uncertain as one receipt, so its text cannot be replayed alone.

The existing `system_tasks.db` owns all records; there is no new database or
change to native tool arguments. A trusted mail turn clears its transient Inner
Voice state when the actual turn ends, including cancellation. User history and
delivery receipts remain intact.

## Administrator API

`GET /api/mail-notifications/receipts?channel=telegram&limit=50&cursor=...` returns
`receipts`, `channels` (counts/limits) and optional `next_cursor`. Omit the channel
for all channels, or use `legacy` for the old pool. Page sizes are 1–50. Responses
are `no-store`; IDs, revisions and cursors are opaque metadata.

`POST /api/mail-notifications/receipts/{id}/resolve` accepts exactly
`{"revision":"<current revision>","resolution":"sent"}` or `not_sent`.
Administrator authority and existing browser CSRF checks apply. Success is 204;
active attempts or stale revisions return 409, missing records 404 and invalid
arguments 400. No resolution endpoint invokes a sender.
