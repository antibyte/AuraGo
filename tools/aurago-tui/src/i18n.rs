//! Minimal i18n scaffolding for the TUI.
//!
//! Goal: Provide a single place for all user-facing strings so that full
//! internationalization (15 languages like the Web UI) can be added later
//! without touching every draw function.
//!
//! Current state: Only English. No runtime switching yet.
//! Future: Add more `static LANG_XX: Strings = ...;` + a `current()` fn
//! driven by config or env var.

pub struct Strings {
    // Titles & general (some used in overlays; others allowed for future integration)
    #[allow(dead_code)]
    pub app_title: &'static str,
    pub help_title: &'static str,
    pub notification_title: &'static str,
    pub sessions_title: &'static str,
    pub navigate_title: &'static str,

    // Common actions / buttons (scaffolding; integrate in follow-up)
    #[allow(dead_code)]
    pub send: &'static str,
    #[allow(dead_code)]
    pub new_line: &'static str,
    #[allow(dead_code)]
    pub login: &'static str,
    #[allow(dead_code)]
    pub logout: &'static str,
    #[allow(dead_code)]
    pub refresh: &'static str,
    #[allow(dead_code)]
    pub delete: &'static str,
    #[allow(dead_code)]
    pub confirm: &'static str,
    #[allow(dead_code)]
    pub cancel: &'static str,

    // Status / toasts (scaffolding)
    #[allow(dead_code)]
    pub connected: &'static str,
    #[allow(dead_code)]
    pub disconnected: &'static str,
    #[allow(dead_code)]
    pub unsaved_changes: &'static str,

    // Help section headers (scaffolding; hardcoded in overlays::draw_help for now)
    #[allow(dead_code)]
    pub help_chat: &'static str,
    #[allow(dead_code)]
    pub help_navigation: &'static str,
    #[allow(dead_code)]
    pub help_list_pages: &'static str,
    #[allow(dead_code)]
    pub help_general: &'static str,

    // Chat specific (integrated in wave 1/4)
    pub message_input_title: &'static str,
    pub new_messages_hint: &'static str,

    // Common UI titles and status (expanding in Wave C / F3)
    pub loading: &'static str,
    pub overview_title: &'static str,
    pub budget_title: &'static str,
    pub scheduled_tasks_title: &'static str,
    pub personality_title: &'static str,
    pub system_title: &'static str,
    pub live_logs_title: &'static str,
    pub history_title: &'static str,
    pub confirm_title: &'static str,

    // List screen titles (expanding F3)
    #[allow(dead_code)]
    pub containers_title: &'static str,
    #[allow(dead_code)]
    pub plans_title: &'static str,
    #[allow(dead_code)]
    pub missions_title: &'static str,
    #[allow(dead_code)]
    pub skills_title: &'static str,
    #[allow(dead_code)]
    pub knowledge_files_title: &'static str,
    #[allow(dead_code)]
    pub media_title: &'static str,
    #[allow(dead_code)]
    pub config_sections_title: &'static str,

    // More UI (details, login, edit - expanding F3)
    pub detail_title: &'static str,
    pub edit_field_title: &'static str,
    pub password_title: &'static str,
    pub otp_title: &'static str,
    pub login_title: &'static str,

    // Confirm actions (F3)
    pub confirm_delete_mission: &'static str,
    pub confirm_remove_container: &'static str,
    pub confirm_delete_knowledge: &'static str,
    pub confirm_delete_media: &'static str,
    pub confirm_clear_chat: &'static str,
}

/// English (default / current only language)
pub static EN: Strings = Strings {
    app_title: "AuraGo",
    help_title: " Help ",
    notification_title: " Notification ",
    sessions_title: " 💬 Sessions ",
    navigate_title: " Navigate ",

    send: "Send message",
    new_line: "New line",
    login: "Login",
    logout: "Logout",
    refresh: "Refresh",
    delete: "Delete",
    confirm: "Confirm",
    cancel: "Cancel",

    connected: "Connected",
    disconnected: "Disconnected",
    unsaved_changes: "unsaved changes",

    help_chat: "── Chat ──────────────────────────",
    help_navigation: "── Navigation ────────────────────",
    help_list_pages: "── List pages ────────────────────",
    help_general: "── General ───────────────────────",

    message_input_title: " Message ",
    new_messages_hint: "New messages — press Ctrl+G to scroll down",

    loading: "Loading...",
    overview_title: " Overview ",
    budget_title: " Budget ",
    scheduled_tasks_title: " Scheduled Tasks ",
    personality_title: " Personality ",
    system_title: " System ",
    live_logs_title: " Live Logs ",
    history_title: " History ",
    confirm_title: " ⚠ Confirm ",

    containers_title: " Containers ",
    plans_title: " Plans ",
    missions_title: " Missions ",
    skills_title: " Skills ",
    knowledge_files_title: " Files ",
    media_title: " Media ",
    config_sections_title: " Sections ",

    detail_title: " Detail ",
    edit_field_title: " Edit Field (Enter=Save, Esc=Cancel) ",
    password_title: " Password ",
    otp_title: " OTP Code ",
    login_title: " 🔐 AuraGo Terminal Chat ",

    confirm_delete_mission: "delete this mission",
    confirm_remove_container: "remove this container",
    confirm_delete_knowledge: "delete this file",
    confirm_delete_media: "delete this media item",
    confirm_clear_chat: "clear chat history",
};

/// Current strings (for now always EN).
/// Later this can become a fn that returns & 'static Strings based on user preference.
pub fn current() -> &'static Strings {
    &EN
}