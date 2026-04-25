use ratatui::{
    layout::Alignment,
    style::{Modifier, Style},
    text::{Line, Span, Text},
    widgets::{Block, Clear, Paragraph},
    Frame,
};

use super::theme::Theme;
use super::utils;

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

    let center = utils::centered_rect(80, 60, area);
    f.render_widget(Clear, center);

    let mut lines: Vec<Line> = LOGO
        .iter()
        .enumerate()
        .map(|(i, line)| {
            let hue = ((tick as usize * 2 + i * 15) % 360) as f32;
            let color = utils::hsv_to_rgb(hue, 1.0, 1.0);
            Line::from(Span::styled(
                *line,
                Style::default().fg(color).add_modifier(Modifier::BOLD),
            ))
        })
        .collect();

    lines.push(Line::from(""));

    // Typing effect for subtitle
    let subtitle = "Terminal Chat Client";
    let visible_chars = ((tick as usize) / 3).min(subtitle.len());
    let typed_text = &subtitle[..visible_chars];
    let cursor = if visible_chars < subtitle.len() && tick % 6 < 3 { "▎" } else { "" };
    let display = format!("{:^40}", format!("{}{}", typed_text, cursor));
    lines.push(Line::from(Span::styled(
        display,
        Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
    )));
    lines.push(Line::from(""));

    // Pulsing "press any key" — fade in/out
    let pulse_alpha = (tick as f32 * 0.05).sin() * 0.5 + 0.5;
    let press_style = if pulse_alpha > 0.5 {
        Style::default().fg(theme.accent)
    } else {
        Style::default().fg(theme.accent_dim)
    };
    lines.push(Line::from(Span::styled(
        "      Press any key to continue...    ",
        press_style,
    )));
    lines.push(Line::from(""));
    lines.push(Line::from(Span::styled(
        "              v0.1.0                   ",
        Style::default().fg(theme.accent_dim),
    )));

    let para = Paragraph::new(Text::from(lines)).alignment(Alignment::Center);
    f.render_widget(para, center);
}


