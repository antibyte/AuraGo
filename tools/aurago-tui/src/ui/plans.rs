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

pub fn draw_plans(f: &mut Frame, app: &AppState, theme: &Theme) {
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

    draw_plans_header(f, app, theme, chunks[0]);
    draw_plans_content(f, app, theme, chunks[1]);
    draw_plans_status(f, app, theme, chunks[2]);

    if let Some(toast) = &app.toast {
        super::chat::draw_toast_simple(f, toast, theme);
    }
}

fn draw_plans_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let title = Span::styled(" 📋 Plans ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD));
    let count = Span::styled(format!(" ({} plans)", app.plans.len()), Style::default().fg(theme.accent_dim));
    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(Line::from(vec![title, count])).block(block);
    f.render_widget(para, area);
}

fn draw_plans_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.plans_loading {
        let loading = Paragraph::new("Loading plans...")
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

    draw_plans_list(f, app, theme, chunks[0]);
    draw_plans_detail(f, app, theme, chunks[1]);
}

fn draw_plans_list(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = if app.plans.is_empty() {
        vec![ListItem::new(Line::from("No plans found"))]
    } else {
        app.plans.iter().enumerate().map(|(i, p)| {
            let status_icon = plan_status_icon(&p.status);
            let is_selected = app.plans_selected == Some(i);
            let style = if is_selected {
                Style::default().bg(theme.accent).fg(theme.bg)
            } else {
                Style::default().fg(theme.fg)
            };
            let name = if p.name.len() > 24 {
                format!("{}...", &p.name[..21])
            } else {
                p.name.clone()
            };
            ListItem::new(Line::from(vec![
                Span::styled(format!("{} ", status_icon), style),
                Span::styled(name, style),
            ]))
        }).collect()
    };

    let block = Block::default()
        .title(" Plans ")
        .borders(Borders::RIGHT)
        .border_style(Style::default().fg(theme.border));
    let list = List::new(items).block(block);
    let mut state = ratatui::widgets::ListState::default();
    state.select(app.plans_selected);
    f.render_stateful_widget(list, area, &mut state);
}

fn draw_plans_detail(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let plan = app.plans_selected.and_then(|i| app.plans.get(i));

    let text = if let Some(p) = plan {
        let progress_bar = utils::make_progress_bar(p.progress, 30);
        let mut lines = vec![
            Line::from(vec![
                Span::styled("Name: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&p.name, Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)),
            ]),
            Line::from(vec![
                Span::styled("Status: ", Style::default().fg(theme.accent_dim)),
                Span::styled(format!("{} {}", plan_status_icon(&p.status), p.status), plan_status_color(&p.status, theme)),
            ]),
            Line::from(vec![
                Span::styled("Progress: ", Style::default().fg(theme.accent_dim)),
                Span::styled(format!("{:.0}%", p.progress * 100.0), Style::default().fg(theme.fg)),
                Span::styled(progress_bar, Style::default().fg(theme.accent_dim)),
            ]),
            Line::from(vec![
                Span::styled("Created: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&p.created_at, Style::default().fg(theme.fg)),
            ]),
            Line::from(vec![
                Span::styled("Updated: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&p.updated_at, Style::default().fg(theme.fg)),
            ]),
            Line::from(""),
            Line::from(Span::styled(
                format!("Tasks ({}):", p.tasks.len()),
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
            )),
        ];

        for task in &p.tasks {
            let icon = task_status_icon(&task.status);
            lines.push(Line::from(vec![
                Span::styled(format!("  {} ", icon), Style::default().fg(theme.fg)),
                Span::styled(&task.title, Style::default().fg(theme.fg)),
            ]));
            if !task.description.is_empty() {
                let desc = if task.description.len() > 80 {
                    format!("     {}...", &task.description[..77])
                } else {
                    format!("     {}", task.description)
                };
                lines.push(Line::from(Span::styled(desc, Style::default().fg(theme.accent_dim))));
            }
        }

        Text::from(lines)
    } else {
        Text::from(Line::from("Select a plan to view details"))
    };

    let block = Block::default()
        .title(" Detail ")
        .borders(Borders::NONE)
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_plans_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let left = format!("⚡ {} ", app.status_message);
    let right = " j/k: navigate │ Enter: advance │ d: archive │ r: refresh │ F1: nav │ ?: help ";
    let total = area.width as usize;
    let spacer = total.saturating_sub(left.len() + right.len());
    let text = format!("{}{}{}", left, " ".repeat(spacer), right);
    let para = Paragraph::new(text).style(Style::default().fg(theme.accent_dim));
    f.render_widget(para, area);
}

fn plan_status_icon(status: &str) -> &'static str {
    match status {
        "active" => "🟢",
        "completed" => "✅",
        "paused" => "⏸️",
        "blocked" => "🔴",
        "cancelled" => "❌",
        "draft" => "📝",
        _ => "📋",
    }
}

fn plan_status_color(status: &str, theme: &Theme) -> ratatui::style::Color {
    match status {
        "active" => theme.success,
        "completed" => theme.accent,
        "blocked" => theme.error,
        "paused" => theme.warning,
        _ => theme.fg,
    }
}

fn task_status_icon(status: &str) -> &'static str {
    match status {
        "done" | "completed" => "✅",
        "in_progress" | "running" => "🔄",
        "blocked" => "🔴",
        "pending" => "⬜",
        _ => "⬜",
    }
}

