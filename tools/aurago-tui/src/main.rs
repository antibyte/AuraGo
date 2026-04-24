use std::io;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use anyhow::{Context, Result};
use clap::Parser;
use crossterm::{
    event::{DisableMouseCapture, EnableMouseCapture, EventStream},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use futures::StreamExt;
use ratatui::{backend::CrosstermBackend, Terminal};
use reqwest::Method;
use tokio::sync::mpsc::{self, UnboundedReceiver, UnboundedSender};
use tokio::time::interval;

mod api;
mod app;
mod config;
mod events;
mod ui;

use api::{auth, sse, types::*, ApiClient};
use app::{AppState, ConfirmAction, DashTab, MediaTab, Screen};
use events::keybindings::{map_key, Action, KeyContext};
use events::AppEvent;
use ui::theme::Theme;

#[derive(Parser, Debug)]
#[command(name = "aurago-tui")]
#[command(about = "AuraGo Terminal Chat Client")]
struct Args {
    #[arg(short, long, default_value = "http://localhost:8080")]
    url: String,
    #[arg(long, help = "Skip TLS certificate verification (insecure)")]
    insecure: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();
    let mut cfg = config::Config::load().unwrap_or_default();
    if !args.url.is_empty() {
        cfg.server_url = args.url;
    }
    cfg.save().ok();

    let mut terminal = setup_terminal()?;
    let app = Arc::new(Mutex::new(AppState {
        server_url: cfg.server_url.clone(),
        theme_name: cfg.theme.clone(),
        ..AppState::default()
    }));

    let client = match ApiClient::new(&cfg.server_url, args.insecure) {
        Ok(c) => c,
        Err(e) => {
            restore_terminal(&mut terminal)?;
            eprintln!("Failed to create API client: {}", e);
            std::process::exit(1);
        }
    };

    // Try to restore saved session cookie
    if let Ok(cookie_path) = config::session_cookie_path() {
        if let Some(cookie) = auth::load_session_cookie(&cookie_path) {
            client.set_session_cookie(cookie);
        }
    }

    let (event_tx, event_rx) = mpsc::unbounded_channel::<AppEvent>();
    let result = run_app(&mut terminal, app.clone(), client, event_tx, event_rx).await;

    restore_terminal(&mut terminal)?;
    result
}

fn setup_terminal() -> Result<Terminal<CrosstermBackend<io::Stdout>>> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    Terminal::new(backend).context("Failed to create terminal")
}

fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<io::Stdout>>) -> Result<()> {
    disable_raw_mode()?;
    execute!(
        terminal.backend_mut(),
        LeaveAlternateScreen,
        DisableMouseCapture
    )?;
    terminal.show_cursor()?;
    Ok(())
}

// ── Main event loop ──────────────────────────────────────────────────────────

async fn run_app(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    app: Arc<Mutex<AppState>>,
    client: ApiClient,
    event_tx: UnboundedSender<AppEvent>,
    mut event_rx: UnboundedReceiver<AppEvent>,
) -> Result<()> {
    let mut reader = EventStream::new().fuse();
    let mut tick = interval(Duration::from_millis(100));

    // Kick off auth check immediately
    let client_clone = client.clone();
    let tx = event_tx.clone();
    tokio::spawn(async move {
        match auth::fetch_auth_status(&client_clone).await {
            Ok(status) => {
                let _ = tx.send(AppEvent::AuthCheckResult(Ok((
                    status.enabled,
                    status.password_set,
                    status.totp_enabled,
                ))));
            }
            Err(e) => {
                let _ = tx.send(AppEvent::AuthCheckResult(Err(e.to_string())));
            }
        }
    });

    let mut sse_handle: Option<tokio::task::JoinHandle<()>> = None;

    loop {
        // ── Draw UI ──────────────────────────────────────────────────────────
        {
            let mut app_lock = app.lock().unwrap();
            let tick_val = app_lock.tick_counter;
            
            // Auto-scroll: clamp scroll to bottom of messages in chat
            if app_lock.auto_scroll && app_lock.screen == Screen::Chat && !app_lock.chat_messages.is_empty() {
                app_lock.scroll = app_lock.chat_messages.len().saturating_sub(1);
            }
            // Clamp scroll to valid range
            if app_lock.scroll > app_lock.chat_messages.len().saturating_sub(1) {
                app_lock.scroll = app_lock.chat_messages.len().saturating_sub(1);
            }
            
            let current_theme = if app_lock.screen == Screen::Chat {
                Theme::from_mood(app_lock.personality.mood.as_deref().unwrap_or("neutral"))
            } else {
                Theme::by_name(&app_lock.theme_name)
            };
            terminal
                .draw(|f| {
                    // Draw nav bar overlay if open
                    if app_lock.nav_bar_open {
                        draw_nav_bar(f, &app_lock, &current_theme);
                    }
                    match app_lock.screen {
                        Screen::Splash => ui::splash::draw_splash(f, &current_theme, tick_val),
                        Screen::Login => ui::login::draw_login(f, &app_lock, &current_theme),
                        Screen::Chat => ui::chat::draw_chat(f, &app_lock, &current_theme),
                        Screen::Dashboard => ui::dashboard::draw_dashboard(f, &app_lock, &current_theme),
                        Screen::Plans => ui::plans::draw_plans(f, &app_lock, &current_theme),
                        Screen::Missions => ui::missions::draw_missions(f, &app_lock, &current_theme),
                        Screen::Skills => ui::skills::draw_skills(f, &app_lock, &current_theme),
                        Screen::Containers => ui::containers::draw_containers(f, &app_lock, &current_theme),
                        Screen::Config => ui::config::draw_config(f, &app_lock, &current_theme),
                        Screen::Knowledge => ui::knowledge::draw_knowledge(f, &app_lock, &current_theme),
                        Screen::Media => ui::media::draw_media(f, &app_lock, &current_theme),
                    }
                    
                    // Draw confirmation dialog overlay
                    if app_lock.confirm_action.is_some() {
                        draw_confirm_dialog(f, &app_lock, &current_theme);
                    }
                })
                .context("Failed to draw UI")?;
        }

        // ── Wait for next event ──────────────────────────────────────────────
        let event = tokio::select! {
            Some(Ok(ev)) = reader.next() => AppEvent::Crossterm(ev),
            _ = tick.tick() => AppEvent::Tick,
            Some(ev) = event_rx.recv() => ev,
        };

        let mut app_lock = app.lock().unwrap();

        match event {
            AppEvent::Crossterm(crossterm::event::Event::Key(key)) => {
                handle_key_event(key, &mut app_lock, &client, &event_tx, &mut sse_handle);
            }
            AppEvent::Crossterm(crossterm::event::Event::Mouse(mouse)) => {
                match mouse.kind {
                    crossterm::event::MouseEventKind::ScrollUp => {
                        if app_lock.screen == Screen::Chat {
                            app_lock.auto_scroll = false;
                            if app_lock.scroll > 0 {
                                app_lock.scroll -= 3; // Scroll 3 lines at a time
                            }
                        } else if app_lock.screen == Screen::Dashboard {
                            // Could scroll logs in future
                        }
                    }
                    crossterm::event::MouseEventKind::ScrollDown => {
                        if app_lock.screen == Screen::Chat {
                            let max = app_lock.chat_messages.len().saturating_sub(1);
                            if app_lock.scroll < max {
                                app_lock.scroll = (app_lock.scroll + 3).min(max);
                            }
                            if app_lock.scroll >= max {
                                app_lock.auto_scroll = true;
                            }
                        }
                    }
                    _ => {}
                }
            }
            AppEvent::Crossterm(_) => {}
            AppEvent::Tick => {
                app_lock.tick();
            }
            AppEvent::Sse(ev) => {
                app_lock.apply_sse_event(ev);
            }
            AppEvent::ChatSent => {}
            AppEvent::ChatError(err) => {
                app_lock.status_message = format!("Chat error: {}", err);
            }
            AppEvent::LoginResult(result) => {
                app_lock.login_loading = false;
                match result {
                    Ok(()) => {
                        app_lock.authenticated = true;
                        app_lock.navigate_to(Screen::Chat);
                        // Persist session cookie
                        if let Ok(path) = config::session_cookie_path() {
                            if let Some(cookie) = client.get_session_cookie() {
                                let _ = auth::save_session_cookie(&cookie, &path);
                            }
                        }
                        let tx = event_tx.clone();
                        let c = client.clone();
                        tokio::spawn(async move {
                            start_chat_session(&c, tx).await;
                        });
                    }
                    Err(e) => {
                        app_lock.login_error = Some(e);
                    }
                }
            }
            AppEvent::AuthCheckResult(result) => {
                match result {
                    Ok((enabled, password_set, totp_enabled)) => {
                        app_lock.auth_enabled = enabled;
                        app_lock.totp_enabled = totp_enabled;
                        if !enabled || !password_set {
                            app_lock.authenticated = true;
                            app_lock.navigate_to(Screen::Chat);
                            let tx = event_tx.clone();
                            let c = client.clone();
                            tokio::spawn(async move {
                                start_chat_session(&c, tx).await;
                            });
                        } else {
                            app_lock.authenticated = false;
                            app_lock.screen = Screen::Login;
                        }
                    }
                    Err(e) => {
                        app_lock.login_error = Some(format!("Auth check failed: {}", e));
                        app_lock.screen = Screen::Login;
                    }
                }
            }
            AppEvent::HistoryLoaded(result) => {
                match result {
                    Ok(history) => {
                        app_lock.load_history(history);
                        app_lock.status_message = "Connected".to_string();
                    }
                    Err(e) => {
                        app_lock.status_message = format!("History error: {}", e);
                    }
                }
            }

            // ── Sessions ─────────────────────────────────────────────────────
            AppEvent::SessionsLoaded(result) => {
                match result {
                    Ok(sessions) => {
                        app_lock.sessions = sessions;
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load sessions: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
            AppEvent::SessionCreated(result) => {
                match result {
                    Ok(session) => {
                        app_lock.active_session_id = session.id.clone();
                        app_lock.sessions.insert(0, session);
                        app_lock.chat_messages.clear();
                        app_lock.status_message = "New session created".to_string();
                        // Fetch history for new session
                        let c = client.clone();
                        let tx = event_tx.clone();
                        let sid = app_lock.active_session_id.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_history_for_session(&c, &sid).await
                                .map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::HistoryLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to create session: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
            AppEvent::SessionDeleted(result) => {
                match result {
                    Ok(()) => {
                        // Refresh sessions
                        let c = client.clone();
                        let tx = event_tx.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_sessions(&c).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::SessionsLoaded(result));
                        });
                        app_lock.status_message = "Session deleted".to_string();
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to delete session: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Dashboard ────────────────────────────────────────────────────
            AppEvent::DashboardSystemLoaded(result) => {
                match result {
                    Ok(info) => app_lock.dash_system = info,
                    Err(e) => app_lock.status_message = format!("System info error: {}", e),
                }
                app_lock.dash_loading = false;
            }
            AppEvent::DashboardBudgetLoaded(result) => {
                match result {
                    Ok(info) => app_lock.dash_budget = info,
                    Err(e) => app_lock.status_message = format!("Budget error: {}", e),
                }
            }
            AppEvent::DashboardOverviewLoaded(result) => {
                match result {
                    Ok(info) => app_lock.dash_overview = info,
                    Err(e) => app_lock.status_message = format!("Overview error: {}", e),
                }
            }
            AppEvent::DashboardPersonalityLoaded(result) => {
                match result {
                    Ok(info) => app_lock.dash_personality = info,
                    Err(e) => app_lock.status_message = format!("Personality error: {}", e),
                }
            }
            AppEvent::DashboardLogsLoaded(result) => {
                match result {
                    Ok(logs) => app_lock.dash_logs = logs,
                    Err(e) => app_lock.status_message = format!("Logs error: {}", e),
                }
            }
            AppEvent::DashboardActivityLoaded(result) => {
                match result {
                    Ok(activity) => app_lock.dash_activity = activity,
                    Err(e) => app_lock.status_message = format!("Activity error: {}", e),
                }
            }

            // ── Plans ────────────────────────────────────────────────────────
            AppEvent::PlansLoaded(result) => {
                match result {
                    Ok(plans) => {
                        app_lock.plans = plans;
                        // Auto-select first if none selected
                        if app_lock.plans_selected.is_none() && !app_lock.plans.is_empty() {
                            app_lock.plans_selected = Some(0);
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load plans: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.plans_loading = false;
            }
            AppEvent::PlanDetailLoaded(result) => {
                match result {
                    Ok(plan) => {
                        // Update the plan in the list if it exists
                        if let Some(idx) = app_lock.plans_selected {
                            if idx < app_lock.plans.len() && app_lock.plans[idx].id == plan.id {
                                app_lock.plans[idx] = plan;
                            }
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load plan: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
            AppEvent::PlanActionDone(result) => {
                match result {
                    Ok(()) => {
                        app_lock.status_message = "Plan action completed".to_string();
                        // Refresh plans
                        let c = client.clone();
                        let tx = event_tx.clone();
                        let sid = app_lock.active_session_id.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_plans(&c, &sid).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::PlansLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Plan action failed: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Missions ─────────────────────────────────────────────────────
            AppEvent::MissionsLoaded(result) => {
                match result {
                    Ok(missions) => {
                        app_lock.missions = missions;
                        if app_lock.missions_selected.is_none() && !app_lock.missions.is_empty() {
                            app_lock.missions_selected = Some(0);
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load missions: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.missions_loading = false;
            }
            AppEvent::MissionActionDone(result) => {
                match result {
                    Ok(()) => {
                        app_lock.status_message = "Mission action completed".to_string();
                        let c = client.clone();
                        let tx = event_tx.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_missions(&c).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::MissionsLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Mission action failed: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Skills ───────────────────────────────────────────────────────
            AppEvent::SkillsLoaded(result) => {
                match result {
                    Ok(skills) => {
                        app_lock.skills = skills;
                        if app_lock.skills_selected.is_none() && !app_lock.skills.is_empty() {
                            app_lock.skills_selected = Some(0);
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load skills: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.skills_loading = false;
            }
            AppEvent::SkillActionDone(result) => {
                match result {
                    Ok(()) => {
                        app_lock.status_message = "Skill action completed".to_string();
                        let c = client.clone();
                        let tx = event_tx.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_skills(&c).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::SkillsLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Skill action failed: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Containers ───────────────────────────────────────────────────
            AppEvent::ContainersLoaded(result) => {
                match result {
                    Ok(containers) => {
                        app_lock.containers = containers;
                        if app_lock.containers_selected.is_none() && !app_lock.containers.is_empty() {
                            app_lock.containers_selected = Some(0);
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load containers: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.containers_loading = false;
            }
            AppEvent::ContainerActionDone(result) => {
                match result {
                    Ok(_) => {
                        app_lock.status_message = "Container action completed".to_string();
                        let c = client.clone();
                        let tx = event_tx.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_containers(&c).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::ContainersLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Container action failed: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
            AppEvent::ContainerLogsLoaded(result) => {
                match result {
                    Ok(val) => {
                        // Display container logs as a toast or in a dedicated area
                        if let Some(logs) = val.as_str() {
                            app_lock.toast = Some(format!("Container logs:\n{}", truncate_str(logs, 500)));
                            app_lock.toast_ticks = 20;
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load container logs: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Config ──────────────────────────────────────────────────────
            AppEvent::ConfigLoaded(result) => {
                match result {
                    Ok(data) => {
                        app_lock.config_data = data;
                        // Extract top-level sections from config data
                        if let Some(obj) = app_lock.config_data.as_object() {
                            app_lock.config_sections = obj.keys().cloned().collect();
                            app_lock.config_sections.sort();
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load config: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.config_loading = false;
            }
            AppEvent::ConfigSchemaLoaded(result) => {
                match result {
                    Ok(schema) => {
                        app_lock.config_schema = schema;
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load config schema: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
            AppEvent::ConfigSaved(result) => {
                match result {
                    Ok(()) => {
                        app_lock.config_dirty = false;
                        app_lock.toast = Some("✓ Configuration saved".to_string());
                        app_lock.toast_ticks = 8;
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to save config: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
            AppEvent::VaultStatusLoaded(result) => {
                match result {
                    Ok(_status) => {
                        // Vault status loaded - could display in config UI
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Vault status error: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Knowledge ───────────────────────────────────────────────────
            AppEvent::KnowledgeFilesLoaded(result) => {
                match result {
                    Ok(files) => {
                        app_lock.knowledge_files = files;
                        if app_lock.knowledge_selected.is_none() && !app_lock.knowledge_files.is_empty() {
                            app_lock.knowledge_selected = Some(0);
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load knowledge: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.knowledge_loading = false;
            }
            AppEvent::KnowledgeFileDeleted(result) => {
                match result {
                    Ok(()) => {
                        app_lock.toast = Some("✓ File deleted".to_string());
                        app_lock.toast_ticks = 8;
                        // Refresh knowledge files
                        let c = client.clone();
                        let tx = event_tx.clone();
                        tokio::spawn(async move {
                            let result = auth::fetch_knowledge_files(&c).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::KnowledgeFilesLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to delete file: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }

            // ── Media ───────────────────────────────────────────────────────
            AppEvent::MediaLoaded(result) => {
                match result {
                    Ok(resp) => {
                        app_lock.media_items = resp.items;
                        app_lock.media_total = resp.total;
                        if app_lock.media_selected.is_none() && !app_lock.media_items.is_empty() {
                            app_lock.media_selected = Some(0);
                        }
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to load media: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
                app_lock.media_loading = false;
            }
            AppEvent::MediaDeleted(result) => {
                match result {
                    Ok(_) => {
                        app_lock.toast = Some("✓ Media deleted".to_string());
                        app_lock.toast_ticks = 8;
                        // Refresh media
                        let c = client.clone();
                        let tx = event_tx.clone();
                        let media_type = match app_lock.media_tab {
                            MediaTab::Audio => "audio",
                            MediaTab::Documents => "documents",
                        }.to_string();
                        let offset = app_lock.media_offset;
                        let query = if app_lock.media_search.is_empty() { None } else { Some(app_lock.media_search.clone()) };
                        tokio::spawn(async move {
                            let result = auth::fetch_media(&c, &media_type, 50, offset, query.as_deref()).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::MediaLoaded(result));
                        });
                    }
                    Err(e) => {
                        app_lock.toast = Some(format!("Failed to delete media: {}", e));
                        app_lock.toast_ticks = 10;
                    }
                }
            }
        }

        if app_lock.should_quit {
            break;
        }
    }

    Ok(())
}

// ── Key event handling ────────────────────────────────────────────────────────

fn handle_key_event(
    key: crossterm::event::KeyEvent,
    app_lock: &mut AppState,
    client: &ApiClient,
    event_tx: &UnboundedSender<AppEvent>,
    sse_handle: &mut Option<tokio::task::JoinHandle<()>>,
) {
    // Splash screen: any key proceeds
    if app_lock.screen == Screen::Splash {
        app_lock.screen = if app_lock.auth_enabled && !app_lock.authenticated {
            Screen::Login
        } else {
            Screen::Chat
        };
        if app_lock.screen == Screen::Chat {
            let tx = event_tx.clone();
            let c = client.clone();
            tokio::spawn(async move {
                start_chat_session(&c, tx).await;
            });
        }
        return;
    }

    // Esc overlay stack: close innermost overlay first
    if key.code == crossterm::event::KeyCode::Esc && key.modifiers.is_empty() {
        if app_lock.show_help {
            app_lock.show_help = false;
            return;
        }
        if app_lock.nav_bar_open {
            app_lock.nav_bar_open = false;
            return;
        }
        if app_lock.session_drawer_open {
            app_lock.session_drawer_open = false;
            return;
        }
        if app_lock.media_search_active {
            app_lock.media_search_active = false;
            app_lock.media_search.clear();
            return;
        }
        if app_lock.knowledge_search_active {
            app_lock.knowledge_search_active = false;
            app_lock.knowledge_search.clear();
            return;
        }
        if app_lock.screen == Screen::Config && app_lock.config_editing {
            app_lock.config_editing = false;
            app_lock.config_edit_value.clear();
            app_lock.config_edit_cursor = 0;
            return;
        }
    }

    // Handle confirmation dialog
    if app_lock.confirm_action.is_some() {
        if key.code == crossterm::event::KeyCode::Char('y') || key.code == crossterm::event::KeyCode::Char('Y') {
            let action = app_lock.confirm_action.take();
            if let Some(confirm) = action {
                execute_confirmed_action(confirm, app_lock, client, event_tx);
            }
            return;
        } else {
            // Any other key cancels
            app_lock.confirm_action = None;
            return;
        }
    }

    // Determine key context
    let context = if app_lock.nav_bar_open {
        KeyContext::NavBar {
            index: app_lock.nav_bar_index,
            max: Screen::nav_items().len(),
        }
    } else {
        match app_lock.screen {
            Screen::Login => KeyContext::Login,
            Screen::Chat => KeyContext::Chat {
                focus_sidebar: app_lock.focus_sidebar,
                session_drawer: app_lock.session_drawer_open,
            },
            Screen::Dashboard => KeyContext::Dashboard {
                tab_index: match app_lock.dash_tab {
                    DashTab::Overview => 0,
                    DashTab::Agent => 1,
                    DashTab::System => 2,
                    DashTab::Logs => 3,
                },
                tab_count: 4,
            },
            Screen::Plans | Screen::Missions | Screen::Skills | Screen::Containers => {
                KeyContext::List {
                    selected: *app_lock.list_selected(),
                    len: app_lock.list_len(),
                }
            }
            Screen::Config => KeyContext::Config {
                section_index: app_lock.config_section_index,
                field_index: app_lock.config_field_index,
                editing: app_lock.config_editing,
            },
            Screen::Knowledge => {
                if app_lock.knowledge_search_active {
                    KeyContext::Search { active: true }
                } else {
                    KeyContext::List {
                        selected: *app_lock.list_selected(),
                        len: app_lock.list_len(),
                    }
                }
            }
            Screen::Media => {
                if app_lock.media_search_active {
                    KeyContext::Search { active: true }
                } else {
                    KeyContext::List {
                        selected: *app_lock.list_selected(),
                        len: app_lock.list_len(),
                    }
                }
            }
            Screen::Splash => KeyContext::Splash,
        }
    };

    let action = map_key(key, context);
    dispatch_action(action, app_lock, client, event_tx, sse_handle);
}

fn dispatch_action(
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
            if let Ok(mut cfg) = config::Config::load() {
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
                        Ok(_) => { let _ = t.send(AppEvent::LoginResult(Ok(()))); }
                        Err(e) => { let _ = t.send(AppEvent::LoginResult(Err(e.to_string()))); }
                    }
                });
            } else if app.screen == Screen::Chat && !app.chat_input.trim().is_empty() {
                let text = app.chat_input.trim().to_string();
                app.chat_input.clear();
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
                    match c.request::<ChatCompletionRequest, serde_json::Value>(
                        Method::POST, "/v1/chat/completions", Some(&req),
                    ).await {
                        Ok(_) => { let _ = t.send(AppEvent::ChatSent); }
                        Err(e) => { let _ = t.send(AppEvent::ChatError(e.to_string())); }
                    }
                });
            }
        }
        Action::NewLine => {
            if app.screen == Screen::Chat {
                app.chat_input.push('\n');
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
            let path = config::session_cookie_path().ok();
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
                    app.config_edit_value.remove(app.config_edit_cursor);
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
                if app.config_edit_cursor < app.config_edit_value.len() {
                    app.config_edit_value.remove(app.config_edit_cursor);
                }
            } else if app.screen == Screen::Chat {
                app.delete_at_cursor();
            }
        }
        Action::CursorLeft => {
            if app.screen == Screen::Config && app.config_editing {
                if app.config_edit_cursor > 0 { app.config_edit_cursor -= 1; }
            } else if app.screen == Screen::Chat {
                app.cursor_left();
            }
        }
        Action::CursorRight => {
            if app.screen == Screen::Config && app.config_editing {
                if app.config_edit_cursor < app.config_edit_value.len() { app.config_edit_cursor += 1; }
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
                app.config_edit_cursor = app.config_edit_value.len();
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
                        app.config_edit_value.remove(app.config_edit_cursor);
                    }
                } else if !c.is_control() {
                    app.config_edit_value.insert(app.config_edit_cursor, c);
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
                        let result = auth::save_config(&c, &data).await.map_err(|e| e.to_string());
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
                        let result = auth::fetch_history_for_session(&c, &sid).await
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
                        let result = auth::delete_session(&c, &id).await.map_err(|e| e.to_string());
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
        Action::SearchActivate => {
            match app.screen {
                Screen::Knowledge => app.knowledge_search_active = true,
                Screen::Media => app.media_search_active = true,
                _ => {}
            }
        }
        Action::SearchDeactivate => {
            match app.screen {
                Screen::Knowledge => {
                    app.knowledge_search_active = false;
                    app.knowledge_search.clear();
                }
                Screen::Media => {
                    app.media_search_active = false;
                    app.media_search.clear();
                }
                _ => {}
            }
        }
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
                    let fields = app.config_data.get(&key).map(|d| collect_config_fields(d));
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
fn load_data_for_screen(
    app: &mut AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
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
                let result = auth::fetch_personality_state(&c).await.map_err(|e| e.to_string());
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
                let result = auth::fetch_config_schema(&c).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::ConfigSchemaLoaded(result));
            });
        }
        Screen::Knowledge => {
            app.knowledge_loading = true;
            let c = client.clone();
            let t = tx.clone();
            tokio::spawn(async move {
                let result = auth::fetch_knowledge_files(&c).await.map_err(|e| e.to_string());
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
            }.to_string();
            let offset = app.media_offset;
            let query = if app.media_search.is_empty() { None } else { Some(app.media_search.clone()) };
            tokio::spawn(async move {
                let result = auth::fetch_media(&c, &media_type, 50, offset, query.as_deref()).await.map_err(|e| e.to_string());
                let _ = t.send(AppEvent::MediaLoaded(result));
            });
        }
        _ => {}
    }
}

/// Load detail data for the currently selected list item
fn load_detail_for_selected(
    app: &AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
    match app.screen {
        Screen::Plans => {
            if let Some(idx) = app.plans_selected {
                if let Some(plan) = app.plans.get(idx) {
                    let id = plan.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::fetch_plan_detail(&c, &id).await.map_err(|e| e.to_string());
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
                        let result = auth::fetch_container_logs(&c, &id).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::ContainerLogsLoaded(result));
                    });
                }
            }
        }
        _ => {}
    }
}

/// Execute the primary action (Enter) for the selected item
fn execute_primary_action(
    app: &AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
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
                    let action_str = if container.state == "running" { "stop" } else { "start" };
                    let id = container.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    let a = action_str.to_string();
                    tokio::spawn(async move {
                        let result = auth::container_action(&c, &id, &a).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::ContainerActionDone(result));
                    });
                }
            }
        }
        _ => {}
    }
}

/// Execute delete action for the selected item
fn execute_delete_action(
    app: &AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
    match app.screen {
        Screen::Missions => {
            if let Some(idx) = app.missions_selected {
                if let Some(mission) = app.missions.get(idx) {
                    let id = mission.id.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::delete_mission(&c, &id).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::MissionActionDone(result));
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
                        let result = auth::remove_container(&c, &id, false).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::ContainerActionDone(result));
                    });
                }
            }
        }
        Screen::Knowledge => {
            if let Some(idx) = app.knowledge_selected {
                if let Some(file) = app.knowledge_files.get(idx) {
                    let name = file.name.clone();
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::delete_knowledge_file(&c, &name).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::KnowledgeFileDeleted(result));
                    });
                }
            }
        }
        Screen::Media => {
            if let Some(idx) = app.media_selected {
                if let Some(item) = app.media_items.get(idx) {
                    let id = item.id;
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::delete_media(&c, id).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::MediaDeleted(result));
                    });
                }
            }
        }
        _ => {}
    }
}

/// Execute toggle action (enable/disable) for the selected item
fn execute_toggle_action(
    app: &AppState,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
    match app.screen {
        Screen::Skills => {
            if let Some(idx) = app.skills_selected {
                if let Some(skill) = app.skills.get(idx) {
                    let id = skill.id.clone();
                    let new_state = !skill.enabled;
                    let c = client.clone();
                    let t = tx.clone();
                    tokio::spawn(async move {
                        let result = auth::toggle_skill(&c, &id, new_state).await.map_err(|e| e.to_string());
                        let _ = t.send(AppEvent::SkillActionDone(result));
                    });
                }
            }
        }
        _ => {}
    }
}

/// Show a delete confirmation dialog for the current context
fn show_delete_confirmation(app: &mut AppState) {
    let confirm = match app.screen {
        Screen::Missions => app.missions_selected.map(|i| ConfirmAction::DeleteMission { index: i }),
        Screen::Containers => app.containers_selected.map(|i| ConfirmAction::DeleteContainer { index: i }),
        Screen::Knowledge => app.knowledge_selected.map(|i| ConfirmAction::DeleteKnowledge { index: i }),
        Screen::Media => app.media_selected.map(|i| ConfirmAction::DeleteMedia { index: i }),
        _ => None,
    };
    if let Some(c) = confirm {
        app.confirm_action = Some(c);
    }
}

/// Execute a previously confirmed destructive action
fn execute_confirmed_action(
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
                    let result = auth::delete_mission(&c, &id).await.map_err(|e| e.to_string());
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
                    let result = auth::remove_container(&c, &id, false).await.map_err(|e| e.to_string());
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
                    let result = auth::delete_knowledge_file(&c, &name).await.map_err(|e| e.to_string());
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

fn draw_confirm_dialog(f: &mut ratatui::Frame, app: &AppState, theme: &Theme) {
    use ratatui::layout::Alignment;
    use ratatui::style::{Modifier, Style};
    use ratatui::text::{Line, Span};
    use ratatui::widgets::{Block, Borders, Clear, Paragraph};

    let area = ui::utils::centered_rect(50, 20, f.area());
    f.render_widget(Clear, area);

    let action_text = match app.confirm_action {
        Some(ConfirmAction::DeleteMission { .. }) => "delete this mission",
        Some(ConfirmAction::DeleteContainer { .. }) => "remove this container",
        Some(ConfirmAction::DeleteKnowledge { .. }) => "delete this file",
        Some(ConfirmAction::DeleteMedia { .. }) => "delete this media item",
        Some(ConfirmAction::ClearChat) => "clear chat history",
        None => "",
    };

    let text = vec![
        Line::from(""),
        Line::from(Span::styled(
            format!(" ⚠️  Confirm: {}? ", action_text),
            Style::default().fg(theme.warning).add_modifier(Modifier::BOLD),
        )),
        Line::from(""),
        Line::from(vec![
            Span::styled("  y", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
            Span::styled(" = Confirm   ", Style::default().fg(theme.fg)),
            Span::styled("Any other key", Style::default().fg(theme.accent)),
            Span::styled(" = Cancel", Style::default().fg(theme.fg)),
        ]),
    ];

    let block = Block::default()
        .title(" ⚠ Confirm ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.warning))
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(text)
        .block(block)
        .alignment(Alignment::Center);
    f.render_widget(para, area);
}

// ── Nav bar drawing ───────────────────────────────────────────────────────────

fn draw_nav_bar(f: &mut ratatui::Frame, app: &AppState, theme: &Theme) {
    use ratatui::layout::{Alignment, Rect};
    use ratatui::style::{Modifier, Style};
    use ratatui::text::{Line, Span};
    use ratatui::widgets::{Block, Borders, Clear, Paragraph};

    let area = f.area();
    let nav_width = 24.min(area.width / 3);
    let nav_height = (Screen::nav_items().len() as u16 * 2) + 4;

    let nav_area = Rect {
        x: (area.width.saturating_sub(nav_width)) / 2,
        y: (area.height.saturating_sub(nav_height)) / 2,
        width: nav_width,
        height: nav_height,
    };

    f.render_widget(Clear, nav_area);

    let block = Block::default()
        .title(" Navigate ")
        .title_alignment(Alignment::Center)
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.accent))
        .style(Style::default().bg(theme.bg));

    let inner = block.inner(nav_area);
    f.render_widget(block, nav_area);

    let items: Vec<Line> = Screen::nav_items()
        .iter()
        .enumerate()
        .map(|(i, screen)| {
            let is_selected = i == app.nav_bar_index;
            let is_current = *screen == app.screen;
            let marker = if is_current { "● " } else { "  " };
            let arrow = if is_selected { "▸ " } else { "  " };
            let style = if is_selected {
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)
            } else if is_current {
                Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.fg)
            };
            let f_key = format!("F{}", i + 2);
            Line::from(vec![
                Span::styled(arrow, Style::default().fg(theme.accent)),
                Span::styled(marker, Style::default().fg(theme.accent)),
                Span::styled(format!("{:<10}", screen.title()), style),
                Span::styled(f_key, Style::default().fg(theme.accent_dim)),
            ])
        })
        .collect();

    let para = Paragraph::new(items);
    f.render_widget(para, inner);
}

// ── SSE / Chat session startup ────────────────────────────────────────────────

async fn start_chat_session(client: &ApiClient, tx: UnboundedSender<AppEvent>) {
    // Fetch history
    match auth::fetch_history(client).await {
        Ok(history) => {
            let _ = tx.send(AppEvent::HistoryLoaded(Ok(history)));
        }
        Err(e) => {
            let _ = tx.send(AppEvent::HistoryLoaded(Err(e.to_string())));
        }
    }

    // Start SSE stream
    let url = client.sse_url("/events");
    let c = client.client.clone();
    let origin = client.base_url.clone();
    let cookie = client.get_session_cookie();
    let (sse_tx, mut sse_rx) = mpsc::unbounded_channel::<sse::SseEvent>();

    tokio::spawn(async move {
        sse::connect_sse(c, url, origin, cookie, sse_tx).await;
    });

    let tx2 = tx.clone();
    tokio::spawn(async move {
        while let Some(ev) = sse_rx.recv().await {
            if tx2.send(AppEvent::Sse(ev)).is_err() {
                break;
            }
        }
    });
}

// ── Config helpers ────────────────────────────────────────────────────────────

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

// ── Utility ───────────────────────────────────────────────────────────────────

fn truncate_str(s: &str, max_len: usize) -> String {
    if s.len() <= max_len {
        s.to_string()
    } else {
        let mut end = max_len;
        while !s.is_char_boundary(end) && end > 0 {
            end -= 1;
        }
        format!("{}…", &s[..end])
    }
}
