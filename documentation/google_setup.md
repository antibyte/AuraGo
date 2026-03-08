# Google Workspace OAuth 2.0 Setup Guide (Headless / Remote Server)

This guide explains how to set up Google OAuth 2.0 credentials for this project. Since this agent is designed to also run on headless servers without a GUI browser, we use the **Desktop App** credential type combined with a **Localhost Redirect** workaround. 

This allows the agent to read your Gmail, Calendar, and Drive files autonomously.

To use this feature you first need to set the `enable_google_workspace` section in `config.yaml`:

```yaml
agent:
  enable_google_workspace: true
``` 

## Step 1: Create a Google Cloud Project & Enable APIs

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Click the project dropdown in the top-nav bar and select **New Project**. Name it something like `AuraGo-Workspace`.
3. In the left sidebar, navigate to **APIs & Services** > **Library**.
4. Search for and **Enable** the following three APIs:
   - `Gmail API`
   - `Google Calendar API`
   - `Google Drive API`

## Step 2: Configure the OAuth Consent Screen

Before creating credentials, you must define what your app looks like to users.

1. Go to **APIs & Services** > **OAuth consent screen**.
2. Select **External** user type and click **Create**.
3. Fill in the required app information (App name, support email, developer contact email). You can skip the logo and domains.
4. **Scopes:** Click **Add or Remove Scopes** and add the following:
   - `.../auth/gmail.modify`
   - `.../auth/calendar.events` (or `.readonly`)
   - `.../auth/drive.readonly`
5. **Test Users (CRITICAL):** While your app is in "Testing" mode, Google will block all logins with an `Error 403: access_denied` unless the email is explicitly whitelisted. Click **+ Add Users** and add your personal Google account email address here.
6. Save and continue.

## Step 3: Create "Desktop App" Credentials

> **⚠️ Important Architecture Note:** Do not choose "Web application" or "TVs and Limited Input devices". "TV" devices restrict Gmail scopes, and "Web" requires valid HTTPS redirect URIs. "Desktop App" is strictly required for our headless localhost workaround.

1. Go to **APIs & Services** > **Credentials**.
2. Click **+ Create Credentials** > **OAuth client ID**.
3. Select **Desktop App** as the Application type.
4. Name it (e.g., `AuraGo-CLI`) and click **Create**.
5. A dialog will appear with your Client ID and Client Secret. Click **Download JSON**.
6. Open the downloaded `client_secret_*.json` file.
7. **Security:** Do not save this file on the server. Instead, give the JSON content to the agent or save it in the vault as `google_workspace_client_secret`.
   - Command: `vault set google_workspace_client_secret "[JSON CONTENT]"`

## Step 4: Authentication & Vault Storage

The agent uses an encrypted Vault to store all sensitive tokens. No local `token.json` or `client_secret.json` files are needed on disk after the initial setup.

1. Trigger a Google Workspace operation (e.g., "list my emails").
2. If unauthorized, the agent prints a **Google Authorization URL**.
3. Open this URL in your local browser, log in, and click **Allow**.
4. You will be redirected to a broken page (e.g., `http://localhost:8080/?code=...`).
5. Copy the **entire URL** from your address bar.
6. Submit it to the agent: `google_workspace submit_auth_url "[URL]"`
7. The agent exchanges the code for a token and saves it securely in the Vault.

- **Maintenance:** The agent automatically refreshes the token in the background and updates the Vault. You only need to repeat this if you revoke access or clear your Vault.