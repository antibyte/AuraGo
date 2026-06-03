use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Margin, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, Paragraph, Scrollbar, ScrollbarOrientation, Wrap},
};

use super::overlays::{draw_help, draw_session_drawer};
use super::theme::{Theme, spinner_frame};
use crate::app::AppState;
use crate::i18n;

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
        .title(i18n::current().history_title)
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

    // Wave 3 viewport optimization (P1): only build visible tail of history + buffer.
    // This avoids O(N) full rebuild + lines.clone() + Paragraph on thousands of messages every frame.
    // Scrollbar uses a cheap full-logical estimate (pre-wrap) so thumb stays accurate.
    // See plan.md and IMPROVEMENTS.md. Preserves all UX (wrap, streaming, auto MAX, unicode, indicator).
    let mut lines: Vec<Line> = Vec::new();
    let total_msgs = app.chat_messages.len();
    let area_h = inner.height as usize;
    let est_lines_per_msg = 3usize; // avg (content lines + sep); long tool output will under-estimate but buffer helps
    let buffer_msgs = 6;
    let visible_msgs_est = (area_h / est_lines_per_msg).max(3) + buffer_msgs;

    // Wave B / F2: scroll-position aware viewport (builds on tail-only from 1-4).
    // For auto/bottom: use tail (shows latest, MAX clamped by Paragraph).
    // Otherwise: walk cumulative logical lines from app.scroll to find window start, compute rel offset for para.
    let auto_or_bottom = app.auto_scroll || app.scroll == usize::MAX;
    let (start_idx, para_scroll) = if auto_or_bottom || total_msgs <= visible_msgs_est {
        (total_msgs.saturating_sub(visible_msgs_est), usize::MAX)
    } else {
        let target = app.scroll;
        let mut cum = 0usize;
        let mut found_idx = total_msgs.saturating_sub(1);
        for (i, m) in app.chat_messages.iter().enumerate() {
            cum += m.content.lines().count() + 1;
            if cum > target {
                found_idx = i;
                break;
            }
        }
        let start = found_idx.saturating_sub(buffer_msgs);
        // rel offset within the window's logical lines
        let mut cum_before = 0usize;
        for m in &app.chat_messages[..start] {
            cum_before += m.content.lines().count() + 1;
        }
        let rel = target.saturating_sub(cum_before);
        (start, rel)
    };

    for msg in &app.chat_messages[start_idx..] {
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
        if msg.image_url.is_some() {
            lines.push(Line::from(vec![
                Span::styled("   ", Style::default()),
                Span::styled("🖼 [Image attached]", Style::default().fg(theme.accent_dim).add_modifier(Modifier::ITALIC)),
            ]));
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
                i18n::current().new_messages_hint,
                Style::default().fg(theme.accent_dim),
            ),
        ]));
    }

    // Full logical line count (pre-wrap, cheap) for scrollbar accuracy even when viewport-sliced.
    // Uses cached_line_count (F6 polish) instead of recomputing lines() every draw.
    let full_logical: usize = app
        .chat_messages
        .iter()
        .map(|m| m.cached_line_count + 1)
        .sum::<usize>()
        + if app.thinking_active { 1 } else { 0 }
        + if !app.auto_scroll && !app.chat_messages.is_empty() { 1 } else { 0 };

    let para = Paragraph::new(Text::from(lines.clone()))
        .scroll((para_scroll as u16, 0))
        .wrap(Wrap { trim: true });
    f.render_widget(para, inner);

    // Scrollbar uses full_logical (pre-wrap estimate across *all* history) so thumb/position correct
    // even though we only built viewport lines (Wave 3). The Paragraph scroll still uses the (small) built lines.
    let mut scrollbar_state =
        ratatui::widgets::ScrollbarState::new(full_logical)
            .position(app.scroll.min(full_logical));
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
        .title(if app.attaching_image {
            " Attach Image Path "
        } else if app.attached_image_url.is_some() {
            " Message [🖼 Image attached] "
        } else {
            i18n::current().message_input_title
        })
        .borders(Borders::ALL)
        .border_style(Style::default().fg(border_color));

    // For attach mode, simple display (cursor at end)
    let (display_text, show_cursor) = if app.attaching_image {
        (format!("Image path: {}", app.image_path_input), false)
    } else if app.attached_image_url.is_some() {
        (format!("{} [🖼]", app.chat_input), true)
    } else {
        (app.chat_input.clone(), true)
    };

    let mut spans = vec![];
    if show_cursor {
        // Split text at cursor and show a blinking cursor
        // Convert character cursor to byte offset for split_at to avoid panic with non-ASCII
        let char_count = display_text.chars().count();
        let cursor_char = app.chat_input_cursor.min(char_count);
        let byte_idx = display_text
            .char_indices()
            .nth(cursor_char)
            .map(|(i, _)| i)
            .unwrap_or_else(|| display_text.len());

        let cursor_visible = app.tick_counter % 4 < 2;
        let cursor_str = if cursor_visible { "▎" } else { " " };
        let (before, after) = display_text.split_at(byte_idx);

        spans.push(Span::styled(before, Style::default().fg(theme.fg)));
        spans.push(Span::styled(cursor_str, Style::default().fg(theme.accent)));
        if !after.is_empty() {
            spans.push(Span::styled(after, Style::default().fg(theme.fg)));
        }
    } else {
        spans.push(Span::styled(display_text, Style::default().fg(theme.fg)));
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


