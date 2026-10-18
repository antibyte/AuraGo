package server

import (
	"context"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/desktopstore"
	"aurago/internal/infra"
	"aurago/internal/tools"
)

type desktopInfraAppTarget struct {
	App      string `json:"app"`
	TargetID string `json:"target_id"`
	Label    string `json:"label"`
}

type desktopInfraTargetsResponse struct {
	Targets []desktopInfraAppTarget `json:"targets"`
}

// desktopInfraTargets exposes only targets backed by an existing configured
// application object. The values are identifiers, never caller-supplied URLs.
func desktopInfraTargets(ctx context.Context, s *Server, cfg *config.Config) ([]desktopInfraAppTarget, map[string]string, error) {
	targets := make([]desktopInfraAppTarget, 0)
	storeAppsByContainer := make(map[string]string)
	if cfg == nil {
		return targets, storeAppsByContainer, nil
	}

	if cfg.Go2RTC.Enabled {
		for _, stream := range cfg.Go2RTC.Streams {
			id := strings.TrimSpace(stream.ID)
			if !stream.Enabled || (!stream.SourceConfigured && strings.TrimSpace(stream.Source) == "") || tools.ValidateGo2RTCStreamID(id) != nil {
				continue
			}
			label := desktopInfraText(stream.Name, 120)
			if label == "" {
				label = id
			}
			targets = append(targets, desktopInfraAppTarget{App: "network-cameras", TargetID: id, Label: label})
		}
	}

	if s != nil && strings.TrimSpace(cfg.Directories.DataDir) != "" {
		desktopCfg := desktop.ConfigFromAuraConfig(cfg)
		normalizedDesktopCfg, err := desktop.NormalizeConfig(desktopCfg)
		if err != nil {
			return nil, nil, err
		}
		apps, err := desktopstore.ReadInfraTargets(ctx, filepath.Join(normalizedDesktopCfg.DataDir, "desktop_store.db"))
		if err != nil {
			return nil, nil, err
		}
		for _, app := range apps {
			appID := strings.TrimSpace(app.AppID)
			containerID := strings.TrimSpace(app.ContainerID)
			if appID == "" || containerID == "" {
				continue
			}
			targets = append(targets, desktopInfraAppTarget{App: "software-store", TargetID: appID, Label: desktopInfraText(appID, 120)})
			storeAppsByContainer[strings.ToLower(containerID)] = appID
		}
	}

	if cfg.MeshCore.Enabled && s != nil && s.MeshCore != nil {
		conversations, err := s.MeshCore.Conversations()
		if err == nil {
			for _, conversation := range conversations {
				id := strings.TrimSpace(conversation.ID)
				if id == "" {
					continue
				}
				label := desktopInfraText(conversation.Name, 120)
				if label == "" {
					label = "MeshCore conversation"
				}
				targets = append(targets, desktopInfraAppTarget{App: "meshcore", TargetID: id, Label: label})
			}
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].App != targets[j].App {
			return targets[i].App < targets[j].App
		}
		return targets[i].TargetID < targets[j].TargetID
	})
	return targets, storeAppsByContainer, nil
}

func desktopInfraTargetsHandler(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config) {
	if r.Method != "GET" {
		desktopInfraMethodNotAllowed(w)
		return
	}
	ctx, cancel := desktopInfraContext(r.Context())
	defer cancel()
	targets, _, err := desktopInfraTargets(ctx, s, cfg)
	if err != nil {
		desktopInfraError(w, 503, "target_list_unavailable", "Available app targets could not be read")
		return
	}
	if targets == nil {
		targets = []desktopInfraAppTarget{}
	}
	desktopInfraJSON(w, 200, desktopInfraTargetsResponse{Targets: targets})
}

func desktopInfraValidateAppBindings(ctx context.Context, s *Server, cfg *config.Config, bindings, existing []infra.AppLink) error {
	if len(bindings) > 32 {
		return infra.ErrValidation
	}
	if len(bindings) == 0 {
		return nil
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, binding := range existing {
		existingSet[binding.App+"\x00"+binding.TargetID] = struct{}{}
	}
	targets, _, err := desktopInfraTargets(ctx, s, cfg)
	allowed := make(map[string]struct{}, len(targets))
	if err == nil {
		for _, target := range targets {
			allowed[target.App+"\x00"+target.TargetID] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding.App != "network-cameras" && binding.App != "software-store" && binding.App != "meshcore" {
			return infra.ErrValidation
		}
		key := binding.App + "\x00" + binding.TargetID
		if binding.TargetID == "" {
			return infra.ErrValidation
		}
		if _, ok := allowed[key]; !ok {
			if _, previouslyValidated := existingSet[key]; previouslyValidated {
				if _, duplicate := seen[key]; duplicate {
					return infra.ErrValidation
				}
				seen[key] = struct{}{}
				continue
			}
			if err != nil {
				return err
			}
			return infra.ErrValidation
		}
		if _, ok := seen[key]; ok {
			return infra.ErrValidation
		}
		seen[key] = struct{}{}
	}
	return nil
}

func desktopInfraCheckAppBindings(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, bindings, existing []infra.AppLink) bool {
	ctx, cancel := desktopInfraContext(r.Context())
	defer cancel()
	if err := desktopInfraValidateAppBindings(ctx, s, cfg, bindings, existing); err != nil {
		if err == infra.ErrValidation {
			desktopInfraError(w, 400, "invalid_app_binding", "An app target is invalid or unavailable")
		} else {
			desktopInfraError(w, 503, "app_targets_unavailable", "App targets could not be verified")
		}
		return false
	}
	return true
}

func desktopInfraDecorateSnapshot(ctx context.Context, s *Server, cfg *config.Config, snapshot *infra.Snapshot) {
	if snapshot == nil {
		return
	}
	snapshot.Sources = desktopInfraProjectSourceStates(cfg, snapshot.Sources)
	snapshot.Nodes = desktopInfraProjectNodes(ctx, s, cfg, desktopInfraVisibleNodes(snapshot))
	if snapshot.Nodes == nil {
		snapshot.Nodes = []infra.Node{}
	}
}

func desktopInfraProjectSourceStates(cfg *config.Config, sources []infra.Source) []infra.Source {
	if sources == nil {
		return []infra.Source{}
	}
	projected := make([]infra.Source, len(sources))
	copy(projected, sources)
	for i := range projected {
		if projected[i].State == infra.SourceCurrent && (cfg == nil || !desktopInfraSourceConfigured(cfg, projected[i])) {
			// Keep safe observations and fetched_at for display, but never claim
			// that a source disabled or replaced in config is currently verified.
			projected[i].State = infra.SourceStale
		}
	}
	return projected
}

func desktopInfraProjectNodes(ctx context.Context, s *Server, cfg *config.Config, nodes []infra.Node) []infra.Node {
	targets, storeAppsByContainer, err := desktopInfraTargets(ctx, s, cfg)
	if err != nil {
		targets = nil
		storeAppsByContainer = nil
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target.App+"\x00"+target.TargetID] = struct{}{}
	}
	if nodes == nil {
		return []infra.Node{}
	}
	for i := range nodes {
		nodes[i] = desktopInfraProjectNode(nodes[i], cfg, targetSet, storeAppsByContainer)
	}
	return nodes
}

func desktopInfraDecorateNode(ctx context.Context, s *Server, cfg *config.Config, node infra.Node) infra.Node {
	projected := desktopInfraProjectNodes(ctx, s, cfg, []infra.Node{node})
	return projected[0]
}

func desktopInfraProjectNode(node infra.Node, cfg *config.Config, targets map[string]struct{}, storeAppsByContainer map[string]string) infra.Node {
	node.Sources = desktopInfraProjectSourceStates(cfg, node.Sources)
	node.AvailableActions, node.ActionReasons, node.SourceActions, node.SourceActionReasons = desktopInfraActionProjection(node, cfg)
	node.AppLinks = make([]infra.AppLink, 0, len(node.Profile.AppBindings)+2)
	seen := make(map[string]struct{}, len(node.Profile.AppBindings)+2)
	add := func(link infra.AppLink) {
		key := link.App + "\x00" + link.TargetID
		if link.App == "" || link.TargetID == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		node.AppLinks = append(node.AppLinks, link)
	}
	for _, source := range node.Sources {
		if source.Kind == "registry" && source.Key == node.RegistryID && source.State == infra.SourceCurrent && node.RegistryID != "" && desktopInfraSourceConfigured(cfg, source) {
			add(infra.AppLink{App: "quick-connect", TargetID: node.RegistryID})
			break
		}
	}
	for _, binding := range node.Profile.AppBindings {
		if _, ok := targets[binding.App+"\x00"+binding.TargetID]; ok {
			add(binding)
		}
	}
	// A Software Store association is trusted only when the full Docker ID
	// matches the Store's persisted container ledger exactly.
	for _, source := range node.Sources {
		if source.Kind != "docker" || source.State != infra.SourceCurrent || !desktopInfraSourceConfigured(cfg, source) {
			continue
		}
		if appID, ok := storeAppsByContainer[strings.ToLower(strings.TrimSpace(source.Key))]; ok {
			if _, current := targets["software-store\x00"+appID]; current {
				add(infra.AppLink{App: "software-store", TargetID: appID})
			}
			break
		}
	}
	node.Power = desktopInfraProjectPower(node.Sources)
	return node
}

func desktopInfraProjectPower(sources []infra.Source) string {
	chosen := ""
	for _, source := range sources {
		if source.State != infra.SourceCurrent || source.Observed.Power == "" {
			continue
		}
		if chosen == "" {
			chosen = source.Observed.Power
			continue
		}
		if chosen != source.Observed.Power {
			return infra.PowerUnknown
		}
	}
	return chosen
}

func desktopInfraActionProjection(node infra.Node, cfg *config.Config) ([]string, map[string]string, map[string][]string, map[string]map[string]string) {
	allowed := make(map[string]struct{})
	reasons := make(map[string]string)
	sourceActions := make(map[string][]string)
	sourceReasons := make(map[string]map[string]string)
	configuredSourceCount := make(map[string]int)
	for _, source := range node.Sources {
		if len(desktopInfraSourceActions(source)) > 0 && desktopInfraSourceConfigured(cfg, source) {
			configuredSourceCount[source.Kind]++
		}
	}
	addSourceReason := func(sourceKey, action, reason string) {
		perSource := sourceReasons[sourceKey]
		if perSource == nil {
			perSource = make(map[string]string)
			sourceReasons[sourceKey] = perSource
		}
		if _, already := perSource[action]; !already || reason == "managed_container" {
			perSource[action] = reason
		}
		if _, exists := allowed[action]; !exists {
			if _, already := reasons[action]; !already || reason == "managed_container" {
				reasons[action] = reason
			}
		}
	}
	for _, source := range node.Sources {
		actions := desktopInfraSourceActions(source)
		if len(actions) == 0 {
			continue
		}
		sourceKey := infra.CandidateID(source.SourceIdentity)
		if !desktopInfraSourceConfigured(cfg, source) {
			for _, action := range actions {
				addSourceReason(sourceKey, action, "source_disabled")
			}
			continue
		}
		if source.State != infra.SourceCurrent {
			for _, action := range actions {
				addSourceReason(sourceKey, action, "source_stale")
			}
			continue
		}
		if configuredSourceCount[source.Kind] > 1 {
			for _, action := range actions {
				addSourceReason(sourceKey, action, "source_policy")
			}
			continue
		}
		if source.Kind == "docker" && source.Observed.Protected {
			for _, action := range actions {
				addSourceReason(sourceKey, action, "managed_container")
			}
			continue
		}
		if !desktopInfraCapabilities(cfg).Control {
			for _, action := range actions {
				addSourceReason(sourceKey, action, "control_disabled")
			}
			continue
		}
		perSourceAllowed := make(map[string]struct{})
		for _, action := range actions {
			if desktopInfraActionAllowed(cfg, source.Kind, action) {
				allowed[action] = struct{}{}
				perSourceAllowed[action] = struct{}{}
				delete(reasons, action)
				continue
			}
			reason := "source_policy"
			switch source.Kind {
			case "docker":
				if cfg.Docker.ReadOnly {
					reason = "source_readonly"
				} else {
					reason = "source_disabled"
				}
			case "proxmox":
				if cfg.Proxmox.ReadOnly {
					reason = "source_readonly"
				} else if !cfg.Proxmox.AllowDestructive && (action == "shutdown" || action == "reboot") {
					reason = "source_policy"
				} else {
					reason = "source_disabled"
				}
			case "fritzbox":
				if cfg.FritzBox.Network.ReadOnly {
					reason = "source_readonly"
				} else {
					reason = "source_disabled"
				}
			}
			addSourceReason(sourceKey, action, reason)
		}
		for _, action := range []string{"start", "stop", "restart", "shutdown", "reboot", "wake"} {
			if _, ok := perSourceAllowed[action]; ok {
				sourceActions[sourceKey] = append(sourceActions[sourceKey], action)
			}
		}
	}
	result := make([]string, 0, len(allowed))
	for _, action := range []string{"start", "stop", "restart", "shutdown", "reboot", "wake"} {
		if _, ok := allowed[action]; ok {
			result = append(result, action)
		}
	}
	if len(sourceActions) == 0 {
		sourceActions = nil
	}
	if len(sourceReasons) == 0 {
		sourceReasons = nil
	}
	return result, reasons, sourceActions, sourceReasons
}

func desktopInfraSourceActions(source infra.Source) []string {
	switch source.Kind {
	case "docker":
		return []string{"start", "stop", "restart"}
	case "proxmox":
		if !strings.HasPrefix(source.Key, "node:") {
			return []string{"start", "shutdown", "reboot"}
		}
	case "fritzbox":
		return []string{"wake"}
	}
	return nil
}
