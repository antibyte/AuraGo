//! Shared UI utility functions to eliminate code duplication.

use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::Color,
};

/// Truncate a string to `max_len` bytes, appending "…" if truncated.
/// Handles Unicode character boundaries correctly.
pub fn truncate_str(s: &str, max_len: usize) -> String {
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

/// Return a centered sub-rectangle within `r`, sized by percentage.
pub fn centered_rect(percent_x: u16, percent_y: u16, r: Rect) -> Rect {
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

/// Convert HSV (h in degrees, s and v in 0..1) to a ratatui `Color::Rgb`.
pub fn hsv_to_rgb(h: f32, s: f32, v: f32) -> Color {
    let c = v * s;
    let x = c * (1.0 - ((h / 60.0) % 2.0 - 1.0).abs());
    let m = v - c;
    let (r, g, b) = match h as u32 / 60 {
        0 => (c, x, 0.0),
        1 => (x, c, 0.0),
        2 => (0.0, c, x),
        3 => (0.0, x, c),
        4 => (x, 0.0, c),
        _ => (c, 0.0, x),
    };
    Color::Rgb(
        ((r + m) * 255.0) as u8,
        ((g + m) * 255.0) as u8,
        ((b + m) * 255.0) as u8,
    )
}

/// Format byte count as human-readable size string.
pub fn format_size(bytes: i64) -> String {
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

/// Make a text-based progress bar: `[████████░░░░░░░░░░░░]`
pub fn make_progress_bar(progress: f64, width: usize) -> String {
    let filled = (progress * width as f64).round() as usize;
    let filled = filled.min(width);
    let empty = width - filled;
    format!(" [{}{}]", "█".repeat(filled), "░".repeat(empty))
}

/// Draw a text-based gauge span: `[████████░░░░░░░░░░░░]`
pub fn draw_gauge_span(percent: f64, width: usize) -> String {
    let filled = (percent / 100.0 * width as f64).round() as usize;
    let filled = filled.min(width);
    let empty = width - filled;
    format!(" [{}{}]", "█".repeat(filled), "░".repeat(empty))
}

/// Return a color based on a percentage value (green → yellow → red).
pub fn percent_color(percent: f64, theme: &super::theme::Theme) -> Color {
    if percent >= 90.0 {
        theme.error
    } else if percent >= 70.0 {
        theme.warning
    } else {
        theme.success
    }
}

/// Capitalize the first letter of a string.
pub fn capitalize(s: &str) -> String {
    let mut c = s.chars();
    match c.next() {
        None => String::new(),
        Some(f) => f.to_uppercase().collect::<String>() + c.as_str(),
    }
}
