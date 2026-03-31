package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// HomepageProject represents a project tracked in the homepage registry.
type HomepageProject struct {
	ID               int64    `json:"id"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Framework        string   `json:"framework"`
	FrameworkVersion string   `json:"framework_version,omitempty"`
	URL              string   `json:"url,omitempty"`
	NetlifySiteID    string   `json:"netlify_site_id,omitempty"`
	DeployHost       string   `json:"deploy_host,omitempty"`
	ProjectDir       string   `json:"project_dir"`
	Status           string   `json:"status"` // active, archived, maintenance
	Tags             []string `json:"tags"`
	LastEditedAt     string   `json:"last_edited_at,omitempty"`
	LastEditReason   string   `json:"last_edit_reason,omitempty"`
	LastDeployedAt   string   `json:"last_deployed_at,omitempty"`
	LastDeployURL    string   `json:"last_deploy_url,omitempty"`
	KnownProblems    string   `json:"known_problems,omitempty"`
	LighthouseScore  string   `json:"lighthouse_score,omitempty"` // JSON string
	Notes            string   `json:"notes,omitempty"`
}

// InitHomepageRegistryDB initializes the homepage registry SQLite database.
func InitHomepageRegistryDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open homepage registry database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS homepage_projects (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		name              TEXT NOT NULL UNIQUE,
		description       TEXT DEFAULT '',
		framework         TEXT DEFAULT '',
		framework_version TEXT DEFAULT '',
		url               TEXT DEFAULT '',
		netlify_site_id   TEXT DEFAULT '',
		deploy_host       TEXT DEFAULT '',
		project_dir       TEXT NOT NULL DEFAULT '',
		status            TEXT NOT NULL DEFAULT 'active',
		tags              TEXT DEFAULT '[]',
		last_edited_at    DATETIME DEFAULT NULL,
		last_edit_reason  TEXT DEFAULT '',
		last_deployed_at  DATETIME DEFAULT NULL,
		last_deploy_url   TEXT DEFAULT '',
		known_problems    TEXT DEFAULT '',
		lighthouse_score  TEXT DEFAULT '',
		notes             TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_hp_name ON homepage_projects(name);
	CREATE INDEX IF NOT EXISTS idx_hp_status ON homepage_projects(status);
	CREATE INDEX IF NOT EXISTS idx_hp_project_dir ON homepage_projects(project_dir);
	CREATE INDEX IF NOT EXISTS idx_hp_last_edited ON homepage_projects(last_edited_at);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create homepage registry schema: %w", err)
	}

	return db, nil
}

// RegisterProject inserts a new homepage project. Returns existing ID if name matches.
func RegisterProject(db *sql.DB, p HomepageProject) (int64, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("homepage registry DB not initialized")
	}

	// Dedup by name
	if p.Name != "" {
		var existingID int64
		err := db.QueryRow("SELECT id FROM homepage_projects WHERE name = ?", p.Name).Scan(&existingID)
		if err == nil {
			return existingID, true, nil
		}
	}

	// Dedup by project_dir
	if p.ProjectDir != "" {
		var existingID int64
		err := db.QueryRow("SELECT id FROM homepage_projects WHERE project_dir = ? AND project_dir != ''", p.ProjectDir).Scan(&existingID)
		if err == nil {
			return existingID, true, nil
		}
	}

	if p.Status == "" {
		p.Status = "active"
	}
	tagsJSON, _ := json.Marshal(p.Tags)
	if p.Tags == nil {
		tagsJSON = []byte("[]")
	}

	res, err := db.Exec(`INSERT INTO homepage_projects
		(name, description, framework, framework_version, url, netlify_site_id, deploy_host,
		 project_dir, status, tags, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.Framework, p.FrameworkVersion, p.URL, p.NetlifySiteID,
		p.DeployHost, p.ProjectDir, p.Status, string(tagsJSON), p.Notes,
	)
	if err != nil {
		return 0, false, fmt.Errorf("failed to insert homepage project: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, false, nil
}

func scanProject(row interface {
	Scan(dest ...interface{}) error
}) (*HomepageProject, error) {
	var p HomepageProject
	var tagsStr string
	var lastEdited, lastDeployed sql.NullString
	err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt, &p.Name, &p.Description, &p.Framework,
		&p.FrameworkVersion, &p.URL, &p.NetlifySiteID, &p.DeployHost, &p.ProjectDir,
		&p.Status, &tagsStr, &lastEdited, &p.LastEditReason, &lastDeployed, &p.LastDeployURL,
		&p.KnownProblems, &p.LighthouseScore, &p.Notes)
	if err != nil {
		return nil, err
	}
	if lastEdited.Valid {
		p.LastEditedAt = lastEdited.String
	}
	if lastDeployed.Valid {
		p.LastDeployedAt = lastDeployed.String
	}
	if err := json.Unmarshal([]byte(tagsStr), &p.Tags); err != nil {
		p.Tags = []string{}
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	return &p, nil
}

const hpSelectCols = "id, created_at, updated_at, name, description, framework, framework_version, url, netlify_site_id, deploy_host, project_dir, status, tags, last_edited_at, last_edit_reason, last_deployed_at, last_deploy_url, known_problems, lighthouse_score, notes"

// GetProject retrieves a project by ID.
func GetProject(db *sql.DB, id int64) (*HomepageProject, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	row := db.QueryRow("SELECT "+hpSelectCols+" FROM homepage_projects WHERE id = ?", id)
	p, err := scanProject(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project %d not found", id)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return p, nil
}

// GetProjectByName retrieves a project by name.
func GetProjectByName(db *sql.DB, name string) (*HomepageProject, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	row := db.QueryRow("SELECT "+hpSelectCols+" FROM homepage_projects WHERE name = ?", name)
	p, err := scanProject(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return p, nil
}

// GetProjectByDir retrieves a project by project_dir.
func GetProjectByDir(db *sql.DB, dir string) (*HomepageProject, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	row := db.QueryRow("SELECT "+hpSelectCols+" FROM homepage_projects WHERE project_dir = ? AND project_dir != ''", dir)
	p, err := scanProject(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project with dir '%s' not found", dir)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return p, nil
}

// SearchProjects searches projects by query, status, and tags.
func SearchProjects(db *sql.DB, query, status string, tags []string, limit, offset int) ([]HomepageProject, int, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("homepage registry DB not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	if query != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ? OR framework LIKE ? OR url LIKE ? OR project_dir LIKE ? OR notes LIKE ?)")
		q := "%" + query + "%"
		args = append(args, q, q, q, q, q, q)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	for _, t := range tags {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%\""+t+"\"%")
	}

	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_projects WHERE "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := db.Query("SELECT "+hpSelectCols+" FROM homepage_projects WHERE "+where+" ORDER BY last_edited_at DESC NULLS LAST, created_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search projects: %w", err)
	}
	defer rows.Close()

	var projects []HomepageProject
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, *p)
	}
	return projects, total, nil
}

// ListProjects lists all projects, optionally filtered by status.
func ListProjects(db *sql.DB, status string, limit, offset int) ([]HomepageProject, int, error) {
	return SearchProjects(db, "", status, nil, limit, offset)
}

// UpdateProject updates fields for a project.
func UpdateProject(db *sql.DB, id int64, fields map[string]interface{}) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}

	allowed := map[string]bool{
		"name": true, "description": true, "framework": true, "framework_version": true,
		"url": true, "netlify_site_id": true, "deploy_host": true, "project_dir": true,
		"status": true, "tags": true, "notes": true,
	}

	var setClauses []string
	var args []interface{}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		if k == "tags" {
			if tagSlice, ok := v.([]string); ok {
				b, _ := json.Marshal(tagSlice)
				setClauses = append(setClauses, k+" = ?")
				args = append(args, string(b))
				continue
			}
		}
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC().Format("2006-01-02 15:04:05"))
	args = append(args, id)

	_, err := db.Exec("UPDATE homepage_projects SET "+strings.Join(setClauses, ", ")+" WHERE id = ?", args...)
	return err
}

// LogEdit records an edit event for a project.
func LogEdit(db *sql.DB, projectID int64, reason string) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec("UPDATE homepage_projects SET last_edited_at = ?, last_edit_reason = ?, updated_at = ? WHERE id = ?",
		now, reason, now, projectID)
	return err
}

// LogDeploy records a deployment event for a project.
func LogDeploy(db *sql.DB, projectID int64, url string) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec("UPDATE homepage_projects SET last_deployed_at = ?, last_deploy_url = ?, updated_at = ? WHERE id = ?",
		now, url, now, projectID)
	return err
}

// LogProblem appends a problem description to known_problems.
func LogProblem(db *sql.DB, projectID int64, problem string) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	var current string
	if err := db.QueryRow("SELECT known_problems FROM homepage_projects WHERE id = ?", projectID).Scan(&current); err != nil {
		return fmt.Errorf("project %d not found", projectID)
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] %s", now, problem)
	if current != "" {
		current = current + "\n" + entry
	} else {
		current = entry
	}
	_, err := db.Exec("UPDATE homepage_projects SET known_problems = ?, updated_at = ? WHERE id = ?", current, now, projectID)
	return err
}

// ResolveProblem removes a problem line (by substring match) from known_problems.
func ResolveProblem(db *sql.DB, projectID int64, problem string) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	var current string
	if err := db.QueryRow("SELECT known_problems FROM homepage_projects WHERE id = ?", projectID).Scan(&current); err != nil {
		return fmt.Errorf("project %d not found", projectID)
	}

	var remaining []string
	for _, line := range strings.Split(current, "\n") {
		if !strings.Contains(line, problem) {
			remaining = append(remaining, line)
		}
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec("UPDATE homepage_projects SET known_problems = ?, updated_at = ? WHERE id = ?",
		strings.Join(remaining, "\n"), now, projectID)
	return err
}

// DispatchHomepageRegistry handles tool calls for the homepage_registry action.
func DispatchHomepageRegistry(db *sql.DB, operation, query, name, description, framework, projectDir, url, status, reason, problem, notes string, tags []string, id int64, lighthouseScore string, limit, offset int) string {
	switch operation {
	case "register":
		if name == "" {
			return `{"status":"error","message":"'name' is required to register a project."}`
		}
		p := HomepageProject{
			Name:        name,
			Description: description,
			Framework:   framework,
			ProjectDir:  projectDir,
			URL:         url,
			Status:      status,
			Tags:        tags,
			Notes:       notes,
		}
		newID, dup, err := RegisterProject(db, p)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		if dup {
			return fmt.Sprintf(`{"status":"duplicate","id":%d,"message":"Project already exists."}`, newID)
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"Project registered."}`, newID)

	case "search":
		projects, total, err := SearchProjects(db, query, status, tags, limit, offset)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(projects)
		return fmt.Sprintf(`{"status":"success","total":%d,"projects":%s}`, total, string(b))

	case "get":
		if id > 0 {
			p, err := GetProject(db, id)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(p)
			return fmt.Sprintf(`{"status":"success","project":%s}`, string(b))
		}
		if name != "" {
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(p)
			return fmt.Sprintf(`{"status":"success","project":%s}`, string(b))
		}
		return `{"status":"error","message":"'id' or 'name' is required for get operation."}`

	case "list":
		projects, total, err := ListProjects(db, status, limit, offset)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(projects)
		return fmt.Sprintf(`{"status":"success","total":%d,"projects":%s}`, total, string(b))

	case "update":
		if id <= 0 && name != "" {
			// Resolve by name
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			id = p.ID
		}
		if id <= 0 {
			return `{"status":"error","message":"'id' or 'name' is required for update."}`
		}
		fields := map[string]interface{}{}
		if description != "" {
			fields["description"] = description
		}
		if framework != "" {
			fields["framework"] = framework
		}
		if url != "" {
			fields["url"] = url
		}
		if status != "" {
			fields["status"] = status
		}
		if projectDir != "" {
			fields["project_dir"] = projectDir
		}
		if notes != "" {
			fields["notes"] = notes
		}
		if tags != nil {
			fields["tags"] = tags
		}
		if len(fields) == 0 {
			return `{"status":"error","message":"No fields provided to update."}`
		}
		if err := UpdateProject(db, id, fields); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Project updated."}`

	case "delete":
		if id <= 0 && name != "" {
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			id = p.ID
		}
		if id <= 0 {
			return `{"status":"error","message":"'id' or 'name' is required for delete."}`
		}
		if _, err := db.Exec("DELETE FROM homepage_projects WHERE id = ?", id); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Project deleted."}`

	case "log_edit":
		if id <= 0 && name != "" {
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			id = p.ID
		}
		if id <= 0 {
			return `{"status":"error","message":"'id' or 'name' is required."}`
		}
		if reason == "" {
			return `{"status":"error","message":"'reason' is required for log_edit."}`
		}
		if err := LogEdit(db, id, reason); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Edit logged."}`

	case "log_deploy":
		if id <= 0 && name != "" {
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			id = p.ID
		}
		if id <= 0 {
			return `{"status":"error","message":"'id' or 'name' is required."}`
		}
		deployURL := url
		if err := LogDeploy(db, id, deployURL); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Deploy logged."}`

	case "log_problem":
		if id <= 0 && name != "" {
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			id = p.ID
		}
		if id <= 0 {
			return `{"status":"error","message":"'id' or 'name' is required."}`
		}
		if problem == "" {
			return `{"status":"error","message":"'problem' is required."}`
		}
		if err := LogProblem(db, id, problem); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Problem logged."}`

	case "resolve_problem":
		if id <= 0 && name != "" {
			p, err := GetProjectByName(db, name)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
			}
			id = p.ID
		}
		if id <= 0 {
			return `{"status":"error","message":"'id' or 'name' is required."}`
		}
		if problem == "" {
			return `{"status":"error","message":"'problem' description to resolve is required."}`
		}
		if err := ResolveProblem(db, id, problem); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Problem resolved."}`

	default:
		return fmt.Sprintf(`{"status":"error","message":"Unknown homepage_registry operation '%s'. Use: register, search, get, list, update, delete, log_edit, log_deploy, log_problem, resolve_problem."}`, operation)
	}
}
