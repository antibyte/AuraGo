use crate::api::types::{HistoryMessage, TokenUpdatePayload, PersonalityUpdatePayload};
use crate::api::sse::SseEvent;

#[derive(Debug, Clone, PartialEq)]
pub enum Screen {
    Splash,
    Login,
    Chat,
}

#[derive(Debug, Clone)]
pub struct ChatMessage {
    pub role: String,
    pub content: String,
    pub is_streaming: bool,
    pub is_tool: bool,
    pub is_thinking: bool,
}

#[derive(Debug, Clone)]
pub struct AppState {
    pub screen: Screen,
    pub server_url: String,
    pub authenticated: bool,
    pub auth_enabled: bool,
    pub totp_enabled: bool,
    pub login_password: String,
    pub login_totp: String,
    pub login_focus_otp: bool,
    pub login_error: Option<String>,
    pub login_loading: bool,
    pub chat_input: String,
    pub chat_messages: Vec<ChatMessage>,
    pub scroll: usize,
    pub status_message: String,
    pub tokens: TokenUpdatePayload,
    pub personality: PersonalityUpdatePayload,
    pub agent_status: String,
    pub show_help: bool,
    pub toast: Option<String>,
    pub toast_ticks: u8,
    pub thinking_active: bool,
    pub focus_sidebar: bool,
    pub sidebar_index: usize,
    pub tick_counter: u64,
}

impl Default for AppState {
    fn default() -> Self {
        Self {
            screen: Screen::Splash,
            server_url: "http://localhost:8080".to_string(),
            authenticated: false,
            auth_enabled: false,
            totp_enabled: false,
            login_password: String::new(),
            login_totp: String::new(),
            login_focus_otp: false,
            login_error: None,
            login_loading: false,
            chat_input: String::new(),
            chat_messages: Vec::new(),
            scroll: 0,
            status_message: "Disconnected".to_string(),
            tokens: TokenUpdatePayload {
                prompt: 0,
                completion: 0,
                total: 0,
                session_total: 0,
                global_total: 0,
                is_estimated: false,
                is_final: false,
            },
            personality: PersonalityUpdatePayload {
                mood: Some("Neutral".to_string()),
                trigger: None,
                current_emotion: Some("calm".to_string()),
            },
            agent_status: "idle".to_string(),
            show_help: false,
            toast: None,
            toast_ticks: 0,
            thinking_active: false,
            focus_sidebar: false,
            sidebar_index: 0,
            tick_counter: 0,
        }
    }
}

impl AppState {
    pub fn push_user_message(&mut self, text: String) {
        self.chat_messages.push(ChatMessage {
            role: "user".to_string(),
            content: text,
            is_streaming: false,
            is_tool: false,
            is_thinking: false,
        });
        self.scroll_to_bottom();
    }

    pub fn start_assistant_stream(&mut self) {
        self.chat_messages.push(ChatMessage {
            role: "assistant".to_string(),
            content: String::new(),
            is_streaming: true,
            is_tool: false,
            is_thinking: false,
        });
        self.scroll_to_bottom();
    }

    pub fn append_stream_delta(&mut self, delta: String) {
        if let Some(last) = self.chat_messages.last_mut() {
            if last.is_streaming {
                last.content.push_str(&delta);
            }
        }
    }

    pub fn finish_stream(&mut self) {
        if let Some(last) = self.chat_messages.last_mut() {
            last.is_streaming = false;
        }
    }

    pub fn scroll_to_bottom(&mut self) {
        self.scroll = self.chat_messages.len().saturating_sub(1);
    }

    pub fn apply_sse_event(&mut self, event: SseEvent) {
        match event {
            SseEvent::Delta(text) => self.append_stream_delta(text),
            SseEvent::DeltaDone => self.finish_stream(),
            SseEvent::ThinkingStart => self.thinking_active = true,
            SseEvent::ThinkingStop => self.thinking_active = false,
            SseEvent::ToolCall(name) => {
                self.chat_messages.push(ChatMessage {
                    role: "tool".to_string(),
                    content: format!("🔧 Tool: {}", name),
                    is_streaming: false,
                    is_tool: true,
                    is_thinking: false,
                });
                self.scroll_to_bottom();
            }
            SseEvent::TokenUpdate(p) => self.tokens = p,
            SseEvent::PersonalityUpdate(p) => self.personality = p,
            SseEvent::AgentStatus(s) => self.agent_status = s,
            SseEvent::Toast(msg) => {
                self.toast = Some(msg);
                self.toast_ticks = 8;
            }
            SseEvent::SystemWarning(msg) => {
                self.toast = Some(format!("⚠️ {}", msg));
                self.toast_ticks = 12;
            }
            SseEvent::LogLine(_) => {}
            SseEvent::DaemonUpdate(_) => {}
            SseEvent::Unknown(_) => {}
        }
    }

    pub fn load_history(&mut self, history: Vec<HistoryMessage>) {
        self.chat_messages = history
            .into_iter()
            .filter(|m| !m.is_internal)
            .map(|m| ChatMessage {
                role: m.role,
                content: m.content,
                is_streaming: false,
                is_tool: false,
                is_thinking: false,
            })
            .collect();
        self.scroll_to_bottom();
    }

    pub fn tick(&mut self) {
        self.tick_counter = self.tick_counter.wrapping_add(1);
        if self.toast_ticks > 0 {
            self.toast_ticks -= 1;
            if self.toast_ticks == 0 {
                self.toast = None;
            }
        }
    }
}
