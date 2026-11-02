package agent

import "slices"

// CoAgentSpawnContext is a server-owned snapshot, never a model argument. Only
// the selected dependencies and authority below cross the delegation boundary.
type CoAgentSpawnContext struct {
	run        RunConfig
	registry   *CoAgentRegistry
	provenance isolatedProvenance
}

func newCoAgentSpawnContext(dc *DispatchContext) CoAgentSpawnContext {
	if dc == nil || dc.Cfg == nil {
		return CoAgentSpawnContext{}
	}
	intent := dc.UserContext
	if dc.AuthorizingUserIntent != nil {
		intent = *dc.AuthorizingUserIntent
	}
	return CoAgentSpawnContext{
		registry:   dc.CoAgentRegistry,
		provenance: modelIsolatedBodies.snapshot(dc.SessionID),
		run: RunConfig{
			Config: dc.Cfg.Clone(), Logger: dc.Logger,
			UserIntent: intent, ParentSessionID: dc.SessionID,
			AllowedTools:       delegatedScope(dc.AllowedTools, dc.ToolScopeRestricted),
			AllowedAgentSkills: delegatedScope(dc.AllowedAgentSkills, dc.SkillScopeRestricted),
			CodeGraph:          dc.CodeGraph, CodeProject: dc.CodeProject,
			LearningScope: dc.LearningScope, LearningTargetScope: dc.LearningTargetScope,
			LearningProjectValid: dc.LearningProjectValid,
			WriteTargets:         dc.WriteTargets, WorkChainID: dc.WorkChainID,
			ShortTermMem: dc.ShortTermMem, LongTermMem: dc.LongTermMem, KG: dc.KG,
			Vault: dc.Vault, Registry: dc.Registry, Manifest: dc.Manifest,
			InventoryDB: dc.InventoryDB, InvasionDB: dc.InvasionDB,
			CheatsheetDB: dc.CheatsheetDB, ImageGalleryDB: dc.ImageGalleryDB,
			MediaRegistryDB: dc.MediaRegistryDB, HomepageRegistryDB: dc.HomepageRegistryDB,
			ContactsDB: dc.ContactsDB, PlannerDB: dc.PlannerDB,
			SQLConnectionsDB: dc.SQLConnectionsDB, SQLConnectionPool: dc.SQLConnectionPool,
			RemoteHub: dc.RemoteHub, MissionManagerV2: dc.MissionManagerV2,
			DaemonSupervisor: dc.DaemonSupervisor, WebhookManager: dc.WebhookManager,
			WorkspaceSearch: dc.WorkspaceSearch, PreparationService: dc.PreparationService,
			BudgetTracker: dc.BudgetTracker,
		},
	}
}

func delegatedScope(allowed map[string]struct{}, restricted bool) []string {
	if !restricted {
		return nil
	}
	// An empty but non-nil slice is an explicit deny-all scope.
	names := make([]string, 0, len(allowed))
	for name := range allowed {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// runTaskIntent selects work context, not authorization. Delegated model text
// must never replace UserIntent or become authenticated HumanInput.
func runTaskIntent(run RunConfig) string {
	if run.DelegatedTask != "" {
		return run.DelegatedTask
	}
	return run.UserIntent
}

func delegatedAuthorizingIntent(run RunConfig) *string {
	if run.DelegatedTask == "" {
		return nil
	}
	intent := run.UserIntent
	return &intent // A missing human request remains explicitly empty.
}
