use std::io;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use anyhow::{Context, Result};
use clap::Parser;
use crossterm::{
    event::{DisableMouseCapture, EnableMouseCapture, EventStream},
    execute,
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use futures::StreamExt;
use ratatui::{Terminal, backend::CrosstermBackend};

use tokio::sync::mpsc::{self, UnboundedReceiver, UnboundedSender};
use tokio::time::interval;

mod actions;
mod api;
mod app;
mod config;
mod events;
mod i18n;
mod ui;

use api::{ApiClient, auth, sse};
use app::{AppState, DashTab, MediaTab, Screen};
use events::AppEvent;
use events::keybindings::{KeyContext, map_key};
use ui::theme::Theme;
use ui::utils::truncate_str;

use actions::execute_confirmed_action;
use ui::overlays::{draw_confirm_dialog, draw_nav_bar};

#[derive(Parser, Debug)]
#[command(name = "aurago-tui")]
#[command(about = "AuraGo Terminal Chat Client")]
struct Args {
    #[arg(short, long)]
    url: Option<String>,
    #[arg(long, help = "Skip TLS certificate verification (insecure)")]
    insecure: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();
    let mut cfg = config::Config::load().unwrap_or_default();
    if let Some(url) = args.url {
        cfg.server_url = url;
        cfg.save().ok();
    }

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
    // These calls are safe to repeat; ignore secondary errors so we always
    // try to put the terminal back into a usable state even on early exit / panic paths.
    let _ = disable_raw_mode();
    let _ = execute!(
        terminal.backend_mut(),
        LeaveAlternateScreen,
        DisableMouseCapture
    );
    let _ = terminal.show_cursor();
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
            let mut app_lock = app.lock().expect("AppState mutex poisoned (impossible in single-threaded TUI unless a prior panic occurred)");
            let tick_val = app_lock.tick_counter;

            // Auto-scroll: use a very large scroll offset; Paragraph clamps it to the real bottom.
            // This guarantees correct bottom alignment regardless of how many visual lines each message has.
            if app_lock.auto_scroll && app_lock.screen == Screen::Chat {
                app_lock.scroll = usize::MAX;
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
                        Screen::Dashboard => {
                            ui::dashboard::draw_dashboard(f, &app_lock, &current_theme)
                        }
                        Screen::Plans => ui::plans::draw_plans(f, &app_lock, &current_theme),
                        Screen::Missions => {
                            ui::missions::draw_missions(f, &app_lock, &current_theme)
                        }
                        Screen::Skills => ui::skills::draw_skills(f, &app_lock, &current_theme),
                        Screen::Containers => {
                            ui::containers::draw_containers(f, &app_lock, &current_theme)
                        }
                        Screen::Config => ui::config::draw_config(f, &app_lock, &current_theme),
                        Screen::Knowledge => {
                            ui::knowledge::draw_knowledge(f, &app_lock, &current_theme)
                        }
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

        // Handle terminal resize immediately so the next draw uses correct dimensions
        if let AppEvent::Crossterm(crossterm::event::Event::Resize(_, _)) = &event {
            let _ = terminal.autoresize();
            // Force a redraw on next loop iteration (we will fall through to draw)
        }

        let mut app_lock = app.lock().expect("AppState mutex poisoned (impossible in single-threaded TUI unless a prior panic occurred)");

        match event {
            AppEvent::Crossterm(crossterm::event::Event::Key(key)) => {
                handle_key_event(key, &mut app_lock, &client, &event_tx, &mut sse_handle);
            }
            AppEvent::Crossterm(crossterm::event::Event::Mouse(mouse)) => {
                match mouse.kind {
                    crossterm::event::MouseEventKind::ScrollUp => {
                        if app_lock.screen == Screen::Chat {
                            app_lock.auto_scroll = false;
                            app_lock.scroll = app_lock.scroll.saturating_sub(3);
                        } else if app_lock.screen == Screen::Dashboard {
                            // Could scroll logs in future
                        }
                    }
                    crossterm::event::MouseEventKind::ScrollDown => {
                        if app_lock.screen == Screen::Chat {
                            // Manual scroll down: keep auto-scroll off.
                            // User can press Ctrl+G (ScrollBottom) or use the action to re-stick to bottom.
                            app_lock.auto_scroll = false;
                            app_lock.scroll = app_lock.scroll.saturating_add(3);
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
                // Finish any in-flight streaming assistant message so we don't leave a dangling spinner
                app_lock.finish_stream();
                // Add a visible error line in the chat
                app_lock.chat_messages.push(crate::app::ChatMessage {
                    role: "system".to_string(),
                    content: format!("⚠️  {}", err),
                    is_streaming: false,
                    is_tool: false,
                    is_thinking: false,
                });
                app_lock.scroll_to_bottom();
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
                        start_new_chat_session(&mut sse_handle, &client, &event_tx);
                    }
                    Err(e) => {
                        app_lock.login_error = Some(e);
                    }
                }
            }
            AppEvent::AuthCheckResult(result) => match result {
                Ok((enabled, password_set, totp_enabled)) => {
                    app_lock.auth_enabled = enabled;
                    app_lock.totp_enabled = totp_enabled;
                    if !enabled || !password_set {
                        app_lock.authenticated = true;
                        app_lock.navigate_to(Screen::Chat);
                        start_new_chat_session(&mut sse_handle, &client, &event_tx);
                    } else {
                        app_lock.authenticated = false;
                        app_lock.screen = Screen::Login;
                    }
                }
                Err(e) => {
                    app_lock.login_error = Some(format!("Auth check failed: {}", e));
                    app_lock.screen = Screen::Login;
                }
            },
            AppEvent::HistoryLoaded(result) => match result {
                Ok(history) => {
                    app_lock.load_history(history);
                    app_lock.status_message = "Connected".to_string();
                }
                Err(e) => {
                    app_lock.status_message = format!("History error: {}", e);
                }
            },

            // ── Sessions ─────────────────────────────────────────────────────
            AppEvent::SessionsLoaded(result) => match result {
                Ok(sessions) => {
                    app_lock.sessions = sessions;
                }
                Err(e) => {
                    app_lock.toast = Some(format!("Failed to load sessions: {}", e));
                    app_lock.toast_ticks = 10;
                }
            },
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
                        let h = tokio::spawn(async move {
                            let result = auth::fetch_history_for_session(&c, &sid)
                                .await
                                .map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::HistoryLoaded(result));
                        });
                        app_lock.spawn_tracked(h);
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
                        let h = tokio::spawn(async move {
                            let result = auth::fetch_sessions(&c).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::SessionsLoaded(result));
                        });
                        app_lock.spawn_tracked(h);
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
            AppEvent::DashboardBudgetLoaded(result) => match result {
                Ok(info) => app_lock.dash_budget = info,
                Err(e) => app_lock.status_message = format!("Budget error: {}", e),
            },
            AppEvent::DashboardOverviewLoaded(result) => match result {
                Ok(info) => app_lock.dash_overview = info,
                Err(e) => app_lock.status_message = format!("Overview error: {}", e),
            },
            AppEvent::DashboardPersonalityLoaded(result) => match result {
                Ok(info) => app_lock.dash_personality = info,
                Err(e) => app_lock.status_message = format!("Personality error: {}", e),
            },
            AppEvent::DashboardLogsLoaded(result) => match result {
                Ok(logs) => app_lock.dash_logs = logs,
                Err(e) => app_lock.status_message = format!("Logs error: {}", e),
            },
            AppEvent::DashboardActivityLoaded(result) => match result {
                Ok(activity) => app_lock.dash_activity = activity,
                Err(e) => app_lock.status_message = format!("Activity error: {}", e),
            },

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
                        let h = tokio::spawn(async move {
                            let result =
                                auth::fetch_plans(&c, &sid).await.map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::PlansLoaded(result));
                        });
                        app_lock.spawn_tracked(h);
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
            AppEvent::MissionActionDone(result) => match result {
                Ok(()) => {
                    app_lock.status_message = "Mission action completed".to_string();
                    let c = client.clone();
                    let tx = event_tx.clone();
                    let h = tokio::spawn(async move {
                        let result = auth::fetch_missions(&c).await.map_err(|e| e.to_string());
                        let _ = tx.send(AppEvent::MissionsLoaded(result));
                    });
                    app_lock.spawn_tracked(h);
                }
                Err(e) => {
                    app_lock.toast = Some(format!("Mission action failed: {}", e));
                    app_lock.toast_ticks = 10;
                }
            },

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
            AppEvent::SkillActionDone(result) => match result {
                Ok(()) => {
                    app_lock.status_message = "Skill action completed".to_string();
                    let c = client.clone();
                    let tx = event_tx.clone();
                    let h = tokio::spawn(async move {
                        let result = auth::fetch_skills(&c).await.map_err(|e| e.to_string());
                        let _ = tx.send(AppEvent::SkillsLoaded(result));
                    });
                    app_lock.spawn_tracked(h);
                }
                Err(e) => {
                    app_lock.toast = Some(format!("Skill action failed: {}", e));
                    app_lock.toast_ticks = 10;
                }
            },

            // ── Containers ───────────────────────────────────────────────────
            AppEvent::ContainersLoaded(result) => {
                match result {
                    Ok(containers) => {
                        app_lock.containers = containers;
                        if app_lock.containers_selected.is_none() && !app_lock.containers.is_empty()
                        {
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
            AppEvent::ContainerActionDone(result) => match result {
                Ok(_) => {
                    app_lock.status_message = "Container action completed".to_string();
                    let c = client.clone();
                    let tx = event_tx.clone();
                    let h = tokio::spawn(async move {
                        let result = auth::fetch_containers(&c).await.map_err(|e| e.to_string());
                        let _ = tx.send(AppEvent::ContainersLoaded(result));
                    });
                    app_lock.spawn_tracked(h);
                }
                Err(e) => {
                    app_lock.toast = Some(format!("Container action failed: {}", e));
                    app_lock.toast_ticks = 10;
                }
            },
            AppEvent::ContainerLogsLoaded(result) => {
                match result {
                    Ok(val) => {
                        // Display container logs as a toast or in a dedicated area
                        if let Some(logs) = val.as_str() {
                            app_lock.toast =
                                Some(format!("Container logs:\n{}", truncate_str(logs, 500)));
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
            AppEvent::ConfigSchemaLoaded(result) => match result {
                Ok(schema) => {
                    app_lock.config_schema = schema;
                }
                Err(e) => {
                    app_lock.toast = Some(format!("Failed to load config schema: {}", e));
                    app_lock.toast_ticks = 10;
                }
            },
            AppEvent::ConfigSaved(result) => match result {
                Ok(()) => {
                    app_lock.config_dirty = false;
                    app_lock.toast = Some("✓ Configuration saved".to_string());
                    app_lock.toast_ticks = 8;
                }
                Err(e) => {
                    app_lock.toast = Some(format!("Failed to save config: {}", e));
                    app_lock.toast_ticks = 10;
                }
            },
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
                        if app_lock.knowledge_selected.is_none()
                            && !app_lock.knowledge_files.is_empty()
                        {
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
                        let h = tokio::spawn(async move {
                            let result = auth::fetch_knowledge_files(&c)
                                .await
                                .map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::KnowledgeFilesLoaded(result));
                        });
                        app_lock.spawn_tracked(h);
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
                        }
                        .to_string();
                        let offset = app_lock.media_offset;
                        let query = if app_lock.media_search.is_empty() {
                            None
                        } else {
                            Some(app_lock.media_search.clone())
                        };
                        let h = tokio::spawn(async move {
                            let result =
                                auth::fetch_media(&c, &media_type, 50, offset, query.as_deref())
                                    .await
                                    .map_err(|e| e.to_string());
                            let _ = tx.send(AppEvent::MediaLoaded(result));
                        });
                        app_lock.spawn_tracked(h);
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
            start_new_chat_session(sse_handle, client, event_tx);
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
        if key.code == crossterm::event::KeyCode::Char('y')
            || key.code == crossterm::event::KeyCode::Char('Y')
        {
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
    actions::dispatch_action(action, app_lock, client, event_tx, sse_handle);
}




/// Starts a chat session: fetches history and opens an SSE connection.
/// Returns a `JoinHandle` for the SSE connection task (the reconnect loop).
/// The caller should abort this handle before starting a new session.
pub fn start_chat_session(
    client: ApiClient,
    tx: UnboundedSender<AppEvent>,
) -> tokio::task::JoinHandle<()> {
    tokio::spawn(async move {
        match auth::fetch_history(&client).await {
            Ok(history) => {
                let _ = tx.send(AppEvent::HistoryLoaded(Ok(history)));
            }
            Err(e) => {
                let _ = tx.send(AppEvent::HistoryLoaded(Err(e.to_string())));
            }
        }

        let url = client.sse_url("/events");
        let c = client.client.clone();
        let origin = client.base_url.clone();
        let cookie = client.get_session_cookie();
        let (sse_tx, mut sse_rx) = mpsc::unbounded_channel::<sse::SseEvent>();

        let tx2 = tx.clone();
        tokio::spawn(async move {
            while let Some(ev) = sse_rx.recv().await {
                if tx2.send(AppEvent::Sse(ev)).is_err() {
                    break;
                }
            }
        });

        sse::connect_sse(c, url, origin, cookie, sse_tx).await;
    })
}

fn start_new_chat_session(
    sse_handle: &mut Option<tokio::task::JoinHandle<()>>,
    client: &ApiClient,
    tx: &UnboundedSender<AppEvent>,
) {
    if let Some(handle) = sse_handle.take() {
        handle.abort();
    }
    *sse_handle = Some(start_chat_session(client.clone(), tx.clone()));
}

// ── Config helpers ────────────────────────────────────────────────────────────


