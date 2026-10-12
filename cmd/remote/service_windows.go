package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func getInstallPath() (string, error) {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "AuraGo", "aurago-remote.exe"), nil
}
func installService(exePath string) error {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService("AuraGoRemote")
	if err != nil {
		service, err = manager.CreateService("AuraGoRemote", exePath, mgr.Config{DisplayName: "AuraGo Remote Agent", StartType: mgr.StartAutomatic}, "--config", configPath())
	} else {
		config, readErr := service.Config()
		if readErr != nil {
			service.Close()
			return readErr
		}
		config.BinaryPathName = windowsServiceBinPath(exePath)
		err = service.UpdateConfig(config)
	}
	if err != nil {
		return fmt.Errorf("install remote service: %w", err)
	}
	defer service.Close()
	if status, err := service.Query(); err == nil && status.State == svc.Running {
		return nil
	}
	return service.Start()
}
func uninstallService() error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	service, err := manager.OpenService("AuraGoRemote")
	if err != nil {
		return err
	}
	defer service.Close()
	if err := stopRemoteWindowsService(service); err != nil {
		return err
	}

	return service.Delete()
}
func windowsServiceBinPath(exePath string) string {
	return syscall.EscapeArg(exePath) + " --config " + syscall.EscapeArg(configPath())
}
func isRunningAsService() bool { return isRunningAsWindowsService(svc.IsWindowsService) }
func isRunningAsWindowsService(probe func() (bool, error)) bool {
	running, err := probe()
	return err == nil && running
}

type remoteWindowsService struct{ client *Client }

func (service *remoteWindowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	done := make(chan struct{})
	go func() { service.client.Run(); close(done) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	status <- current
	for {
		select {
		case <-done:
			status <- svc.Status{State: svc.StopPending}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				status <- current
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				service.client.Stop()
				<-done
				return false, 0
			}
		}
	}
}
func runServiceClient(client *Client) error {
	if isRunningAsService() {
		return svc.Run("AuraGoRemote", &remoteWindowsService{client: client})
	}
	client.Run()
	return nil
}

func stopInstalledService() error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	service, err := manager.OpenService("AuraGoRemote")
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return err
	}
	defer service.Close()
	return stopRemoteWindowsService(service)
}
func stopRemoteWindowsService(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("query remote service before stopping: %w", err)
	}
	if status.State != svc.Stopped {
		if _, err = service.Control(svc.Stop); err != nil {
			return err
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			status, err = service.Query()
			if err != nil {
				return err
			}
			if status.State == svc.Stopped {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("remote service did not stop")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}
