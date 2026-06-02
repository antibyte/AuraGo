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
};

/// Current strings (for now always EN).
/// Later this can become a fn that returns & 'static Strings based on user preference.
pub fn current() -> &'static Strings {
    &EN
}