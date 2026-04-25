use ratatui::{
    layout::{Alignment, Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    widgets::{Block, Borders, Clear, Paragraph, Wrap},
    Frame,
};

use crate::app::AppState;
use super::theme::{spinner_frame, Theme};
use super::utils;

pub fn draw_login(f: &mut Frame, app: &AppState, theme: &Theme) {
    let area = f.area();
    f.render_widget(
        Block::default().style(Style::default().bg(theme.bg).fg(theme.fg)),
        area,
    );

    let center = utils::centered_rect(50, 40, area);
    f.render_widget(Clear, center);

    let block = Block::default()
        .title(" 🔐 AuraGo Terminal Chat ")
        .borders(Borders::ALL)
        .border_style(Style::default().fg(theme.border_focus).add_modifier(Modifier::BOLD))
        .style(Style::default().bg(theme.bg).fg(theme.fg));

    let inner = block.inner(center);
    f.render_widget(block, center);

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(2)
        .constraints([
            Constraint::Length(1),
            Constraint::Length(1),
            Constraint::Length(1),
            Constraint::Length(4),
            Constraint::Length(4),
            Constraint::Length(2),
            Constraint::Length(2),
            Constraint::Min(0),
        ])
        .split(inner);

    let server_line = Paragraph::new(format!("Server: {}", app.server_url))
        .style(Style::default().fg(theme.accent_dim));
    f.render_widget(server_line, chunks[0]);

    let status = if app.auth_enabled {
        "Authentication required"
    } else {
        "No authentication required"
    };
    let status_line = Paragraph::new(status).style(Style::default().fg(theme.accent));
    f.render_widget(status_line, chunks[1]);

    // Password field with integrated label in block title
    let password_masked = "*".repeat(app.login_password.len());
    let pass_border = if app.login_focus_otp {
        Style::default().fg(theme.border)
    } else {
        Style::default().fg(theme.border_focus)
    };
    let pass_input = Paragraph::new(password_masked)
        .block(
            Block::default()
                .title(" Password ")
                .borders(Borders::ALL)
                .border_style(pass_border),
        )
        .style(Style::default().bg(Color::Black).fg(theme.fg));
    f.render_widget(pass_input, chunks[3]);

    if app.totp_enabled {
        let totp_border = if app.login_focus_otp {
            Style::default().fg(theme.border_focus)
        } else {
            Style::default().fg(theme.border)
        };
        let totp_input = Paragraph::new(app.login_totp.clone())
            .block(
                Block::default()
                    .title(" OTP Code ")
                    .borders(Borders::ALL)
                    .border_style(totp_border),
            )
            .style(Style::default().bg(Color::Black).fg(theme.fg));
        f.render_widget(totp_input, chunks[4]);
    }

    let btn_text = if app.login_loading {
        format!(" [{}] Logging in... ", spinner_frame(app.tick_counter))
    } else {
        " [ 🔓 Login ] ".to_string()
    };
    let btn = Paragraph::new(btn_text)
        .alignment(Alignment::Center)
        .style(
            Style::default()
                .fg(theme.bg)
                .bg(theme.accent)
                .add_modifier(Modifier::BOLD),
        );
    f.render_widget(btn, chunks[5]);

    if let Some(err) = &app.login_error {
        let err_para = Paragraph::new(err.as_str())
            .wrap(Wrap { trim: true })
            .style(Style::default().fg(theme.error).add_modifier(Modifier::BOLD));
        f.render_widget(err_para, chunks[6]);
    }
}


