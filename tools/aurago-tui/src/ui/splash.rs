use ratatui::{
    layout::{Alignment, Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Clear, Paragraph},
    Frame,
};

use super::theme::Theme;

const LOGO: &[&str] = &[
    "  ░█████╗░██╗░░░██╗██████╗░░█████╗░░██████╗░░█████╗░  ",
    "  ██╔══██╗██║░░░██║██╔══██╗██╔══██╗██╔════╝░██╔══██╗  ",
    "  ███████║██║░░░██║██████╔╝███████║██║░░██╗░██║░░██║  ",
    "  ██╔══██║██║░░░██║██╔══██╗██╔══██║██║░░╚██╗██║░░██║  ",
    "  ██║░░██║╚██████╔╝██║░░██║██║░░██║╚██████╔╝╚█████╔╝  ",
    "  ╚═╝░░╚═╝░╚═════╝░╚═╝░░╚═╝╚═╝░░╚═╝░╚═════╝░░╚════╝░  ",
];

pub fn draw_splash(f: &mut Frame, theme: &Theme, tick: u64) {
    let area = f.area();
    f.render_widget(
        Block::default().style(Style::default().bg(theme.bg).fg(theme.fg)),
        area,
    );

    let center = centered_rect(80, 60, area);
    f.render_widget(Clear, center);

    let mut lines: Vec<Line> = LOGO
        .iter()
        .enumerate()
        .map(|(i, line)| {
            let hue = ((tick as usize * 2 + i * 15) % 360) as f32;
            let color = hsv_to_rgb(hue, 1.0, 1.0);
            Line::from(Span::styled(
                *line,
                Style::default().fg(color).add_modifier(Modifier::BOLD),
            ))
        })
        .collect();

    lines.push(Line::from(""));
    lines.push(Line::from(Span::styled(
        "         Terminal Chat Client         ",
        Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
    )));
    lines.push(Line::from(""));
    lines.push(Line::from(Span::styled(
        "      Press any key to continue...    ",
        Style::default().fg(theme.accent_dim),
    )));

    let para = Paragraph::new(Text::from(lines)).alignment(Alignment::Center);
    f.render_widget(para, center);
}

fn centered_rect(percent_x: u16, percent_y: u16, r: Rect) -> Rect {
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

fn hsv_to_rgb(h: f32, s: f32, v: f32) -> Color {
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
