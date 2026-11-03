# WhatsApp Setup Guide

AuraGo is a linked companion device named "AuraGo" on a WhatsApp account. It connects through the unofficial WhatsApp Web multi-device protocol, and it talks only to owner numbers: a message from anyone else is dropped and never answered.

## Before you start

- WhatsApp's terms forbid automated messaging, and WhatsApp can ban an account that automates. Use this channel knowing that risk.
- **A dedicated number is recommended.** Give AuraGo its own SIM or eSIM. A ban or a relink then never touches your personal account.
- You need:
  - a phone with WhatsApp, to link AuraGo and to receive its messages;
  - admin access to the AuraGo Web UI, because the link and status endpoints are administrator-only;
  - a data directory that AuraGo can write to (`directories.data_dir`, `./data` by default). The linked-device keys are stored there.
- The channel is not available in egg mode. In egg mode it reports `disabled` and starts nothing.

## Choose a mode

Set `mode` to one of two values:

- `dedicated` (recommended): AuraGo has its own number, and you write to that number from one of your `owner_numbers`. Up to 10 owner numbers are allowed, each in international format such as `+491701234567`. Only these numbers reach AuraGo.
- `self_chat`: AuraGo uses your personal number. Only your "Message yourself" chat reaches AuraGo, and every message AuraGo writes there starts with `self_chat_prefix`. The default prefix is a robot emoji and a space (`🤖 `). It can be up to 16 characters. Messages from your contacts never reach AuraGo in this mode. `owner_numbers` is not used here, because the linked account is the owner. The Security page warns about this mode, since AuraGo then runs on your personal account.

In self-chat mode, a message you write yourself that starts with the prefix is dropped (reason `own_prefix`). AuraGo uses that rule to recognise its own messages after its echo cache has forgotten them. Do not start your own notes with the prefix, and keep a prefix set: an empty prefix turns the check off.

## Step 1: Configure AuraGo

1. Open **Config → Communication → WhatsApp**.
2. Turn on **Enable WhatsApp**, choose the **Number** mode and, in dedicated mode, enter your **Owner numbers**.
3. Click **Save**. The link buttons stay disabled until the channel is turned on, configured and saved.

The same settings in `config.yaml`. The template ships with `enabled: false`, so set it to `true` before you link.

```yaml
whatsapp:
    enabled: false
    # dedicated: AuraGo has its own number and the owner writes to it.
    # self_chat: your personal number; only the "Message yourself" chat reaches AuraGo.
    mode: dedicated
    owner_numbers: ["+491701234567"]  # international format, e.g. "+491701234567"; required in dedicated mode
    readonly: false                    # true blocks send_whatsapp and notifications; replies to the owner still work
    send_read_receipts: true           # dedicated mode only
    self_chat_prefix: "🤖 "            # marks AuraGo's messages in self_chat mode
    # Link the account in the Config UI (QR code or pairing code). The linked-device
    # keys live in <data_dir>/whatsapp/device.db and are never backed up.
```

Saving a changed WhatsApp section restarts the channel. The linked device stays linked, so you do not link again.

## Step 2: Link the account

The **Linked device** panel on the Config page offers two ways to link. Both need the phone.

- **QR code** (**Link via QR code**): on the phone, open WhatsApp and go to Settings → Linked devices → Link a device. Then scan the code that AuraGo shows. The code rotates, so scan the one on screen.
- **Pairing code** (**Link via phone number**): enter the number of the WhatsApp account that AuraGo joins (AuraGo's own number in dedicated mode, your personal number in self-chat mode) in international format. On the phone, choose Linked devices → Link a device → Link with phone number instead, then enter the code that AuraGo shows.

Rules for linking:

- A link session lasts 3 minutes. If it expires, start again.
- Passkey linking is not supported. If the phone asks for a passkey, AuraGo says so. Use the QR code instead.
- If a device is already linked, AuraGo asks for confirmation first. A second link replaces the first after you confirm.
- QR codes and pairing codes appear only on the Config page. AuraGo never logs or stores them.

## Step 3: Send a test message

When the status shows `healthy`, press **Send test message**. AuraGo sends the test text to every owner number (in self-chat mode, to your own account). The button works at most once a minute, and the newspaper's WhatsApp test shares that minute.

## Phone rules

- **Open WhatsApp on the phone at least every 14 days.** Otherwise WhatsApp removes the linked devices, and you link AuraGo again (see Step 2).
- An account allows up to 4 linked devices. Every linked device counts toward that limit, so keep a slot free for AuraGo.
- AuraGo appears as "AuraGo" in the Linked devices list on the phone.

## No bulk sending

- AuraGo sends only to the owner numbers (in self-chat mode, to your own account). No tool, flow or API message reaches other numbers.
- **You must never bulk-send** or script mass messages through the linked device. WhatsApp answers bulk sending with temporary bans.
- A temporary ban carries a WhatsApp reason code. Codes 101–106 are tied to bulk and spam behaviour. AuraGo shows the account as `temp_banned` and keeps the code in `last_error` as `temp_ban_<code>`.
- AuraGo paces its sends to 20 messages per minute, with a burst of 5. The pacing limits the rate. It does not make bulk sending safe.

## Read-only mode

With `whatsapp.readonly: true` (**Read-only** in the Web UI), AuraGo blocks every message it starts itself: `send_whatsapp`, notifications, flows, camera alerts and newspaper delivery. Replies to the owner still work.

## What reaches WhatsApp

Into AuraGo, from an owner number (or from your own chat in self-chat mode):

- text, and the captions of media;
- voice notes and audio files, transcribed with AuraGo's speech-to-text;
- images, described by the vision model;
- documents, saved to `agent_workspace/workdir/attachments/`;
- videos and stickers, which get a placeholder only; AuraGo does not process them;
- locations (as coordinates), contact cards (as third-party data) and quoted replies (the quoted text goes in front of your message);
- forwarded messages, which are third-party content. AuraGo reads them, but runs no commands from them and does not treat them as answers to its questions.

Groups, broadcast lists, newsletters and status updates are ignored.

Out of AuraGo, to the owners:

- answers as plain text without link previews, split into several messages when they are long; up to five images from an answer, while further images and any image that cannot be sent appear as text lines; and voice notes when voice mode (`/voice`) is on;
- `send_whatsapp`, `send_notification` with channel `whatsapp`, camera alerts with the photo, the `notify.whatsapp` flow node, the `whatsapp` value of `notify_on_error`, and newspaper delivery;
- files of up to 50 MB, which go out only through AuraGo's outgoing-attachment check.

## Status and troubleshooting

The **Linked device** panel on the Config page shows the current state. `GET /api/whatsapp/status` (administrators only) returns the same snapshot as JSON. The `integration:whatsapp` self-test reads that status only and never sends a message. The nightly maintenance briefing adds a `whatsapp` line to its integration checks (see "The morning briefing line" below).

| State | What it means | What to do |
|-------|---------------|------------|
| `disabled` | WhatsApp is off, or egg mode is on | Turn WhatsApp on and save. |
| `not_configured` | No data directory, or dedicated mode without a valid owner number | Add an owner number in international format and save. |
| `not_linked` | Enabled and configured, but no device is linked | Link the account (Step 2). |
| `linking` | A link session is running | Scan the QR code or enter the pairing code within 3 minutes. |
| `connecting` | Linked, and the connection is opening | Wait a minute. If it stays here, check the network and the log. |
| `healthy` | Connected and synchronised with WhatsApp | Nothing to do. Messages flow in both directions. |
| `degraded` | The connection dropped, and reconnects run by themselves | Wait. If it lasts, check the server's internet access and that the phone is online. |
| `logged_out` | WhatsApp no longer has this device | Link again (Step 2). |
| `conflict` | Another client uses the same linked device | Stop the other client, then unlink and link again. |
| `temp_banned` | WhatsApp has temporarily blocked the account | Stop all automated sending. AuraGo retries after the expiry shown in the status. Without an expiry, restart AuraGo once the block should be over. |
| `outdated` | WhatsApp rejects this AuraGo build as outdated | Update AuraGo to a release with a newer WhatsApp library. A setting cannot fix this. |

The counters in the status show what AuraGo has dropped or failed to deliver:

- `messages_received` counts every inbound message event before filtering.
- `dropped_non_owner` counts messages from numbers that are not owner numbers. AuraGo does not reply to them.
- `dropped_unresolved_lid` counts messages whose sender AuraGo cannot match to a phone number.
- `dropped_filtered` counts every other drop by its reason, for example `group`, `broadcast`, `newsletter`, `echo`, `duplicate`, `stale` (queued while AuraGo was offline), `own_prefix` and `not_self_chat`.
- `delivery_failures` counts messages that WhatsApp did not accept.

The counters reset when AuraGo restarts. Logs and status show only the last four digits of a number.

### The morning briefing line

The maintenance briefing lists every integration check as `- <name>: <result>`, with the code in parentheses when there is one. The WhatsApp check is named `whatsapp`. It reads the same passive status as the status endpoint, so it never connects, links or sends, and the briefing cannot message you.

| Briefing line | State behind it |
|---------------|-----------------|
| `- whatsapp: skipped (disabled)` | `disabled`: WhatsApp is off, or egg mode is on. |
| `- whatsapp: skipped (not_configured)` | `not_configured`: no data directory, or dedicated mode without a valid owner number. |
| `- whatsapp: passed` | `healthy`: the linked device is connected. |
| `- whatsapp: failed (<code>)` | Every other state: `not_linked`, `linking`, `connecting`, `degraded`, `logged_out`, `conflict`, `temp_banned` and `outdated`. |

On a failed line, the code is the last error code from the status, for example `temp_ban_101` or `keepalive_timeout`. When the status holds no error code, the code is the state name. An enabled channel that you have not linked yet therefore shows `failed (not_linked)`, not a skip. The check passes along no number, no device ID and no message text.

## Unlink, relink and backups

- **Unlink** removes AuraGo's keys. When AuraGo is connected, WhatsApp also removes the device from the phone. When it is not connected, AuraGo deletes only its local keys, and the panel tells you to remove AuraGo under Linked devices on the phone yourself.
- **Relink** by linking again. If a device is still linked, AuraGo asks for confirmation, and the new link replaces the old one.
- `<data_dir>/whatsapp/device.db` is never backed up. After a restore, or after you move AuraGo to another machine, link the account again.

## Security and privacy

- The device store, `<data_dir>/whatsapp/device.db`, is a credential. Anyone who holds it can act as the linked device. The directory is created with mode 0700 and the file with 0600. On Windows both get an owner-only access list.
- Agent file tools, outgoing attachments and Docker host access refuse `device.db` and its SQLite side files (`-wal`, `-shm`, `-journal`).
- Shell and Python run under `agent.allow_unsafe_host_execution` can read the device store, as they can read the vault. Keep that setting off unless you accept the risk. See [Chapter 14: Security](manuals/en/14-security.md).
- Logs and status show only the last four digits of a number. The text of dropped messages is never logged.
- While WhatsApp is on, the Security page warns about two risky setups: `whatsapp_self_chat_personal_account` (self-chat mode on your personal account) and `whatsapp_dedicated_no_owner` (dedicated mode without a valid owner number, so AuraGo answers nobody).

## License

- AuraGo's own source code is MIT-licensed (`LICENSE`).
- Release binaries link `go.mau.fi/libsignal`, which is GPL-3.0, and the GPL-3.0 scanners of `github.com/odvcencio/gotreesitter` (native code graph). The release binaries are therefore GPL-3.0 combined works, and they are distributed under GPL-3.0 terms.
- The third-party notices are in [THIRD_PARTY_NOTICES.md](../licences/THIRD_PARTY_NOTICES.md). They name the GPL-3.0 text, `licences/texts/GPL-3.0-only.txt`.
- Each release's notes link the tagged source of that release.
