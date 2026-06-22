package server

import (
	"context"

	"aurago/internal/desktop"
	"aurago/internal/desktopstore"
)

type desktopStoreNativeRuntime struct {
	desktop *desktop.Service
}

func newDesktopStoreNativeRuntime(svc *desktop.Service) desktopStoreNativeRuntime {
	return desktopStoreNativeRuntime{desktop: svc}
}

func (r desktopStoreNativeRuntime) InstallNativeManagedApp(ctx context.Context, appID string, entry desktopstore.CatalogEntry, deleteData bool) (desktopstore.NativeManagedStatus, error) {
	if appID != "openscad" {
		return desktopstore.NativeManagedStatus{}, unsupportedNativeRuntime(appID)
	}
	svc := r.desktop.OpenSCADContainer()
	if deleteData {
		_ = svc.Remove(ctx, true)
	}
	if err := svc.EnsureInstalled(ctx); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	return r.status(ctx, entry), nil
}

func (r desktopStoreNativeRuntime) StartNativeManagedApp(ctx context.Context, appID string, entry desktopstore.CatalogEntry) (desktopstore.NativeManagedStatus, error) {
	if appID != "openscad" {
		return desktopstore.NativeManagedStatus{}, unsupportedNativeRuntime(appID)
	}
	if err := r.desktop.OpenSCADContainer().EnsureStarted(ctx); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	return r.status(ctx, entry), nil
}

func (r desktopStoreNativeRuntime) StopNativeManagedApp(ctx context.Context, appID string, entry desktopstore.CatalogEntry) (desktopstore.NativeManagedStatus, error) {
	if appID != "openscad" {
		return desktopstore.NativeManagedStatus{}, unsupportedNativeRuntime(appID)
	}
	if err := r.desktop.OpenSCADContainer().Stop(ctx); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	return r.status(ctx, entry), nil
}

func (r desktopStoreNativeRuntime) RestartNativeManagedApp(ctx context.Context, appID string, entry desktopstore.CatalogEntry) (desktopstore.NativeManagedStatus, error) {
	if appID != "openscad" {
		return desktopstore.NativeManagedStatus{}, unsupportedNativeRuntime(appID)
	}
	if err := r.desktop.OpenSCADContainer().Stop(ctx); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	if err := r.desktop.OpenSCADContainer().EnsureStarted(ctx); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	return r.status(ctx, entry), nil
}

func (r desktopStoreNativeRuntime) UpdateNativeManagedApp(ctx context.Context, appID string, entry desktopstore.CatalogEntry) (desktopstore.NativeManagedStatus, error) {
	if appID != "openscad" {
		return desktopstore.NativeManagedStatus{}, unsupportedNativeRuntime(appID)
	}
	if err := r.desktop.OpenSCADContainer().Remove(ctx, false); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	if err := r.desktop.OpenSCADContainer().EnsureInstalled(ctx); err != nil {
		return desktopstore.NativeManagedStatus{}, err
	}
	return r.status(ctx, entry), nil
}

func (r desktopStoreNativeRuntime) UninstallNativeManagedApp(ctx context.Context, appID string, entry desktopstore.CatalogEntry, deleteData bool) error {
	if appID != "openscad" {
		return unsupportedNativeRuntime(appID)
	}
	return r.desktop.OpenSCADContainer().Remove(ctx, deleteData)
}

func (r desktopStoreNativeRuntime) status(ctx context.Context, entry desktopstore.CatalogEntry) desktopstore.NativeManagedStatus {
	status := r.desktop.OpenSCADContainer().Status(ctx)
	storeStatus := desktopstore.AppStatusStopped
	if status.Running {
		storeStatus = desktopstore.AppStatusRunning
	}
	if status.State == "error" {
		storeStatus = desktopstore.AppStatusError
	}
	return desktopstore.NativeManagedStatus{
		ContainerName: desktopstore.NativeManagedContainerName(entry.ID),
		ContainerID:   status.ContainerID,
		Image:         status.Image,
		Status:        storeStatus,
		Running:       status.Running,
		Error:         status.Error,
	}
}

func unsupportedNativeRuntime(appID string) error {
	return &desktopStoreNativeRuntimeError{appID: appID}
}

type desktopStoreNativeRuntimeError struct {
	appID string
}

func (e *desktopStoreNativeRuntimeError) Error() string {
	return "unsupported native desktop store app " + e.appID
}
