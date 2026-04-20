use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

#[derive(Debug, Clone)]
pub enum Action {
    Quit,
    SendMessage,
    NewLine,
    ScrollUp,
    ScrollDown,
    ScrollTop,
    ScrollBottom,
    ToggleHelp,
    ToggleTheme,
    ToggleSidebar,
    ClearChat,
    Logout,
    Backspace,
    DeleteChar,
    CursorLeft,
    CursorRight,
    CursorStart,
    CursorEnd,
    Type(char),
    None,

    // ── Navigation ────────────────────────────────────────────────────────
    NavigateLeft,    // Alt+Left  or Ctrl+H
    NavigateRight,   // Alt+Right or Ctrl+L (when not in chat input)
    OpenNavBar,      // F1 or Ctrl+N
    CloseNavBar,     // Esc when nav bar is open
    NavUp,           // Up in nav bar
    NavDown,         // Down in nav bar
    NavSelect,       // Enter in nav bar
    GoToChat,        // F2
    GoToDashboard,   // F3
    GoToPlans,       // F4
    GoToMissions,    // F5
    GoToSkills,      // F6
    GoToContainers,  // F7
    GoToConfig,      // F8
    GoToKnowledge,   // F9
    GoToMedia,       // F10

    // ── List navigation ───────────────────────────────────────────────────
    ListUp,
    ListDown,
    ListSelect,
    ListBack,

    // ── Session drawer ────────────────────────────────────────────────────
    ToggleSessionDrawer,
    SessionUp,
    SessionDown,
    SessionSelect,
    SessionNew,
    SessionDelete,

    // ── Dashboard tabs ────────────────────────────────────────────────────
    TabLeft,
    TabRight,

    // ── Actions ───────────────────────────────────────────────────────────
    Refresh,         // F5 or Ctrl+Shift+R
    ActionPrimary,   // Enter on detail view
    ActionDelete,    // Delete key on list item
    ActionToggle,    // Space to toggle enabled/disabled

    // ── Search ────────────────────────────────────────────────────────────
    SearchActivate,  // / to start search
    SearchDeactivate, // Esc to exit search
    SearchSubmit,    // Enter to submit search

    // ── Config editing ────────────────────────────────────────────────────
    EditField,       // Enter on config field to edit
    EditSave,        // Enter to save edit
    EditCancel,      // Esc to cancel edit
    SectionUp,       // Move to previous config section
    SectionDown,     // Move to next config section
}

pub fn map_key(key: KeyEvent, context: KeyContext) -> Action {
    match context {
        KeyContext::Splash => map_splash(key),
        KeyContext::Login => map_login(key),
        KeyContext::Chat { focus_sidebar, session_drawer } => {
            map_chat(key, focus_sidebar, session_drawer)
        }
        KeyContext::NavBar { .. } => map_nav_bar(key),
        KeyContext::List { .. } => map_list(key),
        KeyContext::Dashboard { .. } => map_dashboard(key),
        KeyContext::Config { editing, .. } => map_config(key, editing),
        KeyContext::Search { active } => map_search(key, active),
    }
}

#[derive(Debug, Clone)]
pub enum KeyContext {
    Splash,
    Login,
    Chat { focus_sidebar: bool, session_drawer: bool },
    NavBar { index: usize, max: usize },
    List { selected: Option<usize>, len: usize },
    Dashboard { tab_index: usize, tab_count: usize },
    Config { section_index: usize, field_index: usize, editing: bool },
    Search { active: bool },
}

fn map_splash(key: KeyEvent) -> Action {
    match key.code {
        _ => Action::None,
    }
}

fn map_login(key: KeyEvent) -> Action {
    match key.code {
        KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => Action::Quit,
        KeyCode::Enter => Action::SendMessage,
        KeyCode::Tab => Action::ToggleSidebar, // toggles OTP focus
        KeyCode::Backspace => Action::Backspace,
        KeyCode::Delete => Action::Backspace,
        KeyCode::Char('\u{7f}') => Action::Backspace,
        KeyCode::Char('\u{8}') => Action::Backspace,
        KeyCode::Char(c) => Action::Type(c),
        _ => Action::None,
    }
}

fn map_chat(key: KeyEvent, focus_sidebar: bool, session_drawer: bool) -> Action {
    // Global shortcuts first
    if let Some(action) = try_global_keys(key) {
        return action;
    }

    // Session drawer mode
    if session_drawer {
        return match key.code {
            KeyCode::Char('j') | KeyCode::Down => Action::SessionDown,
            KeyCode::Char('k') | KeyCode::Up => Action::SessionUp,
            KeyCode::Enter => Action::SessionSelect,
            KeyCode::Char('n') => Action::SessionNew,
            KeyCode::Char('d') => Action::SessionDelete,
            KeyCode::Esc => Action::ToggleSessionDrawer,
            _ => Action::None,
        };
    }

    // Sidebar mode
    if focus_sidebar {
        return match key.code {
            KeyCode::Char('j') | KeyCode::Down => Action::ScrollDown,
            KeyCode::Char('k') | KeyCode::Up => Action::ScrollUp,
            KeyCode::Enter => Action::ListSelect,
            KeyCode::Esc | KeyCode::Tab => Action::ToggleSidebar,
            _ => Action::None,
        };
    }

    // Normal chat input mode
    match key.code {
        KeyCode::Enter if key.modifiers.contains(KeyModifiers::SHIFT) => Action::NewLine,
        KeyCode::Enter => Action::SendMessage,
        KeyCode::Up if key.modifiers.contains(KeyModifiers::CONTROL) => Action::ScrollTop,
        KeyCode::Down if key.modifiers.contains(KeyModifiers::CONTROL) => Action::ScrollBottom,
        KeyCode::Up => Action::ScrollUp,
        KeyCode::Down => Action::ScrollDown,
        KeyCode::Tab => Action::ToggleSidebar,
        KeyCode::Backspace => Action::Backspace,
        KeyCode::Delete => Action::Backspace,
        KeyCode::Char('\u{7f}') => Action::Backspace,
        KeyCode::Char('\u{8}') => Action::Backspace,
        KeyCode::Left if key.modifiers.contains(KeyModifiers::CONTROL) => Action::CursorStart,
        KeyCode::Right if key.modifiers.contains(KeyModifiers::CONTROL) => Action::CursorEnd,
        KeyCode::Left => Action::CursorLeft,
        KeyCode::Right => Action::CursorRight,
        KeyCode::Home => Action::CursorStart,
        KeyCode::End => Action::CursorEnd,
        KeyCode::Char(c) => Action::Type(c),
        _ => Action::None,
    }
}

fn map_nav_bar(key: KeyEvent) -> Action {
    match key.code {
        KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => Action::Quit,
        KeyCode::Char('j') | KeyCode::Down => Action::NavDown,
        KeyCode::Char('k') | KeyCode::Up => Action::NavUp,
        KeyCode::Enter => Action::NavSelect,
        KeyCode::Esc => Action::CloseNavBar,
        _ => Action::None,
    }
}

fn map_list(key: KeyEvent) -> Action {
    if let Some(action) = try_global_keys(key) {
        return action;
    }

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => Action::ListDown,
        KeyCode::Char('k') | KeyCode::Up => Action::ListUp,
        KeyCode::Enter => Action::ListSelect,
        KeyCode::Esc => Action::ListBack,
        KeyCode::Delete => Action::ActionDelete,
        KeyCode::Char(' ') => Action::ActionToggle,
        KeyCode::Char('r') => Action::Refresh,
        _ => Action::None,
    }
}

fn map_dashboard(key: KeyEvent) -> Action {
    if let Some(action) = try_global_keys(key) {
        return action;
    }

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => Action::ScrollDown,
        KeyCode::Char('k') | KeyCode::Up => Action::ScrollUp,
        KeyCode::Char('h') | KeyCode::Left => Action::TabLeft,
        KeyCode::Char('l') | KeyCode::Right => Action::TabRight,
        KeyCode::Char('r') => Action::Refresh,
        _ => Action::None,
    }
}

/// Config screen keybindings
fn map_config(key: KeyEvent, editing: bool) -> Action {
    // When editing a field, handle edit-mode keys
    if editing {
        if let Some(action) = try_global_keys(key) {
            return action;
        }
        return match key.code {
            KeyCode::Enter => Action::EditSave,
            KeyCode::Esc => Action::EditCancel,
            KeyCode::Backspace => Action::Backspace,
            KeyCode::Delete => Action::Backspace,
            KeyCode::Char('\u{7f}') => Action::Backspace,
            KeyCode::Char('\u{8}') => Action::Backspace,
            KeyCode::Left => Action::CursorLeft,
            KeyCode::Right => Action::CursorRight,
            KeyCode::Home => Action::CursorStart,
            KeyCode::End => Action::CursorEnd,
            KeyCode::Char(c) => Action::Type(c),
            _ => Action::None,
        };
    }

    // Normal config browsing mode
    if let Some(action) = try_global_keys(key) {
        return action;
    }

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => Action::ListDown,
        KeyCode::Char('k') | KeyCode::Up => Action::ListUp,
        KeyCode::Char('h') | KeyCode::Left => Action::SectionUp,
        KeyCode::Char('l') | KeyCode::Right => Action::SectionDown,
        KeyCode::Enter => Action::EditField,
        KeyCode::Char('r') => Action::Refresh,
        KeyCode::Char('/') => Action::SearchActivate,
        _ => Action::None,
    }
}

/// Search mode keybindings (used by Knowledge and Media screens)
fn map_search(key: KeyEvent, _active: bool) -> Action {
    if let Some(action) = try_global_keys(key) {
        return action;
    }

    match key.code {
        KeyCode::Enter => Action::SearchSubmit,
        KeyCode::Esc => Action::SearchDeactivate,
        KeyCode::Backspace => Action::Backspace,
        KeyCode::Delete => Action::Backspace,
        KeyCode::Char('\u{7f}') => Action::Backspace,
        KeyCode::Char('\u{8}') => Action::Backspace,
        KeyCode::Left => Action::CursorLeft,
        KeyCode::Right => Action::CursorRight,
        KeyCode::Home => Action::CursorStart,
        KeyCode::End => Action::CursorEnd,
        KeyCode::Char(c) => Action::Type(c),
        _ => Action::None,
    }
}

/// Global keybindings that work on every screen
fn try_global_keys(key: KeyEvent) -> Option<Action> {
    match key.code {
        KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => Some(Action::Quit),
        KeyCode::Char('q') => Some(Action::Quit),
        KeyCode::Esc => Some(Action::ToggleHelp),
        KeyCode::Char('?') => Some(Action::ToggleHelp),
        KeyCode::F(1) => Some(Action::OpenNavBar),
        KeyCode::F(2) => Some(Action::GoToChat),
        KeyCode::F(3) => Some(Action::GoToDashboard),
        KeyCode::F(4) => Some(Action::GoToPlans),
        KeyCode::F(5) => Some(Action::GoToMissions),
        KeyCode::F(6) => Some(Action::GoToSkills),
        KeyCode::F(7) => Some(Action::GoToContainers),
        KeyCode::F(8) => Some(Action::GoToConfig),
        KeyCode::F(9) => Some(Action::GoToKnowledge),
        KeyCode::F(10) => Some(Action::GoToMedia),
        KeyCode::Char('o') if key.modifiers.contains(KeyModifiers::CONTROL) => Some(Action::Logout),
        KeyCode::Char('s') if key.modifiers.contains(KeyModifiers::CONTROL) => Some(Action::ToggleSessionDrawer),
        KeyCode::Char('n') if key.modifiers.contains(KeyModifiers::CONTROL) => Some(Action::OpenNavBar),
        _ => None,
    }
}
