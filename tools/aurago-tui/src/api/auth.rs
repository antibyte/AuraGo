use anyhow::Result;
use reqwest::Method;

use super::types::*;
use super::ApiClient;

// ── Auth ──────────────────────────────────────────────────────────────────────

pub async fn fetch_auth_status(client: &ApiClient) -> Result<AuthStatus> {
    client.request(Method::GET, "/api/auth/status", None::<&()>).await
}

pub async fn login(client: &ApiClient, password: &str, totp_code: &str) -> Result<LoginResponse> {
    let req = LoginRequest {
        password: password.to_string(),
        totp_code: totp_code.to_string(),
        redirect: String::new(),
    };
    let raw_resp = client
        .request_raw(Method::POST, "/auth/login", Some(&req))
        .await?;

    if let Some(cookie) = raw_resp.headers().get("set-cookie") {
        if let Ok(cookie_str) = cookie.to_str() {
            let session_cookie = cookie_str
                .split(';')
                .find(|s| s.starts_with("aurago_session="))
                .map(|s| s.to_string());
            if let Some(sc) = session_cookie {
                client.set_session_cookie(sc);
            }
        }
    }

    let resp = raw_resp.json::<LoginResponse>().await?;
    Ok(resp)
}

pub async fn logout(client: &ApiClient) -> Result<()> {
    client.request_empty(Method::POST, "/api/auth/logout", None::<&()>).await
}

#[allow(dead_code)]
pub async fn fetch_health(client: &ApiClient) -> Result<HealthStatus> {
    client.request(Method::GET, "/api/health", None::<&()>).await
}

// ── Chat / History ────────────────────────────────────────────────────────────

pub async fn fetch_history(client: &ApiClient) -> Result<Vec<HistoryMessage>> {
    client.request(Method::GET, "/history", None::<&()>).await
}

pub async fn fetch_history_for_session(client: &ApiClient, session_id: &str) -> Result<Vec<HistoryMessage>> {
    let path = format!("/history?session_id={}", session_id);
    client.request(Method::GET, &path, None::<&()>).await
}

pub async fn clear_history(client: &ApiClient) -> Result<()> {
    client.request_empty(Method::DELETE, "/clear", None::<&()>).await
}

// ── Sessions ──────────────────────────────────────────────────────────────────

pub async fn fetch_sessions(client: &ApiClient) -> Result<Vec<ChatSession>> {
    client.request(Method::GET, "/api/chat/sessions", None::<&()>).await
}

pub async fn create_session(client: &ApiClient) -> Result<ChatSession> {
    client.request(Method::POST, "/api/chat/sessions", None::<&()>).await
}

pub async fn delete_session(client: &ApiClient, id: &str) -> Result<()> {
    let path = format!("/api/chat/sessions/{}", id);
    client.request_empty(Method::DELETE, &path, None::<&()>).await
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

pub async fn fetch_system_info(client: &ApiClient) -> Result<SystemInfo> {
    client.request(Method::GET, "/api/dashboard/system", None::<&()>).await
}

pub async fn fetch_budget(client: &ApiClient) -> Result<BudgetInfo> {
    client.request(Method::GET, "/api/budget", None::<&()>).await
}

pub async fn fetch_overview(client: &ApiClient) -> Result<OverviewInfo> {
    client.request(Method::GET, "/api/dashboard/overview", None::<&()>).await
}

pub async fn fetch_personality_state(client: &ApiClient) -> Result<PersonalityState> {
    client.request(Method::GET, "/api/personality/state", None::<&()>).await
}

pub async fn fetch_logs(client: &ApiClient, lines: u32) -> Result<Vec<LogEntry>> {
    let path = format!("/api/dashboard/logs?lines={}", lines);
    client.request(Method::GET, &path, None::<&()>).await
}

pub async fn fetch_activity(client: &ApiClient) -> Result<Vec<CronEntry>> {
    client.request(Method::GET, "/api/dashboard/activity", None::<&()>).await
}

// ── Plans ─────────────────────────────────────────────────────────────────────

pub async fn fetch_plans(client: &ApiClient, session_id: &str) -> Result<Vec<Plan>> {
    let path = format!("/api/plans?session_id={}&limit=50", session_id);
    client.request(Method::GET, &path, None::<&()>).await
}

pub async fn fetch_plan_detail(client: &ApiClient, id: &str) -> Result<Plan> {
    let path = format!("/api/plans/{}", id);
    client.request(Method::GET, &path, None::<&()>).await
}

pub async fn advance_plan(client: &ApiClient, id: &str) -> Result<()> {
    let path = format!("/api/plans/{}/advance", id);
    client.request_empty(Method::POST, &path, None::<&()>).await
}

#[allow(dead_code)]
pub async fn archive_plan(client: &ApiClient, id: &str) -> Result<()> {
    let path = format!("/api/plans/{}/archive", id);
    client.request_empty(Method::POST, &path, None::<&()>).await
}

// ── Missions ──────────────────────────────────────────────────────────────────

pub async fn fetch_missions(client: &ApiClient) -> Result<Vec<Mission>> {
    client.request(Method::GET, "/api/missions/v2", None::<&()>).await
}

pub async fn run_mission(client: &ApiClient, id: &str) -> Result<()> {
    let path = format!("/api/missions/v2/{}/run", id);
    client.request_empty(Method::POST, &path, None::<&()>).await
}

pub async fn delete_mission(client: &ApiClient, id: &str) -> Result<()> {
    let path = format!("/api/missions/v2/{}", id);
    client.request_empty(Method::DELETE, &path, None::<&()>).await
}

#[allow(dead_code)]
pub async fn cancel_mission_queue(client: &ApiClient, id: &str) -> Result<()> {
    let path = format!("/api/missions/v2/{}/queue", id);
    client.request_empty(Method::DELETE, &path, None::<&()>).await
}

// ── Skills ────────────────────────────────────────────────────────────────────

pub async fn fetch_skills(client: &ApiClient) -> Result<Vec<Skill>> {
    client.request(Method::GET, "/api/skills", None::<&()>).await
}

pub async fn toggle_skill(client: &ApiClient, id: &str, enabled: bool) -> Result<()> {
    let path = format!("/api/skills/{}", id);
    let body = serde_json::json!({ "enabled": enabled });
    client.request_empty(Method::PUT, &path, Some(&body)).await
}

#[allow(dead_code)]
pub async fn toggle_daemon(client: &ApiClient, skill_id: &str, action: &str) -> Result<()> {
    let path = format!("/api/daemons/{}/{}", skill_id, action);
    client.request_empty(Method::POST, &path, None::<&()>).await
}

// ── Containers ────────────────────────────────────────────────────────────────

pub async fn fetch_containers(client: &ApiClient) -> Result<Vec<Container>> {
    client.request(Method::GET, "/api/containers", None::<&()>).await
}

pub async fn container_action(client: &ApiClient, id: &str, action: &str) -> Result<serde_json::Value> {
    let path = format!("/api/containers/{}/{}", id, action);
    client.request(Method::POST, &path, None::<&()>).await
}

pub async fn fetch_container_logs(client: &ApiClient, id: &str) -> Result<serde_json::Value> {
    let path = format!("/api/containers/{}/logs?tail=200", id);
    client.request(Method::GET, &path, None::<&()>).await
}

pub async fn remove_container(client: &ApiClient, id: &str, force: bool) -> Result<serde_json::Value> {
    let path = format!("/api/containers/{}?force={}", id, force);
    client.request(Method::DELETE, &path, None::<&()>).await
}

// ── Config ────────────────────────────────────────────────────────────────────

pub async fn fetch_config(client: &ApiClient) -> Result<serde_json::Value> {
    client.request(Method::GET, "/api/config", None::<&()>).await
}

pub async fn fetch_config_schema(client: &ApiClient) -> Result<serde_json::Value> {
    client.request(Method::GET, "/api/config/schema", None::<&()>).await
}

pub async fn save_config(client: &ApiClient, config: &serde_json::Value) -> Result<()> {
    client.request_empty(Method::PUT, "/api/config", Some(config)).await
}

#[allow(dead_code)]
pub async fn fetch_vault_status(client: &ApiClient) -> Result<VaultStatus> {
    client.request(Method::GET, "/api/vault/status", None::<&()>).await
}

// ── Knowledge ─────────────────────────────────────────────────────────────────

pub async fn fetch_knowledge_files(client: &ApiClient) -> Result<Vec<KnowledgeFile>> {
    client.request(Method::GET, "/api/knowledge", None::<&()>).await
}

pub async fn delete_knowledge_file(client: &ApiClient, name: &str) -> Result<()> {
    let path = format!("/api/knowledge/{}", name);
    client.request_empty(Method::DELETE, &path, None::<&()>).await
}

// ── Media ─────────────────────────────────────────────────────────────────────

pub async fn fetch_media(client: &ApiClient, media_type: &str, limit: u32, offset: u32, query: Option<&str>) -> Result<MediaResponse> {
    let mut path = format!("/api/media?type={}&limit={}&offset={}", media_type, limit, offset);
    if let Some(q) = query {
        path.push_str(&format!("&q={}", urlencoding::encode(q)));
    }
    client.request(Method::GET, &path, None::<&()>).await
}

pub async fn delete_media(client: &ApiClient, id: i64) -> Result<serde_json::Value> {
    let path = format!("/api/media/{}", id);
    client.request(Method::DELETE, &path, None::<&()>).await
}

// ── Session persistence ───────────────────────────────────────────────────────

pub fn save_session_cookie(cookie: &str, path: &std::path::Path) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(path, cookie)?;
    Ok(())
}

pub fn load_session_cookie(path: &std::path::Path) -> Option<String> {
    std::fs::read_to_string(path).ok()
}

pub fn delete_session_cookie(path: &std::path::Path) -> Result<()> {
    if path.exists() {
        std::fs::remove_file(path)?;
    }
    Ok(())
}
