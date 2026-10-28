package server

import (
	"context"
	"net/http"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// overview serves GET /hosts/{id}/overview: the Engine's /info figures.
func (dm *dmRequest) overview(hostID string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	info, err := dockerhost.Info(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"info": info})
}

// resourceList serves GET /hosts/{id}/{images|volumes|networks}.
func (dm *dmRequest) resourceList(hostID, kind string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmListTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	switch kind {
	case "images":
		dm.listImages(ctx, client)
	case "volumes":
		dm.listVolumes(ctx, client)
	case "networks":
		dm.listNetworks(ctx, client)
	default:
		dm.notFound()
	}
}

func (dm *dmRequest) listImages(ctx context.Context, client *dockerutil.Client) {
	images, err := dockerhost.ListImages(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	// Without the container list the usage stays unknown: images keep -1.
	if containers, listErr := dockerhost.ListContainers(ctx, client); listErr == nil {
		dockerhost.Annotate(containers, images, nil, nil)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"images": images})
}

func (dm *dmRequest) listVolumes(ctx context.Context, client *dockerutil.Client) {
	volumes, err := dockerhost.ListVolumes(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	if containers, listErr := dockerhost.ListContainers(ctx, client); listErr == nil {
		dockerhost.Annotate(containers, nil, volumes, nil)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"volumes": volumes})
}

func (dm *dmRequest) listNetworks(ctx context.Context, client *dockerutil.Client) {
	networks, err := dockerhost.ListNetworks(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	if containers, listErr := dockerhost.ListContainers(ctx, client); listErr == nil {
		dockerhost.Annotate(containers, nil, nil, networks)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"networks": networks})
}
