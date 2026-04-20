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
    pub prompt: i32,
    pub completion: i32,
    pub total: i32,
    pub session_total: i32,
    pub global_total: i32,
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

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Container {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub image: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub state: String,
    #[serde(default)]
    pub created: i64,
    #[serde(default)]
    pub ports: Vec<String>,
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
