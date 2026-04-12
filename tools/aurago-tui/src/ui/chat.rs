use ratatui::{
    layout::{Constraint, Direction, Layout, Margin, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, Clear, Paragraph, Scrollbar, ScrollbarOrientation, Wrap},
    Frame,
};

use crate::app::{AppState, ChatMessage};
use super::theme::{spinner_frame, Theme};

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

    if app.show_help {
        draw_help(f, theme);
    }

    if let Some(toast) = &app.toast {
        draw_toast(f, toast, theme);
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
            let cursor = format!("{}", spinner_frame(app.tick_counter));
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

    let para = Paragraph::new(Text::from(lines))
        .scroll((app.scroll as u16, 0))
        .wrap(Wrap { trim: true });
    f.render_widget(para, inner);

    let scrollbar = Scrollbar::default()
        .orientation(ScrollbarOrientation::VerticalRight)
        .begin_symbol(None)
        .end_symbol(None);
    let mut scrollbar_state = ratatui::widgets::ScrollbarState::new(app.chat_messages.len())
        .position(app.scroll);
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
    let para = Paragraph::new(app.chat_input.as_str())
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

fn draw_help(f: &mut Frame, theme: &Theme) {
    let area = centered_rect(60, 50, f.area());
    f.render_widget(Clear, area);
    let block = Block::default()
        .title(" Help ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border_focus));
    let text = Text::from(vec![
        Line::from("Enter        Send message"),
        Line::from("Shift+Enter  New line"),
        Line::from("↑ / ↓        Scroll history"),
        Line::from("Ctrl+L       Clear chat"),
        Line::from("Ctrl+O       Logout"),
        Line::from("Ctrl+R       Scroll to latest"),
        Line::from("Ctrl+T       Toggle theme"),
        Line::from("Tab          Focus sidebar"),
        Line::from("Esc / ?      Close help"),
        Line::from("Ctrl+C / q   Quit"),
    ]);
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_toast(f: &mut Frame, toast: &str, theme: &Theme) {
    let area = centered_rect(70, 10, f.area());
    f.render_widget(Clear, area);
    let block = Block::default()
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.warning))
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(toast)
        .block(block)
        .alignment(ratatui::layout::Alignment::Center)
        .wrap(Wrap { trim: true })
        .style(Style::default().fg(theme.warning).add_modifier(Modifier::BOLD));
    f.render_widget(para, area);
}

fn centered_rect(percent_x: u16, percent_y: u16, r: Rect) -> Rect {
    let popup_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Percentage((100 - percent_y) / 2),
            Constraint::Percentage(percent_y),
            Constraint::Percentage((100 - percent_y) / 2),
        ])
        .split(r);

    Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage((100 - percent_x) / 2),
            Constraint::Percentage(percent_x),
            Constraint::Percentage((100 - percent_x) / 2),
        ])
        .split(popup_layout[1])[1]
}
