# Mailer

Mailer is the email application in the virtual Desktop's **Communication**
category. It uses the Desktop's Standard and Fruity themes and their mail icons.
The server synchronizes enabled accounts and processes the outbox even when the
Mailer window is closed.

## Attach files from Desktop apps

Choose **Email …** beside a Desktop app's export controls. Select its format and
review a separate Mailer draft with the generated file already attached. Recipients
and body remain empty, and sending is explicit. Current unsaved editor contents
are used where the app supports direct snapshots. Existing outgoing limits apply;
failed uploads retain retry/remove controls and block Send.

See [Desktop email exports](desktop-email.md) for the supported apps, formats and
failure behavior. Store apps and Tresor are excluded.

## Connect accounts

Open Mailer, enable it, and choose **Connect existing account** or **New account**.
The account picker shows the source, address, available operations, and access
restrictions before linking. Linking does not copy passwords or refresh tokens.

Supported account sources are:

- Classic IMAP or POP3 accounts with SMTP, including separate SMTP credentials.
- Google accounts managed by GoG and the existing Google Workspace integration.
  Both use the direct Gmail API; Mailer does not need shell access.
- The configured AgentMail inbox. Linking does not create another provider inbox.

New classic accounts are also available in Integrations. New Google API accounts
use GoG's multi-account management. New accounts do not grant general agent access
or enable the email watcher. Linking an existing account retains its existing
permissions. An address appearing in two integrations remains two distinct
connections; messages are not merged across accounts.

AgentMail replies use its native conversation endpoint when the subject stays
unchanged. A changed subject or sending account starts a new AgentMail
conversation while retaining the email's standard reply/reference headers.

**Remove from Mailer** unlinks the connection. It does not delete the shared
integration or a provider mailbox. Manage shared credentials and integration
access in the corresponding integration settings.

Test incoming and outgoing connections separately. These tests authenticate and
check capabilities without sending a message. Sending an actual test email is a
separate explicit action. Never disable certificate verification to work around
an incorrect hostname or an untrusted certificate.
Classic account TLS/STARTTLS choices also apply when explicitly enabling the
shared agent email tools or watcher; changing the port does not override an
explicit transport choice.

## OAuth application registration

OAuth requires the operator's own Google or Microsoft application registration.
Use the exact callback address shown during setup. Register that address with the
provider and keep the same external HTTPS origin when authorizing the account.
The callback path is `/api/desktop/mailer/oauth/callback`. Configure
`server.oauth_redirect_base_url` with the public HTTPS origin when using a reverse
proxy, and route this path to AuraGo. Mailer never derives its callback address
from an incoming Host or forwarded header.

Create a Google Web application client, enable the Gmail API for direct Gmail
accounts, and configure the consent screen and authorized redirect URI. The
operator's client secret and account tokens belong in AuraGo's encrypted Vault.
Google API permissions and Google IMAP/POP/SMTP permissions are different:
an existing Gmail API token is not automatically a protocol login. Google
documents the [web-server OAuth flow](https://developers.google.com/identity/protocols/oauth2/web-server)
and [Gmail XOAUTH2 permissions](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol).

For Microsoft, register an application for the intended tenant/account types and
grant the delegated permissions for the selected protocols. Tenant policies can
independently restrict SMTP AUTH or protocol access. Follow Microsoft's
[IMAP, POP and SMTP OAuth instructions](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Authorization uses one-use account-bound state and PKCE. A dedicated short-lived
callback cookie preserves the initiating browser binding without changing the
main session cookie policy. Cancelling authorization, removing permission, or
receiving insufficient scopes does not silently broaden access. Reauthorize when
the requested operation needs additional provider scopes.
Use **Authorize again** in Mailer's account settings to renew consent. Available
Gmail actions are the intersection of integration permissions and the actual
stored provider scopes. Permanent Gmail deletion additionally requires the
integration's Delete permission and explicit consent to the full mail scope;
ordinary GoG authorization continues to request its narrower API scopes.
Older Google Workspace tokens without a recorded provider scope set require
authorization again; Mailer does not infer an OAuth grant from local settings.

## Read and organize mail

The navigation combines all inboxes and provides unread, starred, snoozed, drafts,
and outbox views. Accounts retain their own folders or Gmail labels. Conversations
stay within the receiving account. Search results identify the search scope so
that locally indexed results are not presented as a complete provider archive.
Provider search is a toggle for one folder of one account and can load further
pages of the same search. The notice that more results may exist appears only
when another page can be loaded. A later page asks the provider for matches
that were not already returned. In a single account's inbox it uses that account's
inbox folder. The combined inbox of every account stays on the local index.
Changing account, view, or folder, or clearing the query, turns provider search off.
The list can show a short preview and an attachment count before the full
message is stored. Opening the message still loads the body.

At wide window sizes, navigation, message list, and reader are visible together.
Narrow windows progressively collapse navigation and use a back action between
the list and reader. Selection and scroll position remain stable during refresh.
Keyboard focus and status announcements remain available throughout the workflow.

Opening a message intentionally can mark it read when the account permits it.
Background IMAP fetches use `BODY.PEEK` and do not consume unread mail. A quiet
mailbox waits about five minutes before the next connection. New mail, an
unfinished page, a deletion, or an error brings the next check forward to about
a minute. Manual sync and the first check after AuraGo starts run immediately.
Each IMAP account uses one session for that pass and then looks only for newer
messages. If a command on that session fails, Mailer closes it and connects
again for the next folder. A mailbox identity change rebuilds the snapshot.
Messages removed on the server after that first snapshot are not removed
locally by later IMAP checks.

One message that cannot be read stays in the conversation with a notice, and
the other messages stay visible. If only the formatted part fails, the rest of
that message stays. A formatted part that can be shown is still shown. A POP3
message that cannot be parsed keeps its downloaded
copy; sender and subject come from its headers.

Remote images are blocked initially; sanitized HTML is displayed in an isolated
reader. Attachments and EML exports require the same authenticated access as
the mailbox.

POP3 downloads are permanent local mail. UIDL identifies messages when available;
without UIDL, complete content is compared before adding a new copy. POP3 folders,
read state, and organization are local to Mailer. Normal synchronization never
deletes a message from the POP3 server. Back up the Mailer directory: another
copy may no longer exist on the server.

Rules support sender, recipient, subject, and attachment conditions, with folder
or label actions, read state, starring, and archiving. Preview affected messages
before applying a rule to historical mail. Enabled rules otherwise apply to
new arrivals. Rules cannot forward mail or permanently delete messages.
Automatic rule progress is saved with newly synchronized messages. Interrupted
or failed actions remain visible on the message; already completed actions are
not repeated. Review an uncertain provider outcome before explicitly retrying
the current action. Searching older mail does not queue automatic rules.
Snooze hides a message inside Mailer until the chosen time without changing other
clients. Templates and signatures are account-specific.

## Compose and send

Messages open in a separate Desktop compose window. Choose the sender, recipients,
optional Cc/Bcc, subject, formatting, and attachments. Replies default to the
receiving account. Changing the sender visibly updates the account and signature.
Contacts provide recipient suggestions.

Drafts save after a short pause. A visible save failure requires attention before
closing; a conflicting revision cannot overwrite newer edits. Rich text uses the
bundled Milkdown/ProseMirror editor, and outgoing mail contains sanitized HTML
and a plain-text alternative. Imported HTML must not be silently flattened into
Markdown. Attachment uploads report progress and enforce size and storage limits.
After a concurrent edit, choose whether to keep your local composition against
the latest revision or reload the saved draft. Reloading asks before discarding
local edits; missing attachments remain visible until explicitly resolved.
Sending waits for uploads and forwarded attachment copies. Failed uploads and
attachments removed by another draft editor require attention before sending.

The persistent outbox distinguishes waiting, sending, sent, failed, and unknown
delivery. **Undo send** holds a newly queued message for ten seconds. It cancels a
queued submission; it cannot recall a message already delivered to a server.
The outbox loads older entries in pages; cancellation and retry address the
selected job directly, independently of the displayed page.

Scheduled messages retain their chosen time and timezone. After an outage,
unambiguously unsent overdue jobs run automatically when Mailer and the account
are enabled. Disabling Mailer pauses pending jobs. Missing attachments, revoked
permissions, or a disabled account stop a job before transmission.

If the connection fails after transmission may have succeeded, the result is
**Unknown**. Check the provider before deciding to create another send; Mailer
does not automatically replay an uncertain submission. A failed Sent-folder copy
is separate from delivery and never triggers a second email.

## Optional writing assistance

Ask for a draft from notes, a reply, improvements, a shorter version, another
tone, a translation, or a summary of selected conversation text. The assist panel
shows the selected provider and the content to be submitted before the request.
Attachments and the rest of the mailbox are not included automatically. If email
scanning by LLM Guardian is enabled, its provider and model are also disclosed;
the displayed text goes to that scanner before the selected writing provider.
A changed writing-provider or scanner configuration requires refreshing the
disclosure before use.

AI output appears as a proposal with **Apply**, **Discard**, and **Generate again**.
The existing draft stays unchanged until Apply. A proposal produced for an older
draft revision cannot replace newer edits. The assistant cannot alter recipients,
invoke tools, send mail, or store mail in general agent memory. Provider use is
accounted for through AuraGo's existing budget system and can be cancelled.

## Storage, permissions, and maintenance

Mailer stores its database and private attachment data below
`<data directory>/mailer/`. The database contains linked source references,
message metadata, list previews, read notices, durable POP3 content, drafts,
rules, templates, snoozes, cursors, and queued jobs. A preview or a read notice
does not replace the message body. Credentials remain in the existing encrypted
Vault entries.
Keep the Mailer directory and Vault in a coherent backup; never treat POP3 mail or
unsent drafts as disposable cache. Existing databases are backed up before schema
migration, and newer unsupported schemas fail closed.

Desktop read-only mode blocks writes, including drafts, rules, read markers,
account changes, and sends. Source account permissions and provider capabilities
also apply; local preferences can restrict but cannot expand them. One failing
account must not stop other accounts. Mailer API routes require an authenticated
administrator session or a `desktop:admin` token; downloads follow that boundary.

For development, run the focused Mailer, account/OAuth, existing email/watcher,
GoG/Gmail/AgentMail, Desktop, translation, UI bundle, and web-resource checks.
Browser fixtures use synthetic mail and exercise both themes, color schemes,
narrow/wide windows, keyboard navigation, and large lists. Live OAuth consent and
real delivery require separate acceptance using an operator-controlled test
account; passing local protocol fixtures is not proof of provider availability.

Calendar, PGP/S/MIME, Composio, and autonomous mailbox actions are outside this
version.
