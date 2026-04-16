package agent

import (
	"database/sql"
	"log/slog"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
)

// DispatchContext bundles the shared dependencies passed through the tool-dispatch chain.
// It replaces the 30+ individual function parameters that were previously threaded
// from DispatchToolCall → dispatchInner → dispatchExec/Comm/Services/Infra.
type DispatchContext struct {
	Cfg                *config.Config
	Logger             *slog.Logger
	LLMClient          llm.ChatClient
	Vault              *security.Vault
	Registry           *tools.ProcessRegistry
	Manifest           *tools.Manifest
	CronManager        *tools.CronManager
	MissionManagerV2   *tools.MissionManagerV2
	LongTermMem        memory.VectorDB
	ShortTermMem       *memory.SQLiteMemory
	KG                 *memory.KnowledgeGraph
	InventoryDB        *sql.DB
	InvasionDB         *sql.DB
	CheatsheetDB       *sql.DB
	ImageGalleryDB     *sql.DB
	MediaRegistryDB    *sql.DB
	HomepageRegistryDB *sql.DB
	ContactsDB         *sql.DB
	PlannerDB          *sql.DB
	SQLConnectionsDB   *sql.DB
	SQLConnectionPool  *sqlconnections.ConnectionPool
	RemoteHub          *remote.RemoteHub
	HistoryMgr         *memory.HistoryManager
	IsMaintenance      bool
	SurgeryPlan        string
	Guardian           *security.Guardian
	LLMGuardian        *security.LLMGuardian
	SessionID          string
	CoAgentRegistry    *CoAgentRegistry
	BudgetTracker      *budget.Tracker
	DaemonSupervisor   *tools.DaemonSupervisor
	PreparationService *services.MissionPreparationService
	ExecutionTimeMs    int64
}
