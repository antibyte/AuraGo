package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TrackedProject represents a GitHub project tracked by the agent.
type TrackedProject struct {
	Name         string `json:"name"`
	FullName     string `json:"full_name,omitempty"`
	Purpose      string `json:"purpose"`
	RepoURL      string `json:"repo_url"`
	CloneURL     string `json:"clone_url"`
	Owner        string `json:"owner"`
	Private      bool   `json:"private"`
	AgentCreated bool   `json:"agent_created,omitempty"`
	CreatedAt    string `json:"created_at"`
	LocalPath    string `json:"local_path"`
}

var projectsMu sync.Mutex

// projectsFilePath returns the path to the projects tracker JSON file.
func projectsFilePath(workspaceDir string) string {
	return filepath.Join(workspaceDir, "github", "projects.json")
}

// loadProjects reads the tracked projects from disk.
func loadProjects(workspaceDir string) ([]TrackedProject, error) {
	path := projectsFilePath(workspaceDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var projects []TrackedProject
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// saveProjects writes the tracked projects to disk.
func saveProjects(workspaceDir string, projects []TrackedProject) error {
	dir := filepath.Dir(projectsFilePath(workspaceDir))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(projectsFilePath(workspaceDir), data, 0644)
}

// GitHubTrackProject adds a manually tracked project to the local tracker.
// Manual tracking is local inventory only and does not grant remote repo access.
func GitHubTrackProject(workspaceDir, name, purpose, repoURL, cloneURL, owner string, private bool) string {
	return gitHubTrackProject(workspaceDir, name, purpose, repoURL, cloneURL, owner, private, "", false)
}

// GitHubTrackCreatedProject records a repository that AuraGo created via GitHubCreateRepo.
func GitHubTrackCreatedProject(workspaceDir, name, purpose, repoURL, cloneURL, owner string, private bool, fullName string) string {
	return gitHubTrackProject(workspaceDir, name, purpose, repoURL, cloneURL, owner, private, fullName, true)
}

func gitHubTrackProject(workspaceDir, name, purpose, repoURL, cloneURL, owner string, private bool, fullName string, agentCreated bool) string {
	if name == "" || purpose == "" {
		return errJSON("Project name and purpose are required")
	}
	owner = strings.TrimSpace(owner)
	name = strings.TrimSpace(name)
	fullName = strings.TrimSpace(fullName)
	if fullName == "" && owner != "" {
		fullName = owner + "/" + name
	}
	if fullName != "" {
		fullOwner, fullRepo := GitHubSplitFullName(fullName)
		if fullOwner != "" {
			owner = fullOwner
		}
		if fullRepo != "" {
			name = fullRepo
		}
		fullName = canonicalDisplayRepo(owner, name)
	}

	projectsMu.Lock()
	defer projectsMu.Unlock()

	projects, err := loadProjects(workspaceDir)
	if err != nil {
		return errJSON("Failed to load projects: %v", err)
	}

	canonicalNew := GitHubCanonicalRepo(owner, name)

	// Check for duplicate
	for _, p := range projects {
		canonicalExisting := projectCanonicalRepo(p)
		if (canonicalNew != "" && canonicalExisting != "" && strings.EqualFold(canonicalExisting, canonicalNew)) ||
			(canonicalNew == "" && strings.EqualFold(p.Name, name)) {
			return errJSON("Project '%s' is already tracked", name)
		}
	}

	project := TrackedProject{
		Name:         name,
		FullName:     fullName,
		Purpose:      purpose,
		RepoURL:      repoURL,
		CloneURL:     cloneURL,
		Owner:        owner,
		Private:      private,
		AgentCreated: agentCreated,
		CreatedAt:    time.Now().Format(time.RFC3339),
		LocalPath:    filepath.Join("github", name),
	}

	projects = append(projects, project)
	if err := saveProjects(workspaceDir, projects); err != nil {
		return errJSON("Failed to save project: %v", err)
	}

	// Create local project directory
	localDir := filepath.Join(workspaceDir, "github", name)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return errJSON("Failed to create local directory: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"message":       fmt.Sprintf("Project '%s' tracked", name),
		"name":          project.Name,
		"full_name":     project.FullName,
		"purpose":       project.Purpose,
		"agent_created": project.AgentCreated,
		"local_path":    project.LocalPath,
	})
	return string(out)
}

// GitHubTrustedProjectRepos returns canonical owner/repo names for agent-created projects.
func GitHubTrustedProjectRepos(workspaceDir string) []string {
	projectsMu.Lock()
	defer projectsMu.Unlock()

	projects, err := loadProjects(workspaceDir)
	if err != nil {
		return nil
	}

	var repos []string
	for _, project := range projects {
		if !project.AgentCreated {
			continue
		}
		if canonical := projectCanonicalRepo(project); canonical != "" {
			repos = append(repos, canonical)
		}
	}
	return repos
}

// GitHubTrustedProjectMap returns trusted agent-created repos keyed by canonical owner/repo.
func GitHubTrustedProjectMap(workspaceDir string) map[string]bool {
	repos := GitHubTrustedProjectRepos(workspaceDir)
	out := make(map[string]bool, len(repos))
	for _, repo := range repos {
		out[repo] = true
	}
	return out
}

func projectCanonicalRepo(project TrackedProject) string {
	if project.FullName != "" {
		owner, repo := GitHubSplitFullName(project.FullName)
		if owner != "" {
			return GitHubCanonicalRepo(owner, repo)
		}
	}
	return GitHubCanonicalRepo(project.Owner, project.Name)
}

func canonicalDisplayRepo(owner, repo string) string {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return ""
	}
	return owner + "/" + repo
}

// GitHubListProjects returns all tracked projects.
func GitHubListProjects(workspaceDir string) string {
	projectsMu.Lock()
	defer projectsMu.Unlock()

	projects, err := loadProjects(workspaceDir)
	if err != nil {
		return errJSON("Failed to load projects: %v", err)
	}

	if projects == nil {
		projects = []TrackedProject{}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"count":    len(projects),
		"projects": projects,
	})
	return string(out)
}

// GitHubUntrackProject removes a project from the tracker (does NOT delete the repo).
func GitHubUntrackProject(workspaceDir, name string) string {
	if name == "" {
		return errJSON("Project name is required")
	}

	projectsMu.Lock()
	defer projectsMu.Unlock()

	projects, err := loadProjects(workspaceDir)
	if err != nil {
		return errJSON("Failed to load projects: %v", err)
	}

	found := false
	var updated []TrackedProject
	for _, p := range projects {
		if p.Name == name {
			found = true
			continue
		}
		updated = append(updated, p)
	}

	if !found {
		return errJSON("Project '%s' not found in tracker", name)
	}

	if err := saveProjects(workspaceDir, updated); err != nil {
		return errJSON("Failed to save projects: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Project '%s' removed from tracker", name),
	})
	return string(out)
}
