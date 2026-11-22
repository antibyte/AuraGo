package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aurago/internal/agent"
	"aurago/internal/codegraph"
	"aurago/internal/memory"
	"aurago/internal/services"
)

type projectKnowledge struct {
	mu      sync.Mutex
	graph   *codegraph.Manager
	targets services.WriteTargets
	closed  bool
	jobs    map[string]codegraph.Project
}

func (s *Server) codeGraphService() *codegraph.Manager {
	s.knowledge.mu.Lock()
	defer s.knowledge.mu.Unlock()
	if s.knowledge.closed {
		return nil
	}
	if s.knowledge.graph == nil {
		ctx := s.integrationCtx
		if ctx == nil {
			ctx = context.Background()
		}
		s.knowledge.graph = codegraph.New(ctx, func() bool { cfg := s.ConfigSnapshot(); return cfg != nil && cfg.CodeGraph.Enabled }, s.authorizeGraphProject, s.revalidateGraphResource)
	}
	return s.knowledge.graph
}

func (s *Server) revalidateGraphResource(ctx context.Context, p codegraph.Project, file, fingerprint string) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Learnings.Enabled || !cfg.Tools.Memory.Enabled || cfg.Tools.Memory.ReadOnly || s.ShortTermMem == nil || strings.HasPrefix(p.ID, "game-maker-job:") {
		return
	}
	if err := s.ShortTermMem.RevalidateLearningResource(ctx, memory.LearningScope{Project: p.ID, Resource: file}, fingerprint); err != nil && s.Logger != nil {
		s.Logger.Warn("Learning resource revalidation failed", "error", err)
	}
}

func (s *Server) authorizeGraphProject(p codegraph.Project) bool {
	if strings.HasPrefix(p.ID, "game-maker-job:") || strings.HasPrefix(p.ID, "cyd-creator-job:") {
		cfg := s.ConfigSnapshot()
		s.knowledge.mu.Lock()
		registered := s.knowledge.jobs[p.ID]
		s.knowledge.mu.Unlock()
		canonical, rootErr := filepath.EvalSymlinks(p.Root)
		enabled := cfg != nil && ((strings.HasPrefix(p.ID, "game-maker-job:") && cfg.GameMaker.Enabled) || (strings.HasPrefix(p.ID, "cyd-creator-job:") && cfg.CYDCreator.Enabled))
		return enabled && registered == p && rootErr == nil && filepath.Clean(canonical) == filepath.Clean(p.Root)
	}
	current, _, _, err := s.resolveKnowledgeProject(context.Background(), p.ID, "")
	return err == nil && current == p
}

// Job roots originate from the Game Maker service, never model or browser arguments.
func (s *Server) configureGameKnowledge(run *agent.RunConfig, projectID, jobID, root string) func() {
	run.LearningScope = memory.LearningScope{Project: "game-maker:" + projectID}
	run.WorkChainID = "game-maker:" + jobID
	run.AllowProjectLearnings = true
	p := codegraph.Project{ID: "game-maker-job:" + jobID, Root: root}
	run.CodeProject = p
	baseResolver := run.LearningTargetScope
	run.LearningTargetScope = func(action, resource string) (memory.LearningScope, bool) {
		if action == "game_maker_file" {
			if !s.authorizeGraphProject(p) {
				return memory.LearningScope{}, false
			}
			if _, err := codegraph.Fingerprint(p, resource); err != nil {
				return memory.LearningScope{}, false
			}
			return memory.LearningScope{Project: "game-maker:" + projectID, Resource: resource}, true
		}
		if baseResolver != nil {
			return baseResolver(action, resource)
		}
		return memory.LearningScope{}, false
	}
	s.knowledge.mu.Lock()
	if s.knowledge.jobs == nil {
		s.knowledge.jobs = make(map[string]codegraph.Project)
	}
	s.knowledge.jobs[p.ID] = p
	s.knowledge.mu.Unlock()
	return func() {
		s.knowledge.mu.Lock()
		delete(s.knowledge.jobs, p.ID)
		s.knowledge.mu.Unlock()
		if run.CodeGraph != nil {
			run.CodeGraph.Forget(p.ID)
		}
	}
}

func (s *Server) closeProjectKnowledge() {
	s.knowledge.mu.Lock()
	s.knowledge.closed = true
	graph := s.knowledge.graph
	s.knowledge.graph = nil
	s.knowledge.mu.Unlock()
	if graph != nil {
		graph.Close()
	}
	s.knowledge.targets.Clear()
}

func knowledgeRootID(kind, root string) string {
	canonical := filepath.Clean(root)
	if filepath.Separator == '\\' {
		canonical = strings.ToLower(canonical)
	}
	hash := sha256.Sum256([]byte(canonical))
	return kind + ":" + hex.EncodeToString(hash[:8])
}

func (s *Server) resolveKnowledgeProject(ctx context.Context, id, resource string) (codegraph.Project, memory.LearningScope, string, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("configuration unavailable")
	}
	root, kind := "", ""
	switch {
	case id == "workspace" || strings.HasPrefix(id, "workspace:"):
		root, kind = cfg.Directories.WorkspaceDir, "workspace"
	case id == "code-studio" || strings.HasPrefix(id, "code-studio:"):
		if !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.CodeStudio.Enabled {
			return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("project disabled")
		}
		root, kind = cfg.VirtualDesktop.WorkspaceDir, "code-studio"
	case id == "homepage" || strings.HasPrefix(id, "homepage:"):
		if !cfg.Homepage.Enabled {
			return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("project disabled")
		}
		root, kind = cfg.Homepage.WorkspacePath, "homepage"
	case strings.HasPrefix(id, "game-maker:"):
		if !cfg.GameMaker.Enabled || s.GameMaker == nil {
			return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("project disabled")
		}
		project, err := s.GameMaker.GetProject(ctx, strings.TrimPrefix(id, "game-maker:"))
		if err != nil {
			return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("project unavailable")
		}
		root, kind = filepath.Join(cfg.GameMaker.WorkspacePath, filepath.FromSlash(project.ProjectKey)), id
	default:
		return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("unregistered project")
	}
	if strings.TrimSpace(root) == "" {
		return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("workspace unavailable")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return codegraph.Project{}, memory.LearningScope{}, "", err
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("workspace unavailable")
	}
	projectID := kind
	if !strings.HasPrefix(kind, "game-maker:") {
		projectID = knowledgeRootID(kind, canonical)
		if strings.Contains(id, ":") && id != projectID {
			return codegraph.Project{}, memory.LearningScope{}, "", fmt.Errorf("workspace registration changed")
		}
	}
	p := codegraph.Project{ID: projectID, Root: canonical}
	scope := memory.LearningScope{Project: projectID}
	resource = strings.TrimPrefix(strings.ReplaceAll(resource, "\\", "/"), "/workspace/")
	if resource == "" {
		return p, scope, "", nil
	}
	if filepath.IsAbs(resource) || strings.Contains(resource, ":") {
		return p, scope, "", fmt.Errorf("invalid project resource")
	}
	resource = filepath.ToSlash(filepath.Clean(resource))
	if resource == "." || resource == ".." || strings.HasPrefix(resource, "../") {
		return p, scope, "", fmt.Errorf("invalid project resource")
	}
	r, err := os.OpenRoot(canonical)
	if err != nil {
		return p, scope, "", fmt.Errorf("workspace unavailable")
	}
	defer r.Close()
	parts := strings.Split(resource, "/")
	for i := range parts {
		info, e := r.Lstat(strings.Join(parts[:i+1], "/"))
		if os.IsNotExist(e) {
			break
		}
		if e != nil || info.Mode()&os.ModeSymlink != 0 {
			return p, scope, "", fmt.Errorf("resource boundary unavailable")
		}
	}
	scope.Resource = resource
	fingerprint, _ := codegraph.Fingerprint(p, resource)
	return p, scope, fingerprint, nil
}

func (s *Server) configureRunKnowledge(run *agent.RunConfig) {
	if s == nil || run.Config == nil || (!run.Config.Learnings.Enabled && !run.Config.CodeGraph.Enabled) {
		return
	}
	if run.Config.CodeGraph.Enabled {
		run.CodeGraph = s.codeGraphService()
	}
	if run.Config.Learnings.Enabled {
		run.LearningProjectValid = s.authorizeGraphProject
		run.WriteTargets = &s.knowledge.targets
		if run.LearningTargetScope == nil {
			run.LearningTargetScope = func(action, resource string) (memory.LearningScope, bool) {
				project := "workspace"
				switch action {
				case "virtual_desktop", "virtual_desktop_files":
					project = "code-studio"
				case "homepage", "homepage_file":
					project = "homepage"
				case "game_maker_file", "cyd_creator_write_file", "cyd_creator_delete_file":
					return memory.LearningScope{}, false
				}
				p, scope, fingerprint, err := s.resolveKnowledgeProject(context.Background(), project, resource)
				if err == nil && fingerprint != "" {
					s.revalidateGraphResource(context.Background(), p, resource, fingerprint)
				}
				return scope, err == nil
			}
		}
	}
	if p, _, _, err := s.resolveKnowledgeProject(context.Background(), "workspace", ""); err == nil && run.CodeProject.ID == "" {
		run.CodeProject = p
	}
}
