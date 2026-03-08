package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrackedProject represents a GitHub project tracked by the agent.
type TrackedProject struct {
	Name      string `json:"name"`
	Purpose   string `json:"purpose"`
	RepoURL   string `json:"repo_url"`
	CloneURL  string `json:"clone_url"`
	Owner     string `json:"owner"`
	Private   bool   `json:"private"`
	CreatedAt string `json:"created_at"`
	LocalPath string `json:"local_path"`
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

// GitHubTrackProject adds a project to the local tracker.
// Called automatically when a repo is created via the agent.
func GitHubTrackProject(workspaceDir, name, purpose, repoURL, cloneURL, owner string, private bool) string {
	if name == "" || purpose == "" {
		return errJSON("Project name and purpose are required")
	}

	projectsMu.Lock()
	defer projectsMu.Unlock()

	projects, err := loadProjects(workspaceDir)
	if err != nil {
		return errJSON("Failed to load projects: %v", err)
	}

	// Check for duplicate
	for _, p := range projects {
		if p.Name == name {
			return errJSON("Project '%s' is already tracked", name)
		}
	}

	project := TrackedProject{
		Name:      name,
		Purpose:   purpose,
		RepoURL:   repoURL,
		CloneURL:  cloneURL,
		Owner:     owner,
		Private:   private,
		CreatedAt: time.Now().Format(time.RFC3339),
		LocalPath: filepath.Join("github", name),
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
		"status":     "ok",
		"message":    fmt.Sprintf("Project '%s' tracked", name),
		"name":       project.Name,
		"purpose":    project.Purpose,
		"local_path": project.LocalPath,
	})
	return string(out)
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
