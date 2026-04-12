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
use tokio::time::{interval, Interval};

mod api;
mod app;
mod config;
mod events;
mod ui;

use api::{auth, sse, types::*, ApiClient};
use app::{AppState, Screen};
use events::keybindings::{map_key, Action};
use events::AppEvent;
use ui::theme::Theme;

#[derive(Parser, Debug)]
#[command(name = "aurago-tui")]
#[command(about = "AuraGo Terminal Chat Client")]
struct Args {
    #[arg(short, long, default_value = "http://localhost:8080")]
    url: String,
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
        ..AppState::default()
    }));

    let client = match ApiClient::new(&cfg.server_url) {
        Ok(c) => c,
        Err(e) => {
            restore_terminal(&mut terminal)?;
            eprintln!("Failed to create API client: {}", e);
            std::process::exit(1);
        }
    };

    let (event_tx, mut event_rx) = mpsc::unbounded_channel::<AppEvent>();
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
    let mut theme = Theme::default();

    loop {
        // Draw UI
        {
            let app_lock = app.lock().unwrap();
            let tick = app_lock.tick_counter;
            let current_theme = if app_lock.screen == Screen::Chat {
                Theme::from_mood(app_lock.personality.mood.as_deref().unwrap_or("neutral"))
            } else {
                theme.clone()
            };
            terminal
                .draw(|f| match app_lock.screen {
                    Screen::Splash => ui::splash::draw_splash(f, &current_theme, tick),
                    Screen::Login => ui::login::draw_login(f, &app_lock, &current_theme),
                    Screen::Chat => ui::chat::draw_chat(f, &app_lock, &current_theme),
                })
                .context("Failed to draw UI")?;
        }

        // Wait for next event
        let event = tokio::select! {
            Some(Ok(ev)) = reader.next() => AppEvent::Crossterm(ev),
            _ = tick.tick() => AppEvent::Tick,
            Some(ev) = event_rx.recv() => ev,
        };

        let mut app_lock = app.lock().unwrap();

        match event {
            AppEvent::Crossterm(crossterm::event::Event::Key(key)) => {
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
                    continue;
                }

                let action = map_key(key, app_lock.focus_sidebar);
                match action {
                    Action::Quit => break,
                    Action::ToggleHelp => app_lock.show_help = !app_lock.show_help,
                    Action::ToggleTheme => {
                        // Cycle theme for fun
                    }
                    Action::ToggleSidebar => {
                        if app_lock.screen == Screen::Login && app_lock.totp_enabled {
                            app_lock.login_focus_otp = !app_lock.login_focus_otp;
                        } else {
                            app_lock.focus_sidebar = !app_lock.focus_sidebar;
                        }
                    }
                    Action::ClearChat => {
                        let c = client.clone();
                        tokio::spawn(async move {
                            let _ = auth::clear_history(&c).await;
                        });
                        app_lock.chat_messages.clear();
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
                        app_lock.authenticated = false;
                        app_lock.screen = Screen::Login;
                        if let Some(h) = sse_handle.take() {
                            h.abort();
                        }
                    }
                    Action::SendMessage => {
                        if app_lock.screen == Screen::Login {
                            let password = app_lock.login_password.clone();
                            let totp = app_lock.login_totp.clone();
                            app_lock.login_loading = true;
                            app_lock.login_error = None;
                            let c = client.clone();
                            let tx = event_tx.clone();
                            tokio::spawn(async move {
                                match auth::login(&c, &password, &totp).await {
                                    Ok(_) => {
                                        let _ = tx.send(AppEvent::LoginResult(Ok(())));
                                    }
                                    Err(e) => {
                                        let _ = tx.send(AppEvent::LoginResult(Err(e.to_string())));
                                    }
                                }
                            });
                        } else if app_lock.screen == Screen::Chat && !app_lock.chat_input.trim().is_empty() {
                            let text = app_lock.chat_input.trim().to_string();
                            app_lock.chat_input.clear();
                            app_lock.push_user_message(text.clone());
                            app_lock.start_assistant_stream();

                            let c = client.clone();
                            let tx = event_tx.clone();
                            let messages: Vec<ChatMessage> = app_lock
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
                                match c.request::<ChatCompletionRequest, serde_json::Value>(Method::POST, "/v1/chat/completions", Some(&req)).await {
                                    Ok(_) => {
                                        let _ = tx.send(AppEvent::ChatSent);
                                    }
                                    Err(e) => {
                                        let _ = tx.send(AppEvent::ChatError(e.to_string()));
                                    }
                                }
                            });
                        }
                    }
                    Action::NewLine => {
                        if app_lock.screen == Screen::Chat {
                            app_lock.chat_input.push('\n');
                        }
                    }
                    Action::ScrollUp => {
                        if app_lock.scroll > 0 {
                            app_lock.scroll -= 1;
                        }
                    }
                    Action::ScrollDown => {
                        let max = app_lock.chat_messages.len().saturating_sub(1);
                        if app_lock.scroll < max {
                            app_lock.scroll += 1;
                        }
                    }
                    Action::ScrollTop => app_lock.scroll = 0,
                    Action::ScrollBottom => app_lock.scroll_to_bottom(),
                    Action::Backspace => {
                        if app_lock.screen == Screen::Login {
                            if app_lock.login_focus_otp {
                                app_lock.login_totp.pop();
                            } else {
                                app_lock.login_password.pop();
                            }
                        } else {
                            app_lock.chat_input.pop();
                        }
                    }
                    Action::DeleteChar => {}
                    Action::CursorLeft => {}
                    Action::CursorRight => {}
                    Action::CursorStart => {}
                    Action::CursorEnd => {}
                    Action::Type(c) => {
                        if app_lock.screen == Screen::Login {
                            if app_lock.login_focus_otp {
                                if c.is_ascii_digit() && app_lock.login_totp.len() < 6 {
                                    app_lock.login_totp.push(c);
                                }
                            } else {
                                app_lock.login_password.push(c);
                            }
                        } else {
                            app_lock.chat_input.push(c);
                        }
                    }
                    Action::None => {}
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
                        app_lock.screen = Screen::Chat;
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
                            app_lock.screen = Screen::Chat;
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
        }
    }

    Ok(())
}

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
    let (sse_tx, mut sse_rx) = mpsc::unbounded_channel::<sse::SseEvent>();

    tokio::spawn(async move {
        sse::connect_sse(c, url, sse_tx).await;
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
