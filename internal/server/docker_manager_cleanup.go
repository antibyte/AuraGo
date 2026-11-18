package server

import (
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerutil"
)

const dmCleanupLimit = 200

type dmCleanupItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
	Size     int64  `json:"size"` // -1 when Docker cannot report a size
	created  string
}

type dmCleanupSelection struct {
	ID          string `json:"id"`
	Revision    string `json:"revision"`
	ConfirmName string `json:"confirm_name,omitempty"`
}

type dmCleanupResult struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

func (dm *dmRequest) cleanup(hostID string, preview bool) {
	if !dm.allow(http.MethodPost) {
		return
	}
	if !preview && dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	var in struct {
		Kind  string               `json:"kind"`
		Items []dmCleanupSelection `json:"items"`
	}
	if !dm.decode(&in) {
		return
	}
	switch in.Kind {
	case "images", "containers", "networks", "volumes":
	default:
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "unknown cleanup category", nil)
		return
	}
	if !preview && (len(in.Items) == 0 || len(in.Items) > dmCleanupLimit) {
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "select between 1 and 200 items", nil)
		return
	}
	seen := make(map[string]bool, len(in.Items))
	for _, item := range in.Items {
		if !dockerhost.ValidCleanupID(in.Kind, item.ID) || item.Revision == "" || seen[item.ID] || (in.Kind == "volumes" && item.ConfirmName != item.ID) {
			dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "invalid selection or missing volume name confirmation", nil)
			return
		}
		seen[item.ID] = true
	}
	ctx, cancel := dm.context(dmActionTimeout)
	defer cancel()
	host, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	items, err := dm.cleanupCandidates(ctx, host, client, in.Kind)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	if preview {
		if len(items) > 0 {
			sizeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			sizes := dockerhost.CleanupSizeHints(sizeCtx, client, in.Kind)
			stop()
			for i := range items {
				if size, found := sizes[items[i].ID]; found {
					items[i].Size = size
				}
			}
		}
		slices.SortFunc(items, func(a, b dmCleanupItem) int {
			return cmp.Or(cmp.Compare(b.Size, a.Size), cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
		})
		dmWriteOK(dm.w, http.StatusOK, dmFields{"items": items, "limit": dmCleanupLimit})
		return
	}
	current := make(map[string]dmCleanupItem, len(items))
	for _, item := range items {
		current[item.ID] = item
	}
	results := make([]dmCleanupResult, len(in.Items))
	cfg, _, _ := containerDockerConfig(dm.s)
	local := host.ID == dockerhost.LocalHostID || host.ReachesEngineAt(cfg.Host)
	for index, pick := range in.Items {
		result := &results[index]
		result.ID, result.State = pick.ID, "skipped"
		item, found := current[pick.ID]
		if !found || item.Revision != pick.Revision || ctx.Err() != nil {
			continue
		}
		activeCfg, enabled, readOnly := containerDockerConfig(dm.s)
		if !enabled || readOnly || (local && activeCfg.Host != cfg.Host) {
			continue
		}
		// The full local action check also fails closed on an unverifiable endpoint.
		if in.Kind == "containers" && local && containerProtectionFor(ctx, dm.s, cfg, item.ID).protected() {
			continue
		}
		if err := dockerhost.RemoveCleanupItem(ctx, client, in.Kind, item.ID, item.created); err != nil {
			result.Message = dockerutil.SanitizeOneLine(err.Error(), 512)
			if status := dockerhost.StatusOf(err); status != http.StatusNotFound && status != http.StatusConflict {
				result.State = "failed"
			}
			continue
		}
		result.State = "removed"
	}
	if dm.s.DockerWatch != nil {
		dm.s.DockerWatch.InvalidateDiskUsage(hostID)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"results": results})
}

// Both the preview and the delete path derive eligibility from live inventory, including stopped containers.
func (dm *dmRequest) cleanupCandidates(ctx context.Context, host dockerhost.Host, client *dockerutil.Client, kind string) ([]dmCleanupItem, error) {
	containers, entries, err := dockerhost.ListContainersWithEntries(ctx, client)
	if err != nil {
		return nil, err
	}
	items := []dmCleanupItem{}
	add := func(id, name, generation string, size int64) {
		if !dockerhost.ValidCleanupID(kind, id) {
			return
		}
		name = cmp.Or(name, id)
		stamp := strings.Join([]string{host.ID, client.Host(), host.UpdatedAt.String(), kind, id, name, generation}, "\x00")
		items = append(items, dmCleanupItem{ID: id, Name: name, Size: size, Revision: fmt.Sprintf("%x", sha256.Sum256([]byte(stamp))), created: generation})
	}
	switch kind {
	case "images":
		for _, ctr := range containers {
			if ctr.ImageID == "" {
				return nil, fmt.Errorf("Docker did not identify the image of container %s", ctr.ID)
			}
		}
		images, err := dockerhost.ListImages(ctx, client)
		if err != nil {
			return nil, err
		}
		dockerhost.Annotate(containers, images, nil, nil)
		for _, image := range images {
			if image.Containers != 0 || image.Managed {
				continue
			}
			refs := append(slices.Clone(image.Tags), image.Digests...)
			if slices.ContainsFunc(refs, dmCleanupReservedImage) {
				continue
			}
			slices.Sort(refs)
			slices.Sort(image.Tags)
			name := strings.Join(image.Tags, ", ")
			if name == "" {
				name = strings.Join(refs, ", ")
			}
			add(image.ID, name, strings.Join(refs, "\n"), image.Size)
		}
	case "containers":
		cfg, _, _ := containerDockerConfig(dm.s)
		var flags map[string]adminContainerEntry
		if host.ID == dockerhost.LocalHostID || host.ReachesEngineAt(cfg.Host) {
			flags = dm.localProtectionFlags(ctx, entries)
		}
		inDocker := containerRuntimeIsDocker(dm.s)
		for _, ctr := range containers {
			if ctr.State != "exited" && ctr.State != "dead" {
				continue
			}
			f := flags[ctr.ID]
			if f.Self || f.DockerEndpoint || f.SharedNetwork || f.ProtectedOwner != "" || containerIsSelf(inDocker, ctr.FullID) ||
				firstContainerOwner(ctr.Names, ctr.Labels) != "" || dockerhost.CleanupManagedLabels(ctr.Labels) {
				continue
			}
			add(ctr.FullID, strings.Join(ctr.Names, ", "), ctr.State, -1)
		}
	case "networks":
		networks, err := dockerhost.ListNetworks(ctx, client)
		if err != nil {
			return nil, err
		}
		dockerhost.Annotate(containers, nil, nil, networks)
		for _, n := range networks {
			if n.Containers == 0 && n.Scope == "local" && !n.Builtin && !n.Managed && n.Name != "docker_gwbridge" && !strings.EqualFold(n.Name, "nat") && !dockerhost.CleanupReservedName(n.Name) {
				add(n.ID, n.Name, n.Driver, -1)
			}
		}
	case "volumes":
		volumes, err := dockerhost.ListVolumes(ctx, client)
		if err != nil {
			return nil, err
		}
		dockerhost.Annotate(containers, nil, volumes, nil)
		for _, v := range volumes {
			if len(v.UsedBy) == 0 && v.Driver == "local" && v.CreatedAt != "" && !dockerhost.CleanupManagedLabels(v.Labels) &&
				!dockerhost.CleanupReservedName(v.Name) && !dockerutil.IsRustFSVolumeName(v.Name) && !dockerutil.IsLocalLLMVolumeName(v.Name) {
				add(v.Name, v.Name, v.CreatedAt, -1)
			}
		}
	}
	return items, nil
}

func dmCleanupReservedImage(ref string) bool {
	return dockerhost.CleanupReservedName(path.Base(dockerutil.ImageRepository(ref))) || ref == dockerstacks.RunnerImageTag ||
		dockerutil.ImageReferenceDigest(ref) == dockerstacks.RunnerImageDigest
}
