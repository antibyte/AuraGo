use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Margin, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, Clear, Paragraph, Scrollbar, ScrollbarOrientation, Wrap},
};

use super::overlays::{draw_help, draw_session_drawer};
use super::theme::{Theme, spinner_frame};
use super::utils;
use crate::app::AppState;

// Re-export toast helpers from the new overlays module so other UI modules
// that previously did `super::chat::draw_toast_simple(...)` continue to work
// during the transition. New code should import from `ui::overlays` directly.
pub use super::overlays::{draw_toast, draw_toast_simple};

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

    let para = Paragraph::new(header_text).block(block).style(
        Style::default()
            .fg(theme.accent)
            .add_modifier(Modifier::BOLD),
    );
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
        let prefix_span = Span::styled(
            prefix,
            Style::default().fg(color).add_modifier(Modifier::BOLD),
        );
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
            Span::styled(
                "  ↓ ",
                Style::default()
                    .fg(theme.accent)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::styled(
                "New messages — press Ctrl+G to scroll down",
                Style::default().fg(theme.accent_dim),
            ),
        ]));
    }

    let para = Paragraph::new(Text::from(lines.clone()))
        .scroll((app.scroll as u16, 0))
        .wrap(Wrap { trim: true });
    f.render_widget(para, inner);

    // Use the *actual* number of rendered lines for the scrollbar (was previously a wrong message*3 hack)
    let total_lines = lines.len();
    let mut scrollbar_state =
        ratatui::widgets::ScrollbarState::new(total_lines)
            .position(app.scroll.min(total_lines));
    let scrollbar = Scrollbar::default()
        .orientation(ScrollbarOrientation::VerticalRight)
        .begin_symbol(Some("↑"))
        .end_symbol(Some("↓"))
        .track_symbol(Some("│"));
    f.render_stateful_widget(
        scrollbar,
        inner.inner(Margin {
            horizontal: 0,
            vertical: 0,
        }),
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
    let byte_idx = app
        .chat_input
        .char_indices()
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
        .style(Style::default().bg(theme.bg).fg(theme.fg));
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


