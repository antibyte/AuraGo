# Chapter 28: Mailer – email on the desktop

Mailer is the email app in the Desktop's Communication category. It shows your existing mailboxes (IMAP or POP3 with
SMTP, Google, Microsoft and AgentMail) in one window and keeps them in sync in the background, even while the window is
closed. It is for reading and writing your own mail. It is not a calendar, and it has no PGP or S/MIME support.

Technical details for administrators (configuration, OAuth callback address, storage, API routes) are in the
[operator reference](../../mailer.md).

## Table of Contents
1. [What You Need](#what-you-need)
2. [First Steps](#first-steps)
3. [Connecting Accounts](#connecting-accounts)
4. [Reading and Organizing Mail](#reading-and-organizing-mail)
5. [Composing and Sending](#composing-and-sending)
6. [Writing Assistance](#writing-assistance)
7. [Background Sync and Agent Access](#background-sync-and-agent-access)
8. [Good to Know](#good-to-know)
9. [Privacy and Safety](#privacy-and-safety)
10. [When Something Goes Wrong](#when-something-goes-wrong)

---

## What You Need

Mailer is a built-in Desktop app with no switch of its own. It is there whenever the Virtual Desktop is on, and it
depends on these settings:

- **Config → Virtual Desktop → "Enable Virtual Desktop"** (`virtual_desktop.enabled`; the shipped `config_template.yaml` turns it on, a config
  without the key leaves it off). While the desktop
  is off, Mailer refuses all changes.
- **Mailer → Settings → Accounts → "Sync in the background"**. This is Mailer's own on/off switch. It is off on a new
  installation. While it is off, Mailer does not sync or load message bodies, and it refuses changes and sends. You can
  still link accounts. The value is stored in Mailer's database, not in `config.yaml`.
- **Config → Virtual Desktop → "Read-only mode"** (`virtual_desktop.readonly`). Mail stays readable, but drafts, rules,
  read marks, account changes and sends are blocked.

Each account type has its own prerequisites:

- **Classic IMAP or POP3 accounts** need server names and a password. Use TLS or STARTTLS; see [Email Configuration](./08-integrations.md#email-configuration).
- **Google accounts** need the gog integration (**Config → Google Workspace via gog**) or **Config → Google Workspace**,
  with mail permissions for the account. Gmail API accounts also need **Config → Danger Zone → "Network Requests
  (api_request)"** (`agent.allow_network_requests`) switched on. That switch also lets the agent send arbitrary HTTP
  requests, so weigh it first. See [Google Workspace Through gog](./08-integrations.md#google-workspace-through-gog).
- **Microsoft accounts** need your own Microsoft app registration (client ID and secret).
- **AgentMail** needs **Config → AgentMail** with **Enabled** on and a primary inbox ID. Mailer links that inbox; it does
  not create one. See [AgentMail Integration](./08-integrations.md#agentmail-integration).

## First Steps

1. Open **Mailer** from the start menu, in the **Communication** category. It opens maximized.
2. Open **Settings**, select the **Accounts** tab and turn on **Sync in the background**.
3. Add an account with **Add email account**, **Add Google account** or **Add Microsoft account**, or link one under
   **Connect existing account**.
4. Under **Connected accounts**, run **Test receiving** and **Test sending**. The tests log in and check capabilities;
   they do not send a message.
5. The list shows all inboxes, plus **Unread**, **Starred**, **Snoozed**, **Drafts** and **Outbox**.

## Connecting Accounts

A source is an account an integration already knows: a classic account, a GoG account, a Google Workspace account or the
AgentMail inbox. Linking does not copy passwords or tokens into Mailer.

### Classic IMAP or POP3 accounts

Click **Add email account** and fill in the account name, the email address, the **Receiving protocol** and **Receiving
security** (**TLS from connection start** or **STARTTLS**), the incoming server and port, the **Outgoing SMTP server**,
its port and **Sending security**, plus **Username** and **Password**.

- The **Incoming port** field is shared by both protocols and starts at 993. Set 995 for POP3 with TLS, 143 for IMAP with
  STARTTLS or 110 for POP3 with STARTTLS.
- The password is stored in the Vault as `email_<id>_password`. SMTP username and SMTP password are optional; the SMTP
  password is stored as `email_<id>_smtp_password`.
- The new account starts with agent access and the inbox watcher off. If Mailer cannot connect, the account stays saved
  with the message "The account was created, but its connection could not be completed."

**Known issue in this build:** the form sends **Password** as the SMTP authentication, which the server does not accept
for classic accounts yet. Until this is fixed, add the account under **Config → Email** with the same details.

### Google accounts

**Add Google account** offers **Gmail API** (the default), IMAP and POP3. Pick an existing GoG or Google Workspace source in
the list, or **Set up Google OAuth** to use your own client ID and client secret (stored in the Vault). New Google accounts
may read, send, change read state, labels and folders, but not delete permanently. For the redirect address, see the [operator reference](../../mailer.md#oauth-application-registration).

### Microsoft accounts and AgentMail

**Add Microsoft account** needs your own Microsoft app registration. Receiving uses IMAP or POP3, and sending uses
Microsoft's SMTP server. Tenant policies can still block SMTP sign-in.

AgentMail appears as a source once a primary inbox is configured. Replies stay in the AgentMail conversation while the
subject is unchanged.

### Linking, unlinking and account options

**Connect** links a source. **Disconnect** removes a connected account from Mailer. The confirmation reads: "Remove this
account from Mailer? Its shared integration and stored mail remain intact." The integration and the provider mailbox are
not deleted.

Each connected account also has these options under **Settings → Accounts**: **Disable account**, **Read-only account**
(Mailer can only restrict an account, never widen it), **Allow agent access** (classic IMAP password accounts only, see
[Background Sync and Agent Access](#background-sync-and-agent-access)), **Re-authorize** when the provider needs new
permissions, and **Folder mapping** for Archive, Drafts, Junk, Sent and Trash.

## Reading and Organizing Mail

The list can show all accounts at once. Each account keeps its own folders or Gmail labels, and a conversation stays in
the account that received it.

- **Search** covers messages already synced locally: "Search covers only messages synced locally."
- **Search provider** asks the provider for more results in the account and folder you are viewing, and can load further
  pages. Changing the account, view or folder, or clearing the search, turns it off.
- **Reply**, **Reply to all**, **Forward**, **Archive**, **Trash**, **Move to…**, **Mark as read**, **Star** and labels
  work on a message. **Delete** asks "Delete this message permanently?" first; use **Trash** if you may want it back.
- **Snooze** hides a message inside Mailer until the time you choose. **Download EML** saves a message as a file.
  **Print** opens a preview, so allow pop-ups.

Opening a message marks it as read if the account allows it. Background IMAP checks do not change read state. The list
shows a preview and an attachment count before the body loads. A message that cannot be read stays in the conversation
with a notice. Remote images are blocked at first; **Load external images** allows them for one conversation. Loaded
images come straight from the sender's server.

**Rules** (**Settings → Rules**) organize new arrivals. A rule matches **From**, **To**, **Subject**, **Message body** or
**Has attachments**, and can mark, star, move, archive, trash, label or snooze the message. **Preview affected messages**
shows what a rule would change in existing mail before **Apply to selected messages**. Rules never forward or permanently
delete. A failed action stays visible under **Message rules**, where you can retry it. **Settings → Templates** and
**Settings → Signatures** store recurring text; a signature belongs to one sending account.

## Composing and Sending

Click **Compose**. The message opens in its own Desktop window, titled **New message**. **From** chooses the sending
account and updates the signature. Contacts suggest recipients, and **Add Cc** and **Add Bcc** show the extra fields.

Bold, italic, headings, quotes and lists are available. Imported HTML stays as it is until you choose **Convert to
editable text**. **Attach file** uploads files with a progress indicator. **Stickers** inserts an image from the Desktop
sticker library; it is sent as an inline PNG named `sticker-<id>.png`.

Drafts save shortly after your last change ("Draft saved"). If another window saved the draft first, choose **Keep my
edits** or **Load the other version**.

- **Send** puts the message in the outbox. **Undo send** cancels it within ten seconds. After that, Mailer hands the
  message to the provider, and undo can no longer recall it.
- **Send later** keeps the chosen time and time zone. Overdue jobs run after an outage if Mailer and the account are
  enabled; switching Mailer off pauses them.
- The outbox shows **Scheduled**, **Sending**, **Sent**, **Failed**, **Cancelled** and **Delivery status unknown**.
  **Retry sending** requeues a failed message.
- **Delivery status unknown** means the connection broke after the provider may have accepted the message. Mailer does
  not resend it by itself. Check the provider's Sent folder first.

## Writing Assistance

**Writing assistance** in the compose window offers **Write from notes**, **Draft a reply**, **Improve writing**,
**Shorten**, **Change tone**, **Translate** and **Summarize conversation**. It uses the AI provider you pick in the panel,
by default AuraGo's main provider. There is no separate switch; the panel is available while the desktop is on and not
read-only.

Before a request is sent, the panel shows the provider and the exact draft and selected context. Attachments and the rest
of the mailbox are never included. If LLM Guardian scans emails, the text goes to the scanner first, and the panel shows
its provider and model too. Content the safety scan flags is refused.

The result is a proposal. **Apply suggestion** puts it into the draft; **Discard** drops it. A proposal made for an older
draft cannot overwrite newer edits. The assistant has no tools and cannot change recipients, send, read other mailboxes
or store memory. Its use is counted in AuraGo's budget under the category `mailer`.

## Background Sync and Agent Access

While **Sync in the background** is on, AuraGo syncs each account whether or not the Mailer window is open. A quiet
mailbox is checked about every five minutes. New mail, an unfinished page, a deletion or an error brings the next check
forward to about one minute. A manual sync and the first check after startup run at once. Open windows update live.

**Agent access.** The agent has its own email tools. They work with the accounts in **Config → Email** (`email_accounts`)
that allow agent access:

- **Allow agent access** sets `agent_access` for an account. Accounts created in Mailer start with it off. Older IMAP
  accounts without that entry keep the access they had.
- POP3 accounts and accounts that use OAuth or XOAUTH2 never give the agent access, even when the setting is on.
- A read-only account lets the agent read mail, but not send it.

**Inbox watcher.** Under **Config → Email → "Inbox Watcher"**, switch on **"Watch enabled"** and set **"Watch Folder"**
(INBOX by default) and **"Interval (seconds)"** (120 by default). The watcher checks IMAP accounts that allow agent
access and tells the agent about new mail, which it can then fetch. Flagged messages are reported separately. In the same
section, **"Account enabled"** switched off disables an account, and **"Allow sending emails"** switched off makes it
read-only.

## Good to Know

| Item | Limit |
|---|---|
| Attachment, per file | 25 MiB |
| Attachments, per outgoing message | 25 MiB together |
| Outgoing message, including text | 32 MiB |
| Attachments stored locally, in total | 512 MiB |
| Writing assistant input | 32 KB draft and 32 KB context |
| Classic email accounts in Mailer | 100 |
| GoG accounts in total | 20 |
| IMAP message that can be loaded | 16 MiB |
| AgentMail attachments (sent or downloaded) | 25 MiB, regardless of `agentmail.max_attachment_mb` |

- **POP3 mail lives on your computer.** Folders and read state are local, and sync never deletes messages on the server.
- **Not in this version:** calendar, PGP and S/MIME, and autonomous mailbox actions by the assistant.

## Privacy and Safety

**What leaves the machine.** Mail traffic goes to the servers you configure: IMAP, POP3 and SMTP, the Gmail API for Google
accounts, Microsoft's servers for Microsoft accounts, and the AgentMail API. Writing assistance sends the displayed draft and context to your chosen provider, and to the email scanner if
LLM Guardian scans emails. Remote images load from the sender's server; AuraGo does not proxy them, so the sender sees your
network address and when you opened the message.

**What is stored where.**

- `data/mailer/mailer.db`: metadata, list previews, POP3 copies, drafts, rules, templates, signatures, outbox jobs and sync
  positions.
- `data/mailer/blobs/`: attachments.
- Before a schema upgrade, Mailer saves a backup beside the database (`mailer.db.pre-v<n>-<timestamp>.bak`, owner-readable
  only).
- Passwords, client secrets and OAuth tokens stay in the Vault: `email_<id>_password`, `email_<id>_smtp_password`,
  `email_<id>_oauth_client_secret` and `oauth_gog_client_<id>`. The configuration keeps only hosts, protocols and references.
- Back up the Mailer directory and the Vault together. POP3 mail and unsent drafts are not a cache.

**Who can use Mailer.** The Mailer API requires an administrator session or a token with the `desktop:admin` scope, and
changes need a same-origin request. Mailer has no audit log of its own in this version.

**What the agent can and cannot do.** The agent reaches only accounts that allow agent access, through its own email
tools, which do not read Mailer's database. The writing assistant cannot send, move or read other mail. Rules never
forward or permanently delete. If a server's certificate fails the check, fix the host name or trust the certificate.

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "Could not load mail" | Click **Retry**. If it keeps failing, test the account under **Settings → Accounts**. |
| "Mailer is read-only. Changes are disabled." | Desktop read-only mode or a read-only account blocks the change. The same text appears when **Sync in the background** is off, so check that first. |
| "The action could not be completed" | Check the account status under **Settings → Accounts**, then try again. For a classic account, see the known issue above. |
| "Background synchronization paused" | Turn on **Sync in the background** under **Settings → Accounts**. |
| "Connection test failed" | Check host, port, security mode, username and password. The message shows an error code; use it to find the cause. |
| "Connection test partly passed" | Receiving or sending works, but not both. Test each direction on its own. |
| "The account was created, but its connection could not be completed." | The account is saved. Fix the settings and connect it again under **Connect existing account**. |
| "Sign-in is required or has expired. Authorize this account again." | Click **Re-authorize** under **Settings → Accounts**. |
| "Network access is disabled in AuraGo settings." | An administrator enables **Network Requests (api_request)** in the Danger Zone. Gmail accounts need it. |
| "Allow pop-ups to open the authorization window." | Allow pop-ups for AuraGo, then click **Continue in the authorization window**. |
| "Send error: {code}" | Click **Retry sending**. The code comes from the provider or the connection. |
| "Upload failed. Retry or remove this file before sending." | Retry the upload or remove the attachment. A file may be at most 25 MiB. |
| "This draft changed elsewhere. Reload it before saving again." | Choose **Keep my edits** or **Load the other version**. |
| "Local Mailer storage is full. Free up space before syncing." | Free disk space, or remove old attachments and mail. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) ·
[Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 27: Bookshelf](./27-bookshelf.md)
**Next Chapter:** [Chapter 29: Infra](./29-infra.md)
