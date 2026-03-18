// Package tools – upnp_scan: UPnP/SSDP device discovery via goupnp.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/huin/goupnp"
)

// upnpResult is the JSON payload returned by ExecuteUPnPScan.
type upnpResult struct {
	Status  string       `json:"status"`
	Count   int          `json:"count,omitempty"`
	Devices []upnpDevice `json:"devices,omitempty"`
	Message string       `json:"message,omitempty"`
}

// upnpDevice represents a discovered UPnP device.
type upnpDevice struct {
	USN              string        `json:"usn"`
	Location         string        `json:"location,omitempty"`
	FriendlyName     string        `json:"friendly_name,omitempty"`
	DeviceType       string        `json:"device_type,omitempty"`
	Manufacturer     string        `json:"manufacturer,omitempty"`
	ModelName        string        `json:"model_name,omitempty"`
	ModelDescription string        `json:"model_description,omitempty"`
	SerialNumber     string        `json:"serial_number,omitempty"`
	Services         []upnpService `json:"services,omitempty"`
}

// upnpService represents a service exposed by a UPnP device.
type upnpService struct {
	ServiceType string `json:"service_type"`
	ServiceID   string `json:"service_id"`
}

func upnpJSON(r upnpResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// ExecuteUPnPScan discovers UPnP/SSDP devices on the local network.
//
// searchTarget – UPnP search target (default: "ssdp:all")
//
//	common values: "ssdp:all", "upnp:rootdevice",
//	"urn:schemas-upnp-org:device:MediaRenderer:1",
//	"urn:schemas-upnp-org:device:InternetGatewayDevice:1"
//
// timeoutSecs  – discovery timeout in seconds (default: 5, max: 30)
func ExecuteUPnPScan(searchTarget string, timeoutSecs int) string {
	if searchTarget == "" {
		searchTarget = "ssdp:all"
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 5
	}
	if timeoutSecs > 30 {
		timeoutSecs = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	maybeDevices, err := goupnp.DiscoverDevicesCtx(ctx, searchTarget)
	if err != nil {
		return upnpJSON(upnpResult{Status: "error", Message: fmt.Sprintf("UPnP discovery failed: %v", err)})
	}

	devices := make([]upnpDevice, 0, len(maybeDevices))
	seen := make(map[string]bool)

	for _, maybe := range maybeDevices {
		if maybe.Err != nil || maybe.Root == nil {
			continue
		}

		// Deduplicate by UDN
		udn := maybe.Root.Device.UDN
		if udn == "" {
			udn = maybe.USN
		}
		if seen[udn] {
			continue
		}
		seen[udn] = true

		loc := ""
		if maybe.Location != nil {
			loc = maybe.Location.String()
		}

		d := upnpDevice{
			USN:              maybe.USN,
			Location:         loc,
			FriendlyName:     maybe.Root.Device.FriendlyName,
			DeviceType:       cleanDeviceType(maybe.Root.Device.DeviceType),
			Manufacturer:     maybe.Root.Device.Manufacturer,
			ModelName:        maybe.Root.Device.ModelName,
			ModelDescription: maybe.Root.Device.ModelDescription,
			SerialNumber:     maybe.Root.Device.SerialNumber,
		}

		// Collect services
		maybe.Root.Device.VisitServices(func(svc *goupnp.Service) {
			d.Services = append(d.Services, upnpService{
				ServiceType: svc.ServiceType,
				ServiceID:   svc.ServiceId,
			})
		})

		devices = append(devices, d)
	}

	if len(devices) == 0 {
		return upnpJSON(upnpResult{
			Status:  "success",
			Count:   0,
			Message: fmt.Sprintf("No UPnP devices found (search_target: %s)", searchTarget),
		})
	}

	return upnpJSON(upnpResult{
		Status:  "success",
		Count:   len(devices),
		Devices: devices,
	})
}

// cleanDeviceType strips the URN prefix for readability, e.g.
// "urn:schemas-upnp-org:device:MediaRenderer:1" → "MediaRenderer"
func cleanDeviceType(dt string) string {
	parts := strings.Split(dt, ":")
	if len(parts) >= 4 {
		return parts[3]
	}
	return dt
}
