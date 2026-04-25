use ratatui::{
    layout::{Constraint, Direction, Layout, Margin, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, Clear, Paragraph, Scrollbar, ScrollbarOrientation, Wrap},
    Frame,
};

use crate::app::AppState;
use super::theme::{spinner_frame, Theme};
use super::utils;

pub fn draw_chat(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = f.area();
    f.render_widget(
        Block::default().style(Style::default().bg(theme.bg).fg(theme.fg)),
        area,
    );

    let main_chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3), // header
            Constraint::Min(0),    // chat
            Constraint::Length(3), // input
            Constraint::Length(1), // status
        ])
        .split(area);

    draw_header(f, app, theme, main_chunks[0]);

    let body_chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Length(20), Constraint::Min(0)])
        .split(main_chunks[1]);

    draw_sidebar(f, app, theme, body_chunks[0]);
    draw_messages(f, app, theme, body_chunks[1]);
    draw_input(f, app, theme, main_chunks[2]);
    draw_status(f, app, theme, main_chunks[3]);

    if app.session_drawer_open {
        draw_session_drawer(f, app, theme);
    }

    if app.show_help {
        draw_help(f, theme);
    }

    if let Some(toast) = &app.toast {
        draw_toast(f, toast, theme, app.toast_anim, app.toast_ticks);
    }
}

fn draw_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let wave = wave_line(area.width as usize, app.tick_counter);
    let mood = app.personality.mood.as_deref().unwrap_or("Neutral");
    let emotion = app.personality.current_emotion.as_deref().unwrap_or("calm");
    let right = format!("🌙 Mood: {} | {} ", mood, emotion);

    let left_len = wave.len();
    let right_len = right.len();
    let total = area.width as usize;
    let spacer = total.saturating_sub(left_len + right_len);
    let header_text = format!("{}{}{}", wave, " ".repeat(spacer), right);

    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border))
        .style(Style::default().bg(theme.bg));

    let para = Paragraph::new(header_text)
        .block(block)
        .style(Style::default().fg(theme.accent).add_modifier(Modifier::BOLD));
    f.render_widget(para, area);
}

fn wave_line(width: usize, tick: u64) -> String {
    let mut s = String::with_capacity(width);
    for x in 0..width.min(40) {
        let y = ((x as f32 + tick as f32 * 0.2).sin() * 1.5 + 2.0) as i32;
        let ch = match y {
            0 => "▁",
            1 => "▃",
            2 => "▅",
            3 => "▇",
            _ => "█",
        };
        s.push_str(ch);
    }
    s
}

fn draw_sidebar(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let block = Block::default()
        .title(" History ")
        .borders(Borders::RIGHT)
        .border_style(if app.focus_sidebar {
            Style::default().fg(theme.border_focus)
        } else {
            Style::default().fg(theme.border)
        });
    let text = Text::from(vec![Line::from("Current Chat").style(
        if app.focus_sidebar && app.sidebar_index == 0 {
            Style::default().bg(theme.accent).fg(theme.bg)
        } else {
            Style::default().fg(theme.fg)
        },
    )]);
    let para = Paragraph::new(text).block(block);
    f.render_widget(para, area);
}

fn draw_messages(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let block = Block::default()
        .borders(Borders::NONE)
        .style(Style::default().bg(theme.bg));
    let inner = block.inner(area);
    f.render_widget(block, area);

    let mut lines: Vec<Line> = Vec::new();
    for msg in &app.chat_messages {
        let (prefix, color) = match msg.role.as_str() {
            "user" => ("🧑 ", theme.user_msg),
            "assistant" => ("🤖 ", theme.assistant_msg),
            "tool" => ("🔧 ", theme.tool_msg),
            _ => ("💬 ", theme.system_msg),
        };
        let prefix_span = Span::styled(prefix, Style::default().fg(color).add_modifier(Modifier::BOLD));
        for (i, line_text) in msg.content.lines().enumerate() {
            if i == 0 {
                lines.push(Line::from(vec![
                    prefix_span.clone(),
                    Span::styled(line_text, Style::default().fg(theme.fg)),
                ]));
            } else {
                lines.push(Line::from(vec![
                    Span::styled("   ", Style::default()),
                    Span::styled(line_text, Style::default().fg(theme.fg)),
                ]));
            }
        }
        if msg.is_streaming {
            let cursor = spinner_frame(app.tick_counter).to_string();
            lines.push(Line::from(vec![
                Span::styled("   ", Style::default()),
                Span::styled(cursor, Style::default().fg(theme.accent)),
            ]));
        }
        lines.push(Line::from(""));
    }

    if app.thinking_active {
        lines.push(Line::from(vec![
            Span::styled("💭 ", Style::default().fg(theme.accent)),
            Span::styled(
                format!("Thinking... {}", spinner_frame(app.tick_counter)),
                Style::default().fg(theme.accent_dim),
            ),
        ]));
    }

    // Show "new messages" indicator when not auto-scrolling
    if !app.auto_scroll && !app.chat_messages.is_empty() {
        lines.push(Line::from(vec![
            Span::styled("  ↓ ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
            Span::styled("New messages — press Ctrl+G to scroll down", Style::default().fg(theme.accent_dim)),
        ]));
    }

    let para = Paragraph::new(Text::from(lines))
        .scroll((app.scroll as u16, 0))
        .wrap(Wrap { trim: true });
    f.render_widget(para, inner);

    let scrollbar = Scrollbar::default()
        .orientation(ScrollbarOrientation::VerticalRight)
        .begin_symbol(Some("↑"))
        .end_symbol(Some("↓"))
        .track_symbol(Some("│"));
    let mut scrollbar_state = ratatui::widgets::ScrollbarState::new(app.chat_messages.len().saturating_mul(3))
        .position(app.scroll.min(app.chat_messages.len().saturating_mul(3)));
    f.render_stateful_widget(
        scrollbar,
        inner.inner(Margin { horizontal: 0, vertical: 0 }),
        &mut scrollbar_state,
    );
}

fn draw_input(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let glow = theme.glow_color(app.tick_counter);
    let border_color = if app.focus_sidebar {
        theme.border
    } else {
        glow
    };
    let block = Block::default()
        .title(" Message ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(border_color));

    // Split text at cursor and show a blinking cursor
    // Convert character cursor to byte offset for split_at to avoid panic with non-ASCII
    let char_count = app.chat_input.chars().count();
    let cursor_char = app.chat_input_cursor.min(char_count);
    let byte_idx = app.chat_input.char_indices()
        .nth(cursor_char)
        .map(|(i, _)| i)
        .unwrap_or_else(|| app.chat_input.len());

    let cursor_visible = app.tick_counter % 4 < 2;
    let cursor_str = if cursor_visible { "▎" } else { " " };
    let (before, after) = app.chat_input.split_at(byte_idx);

    let mut spans = vec![
        Span::styled(before, Style::default().fg(theme.fg)),
        Span::styled(cursor_str, Style::default().fg(theme.accent)),
    ];
    if !after.is_empty() {
        spans.push(Span::styled(after, Style::default().fg(theme.fg)));
    }

    let para = Paragraph::new(Line::from(spans))
        .block(block)
        .style(Style::default().bg(Color::Black).fg(theme.fg));
    f.render_widget(para, area);
}

fn draw_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let left = format!("⚡ {} ", app.status_message);
    let right = format!(
        "Tokens: {} │ Status: {} ",
        app.tokens.total, app.agent_status
    );
    let total = area.width as usize;
    let spacer = total.saturating_sub(left.len() + right.len());
    let text = format!("{}{}{}", left, " ".repeat(spacer), right);
    let para = Paragraph::new(text).style(Style::default().fg(theme.accent_dim));
    f.render_widget(para, area);
}

/// Draw session drawer as an overlay panel on the right side
fn draw_session_drawer(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = f.area();
    let drawer_width = 36.min(area.width / 2);

    let drawer_area = Rect {
        x: area.width.saturating_sub(drawer_width),
        y: 0,
        width: drawer_width,
        height: area.height,
    };

    f.render_widget(Clear, drawer_area);

    let block = Block::default()
        .title(" 💬 Sessions ")
        .borders(Borders::LEFT | Borders::TOP | Borders::BOTTOM)
        .border_style(Style::default().fg(theme.accent))
        .style(Style::default().bg(theme.bg));
    let inner = block.inner(drawer_area);
    f.render_widget(block, drawer_area);

    // Split into list + footer
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Min(0),    // session list
            Constraint::Length(2), // footer hints
        ])
        .split(inner);

    // Session list
    let items: Vec<Line> = if app.sessions.is_empty() {
        vec![Line::from(Span::styled(
            "  No sessions yet",
            Style::default().fg(theme.accent_dim),
        ))]
    } else {
        app.sessions
            .iter()
            .enumerate()
            .map(|(i, s)| {
                let is_active = s.id == app.active_session_id;
                let is_highlighted = i == app.session_drawer_index;
                let marker = if is_active { "● " } else if is_highlighted { "▸ " } else { "  " };
                let name = if s.name.is_empty() {
                    format!("Session {}", &s.id[..8.min(s.id.len())])
                } else {
                    s.name.clone()
                };
                let count = format!(" ({})", s.message_count);
                let style = if is_highlighted {
                    Style::default().bg(theme.accent).fg(theme.bg).add_modifier(Modifier::BOLD)
                } else if is_active {
                    Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)
                } else {
                    Style::default().fg(theme.fg)
                };
                Line::from(vec![
                    Span::styled(marker, Style::default().fg(theme.accent)),
                    Span::styled(utils::truncate_str(&name, 20), style),
                    Span::styled(count, Style::default().fg(theme.accent_dim)),
                ])
            })
            .collect()
    };

    let para = Paragraph::new(Text::from(items));
    f.render_widget(para, chunks[0]);

    // Footer hints
    let hints = Line::from(vec![
        Span::styled(" j/k", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
        Span::styled(" Navigate  ", Style::default().fg(theme.accent_dim)),
        Span::styled("n", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
        Span::styled(" New  ", Style::default().fg(theme.accent_dim)),
        Span::styled("d", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
        Span::styled(" Del  ", Style::default().fg(theme.accent_dim)),
        Span::styled("Esc", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
        Span::styled(" Close", Style::default().fg(theme.accent_dim)),
    ]);
    let hint_para = Paragraph::new(hints)
        .style(Style::default().bg(theme.bg));
    f.render_widget(hint_para, chunks[1]);
}

fn draw_help(f: &mut Frame, theme: &Theme) {
    let area = utils::centered_rect(60, 60, f.area());
    f.render_widget(Clear, area);
    let block = Block::default()
        .title(" Help ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border_focus));
    let text = Text::from(vec![
        Line::from(""),
        Line::from(Span::styled("── Chat ──────────────────────────", Style::default().fg(theme.accent))),
        Line::from("Enter          Send message"),
        Line::from("Shift+Enter    New line"),
        Line::from("↑ / ↓          Scroll messages"),
        Line::from("Ctrl+L         Clear chat"),
        Line::from("Ctrl+G         Scroll to latest"),
        Line::from("Ctrl+S         Session drawer"),
        Line::from("Tab            Focus sidebar"),
        Line::from(""),
        Line::from(Span::styled("── Navigation ────────────────────", Style::default().fg(theme.accent))),
        Line::from("F1             Open nav bar"),
        Line::from("F2             Chat"),
        Line::from("F3             Dashboard"),
        Line::from("F4             Plans"),
        Line::from("F5             Missions"),
        Line::from("F6             Skills"),
        Line::from("F7             Containers"),
        Line::from("Ctrl+N         Open nav bar"),
        Line::from("Ctrl+O         Logout"),
        Line::from(""),
        Line::from(Span::styled("── List pages ────────────────────", Style::default().fg(theme.accent))),
        Line::from("j / ↓          Move down"),
        Line::from("k / ↑          Move up"),
        Line::from("Enter          Select / detail"),
        Line::from("Esc            Back to list"),
        Line::from("Space          Toggle enabled"),
        Line::from("Delete         Delete item"),
        Line::from("r              Refresh"),
        Line::from(""),
        Line::from(Span::styled("── General ───────────────────────", Style::default().fg(theme.accent))),
        Line::from("Esc / ?        Close help"),
        Line::from("Ctrl+T         Toggle theme"),
        Line::from("Ctrl+C         Quit"),
    ]);
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

pub fn draw_toast(f: &mut Frame, toast: &str, theme: &Theme, anim: u16, _max_ticks: u16) {
    let area = f.area();
    let toast_width = (area.width as usize * 70 / 100).max(40);
    let lines_needed = toast.lines().count().max(1);
    let wrapped_lines = (toast.len() / toast_width.saturating_sub(4)).max(0) + lines_needed;
    let height = (wrapped_lines + 4).min(area.height as usize).max(5) as u16;

    let toast_area = utils::centered_rect(70, (height * 100) / area.height.max(1), area);
    f.render_widget(Clear, toast_area);

    let is_success = toast.starts_with('✓');
    let border_color = if is_success { theme.success } else { theme.warning };
    let text_color = if is_success { theme.success } else { theme.warning };

    // Animate: pulse the border brightness
    let pulse = if anim < 5 { 0.5 + (anim as f32 / 5.0) * 0.5 } else { 1.0 };
    let _ = pulse;

    let block = Block::default()
        .title(" Notification ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(border_color))
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(toast)
        .block(block)
        .alignment(ratatui::layout::Alignment::Center)
        .wrap(Wrap { trim: true })
        .style(Style::default().fg(text_color).add_modifier(Modifier::BOLD));
    f.render_widget(para, toast_area);
}

/// Convenience wrapper for callers that don't track animation state
pub fn draw_toast_simple(f: &mut Frame, toast: &str, theme: &Theme) {
    draw_toast(f, toast, theme, 10, 10);
}
