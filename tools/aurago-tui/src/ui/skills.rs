use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, List, ListItem, Paragraph, Wrap},
    Frame,
};

use crate::app::AppState;
use super::theme::Theme;
use super::utils;

pub fn draw_skills(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = f.area();
    f.render_widget(
        Block::default().style(Style::default().bg(theme.bg).fg(theme.fg)),
        area,
    );

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3), // header
            Constraint::Min(0),    // content
            Constraint::Length(1), // status
        ])
        .split(area);

    draw_skills_header(f, app, theme, chunks[0]);
    draw_skills_content(f, app, theme, chunks[1]);
    draw_skills_status(f, app, theme, chunks[2]);

    if let Some(toast) = &app.toast {
        super::chat::draw_toast_simple(f, toast, theme);
    }
}

fn draw_skills_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let title = Span::styled(" 🧩 Skills ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD));
    let count = Span::styled(format!(" ({} skills)", app.skills.len()), Style::default().fg(theme.accent_dim));
    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(Line::from(vec![title, count])).block(block);
    f.render_widget(para, area);
}

fn draw_skills_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.skills_loading {
        let loading = Paragraph::new("Loading skills...")
            .style(Style::default().fg(theme.accent_dim))
            .alignment(ratatui::layout::Alignment::Center);
        f.render_widget(loading, area);
        return;
    }

    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Length(35), // list
            Constraint::Min(0),     // detail
        ])
        .split(area);

    draw_skills_list(f, app, theme, chunks[0]);
    draw_skills_detail(f, app, theme, chunks[1]);
}

fn draw_skills_list(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = if app.skills.is_empty() {
        vec![ListItem::new(Line::from("No skills found"))]
    } else {
        app.skills.iter().enumerate().map(|(i, s)| {
            let is_selected = app.skills_selected == Some(i);
            let style = if is_selected {
                Style::default().bg(theme.accent).fg(theme.bg)
            } else {
                Style::default().fg(theme.fg)
            };
            let sec_icon = security_icon(&s.security_status);
            let daemon_icon = if s.is_daemon {
                if s.daemon_running { "👹" } else { "💤" }
            } else {
                ""
            };
            let enabled_icon = if s.enabled { "✅" } else { "⏸️" };
            let name = if s.name.len() > 18 {
                format!("{}...", &s.name[..15])
            } else {
                s.name.clone()
            };
            ListItem::new(Line::from(vec![
                Span::styled(format!("{}{}{} ", sec_icon, daemon_icon, enabled_icon), style),
                Span::styled(name, style),
            ]))
        }).collect()
    };

    let block = Block::default()
        .title(" Skills ")
        .borders(Borders::RIGHT)
        .border_style(Style::default().fg(theme.border));
    let list = List::new(items).block(block);
    let mut state = ratatui::widgets::ListState::default();
    state.select(app.skills_selected);
    f.render_stateful_widget(list, area, &mut state);
}

fn draw_skills_detail(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let skill = app.skills_selected.and_then(|i| app.skills.get(i));

    let text = if let Some(s) = skill {
        let mut lines = vec![
            Line::from(vec![
                Span::styled("Name: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&s.name, Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)),
            ]),
            Line::from(vec![
                Span::styled("Source: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&s.source, Style::default().fg(theme.accent)),
            ]),
            Line::from(vec![
                Span::styled("Category: ", Style::default().fg(theme.accent_dim)),
                Span::styled(if s.category.is_empty() { "—".to_string() } else { s.category.clone() }, Style::default().fg(theme.fg)),
            ]),
            Line::from(vec![
                Span::styled("Security: ", Style::default().fg(theme.accent_dim)),
                Span::styled(
                    format!("{} {}", security_icon(&s.security_status), s.security_status),
                    security_color(&s.security_status, theme),
                ),
            ]),
            Line::from(vec![
                Span::styled("Enabled: ", Style::default().fg(theme.accent_dim)),
                Span::styled(
                    if s.enabled { "✅ Yes" } else { "⏸️ No" },
                    if s.enabled { Style::default().fg(theme.success) } else { Style::default().fg(theme.warning) },
                ),
            ]),
        ];

        if s.is_daemon {
            lines.push(Line::from(vec![
                Span::styled("Daemon: ", Style::default().fg(theme.accent_dim)),
                Span::styled(
                    if s.daemon_running { "👹 Running" } else { "💤 Stopped" },
                    if s.daemon_running { Style::default().fg(theme.success) } else { Style::default().fg(theme.accent_dim) },
                ),
            ]));
        }

        if !s.description.is_empty() {
            lines.push(Line::from(""));
            lines.push(Line::from(Span::styled(
                "Description:",
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
            )));
            let desc_lines: Vec<&str> = s.description.lines().take(15).collect();
            for line in desc_lines {
                let truncated = if line.len() > 100 {
                    format!("  {}...", &line[..97])
                } else {
                    format!("  {}", line)
                };
                lines.push(Line::from(Span::styled(truncated, Style::default().fg(theme.fg))));
            }
        }

        Text::from(lines)
    } else {
        Text::from(Line::from("Select a skill to view details"))
    };

    let block = Block::default()
        .title(" Detail ")
        .borders(Borders::NONE)
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_skills_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let left = format!("⚡ {} ", app.status_message);
    let right = " j/k: navigate │ Space: toggle │ r: refresh │ F1: nav │ ?: help ";
    let total = area.width as usize;
    let spacer = total.saturating_sub(left.len() + right.len());
    let text = format!("{}{}{}", left, " ".repeat(spacer), right);
    let para = Paragraph::new(text).style(Style::default().fg(theme.accent_dim));
    f.render_widget(para, area);
}

fn security_icon(status: &str) -> &'static str {
    match status {
        "clean" => "✅",
        "warning" => "⚠️",
        "dangerous" => "🔴",
        "pending" => "⏳",
        _ => "❓",
    }
}

fn security_color(status: &str, theme: &Theme) -> ratatui::style::Color {
    match status {
        "clean" => theme.success,
        "warning" => theme.warning,
        "dangerous" => theme.error,
        _ => theme.fg,
    }
}
