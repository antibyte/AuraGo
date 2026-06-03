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

    // Nav actions (F3 close)
    pub nav_new: &'static str,
    pub nav_del: &'static str,
    pub nav_label: &'static str,

    // Empty states (completing F3 i18n polish - Point 1 sequential)
    pub no_plans_found: &'static str,
    pub no_missions_found: &'static str,
    pub no_skills_found: &'static str,
    pub no_containers_found: &'static str,
    pub no_knowledge_files_found: &'static str,
    pub no_sessions_yet: &'static str,
    pub no_media_audio_found: &'static str,
    pub no_media_documents_found: &'static str,
    pub no_scheduled_tasks: &'static str,

    // Inner list header labels (emoji + name for draw_*_header)
    pub plans_header: &'static str,
    pub missions_header: &'static str,
    pub skills_header: &'static str,
    pub containers_header: &'static str,
    pub knowledge_header: &'static str,

    // Media tabs + search
    pub media_tab_audio: &'static str,
    pub media_tab_documents: &'static str,
    pub search_label: &'static str,

    // Overlays small remaining texts
    pub session_close: &'static str,
    pub confirm_y_label: &'static str,
    pub confirm_other_label: &'static str,
    pub confirm_cancel_label: &'static str,

    // Dashboard status bar hints (for draw_dash_status right side + F6 tasks)
    pub status_hint_tabs: &'static str,
    pub status_hint_scroll: &'static str,
    pub status_hint_refresh: &'static str,
    pub status_hint_nav: &'static str,
    pub status_hint_help: &'static str,

    // Screen titles (for nav bar + Screen::title)
    pub screen_chat: &'static str,
    pub screen_dashboard: &'static str,
    pub screen_plans: &'static str,
    pub screen_missions: &'static str,
    pub screen_skills: &'static str,
    pub screen_containers: &'static str,
    pub screen_config: &'static str,
    pub screen_knowledge: &'static str,
    pub screen_media: &'static str,
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

    nav_new: " New  ",
    nav_del: " Del  ",
    nav_label: " Navigate  ",

    // Empty states
    no_plans_found: "No plans found",
    no_missions_found: "No missions found",
    no_skills_found: "No skills found",
    no_containers_found: "No containers found",
    no_knowledge_files_found: "  No knowledge files found",
    no_sessions_yet: "  No sessions yet",
    no_media_audio_found: "  No audio files found",
    no_media_documents_found: "  No document files found",
    no_scheduled_tasks: "No scheduled tasks",

    // Headers (full with emoji for direct use in draw_*_header)
    plans_header: " 📋 Plans ",
    missions_header: " 🚀 Missions ",
    skills_header: " 🧩 Skills ",
    containers_header: " 🐳 Containers ",
    knowledge_header: " 📚 Knowledge ",

    // Media
    media_tab_audio: " 🎵 Audio ",
    media_tab_documents: " 📄 Documents ",
    search_label: "Search",

    // Overlays
    session_close: " Close",
    confirm_y_label: " = Confirm   ",
    confirm_other_label: "Any other key",
    confirm_cancel_label: " = Cancel",

    // Status hints
    status_hint_tabs: "tabs",
    status_hint_scroll: "scroll",
    status_hint_refresh: "refresh",
    status_hint_nav: "nav",
    status_hint_help: "help",

    // Screen names (for nav + title())
    screen_chat: "Chat",
    screen_dashboard: "Dashboard",
    screen_plans: "Plans",
    screen_missions: "Missions",
    screen_skills: "Skills",
    screen_containers: "Containers",
    screen_config: "Config",
    screen_knowledge: "Knowledge",
    screen_media: "Media",
};

/// Current strings (for now always EN).
/// Later this can become a fn that returns & 'static Strings based on user preference.
pub fn current() -> &'static Strings {
    &EN
}