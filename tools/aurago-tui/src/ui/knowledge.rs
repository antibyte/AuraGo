use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph},
};

use crate::app::AppState;
use super::theme::Theme;

pub fn draw_knowledge(f: &mut Frame, app: &AppState, theme: &Theme) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),  // header
            Constraint::Min(5),    // content
            Constraint::Length(1), // status
        ])
        .split(f.area());

    draw_knowledge_header(f, app, theme, chunks[0]);
    draw_knowledge_content(f, app, theme, chunks[1]);
    draw_knowledge_status(f, app, theme, chunks[2]);
}

fn draw_knowledge_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let search_indicator = if app.knowledge_search_active {
        format!(" Search: {}▎", app.knowledge_search)
    } else {
        String::new()
    };

    let header = Line::from(vec![
        Span::styled(" 📚 ", Style::default().fg(theme.accent)),
        Span::styled(
            "Knowledge",
            Style::default().fg(theme.fg).add_modifier(Modifier::BOLD),
        ),
        Span::styled(
            format!("  ({} files)", app.knowledge_files.len()),
            Style::default().fg(theme.accent_dim),
        ),
        Span::styled(
            search_indicator,
            Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
        ),
    ]);

    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);
    f.render_widget(Paragraph::new(header), inner);
}

fn draw_knowledge_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.knowledge_loading {
        let msg = Paragraph::new(Line::from(vec![
            Span::styled("  Loading knowledge files…", Style::default().fg(theme.accent_dim)),
        ]));
        f.render_widget(msg, area);
        return;
    }

    if app.knowledge_files.is_empty() {
        let msg = Paragraph::new(Line::from(vec![
            Span::styled("  No knowledge files found", Style::default().fg(theme.accent_dim)),
        ]));
        f.render_widget(msg, area);
        return;
    }

    // Split into list (left) and detail (right)
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage(40),
            Constraint::Percentage(60),
        ])
        .split(area);

    draw_knowledge_list(f, app, theme, chunks[0]);
    draw_knowledge_detail(f, app, theme, chunks[1]);
}

fn draw_knowledge_list(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = app
        .knowledge_files
        .iter()
        .enumerate()
        .map(|(i, file)| {
            let is_selected = Some(i) == app.knowledge_selected;
            let style = if is_selected {
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.fg)
            };
            let marker = if is_selected { "▸ " } else { "  " };
            let icon = file_icon(&file.name);

            ListItem::new(Line::from(vec![
                Span::styled(marker, Style::default().fg(theme.accent)),
                Span::styled(icon, Style::default().fg(theme.accent)),
                Span::styled(" ", Style::default()),
                Span::styled(truncate_str(&file.name, (area.width as usize).saturating_sub(8)), style),
            ]))
        })
        .collect();

    let block = Block::default()
        .title(" Files ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);

    let list = List::new(items);
    let mut state = ListState::default();
    state.select(app.knowledge_selected);
    f.render_stateful_widget(list, inner, &mut state);
}

fn draw_knowledge_detail(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let block = Block::default()
        .title(" Details ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);

    let selected_idx = match app.knowledge_selected {
        Some(idx) => idx,
        None => {
            let msg = Paragraph::new(Line::from(vec![
                Span::styled("  Select a file to view details", Style::default().fg(theme.accent_dim)),
            ]));
            f.render_widget(msg, inner);
            return;
        }
    };

    let file = match app.knowledge_files.get(selected_idx) {
        Some(f) => f,
        None => return,
    };

    let size_str = format_size(file.size);
    let lines: Vec<Line> = vec![
        Line::from(vec![
            Span::styled("  Name:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(&file.name, Style::default().fg(theme.fg).add_modifier(Modifier::BOLD)),
        ]),
        Line::from(vec![
            Span::styled("  Size:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(size_str, Style::default().fg(theme.fg)),
        ]),
        Line::from(vec![
            Span::styled("  Modified:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(&file.modified, Style::default().fg(theme.fg)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("  Type:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(
                file_type_label(&file.name),
                Style::default().fg(theme.fg),
            ),
        ]),
    ];

    let para = Paragraph::new(lines);
    f.render_widget(para, inner);
}

fn draw_knowledge_status(f: &mut Frame, _app: &AppState, theme: &Theme, area: Rect) {
    let spans = vec![
        Span::styled("  j/k", Style::default().fg(theme.accent)),
        Span::styled(" navigate ", Style::default().fg(theme.accent_dim)),
        Span::styled("Enter", Style::default().fg(theme.accent)),
        Span::styled(" view ", Style::default().fg(theme.accent_dim)),
        Span::styled("Del", Style::default().fg(theme.accent)),
        Span::styled(" delete ", Style::default().fg(theme.accent_dim)),
        Span::styled("/", Style::default().fg(theme.accent)),
        Span::styled(" search ", Style::default().fg(theme.accent_dim)),
        Span::styled("r", Style::default().fg(theme.accent)),
        Span::styled(" reload", Style::default().fg(theme.accent_dim)),
    ];

    let para = Paragraph::new(Line::from(spans));
    f.render_widget(para, area);
}

// ── Helpers ────────────────────────────────────────────────────────────────────

fn file_icon(name: &str) -> &'static str {
    let ext = name.rsplit('.').next().unwrap_or("").to_lowercase();
    match ext.as_str() {
        "pdf" => "📄",
        "txt" | "md" | "markdown" => "📝",
        "json" | "yaml" | "yml" | "toml" => "📋",
        "py" => "🐍",
        "go" => "🔵",
        "rs" => "🦀",
        "jpg" | "jpeg" | "png" | "gif" | "svg" | "webp" => "🖼️",
        "mp3" | "wav" | "ogg" | "flac" => "🎵",
        "mp4" | "mkv" | "avi" => "🎬",
        "zip" | "tar" | "gz" | "bz2" => "📦",
        _ => "📎",
    }
}

fn file_type_label(name: &str) -> String {
    let ext = name.rsplit('.').next().unwrap_or("").to_lowercase();
    match ext.as_str() {
        "pdf" => "PDF Document".to_string(),
        "txt" => "Plain Text".to_string(),
        "md" | "markdown" => "Markdown".to_string(),
        "json" => "JSON".to_string(),
        "yaml" | "yml" => "YAML".to_string(),
        "toml" => "TOML".to_string(),
        "py" => "Python Script".to_string(),
        "go" => "Go Source".to_string(),
        "rs" => "Rust Source".to_string(),
        _ => format!("{} file", ext.to_uppercase()),
    }
}

fn format_size(bytes: i64) -> String {
    const KB: f64 = 1024.0;
    const MB: f64 = KB * 1024.0;
    const GB: f64 = MB * 1024.0;
    let b = bytes as f64;
    if b >= GB {
        format!("{:.1} GB", b / GB)
    } else if b >= MB {
        format!("{:.1} MB", b / MB)
    } else if b >= KB {
        format!("{:.1} KB", b / KB)
    } else {
        format!("{} B", bytes)
    }
}

fn truncate_str(s: &str, max_len: usize) -> String {
    if s.len() <= max_len {
        s.to_string()
    } else {
        let mut end = max_len;
        while !s.is_char_boundary(end) && end > 0 {
            end -= 1;
        }
        format!("{}…", &s[..end])
    }
}
