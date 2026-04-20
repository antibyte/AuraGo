use crate::api::types::*;
use crate::api::sse::SseEvent;

#[derive(Debug, Clone, PartialEq)]
pub enum Screen {
    Splash,
    Login,
    Chat,
    Dashboard,
    Plans,
    Missions,
    Skills,
    Containers,
}

impl Screen {
    pub fn title(&self) -> &str {
        match self {
            Screen::Splash => "AuraGo",
            Screen::Login => "Login",
            Screen::Chat => "Chat",
            Screen::Dashboard => "Dashboard",
            Screen::Plans => "Plans",
            Screen::Missions => "Missions",
            Screen::Skills => "Skills",
            Screen::Containers => "Containers",
        }
    }

    /// All navigable screens (excluding Splash and Login)
    pub fn nav_items() -> &'static [Screen] {
        &[
            Screen::Chat,
            Screen::Dashboard,
            Screen::Plans,
            Screen::Missions,
            Screen::Skills,
            Screen::Containers,
        ]
    }

    pub fn nav_index(&self) -> usize {
        match self {
            Screen::Chat => 0,
            Screen::Dashboard => 1,
            Screen::Plans => 2,
            Screen::Missions => 3,
            Screen::Skills => 4,
            Screen::Containers => 5,
            _ => 0,
        }
    }

    pub fn from_nav_index(i: usize) -> Option<Self> {
        match i {
            0 => Some(Screen::Chat),
            1 => Some(Screen::Dashboard),
            2 => Some(Screen::Plans),
            3 => Some(Screen::Missions),
            4 => Some(Screen::Skills),
            5 => Some(Screen::Containers),
            _ => None,
        }
    }
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
    // ── Core ──────────────────────────────────────────────────────────────
    pub screen: Screen,
    pub server_url: String,
    pub authenticated: bool,
    pub auth_enabled: bool,
    pub totp_enabled: bool,

    // ── Login ─────────────────────────────────────────────────────────────
    pub login_password: String,
    pub login_totp: String,
    pub login_focus_otp: bool,
    pub login_error: Option<String>,
    pub login_loading: bool,

    // ── Chat ──────────────────────────────────────────────────────────────
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

    // ── Sessions ──────────────────────────────────────────────────────────
    pub sessions: Vec<ChatSession>,
    pub active_session_id: String,
    pub session_drawer_open: bool,

    // ── Dashboard ─────────────────────────────────────────────────────────
    pub dash_system: SystemInfo,
    pub dash_budget: BudgetInfo,
    pub dash_overview: OverviewInfo,
    pub dash_personality: PersonalityState,
    pub dash_logs: Vec<LogEntry>,
    pub dash_activity: Vec<CronEntry>,
    pub dash_tab: DashTab,
    pub dash_loading: bool,

    // ── Plans ─────────────────────────────────────────────────────────────
    pub plans: Vec<Plan>,
    pub plans_selected: Option<usize>,
    pub plans_loading: bool,

    // ── Missions ──────────────────────────────────────────────────────────
    pub missions: Vec<Mission>,
    pub missions_selected: Option<usize>,
    pub missions_loading: bool,

    // ── Skills ────────────────────────────────────────────────────────────
    pub skills: Vec<Skill>,
    pub skills_selected: Option<usize>,
    pub skills_loading: bool,

    // ── Containers ────────────────────────────────────────────────────────
    pub containers: Vec<Container>,
    pub containers_selected: Option<usize>,
    pub containers_loading: bool,

    // ── Navigation ────────────────────────────────────────────────────────
    pub nav_bar_open: bool,
    pub nav_bar_index: usize,

    /// Dummy field for list_selected_mut() default case
    pub _list_dummy: Option<usize>,
}

#[derive(Debug, Clone, PartialEq)]
pub enum DashTab {
    Overview,
    Agent,
    System,
    Logs,
}

impl Default for DashTab {
    fn default() -> Self {
        DashTab::Overview
    }
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
            sessions: Vec::new(),
            active_session_id: "default".to_string(),
            session_drawer_open: false,
            dash_system: SystemInfo::default(),
            dash_budget: BudgetInfo::default(),
            dash_overview: OverviewInfo::default(),
            dash_personality: PersonalityState::default(),
            dash_logs: Vec::new(),
            dash_activity: Vec::new(),
            dash_tab: DashTab::default(),
            dash_loading: false,
            plans: Vec::new(),
            plans_selected: None,
            plans_loading: false,
            missions: Vec::new(),
            missions_selected: None,
            missions_loading: false,
            skills: Vec::new(),
            skills_selected: None,
            skills_loading: false,
            containers: Vec::new(),
            containers_selected: None,
            containers_loading: false,
            nav_bar_open: false,
            nav_bar_index: 0,
            _list_dummy: None,
        }
    }
}

impl AppState {
    // ── Chat helpers ──────────────────────────────────────────────────────

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

    // ── Navigation helpers ────────────────────────────────────────────────

    pub fn navigate_to(&mut self, screen: Screen) {
        self.screen = screen.clone();
        self.nav_bar_index = screen.nav_index();
        self.nav_bar_open = false;
    }

    /// Returns true if the current screen uses a list + detail layout
    pub fn has_list_layout(&self) -> bool {
        matches!(self.screen, Screen::Plans | Screen::Missions | Screen::Skills | Screen::Containers)
    }

    /// Get the selected item index for the current list-based screen
    pub fn list_selected(&self) -> &Option<usize> {
        match self.screen {
            Screen::Plans => &self.plans_selected,
            Screen::Missions => &self.missions_selected,
            Screen::Skills => &self.skills_selected,
            Screen::Containers => &self.containers_selected,
            _ => &None,
        }
    }

    /// Get mutable selected item index
    pub fn list_selected_mut(&mut self) -> &mut Option<usize> {
        match self.screen {
            Screen::Plans => &mut self.plans_selected,
            Screen::Missions => &mut self.missions_selected,
            Screen::Skills => &mut self.skills_selected,
            Screen::Containers => &mut self.containers_selected,
            _ => &mut self._list_dummy,
        }
    }

    /// Get the list length for the current screen
    pub fn list_len(&self) -> usize {
        match self.screen {
            Screen::Plans => self.plans.len(),
            Screen::Missions => self.missions.len(),
            Screen::Skills => self.skills.len(),
            Screen::Containers => self.containers.len(),
            _ => 0,
        }
    }
}
