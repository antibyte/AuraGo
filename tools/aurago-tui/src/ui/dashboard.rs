use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Borders, List, ListItem, Paragraph, Wrap},
    Frame,
};

use crate::app::{AppState, DashTab};
use super::theme::Theme;

pub fn draw_dashboard(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = f.area();
    f.render_widget(
        Block::default().style(Style::default().bg(theme.bg).fg(theme.fg)),
        area,
    );

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3), // header + tabs
            Constraint::Min(0),    // content
            Constraint::Length(1), // status
        ])
        .split(area);

    draw_dash_header(f, app, theme, chunks[0]);
    draw_dash_content(f, app, theme, chunks[1]);
    draw_dash_status(f, app, theme, chunks[2]);

    if let Some(toast) = &app.toast {
        super::chat::draw_toast(f, toast, theme);
    }
}

fn draw_dash_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let tabs = ["Overview", "Agent", "System", "Logs"];
    let tab_idx = match app.dash_tab {
        DashTab::Overview => 0,
        DashTab::Agent => 1,
        DashTab::System => 2,
        DashTab::Logs => 3,
    };

    let mut spans: Vec<Span> = Vec::new();
    for (i, tab) in tabs.iter().enumerate() {
        let style = if i == tab_idx {
            Style::default().fg(theme.accent).add_modifier(Modifier::BOLD | Modifier::UNDERLINED)
        } else {
            Style::default().fg(theme.fg)
        };
        spans.push(Span::styled(format!(" {} ", tab), style));
        if i < tabs.len() - 1 {
            spans.push(Span::styled(" │ ", Style::default().fg(theme.border)));
        }
    }

    let title = Span::styled(" 📊 Dashboard ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD));
    let mut header_line = vec![title];
    header_line.push(Span::styled("  ", Style::default()));
    header_line.append(&mut spans);

    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(Line::from(header_line)).block(block);
    f.render_widget(para, area);
}

fn draw_dash_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.dash_loading {
        let loading = Paragraph::new("Loading...")
            .style(Style::default().fg(theme.accent_dim))
            .alignment(ratatui::layout::Alignment::Center);
        f.render_widget(loading, area);
        return;
    }

    match app.dash_tab {
        DashTab::Overview => draw_overview_tab(f, app, theme, area),
        DashTab::Agent => draw_agent_tab(f, app, theme, area),
        DashTab::System => draw_system_tab(f, app, theme, area),
        DashTab::Logs => draw_logs_tab(f, app, theme, area),
    }
}

fn draw_overview_tab(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(8),  // overview cards
            Constraint::Length(6),  // budget
            Constraint::Min(0),    // activity
        ])
        .split(area);

    // Overview card
    let ov = &app.dash_overview;
    let overview_text = Text::from(vec![
        Line::from(vec![
            Span::styled("🤖 Agent: ", Style::default().fg(theme.fg)),
            Span::styled(&ov.agent_status, Style::default().fg(theme.success).add_modifier(Modifier::BOLD)),
        ]),
        Line::from(vec![
            Span::styled("🧠 Model: ", Style::default().fg(theme.fg)),
            Span::styled(&ov.model, Style::default().fg(theme.accent)),
            Span::styled(" via ", Style::default().fg(theme.fg)),
            Span::styled(&ov.provider, Style::default().fg(theme.accent)),
        ]),
        Line::from(vec![
            Span::styled("📐 Context: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{:.0}%", ov.context_percent), context_color(ov.context_percent, theme)),
            draw_gauge(ov.context_percent, 20, theme),
        ]),
        Line::from(vec![
            Span::styled("🔌 Integrations: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{}", ov.integrations), Style::default().fg(theme.accent)),
            Span::styled("  🔧 Tools: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{}", ov.tools_count), Style::default().fg(theme.accent)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("Tokens: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{}", app.tokens.total), Style::default().fg(theme.accent)),
            Span::styled(" (session: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{}", app.tokens.session_total), Style::default().fg(theme.accent_dim)),
            Span::styled(" global: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{}", app.tokens.global_total), Style::default().fg(theme.accent_dim)),
            Span::styled(")", Style::default().fg(theme.fg)),
        ]),
    ]);
    let overview_block = Block::default()
        .title(" Overview ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let overview_para = Paragraph::new(overview_text).block(overview_block).wrap(Wrap { trim: true });
    f.render_widget(overview_para, chunks[0]);

    // Budget card
    let b = &app.dash_budget;
    let budget_text = if b.enabled {
        let cost = b.spent_usd;
        let limit = b.daily_limit_usd;
        let pct = if limit > 0.0 { cost / limit * 100.0 } else { 0.0 };
        Text::from(vec![
            Line::from(vec![
                Span::styled("💰 Spent: ", Style::default().fg(theme.fg)),
                Span::styled(format!("${:.3}", cost), Style::default().fg(theme.warning)),
                Span::styled(" / ", Style::default().fg(theme.fg)),
                Span::styled(format!("${:.2}", limit), Style::default().fg(theme.accent)),
                Span::styled(format!(" ({:.0}%)", pct), budget_color(pct, theme)),
            ]),
            Line::from(vec![
                Span::styled("Enforcement: ", Style::default().fg(theme.fg)),
                Span::styled(&b.enforcement, Style::default().fg(theme.accent_dim)),
            ]),
        ])
    } else {
        Text::from(Line::from("Budget tracking disabled"))
    };
    let budget_block = Block::default()
        .title(" Budget ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let budget_para = Paragraph::new(budget_text).block(budget_block).wrap(Wrap { trim: true });
    f.render_widget(budget_para, chunks[1]);

    // Activity / Cron
    let items: Vec<ListItem> = if app.dash_activity.is_empty() {
        vec![ListItem::new(Line::from("No scheduled tasks"))]
    } else {
        app.dash_activity.iter().map(|c| {
            let status_icon = if c.enabled { "✅" } else { "⏸️" };
            ListItem::new(Line::from(vec![
                Span::styled(format!("{} ", status_icon), Style::default().fg(theme.fg)),
                Span::styled(&c.expression, Style::default().fg(theme.accent)),
                Span::styled(format!(" – {}", if c.prompt.len() > 60 { format!("{}...", &c.prompt[..57]) } else { c.prompt.clone() }), Style::default().fg(theme.fg)),
            ]))
        }).collect()
    };
    let activity_block = Block::default()
        .title(" Scheduled Tasks ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let activity_list = List::new(items).block(activity_block);
    f.render_widget(activity_list, chunks[2]);
}

fn draw_agent_tab(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let p = &app.dash_personality;
    let traits_text: Vec<Line> = if p.traits.is_empty() {
        vec![Line::from("No personality data")]
    } else {
        let mut traits: Vec<_> = p.traits.iter().collect();
        traits.sort_by(|a, b| a.0.cmp(b.0));
        traits.iter().map(|(k, v)| {
            let bar_len = (*v * 20.0) as usize;
            let bar: String = "█".repeat(bar_len);
            let empty: String = "░".repeat(20 - bar_len);
            Line::from(vec![
                Span::styled(format!("{:>15} ", k), Style::default().fg(theme.fg)),
                Span::styled(bar, Style::default().fg(theme.accent)),
                Span::styled(empty, Style::default().fg(theme.accent_dim)),
                Span::styled(format!(" {:.0}%", *v * 100.0), Style::default().fg(theme.fg)),
            ])
        }).collect()
    };

    let text = Text::from(vec![
        Line::from(vec![
            Span::styled("🌙 Mood: ", Style::default().fg(theme.fg)),
            Span::styled(&p.mood, Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
        ]),
        Line::from(vec![
            Span::styled("💭 Emotion: ", Style::default().fg(theme.fg)),
            Span::styled(&p.emotion, Style::default().fg(theme.accent)),
        ]),
        Line::from(""),
    ].into_iter().chain(traits_text).collect::<Vec<_>>());

    let block = Block::default()
        .title(" Personality ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_system_tab(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let s = &app.dash_system;
    let uptime_str = format_uptime(s.uptime_seconds);

    let text = Text::from(vec![
        Line::from(vec![
            Span::styled("🖥️  CPU: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{:.1}%", s.cpu_percent), gauge_color(s.cpu_percent, theme)),
            draw_gauge(s.cpu_percent, 30, theme),
        ]),
        Line::from(vec![
            Span::styled("💾 RAM: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{:.1}%", s.memory_percent), gauge_color(s.memory_percent, theme)),
            draw_gauge(s.memory_percent, 30, theme),
        ]),
        Line::from(vec![
            Span::styled("💿 Disk: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{:.1}%", s.disk_percent), gauge_color(s.disk_percent, theme)),
            draw_gauge(s.disk_percent, 30, theme),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("📡 Net ↑: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{:.1} MB", s.network_sent_mb), Style::default().fg(theme.accent)),
            Span::styled("  ↓: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{:.1} MB", s.network_recv_mb), Style::default().fg(theme.accent)),
        ]),
        Line::from(vec![
            Span::styled("👥 SSE Clients: ", Style::default().fg(theme.fg)),
            Span::styled(format!("{}", s.sse_clients), Style::default().fg(theme.accent)),
        ]),
        Line::from(vec![
            Span::styled("⏱️  Uptime: ", Style::default().fg(theme.fg)),
            Span::styled(uptime_str, Style::default().fg(theme.success)),
        ]),
    ]);

    let block = Block::default()
        .title(" System ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let para = Paragraph::new(text).block(block).wrap(Wrap { trim: true });
    f.render_widget(para, area);
}

fn draw_logs_tab(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = if app.dash_logs.is_empty() {
        vec![ListItem::new(Line::from("No logs available"))]
    } else {
        app.dash_logs.iter().map(|log| {
            let level_color = match log.level.to_lowercase().as_str() {
                "error" | "err" => theme.error,
                "warn" | "warning" => theme.warning,
                "info" => theme.success,
                "debug" => theme.accent_dim,
                _ => theme.fg,
            };
            let msg = if log.message.len() > 120 {
                format!("{}...", &log.message[..117])
            } else {
                log.message.clone()
            };
            ListItem::new(Line::from(vec![
                Span::styled(format!("{} ", log.time), Style::default().fg(theme.accent_dim)),
                Span::styled(format!("{:>5} ", log.level), Style::default().fg(level_color).add_modifier(Modifier::BOLD)),
                Span::styled(msg, Style::default().fg(theme.fg)),
            ]))
        }).collect()
    };

    let block = Block::default()
        .title(" Live Logs ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let list = List::new(items).block(block);
    f.render_widget(list, area);
}

fn draw_dash_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let left = format!("⚡ {} ", app.status_message);
    let right = " h/l: tabs │ j/k: scroll │ r: refresh │ F1: nav │ ?: help ";
    let total = area.width as usize;
    let spacer = total.saturating_sub(left.len() + right.len());
    let text = format!("{}{}{}", left, " ".repeat(spacer), right);
    let para = Paragraph::new(text).style(Style::default().fg(theme.accent_dim));
    f.render_widget(para, area);
}

// ── Helpers ───────────────────────────────────────────────────────────────────

fn draw_gauge(percent: f64, width: usize, theme: &Theme) -> Span<'static> {
    let filled = (percent / 100.0 * width as f64).round() as usize;
    let filled = filled.min(width);
    let empty = width - filled;
    let bar = format!(" [{}{}]", "█".repeat(filled), "░".repeat(empty));
    Span::styled(bar, Style::default().fg(theme.accent_dim))
}

fn gauge_color(percent: f64, theme: &Theme) -> ratatui::style::Color {
    if percent >= 90.0 {
        theme.error
    } else if percent >= 70.0 {
        theme.warning
    } else {
        theme.success
    }
}

fn context_color(percent: f64, theme: &Theme) -> ratatui::style::Color {
    if percent >= 90.0 {
        theme.error
    } else if percent >= 70.0 {
        theme.warning
    } else {
        theme.success
    }
}

fn budget_color(percent: f64, theme: &Theme) -> ratatui::style::Color {
    if percent >= 100.0 {
        theme.error
    } else if percent >= 80.0 {
        theme.warning
    } else {
        theme.success
    }
}

fn format_uptime(seconds: i64) -> String {
    let days = seconds / 86400;
    let hours = (seconds % 86400) / 3600;
    let mins = (seconds % 3600) / 60;
    if days > 0 {
        format!("{}d {}h {}m", days, hours, mins)
    } else if hours > 0 {
        format!("{}h {}m", hours, mins)
    } else {
        format!("{}m", mins)
    }
}
