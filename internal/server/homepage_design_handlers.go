package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/designs"
	"aurago/internal/tools"
)

// maxHomepageDesignRequestBytes caps the design selection request body.
const maxHomepageDesignRequestBytes = 8 * 1024

// homepageDesignSelection is the body of a design selection request.
type homepageDesignSelection struct {
	ProjectDir string `json:"project_dir"`
	DesignID   string `json:"design_id"`
}

// maxHomepageDesignPromptChars caps the design guidance that is injected into
// a Homepage Studio turn. The head of a DESIGN.md carries the overview,
// colors and typography, so a head-truncation keeps the decisive parts.
const maxHomepageDesignPromptChars = 6000

// homepageStudioDesignPrompt renders the selected design template as agent
// guidance. An unknown or empty template id yields no guidance.
func homepageStudioDesignPrompt(designID string) string {
	designID = strings.TrimSpace(designID)
	if designID == "" {
		return ""
	}
	if _, ok := designs.Lookup(designID); !ok {
		return ""
	}
	prompt, err := designs.Prompt(designID)
	if err != nil {
		return ""
	}
	prompt = strings.TrimSpace(prompt)
	if len(prompt) <= maxHomepageDesignPromptChars {
		return prompt
	}
	cut := prompt[:maxHomepageDesignPromptChars]
	if idx := strings.LastIndex(cut, "\n## "); idx > 0 {
		cut = cut[:idx]
	}
	return strings.TrimSpace(cut) + "\n\n[design system truncated]"
}

// handleHomepageDesigns returns the bundled design template catalog plus the
// template currently bound to the requested project.
func handleHomepageDesigns(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		selected := ""
		projectDir := strings.TrimSpace(r.URL.Query().Get("project_dir"))
		if projectDir != "" && s.HomepageRegistryDB != nil {
			if id, err := tools.GetHomepageProjectDesign(s.HomepageRegistryDB, projectDir); err == nil {
				selected = id
			}
		}
		payload := map[string]interface{}{
			"designs":         designs.Catalog(),
			"selected_design": selected,
			"project_dir":     projectDir,
		}
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			s.Logger.Warn("[HomepageDesigns] failed to encode catalog", "error", err)
		}
	}
}

// handleHomepageDesignByID serves one design template: its metadata with the
// full DESIGN.md body, or its preview image.
func handleHomepageDesignByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/homepage/designs/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		id := strings.TrimSpace(parts[0])
		design, ok := designs.Lookup(id)
		if !ok {
			jsonError(w, "Unknown design template", http.StatusNotFound)
			return
		}

		if len(parts) == 2 && parts[1] == "preview" {
			data, ok := designs.Preview(design.ID, r.URL.Query().Get("variant"))
			if !ok {
				jsonError(w, "Unknown preview variant", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			if _, err := w.Write(data); err != nil {
				s.Logger.Warn("[HomepageDesigns] failed to write preview", "id", design.ID, "error", err)
			}
			return
		}
		if len(parts) != 1 {
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}

		content, ok := designs.Content(design.ID)
		if !ok {
			jsonError(w, "Design template has no content", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]interface{}{
			"design":  design,
			"content": content,
		}
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			s.Logger.Warn("[HomepageDesigns] failed to encode design", "id", design.ID, "error", err)
		}
	}
}

// handleHomepageDesignSelect binds a design template to a homepage project.
// The template is written into the project as DESIGN.md so the agent applies
// it while working on that site, and the binding is stored in the registry.
func handleHomepageDesignSelect(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxHomepageDesignRequestBytes))
		if err != nil {
			jsonError(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		var selection homepageDesignSelection
		if err := json.Unmarshal(body, &selection); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		design, ok := designs.Lookup(selection.DesignID)
		if !ok {
			jsonError(w, "Unknown design template", http.StatusBadRequest)
			return
		}
		projectDir := strings.TrimSpace(selection.ProjectDir)

		applied := false
		registered := false
		if projectDir != "" {
			cfg := homepageConfigFromServer(s)
			if err := tools.ApplyHomepageDesign(cfg, projectDir, design.ID); err != nil {
				s.Logger.Warn("[HomepageDesigns] failed to write project design", "project_dir", projectDir, "design", design.ID, "error", err)
				jsonError(w, "Failed to apply the design to the project", http.StatusInternalServerError)
				return
			}
			applied = true
			if s.HomepageRegistryDB != nil {
				if err := tools.SetHomepageProjectDesign(s.HomepageRegistryDB, projectDir, design.ID); err != nil {
					s.Logger.Warn("[HomepageDesigns] failed to store project design", "project_dir", projectDir, "design", design.ID, "error", err)
				} else {
					registered = true
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		payload := map[string]interface{}{
			"status":      "ok",
			"design":      design,
			"project_dir": projectDir,
			"applied":     applied,
			"registered":  registered,
		}
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			s.Logger.Warn("[HomepageDesigns] failed to encode selection", "error", err)
		}
	}
}
