package server

import (
	"aurago/internal/agent"
	"aurago/internal/codegraph"
	"aurago/internal/cydcreator"
	"aurago/internal/memory"
)

func (s *Server) configureCYDKnowledge(run *agent.RunConfig, service *cydcreator.Service, projectID, jobID string) (func(), error) {
	root, err := service.JobAnalysisRoot(jobID)
	if err != nil {
		return nil, err
	}
	p := codegraph.Project{ID: "cyd-creator-job:" + jobID, Root: root}
	run.CodeProject = p
	run.LearningScope = memory.LearningScope{Project: "cyd-creator:" + projectID}
	run.WorkChainID = p.ID
	run.AllowProjectLearnings = true
	s.knowledge.mu.Lock()
	if s.knowledge.jobs == nil {
		s.knowledge.jobs = map[string]codegraph.Project{}
	}
	s.knowledge.jobs[p.ID] = p
	s.knowledge.mu.Unlock()
	run.LearningTargetScope = func(action, resource string) (memory.LearningScope, bool) {
		if action != cydcreator.ToolWriteFile && action != cydcreator.ToolDeleteFile {
			return memory.LearningScope{}, false
		}
		if !s.authorizeGraphProject(p) {
			return memory.LearningScope{}, false
		}
		current, err := service.JobAnalysisRoot(jobID)
		if err != nil || current != root {
			return memory.LearningScope{}, false
		}
		if _, err := codegraph.Fingerprint(p, resource); err != nil {
			return memory.LearningScope{}, false
		}
		return memory.LearningScope{Project: "cyd-creator:" + projectID, Resource: resource}, true
	}
	return func() {
		s.knowledge.mu.Lock()
		delete(s.knowledge.jobs, p.ID)
		s.knowledge.mu.Unlock()
		if run.CodeGraph != nil {
			run.CodeGraph.Forget(p.ID)
		}
	}, nil
}
