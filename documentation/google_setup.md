# Google Workspace OAuth 2.0 Setup Guide

This guide explains how to set up Google OAuth 2.0 credentials for the native Google Workspace integration. AuraGo can read and manage Gmail, Calendar, Drive, Docs, and Sheets autonomously.

## Configuration

Enable Google Workspace in the **Config UI** under the **Google Workspace** section, or add to `config.yaml`:

```yaml
google_workspace:
  enabled: true
  gmail: true
  gmail_send: true
  calendar: true
  calendar_write: true
  drive: true
  docs: true
  docs_write: true
  sheets: true
  sheets_write: true
  client_id: "YOUR_CLIENT_ID.apps.googleusercontent.com"
```

The `client_secret` is stored securely in the vault (via the Config UI or `vault set google_workspace_client_secret "YOUR_SECRET"`).

## Step 1: Create a Google Cloud Project & Enable APIs

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Click the project dropdown in the top-nav bar and select **New Project**. Name it something like `AuraGo-Workspace`.
3. In the left sidebar, navigate to **APIs & Services** > **Library**.
4. Search for and **Enable** the following APIs:
   - `Gmail API`
   - `Google Calendar API`
   - `Google Drive API`
   - `Google Docs API`
   - `Google Sheets API`

## Step 2: Configure the OAuth Consent Screen

1. Go to **APIs & Services** > **OAuth consent screen**.
2. Select **External** user type and click **Create**.
3. Fill in the required app information (App name, support email, developer contact email).
4. **Scopes:** Click **Add or Remove Scopes** and add the scopes matching your enabled services.
5. **Test Users (CRITICAL):** While your app is in "Testing" mode, Google blocks all logins unless the email is whitelisted. Click **+ Add Users** and add your Google account email.
6. Save and continue.

## Step 3: Create "Web Application" Credentials

1. Go to **APIs & Services** > **Credentials**.
2. Click **+ Create Credentials** > **OAuth client ID**.
3. Select **Web application** as the Application type.
4. Name it (e.g., `AuraGo`) and click **Create**.
5. Under **Authorized redirect URIs**, add your AuraGo callback URL: `https://YOUR_DOMAIN/api/oauth/callback`
6. Copy the **Client ID** and **Client Secret**.

## Step 4: Connect via the Config UI

1. Open the AuraGo Config UI and navigate to **Google Workspace**.
2. Enter the **Client ID** in the text field.
3. Save the **Client Secret** to the vault using the save button.
4. Toggle the desired service scopes (Gmail, Calendar, Drive, etc.).
5. **Save the config** so that the scopes take effect.
6. Click **Connect** — a popup opens with Google's authorization flow.
7. Log in, grant access, and the popup closes automatically.
8. The status indicator shows "Connected" with auto-refresh active.

## Token Management

- Tokens are stored encrypted in the vault as `oauth_google_workspace`.
- The agent automatically refreshes expired tokens using the refresh token.
- To revoke access, click **Disconnect** in the Config UI or revoke from [Google Account Permissions](https://myaccount.google.com/permissions).

## Read-Only Mode

Enable `readonly: true` in config to block all write operations (sending emails, creating events, editing docs). The AI can still read and search.