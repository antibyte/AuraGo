use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph},
};

use crate::app::{AppState, MediaTab};
use super::theme::Theme;

pub fn draw_media(f: &mut Frame, app: &AppState, theme: &Theme) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),  // header with tabs
            Constraint::Min(5),    // content
            Constraint::Length(1), // status
        ])
        .split(f.area());

    draw_media_header(f, app, theme, chunks[0]);
    draw_media_content(f, app, theme, chunks[1]);
    draw_media_status(f, app, theme, chunks[2]);
}

fn draw_media_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let tab_audio = match app.media_tab {
        MediaTab::Audio => " 🎵 Audio ",
        MediaTab::Documents => "   Audio ",
    };
    let tab_docs = match app.media_tab {
        MediaTab::Documents => " 📄 Documents ",
        MediaTab::Audio => "   Documents ",
    };

    let search_indicator = if app.media_search_active {
        format!("  Search: {}▎", app.media_search)
    } else {
        String::new()
    };

    let audio_style = match app.media_tab {
        MediaTab::Audio => Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
        MediaTab::Documents => Style::default().fg(theme.accent_dim),
    };
    let docs_style = match app.media_tab {
        MediaTab::Documents => Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
        MediaTab::Audio => Style::default().fg(theme.accent_dim),
    };

    let header = Line::from(vec![
        Span::styled(tab_audio, audio_style),
        Span::styled("│", Style::default().fg(theme.border)),
        Span::styled(tab_docs, docs_style),
        Span::styled(
            format!("  ({} items)", app.media_items.len()),
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

fn draw_media_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.media_loading {
        let msg = Paragraph::new(Line::from(vec![
            Span::styled("  Loading media…", Style::default().fg(theme.accent_dim)),
        ]));
        f.render_widget(msg, area);
        return;
    }

    if app.media_items.is_empty() {
        let msg = Paragraph::new(Line::from(vec![
            Span::styled(
                format!("  No {} files found", match app.media_tab {
                    MediaTab::Audio => "audio",
                    MediaTab::Documents => "document",
                }),
                Style::default().fg(theme.accent_dim),
            ),
        ]));
        f.render_widget(msg, area);
        return;
    }

    // Split into list (left) and detail (right)
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage(45),
            Constraint::Percentage(55),
        ])
        .split(area);

    draw_media_list(f, app, theme, chunks[0]);
    draw_media_detail(f, app, theme, chunks[1]);
}

fn draw_media_list(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = app
        .media_items
        .iter()
        .enumerate()
        .map(|(i, item)| {
            let is_selected = Some(i) == app.media_selected;
            let style = if is_selected {
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.fg)
            };
            let marker = if is_selected { "▸ " } else { "  " };

            let icon = match app.media_tab {
                MediaTab::Audio => "🎵",
                MediaTab::Documents => doc_icon(&item.filename),
            };

            let name = truncate_str(&item.filename, (area.width as usize).saturating_sub(10));

            ListItem::new(Line::from(vec![
                Span::styled(marker, Style::default().fg(theme.accent)),
                Span::styled(format!("{} ", icon), Style::default()),
                Span::styled(name, style),
            ]))
        })
        .collect();

    let title = match app.media_tab {
        MediaTab::Audio => " Audio ",
        MediaTab::Documents => " Documents ",
    };

    let block = Block::default()
        .title(title)
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);

    let list = List::new(items);
    let mut state = ListState::default();
    state.select(app.media_selected);
    f.render_stateful_widget(list, inner, &mut state);
}

fn draw_media_detail(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let block = Block::default()
        .title(" Details ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);

    let selected_idx = match app.media_selected {
        Some(idx) => idx,
        None => {
            let msg = Paragraph::new(Line::from(vec![
                Span::styled("  Select an item to view details", Style::default().fg(theme.accent_dim)),
            ]));
            f.render_widget(msg, inner);
            return;
        }
    };

    let item = match app.media_items.get(selected_idx) {
        Some(i) => i,
        None => return,
    };

    let size_str = format_size(0); // MediaItem has no size field
    let icon = match app.media_tab {
        MediaTab::Audio => "🎵",
        MediaTab::Documents => doc_icon(&item.filename),
    };

    let lines: Vec<Line> = vec![
        Line::from(vec![
            Span::styled(format!("  {} ", icon), Style::default()),
            Span::styled(
                truncate_str(&item.filename, (area.width as usize).saturating_sub(6)),
                Style::default().fg(theme.fg).add_modifier(Modifier::BOLD),
            ),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("  Size:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(size_str, Style::default().fg(theme.fg)),
        ]),
        Line::from(vec![
            Span::styled("  Type:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(&item.media_type, Style::default().fg(theme.fg)),
        ]),
        Line::from(vec![
            Span::styled("  Created:  ", Style::default().fg(theme.accent_dim)),
            Span::styled(&item.created_at, Style::default().fg(theme.fg)),
        ]),
    ];

    let para = Paragraph::new(lines);
    f.render_widget(para, inner);
}

fn draw_media_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let tab_hint = match app.media_tab {
        MediaTab::Audio => "Tab→ docs",
        MediaTab::Documents => "Tab← audio",
    };

    let page_info = if app.media_total > 0 {
        let current_end = (app.media_offset as i64 + app.media_items.len() as i64).min(app.media_total);
        format!("  {}-{}/{}", app.media_offset + 1, current_end, app.media_total)
    } else {
        String::new()
    };

    let spans = vec![
        Span::styled("  j/k", Style::default().fg(theme.accent)),
        Span::styled(" navigate ", Style::default().fg(theme.accent_dim)),
        Span::styled("Del", Style::default().fg(theme.accent)),
        Span::styled(" delete ", Style::default().fg(theme.accent_dim)),
        Span::styled("h/l", Style::default().fg(theme.accent)),
        Span::styled(format!(" {} ", tab_hint), Style::default().fg(theme.accent_dim)),
        Span::styled("/", Style::default().fg(theme.accent)),
        Span::styled(" search ", Style::default().fg(theme.accent_dim)),
        Span::styled("r", Style::default().fg(theme.accent)),
        Span::styled(" reload", Style::default().fg(theme.accent_dim)),
        Span::styled(page_info, Style::default().fg(theme.accent_dim)),
    ];

    let para = Paragraph::new(Line::from(spans));
    f.render_widget(para, area);
}

// ── Helpers ────────────────────────────────────────────────────────────────────

fn doc_icon(filename: &str) -> &'static str {
    let ext = filename.rsplit('.').next().unwrap_or("").to_lowercase();
    match ext.as_str() {
        "pdf" => "📄",
        "txt" | "md" => "📝",
        "doc" | "docx" => "📘",
        "xls" | "xlsx" => "📊",
        "ppt" | "pptx" => "📙",
        "csv" => "📋",
        _ => "📎",
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
