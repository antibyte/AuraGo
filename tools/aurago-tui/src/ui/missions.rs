use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, List, ListItem, Paragraph, Wrap},
    Frame,
};

use crate::app::AppState;
use super::theme::Theme;

pub fn draw_missions(f: &mut Frame, app: &AppState, theme: &Theme) {
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

    draw_missions_header(f, app, theme, chunks[0]);
    draw_missions_content(f, app, theme, chunks[1]);
    draw_missions_status(f, app, theme, chunks[2]);

    if let Some(toast) = &app.toast {
        super::chat::draw_toast_simple(f, toast, theme);
    }
}

fn draw_missions_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let title = Span::styled(" 🚀 Missions ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD));
    let count = Span::styled(format!(" ({} missions)", app.missions.len()), Style::default().fg(theme.accent_dim));
    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(Line::from(vec![title, count])).block(block);
    f.render_widget(para, area);
}

fn draw_missions_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.missions_loading {
        let loading = Paragraph::new("Loading missions...")
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

    draw_missions_list(f, app, theme, chunks[0]);
    draw_missions_detail(f, app, theme, chunks[1]);
}

fn draw_missions_list(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = if app.missions.is_empty() {
        vec![ListItem::new(Line::from("No missions found"))]
    } else {
        app.missions.iter().enumerate().map(|(i, m)| {
            let is_selected = app.missions_selected == Some(i);
            let style = if is_selected {
                Style::default().bg(theme.accent).fg(theme.bg)
            } else {
                Style::default().fg(theme.fg)
            };
            let icon = mission_type_icon(&m.exec_type);
            let name = if m.name.len() > 22 {
                format!("{}...", &m.name[..19])
            } else {
                m.name.clone()
            };
            let status_dot = mission_status_dot(&m.status);
            ListItem::new(Line::from(vec![
                Span::styled(format!("{}{} ", icon, status_dot), style),
                Span::styled(name, style),
            ]))
        }).collect()
    };

    let block = Block::default()
        .title(" Missions ")
        .borders(Borders::RIGHT)
        .border_style(Style::default().fg(theme.border));
    let list = List::new(items).block(block);
    let mut state = ratatui::widgets::ListState::default();
    state.select(app.missions_selected);
    f.render_stateful_widget(list, area, &mut state);
}

fn draw_missions_detail(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let mission = app.missions_selected.and_then(|i| app.missions.get(i));

    let text = if let Some(m) = mission {
        let mut lines = vec![
            Line::from(vec![
                Span::styled("Name: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&m.name, Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)),
            ]),
            Line::from(vec![
                Span::styled("Status: ", Style::default().fg(theme.accent_dim)),
                Span::styled(
                    format!("{} {}", mission_status_dot(&m.status), m.status),
                    mission_status_color(&m.status, theme),
                ),
            ]),
            Line::from(vec![
                Span::styled("Type: ", Style::default().fg(theme.accent_dim)),
                Span::styled(format!("{} {}", mission_type_icon(&m.exec_type), m.exec_type), Style::default().fg(theme.accent)),
            ]),
            Line::from(vec![
                Span::styled("Priority: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&m.priority, priority_color(&m.priority, theme)),
            ]),
        ];

        if let Some(cron) = &m.cron_schedule {
            lines.push(Line::from(vec![
                Span::styled("Schedule: ", Style::default().fg(theme.accent_dim)),
                Span::styled(cron, Style::default().fg(theme.accent)),
            ]));
        }

        if let Some(last) = &m.last_run {
            lines.push(Line::from(vec![
                Span::styled("Last Run: ", Style::default().fg(theme.accent_dim)),
                Span::styled(last, Style::default().fg(theme.fg)),
            ]));
        }

        if let Some(next) = &m.next_run {
            lines.push(Line::from(vec![
                Span::styled("Next Run: ", Style::default().fg(theme.accent_dim)),
                Span::styled(next, Style::default().fg(theme.fg)),
            ]));
        }

        lines.push(Line::from(""));
        lines.push(Line::from(Span::styled(
            "Prompt:",
            Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
        )));

        // Wrap prompt text
        let prompt_lines: Vec<&str> = m.prompt.lines().take(20).collect();
        for line in prompt_lines {
            let truncated = if line.len() > 100 {
                format!("{}...", &line[..97])
            } else {
                line.to_string()
            };
            lines.push(Line::from(Span::styled(
                format!("  {}", truncated),
                Style::default().fg(theme.fg),
            )));
        }

        if m.locked {
            lines.push(Line::from(""));
            lines.push(Line::from(Span::styled(
                "🔒 Locked",
                Style::default().fg(theme.warning),
            )));
        }

        Text::from(lines)
    } else {
        Text::from(Line::from("Select a mission to view details"))
    };

    let block = Block::default()
        .title(" Detail ")
        .borders(Borders::NONE)
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_missions_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let left = format!("⚡ {} ", app.status_message);
    let right = " j/k: navigate │ Enter: run │ Del: delete │ r: refresh │ F1: nav │ ?: help ";
    let total = area.width as usize;
    let spacer = total.saturating_sub(left.len() + right.len());
    let text = format!("{}{}{}", left, " ".repeat(spacer), right);
    let para = Paragraph::new(text).style(Style::default().fg(theme.accent_dim));
    f.render_widget(para, area);
}

fn mission_type_icon(exec_type: &str) -> &'static str {
    match exec_type {
        "manual" => "👆",
        "scheduled" => "📅",
        "triggered" => "⚡",
        _ => "📋",
    }
}

fn mission_status_dot(status: &str) -> &'static str {
    match status {
        "running" | "active" => "🟢",
        "queued" => "🟡",
        "completed" | "done" => "✅",
        "failed" | "error" => "🔴",
        "idle" | "ready" => "⬜",
        "cancelled" => "❌",
        _ => "⬜",
    }
}

fn mission_status_color(status: &str, theme: &Theme) -> ratatui::style::Color {
    match status {
        "running" | "active" => theme.success,
        "completed" | "done" => theme.accent,
        "failed" | "error" => theme.error,
        "queued" => theme.warning,
        _ => theme.fg,
    }
}

fn priority_color(priority: &str, theme: &Theme) -> ratatui::style::Color {
    match priority {
        "high" => theme.error,
        "medium" => theme.warning,
        "low" => theme.success,
        _ => theme.fg,
    }
}
