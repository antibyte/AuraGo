// Package fritzbox – storage service calls.
// Covers: NAS storage info, FTP status, USB device list, media server status.
package fritzbox

import (
	"fmt"
)

// StorageInfo holds general NAS storage information.
type StorageInfo struct {
	Path     string // mount path of the primary storage
	Type     string // e.g. "USB", "internal"
	Size     string // total size (human-readable as returned by Fritz!Box)
	Used     string // used space
	Free     string // free space
	Writable bool
}

// USBDevice represents a USB device connected to the Fritz!Box.
type USBDevice struct {
	DeviceType    string // e.g. "Storage", "Printer"
	DeviceName    string
	PartitionName string
	FileSystem    string
	Total         string
	Free          string
}

// GetStorageInfo returns information about the Fritz!Box NAS storage.
func (c *Client) GetStorageInfo() (*StorageInfo, error) {
	res, err := c.SOAP(svcStorage, ctlStorage, "X_AVM-DE_GetStorageInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox storage: GetStorageInfo: %w", err)
	}
	return &StorageInfo{
		Path:     res["NewX_AVM-DE_Path"],
		Type:     res["NewX_AVM-DE_Type"],
		Size:     res["NewX_AVM-DE_Size"],
		Used:     res["NewX_AVM-DE_Used"],
		Free:     res["NewX_AVM-DE_Free"],
		Writable: res["NewX_AVM-DE_Writable"] == "1",
	}, nil
}

// GetFTPStatus returns whether FTP is enabled on the Fritz!Box.
func (c *Client) GetFTPStatus() (enabled bool, err error) {
	res, err := c.SOAP(svcStorage, ctlStorage, "X_AVM-DE_GetFTPServerEnable", nil)
	if err != nil {
		return false, fmt.Errorf("fritzbox storage: GetFTPStatus: %w", err)
	}
	return res["NewX_AVM-DE_FTPServerEnable"] == "1", nil
}

// SetFTPEnabled enables or disables the Fritz!Box FTP server.
// Blocked when ReadOnly is true.
func (c *Client) SetFTPEnabled(enabled bool) error {
	if c.StorageReadOnly() {
		return fmt.Errorf("fritzbox storage: FTP toggle blocked (readonly mode)")
	}
	val := "0"
	if enabled {
		val = "1"
	}
	_, err := c.SOAP(svcStorage, ctlStorage, "X_AVM-DE_SetFTPServerEnable",
		map[string]string{"NewX_AVM-DE_FTPServerEnable": val})
	if err != nil {
		return fmt.Errorf("fritzbox storage: SetFTPEnabled: %w", err)
	}
	return nil
}

// GetMediaServerStatus returns whether the Fritz!Box DLNA media server is enabled.
func (c *Client) GetMediaServerStatus() (enabled bool, err error) {
	res, err := c.SOAP(svcStorage, ctlStorage, "X_AVM-DE_GetDLNAServerEnable", nil)
	if err != nil {
		return false, fmt.Errorf("fritzbox storage: GetMediaServerStatus: %w", err)
	}
	return res["NewX_AVM-DE_DLNAServerEnable"] == "1", nil
}
