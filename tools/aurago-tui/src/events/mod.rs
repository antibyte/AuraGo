use crossterm::event::Event as CrosstermEvent;

use crate::api::sse::SseEvent;
use crate::api::types::*;

pub mod keybindings;

#[derive(Debug)]
pub enum AppEvent {
    // ── Terminal events ───────────────────────────────────────────────────
    Crossterm(CrosstermEvent),
    Tick,

    // ── SSE ───────────────────────────────────────────────────────────────
    Sse(SseEvent),

    // ── Auth ──────────────────────────────────────────────────────────────
    LoginResult(Result<(), String>),
    AuthCheckResult(Result<(bool, bool, bool), String>),

    // ── Chat ──────────────────────────────────────────────────────────────
    ChatSent,
    ChatError(String),
    HistoryLoaded(Result<Vec<HistoryMessage>, String>),

    // ── Sessions ──────────────────────────────────────────────────────────
    SessionsLoaded(Result<Vec<ChatSession>, String>),
    SessionCreated(Result<ChatSession, String>),
    SessionDeleted(Result<(), String>),

    // ── Dashboard ─────────────────────────────────────────────────────────
    DashboardSystemLoaded(Result<SystemInfo, String>),
    DashboardBudgetLoaded(Result<BudgetInfo, String>),
    DashboardOverviewLoaded(Result<OverviewInfo, String>),
    DashboardPersonalityLoaded(Result<PersonalityState, String>),
    DashboardLogsLoaded(Result<Vec<LogEntry>, String>),
    DashboardActivityLoaded(Result<Vec<CronEntry>, String>),

    // ── Plans ─────────────────────────────────────────────────────────────
    PlansLoaded(Result<Vec<Plan>, String>),
    PlanDetailLoaded(Result<Plan, String>),
    PlanActionDone(Result<(), String>),

    // ── Missions ──────────────────────────────────────────────────────────
    MissionsLoaded(Result<Vec<Mission>, String>),
    MissionActionDone(Result<(), String>),

    // ── Skills ────────────────────────────────────────────────────────────
    SkillsLoaded(Result<Vec<Skill>, String>),
    SkillActionDone(Result<(), String>),

    // ── Containers ────────────────────────────────────────────────────────
    ContainersLoaded(Result<Vec<Container>, String>),
    ContainerActionDone(Result<serde_json::Value, String>),
    ContainerLogsLoaded(Result<serde_json::Value, String>),
}
