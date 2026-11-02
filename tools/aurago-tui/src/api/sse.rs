use eventsource_stream::Eventsource;
use futures::StreamExt;
use reqwest::Client;
use serde_json::Value;
use std::time::Duration;

use super::types::SseEventWrapper;

#[derive(Debug, Clone)]
#[allow(dead_code)]
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

/// Connect to the SSE stream with automatic reconnection and exponential backoff.
/// This function runs indefinitely, reconnecting whenever the connection drops.
pub async fn connect_sse(
    client: Client,
    url: String,
    origin: String,
    cookie: Option<String>,
    tx: tokio::sync::mpsc::UnboundedSender<SseEvent>,
) {
    let mut retry_delay_secs: u64 = 1;
    let max_retry_delay_secs: u64 = 30;

    loop {
        // Notify UI that we're connecting
        let _ = tx.send(SseEvent::AgentStatus("connecting".to_string()));

        let connected = connect_sse_once(&client, &url, &origin, cookie.as_deref(), &tx).await;

        if connected {
            // Reset backoff after successful connection so next retry starts fresh
            retry_delay_secs = 1;
            // Connection was established and then closed — try reconnect
            let _ = tx.send(SseEvent::AgentStatus("reconnecting".to_string()));
        }
        // If not connected, the error was already sent via tx

        // Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s (max)
        let _ = tx.send(SseEvent::AgentStatus(format!(
            "SSE disconnected — reconnecting in {}s...",
            retry_delay_secs
        )));

        tokio::time::sleep(Duration::from_secs(retry_delay_secs)).await;

        retry_delay_secs = (retry_delay_secs * 2).min(max_retry_delay_secs);
    }
}

/// Attempt a single SSE connection. Returns `true` if the connection was
/// successfully established (even if it later dropped), `false` if the
/// initial connection failed.
async fn connect_sse_once(
    client: &Client,
    url: &str,
    origin: &str,
    cookie: Option<&str>,
    tx: &tokio::sync::mpsc::UnboundedSender<SseEvent>,
) -> bool {
    let mut req = client.get(url).header("Origin", origin);
    if let Some(c) = cookie {
        req = req.header("Cookie", c);
    }

    let resp = match req.send().await {
        Ok(r) => r,
        Err(e) => {
            let _ = tx.send(SseEvent::Unknown(format!("SSE connect error: {}", e)));
            return false;
        }
    };

    // Connection succeeded — reset backoff hint (caller resets based on return value)
    let _ = tx.send(SseEvent::AgentStatus("connected".to_string()));

    let mut stream = resp.bytes_stream().eventsource();
    while let Some(event) = stream.next().await {
        match event {
            Ok(ev) => {
                if let Ok(wrapper) = serde_json::from_str::<SseEventWrapper>(&ev.data) {
                    let parsed = parse_sse_event(wrapper);
                    let _ = tx.send(parsed);
                } else if let Ok(val) = serde_json::from_str::<Value>(&ev.data) {
                    if let Some(event_name) = val.get("event").and_then(|v| v.as_str()) {
                        if let Some(detail) = val.get("detail").and_then(|v| v.as_str()) {
                            let _ =
                                tx.send(SseEvent::LogLine(format!("{}: {}", event_name, detail)));
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

    // Stream ended normally (server closed connection)
    let _ = tx.send(SseEvent::Unknown("SSE stream ended".to_string()));
    true
}

fn parse_sse_event(wrapper: SseEventWrapper) -> SseEvent {
    match wrapper.event_type.as_str() {
        "llm_stream_delta" => {
            if let Ok(delta) =
                serde_json::from_value::<super::types::LLMStreamDelta>(wrapper.payload)
            {
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
                let action = v
                    .get("action")
                    .and_then(|s| s.as_str())
                    .unwrap_or("unknown");
                SseEvent::ToolCall(action.to_string())
            } else {
                SseEvent::Unknown("bad tool_call_preview".to_string())
            }
        }
        "token_update" => {
            if let Ok(p) =
                serde_json::from_value::<super::types::TokenUpdatePayload>(wrapper.payload)
            {
                SseEvent::TokenUpdate(p)
            } else {
                SseEvent::Unknown("bad token_update".to_string())
            }
        }
        "personality_update" => {
            if let Ok(p) =
                serde_json::from_value::<super::types::PersonalityUpdatePayload>(wrapper.payload)
            {
                SseEvent::PersonalityUpdate(p)
            } else {
                SseEvent::Unknown("bad personality_update".to_string())
            }
        }
        "agent_status" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                let text = v
                    .get("status")
                    .and_then(|s| s.as_str())
                    .unwrap_or("unknown")
                    .to_string();
                SseEvent::AgentStatus(text)
            } else {
                SseEvent::Unknown("bad agent_status".to_string())
            }
        }
        "toast" => {
            if let Ok(v) = serde_json::from_value::<Value>(wrapper.payload) {
                let text = v
                    .get("message")
                    .and_then(|s| s.as_str())
                    .unwrap_or("")
                    .to_string();
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::ApiClient;
    use std::io::{BufRead, BufReader, Write};
    use std::net::TcpListener;

    #[tokio::test]
    async fn streaming_survives_thirty_seconds_while_regular_requests_expire() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("http://{}", listener.local_addr().unwrap());
        let server = std::thread::spawn(move || {
            let mut workers = Vec::new();
            for connection in listener.incoming().take(2) {
                let mut connection = connection.unwrap();
                workers.push(std::thread::spawn(move || {
                    connection.set_read_timeout(Some(Duration::from_secs(5))).unwrap();
                    let mut reader = BufReader::new(connection.try_clone().unwrap());
                    let mut line = String::new();
                    reader.read_line(&mut line).unwrap();
                    let streaming = line.starts_with("GET /events ");
                    loop {
                        line.clear();
                        if reader.read_line(&mut line).unwrap() == 0 || line == "\r\n" {
                            break;
                        }
                    }
                    if streaming {
                        connection.write_all(b"HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nConnection: close\r\n\r\ndata: {\"type\":\"llm_stream_delta\",\"payload\":{\"content\":\"before\"}}\n\n").unwrap();
                    }
                    std::thread::sleep(Duration::from_secs(31));
                    if streaming {
                        let _ = connection.write_all(b"data: {\"type\":\"llm_stream_done\",\"payload\":{}}\n\n");
                    } else {
                        let _ = connection.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}");
                    }
                }));
            }
            for worker in workers {
                worker.join().unwrap();
            }
        });
        let client = ApiClient::new(&url, false).unwrap();
        let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
        let stream_url = client.sse_url("/events");
        let stream = connect_sse_once(&client.client, &stream_url, &url, None, &tx);
        let request = client.request::<(), Value>(reqwest::Method::GET, "/slow", None);
        let (connected, response) = tokio::time::timeout(Duration::from_secs(40), async {
            tokio::join!(stream, request)
        }).await.expect("local timeout regression server did not finish");
        assert!(connected);
        assert!(response.unwrap_err().downcast_ref::<reqwest::Error>().is_some_and(|err| err.is_timeout()));
        let mut completed = false;
        while let Ok(event) = rx.try_recv() {
            completed |= matches!(event, SseEvent::DeltaDone);
        }
        assert!(completed, "the stream lost its event after the ordinary request deadline");
        server.join().unwrap();
    }
}
