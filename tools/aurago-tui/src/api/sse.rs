use eventsource_stream::Eventsource;
use futures::StreamExt;
use reqwest::Client;
use serde_json::Value;

use super::types::SseEventWrapper;

#[derive(Debug, Clone)]
pub enum SseEvent {
    Delta(String),
    DeltaDone,
    ThinkingStart,
    ThinkingStop,
    ToolCall(String),
    TokenUpdate(super::types::TokenUpdatePayload),
    PersonalityUpdate(super::types::PersonalityUpdatePayload),
    AgentStatus(String),
    Toast(String),
    SystemWarning(String),
    LogLine(String),
    DaemonUpdate(Value),
    Unknown(String),
}

pub async fn connect_sse(
    client: Client,
    url: String,
    cookie: Option<String>,
    tx: tokio::sync::mpsc::UnboundedSender<SseEvent>,
) {
    let mut req = client.get(&url);
    if let Some(c) = cookie {
        req = req.header("Cookie", c);
    }
    let resp = match req.send().await {
        Ok(r) => r,
        Err(e) => {
            let _ = tx.send(SseEvent::Unknown(format!("SSE connect error: {}", e)));
            return;
        }
    };

    let mut stream = resp.bytes_stream().eventsource();
    while let Some(event) = stream.next().await {
        match event {
            Ok(ev) => {
                if let Ok(wrapper) = serde_json::from_str::<SseEventWrapper>(&ev.data) {
                    let parsed = parse_sse_event(wrapper);
                    let _ = tx.send(parsed);
                } else if let Ok(val) = serde_json::from_str::<Value>(&ev.data) {
                    if let Some(event) = val.get("event").and_then(|v| v.as_str()) {
                        if let Some(detail) = val.get("detail").and_then(|v| v.as_str()) {
                            let _ = tx.send(SseEvent::LogLine(format!("{}: {}", event, detail)));
                            continue;
                        }
                    }
                    let _ = tx.send(SseEvent::Unknown(ev.data));
                } else {
                    let _ = tx.send(SseEvent::Unknown(ev.data));
                }
            }
            Err(e) => {
                let _ = tx.send(SseEvent::Unknown(format!("SSE stream error: {}", e)));
            }
        }
    }
    let _ = tx.send(SseEvent::Unknown("SSE disconnected".to_string()));
}

fn parse_sse_event(wrapper: SseEventWrapper) -> SseEvent {
    match wrapper.event_type.as_str() {
        "llm_stream_delta" => {
            if let Ok(delta) = serde_json::from_value::<super::types::LLMStreamDelta>(wrapper.payload) {
                SseEvent::Delta(delta.content.unwrap_or_default())
            } else {
                SseEvent::Unknown("bad llm_stream_delta".to_string())
            }
        }
        "llm_stream_done" => SseEvent::DeltaDone,
        "thinking_block" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                match v.get("state").and_then(|s| s.as_str()) {
                    Some("start") => SseEvent::ThinkingStart,
                    Some("stop") => SseEvent::ThinkingStop,
                    _ => SseEvent::Unknown("thinking_block delta".to_string()),
                }
            } else {
                SseEvent::Unknown("bad thinking_block".to_string())
            }
        }
        "tool_call_preview" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                let action = v.get("action").and_then(|s| s.as_str()).unwrap_or("unknown");
                SseEvent::ToolCall(action.to_string())
            } else {
                SseEvent::Unknown("bad tool_call_preview".to_string())
            }
        }
        "token_update" => {
            if let Ok(p) = serde_json::from_value::<super::types::TokenUpdatePayload>(wrapper.payload) {
                SseEvent::TokenUpdate(p)
            } else {
                SseEvent::Unknown("bad token_update".to_string())
            }
        }
        "personality_update" => {
            if let Ok(p) = serde_json::from_value::<super::types::PersonalityUpdatePayload>(wrapper.payload) {
                SseEvent::PersonalityUpdate(p)
            } else {
                SseEvent::Unknown("bad personality_update".to_string())
            }
        }
        "agent_status" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                let text = v.get("status").and_then(|s| s.as_str()).unwrap_or("unknown").to_string();
                SseEvent::AgentStatus(text)
            } else {
                SseEvent::Unknown("bad agent_status".to_string())
            }
        }
        "toast" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                let text = v.get("message").and_then(|s| s.as_str()).unwrap_or("").to_string();
                SseEvent::Toast(text)
            } else {
                SseEvent::Unknown("bad toast".to_string())
            }
        }
        "system_warning" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                let text = serde_json::to_string(&v).unwrap_or_default();
                SseEvent::SystemWarning(text)
            } else {
                SseEvent::Unknown("bad system_warning".to_string())
            }
        }
        "log_line" => SseEvent::LogLine(wrapper.payload.to_string()),
        "daemon_update" => SseEvent::DaemonUpdate(wrapper.payload),
        _ => SseEvent::Unknown(format!("unknown event type: {}", wrapper.event_type)),
    }
}
