//! Action dispatch, loading, execution helpers, and small overlay drawers.
//! Extracted from main.rs during the 2026-05-18 audit refactor for maintainability.

use std::sync::Arc;

use crossterm::event::KeyEvent;
use ratatui::{
    Frame,
    layout::{Alignment, Margin, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, Clear, Paragraph},
};
use tokio::sync::mpsc::UnboundedSender;
use reqwest::Method;

use crate::api::{ApiClient, auth, types::*};
use crate::app::{AppState, ConfirmAction, DashTab, MediaTab, Screen, char_len, char_to_byte};
use crate::events::AppEvent;
use crate::events::keybindings::{Action, KeyContext, map_key};
use crate::ui::{theme::Theme, utils::truncate_str};



/// Public entry point called from handle_key_event (which stays thin in main or can also move later).
pub fn dispatch_action(
    action: Action,
    app: &mut AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
    sse_handle: &mut Option<tokio::task::JoinHandle<()>>,
) {
    match action {
        Action::None => {}
        Action::Quit => {
            app.should_quit = true;
        }

        // ── Help & Theme ─────────────────────────────────────────────────────
        Action::ToggleHelp => app.show_help = !app.show_help,
        Action::ToggleTheme => {
            app.theme_name = Theme::next_name(&app.theme_name).to_string();
            // Persist theme choice
            if let Ok(mut cfg) = crate::config::Config::load() {
                cfg.theme = app.theme_name.clone();
                let _ = cfg.save();
            }
        }

        // ── Sidebar / Tab ────────────────────────────────────────────────────
        Action::ToggleSidebar => {
            if app.screen == Screen::Login && app.totp_enabled {
                app.login_focus_otp = !app.login_focus_otp;
            } else {
                app.focus_sidebar = !app.focus_sidebar;
            }
        }

        // ── Chat actions ─────────────────────────────────────────────────────
        Action::SendMessage => {
            if app.screen == Screen::Login {
                let password = app.login_password.clone();
                let totp = app.login_totp.clone();
                app.login_loading = true;
                app.login_error = None;
                let c = client.clone();
                let t = tx.clone();
                tokio::spawn(async move {
                    match auth::login(&c, &password, &totp).await {
                        Ok(_) => {
                            let _ = t.send(AppEvent::LoginResult(Ok(())));
                        }
                        Err(e) => {
                            let _ = t.send(AppEvent::LoginResult(Err(e.to_string())));
                        }
                    }
                });
            } else if app.screen == Screen::Chat && !app.chat_input.trim().is_empty() {
                let text = app.chat_input.trim().to_string();
                app.chat_input.clear();
                app.chat_input_cursor = 0;
                app.push_user_message(text.clone());
                app.start_assistant_stream();

                let c = client.clone();
                let t = tx.clone();
                let messages: Vec<ChatMessage> = app
                    .chat_messages
                    .iter()
                    .filter(|m| !m.is_tool && !m.is_thinking)
                    .map(|m| ChatMessage {
                        role: m.role.clone(),
                        content: m.content.clone(),
                    })
                    .collect();

                tokio::spawn(async move {
                    let req = ChatCompletionRequest {
                        model: "aurago".to_string(),
                        messages,
                        stream: true,
                    };
                    match c
                        .request::<ChatCompletionRequest, serde_json::Value>(
                            Method::POST,
                            "/v1/chat/completions",
                            Some(&req),
                        )
                        .await
                    {
                        Ok(_) => {
                            let _ = t.send(AppEvent::ChatSent);
                        }
                        Err(e) => {
                            let _ = t.send(AppEvent::ChatError(e.to_string()));
                        }
                    }
                });
            }
        }
        Action::NewLine => {
            if app.screen == Screen::Chat {
                app.insert_at_cursor('\n');
            }
        }
        Action::ClearChat => {
            let c = client.clone();
            tokio::spawn(async move {
                let _ = auth::clear_history(&c).await;
            });
            app.chat_messages.clear();
            app.scroll = 0;
        }
        Action::Logout => {
            let c = client.clone();
            let path = crate::config::session_cookie_path().ok();
            tokio::spawn(async move {
                let _ = auth::logout(&c).await;
                if let Some(p) = path {
                    let _ = auth::delete_session_cookie(&p);
                }
            });
            app.authenticated = false;
            app.screen = Screen::Login;
            app.nav_bar_open = false;
            app.chat_input.clear();
            app.chat_input_cursor = 0;
            app.chat_messages.clear();
            app.scroll = 0;
            if let Some(h) = sse_handle.take() {
                h.abort();
            }
        }

        // ── Scrolling ────────────────────────────────────────────────────────
        Action::ScrollUp => {
            if app.screen == Screen::Chat {
                app.auto_scroll = false;
                let max = app.chat_messages.len().saturating_sub(1);
                if app.scroll > max {
                    app.scroll = max;
                } else if app.scroll > 0 {
                    app.scroll -= 1;
                }
            } else if app.screen == Screen::Dashboard {
                // Scroll logs
            }
        }
        Action::ScrollDown => {
            if app.screen == Screen::Chat {
                let max = app.chat_messages.len().saturating_sub(1);
                if app.scroll > max {
                    app.scroll = max;
                } else if app.scroll < max {
                    app.scroll += 1;
                }
                // Re-enable auto-scroll if we've scrolled to the bottom
                if app.scroll >= max {
                    app.auto_scroll = true;
                }
            }
        }
        Action::ScrollTop => {
            app.scroll = 0;
            if app.screen == Screen::Chat {
                app.auto_scroll = false;
            }
        }
        Action::ScrollBottom => {
            if app.screen == Screen::Chat {
                app.auto_scroll = true;
            } else {
                app.scroll = 0;
            }
        }

        // ── Cursor ───────────────────────────────────────────────────────────
        Action::Backspace => {
            if app.screen == Screen::Login {
                if app.login_focus_otp {
                    app.login_totp.pop();
                } else {
                    app.login_password.pop();
                }
            } else if app.screen == Screen::Config && app.config_editing {
                if app.config_edit_cursor > 0 {
                    app.config_edit_cursor -= 1;
                    let byte_idx = char_to_byte(&app.config_edit_value, app.config_edit_cursor);
                    if byte_idx < app.config_edit_value.len() {
                        app.config_edit_value.remove(byte_idx);
                    }
                }
            } else if app.screen == Screen::Knowledge && app.knowledge_search_active {
                app.knowledge_search.pop();
            } else if app.screen == Screen::Media && app.media_search_active {
                app.media_search.pop();
            } else {
                app.backspace_at_cursor();
            }
        }
        Action::DeleteChar => {
            if app.screen == Screen::Config && app.config_editing {
                if app.config_edit_cursor < char_len(&app.config_edit_value) {
                    let byte_idx = char_to_byte(&app.config_edit_value, app.config_edit_cursor);
                    if byte_idx < app.config_edit_value.len() {
                        app.config_edit_value.remove(byte_idx);
                    }
                }
            } else if app.screen == Screen::Chat {
                app.delete_at_cursor();
            }
        }
        Action::CursorLeft => {
            if app.screen == Screen::Config && app.config_editing {
                if app.config_edit_cursor > 0 {
                    app.config_edit_cursor -= 1;
                }
            } else if app.screen == Screen::Chat {
                app.cursor_left();
            }
        }
        Action::CursorRight => {
            if app.screen == Screen::Config && app.config_editing {
                if app.config_edit_cursor < char_len(&app.config_edit_value) {
                    app.config_edit_cursor += 1;
                }
            } else if app.screen == Screen::Chat {
                app.cursor_right();
            }
        }
        Action::CursorStart => {
            if app.screen == Screen::Config && app.config_editing {
                app.config_edit_cursor = 0;
            } else if app.screen == Screen::Chat {
                app.cursor_start();
            }
        }
        Action::CursorEnd => {
            if app.screen == Screen::Config && app.config_editing {
                app.config_edit_cursor = char_len(&app.config_edit_value);
            } else if app.screen == Screen::Chat {
                app.cursor_end();
            }
        }
        Action::Type(c) => {
            let is_backspace = c == '\u{7f}' || c == '\u{8}';
            if app.screen == Screen::Login {
                if app.login_focus_otp {
                    if is_backspace {
                        app.login_totp.pop();
                    } else if c.is_ascii_digit() && app.login_totp.len() < 6 {
                        app.login_totp.push(c);
                    }
                } else {
                    if is_backspace {
                        app.login_password.pop();
                    } else if !c.is_control() {
                        app.login_password.push(c);
                    }
                }
            } else if app.screen == Screen::Config && app.config_editing {
                if is_backspace {
                    if app.config_edit_cursor > 0 {
                        app.config_edit_cursor -= 1;
                        let byte_idx = char_to_byte(&app.config_edit_value, app.config_edit_cursor);
                        if byte_idx < app.config_edit_value.len() {
                            app.config_edit_value.remove(byte_idx);
                        }
                    }
                } else if !c.is_control() {
                    let byte_idx = char_to_byte(&app.config_edit_value, app.config_edit_cursor);
                    app.config_edit_value.insert(byte_idx, c);
                    app.config_edit_cursor += 1;
                }
            } else if app.screen == Screen::Knowledge && app.knowledge_search_active {
                if is_backspace {
                    app.knowledge_search.pop();
                } else if !c.is_control() {
                    app.knowledge_search.push(c);
                }
            } else if app.screen == Screen::Media && app.media_search_active {
                if is_backspace {
                    app.media_search.pop();
                } else if !c.is_control() {
                    app.media_search.push(c);
                }
            } else {
                if is_backspace {
                    app.backspace_at_cursor();
                } else if c == '\n' || !c.is_control() {
                    app.insert_at_cursor(c);
                }
            }
        }

        // ── Navigation ───────────────────────────────────────────────────────
        Action::NavigateLeft => {
            let idx = app.screen.nav_index();
            if idx > 0 {
                if let Some(prev) = Screen::from_nav_index(idx - 1) {
                    navigate_and_load(app, prev, client, tx);
                }
            }
        }
        Action::NavigateRight => {
            let idx = app.screen.nav_index();
            if let Some(next) = Screen::from_nav_index(idx + 1) {
                navigate_and_load(app, next, client, tx);
            }
        }
        Action::OpenNavBar => {
            app.nav_bar_open = true;
            app.nav_bar_index = app.screen.nav_index();
        }
        Action::CloseNavBar => {
            app.nav_bar_open = false;
        }
        Action::NavUp => {
            if app.nav_bar_index > 0 {
                app.nav_bar_index -= 1;
            }
        }
        Action::NavDown => {
            let max = Screen::nav_items().len().saturating_sub(1);
            if app.nav_bar_index < max {
                app.nav_bar_index += 1;
            }
        }
        Action::NavSelect => {
            if let Some(screen) = Screen::from_nav_index(app.nav_bar_index) {
                navigate_and_load(app, screen, client, tx);
            }
            app.nav_bar_open = false;
        }
        Action::GoToChat => navigate_and_load(app, Screen::Chat, client, tx),
        Action::GoToDashboard => navigate_and_load(app, Screen::Dashboard, client, tx),
        Action::GoToPlans => navigate_and_load(app, Screen::Plans, client, tx),
        Action::GoToMissions => navigate_and_load(app, Screen::Missions, client, tx),
        Action::GoToSkills => navigate_and_load(app, Screen::Skills, client, tx),
        Action::GoToContainers => navigate_and_load(app, Screen::Containers, client, tx),
        Action::GoToConfig => navigate_and_load(app, Screen::Config, client, tx),
        Action::GoToKnowledge => navigate_and_load(app, Screen::Knowledge, client, tx),
        Action::GoToMedia => navigate_and_load(app, Screen::Media, client, tx),

        // ── List navigation ──────────────────────────────────────────────────
        Action::ListUp => {
            let len = app.list_len();
            let selected = app.list_selected_mut();
            match selected {
                Some(idx) if *idx > 0 => *idx -= 1,
                None if len > 0 => *selected = Some(0),
                _ => {}
            }
        }
        Action::ListDown => {
            let len = app.list_len();
            let selected = app.list_selected_mut();
            match selected {
                Some(idx) if *idx < len.saturating_sub(1) => *idx += 1,
                None if len > 0 => *selected = Some(0),
                _ => {}
            }
        }
        Action::ListSelect => {
            // Load detail for selected item
            load_detail_for_selected(app, client, tx);
        }
        Action::ListBack => {
            // Deselect / go back to list from detail
            *app.list_selected_mut() = None;
        }

        // ── Session drawer ───────────────────────────────────────────────────
        Action::ToggleSessionDrawer => {
            if app.screen == Screen::Config {
                // Save config instead of toggling drawer when in config
                if app.config_dirty {
                    let c = client.clone();
                    let t = tx.clone();
                    let data = app.config_data.clone();
                    tokio::spawn(async move {
                        let result = auth::save_config(&c, &data)
                            .await
                            .map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::ConfigSaved(result));
                    });
                }
            } else {
                app.session_drawer_open = !app.session_drawer_open;
                app.session_drawer_index = 0;
                if app.session_drawer_open {
                    // Load sessions
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::fetch_sessions(&c).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::SessionsLoaded(result));
                    });
                }
            }
        }
        Action::SessionUp => {
            if app.session_drawer_open && app.session_drawer_index > 0 {
                app.session_drawer_index -= 1;
            }
        }
        Action::SessionDown => {
            if app.session_drawer_open {
                let max = app.sessions.len().saturating_sub(1);
                if app.session_drawer_index < max {
                    app.session_drawer_index += 1;
                }
            }
        }
        Action::SessionSelect => {
            // Select session at drawer index
            if let Some(session) = app.sessions.get(app.session_drawer_index) {
                if session.id != app.active_session_id {
                    let new_id = session.id.clone();
                    app.active_session_id = new_id;
                    app.chat_messages.clear();
                    app.session_drawer_open = false;
                    app.status_message = "Switched session".to_string();
                    // Load history for selected session
                    let c = client.clone();
                    let t = tx.clone();
                    let sid = app.active_session_id.clone();
                    tokio::spawn(async move {
                        let result = auth::fetch_history_for_session(&c, &sid)
                            .await
                            .map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::HistoryLoaded(result));
                    });
                }
            }
        }
        Action::SessionNew => {
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::create_session(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::SessionCreated(result));
            });
        }
        Action::SessionDelete => {
            // Delete session at drawer index (prevent deleting active session)
            if let Some(session) = app.sessions.get(app.session_drawer_index) {
                if session.id != app.active_session_id {
                    let id = session.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::delete_session(&c, &id)
                            .await
                            .map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::SessionDeleted(result));
                    });
                } else {
                    app.toast = Some("Cannot delete active session".to_string());
                    app.toast_ticks = 8;
                }
            }
        }

        // ── Dashboard tabs ───────────────────────────────────────────────────
        Action::TabLeft => {
            app.dash_tab = match app.dash_tab {
                DashTab::Overview => DashTab::Logs,
                DashTab::Agent => DashTab::Overview,
                DashTab::System => DashTab::Agent,
                DashTab::Logs => DashTab::System,
            };
        }
        Action::TabRight => {
            app.dash_tab = match app.dash_tab {
                DashTab::Overview => DashTab::Agent,
                DashTab::Agent => DashTab::System,
                DashTab::System => DashTab::Logs,
                DashTab::Logs => DashTab::Overview,
            };
        }

        // ── Actions ──────────────────────────────────────────────────────────
        Action::Refresh => {
            load_data_for_screen(app, client, tx);
        }
        Action::ActionPrimary => {
            // Execute primary action on selected item (run/advance/start)
            execute_primary_action(app, client, tx);
        }
        Action::ActionDelete => {
            // Show confirmation dialog instead of immediately deleting
            show_delete_confirmation(app);
        }
        Action::ActionToggle => {
            execute_toggle_action(app, client, tx);
        }

        // ── Search ────────────────────────────────────────────────────────────
        Action::SearchActivate => match app.screen {
            Screen::Knowledge => app.knowledge_search_active = true,
            Screen::Media => app.media_search_active = true,
            _ => {}
        },
        Action::SearchDeactivate => match app.screen {
            Screen::Knowledge => {
                app.knowledge_search_active = false;
                app.knowledge_search.clear();
            }
            Screen::Media => {
                app.media_search_active = false;
                app.media_search.clear();
            }
            _ => {}
        },
        Action::SearchSubmit => {
            match app.screen {
                Screen::Knowledge => {
                    app.knowledge_search_active = false;
                    // Reload with search query (future: server-side search)
                    load_data_for_screen(app, client, tx);
                }
                Screen::Media => {
                    app.media_search_active = false;
                    app.media_offset = 0;
                    load_data_for_screen(app, client, tx);
                }
                _ => {}
            }
        }

        // ── Config editing ────────────────────────────────────────────────────
        Action::EditField => {
            if app.screen == Screen::Config && !app.config_editing {
                // Get current field value and start editing
                let section_key = app.config_sections.get(app.config_section_index);
                if let Some(key) = section_key {
                    if let Some(data) = app.config_data.get(key) {
                        let fields = collect_config_fields(data);
                        // Clamp field index to valid range
                        if app.config_field_index >= fields.len() {
                            app.config_field_index = fields.len().saturating_sub(1);
                        }
                        if let Some((_, value)) = fields.get(app.config_field_index) {
                            app.config_edit_value = value.to_string();
                            app.config_edit_cursor = char_len(&app.config_edit_value);
                            app.config_editing = true;
                        }
                    }
                }
            }
        }
        Action::EditSave => {
            if app.screen == Screen::Config && app.config_editing {
                app.config_editing = false;
                app.config_dirty = true;
                // Apply the edit to config_data
                let section_key = app.config_sections.get(app.config_section_index).cloned();
                if let Some(key) = section_key {
                    let fields = app.config_data.get(&key).map(collect_config_fields);
                    if let Some(fields) = fields {
                        if let Some((field_key, _)) = fields.get(app.config_field_index) {
                            // Parse the edit value
                            let new_val = parse_edit_value(&app.config_edit_value);
                            set_nested_config_value(&mut app.config_data, &key, field_key, new_val);
                        }
                    }
                }
                app.config_edit_value.clear();
            }
        }
        Action::EditCancel => {
            app.config_editing = false;
            app.config_edit_value.clear();
        }
        Action::SectionUp => {
            if app.screen == Screen::Config {
                if app.config_section_index > 0 {
                    app.config_section_index -= 1;
                    app.config_field_index = 0;
                }
            } else if app.screen == Screen::Media {
                // Switch tab left
                app.media_tab = match app.media_tab {
                    MediaTab::Audio => MediaTab::Documents,
                    MediaTab::Documents => MediaTab::Audio,
                };
                app.media_offset = 0;
                load_data_for_screen(app, client, tx);
            }
        }
        Action::SectionDown => {
            if app.screen == Screen::Config {
                if app.config_section_index < app.config_sections.len().saturating_sub(1) {
                    app.config_section_index += 1;
                    app.config_field_index = 0;
                }
            } else if app.screen == Screen::Media {
                // Switch tab right
                app.media_tab = match app.media_tab {
                    MediaTab::Audio => MediaTab::Documents,
                    MediaTab::Documents => MediaTab::Audio,
                };
                app.media_offset = 0;
                load_data_for_screen(app, client, tx);
            }
        }
    }
}

// ── Navigation helpers ────────────────────────────────────────────────────────

fn navigate_and_load(
    app: &mut AppState,
    screen: Screen,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
    app.navigate_to(screen);
    load_data_for_screen(app, client, tx);
}

/// Load data appropriate for the current screen
fn load_data_for_screen(app: &mut AppState, client: &ApiClient, tx: &UnboundedSender<AppEvent>) {
    match app.screen {
        Screen::Dashboard => {
            app.dash_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_system_info(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::DashboardSystemLoaded(result));
            });
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_budget(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::DashboardBudgetLoaded(result));
            });
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_overview(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::DashboardOverviewLoaded(result));
            });
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_personality_state(&c)
                    .await
                    .map_err(|e| e.to_string());
                let _ = t.send(AppEvent::DashboardPersonalityLoaded(result));
            });
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_logs(&c, 100).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::DashboardLogsLoaded(result));
            });
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_activity(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::DashboardActivityLoaded(result));
            });
        }
        Screen::Plans => {
            app.plans_loading = true;
            let c = client.clone();
            let t = tx.clone();
            let sid = app.active_session_id.clone();
            tokio::spawn(async move {
                let result = auth::fetch_plans(&c, &sid).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::PlansLoaded(result));
            });
        }
        Screen::Missions => {
            app.missions_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_missions(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::MissionsLoaded(result));
            });
        }
        Screen::Skills => {
            app.skills_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_skills(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::SkillsLoaded(result));
            });
        }
        Screen::Containers => {
            app.containers_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_containers(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::ContainersLoaded(result));
            });
        }
        Screen::Config => {
            app.config_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_config(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::ConfigLoaded(result));
            });
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_config_schema(&c)
                    .await
                    .map_err(|e| e.to_string());
                let _ = t.send(AppEvent::ConfigSchemaLoaded(result));
            });
        }
        Screen::Knowledge => {
            app.knowledge_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_knowledge_files(&c)
                    .await
                    .map_err(|e| e.to_string());
                let _ = t.send(AppEvent::KnowledgeFilesLoaded(result));
            });
        }
        Screen::Media => {
            app.media_loading = true;
            let c = client.clone();
            let t = tx.clone();
            let media_type = match app.media_tab {
                MediaTab::Audio => "audio",
                MediaTab::Documents => "documents",
            }
            .to_string();
            let offset = app.media_offset;
            let query = if app.media_search.is_empty() {
                None
            } else {
                Some(app.media_search.clone())
            };
            tokio::spawn(async move {
                let result = auth::fetch_media(&c, &media_type, 50, offset, query.as_deref())
                    .await
                    .map_err(|e| e.to_string());
                let _ = t.send(AppEvent::MediaLoaded(result));
            });
        }
        _ => {}
    }
}

/// Load detail data for the currently selected list item
fn load_detail_for_selected(app: &AppState, client: &ApiClient, tx: &UnboundedSender<AppEvent>) {
    match app.screen {
        Screen::Plans => {
            if let Some(idx) = app.plans_selected {
                if let Some(plan) = app.plans.get(idx) {
                    let id = plan.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::fetch_plan_detail(&c, &id)
                            .await
                            .map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::PlanDetailLoaded(result));
                    });
                }
            }
        }
        Screen::Containers => {
            if let Some(idx) = app.containers_selected {
                if let Some(container) = app.containers.get(idx) {
                    let id = container.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::fetch_container_logs(&c, &id)
                            .await
                            .map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::ContainerLogsLoaded(result));
                    });
                }
            }
        }
        _ => {}
    }
}

/// Execute the primary action (Enter) for the selected item
fn execute_primary_action(app: &AppState, client: &ApiClient, tx: &UnboundedSender<AppEvent>) {
    match app.screen {
        Screen::Plans => {
            if let Some(idx) = app.plans_selected {
                if let Some(plan) = app.plans.get(idx) {
                    let id = plan.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::advance_plan(&c, &id).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::PlanActionDone(result));
                    });
                }
            }
        }
        Screen::Missions => {
            if let Some(idx) = app.missions_selected {
                if let Some(mission) = app.missions.get(idx) {
                    let id = mission.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::run_mission(&c, &id).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::MissionActionDone(result));
                    });
                }
            }
        }
        Screen::Containers => {
            if let Some(idx) = app.containers_selected {
                if let Some(container) = app.containers.get(idx) {
                    let action_str = if container.state == "running" {
                        "stop"
                    } else {
                        "start"
                    };
                    let id = container.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    let a = action_str.to_string();
                    tokio::spawn(async move {
                        let result = auth::container_action(&c, &id, &a)
                            .await
                            .map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::ContainerActionDone(result));
                    });
                }
            }
        }
        _ => {}
    }
}

/// Execute toggle action (enable/disable) for the selected item
fn execute_toggle_action(app: &AppState, client: &ApiClient, tx: &UnboundedSender<AppEvent>) {
    if app.screen == Screen::Skills {
        if let Some(idx) = app.skills_selected {
            if let Some(skill) = app.skills.get(idx) {
                let id = skill.id.clone();
                let new_state = !skill.enabled;
                let c = client.clone();
                let t = tx.clone();
                tokio::spawn(async move {
                    let result = auth::toggle_skill(&c, &id, new_state)
                        .await
                        .map_err(|e| e.to_string());
                    let _ = t.send(AppEvent::SkillActionDone(result));
                });
            }
        }
    }
}

/// Show a delete confirmation dialog for the current context
fn show_delete_confirmation(app: &mut AppState) {
    let confirm = match app.screen {
        Screen::Missions => app
            .missions_selected
            .map(|i| ConfirmAction::DeleteMission { index: i }),
        Screen::Containers => app
            .containers_selected
            .map(|i| ConfirmAction::DeleteContainer { index: i }),
        Screen::Knowledge => app
            .knowledge_selected
            .map(|i| ConfirmAction::DeleteKnowledge { index: i }),
        Screen::Media => app
            .media_selected
            .map(|i| ConfirmAction::DeleteMedia { index: i }),
        _ => None,
    };
    if let Some(c) = confirm {
        app.confirm_action = Some(c);
    }
}

/// Execute a previously confirmed destructive action
pub fn execute_confirmed_action(
    confirm: ConfirmAction,
    app: &mut AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
    match confirm {
        ConfirmAction::DeleteMission { index } => {
            if let Some(mission) = app.missions.get(index) {
                let id = mission.id.clone();
                let c = client.clone();
                let t = tx.clone();
                tokio::spawn(async move {
                    let result = auth::delete_mission(&c, &id)
                        .await
                        .map_err(|e| e.to_string());
                    let _ = t.send(AppEvent::MissionActionDone(result));
                });
            }
        }
        ConfirmAction::DeleteContainer { index } => {
            if let Some(container) = app.containers.get(index) {
                let id = container.id.clone();
                let c = client.clone();
                let t = tx.clone();
                tokio::spawn(async move {
                    let result = auth::remove_container(&c, &id, false)
                        .await
                        .map_err(|e| e.to_string());
                    let _ = t.send(AppEvent::ContainerActionDone(result));
                });
            }
        }
        ConfirmAction::DeleteKnowledge { index } => {
            if let Some(file) = app.knowledge_files.get(index) {
                let name = file.name.clone();
                let c = client.clone();
                let t = tx.clone();
                tokio::spawn(async move {
                    let result = auth::delete_knowledge_file(&c, &name)
                        .await
                        .map_err(|e| e.to_string());
                    let _ = t.send(AppEvent::KnowledgeFileDeleted(result));
                });
            }
        }
        ConfirmAction::DeleteMedia { index } => {
            if let Some(item) = app.media_items.get(index) {
                let id = item.id;
                let c = client.clone();
                let t = tx.clone();
                tokio::spawn(async move {
                    let result = auth::delete_media(&c, id).await.map_err(|e| e.to_string());
                    let _ = t.send(AppEvent::MediaDeleted(result));
                });
            }
        }
        ConfirmAction::ClearChat => {
            let c = client.clone();
            tokio::spawn(async move {
                let _ = auth::clear_history(&c).await;
            });
            app.chat_messages.clear();
            app.scroll = 0;
        }
    }
}



// ── Config helpers (moved from main.rs) ───────────────────────────────────────

/// Collect flat key-value pairs from a JSON object (one level)
fn collect_config_fields(data: &serde_json::Value) -> Vec<(String, serde_json::Value)> {
    let mut result = Vec::new();
    if let Some(obj) = data.as_object() {
        for (key, value) in obj {
            result.push((key.clone(), value.clone()));
        }
    }
    result
}

/// Parse a string edit value into a JSON value
fn parse_edit_value(s: &str) -> serde_json::Value {
    // Try to parse as JSON first
    if let Ok(v) = serde_json::from_str::<serde_json::Value>(s) {
        return v;
    }
    // Try boolean
    match s.to_lowercase().as_str() {
        "true" => return serde_json::Value::Bool(true),
        "false" => return serde_json::Value::Bool(false),
        _ => {}
    }
    // Try number
    if let Ok(n) = s.parse::<i64>() {
        return serde_json::json!(n);
    }
    if let Ok(f) = s.parse::<f64>() {
        return serde_json::json!(f);
    }
    // Default to string
    serde_json::Value::String(s.to_string())
}

/// Set a nested config value at section.field_path
fn set_nested_config_value(
    config: &mut serde_json::Value,
    section: &str,
    field_key: &str,
    new_val: serde_json::Value,
) {
    if let Some(section_data) = config.get_mut(section) {
        if let Some(obj) = section_data.as_object_mut() {
            obj.insert(field_key.to_string(), new_val);
        }
    }
}