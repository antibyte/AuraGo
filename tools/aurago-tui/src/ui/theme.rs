use ratatui::style::{Color, Modifier, Style};

#[derive(Debug, Clone)]
pub struct Theme {
    pub bg: Color,
    pub fg: Color,
    pub accent: Color,
    pub accent_dim: Color,
    pub success: Color,
    pub warning: Color,
    pub error: Color,
    pub border: Color,
    pub border_focus: Color,
    pub user_msg: Color,
    pub assistant_msg: Color,
    pub system_msg: Color,
    pub tool_msg: Color,
}

impl Default for Theme {
    fn default() -> Self {
        Self {
            bg: Color::Black,
            fg: Color::White,
            accent: Color::Cyan,
            accent_dim: Color::DarkGray,
            success: Color::Green,
            warning: Color::Yellow,
            error: Color::Red,
            border: Color::DarkGray,
            border_focus: Color::Cyan,
            user_msg: Color::Blue,
            assistant_msg: Color::Magenta,
            system_msg: Color::Gray,
            tool_msg: Color::Yellow,
        }
    }
}

impl Theme {
    pub fn from_mood(mood: &str) -> Self {
        let base = Self::default();
        match mood.to_lowercase().as_str() {
            "happy" | "fröhlich" | "excited" => Self {
                accent: Color::Yellow,
                border_focus: Color::Rgb(255, 165, 0),
                assistant_msg: Color::Rgb(255, 140, 0),
                ..base
            },
            "sad" | "nachdenklich" | "melancholic" => Self {
                accent: Color::Blue,
                border_focus: Color::Rgb(100, 149, 237),
                assistant_msg: Color::Rgb(70, 130, 180),
                ..base
            },
            "angry" | "wütend" | "stressed" => Self {
                accent: Color::Red,
                border_focus: Color::Rgb(220, 20, 60),
                assistant_msg: Color::Rgb(178, 34, 34),
                ..base
            },
            "curious" | "neugierig" => Self {
                accent: Color::Green,
                border_focus: Color::Rgb(50, 205, 50),
                assistant_msg: Color::Rgb(60, 179, 113),
                ..base
            },
            _ => base,
        }
    }

    pub fn glow_color(&self, tick: u64) -> Color {
        let hue = (tick % 360) as f32;
        hsv_to_rgb(hue, 1.0, 1.0)
    }
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

pub fn spinner_frame(tick: u64) -> &'static str {
    const FRAMES: &[&str] = &["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
    FRAMES[tick as usize % FRAMES.len()]
}
