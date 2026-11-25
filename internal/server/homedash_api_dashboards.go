package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/homedash"
)

// prepare is the save pipeline every save path shares (decision D4): Migrate, Normalize, Autopack,
// Validate. Issues refuse the save as invalid_document; warnings pass. ApplyPatch never saves alone.
func (svc *homeDashService) prepare(ctx context.Context, doc *homedash.Dashboard) (homedash.Report, error) {
	if err := homedash.Migrate(doc); err != nil {
		return homedash.Report{}, err
	}
	homedash.Normalize(doc)
	homedash.Autopack(doc, svc.cat)
	known := svc.known(ctx, *doc)
	rep := homedash.Validate(*doc, svc.cat, func(id homedash.EntityID) (homedash.Entity, bool) {
		e, ok := known[id]
		return e, ok
	})
	if !rep.OK() {
		return rep, &homedash.Error{Code: homedash.CodeInvalidDocument, Message: "the dashboard is not valid", Issues: rep.Issues}
	}
	return rep, nil
}

// known looks up the bound entities (catalogs fetched when stale, no source warmed) so Validate can
// warn about unknown or unfitting entities.
func (svc *homeDashService) known(ctx context.Context, doc homedash.Dashboard) map[homedash.EntityID]homedash.Entity {
	ids := homeDashBoundIDs(doc)
	if len(ids) == 0 {
		return map[homedash.EntityID]homedash.Entity{}
	}
	lctx, cancel := context.WithTimeout(ctx, homeDashCatalogWait)
	defer cancel()
	return svc.hub.LookupEntities(lctx, ids)
}

// homeDashBoundIDs lists the distinct syntactically valid entity ids a document binds.
func homeDashBoundIDs(doc homedash.Dashboard) []homedash.EntityID {
	seen := map[homedash.EntityID]bool{}
	var ids []homedash.EntityID
	for _, p := range doc.Pages {
		for _, w := range p.Widgets {
			for _, raw := range w.Bindings {
				if _, _, err := homedash.ParseEntityID(raw); err != nil {
					continue
				}
				if id := homedash.EntityID(raw); !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

// saveNew stores doc as a new dashboard (a user save, approved). An empty slug is derived from
// slugBase (the name when empty) and made unique; an explicit slug is kept as given.
func (svc *homeDashService) saveNew(ctx context.Context, doc homedash.Dashboard, slugBase string) (homedash.Dashboard, homedash.Report, error) {
	doc.ID, doc.Revision, doc.ApprovedRevision, doc.UpdatedAt, doc.SavedBy = "", 0, 0, time.Time{}, ""
	if doc.Slug == "" {
		if slugBase == "" {
			slugBase = doc.Name
		}
		slug, err := svc.freeSlug(ctx, homeDashSlug(slugBase))
		if err != nil {
			return homedash.Dashboard{}, homedash.Report{}, err
		}
		doc.Slug = slug
	}
	rep, err := svc.prepare(ctx, &doc)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	saved, err := svc.store.CreateDashboard(ctx, doc, homedash.SavedByUser)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	svc.hub.DashboardChanged(saved.ID, saved.Revision, saved.ApprovedRevision)
	return saved, rep, nil
}

// saveExisting stores doc as the next revision of id when revision is still current (a user save,
// approved). Server-owned fields of doc are ignored.
func (svc *homeDashService) saveExisting(ctx context.Context, id string, revision int64, doc homedash.Dashboard) (homedash.Dashboard, homedash.Report, error) {
	doc.ID, doc.Revision, doc.ApprovedRevision, doc.UpdatedAt, doc.SavedBy = id, 0, 0, time.Time{}, ""
	rep, err := svc.prepare(ctx, &doc)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	saved, err := svc.store.UpdateDashboard(ctx, doc, revision, homedash.SavedByUser)
	if err != nil {
		return homedash.Dashboard{}, rep, err
	}
	svc.hub.DashboardChanged(saved.ID, saved.Revision, saved.ApprovedRevision)
	return saved, rep, nil
}

// freeSlug returns base, or base-2 … base-99, whichever no dashboard uses yet. The store's unique
// index still refuses a race with 409 slug_taken.
func (svc *homeDashService) freeSlug(ctx context.Context, base string) (string, error) {
	list, err := svc.store.ListDashboards(ctx)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(list))
	for _, d := range list {
		taken[d.Slug] = true
	}
	if !taken[base] {
		return base, nil
	}
	for n := 2; n < 100; n++ {
		if candidate := base + "-" + strconv.Itoa(n); !taken[candidate] {
			return candidate, nil
		}
	}
	return "", &homedash.Error{Code: homedash.CodeSlugTaken, Message: "no free slug for this name"}
}

// homeDashSlug derives a slug base: lower-case ASCII letters and digits, every other run of
// characters one dash, at most 44 characters (room for "-99"), "dashboard" when nothing is left.
func homeDashSlug(text string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
		if b.Len() >= 44 {
			break
		}
	}
	s := b.String()
	if len(s) > 44 {
		s = s[:44]
	}
	if s = strings.Trim(s, "-"); s == "" {
		return "dashboard"
	}
	return s
}

func (h *hdRequest) listDashboards() {
	list, err := h.svc.store.ListDashboards(h.r.Context())
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboards": homeDashNonNil(list)})
}

type homeDashCreateRequest struct {
	Document json.RawMessage `json:"document"`
	Template string          `json:"template"`
	Name     string          `json:"name"`
}

func (h *hdRequest) createDashboard() {
	var req homeDashCreateRequest
	if !h.decode(&req, homeDashDocumentBody) {
		return
	}
	var doc homedash.Dashboard
	slugBase := ""
	switch {
	case len(req.Document) > 0 && req.Template == "" && req.Name == "":
		d, err := homedash.DecodeDashboard(req.Document)
		if err != nil {
			h.fail(err)
			return
		}
		doc = d
	case len(req.Document) == 0 && req.Template == homeDashTemplateSystem:
		doc, slugBase = homeDashStarterSystem(req.Name), homeDashTemplateSystem
	case len(req.Document) == 0 && req.Template != "":
		h.bad("unknown template")
		return
	default:
		h.bad("send either a document or a template")
		return
	}
	saved, rep, err := h.svc.saveNew(h.r.Context(), doc, slugBase)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusCreated, map[string]any{"dashboard": saved, "warnings": homeDashNonNil(rep.Warnings)})
}

func (h *hdRequest) getDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	d, err := h.svc.store.GetDashboard(h.r.Context(), id)
	var he *homedash.Error
	if errors.As(err, &he) && he.Code == homedash.CodeSchemaTooNew {
		homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": d, "schema_too_new": true})
		return
	}
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": d, "schema_too_new": false})
}

type homeDashUpdateRequest struct {
	Revision int64           `json:"revision"`
	Document json.RawMessage `json:"document"`
}

func (h *hdRequest) updateDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	var req homeDashUpdateRequest
	if !h.decode(&req, homeDashDocumentBody) {
		return
	}
	if req.Revision < 1 || len(req.Document) == 0 {
		h.bad("revision and document are required")
		return
	}
	doc, err := homedash.DecodeDashboard(req.Document)
	if err != nil {
		h.fail(err)
		return
	}
	saved, rep, err := h.svc.saveExisting(h.r.Context(), id, req.Revision, doc)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"dashboard": saved, "warnings": homeDashNonNil(rep.Warnings)})
}

func (h *hdRequest) deleteDashboard() {
	id, ok := h.dashboardID()
	if !ok {
		return
	}
	rev, ok := h.revisionParam()
	if !ok {
		return
	}
	devices, err := h.svc.store.DeleteDashboard(h.r.Context(), id, rev)
	if err != nil {
		h.fail(err)
		return
	}
	for _, dev := range devices {
		h.svc.hub.RevokeDevice(dev, homedash.CloseDeviceRevoked)
	}
	h.svc.hub.DashboardChanged(id, 0, 0)
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"id": id})
}

// homeDashNonNil turns a nil slice into an empty one so the JSON answer is [] and never null.
func homeDashNonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
