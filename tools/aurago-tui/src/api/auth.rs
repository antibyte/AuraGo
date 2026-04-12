use super::{types::*, ApiClient};
use anyhow::{Context, Result};
use reqwest::Method;
use std::path::Path;

pub async fn fetch_auth_status(client: &ApiClient) -> Result<AuthStatus> {
    client.request(Method::GET, "/api/auth/status", None::<&()>).await
}

pub async fn login(client: &ApiClient, password: &str, totp_code: &str) -> Result<LoginResponse> {
    let req = LoginRequest {
        password: password.to_string(),
        totp_code: totp_code.to_string(),
        redirect: "/".to_string(),
    };
    let raw_resp = client
        .request_raw(Method::POST, "/api/auth/login", Some(&req))
        .await?;
    // Extract and store session cookie manually for robustness
    if let Some(set_cookie) = raw_resp.headers().get("set-cookie") {
        if let Ok(cookie_val) = set_cookie.to_str() {
            client.set_session_cookie(cookie_val.to_string());
        }
    }
    let resp = raw_resp.json::<LoginResponse>().await.context("Failed to decode login response")?;
    Ok(resp)
}

pub async fn logout(client: &ApiClient) -> Result<()> {
    client.request_empty(Method::POST, "/api/auth/logout", None::<&()>).await
}

pub async fn fetch_health(client: &ApiClient) -> Result<HealthStatus> {
    client.request(Method::GET, "/api/health", None::<&()>).await
}

pub async fn fetch_history(client: &ApiClient) -> Result<Vec<HistoryMessage>> {
    client.request(Method::GET, "/history", None::<&()>).await
}

pub async fn clear_history(client: &ApiClient) -> Result<()> {
    client.request_empty(Method::DELETE, "/clear", None::<&()>).await
}

/// Save the session cookie string to disk.
pub fn save_session_cookie(path: &Path, cookie: &str) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(path, cookie).context("Failed to write session cookie")
}

/// Load a previously saved session cookie string.
pub fn load_session_cookie(path: &Path) -> Option<String> {
    std::fs::read_to_string(path).ok()
}

/// Remove saved session cookie.
pub fn delete_session_cookie(path: &Path) -> Result<()> {
    if path.exists() {
        std::fs::remove_file(path).context("Failed to delete session cookie")?;
    }
    Ok(())
}
