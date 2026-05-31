//! Overlay / modal drawing functions (help, session drawer, toasts, confirm, nav bar).
//! Extracted during post-audit polish (2026-05) for better separation of concerns.

use ratatui::{
    Frame,
    layout::{Alignment, Constraint, Direction, Layout, Margin, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, Clear, Paragraph, Wrap},
};

use crate::app::{AppState, ConfirmAction};
use crate::i18n;
use crate::ui::theme::Theme;
use crate::ui::utils::{self, truncate_str};

/// Draw session drawer as an overlay panel on the right side
pub fn draw_session_drawer(f: &mut Frame, app: &AppState, theme: &Theme) {
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
        .title(i18n::current().sessions_title)
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
                let marker = if is_active {
                    "● "
                } else if is_highlighted {
                    "▸ "
                } else {
                    "  "
                };
                let name = if s.name.is_empty() {
                    format!("Session {}", truncate_str(&s.id, 8))
                } else {
                    s.name.clone()
                };
                let count = format!(" ({})", s.message_count);
                let style = if is_highlighted {
                    Style::default()
                        .bg(theme.accent)
                        .fg(theme.bg)
                        .add_modifier(Modifier::BOLD)
                } else if is_active {
                    Style::default()
                        .fg(theme.accent)
                        .add_modifier(Modifier::BOLD)
                } else {
                    Style::default().fg(theme.fg)
                };
                Line::from(vec![
                    Span::styled(marker, Style::default().fg(theme.accent)),
                    Span::styled(truncate_str(&name, 20), style),
                    Span::styled(count, Style::default().fg(theme.accent_dim)),
                ])
            })
            .collect()
    };

    let para = Paragraph::new(Text::from(items));
    f.render_widget(para, chunks[0]);

    // Footer hints
    let hints = Line::from(vec![
        Span::styled(
            " j/k",
            Style::default()
                .fg(theme.accent)
                .add_modifier(Modifier::BOLD),
        ),
        Span::styled(" Navigate  ", Style::default().fg(theme.accent_dim)),
        Span::styled(
            "n",
            Style::default()
                .fg(theme.accent)
                .add_modifier(Modifier::BOLD),
        ),
        Span::styled(" New  ", Style::default().fg(theme.accent_dim)),
        Span::styled(
            "d",
            Style::default()
                .fg(theme.accent)
                .add_modifier(Modifier::BOLD),
        ),
        Span::styled(" Del  ", Style::default().fg(theme.accent_dim)),
        Span::styled(
            "Esc",
            Style::default()
                .fg(theme.accent)
                .add_modifier(Modifier::BOLD),
        ),
        Span::styled(" Close", Style::default().fg(theme.accent_dim)),
    ]);
    let hint_para = Paragraph::new(hints).style(Style::default().bg(theme.bg));
    f.render_widget(hint_para, chunks[1]);
}

pub fn draw_help(f: &mut Frame, theme: &Theme) {
    let area = utils::centered_rect(60, 60, f.area());
    f.render_widget(Clear, area);
    let block = Block::default()
        .title(i18n::current().help_title)
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border_focus));
    let text = Text::from(vec![
        Line::from(""),
        Line::from(Span::styled(
            "── Chat ──────────────────────────",
            Style::default().fg(theme.accent),
        )),
        Line::from("Enter          Send message"),
        Line::from("Shift+Enter    New line"),
        Line::from("↑ / ↓          Scroll messages"),
        Line::from("Ctrl+L         Clear chat"),
        Line::from("Ctrl+G         Scroll to latest"),
        Line::from("Ctrl+S         Session drawer"),
        Line::from("Tab            Focus sidebar"),
        Line::from(""),
        Line::from(Span::styled(
            "── Navigation ────────────────────",
            Style::default().fg(theme.accent),
        )),
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
        Line::from(Span::styled(
            "── List pages ────────────────────",
            Style::default().fg(theme.accent),
        )),
        Line::from("j / ↓          Move down"),
        Line::from("k / ↑          Move up"),
        Line::from("Enter          Select / detail"),
        Line::from("Esc            Back to list"),
        Line::from("Space          Toggle enabled"),
        Line::from("Delete         Delete item"),
        Line::from("r              Refresh"),
        Line::from(""),
        Line::from(Span::styled(
            "── General ───────────────────────",
            Style::default().fg(theme.accent),
        )),
        Line::from("Esc / ?        Close help"),
        Line::from("Ctrl+T         Toggle theme"),
        Line::from("Ctrl+C         Quit"),
    ]);
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

pub fn draw_toast(f: &mut Frame, toast: &str, theme: &Theme, _anim: u16, _max_ticks: u16) {
    let area = f.area();
    let toast_width = (area.width as usize * 70 / 100).max(40);
    let lines_needed = toast.lines().count().max(1);
    let wrapped_lines = (toast.len() / toast_width.saturating_sub(4)).max(0) + lines_needed;
    let height = (wrapped_lines + 4).min(area.height as usize).max(5) as u16;

    let toast_area = utils::centered_rect(70, (height * 100) / area.height.max(1), area);
    f.render_widget(Clear, toast_area);

    let is_success = toast.starts_with('✓');
    let border_color = if is_success {
        theme.success
    } else {
        theme.warning
    };
    let text_color = if is_success {
        theme.success
    } else {
        theme.warning
    };

    let block = Block::default()
        .title(i18n::current().notification_title)
        .borders(Borders::ALL)
        .border_style(Style::default().fg(border_color))
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(toast)
        .block(block)
        .alignment(Alignment::Center)
        .wrap(Wrap { trim: true })
        .style(Style::default().fg(text_color).add_modifier(Modifier::BOLD));
    f.render_widget(para, toast_area);
}

/// Convenience wrapper for callers that don't track animation state
pub fn draw_toast_simple(f: &mut Frame, toast: &str, theme: &Theme) {
    draw_toast(f, toast, theme, 10, 10);
}

pub fn draw_confirm_dialog(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = utils::centered_rect(50, 20, f.area());
    f.render_widget(Clear, area);

    let action_text = match app.confirm_action {
        Some(ConfirmAction::DeleteMission { .. }) => "delete this mission",
        Some(ConfirmAction::DeleteContainer { .. }) => "remove this container",
        Some(ConfirmAction::DeleteKnowledge { .. }) => "delete this file",
        Some(ConfirmAction::DeleteMedia { .. }) => "delete this media item",
        Some(ConfirmAction::ClearChat) => "clear chat history",
        None => "",
    };

    let text = vec![
        Line::from(""),
        Line::from(Span::styled(
            format!(" ⚠️  Confirm: {}? ", action_text),
            Style::default()
                .fg(theme.warning)
                .add_modifier(Modifier::BOLD),
        )),
        Line::from(""),
        Line::from(vec![
            Span::styled(
                "  y",
                Style::default()
                    .fg(theme.accent)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::styled(" = Confirm   ", Style::default().fg(theme.fg)),
            Span::styled("Any other key", Style::default().fg(theme.accent)),
            Span::styled(" = Cancel", Style::default().fg(theme.fg)),
        ]),
    ];

    let block = Block::default()
        .title(" ⚠ Confirm ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.warning))
        .style(Style::default().bg(theme.bg));
    let para = Paragraph::new(text)
        .block(block)
        .alignment(Alignment::Center);
    f.render_widget(para, area);
}

pub fn draw_nav_bar(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = f.area();
    let nav_width = 24.min(area.width / 3);
    let nav_height = (crate::app::Screen::nav_items().len() as u16 * 2) + 4;

    let nav_area = Rect {
        x: (area.width.saturating_sub(nav_width)) / 2,
        y: (area.height.saturating_sub(nav_height)) / 2,
        width: nav_width,
        height: nav_height,
    };

    f.render_widget(Clear, nav_area);

    let block = Block::default()
        .title(i18n::current().navigate_title)
        .title_alignment(Alignment::Center)
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.accent))
        .style(Style::default().bg(theme.bg));

    let inner = block.inner(nav_area);
    f.render_widget(block, nav_area);

    let items: Vec<Line> = crate::app::Screen::nav_items()
        .iter()
        .enumerate()
        .map(|(i, screen)| {
            let is_selected = i == app.nav_bar_index;
            let is_current = *screen == app.screen;
            let marker = if is_current { "● " } else { "  " };
            let arrow = if is_selected { "▸ " } else { "  " };
            let style = if is_selected {
                Style::default()
                    .fg(theme.accent)
                    .add_modifier(Modifier::BOLD)
            } else if is_current {
                Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.fg)
            };
            let f_key = format!("F{}", i + 2);
            Line::from(vec![
                Span::styled(arrow, Style::default().fg(theme.accent)),
                Span::styled(marker, Style::default().fg(theme.accent)),
                Span::styled(format!("{:<10}", screen.title()), style),
                Span::styled(f_key, Style::default().fg(theme.accent_dim)),
            ])
        })
        .collect();

    let para = Paragraph::new(items);
    f.render_widget(para, inner);
}