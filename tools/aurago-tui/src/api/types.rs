use serde::{Deserialize, Serialize};

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
