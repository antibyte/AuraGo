package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sync"

	"aurago/internal/config"
	"aurago/internal/findit"
)

var findItPublished = struct {
	sync.RWMutex
	store         *findit.Store
	currentConfig func() *config.Config
}{}

// SetFindItStore publishes the server-owned FindIt store and live config getter
// to the read-only agent tool. The store must be withdrawn before it is closed.
func SetFindItStore(store *findit.Store, currentConfig func() *config.Config) {
	findItPublished.Lock()
	defer findItPublished.Unlock()
	findItPublished.store = store
	findItPublished.currentConfig = currentConfig
}

// WithdrawFindItStore removes store only if it is still the published instance.
// The write lock waits for active Query calls to release their store read lock.
func WithdrawFindItStore(store *findit.Store) {
	if store == nil {
		return
	}
	findItPublished.Lock()
	defer findItPublished.Unlock()
	if findItPublished.store == store {
		findItPublished.store = nil
		findItPublished.currentConfig = nil
	}
}

// FindItAvailable reports whether a published store currently has the explicit
// agent read grant and its owning Desktop app is enabled.
func FindItAvailable() bool {
	findItPublished.RLock()
	store := findItPublished.store
	currentConfig := findItPublished.currentConfig
	findItPublished.RUnlock()
	return store != nil && currentConfig != nil && findItAgentAllowed(currentConfig())
}

// ExecuteFindIt performs one strictly read-only FindIt query. It holds the
// publication read lock for the full query so server shutdown cannot close the
// database while a read is in progress. The live config is checked on both
// sides of the query so revocation suppresses stale-run results.
func ExecuteFindIt(ctx context.Context, cfg *config.Config, req findit.QueryRequest) string {
	findItPublished.RLock()
	defer findItPublished.RUnlock()

	if findItPublished.store == nil {
		return findItFailure("unavailable", "FindIt is not available right now.")
	}
	current := findItPublished.currentConfig
	if current == nil {
		return findItFailure("unavailable", "FindIt access policy is not available right now.")
	}
	cfg = current()
	if !findItAgentAllowed(cfg) {
		return findItFailure("policy_denied", "FindIt is not enabled for agent access.")
	}
	if !validFindItReadRequest(req) {
		return findItFailure("invalid", "Use FindIt search or get for items, locations, projects, or templates; availability is for items and shortages is for projects.")
	}

	result, err := findItPublished.store.Query(ctx, req)
	// Recheck even when the query failed, so a newly revoked grant does not
	// retain a stale store result or error in an already-running turn.
	if !findItAgentAllowed(current()) {
		return findItFailure("policy_denied", "FindIt agent access was disabled while the query was running.")
	}
	if err != nil {
		code := findit.ErrorCode(err)
		message := "FindIt query failed."
		var domainErr *findit.DomainError
		if errors.As(err, &domainErr) && domainErr != nil {
			message = domainErr.Error()
		}
		return findItFailure(code, message)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return findItFailure("unavailable", "FindIt result could not be serialized.")
	}
	if len(data) > findItResultMaxBytes {
		return findItFailure("result_too_large", "FindIt returned too much data. Narrow the search or request a smaller page.")
	}
	payload, err := json.Marshal(map[string]any{
		"status":    "success",
		"result":    json.RawMessage(data),
		"app_links": buildFindItAppLinks(result),
	})
	if err != nil {
		return findItFailure("unavailable", "FindIt result could not be serialized.")
	}
	if len(payload) > findItResultMaxBytes {
		return findItFailure("result_too_large", "FindIt returned too much data. Narrow the search or request a smaller page.")
	}
	return "Tool Output: " + string(payload)
}

const findItResultMaxBytes = 128 * 1024

func findItAgentAllowed(cfg *config.Config) bool {
	return cfg != nil && cfg.FindIt.Enabled && cfg.FindIt.AgentAccess && cfg.VirtualDesktop.Enabled
}

func validFindItReadRequest(req findit.QueryRequest) bool {
	if len(req.Filters) > 8 || len(req.Query) > 512 || req.Limit < 0 || req.Limit > 200 || req.Offset < 0 || req.Offset > 1_000_000 {
		return false
	}
	switch req.Operation {
	case "search":
		switch req.Entity {
		case "items", "locations", "projects", "templates":
			return true
		}
	case "get":
		if req.ID == "" {
			return false
		}
		switch req.Entity {
		case "items", "locations", "projects", "templates":
			return true
		}
	case "availability":
		return req.Entity == "items" && req.ID != ""
	case "shortages":
		return req.Entity == "projects" && req.ID != ""
	}
	return false
}

func findItFailure(code, message string) string {
	payload, _ := json.Marshal(map[string]string{"status": "error", "code": code, "message": message})
	return "Tool Output: " + string(payload)
}

const findItMaxAppLinks = 32

var findItAppLinkID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

type findItAppLink struct {
	Entity   string `json:"entity"`
	ID       string `json:"id"`
	URL      string `json:"url"`
	DeepLink bool   `json:"deep_link"`
}

func buildFindItAppLinks(result any) []findItAppLink {
	links := make([]findItAppLink, 0, findItMaxAppLinks)
	seen := make(map[string]struct{}, findItMaxAppLinks)
	add := func(entity, id string) {
		if len(links) >= findItMaxAppLinks || !findItAppLinkID.MatchString(id) {
			return
		}
		key := entity + "\x00" + id
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}

		query := url.Values{"app": {"findit"}}
		deepLink := entity == "item" || entity == "location"
		if deepLink {
			query.Set(entity, id)
		}
		links = append(links, findItAppLink{Entity: entity, ID: id, URL: "/desktop?" + query.Encode(), DeepLink: deepLink})
	}
	addProject := func(project findit.Project) {
		add("project", project.ID)
		for _, line := range project.Lines {
			add("item", line.ItemID)
		}
	}
	var addRecord func(any)
	addRecord = func(value any) {
		switch record := value.(type) {
		case findit.Item:
			add("item", record.ID)
			for _, movement := range record.Movements {
				add("location", movement.SourceLocationID)
				add("location", movement.TargetLocationID)
			}
		case *findit.Item:
			if record != nil {
				addRecord(*record)
			}
		case findit.Location:
			add("location", record.ID)
			add("location", record.ParentID)
		case *findit.Location:
			if record != nil {
				addRecord(*record)
			}
		case findit.Project:
			addProject(record)
		case *findit.Project:
			if record != nil {
				addProject(*record)
			}
		case findit.Template:
			add("template", record.ID)
		case *findit.Template:
			if record != nil {
				addRecord(*record)
			}
		}
	}

	switch value := result.(type) {
	case findit.SearchResult:
		for _, item := range value.Items {
			addRecord(item)
		}
	case *findit.SearchResult:
		if value != nil {
			return buildFindItAppLinks(*value)
		}
	case findit.Item:
		addRecord(value)
	case *findit.Item:
		addRecord(value)
	case findit.Location:
		addRecord(value)
	case *findit.Location:
		addRecord(value)
	case findit.Project:
		addRecord(value)
	case *findit.Project:
		addRecord(value)
	case findit.Template:
		addRecord(value)
	case *findit.Template:
		addRecord(value)
	case findit.AvailabilityResult:
		add("item", value.ItemID)
		for _, stock := range value.Stocks {
			add("location", stock.LocationID)
		}
	case *findit.AvailabilityResult:
		if value != nil {
			add("item", value.ItemID)
			for _, stock := range value.Stocks {
				add("location", stock.LocationID)
			}
		}
	case findit.ShortagesResult:
		addProject(value.Project)
		for _, line := range value.Lines {
			add("item", line.ItemID)
		}
	case *findit.ShortagesResult:
		if value != nil {
			addProject(value.Project)
			for _, line := range value.Lines {
				add("item", line.ItemID)
			}
		}
	}
	return links
}
