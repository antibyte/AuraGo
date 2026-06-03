use crate::api::sse::SseEvent;
use crate::api::types::*;
use crate::i18n;

/// Convert a character index to a byte index in a string.
/// Returns `s.len()` if char_idx is out of bounds.
pub fn char_to_byte(s: &str, char_idx: usize) -> usize {
    s.char_indices()
        .nth(char_idx)
        .map(|(i, _)| i)
        .unwrap_or(s.len())
}

/// Count the number of characters (NOT bytes) in a string.
pub fn char_len(s: &str) -> usize {
    s.chars().count()
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum Screen {
    Splash,
    Login,
    Chat,
    Dashboard,
    Plans,
    Missions,
    Skills,
    Containers,
    Config,
    Knowledge,
    Media,
}

impl Screen {
    pub fn title(&self) -> &str {
        match self {
            Screen::Splash => i18n::current().app_title,
            Screen::Login => "Login", // not in main nav
            Screen::Chat => i18n::current().screen_chat,
            Screen::Dashboard => i18n::current().screen_dashboard,
            Screen::Plans => i18n::current().screen_plans,
            Screen::Missions => i18n::current().screen_missions,
            Screen::Skills => i18n::current().screen_skills,
            Screen::Containers => i18n::current().screen_containers,
            Screen::Config => i18n::current().screen_config,
            Screen::Knowledge => i18n::current().screen_knowledge,
            Screen::Media => i18n::current().screen_media,
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
            Screen::Config,
            Screen::Knowledge,
            Screen::Media,
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
            Screen::Config => 6,
            Screen::Knowledge => 7,
            Screen::Media => 8,
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
            6 => Some(Screen::Config),
            7 => Some(Screen::Knowledge),
            8 => Some(Screen::Media),
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
    /// Cached for fast full_logical sum in chat scrollbar (F6 polish)
    pub cached_line_count: usize,
}

#[derive(Debug)]
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
    pub toast_ticks: u16,
    pub toast_anim: u16,
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

    // ── Config ────────────────────────────────────────────────────────────
    pub config_data: serde_json::Value,
    pub config_schema: serde_json::Value,
    pub config_sections: Vec<String>,
    pub config_section_index: usize,
    pub config_field_index: usize,
    pub config_loading: bool,
    pub config_dirty: bool,
    pub config_editing: bool,
    pub config_edit_value: String,

    // ── Knowledge ─────────────────────────────────────────────────────────
    pub knowledge_files: Vec<KnowledgeFile>,
    pub knowledge_selected: Option<usize>,
    pub knowledge_loading: bool,
    pub knowledge_search: String,
    pub knowledge_search_active: bool,

    // ── Media ─────────────────────────────────────────────────────────────
    pub media_items: Vec<MediaItem>,
    pub media_total: i64,
    pub media_selected: Option<usize>,
    pub media_loading: bool,
    pub media_tab: MediaTab,
    pub media_search: String,
    pub media_search_active: bool,
    pub media_offset: u32,

    // ── Navigation ────────────────────────────────────────────────────────
    pub nav_bar_open: bool,
    pub nav_bar_index: usize,

    /// Input cursor positions for TUI polish
    pub chat_input_cursor: usize,
    pub config_edit_cursor: usize,
    pub session_drawer_index: usize,

    /// Graceful quit flag
    pub should_quit: bool,

    /// Current theme name
    pub theme_name: String,

    // ── Auto-scroll & Confirmation ──────────────────────────────────────
    pub auto_scroll: bool,
    pub confirm_action: Option<ConfirmAction>,

    /// Dummy field for list_selected_mut() default case
    pub _list_dummy: Option<usize>,

    // ── Background task tracking (Wave 2 / P2: abort fire-and-forget spawns on nav/quit) ─
    /// JoinHandles for cancellable background tasks (loads, deletes, etc.). Not for sse_handle (special cased).
    pub background_tasks: Vec<tokio::task::JoinHandle<()>>,
}

#[derive(Debug, Clone, PartialEq, Default)]
pub enum DashTab {
    #[default]
    Overview,
    Agent,
    System,
    Logs,
}

#[derive(Debug, Clone, PartialEq, Default)]
pub enum MediaTab {
    #[default]
    Audio,
    Documents,
}

#[derive(Debug, Clone, PartialEq)]
#[allow(dead_code)]
pub enum ConfirmAction {
    DeleteMission { index: usize },
    DeleteContainer { index: usize },
    DeleteKnowledge { index: usize },
    DeleteMedia { index: usize },
    ClearChat,
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
            chat_input_cursor: 0,
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
            toast_anim: 0,
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
            config_data: serde_json::Value::Null,
            config_schema: serde_json::Value::Null,
            config_sections: Vec::new(),
            config_section_index: 0,
            config_field_index: 0,
            config_loading: false,
            config_dirty: false,
            config_editing: false,
            config_edit_value: String::new(),
            knowledge_files: Vec::new(),
            knowledge_selected: None,
            knowledge_loading: false,
            knowledge_search: String::new(),
            knowledge_search_active: false,
            media_items: Vec::new(),
            media_total: 0,
            media_selected: None,
            media_loading: false,
            media_tab: MediaTab::default(),
            media_search: String::new(),
            media_search_active: false,
            media_offset: 0,
            nav_bar_open: false,
            nav_bar_index: 0,
            config_edit_cursor: 0,
            session_drawer_index: 0,
            should_quit: false,
            theme_name: "default".to_string(),
            auto_scroll: true,
            confirm_action: None,
            _list_dummy: None,
            background_tasks: Vec::new(),
        }
    }
}

impl AppState {
    // ── Chat helpers ──────────────────────────────────────────────────────

    pub fn push_user_message(&mut self, text: String) {
        let line_count = text.lines().count();
        self.chat_messages.push(ChatMessage {
            role: "user".to_string(),
            content: text,
            is_streaming: false,
            is_tool: false,
            is_thinking: false,
            cached_line_count: line_count,
        });
        self.scroll_to_bottom();
        self.prune_old_messages();
    }

    pub fn insert_at_cursor(&mut self, c: char) {
        let char_count = char_len(&self.chat_input);
        if self.chat_input_cursor > char_count {
            self.chat_input_cursor = char_count;
        }
        let byte_idx = char_to_byte(&self.chat_input, self.chat_input_cursor);
        self.chat_input.insert(byte_idx, c);
        self.chat_input_cursor += 1;
    }

    pub fn backspace_at_cursor(&mut self) {
        if self.chat_input_cursor > 0 {
            self.chat_input_cursor -= 1;
            let byte_idx = char_to_byte(&self.chat_input, self.chat_input_cursor);
            if byte_idx < self.chat_input.len() {
                self.chat_input.remove(byte_idx);
            }
        }
    }

    pub fn delete_at_cursor(&mut self) {
        let char_count = char_len(&self.chat_input);
        if self.chat_input_cursor < char_count {
            let byte_idx = char_to_byte(&self.chat_input, self.chat_input_cursor);
            if byte_idx < self.chat_input.len() {
                self.chat_input.remove(byte_idx);
            }
        }
    }

    pub fn cursor_left(&mut self) {
        if self.chat_input_cursor > 0 {
            self.chat_input_cursor -= 1;
        }
    }

    pub fn cursor_right(&mut self) {
        if self.chat_input_cursor < char_len(&self.chat_input) {
            self.chat_input_cursor += 1;
        }
    }

    pub fn cursor_start(&mut self) {
        self.chat_input_cursor = 0;
    }

    pub fn cursor_end(&mut self) {
        self.chat_input_cursor = char_len(&self.chat_input);
    }

    pub fn start_assistant_stream(&mut self) {
        self.chat_messages.push(ChatMessage {
            role: "assistant".to_string(),
            content: String::new(),
            is_streaming: true,
            is_tool: false,
            is_thinking: false,
            cached_line_count: 0,
        });
        self.scroll_to_bottom();
        self.prune_old_messages();
    }

    pub fn append_stream_delta(&mut self, delta: String) {
        if let Some(last) = self.chat_messages.last_mut() {
            if last.is_streaming {
                last.content.push_str(&delta);
                last.cached_line_count = last.content.lines().count();
            }
        }
    }

    pub fn finish_stream(&mut self) {
        if let Some(last) = self.chat_messages.last_mut() {
            last.is_streaming = false;
        }
    }

    pub fn scroll_to_bottom(&mut self) {
        // Set a flag to auto-scroll; actual value will be clamped during rendering
        self.auto_scroll = true;
    }

    /// Prune old messages from UI to bound memory for very long chats (Point 4 perf).
    /// Keeps the most recent MAX_CHAT_MESSAGES. Server history remains full.
    /// Adjusts scroll on prune (from top) so manual scroll view stays relative if possible.
    pub fn prune_old_messages(&mut self) {
        const MAX: usize = 400; // Tunable; ~400 msgs keeps UI responsive while supporting long sessions.
        let len = self.chat_messages.len();
        if len > MAX {
            let excess = len - MAX;
            // Sum logical lines removed (cached + separator) for scroll adjust.
            let removed_logical: usize = self
                .chat_messages
                .iter()
                .take(excess)
                .map(|m| m.cached_line_count + 1)
                .sum();
            self.chat_messages.drain(0..excess);
            if !self.auto_scroll {
                self.scroll = self.scroll.saturating_sub(removed_logical);
            }
        }
    }

    pub fn apply_sse_event(&mut self, event: SseEvent) {
        match event {
            SseEvent::Delta(text) => {
                self.append_stream_delta(text);
                // When auto_scroll is on, the draw loop will force scroll = usize::MAX
                // so we no longer need per-delta nudges here.
            }
            SseEvent::DeltaDone => self.finish_stream(),
            SseEvent::ThinkingStart => self.thinking_active = true,
            SseEvent::ThinkingStop => self.thinking_active = false,
            SseEvent::ToolCall(name) => {
                let content = format!("🔧 Tool: {}", name);
                self.chat_messages.push(ChatMessage {
                    role: "tool".to_string(),
                    content,
                    is_streaming: false,
                    is_tool: true,
                    is_thinking: false,
                    cached_line_count: 1,
                });
                if self.auto_scroll {
                    self.scroll_to_bottom(); // will be turned into usize::MAX in next draw
                }
                self.prune_old_messages();
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
            .map(|m| {
                let line_count = m.content.lines().count();
                ChatMessage {
                    role: m.role,
                    content: m.content,
                    is_streaming: false,
                    is_tool: false,
                    is_thinking: false,
                    cached_line_count: line_count,
                }
            })
            .collect();
        self.prune_old_messages();
        self.scroll_to_bottom();
    }

    pub fn tick(&mut self) {
        self.tick_counter = self.tick_counter.wrapping_add(1);
        if self.toast_ticks > 0 {
            self.toast_ticks -= 1;
            self.toast_anim += 1;
            if self.toast_ticks == 0 {
                self.toast = None;
                self.toast_anim = 0;
            }
        }
        // Occasional prune of finished tracked tasks (Wave 2)
        if self.tick_counter % 10 == 0 {
            self.prune_finished_tasks();
        }
    }

    // ── Navigation helpers ────────────────────────────────────────────────

    pub fn navigate_to(&mut self, screen: Screen) {
        // Wave 2: abort prior background loads to prevent pile-up on rapid nav (P2)
        // sse_handle for Chat is managed separately (abort only on explicit session change / logout).
        self.prune_finished_tasks();
        if self.screen != screen {
            self.abort_all_background();
        }
        self.screen = screen;
        self.nav_bar_index = screen.nav_index();
        self.nav_bar_open = false;
    }

    /// Get the selected item index for the current list-based screen
    pub fn list_selected(&self) -> &Option<usize> {
        match self.screen {
            Screen::Plans => &self.plans_selected,
            Screen::Missions => &self.missions_selected,
            Screen::Skills => &self.skills_selected,
            Screen::Containers => &self.containers_selected,
            Screen::Knowledge => &self.knowledge_selected,
            Screen::Media => &self.media_selected,
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
            Screen::Knowledge => &mut self.knowledge_selected,
            Screen::Media => &mut self.media_selected,
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
            Screen::Knowledge => self.knowledge_files.len(),
            Screen::Media => self.media_items.len(),
            _ => 0,
        }
    }

    // ── Background task tracking (Wave 2 P2 optimization) ─────────────────
    /// Spawn a task and track its JoinHandle so it can be aborted on nav/quit.
    pub fn spawn_tracked(&mut self, handle: tokio::task::JoinHandle<()>) {
        self.background_tasks.push(handle);
    }

    /// Abort all tracked background tasks (e.g. on screen nav away from loaders, or quit).
    /// Does not touch the special sse_handle (managed separately in main.rs).
    pub fn abort_all_background(&mut self) {
        for handle in self.background_tasks.drain(..) {
            handle.abort();
        }
    }

    /// Remove finished tasks from the tracker (call periodically or on nav).
    pub fn prune_finished_tasks(&mut self) {
        self.background_tasks.retain(|h| !h.is_finished());
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn char_len_ascii_and_unicode() {
        assert_eq!(char_len("hello"), 5);
        assert_eq!(char_len(""), 0);
        assert_eq!(char_len("a🦀b"), 3); // emoji is one char
        assert_eq!(char_len("日本語"), 3);
    }

    #[test]
    fn char_to_byte_basic() {
        assert_eq!(char_to_byte("hello", 0), 0);
        assert_eq!(char_to_byte("hello", 5), 5);
        assert_eq!(char_to_byte("hello", 99), 5); // OOB -> len
    }

    #[test]
    fn cursor_roundtrip_ascii() {
        let mut s = AppState { chat_input: "abc".to_string(), chat_input_cursor: 0, ..Default::default() };
        s.insert_at_cursor('x');
        assert_eq!(s.chat_input, "xabc");
        assert_eq!(s.chat_input_cursor, 1);
        s.backspace_at_cursor();
        assert_eq!(s.chat_input, "abc");
    }

    #[test]
    fn tracker_default_prune() {
        let mut s = AppState::default();
        assert!(s.background_tasks.is_empty());
        s.prune_finished_tasks();
        assert!(s.background_tasks.is_empty());
        // Full spawn_tracked/abort tests require async runtime; covered in manual/integration for Wave A
    }

    #[test]
    fn apply_sse_delta_updates_content_and_cache() {
        let mut s = AppState::default();
        s.start_assistant_stream();
        s.append_stream_delta("hello\nworld".to_string());
        assert!(s.chat_messages.last().unwrap().is_streaming);
        assert_eq!(s.chat_messages.last().unwrap().cached_line_count, 2);
        assert_eq!(s.chat_messages.last().unwrap().content, "hello\nworld");
    }

    #[test]
    fn apply_sse_deltadone_finishes_stream() {
        let mut s = AppState::default();
        s.start_assistant_stream();
        s.apply_sse_event(SseEvent::DeltaDone);
        assert!(!s.chat_messages.last().unwrap().is_streaming);
    }

    #[test]
    fn apply_sse_toolcall_adds_with_cache_and_respects_auto() {
        let mut s = AppState::default();
        s.auto_scroll = true;
        s.apply_sse_event(SseEvent::ToolCall("foo".to_string()));
        let last = s.chat_messages.last().unwrap();
        assert_eq!(last.role, "tool");
        assert!(last.is_tool);
        assert_eq!(last.cached_line_count, 1);
        assert!(s.auto_scroll);
    }

    #[test]
    fn load_history_filters_internal_sets_cache_and_scroll() {
        let mut s = AppState::default();
        let hist = vec![
            HistoryMessage { role: "user".into(), content: "hi".into(), id: None, is_internal: false, timestamp: None },
            HistoryMessage { role: "system".into(), content: "internal".into(), id: None, is_internal: true, timestamp: None },
            HistoryMessage { role: "assistant".into(), content: "bye\nthere".into(), id: None, is_internal: false, timestamp: None },
        ];
        s.load_history(hist);
        assert_eq!(s.chat_messages.len(), 2);
        assert_eq!(s.chat_messages[0].cached_line_count, 1);
        assert_eq!(s.chat_messages[1].cached_line_count, 2);
        assert!(s.auto_scroll);
    }

    #[test]
    fn scroll_to_bottom_sets_auto_scroll() {
        let mut s = AppState::default();
        s.auto_scroll = false;
        s.scroll_to_bottom();
        assert!(s.auto_scroll);
    }
}
