use ratatui::style::Color;
use super::utils;

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
        Self::dark()
    }
}

impl Theme {
    pub fn dark() -> Self {
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

    pub fn light() -> Self {
        Self {
            bg: Color::Rgb(250, 250, 250),
            fg: Color::Black,
            accent: Color::Blue,
            accent_dim: Color::Gray,
            success: Color::Green,
            warning: Color::Rgb(200, 150, 0),
            error: Color::Red,
            border: Color::Gray,
            border_focus: Color::Blue,
            user_msg: Color::Blue,
            assistant_msg: Color::Magenta,
            system_msg: Color::Gray,
            tool_msg: Color::Rgb(180, 140, 0),
        }
    }

    pub fn midnight() -> Self {
        Self {
            bg: Color::Rgb(15, 15, 35),
            fg: Color::Rgb(220, 220, 240),
            accent: Color::Rgb(100, 200, 255),
            accent_dim: Color::Rgb(60, 60, 100),
            success: Color::Rgb(100, 255, 150),
            warning: Color::Rgb(255, 200, 100),
            error: Color::Rgb(255, 100, 120),
            border: Color::Rgb(50, 50, 80),
            border_focus: Color::Rgb(100, 200, 255),
            user_msg: Color::Rgb(120, 160, 255),
            assistant_msg: Color::Rgb(200, 120, 255),
            system_msg: Color::Rgb(140, 140, 170),
            tool_msg: Color::Rgb(255, 220, 100),
        }
    }

    pub fn by_name(name: &str) -> Self {
        match name {
            "light" => Self::light(),
            "midnight" => Self::midnight(),
            _ => Self::dark(),
        }
    }

    pub fn next_name(current: &str) -> &'static str {
        match current {
            "default" => "light",
            "light" => "midnight",
            "midnight" => "default",
            _ => "light",
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
        utils::hsv_to_rgb(hue, 1.0, 1.0)
    }
}



pub fn spinner_frame(tick: u64) -> &'static str {
    const FRAMES: &[&str] = &["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
    FRAMES[tick as usize % FRAMES.len()]
}
