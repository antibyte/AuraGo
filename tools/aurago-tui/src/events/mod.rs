use crossterm::event::Event as CrosstermEvent;
use tokio::sync::mpsc;

use crate::api::sse::SseEvent;

pub mod keybindings;

#[derive(Debug)]
pub enum AppEvent {
    Crossterm(CrosstermEvent),
    Tick,
    Sse(SseEvent),
    ChatSent,
    ChatError(String),
    LoginResult(Result<(), String>),
    AuthCheckResult(Result<(bool, bool, bool), String>),
    HistoryLoaded(Result<Vec<crate::api::types::HistoryMessage>, String>),
}

pub fn sse_sender() -> mpsc::UnboundedSender<SseEvent> {
    let (tx, _rx) = mpsc::unbounded_channel();
    tx
}
