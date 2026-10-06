#![allow(dead_code)]
use serde::{Deserialize, Serialize};

// ── Auth ──────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Deserialize)]
pub struct AuthStatus {
    pub enabled: bool,
    pub password_set: bool,
    pub totp_enabled: bool,
    pub authenticated: bool,
}

#[derive(Debug, Clone, Serialize)]
pub struct LoginRequest {
    pub password: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub totp_code: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub redirect: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct LoginResponse {
    pub ok: bool,
    pub redirect: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct HealthStatus {
    pub status: String,
}

// ── Chat / History ────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Deserialize)]
pub struct HistoryMessage {
    pub role: String,
    pub content: String,
    pub id: Option<i64>,
    #[serde(default)]
    pub is_internal: bool,
    pub timestamp: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessage {
    pub role: String,
    pub content: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct ChatCompletionRequest {
    pub model: String,
    pub messages: Vec<ChatMessage>,
    pub stream: bool,
}

// ── SSE ───────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Deserialize)]
pub struct SseEventWrapper {
    #[serde(rename = "type")]
    pub event_type: String,
    pub payload: serde_json::Value,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct LLMStreamDelta {
    pub content: Option<String>,
    pub reasoning: Option<String>,
    pub tool_name: Option<String>,
    pub tool_id: Option<String>,
    pub finish_reason: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct TokenUpdatePayload {
    pub prompt: i64,
    pub completion: i64,
    pub total: i64,
    pub session_total: i64,
    pub global_total: i64,
    #[serde(default)]
    pub is_estimated: bool,
    #[serde(default)]
    pub is_final: bool,
}

#[derive(Debug, Clone, Deserialize)]
pub struct PersonalityUpdatePayload {
    pub mood: Option<String>,
    pub trigger: Option<String>,
    pub current_emotion: Option<String>,
}

// ── Sessions ──────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Deserialize)]
pub struct ChatSession {
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub updated_at: String,
    #[serde(default)]
    pub message_count: i64,
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct SystemInfo {
    #[serde(default)]
    pub cpu_percent: f64,
    #[serde(default)]
    pub memory_percent: f64,
    #[serde(default)]
    pub disk_percent: f64,
    #[serde(default)]
    pub uptime_seconds: i64,
    #[serde(default)]
    pub network_sent_mb: f64,
    #[serde(default)]
    pub network_recv_mb: f64,
    #[serde(default)]
    pub sse_clients: i32,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct BudgetInfo {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub spent_usd: f64,
    #[serde(default)]
    pub daily_limit_usd: f64,
    #[serde(default)]
    pub total_cost_usd: f64,
    #[serde(default)]
    pub enforcement: String,
    #[serde(default)]
    pub is_exceeded: bool,
    #[serde(default)]
    pub is_warning: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct OverviewInfo {
    #[serde(default)]
    pub agent_status: String,
    #[serde(default)]
    pub model: String,
    #[serde(default)]
    pub provider: String,
    #[serde(default)]
    pub context_percent: f64,
    #[serde(default)]
    pub integrations: i32,
    #[serde(default)]
    pub tools_count: i32,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct PersonalityState {
    #[serde(default)]
    pub mood: String,
    #[serde(default)]
    pub emotion: String,
    #[serde(default)]
    pub traits: std::collections::HashMap<String, f64>,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct LogEntry {
    #[serde(default)]
    pub time: String,
    #[serde(default)]
    pub level: String,
    #[serde(default)]
    pub message: String,
}

// ── Plans ─────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Plan {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub progress: f64,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub updated_at: String,
    #[serde(default)]
    pub tasks: Vec<PlanTask>,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct PlanTask {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub description: String,
}

// ── Missions ──────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Mission {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub exec_type: String,
    #[serde(default)]
    pub priority: String,
    #[serde(default)]
    pub prompt: String,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub last_run: Option<String>,
    #[serde(default)]
    pub next_run: Option<String>,
    #[serde(default)]
    pub cron_schedule: Option<String>,
    #[serde(default)]
    pub locked: bool,
}

// ── Skills ────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Skill {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub security_status: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub is_daemon: bool,
    #[serde(default)]
    pub daemon_running: bool,
    #[serde(default)]
    pub created_at: String,
}

// ── Containers ────────────────────────────────────────────────────────────────

/// Treats JSON null like a missing field: the Go server encodes a nil slice
/// (no containers, no names) as null.
fn null_as_default<'de, D, T>(deserializer: D) -> Result<T, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Default + Deserialize<'de>,
{
    Ok(Option::<T>::deserialize(deserializer)?.unwrap_or_default())
}

/// GET /api/containers answer: {"status","count","containers":[...]}. Only a
/// JSON object decodes; serde would otherwise read a bare array as an empty
/// list and hide a format mismatch.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(try_from = "serde_json::Map<String, serde_json::Value>")]
pub struct ContainerList {
    pub status: String,
    pub message: String,
    pub containers: Vec<Container>,
}

#[derive(Deserialize)]
struct ContainerListFields {
    #[serde(default, deserialize_with = "null_as_default")]
    status: String,
    #[serde(default, deserialize_with = "null_as_default")]
    message: String,
    #[serde(default, deserialize_with = "null_as_default")]
    containers: Vec<Container>,
}

impl TryFrom<serde_json::Map<String, serde_json::Value>> for ContainerList {
    type Error = serde_json::Error;

    fn try_from(map: serde_json::Map<String, serde_json::Value>) -> Result<Self, Self::Error> {
        let fields: ContainerListFields = serde_json::from_value(serde_json::Value::Object(map))?;
        Ok(ContainerList {
            status: fields.status,
            message: fields.message,
            containers: fields.containers,
        })
    }
}

/// One container of the list. Every field is optional: older servers send no
/// protection fields, and those containers are simply not marked protected.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Container {
    #[serde(default, deserialize_with = "null_as_default")]
    pub id: String,
    /// Display name; `ContainerList::into_containers` fills it from `names`.
    #[serde(default, deserialize_with = "null_as_default")]
    pub name: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub names: Vec<String>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub image: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub status: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub state: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub health: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub created: i64,
    #[serde(default, deserialize_with = "null_as_default")]
    pub ports: Vec<String>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub protected_owner: String,
    #[serde(default, rename = "self", deserialize_with = "null_as_default")]
    pub is_self: bool,
    #[serde(default, deserialize_with = "null_as_default")]
    pub docker_endpoint: bool,
    #[serde(default, deserialize_with = "null_as_default")]
    pub shared_network: bool,
}

impl Container {
    /// Why AuraGo protects this container, in the server's "owner" order:
    /// "self", "docker-endpoint", "shared-network", the managing owner, or "".
    pub fn protection(&self) -> &str {
        if self.is_self {
            "self"
        } else if self.docker_endpoint {
            "docker-endpoint"
        } else if self.shared_network {
            "shared-network"
        } else {
            self.protected_owner.as_str()
        }
    }

    pub fn is_protected(&self) -> bool {
        !self.protection().is_empty()
    }
}

impl ContainerList {
    /// The containers with display names, or the server's error message.
    pub fn into_containers(self) -> anyhow::Result<Vec<Container>> {
        if !self.status.is_empty() && self.status != "ok" {
            let message = if self.message.is_empty() {
                "container list failed".to_string()
            } else {
                self.message
            };
            anyhow::bail!("{}", message);
        }
        Ok(self
            .containers
            .into_iter()
            .map(|mut c| {
                if c.name.is_empty() {
                    c.name = c
                        .names
                        .first()
                        .map(|n| n.trim_start_matches('/').to_string())
                        .filter(|n| !n.is_empty())
                        .unwrap_or_else(|| c.id.clone());
                }
                c
            })
            .collect())
    }
}

/// The last `max_chars` characters of a GET /api/containers/{id}/logs answer
/// ({"status":"ok","logs":"..."}); the newest lines are at the end. A list of
/// lines is joined, so a server variant that sends one still shows its logs.
pub fn container_logs_tail(answer: &serde_json::Value, max_chars: usize) -> String {
    let logs = match answer.get("logs") {
        Some(serde_json::Value::String(text)) => text.clone(),
        Some(serde_json::Value::Array(lines)) => lines
            .iter()
            .map(|line| line.as_str().map(str::to_string).unwrap_or_else(|| line.to_string()))
            .collect::<Vec<_>>()
            .join("\n"),
        _ => return "(no logs in answer)".to_string(),
    };
    if logs.trim().is_empty() {
        return "(empty)".to_string();
    }
    let count = logs.chars().count();
    logs.chars().skip(count.saturating_sub(max_chars)).collect()
}

// ── Activity / Cron ───────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct CronEntry {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub expression: String,
    #[serde(default)]
    pub prompt: String,
    #[serde(default)]
    pub last_run: Option<String>,
    #[serde(default)]
    pub next_run: Option<String>,
    #[serde(default)]
    pub enabled: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct VaultStatus {
    #[serde(default)]
    pub exists: bool,
    #[serde(default)]
    pub sealed: bool,
    #[serde(default)]
    pub entry_count: i32,
}

// ── Knowledge ─────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct KnowledgeFile {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub size: i64,
    #[serde(default)]
    pub modified: String,
}

// ── Media ─────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Default, Deserialize)]
pub struct MediaItem {
    #[serde(default)]
    pub id: i64,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub filename: String,
    #[serde(default)]
    pub format: String,
    #[serde(default)]
    pub media_type: String,
    #[serde(default)]
    pub size: i64,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub web_path: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct MediaResponse {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub items: Vec<MediaItem>,
    #[serde(default)]
    pub total: i64,
    #[serde(default)]
    pub message: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn container_list_decodes_the_server_envelope() {
        let raw = r#"{"status":"ok","count":2,"containers":[
            {"id":"0123456789ab","names":["/aurago"],"image":"ghcr.io/antibyte/aurago:latest","state":"running","status":"Up 1 hour","protected_owner":"aurago-app","self":true},
            {"id":"bbbbbbbbbbbb","names":[],"image":"nginx:1","state":"exited","status":"Exited (0)"}
        ]}"#;
        let list: ContainerList = serde_json::from_str(raw).expect("decode envelope");
        let containers = list.into_containers().expect("ok list");
        assert_eq!(containers.len(), 2);
        assert_eq!(containers[0].name, "aurago");
        assert!(containers[0].is_self);
        assert_eq!(containers[0].protection(), "self");
        assert_eq!(containers[1].name, "bbbbbbbbbbbb");
        assert_eq!(containers[1].protection(), "");
    }

    #[test]
    fn container_list_reports_an_error_envelope() {
        let list: ContainerList =
            serde_json::from_str(r#"{"status":"error","message":"Docker error (HTTP 409): engine refused"}"#).unwrap();
        assert!(list.into_containers().unwrap_err().to_string().contains("engine refused"));
    }

    #[test]
    fn a_bare_array_is_not_the_server_format() {
        assert!(serde_json::from_str::<ContainerList>("[]").is_err());
    }

    /// Go encodes a nil slice as null: an empty list and a container without
    /// names arrive as null. Older servers send no protection fields at all.
    #[test]
    fn container_list_accepts_null_lists_and_an_older_server() {
        let empty: ContainerList = serde_json::from_str(r#"{"status":"ok","count":0,"containers":null}"#).expect("null list");
        assert!(empty.into_containers().expect("ok").is_empty());
        let old = r#"{"status":"ok","count":1,"containers":[{"id":"cccccccccccc","names":null,"image":"nginx","state":"running","status":"Up"}]}"#;
        let containers = serde_json::from_str::<ContainerList>(old).expect("older server").into_containers().expect("ok");
        assert_eq!(containers[0].name, "cccccccccccc");
        assert!(!containers[0].is_protected());
        let unknown = r#"{"status":"ok","containers":[{"id":"d","names":["/d"],"new_field":{"x":1}}]}"#;
        assert_eq!(serde_json::from_str::<ContainerList>(unknown).expect("unknown fields").into_containers().unwrap()[0].name, "d");
    }

    #[test]
    fn container_logs_tail_reads_the_logs_field() {
        let answer = serde_json::json!({"status":"ok","container_id":"web","logs":"first\nsecond\nlast line"});
        assert_eq!(container_logs_tail(&answer, 500), "first\nsecond\nlast line");
        assert_eq!(container_logs_tail(&answer, 9), "last line");
        assert_eq!(container_logs_tail(&serde_json::json!({"status":"ok","logs":"  "}), 500), "(empty)");
        assert_eq!(container_logs_tail(&serde_json::json!({"status":"ok"}), 500), "(no logs in answer)");
        assert_eq!(container_logs_tail(&serde_json::json!({"logs":["a","b"]}), 500), "a\nb");
    }
}
