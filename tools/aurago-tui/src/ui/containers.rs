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

pub fn draw_containers(f: &mut Frame, app: &AppState, theme: &Theme) {
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

    draw_containers_header(f, app, theme, chunks[0]);
    draw_containers_content(f, app, theme, chunks[1]);
    draw_containers_status(f, app, theme, chunks[2]);

    if let Some(toast) = &app.toast {
        super::chat::draw_toast_simple(f, toast, theme);
    }
}

fn draw_containers_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let running = app.containers.iter().filter(|c| c.state == "running").count();
    let stopped = app.containers.len().saturating_sub(running);
    let title = Span::styled(" 🐳 Containers ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD));
    let stats = Span::styled(
        format!(" ({} total │ {} running │ {} stopped)", app.containers.len(), running, stopped),
        Style::default().fg(theme.accent_dim),
    );
    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(Line::from(vec![title, stats])).block(block);
    f.render_widget(para, area);
}

fn draw_containers_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.containers_loading {
        let loading = Paragraph::new("Loading containers...")
            .style(Style::default().fg(theme.accent_dim))
            .alignment(ratatui::layout::Alignment::Center);
        f.render_widget(loading, area);
        return;
    }

    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Length(40), // list
            Constraint::Min(0),     // detail
        ])
        .split(area);

    draw_containers_list(f, app, theme, chunks[0]);
    draw_containers_detail(f, app, theme, chunks[1]);
}

fn draw_containers_list(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = if app.containers.is_empty() {
        vec![ListItem::new(Line::from("No containers found"))]
    } else {
        app.containers.iter().enumerate().map(|(i, c)| {
            let is_selected = app.containers_selected == Some(i);
            let style = if is_selected {
                Style::default().bg(theme.accent).fg(theme.bg)
            } else {
                Style::default().fg(theme.fg)
            };
            let state_icon = container_state_icon(&c.state);
            let name = if c.name.len() > 24 {
                format!("{}...", &c.name[..21])
            } else {
                c.name.clone()
            };
            ListItem::new(Line::from(vec![
                Span::styled(format!("{} ", state_icon), style),
                Span::styled(name, style),
            ]))
        }).collect()
    };

    let block = Block::default()
        .title(" Containers ")
        .borders(Borders::RIGHT)
        .border_style(Style::default().fg(theme.border));
    let list = List::new(items).block(block);
    let mut state = ratatui::widgets::ListState::default();
    state.select(app.containers_selected);
    f.render_stateful_widget(list, area, &mut state);
}

fn draw_containers_detail(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let container = app.containers_selected.and_then(|i| app.containers.get(i));

    let text = if let Some(c) = container {
        let mut lines = vec![
            Line::from(vec![
                Span::styled("Name: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&c.name, Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)),
            ]),
            Line::from(vec![
                Span::styled("Image: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&c.image, Style::default().fg(theme.accent)),
            ]),
            Line::from(vec![
                Span::styled("State: ", Style::default().fg(theme.accent_dim)),
                Span::styled(
                    format!("{} {}", container_state_icon(&c.state), c.state),
                    container_state_color(&c.state, theme),
                ),
            ]),
            Line::from(vec![
                Span::styled("Status: ", Style::default().fg(theme.accent_dim)),
                Span::styled(&c.status, Style::default().fg(theme.fg)),
            ]),
        ];

        if !c.ports.is_empty() {
            lines.push(Line::from(vec![
                Span::styled("Ports: ", Style::default().fg(theme.accent_dim)),
                Span::styled(c.ports.join(", "), Style::default().fg(theme.accent)),
            ]));
        }

        lines.push(Line::from(vec![
            Span::styled("ID: ", Style::default().fg(theme.accent_dim)),
            Span::styled(if c.id.len() > 12 { &c.id[..12] } else { &c.id }, Style::default().fg(theme.accent_dim)),
        ]));

        Text::from(lines)
    } else {
        Text::from(Line::from("Select a container to view details"))
    };

    let block = Block::default()
        .title(" Detail ")
        .borders(Borders::NONE)
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_containers_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let left = format!("⚡ {} ", app.status_message);
    let right = " j/k: navigate │ Enter: start/stop │ Del: remove │ r: refresh │ F1: nav │ ?: help ";
    let total = area.width as usize;
    let spacer = total.saturating_sub(left.len() + right.len());
    let text = format!("{}{}{}", left, " ".repeat(spacer), right);
    let para = Paragraph::new(text).style(Style::default().fg(theme.accent_dim));
    f.render_widget(para, area);
}

fn container_state_icon(state: &str) -> &'static str {
    match state {
        "running" => "🟢",
        "paused" => "🟡",
        "exited" | "stopped" | "dead" => "🔴",
        "created" => "⬜",
        "restarting" => "🔄",
        _ => "❓",
    }
}

fn container_state_color(state: &str, theme: &Theme) -> ratatui::style::Color {
    match state {
        "running" => theme.success,
        "paused" => theme.warning,
        "exited" | "stopped" | "dead" => theme.error,
        _ => theme.fg,
    }
}
