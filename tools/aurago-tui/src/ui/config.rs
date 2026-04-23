use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Clear, List, ListItem, ListState, Paragraph},
};

use crate::app::AppState;
use super::theme::Theme;

pub fn draw_config(f: &mut Frame, app: &AppState, theme: &Theme) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),  // header
            Constraint::Min(5),    // content
            Constraint::Length(1), // status
        ])
        .split(f.area());

    draw_config_header(f, app, theme, chunks[0]);
    draw_config_content(f, app, theme, chunks[1]);
    draw_config_status(f, app, theme, chunks[2]);
}

fn draw_config_header(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let tabs: Vec<Line> = vec![Line::from(vec![
        Span::styled(" ⚙ ", Style::default().fg(theme.accent)),
        Span::styled(
            "Configuration",
            Style::default().fg(theme.fg).add_modifier(Modifier::BOLD),
        ),
        if app.config_dirty {
            Span::styled(" ● ", Style::default().fg(ratatui::style::Color::Yellow))
        } else {
            Span::raw("")
        },
        if app.config_dirty {
            Span::styled("unsaved", Style::default().fg(ratatui::style::Color::Yellow))
        } else {
            Span::raw("")
        },
        Span::raw("  "),
        Span::styled(
            format!("Section {}/{}  ", app.config_section_index + 1, app.config_sections.len().max(1)),
            Style::default().fg(theme.accent_dim),
        ),
    ])];

    let block = Block::default()
        .borders(Borders::BOTTOM)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);
    f.render_widget(Paragraph::new(tabs), inner);
}

fn draw_config_content(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    if app.config_loading {
        let msg = Paragraph::new(Line::from(vec![
            Span::styled("  Loading configuration…", Style::default().fg(theme.accent_dim)),
        ]));
        f.render_widget(msg, area);
        return;
    }

    // Split into section sidebar (left) and fields (right)
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Length(22), // section sidebar
            Constraint::Min(30),   // fields
        ])
        .split(area);

    draw_section_sidebar(f, app, theme, chunks[0]);
    draw_fields_panel(f, app, theme, chunks[1]);
}

fn draw_section_sidebar(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let items: Vec<ListItem> = app
        .config_sections
        .iter()
        .enumerate()
        .map(|(i, section)| {
            let is_selected = i == app.config_section_index;
            let style = if is_selected {
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.fg)
            };
            let marker = if is_selected { "▸ " } else { "  " };
            let label = truncate_str(section, (area.width as usize).saturating_sub(4));
            ListItem::new(Line::from(vec![
                Span::styled(marker, Style::default().fg(theme.accent)),
                Span::styled(label, style),
            ]))
        })
        .collect();

    let block = Block::default()
        .title(" Sections ")
        .borders(Borders::RIGHT)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(area);
    f.render_widget(block, area);

    let list = List::new(items);
    let mut state = ListState::default();
    state.select(Some(app.config_section_index));
    f.render_stateful_widget(list, inner, &mut state);
}

fn draw_fields_panel(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    // Get the current section's fields from config_data
    let section_key = app.config_sections.get(app.config_section_index);
    let section_key = match section_key {
        Some(k) => k.as_str(),
        None => return,
    };

    let section_data = app.config_data.get(section_key);
    let section_data = match section_data {
        Some(data) => data,
        None => {
            let msg = Paragraph::new(Line::from(vec![
                Span::styled("  No data for this section", Style::default().fg(theme.accent_dim)),
            ]));
            f.render_widget(msg, area);
            return;
        }
    };

    // Collect fields as key-value pairs
    let fields = collect_fields(section_data, "");

    // Split into field list (top) and detail/edit area (bottom)
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Min(3),    // field list
            Constraint::Length(3), // edit area (if editing)
        ])
        .split(area);

    // Draw field list
    let items: Vec<ListItem> = fields
        .iter()
        .enumerate()
        .map(|(i, (key, value))| {
            let is_selected = i == app.config_field_index;
            let style = if is_selected {
                Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.fg)
            };
            let key_style = Style::default().fg(theme.accent_dim);
            let marker = if is_selected { "▸ " } else { "  " };

            let display_value = format_value(value, 60);
            ListItem::new(Line::from(vec![
                Span::styled(marker, Style::default().fg(theme.accent)),
                Span::styled(truncate_str(key, 28), key_style),
                Span::styled(" = ", Style::default().fg(theme.border)),
                Span::styled(display_value, style),
            ]))
        })
        .collect();

    let block = Block::default()
        .title(format!(" {} ", capitalize(section_key)))
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border));
    let inner = block.inner(chunks[0]);
    f.render_widget(block, chunks[0]);

    let list = List::new(items);
    let mut state = ListState::default();
    if !fields.is_empty() {
        state.select(Some(app.config_field_index.min(fields.len() - 1)));
    }
    f.render_stateful_widget(list, inner, &mut state);

    // Draw edit area
    if app.config_editing {
        draw_edit_field(f, app, theme, chunks[1]);
    }
}

fn draw_edit_field(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    f.render_widget(Clear, area);

    let block = Block::default()
        .title(" Edit Field (Enter=Save, Esc=Cancel) ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.accent));
    let inner = block.inner(area);
    f.render_widget(block, area);

    // Show cursor at edit position
    let cursor_idx = app.config_edit_cursor.min(app.config_edit_value.len());
    let (before, after) = app.config_edit_value.split_at(cursor_idx);
    let cursor_visible = app.tick_counter % 4 < 2;
    let cursor_str = if cursor_visible { "▎" } else { " " };

    let mut spans = vec![
        Span::styled(" ", Style::default()),
        Span::styled(before, Style::default().fg(theme.fg)),
        Span::styled(cursor_str, Style::default().fg(theme.accent)),
    ];
    if !after.is_empty() {
        spans.push(Span::styled(after, Style::default().fg(theme.fg)));
    }

    let input = Paragraph::new(Line::from(spans));
    f.render_widget(input, inner);
}

fn draw_config_status(f: &mut Frame, app: &AppState, theme: &Theme, area: Rect) {
    let mut spans = vec![
        Span::styled("  h/l", Style::default().fg(theme.accent)),
        Span::styled(" sections ", Style::default().fg(theme.accent_dim)),
        Span::styled("j/k", Style::default().fg(theme.accent)),
        Span::styled(" fields ", Style::default().fg(theme.accent_dim)),
        Span::styled("Enter", Style::default().fg(theme.accent)),
        Span::styled(" edit ", Style::default().fg(theme.accent_dim)),
        Span::styled("Ctrl+S", Style::default().fg(theme.accent)),
        Span::styled(" save ", Style::default().fg(theme.accent_dim)),
        Span::styled("r", Style::default().fg(theme.accent)),
        Span::styled(" reload", Style::default().fg(theme.accent_dim)),
    ];

    if app.config_dirty {
        spans.push(Span::styled("  ● ", Style::default().fg(ratatui::style::Color::Yellow)));
        spans.push(Span::styled("unsaved changes", Style::default().fg(ratatui::style::Color::Yellow)));
    }

    let para = Paragraph::new(Line::from(spans));
    f.render_widget(para, area);
}

// ── Helpers ────────────────────────────────────────────────────────────────────

/// Collect flat key-value pairs from a JSON object (one level deep)
fn collect_fields<'a>(data: &'a serde_json::Value, prefix: &str) -> Vec<(String, &'a serde_json::Value)> {
    let mut result = Vec::new();
    if let Some(obj) = data.as_object() {
        for (key, value) in obj {
            let full_key = if prefix.is_empty() {
                key.clone()
            } else {
                format!("{}.{}", prefix, key)
            };
            // Only flatten one level for display
            if value.is_object() && !prefix.is_empty() {
                // Nested objects: show as nested
                result.push((full_key, value));
            } else if value.is_object() {
                // Top-level objects: recurse one level
                result.extend(collect_fields(value, &full_key));
            } else {
                result.push((full_key, value));
            }
        }
    }
    result
}

fn format_value(value: &serde_json::Value, max_len: usize) -> String {
    let s = match value {
        serde_json::Value::String(s) => s.clone(),
        serde_json::Value::Bool(b) => b.to_string(),
        serde_json::Value::Number(n) => n.to_string(),
        serde_json::Value::Null => "(null)".to_string(),
        serde_json::Value::Array(arr) => format!("[{} items]", arr.len()),
        serde_json::Value::Object(obj) => format!("{{{} keys}}", obj.len()),
    };
    truncate_str(&s, max_len)
}

fn capitalize(s: &str) -> String {
    let mut c = s.chars();
    match c.next() {
        None => String::new(),
        Some(f) => f.to_uppercase().collect::<String>() + c.as_str(),
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
