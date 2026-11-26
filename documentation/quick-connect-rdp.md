# Quick Connect RDP access

Quick Connect supports three remote protocols: SSH, VNC and RDP. RDP opens the
Windows remote desktop of a device from your inventory in the browser. Select
the **RDP** filter chip to show only RDP devices, and choose **RDP** as the
protocol in the server dialog. The default port is 3389.

The browser does not connect to the Windows machine directly. The connection
goes through AuraGo, which connects to the device, negotiates the RDP session
with the server and relays the encrypted stream to your browser. The browser
cannot pick another target: AuraGo only connects to the device stored in the
inventory.

The RDP client is [IronRDP](https://github.com/Devolutions/IronRDP) (MIT OR
Apache-2.0), compiled to WebAssembly and shipped with AuraGo under
`ui/js/vendor/ironrdp/`. No Internet access is needed. The WebAssembly module is
about 4.3 MB and is loaded only when an RDP session starts.

## Windows setup

RDP needs a Windows edition that can host remote sessions. Windows Pro,
Enterprise and Server can host RDP; Windows Home cannot. Check the following
on the target machine:

1. Open **Settings → System → Remote Desktop** and turn Remote Desktop on.
2. Keep **Require devices to use Network Level Authentication** on. Quick
   Connect signs in with NLA (CredSSP/NTLM).
3. Allow TCP port 3389 in the firewall, at least from the AuraGo host.
4. The account needs a password. Accounts without a password cannot sign in
   over RDP.
5. The account must be allowed to sign in remotely, for example by being a
   member of the **Remote Desktop Users** group (or being an administrator).

Microsoft accounts sign in with the Microsoft account e-mail address and its
password, not with the Windows Hello PIN.

xrdp on Linux works too, as long as it offers TLS and NLA.

## Adding the device and connecting

In Quick Connect, click **+** (Add Server) or edit an existing entry, choose
**Protocol: RDP** and fill in:

- **Host / IP**: the name or IP address AuraGo can reach.
- **Port**: 3389 unless the server listens elsewhere (filled in when you pick RDP).
- **Username**: either `DOMAIN\user` (AuraGo splits it into domain and user)
  or `user@domain`. For a local account, use the plain user name.
- **Password**: optional. When you save one, it goes into the vault as the
  device's login credential; when you leave it empty, Quick Connect asks for it
  on every connection.

The device appears as connectable in Quick Connect when a credential or user
name is set, or when a probe sees an RDP server answering on the port. The probe
only checks that the server confirms an X.224 connection. It does not attempt a
login.

Select the device and start the RDP connection. The browser loads the client,
opens a WebSocket to `/api/desktop/rdp` and shows the sign-in state. If a
password is not available, Quick Connect asks for it in a dialog.

### Toolbar

- **Fit** scales the remote desktop to the window. **1:1** shows it at its
  native size.
- **Ctrl+Alt+Del** sends the secure attention sequence to the remote machine.
- **Win** sends the Windows key.
- **Reconnect** starts a new session with the same device.

The remote desktop resizes to the window (display control), so the session
follows the size of the Quick Connect window.

## Passwords

AuraGo handles passwords in one of two ways.

**Stored password.** If the device's linked credential has a stored password,
AuraGo hands it to your browser once per connection through a one-time ticket.
The ticket is valid for 60 seconds, can be used once, is bound to the device and
is not cached. Each issue is recorded in the desktop audit log as
`desktop_rdp_ticket_issued`. At most 10 tickets per device can be issued per
minute.

Tickets are issued only for a logged-in browser session with authentication
enabled, or for a token with the `desktop:admin` scope. In all other cases, and
whenever no password is stored, Quick Connect asks for the password in a dialog.
The typed password is not stored.

If a ticket expires before the connection starts, reconnect; a new ticket is
issued.

## Certificate pinning

RDP servers normally use self-signed certificates, so AuraGo cannot check them
against a certificate authority. Instead it uses trust on first use:

- On the first successful connection, AuraGo stores the SHA-256 fingerprint of
  the server certificate for that device.
- On later connections the fingerprint must match. If it changes, the
  connection is refused with the message "The remote computer's certificate has
  changed…".

A changed certificate is not always a problem. Windows renews its self-signed
RDP certificate periodically, and a reinstall or a changed server replaces it.
Before accepting the new certificate, confirm that the change is expected.
Then click **Trust new certificate**. This needs a logged-in session or the
`desktop:admin` scope, and the action is recorded as `desktop_rdp_pin_reset` in
the audit log.

Deleting the device removes its pin.

TLS between AuraGo and the RDP server uses TLS 1.2. Windows RDP servers abort
TLS 1.3 handshakes from Go clients, so TLS 1.2 is required.

## File access

The **Files** tab sits next to **Desktop** in an open RDP session. Use it to
browse folders, upload and download files, and create, rename, copy, move and
delete them. Files are reached on the same host over SFTP or SMB. They do not
go through the remote desktop.

The Files tab is always shown for RDP devices. When file access is off, the tab
says so and offers an **Edit device** button.

### Modes

Set the mode for each device in the server dialog:

| Mode | What AuraGo does |
| --- | --- |
| **Off** | No file access. AuraGo does not connect to ports 22 or 445 for this device. |
| **Automatic (SFTP, then SMB)** | Tries SFTP on the SFTP port first. If no SSH server answers, tries SMB on port 445. |
| **SFTP (SSH server)** | Uses SFTP only. |
| **SMB share (Windows)** | Uses SMB only. |

In Automatic mode, AuraGo remembers which protocol answered for 5 minutes.

The other fields are:

- **SFTP port**: default 22. Used in Automatic and SFTP modes.
- **SMB share**: default `C$`. Used in Automatic and SMB modes.
- **Credential for file access**: default **Same as RDP login**. Pick a saved
  credential here if the file login is different.

New RDP devices start in Automatic mode. RDP devices that already exist start
in Off, so AuraGo makes no new connections until you turn file access on.

The password stays on the AuraGo server. The browser never receives it.

### Credential for file access

GNOME Remote Desktop has its own RDP login. It usually differs from the Linux
account. For a GNOME host, choose a credential that holds the Linux account
here. AuraGo ignores the host stored in that credential and uses the host of
the RDP device.

### Linux hosts

- Install an SSH server with SFTP support. On Debian and Ubuntu, run
  `sudo apt install openssh-server`. Ubuntu Desktop does not run an SSH server
  by default.
- SFTP is limited to the home folder of the account.

### Windows hosts

Choose one of these options:

- **SMB share (recommended).** Share a dedicated folder, and use a dedicated
  account with access only to that share. For example:
  `New-SmbShare -Name "AuraGo" -Path "D:\AuraGo" -FullAccess "LAB\auragofiles"`.
  Then enter the share name in AuraGo.
- **C$ (default admin share).** Needs an administrator account. For a local
  account that is not a domain account, UAC remote restrictions remove the
  admin rights over the network. C$ is then denied unless
  `LocalAccountTokenFilterPolicy` is set to 1. That setting lowers protection,
  so use a dedicated share where you can.
- **OpenSSH Server feature.** Install it under **Settings → System → Optional
  features**, or with `Add-WindowsCapability`. Automatic mode then uses SFTP.

SMB has these rules:

- SMB signing is always required.
- Encryption depends on the server. To require it for all shares, run
  `Set-SmbServerConfiguration -EncryptData $true`. To require it for one share,
  run `Set-SmbShare -Name "AuraGo" -EncryptData $true`.
- Only NTLM logins work. Kerberos is not supported yet.
- SMB uses port 445 only. Enter an IPv4 address or a host name. IPv6 addresses
  are not supported.
- Microsoft accounts sign in with the e-mail address and the account password.

### Who can use file access

File access to an RDP device needs a signed-in browser session with
authentication enabled, or a token with the `desktop:admin` scope. Other
tokens are refused. This is the same rule that applies to RDP passwords.

After a rejected login, file access pauses for 30 seconds. This protects the
account from lockouts.

### Trusting the SSH host key

The first SFTP connection to a host needs your approval. AuraGo shows the
fingerprint of the host key. Compare it with the key on the server before you
confirm. On a Linux host, `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`
shows the fingerprint.

After you confirm, AuraGo stores the key in its known_hosts file and uses it
from then on. If the key later changes, access is blocked. The dialog does not
offer an override. Check that the change is expected, such as a reinstall, and
then fix the entry in known_hosts on the AuraGo host.

### Transfers and timeouts

- Uploads are limited to 50 MiB per file.
- The RDP session closes after it has been idle for a while (5 minutes by
  default). Browsing in the Files tab does not count as input to the remote
  desktop, so the desktop can disconnect while you work in Files. File access
  does not depend on the RDP session. Click **Reconnect** to get the desktop
  back.

## Security notes

- The password is only sent to the browser when the session rules above allow
  it, and only as a one-time ticket. Do not share a browser session that has
  access to a stored password.
- Pinning protects against a server being swapped after the first connection.
  It does not verify the server's identity on the first connection. Connect for
  the first time on a network you trust.
- Passwords typed into the dialog are used for the current connection only.
- Use a dedicated account with the minimum rights needed for remote sign-in
  rather than a full administrator account, where possible.

## Troubleshooting

| Message or symptom | Meaning | What to do |
| --- | --- | --- |
| Wrong password / logon failure | The server rejected the user name, domain or password. | Check the user name, the domain (`DOMAIN\user` or `user@domain`) and the password. For Microsoft accounts, use the account e-mail address. |
| Access denied | The account is not allowed to sign in remotely. | Add the account to **Remote Desktop Users** or use an administrator account. |
| Target unreachable | AuraGo could not open a TCP connection to host and port. | Check the host name, the port, the firewall and that Remote Desktop is on. |
| DNS failed | The host name could not be resolved on the AuraGo host. | Use an IP address, or fix DNS for the AuraGo host. |
| TLS failed | The TLS handshake with the server failed. | Check that the server offers TLS 1.2. Windows RDP servers that only offer TLS 1.3 cannot be used. |
| Negotiation failed | The server requires a security protocol the client does not offer. | Turn on Network Level Authentication, or enable a protocol that AuraGo supports. "RDP security" alone is not enough. |
| Ticket expired | The one-time password ticket was older than 60 seconds or already used. | Click **Reconnect**. |
| The remote computer's certificate has changed… | The stored fingerprint no longer matches the server certificate. | Check that the change is expected (renewal, reinstall). Then click **Trust new certificate**. |
| Device not shown as connectable | No credential or user name is set, and no RDP server answered the probe. | Set a user name or linked credential, or check the port and firewall. |
| File access is off for this device. | The device's file access mode is **Off**. | Click **Edit device**, choose a mode other than Off and save. |
| File access to RDP devices needs a signed-in administrator session. (`file_access_admin_required`) | The browser session is not signed in, authentication is disabled, or the token lacks `desktop:admin`. | Sign in with authentication enabled, then try again. |
| No SFTP or SMB service answered on this host. (`file_access_unavailable`) | Automatic mode found no SSH server on the SFTP port and no SMB server on port 445. | Install and start an SSH server, or turn on file sharing on Windows. Check the port and the firewall. |
| Unknown host key prompt. (`unknown_host_key`) | This is the first SFTP connection, and the host key is not trusted yet. | Compare the fingerprint with the key on the server. Confirm only if it matches. |
| The SSH host key of this host has changed. (`host_key_changed`) | The key in known_hosts no longer matches the server. | Check that the change is expected, such as a reinstall or renewed key. Then fix the entry in known_hosts on the AuraGo host. Access stays blocked until then. |
| The host key changed while you were confirming it. (`host_key_mismatch`) | The key the server showed in the trust step is not the key you confirmed. | Try again and check the new fingerprint against the server. |
| The host rejected the stored credential. Try again in 30 s. (`file_access_auth_backoff`) | The login failed. File access pauses for 30 seconds to avoid lockouts. | Check the user name, password and file-access credential. Wait for the pause to end. |
| Access denied on C$ | The account has no admin rights over the network. This is usually UAC remote restrictions on a local account. | Use a dedicated SMB share, or set `LocalAccountTokenFilterPolicy` to 1 (lowers protection). |

## Limits

The current version has these limits:

- No clipboard sharing.
- File transfer works through the Files tab. Clipboard file transfer is not
  supported yet.
- No audio.
- No printer redirection.
- No multi-monitor support.
- Agent tools do not use RDP. The AI agent cannot open or control RDP sessions.
