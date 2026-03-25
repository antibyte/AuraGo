# TrueNAS Integration Plan für AuraGo

**Status:** Design Phase  
**Priorität:** Hoch  
**Komplexität:** Mittel-Hoch  
**Geschätzter Aufwand:** 3-4 Tage

---

## 1. Executive Summary

TrueNAS SCALE/Core ist der führende Open-Source NAS für Home Labs. Diese Integration ermöglicht AuraGo die vollständige Verwaltung von Storage, Snapshots, Freigaben und Replikationen über die TrueNAS REST API.

### Hauptfeatures:
- Pool- und Dataset-Verwaltung
- Automatisiertes Snapshot-Management
- SMB/NFS/iSCSI-Freigaben
- Cloud-Sync zu S3/B2/Azure
- Disk-Health-Monitoring
- Replikations-Jobs

---

## 2. Architektur-Übersicht

```
┌─────────────────────────────────────────────────────────────┐
│                        AuraGo                               │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ TrueNAS Tool │  │   Registry   │  │  Snapshot Mgr    │  │
│  │  (Core API)  │  │   (SQLite)   │  │   (Scheduler)    │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
└─────────┼─────────────────┼───────────────────┼────────────┘
          │                 │                   │
          │         ┌───────┴───────┐          │
          └────────►│  Config Store │◄─────────┘
                    └───────┬───────┘
                            │
                    ┌───────┴───────┐
                    │  TrueNAS API  │
                    │   (REST v2)   │
                    └───────┬───────┘
                            │
                    ┌───────┴───────┐
                    │  TrueNAS SCALE│
                    │   oder Core   │
                    └───────────────┘
```

---

## 3. Konfigurationsstruktur

### 3.1 Config Types (internal/config/config_types.go)

```go
// TrueNASConfig definiert die Verbindung zu TrueNAS
 type TrueNASConfig struct {
    Enabled          bool              `yaml:"enabled" json:"enabled"`
    Host             string            `yaml:"host" json:"host"`                         // z.B. "truenas.local"
    Port             int               `yaml:"port" json:"port"`                         // Standard: 443
    UseHTTPS         bool              `yaml:"use_https" json:"use_https"`              // Standard: true
    APIKey           string            `yaml:"api_key" json:"api_key"`                   // Vault-Referenz
    AllowDestructive bool              `yaml:"allow_destructive" json:"allow_destructive"` // Löschen von Pools/Datasets
    DefaultShares    TrueNASShareDefaults `yaml:"default_shares,omitempty" json:"default_shares,omitempty"`
}

type TrueNASShareDefaults struct {
    SMBEnabled bool `yaml:"smb_enabled" json:"smb_enabled"`
    NFSEnabled bool `yaml:"nfs_enabled" json:"nfs_enabled"`
    AclMode    string `yaml:"acl_mode" json:"acl_mode"` // "passthrough", "restricted", "discard"
}
```

### 3.2 Hauptkonfiguration (Config struct)

```go
type Config struct {
    // ... bestehende Felder ...
    
    TrueNAS TrueNASConfig `yaml:"truenas,omitempty" json:"truenas,omitempty"`
}
```

---

## 4. Datenbank-Schema (Registry)

### 4.1 Tabellenstruktur (internal/truenas/registry.go)

```sql
-- TrueNAS Server Registry
CREATE TABLE IF NOT EXISTS truenas_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    host TEXT NOT NULL,
    port INTEGER DEFAULT 443,
    use_https BOOLEAN DEFAULT 1,
    api_key_ref TEXT,  -- Vault reference
    version TEXT,      -- SCALE oder Core Version
    status TEXT DEFAULT 'unknown', -- online, offline, error
    last_check DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Pool-Übersicht
CREATE TABLE IF NOT EXISTS truenas_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    pool_id TEXT NOT NULL,  -- TrueNAS interne ID
    name TEXT NOT NULL,
    guid TEXT UNIQUE,
    status TEXT,  -- ONLINE, DEGRADED, FAULTED
    size_bytes INTEGER,
    allocated_bytes INTEGER,
    free_bytes INTEGER,
    health TEXT,
    scan_status TEXT,
    created_at DATETIME,
    last_scrub DATETIME,
    FOREIGN KEY (server_id) REFERENCES truenas_servers(id) ON DELETE CASCADE,
    UNIQUE(server_id, name)
);

-- Datasets (ZFS Datasets)
CREATE TABLE IF NOT EXISTS truenas_datasets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id INTEGER NOT NULL,
    dataset_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,  -- FILESYSTEM, VOLUME
    mountpoint TEXT,
    size_bytes INTEGER,
    used_bytes INTEGER,
    available_bytes INTEGER,
    compression TEXT,
    quota_bytes INTEGER,
    refquota_bytes INTEGER,
    reservation_bytes INTEGER,
    readonly BOOLEAN DEFAULT 0,
    exec_enabled BOOLEAN DEFAULT 1,
    atime_enabled BOOLEAN DEFAULT 1,
    snapdir_visible BOOLEAN DEFAULT 0,
    share_type TEXT,  -- GENERIC, SMB
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (pool_id) REFERENCES truenas_pools(id) ON DELETE CASCADE,
    UNIQUE(pool_id, name)
);

-- Snapshots
CREATE TABLE IF NOT EXISTS truenas_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dataset_id INTEGER NOT NULL,
    snapshot_id TEXT NOT NULL,
    name TEXT NOT NULL,
    snapshot_type TEXT DEFAULT 'manual',  -- manual, scheduled, replication
    size_bytes INTEGER,
    created_at DATETIME,
    replicated BOOLEAN DEFAULT 0,
    retention_days INTEGER,
    expires_at DATETIME,
    FOREIGN KEY (dataset_id) REFERENCES truenas_datasets(id) ON DELETE CASCADE,
    UNIQUE(dataset_id, name)
);

-- SMB/NFS Shares
CREATE TABLE IF NOT EXISTS truenas_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    share_id TEXT NOT NULL,
    name TEXT NOT NULL,
    share_type TEXT,  -- SMB, NFS, ISCSI
    dataset_id INTEGER,
    path TEXT,
    enabled BOOLEAN DEFAULT 1,
    ro_users TEXT,    -- JSON Array
    rw_users TEXT,    -- JSON Array
    guest_allowed BOOLEAN DEFAULT 0,
    recyclebin_enabled BOOLEAN DEFAULT 1,
    shadowcopy_enabled BOOLEAN DEFAULT 1,
    aux_params TEXT,  -- Zusätzliche SMB/NFS Parameter
    FOREIGN KEY (server_id) REFERENCES truenas_servers(id) ON DELETE CASCADE,
    FOREIGN KEY (dataset_id) REFERENCES truenas_datasets(id) ON DELETE SET NULL
);

-- Cloud Sync Jobs
CREATE TABLE IF NOT EXISTS truenas_cloudsync (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    job_id TEXT NOT NULL,
    description TEXT,
    direction TEXT,  -- PUSH, PULL
    path TEXT,       -- Lokaler Pfad
    remote_provider TEXT,  -- S3, B2, AZURE, etc.
    remote_bucket TEXT,
    schedule TEXT,   -- Cron expression
    enabled BOOLEAN DEFAULT 1,
    last_run DATETIME,
    last_result TEXT,
    FOREIGN KEY (server_id) REFERENCES truenas_servers(id) ON DELETE CASCADE
);

-- Disk Health
CREATE TABLE IF NOT EXISTS truenas_disks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    disk_id TEXT NOT NULL,
    name TEXT NOT NULL,
    serial TEXT,
    model TEXT,
    size_bytes INTEGER,
    smart_enabled BOOLEAN DEFAULT 0,
    smart_status TEXT,
    temperature INTEGER,
    power_on_hours INTEGER,
    reallocated_sectors INTEGER,
    last_smart_scan DATETIME,
    FOREIGN KEY (server_id) REFERENCES truenas_servers(id) ON DELETE CASCADE,
    UNIQUE(server_id, name)
);

-- Replikations-Jobs
CREATE TABLE IF NOT EXISTS truenas_replication (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    job_id TEXT NOT NULL,
    name TEXT,
    source_dataset TEXT,
    target_dataset TEXT,
    target_server TEXT,  -- NULL für lokal
    schedule TEXT,
    enabled BOOLEAN DEFAULT 1,
    auto_resume BOOLEAN DEFAULT 1,
    last_run DATETIME,
    last_result TEXT,
    FOREIGN KEY (server_id) REFERENCES truenas_servers(id) ON DELETE CASCADE
);
```

---

## 5. Core Implementation

### 5.1 Client-Struktur (internal/truenas/client.go)

```go
package truenas

import (
    "bytes"
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
    
    "aurago/internal/config"
    "aurago/internal/security"
)

// Client repräsentiert eine Verbindung zu TrueNAS
type Client struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
    vault      *security.Vault
}

// NewClient erstellt einen neuen TrueNAS Client
func NewClient(cfg config.TrueNASConfig, vault *security.Vault) (*Client, error) {
    scheme := "http"
    if cfg.UseHTTPS {
        scheme = "https"
    }
    
    baseURL := fmt.Sprintf("%s://%s:%d/api/v2.0", scheme, cfg.Host, cfg.Port)
    
    // API Key aus Vault laden
    apiKey := cfg.APIKey
    if vault != nil && apiKey == "" {
        key, err := vault.ReadSecret("truenas_api_key")
        if err == nil && key != "" {
            apiKey = key
        }
    }
    
    if apiKey == "" {
        return nil, fmt.Errorf("no API key configured for TrueNAS")
    }
    
    tlsConfig := &tls.Config{
        InsecureSkipVerify: false,
    }
    
    // Bei Self-Signed Zertifikaten (Home Lab typisch)
    if cfg.UseHTTPS && isSelfSigned(cfg.Host) {
        tlsConfig.InsecureSkipVerify = true
    }
    
    return &Client{
        baseURL: baseURL,
        apiKey:  apiKey,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: tlsConfig,
            },
        },
        vault: vault,
    }, nil
}

// Request führt einen API-Call aus
func (c *Client) Request(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
    url := c.baseURL + endpoint
    
    var bodyReader io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("marshal body: %w", err)
        }
        bodyReader = bytes.NewReader(jsonBody)
    }
    
    req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    return c.httpClient.Do(req)
}

// Get führt einen GET-Request aus
func (c *Client) Get(ctx context.Context, endpoint string, result interface{}) error {
    resp, err := c.Request(ctx, "GET", endpoint, nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }
    
    if result != nil {
        return json.NewDecoder(resp.Body).Decode(result)
    }
    return nil
}

// Post führt einen POST-Request aus
func (c *Client) Post(ctx context.Context, endpoint string, body, result interface{}) error {
    resp, err := c.Request(ctx, "POST", endpoint, body)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }
    
    if result != nil {
        return json.NewDecoder(resp.Body).Decode(result)
    }
    return nil
}

// Delete führt einen DELETE-Request aus
func (c *Client) Delete(ctx context.Context, endpoint string) error {
    resp, err := c.Request(ctx, "DELETE", endpoint, nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }
    return nil
}

// Health prüft die Verbindung
func (c *Client) Health(ctx context.Context) (map[string]interface{}, error) {
    var result map[string]interface{}
    err := c.Get(ctx, "/system/info", &result)
    return result, err
}
```

### 5.2 Pool-Verwaltung (internal/truenas/pool.go)

```go
package truenas

import (
    "context"
    "fmt"
)

// Pool repräsentiert einen ZFS Pool
type Pool struct {
    ID       int64  `json:"id"`
    Name     string `json:"name"`
    GUID     string `json:"guid"`
    Path     string `json:"path"`
    Status   string `json:"status"`
    Scan     Scan   `json:"scan"`
    Topology struct {
        Data  []VDev `json:"data"`
        Cache []VDev `json:"cache,omitempty"`
        Log   []VDev `json:"log,omitempty"`
        Spare []VDev `json:"spare,omitempty"`
    } `json:"topology"`
    Size struct {
        Allocated int64 `json:"allocated"`
        Free      int64 `json:"free"`
        Total     int64 `json:"total"`
    } `json:"size"`
}

type Scan struct {
    Function string `json:"function"`
    State    string `json:"state"`
    StartTime string `json:"start_time,omitempty"`
    EndTime   string `json:"end_time,omitempty"`
    Percentage float64 `json:"percentage"`
}

type VDev struct {
    Type  string `json:"type"`
    Path  string `json:"path,omitempty"`
    Name  string `json:"name,omitempty"`
    Status string `json:"status"`
    Children []VDev `json:"children,omitempty"`
}

// ListPools gibt alle Pools zurück
func (c *Client) ListPools(ctx context.Context) ([]Pool, error) {
    var pools []Pool
    err := c.Get(ctx, "/pool", &pools)
    return pools, err
}

// GetPool gibt einen Pool anhand des Namens zurück
func (c *Client) GetPool(ctx context.Context, name string) (*Pool, error) {
    pools, err := c.ListPools(ctx)
    if err != nil {
        return nil, err
    }
    
    for _, p := range pools {
        if p.Name == name {
            return &p, nil
        }
    }
    return nil, fmt.Errorf("pool %q not found", name)
}

// GetPoolByID gibt einen Pool anhand der ID zurück
func (c *Client) GetPoolByID(ctx context.Context, id int) (*Pool, error) {
    var pool Pool
    err := c.Get(ctx, fmt.Sprintf("/pool/id/%d", id), &pool)
    return &pool, err
}

// ScrubPool startet einen Scrub für einen Pool
func (c *Client) ScrubPool(ctx context.Context, poolID int) error {
    return c.Post(ctx, "/pool/scrub", map[string]interface{}{
        "id": poolID,
    }, nil)
}
```

### 5.3 Dataset-Verwaltung (internal/truenas/dataset.go)

```go
package truenas

import (
    "context"
    "fmt"
)

// Dataset repräsentiert ein ZFS Dataset
type Dataset struct {
    ID         int64  `json:"id"`
    Type       string `json:"type"`  // FILESYSTEM, VOLUME
    Name       string `json:"name"`
    Pool       string `json:"pool"`
    Mountpoint string `json:"mountpoint"`
    
    // Größen
    Available struct {
        Parsed int64 `json:"parsed"`
    } `json:"available"`
    Used struct {
        Parsed int64 `json:"parsed"`
    } `json:"used"`
    
    // Eigenschaften
    Compression struct {
        Value string `json:"value"`
    } `json:"compression"`
    Quota struct {
        Parsed int64 `json:"parsed"`
    } `json:"quota"`
    Refquota struct {
        Parsed int64 `json:"parsed"`
    } `json:"refquota"`
    Reservation struct {
        Parsed int64 `json:"parsed"`
    } `json:"reservation"`
    Readonly struct {
        Value string `json:"value"`
    } `json:"readonly"`
    Exec struct {
        Value string `json:"value"`
    } `json:"exec"`
    Atime struct {
        Value string `json:"value"`
    } `json:"atime"`
    Snapdir struct {
        Value string `json:"value"`
    } `json:"snapdir"`
    ShareType string `json:"share_type,omitempty"`
    
    // Erstelldatum
    Creation struct {
        Value string `json:"$date"`
    } `json:"creation"`
}

// CreateDatasetRequest für neue Datasets
type CreateDatasetRequest struct {
    Name        string                 `json:"name"`        // z.B. "tank/share"
    Type        string                 `json:"type"`        // FILESYSTEM, VOLUME
    Comments    string                 `json:"comments,omitempty"`
    Compression string                 `json:"compression,omitempty"`
    Atime       string                 `json:"atime,omitempty"`
    Exec        string                 `json:"exec,omitempty"`
    Quota       int64                  `json:"quota,omitempty"`
    Refquota    int64                  `json:"refquota,omitempty"`
    Reservation int64                  `json:"reservation,omitempty"`
    Snapdir     string                 `json:"snapdir,omitempty"`
    Copies      int                    `json:"copies,omitempty"`
    SharesMB    map[string]interface{} `json:"sharesmb,omitempty"` // SMB-Share erstellen
}

// ListDatasets gibt alle Datasets zurück
func (c *Client) ListDatasets(ctx context.Context) ([]Dataset, error) {
    var datasets []Dataset
    err := c.Get(ctx, "/pool/dataset", &datasets)
    return datasets, err
}

// GetDataset gibt ein Dataset anhand des Namens zurück
func (c *Client) GetDataset(ctx context.Context, name string) (*Dataset, error) {
    var dataset Dataset
    err := c.Get(ctx, fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(name)), &dataset)
    return &dataset, err
}

// CreateDataset erstellt ein neues Dataset
func (c *Client) CreateDataset(ctx context.Context, req CreateDatasetRequest) (*Dataset, error) {
    var dataset Dataset
    err := c.Post(ctx, "/pool/dataset", req, &dataset)
    return &dataset, err
}

// DeleteDataset löscht ein Dataset
func (c *Client) DeleteDataset(ctx context.Context, name string, recursive bool) error {
    endpoint := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(name))
    if recursive {
        endpoint += "?recursive=true"
    }
    return c.Delete(ctx, endpoint)
}

// UpdateDataset aktualisiert Dataset-Eigenschaften
func (c *Client) UpdateDataset(ctx context.Context, name string, updates map[string]interface{}) error {
    endpoint := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(name))
    return c.Put(ctx, endpoint, updates)
}
```

### 5.4 Snapshot-Verwaltung (internal/truenas/snapshot.go)

```go
package truenas

import (
    "context"
    "fmt"
    "time"
)

// Snapshot repräsentiert einen ZFS Snapshot
type Snapshot struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`      // z.B. "tank/share@auto-2024-01-01_00-00"
    Dataset   string `json:"dataset"`   // Parent dataset
    SnapshotName string `json:"snapshot_name"` // nur der Teil nach @
    Type      string `json:"type"`
    
    // Properties
    Properties struct {
        Used struct {
            Parsed int64 `json:"parsed"`
        } `json:"used"`
        Referenced struct {
            Parsed int64 `json:"parsed"`
        } `json:"referenced"`
    } `json:"properties"`
    
    // Metadata
    Retention struct {
        Value string `json:"value"`
    } `json:"retention,omitempty"`
    
    Creation struct {
        Timestamp time.Time `json:"$date"`
    } `json:"creation"`
}

// CreateSnapshotRequest für neue Snapshots
type CreateSnapshotRequest struct {
    Dataset     string   `json:"dataset"`
    Name        string   `json:"name"`
    Recursive   bool     `json:"recursive,omitempty"`
    Retention   int      `json:"retention,omitempty"` // in Tagen
    VmwareSync  bool     `json:"vmware_sync,omitempty"`
    NamingSchema string  `json:"naming_schema,omitempty"` // z.B. "auto-%Y-%m-%d_%H-%M"
}

// ListSnapshots gibt alle Snapshots zurück
func (c *Client) ListSnapshots(ctx context.Context, filters map[string]string) ([]Snapshot, error) {
    endpoint := "/pool/snapshot"
    if dataset, ok := filters["dataset"]; ok {
        endpoint = fmt.Sprintf("/pool/snapshot?dataset=%s", url.QueryEscape(dataset))
    }
    
    var snapshots []Snapshot
    err := c.Get(ctx, endpoint, &snapshots)
    return snapshots, err
}

// CreateSnapshot erstellt einen neuen Snapshot
func (c *Client) CreateSnapshot(ctx context.Context, req CreateSnapshotRequest) (*Snapshot, error) {
    var snapshot Snapshot
    err := c.Post(ctx, "/pool/snapshot", req, &snapshot)
    return &snapshot, err
}

// DeleteSnapshot löscht einen Snapshot
func (c *Client) DeleteSnapshot(ctx context.Context, name string) error {
    endpoint := fmt.Sprintf("/pool/snapshot/id/%s", url.PathEscape(name))
    return c.Delete(ctx, endpoint)
}

// RollbackSnapshot führt einen Rollback zu einem Snapshot durch
func (c *Client) RollbackSnapshot(ctx context.Context, name string, force bool) error {
    endpoint := fmt.Sprintf("/pool/snapshot/id/%s/rollback", url.PathEscape(name))
    body := map[string]bool{
        "force": force,
    }
    return c.Post(ctx, endpoint, body, nil)
}

// CloneSnapshot klont einen Snapshot zu einem neuen Dataset
func (c *Client) CloneSnapshot(ctx context.Context, snapshotName, datasetDest string) error {
    return c.Post(ctx, "/pool/snapshot/clone", map[string]string{
        "snapshot": snapshotName,
        "dataset_dst": datasetDest,
    }, nil)
}
```

### 5.5 Share-Verwaltung (internal/truenas/share.go)

```go
package truenas

import (
    "context"
    "fmt"
)

// SMBShare repräsentiert eine SMB-Freigabe
type SMBShare struct {
    ID           int64    `json:"id"`
    Path         string   `json:"path"`
    PathSuffix   string   `json:"path_suffix,omitempty"`
    Name         string   `json:"name"`
    Purpose      string   `json:"purpose,omitempty"`
    Comment      string   `json:"comment,omitempty"`
    Enabled      bool     `json:"enabled"`
    
    // Berechtigungen
    ACL          []ACLEntry `json:"acl,omitempty"`
    
    // SMB-Optionen
    GuestOK      bool     `json:"guestok,omitempty"`
    Browseable   bool     `json:"browseable,omitempty"`
    ReadOnly     bool     `json:"ro,omitempty"`
    ShowHiddenFiles bool  `json:"showhiddenfiles,omitempty"`
    HomeShare    bool     `json:"home_share,omitempty"`
    Timemachine  bool     `json:"timemachine,omitempty"`
    
    // Recycling
    RecycleBin   bool     `json:"recyclebin,omitempty"`
    
    // Shadow Copies
    ShadowCopy   bool     `json:"shadowcopy,omitempty"`
    
    // Aux Parameters
    Auxiliary    string   `json:"auxsmbconf,omitempty"`
}

type ACLEntry struct {
    ID      int64  `json:"id,omitempty"`
    Tag     string `json:"tag"`     // USER, GROUP, OWNER@, GROUP@, EVERYONE@
    Who     string `json:"who,omitempty"`
    Type    string `json:"type"`    // ALLOW, DENY
    Perms   Permissions `json:"perms"`
}

type Permissions struct {
    ReadData      bool `json:"READ_DATA"`
    WriteData     bool `json:"WRITE_DATA"`
    Execute       bool `json:"EXECUTE"`
    AppendData    bool `json:"APPEND_DATA"`
    DeleteChild   bool `json:"DELETE_CHILD"`
    Delete        bool `json:"DELETE"`
    ReadAttributes bool `json:"READ_ATTRIBUTES"`
    WriteAttributes bool `json:"WRITE_ATTRIBUTES"`
    ReadXattr     bool `json:"READ_NAMED_ATTRS"`
    WriteXattr    bool `json:"WRITE_NAMED_ATTRS"`
    ReadACL       bool `json:"READ_ACL"`
    WriteACL      bool `json:"WRITE_ACL"`
    WriteOwner    bool `json:"WRITE_OWNER"`
    Synchronize   bool `json:"SYNCHRONIZE"`
    Basic         int  `json:"BASIC,omitempty"` // 0 = NONE, 1 = READ, 2 = MODIFY, 3 = FULL_CONTROL
}

// NFSShare repräsentiert eine NFS-Freigabe
type NFSShare struct {
    ID          int64    `json:"id"`
    Path        string   `json:"path"`
    Comment     string   `json:"comment,omitempty"`
    Enabled     bool     `json:"enabled"`
    
    // Clients
    Networks    []string `json:"networks,omitempty"`
    Hosts       []string `json:"hosts,omitempty"`
    
    // Optionen
    ReadOnly    bool     `json:"ro,omitempty"`
    Quiet       bool     `json:"quiet,omitempty"`
    MaprootUser string   `json:"maproot_user,omitempty"`
    MaprootGroup string `json:"maproot_group,omitempty"`
    MapallUser  string  `json:"mapall_user,omitempty"`
    MapallGroup string  `json:"mapall_group,omitempty"`
    Security    []string `json:"security,omitempty"` // SYS, KRB5, KRB5I, KRB5P
}

// ListSMBShares gibt alle SMB-Freigaben zurück
func (c *Client) ListSMBShares(ctx context.Context) ([]SMBShare, error) {
    var shares []SMBShare
    err := c.Get(ctx, "/sharing/smb", &shares)
    return shares, err
}

// CreateSMBShare erstellt eine neue SMB-Freigabe
func (c *Client) CreateSMBShare(ctx context.Context, share SMBShare) (*SMBShare, error) {
    var result SMBShare
    err := c.Post(ctx, "/sharing/smb", share, &result)
    return &result, err
}

// DeleteSMBShare löscht eine SMB-Freigabe
func (c *Client) DeleteSMBShare(ctx context.Context, id int64) error {
    return c.Delete(ctx, fmt.Sprintf("/sharing/smb/id/%d", id))
}

// ListNFSShares gibt alle NFS-Freigaben zurück
func (c *Client) ListNFSShares(ctx context.Context) ([]NFSShare, error) {
    var shares []NFSShare
    err := c.Get(ctx, "/sharing/nfs", &shares)
    return shares, err
}

// CreateNFSShare erstellt eine neue NFS-Freigabe
func (c *Client) CreateNFSShare(ctx context.Context, share NFSShare) (*NFSShare, error) {
    var result NFSShare
    err := c.Post(ctx, "/sharing/nfs", share, &result)
    return &result, err
}
```

---

## 6. Tool-Integration

### 6.1 Tool-Definitionen (internal/tools/truenas.go)

```go
package tools

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log/slog"
    "strings"
    
    "aurago/internal/config"
    "aurago/internal/truenas"
)

// TrueNASPoolList zeigt alle Pools an
func TrueNASPoolList(cfg config.TrueNASConfig, db *sql.DB, logger *slog.Logger) string {
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    pools, err := client.ListPools(context.Background())
    if err != nil {
        return errJSON("Failed to list pools: %v", err)
    }
    
    // Sync mit lokaler Registry
    if db != nil {
        for _, p := range pools {
            syncPoolToDB(db, cfg.Host, p)
        }
    }
    
    return okJSON("pools", pools)
}

// TrueNASDatasetCreate erstellt ein Dataset
func TrueNASDatasetCreate(cfg config.TrueNASConfig, name, compression string, quotaGB int64, logger *slog.Logger) string {
    if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
        return errJSON("Invalid dataset name")
    }
    
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    req := truenas.CreateDatasetRequest{
        Name:        name,
        Type:        "FILESYSTEM",
        Compression: compression,
    }
    
    if quotaGB > 0 {
        req.Quota = quotaGB * 1024 * 1024 * 1024
    }
    
    dataset, err := client.CreateDataset(context.Background(), req)
    if err != nil {
        return errJSON("Failed to create dataset: %v", err)
    }
    
    return okJSON("dataset", dataset)
}

// TrueNASSnapshotCreate erstellt einen Snapshot
func TrueNASSnapshotCreate(cfg config.TrueNASConfig, dataset, name string, recursive bool, retentionDays int, logger *slog.Logger) string {
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    // Generiere Namen wenn nicht angegeben
    if name == "" {
        name = fmt.Sprintf("aura-%s", time.Now().Format("20060102-150405"))
    }
    
    req := truenas.CreateSnapshotRequest{
        Dataset:   dataset,
        Name:      name,
        Recursive: recursive,
        Retention: retentionDays,
    }
    
    snapshot, err := client.CreateSnapshot(context.Background(), req)
    if err != nil {
        return errJSON("Failed to create snapshot: %v", err)
    }
    
    return okJSON("snapshot", snapshot)
}

// TrueNASSnapshotList listet Snapshots
func TrueNASSnapshotList(cfg config.TrueNASConfig, dataset string, logger *slog.Logger) string {
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    filters := map[string]string{}
    if dataset != "" {
        filters["dataset"] = dataset
    }
    
    snapshots, err := client.ListSnapshots(context.Background(), filters)
    if err != nil {
        return errJSON("Failed to list snapshots: %v", err)
    }
    
    return okJSON("snapshots", snapshots)
}

// TrueNASSnapshotRollback führt Rollback durch
func TrueNASSnapshotRollback(cfg config.TrueNASConfig, snapshotName string, force bool, logger *slog.Logger) string {
    if !cfg.AllowDestructive {
        return errJSON("Destructive operations not allowed (set allow_destructive: true)")
    }
    
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    err = client.RollbackSnapshot(context.Background(), snapshotName, force)
    if err != nil {
        return errJSON("Failed to rollback: %v", err)
    }
    
    return okJSON("message", "Rollback completed successfully")
}

// TrueNASHealth gibt Health-Status zurück
func TrueNASHealth(cfg config.TrueNASConfig, logger *slog.Logger) string {
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    ctx := context.Background()
    
    // System-Info
    info, err := client.Health(ctx)
    if err != nil {
        return errJSON("Failed to get health: %v", err)
    }
    
    // Pools
    pools, err := client.ListPools(ctx)
    if err != nil {
        return errJSON("Failed to list pools: %v", err)
    }
    
    // Alerts (wenn verfügbar)
    alerts, _ := client.ListAlerts(ctx)
    
    return okJSON("health", map[string]interface{}{
        "system": info,
        "pools": pools,
        "alerts": alerts,
        "status": "healthy", // oder "degraded", "error"
    })
}

// TrueNASSMBCreate erstellt SMB-Freigabe
func TrueNASSMBCreate(cfg config.TrueNASConfig, name, path string, guestOK, timemachine bool, logger *slog.Logger) string {
    client, err := truenas.NewClient(cfg, nil)
    if err != nil {
        return errJSON("Failed to create client: %v", err)
    }
    
    share := truenas.SMBShare{
        Name:       name,
        Path:       path,
        Enabled:    true,
        GuestOK:    guestOK,
        Timemachine: timemachine,
        Browseable: true,
        ShadowCopy: true,
        RecycleBin: true,
    }
    
    result, err := client.CreateSMBShare(context.Background(), share)
    if err != nil {
        return errJSON("Failed to create SMB share: %v", err)
    }
    
    return okJSON("share", result)
}
```

### 6.2 Dispatch-Integration

```go
// DispatchTrueNASTool routed TrueNAS Tool-Calls
func DispatchTrueNASTool(name string, params map[string]interface{}, cfg *config.Config, db *sql.DB, logger *slog.Logger) string {
    if !cfg.TrueNAS.Enabled {
        return errJSON("TrueNAS integration is disabled")
    }
    
    switch name {
    case "truenas_pool_list":
        return TrueNASPoolList(cfg.TrueNAS, db, logger)
        
    case "truenas_dataset_create":
        name := getString(params, "name")
        compression := getString(params, "compression", "lz4")
        quotaGB := getInt64(params, "quota_gb", 0)
        return TrueNASDatasetCreate(cfg.TrueNAS, name, compression, quotaGB, logger)
        
    case "truenas_dataset_list":
        return TrueNASDatasetList(cfg.TrueNAS, getString(params, "pool"), logger)
        
    case "truenas_dataset_delete":
        if !cfg.TrueNAS.AllowDestructive {
            return errJSON("Destructive operations not allowed")
        }
        return TrueNASDatasetDelete(cfg.TrueNAS, getString(params, "name"), 
            getBool(params, "recursive", false), logger)
        
    case "truenas_snapshot_create":
        dataset := getString(params, "dataset")
        name := getString(params, "name", "")
        recursive := getBool(params, "recursive", false)
        retention := getInt(params, "retention_days", 0)
        return TrueNASSnapshotCreate(cfg.TrueNAS, dataset, name, recursive, retention, logger)
        
    case "truenas_snapshot_list":
        return TrueNASSnapshotList(cfg.TrueNAS, getString(params, "dataset", ""), logger)
        
    case "truenas_snapshot_delete":
        return TrueNASSnapshotDelete(cfg.TrueNAS, getString(params, "name"), logger)
        
    case "truenas_snapshot_rollback":
        return TrueNASSnapshotRollback(cfg.TrueNAS, getString(params, "name"),
            getBool(params, "force", false), logger)
        
    case "truenas_smb_create":
        return TrueNASSMBCreate(cfg.TrueNAS, getString(params, "name"),
            getString(params, "path"), getBool(params, "guest_ok", false),
            getBool(params, "timemachine", false), logger)
            
    case "truenas_smb_list":
        return TrueNASSMBList(cfg.TrueNAS, logger)
        
    case "truenas_health":
        return TrueNASHealth(cfg.TrueNAS, logger)
        
    case "truenas_sync_to_registry":
        return TrueNASSyncToRegistry(cfg.TrueNAS, db, logger)
        
    default:
        return errJSON("Unknown TrueNAS tool: %s", name)
    }
}
```

---

## 7. Scheduler-Integration

### 7.1 Automatische Snapshots (internal/truenas/scheduler.go)

```go
package truenas

import (
    "context"
    "database/sql"
    "log/slog"
    "time"
    
    "github.com/robfig/cron/v3"
)

// SnapshotJob repräsentiert einen geplanten Snapshot
type SnapshotJob struct {
    ID            int64
    Dataset       string
    NamePattern   string  // z.B. "auto-%Y-%m-%d_%H-%M"
    Schedule      string  // Cron expression
    Recursive     bool
    RetentionDays int
    Enabled       bool
}

// Scheduler verwaltet geplante Snapshots
type Scheduler struct {
    cron   *cron.Cron
    db     *sql.DB
    client *Client
    logger *slog.Logger
    jobs   map[int64]cron.EntryID
}

// NewScheduler erstellt einen neuen Scheduler
func NewScheduler(db *sql.DB, client *Client, logger *slog.Logger) *Scheduler {
    return &Scheduler{
        cron:   cron.New(cron.WithSeconds()),
        db:     db,
        client: client,
        logger: logger,
        jobs:   make(map[int64]cron.EntryID),
    }
}

// Start initialisiert alle Jobs
func (s *Scheduler) Start() error {
    rows, err := s.db.Query(`
        SELECT id, dataset, name_pattern, schedule, recursive, retention_days 
        FROM truenas_snapshot_jobs 
        WHERE enabled = 1
    `)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var job SnapshotJob
        if err := rows.Scan(&job.ID, &job.Dataset, &job.NamePattern, 
            &job.Schedule, &job.Recursive, &job.RetentionDays); err != nil {
            s.logger.Error("Failed to scan job", "error", err)
            continue
        }
        s.addJob(job)
    }
    
    s.cron.Start()
    return nil
}

// Stop hält den Scheduler an
func (s *Scheduler) Stop() {
    s.cron.Stop()
}

func (s *Scheduler) addJob(job SnapshotJob) {
    entryID, err := s.cron.AddFunc(job.Schedule, func() {
        s.executeSnapshot(job)
    })
    if err != nil {
        s.logger.Error("Failed to add cron job", "job", job.ID, "error", err)
        return
    }
    s.jobs[job.ID] = entryID
}

func (s *Scheduler) executeSnapshot(job SnapshotJob) {
    ctx := context.Background()
    
    name := time.Now().Format(job.NamePattern)
    
    req := CreateSnapshotRequest{
        Dataset:   job.Dataset,
        Name:      name,
        Recursive: job.Recursive,
        Retention: job.RetentionDays,
    }
    
    snapshot, err := s.client.CreateSnapshot(ctx, req)
    if err != nil {
        s.logger.Error("Scheduled snapshot failed", 
            "dataset", job.Dataset, 
            "error", err)
        return
    }
    
    s.logger.Info("Scheduled snapshot created",
        "dataset", job.Dataset,
        "name", snapshot.Name)
    
    // Alte Snapshots bereinigen
    if job.RetentionDays > 0 {
        s.cleanupOldSnapshots(ctx, job)
    }
}

func (s *Scheduler) cleanupOldSnapshots(ctx context.Context, job SnapshotJob) {
    cutoff := time.Now().AddDate(0, 0, -job.RetentionDays)
    
    snapshots, err := s.client.ListSnapshots(ctx, map[string]string{
        "dataset": job.Dataset,
    })
    if err != nil {
        s.logger.Error("Failed to list snapshots for cleanup", "error", err)
        return
    }
    
    for _, snap := range snapshots {
        if snap.Creation.Timestamp.Before(cutoff) && 
           strings.HasPrefix(snap.SnapshotName, strings.Split(job.NamePattern, "%")[0]) {
            if err := s.client.DeleteSnapshot(ctx, snap.Name); err != nil {
                s.logger.Error("Failed to delete old snapshot", 
                    "snapshot", snap.Name, 
                    "error", err)
            } else {
                s.logger.Info("Deleted old snapshot", "snapshot", snap.Name)
            }
        }
    }
}
```

---

## 8. Web UI Integration

### 8.1 Neue UI-Seite: truenas.html

```html
<!-- ui/truenas.html -->
<!DOCTYPE html>
<html lang="de">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TrueNAS - AuraGo</title>
    <link rel="stylesheet" href="/css/truenas.css">
</head>
<body>
    <div class="truenas-container">
        <!-- Header -->
        <header class="truenas-header">
            <h1><i class="icon-storage"></i> TrueNAS</h1>
            <div class="connection-status">
                <span id="status-indicator" class="status-unknown">Prüfe Verbindung...</span>
            </div>
        </header>
        
        <!-- Navigation -->
        <nav class="truenas-nav">
            <button class="nav-btn active" data-tab="overview">Übersicht</button>
            <button class="nav-btn" data-tab="pools">Pools</button>
            <button class="nav-btn" data-tab="datasets">Datasets</button>
            <button class="nav-btn" data-tab="snapshots">Snapshots</button>
            <button class="nav-btn" data-tab="shares">Freigaben</button>
            <button class="nav-btn" data-tab="settings">Einstellungen</button>
        </nav>
        
        <!-- Content -->
        <main class="truenas-content">
            <!-- Overview Tab -->
            <div id="tab-overview" class="tab-content active">
                <div class="stats-grid">
                    <div class="stat-card">
                        <h3>Pools</h3>
                        <div class="stat-value" id="pool-count">-</div>
                        <div class="stat-detail" id="pool-status"></div>
                    </div>
                    <div class="stat-card">
                        <h3>Speicher</h3>
                        <div class="stat-value" id="total-storage">-</div>
                        <div class="progress-bar">
                            <div class="progress-fill" id="storage-progress"></div>
                        </div>
                    </div>
                    <div class="stat-card">
                        <h3>Snapshots</h3>
                        <div class="stat-value" id="snapshot-count">-</div>
                        <div class="stat-detail">Letzte 24h: <span id="recent-snapshots">-</span></div>
                    </div>
                    <div class="stat-card">
                        <h3>Freigaben</h3>
                        <div class="stat-value" id="share-count">-</div>
                        <div class="stat-detail">SMB: <span id="smb-count">-</span> | NFS: <span id="nfs-count">-</span></div>
                    </div>
                </div>
                
                <!-- Health Alerts -->
                <div class="alerts-section" id="health-alerts">
                    <h2>System Status</h2>
                    <div id="alerts-container"></div>
                </div>
            </div>
            
            <!-- Pools Tab -->
            <div id="tab-pools" class="tab-content">
                <div class="section-header">
                    <h2>Storage Pools</h2>
                    <button class="btn btn-refresh" onclick="refreshPools()">
                        <i class="icon-refresh"></i> Aktualisieren
                    </button>
                </div>
                <div id="pools-container" class="pools-grid"></div>
            </div>
            
            <!-- Datasets Tab -->
            <div id="tab-datasets" class="tab-content">
                <div class="section-header">
                    <h2>Datasets</h2>
                    <button class="btn btn-primary" onclick="showCreateDataset()">
                        <i class="icon-add"></i> Neues Dataset
                    </button>
                </div>
                <div id="datasets-container" class="datasets-tree"></div>
            </div>
            
            <!-- Snapshots Tab -->
            <div id="tab-snapshots" class="tab-content">
                <div class="section-header">
                    <h2>Snapshots</h2>
                    <div class="filter-controls">
                        <select id="snapshot-pool-filter" onchange="filterSnapshots()">
                            <option value="">Alle Pools</option>
                        </select>
                        <button class="btn btn-primary" onclick="showCreateSnapshot()">
                            <i class="icon-camera"></i> Snapshot erstellen
                        </button>
                    </div>
                </div>
                <div id="snapshots-container" class="snapshots-list"></div>
            </div>
            
            <!-- Shares Tab -->
            <div id="tab-shares" class="tab-content">
                <div class="section-header">
                    <h2>Freigaben</h2>
                    <button class="btn btn-primary" onclick="showCreateShare()">
                        <i class="icon-share"></i> Neue Freigabe
                    </button>
                </div>
                <div id="shares-container" class="shares-list"></div>
            </div>
            
            <!-- Settings Tab -->
            <div id="tab-settings" class="tab-content">
                <h2>TrueNAS Einstellungen</h2>
                <form id="truenas-settings-form">
                    <div class="form-group">
                        <label>Host</label>
                        <input type="text" id="setting-host" placeholder="truenas.local">
                    </div>
                    <div class="form-group">
                        <label>API Key</label>
                        <input type="password" id="setting-apikey" placeholder="API Key aus TrueNAS">
                        <small>Erstellen Sie einen API Key unter System → API Keys</small>
                    </div>
                    <div class="form-group">
                        <label class="checkbox">
                            <input type="checkbox" id="setting-https" checked>
                            HTTPS verwenden
                        </label>
                    </div>
                    <div class="form-group">
                        <label class="checkbox">
                            <input type="checkbox" id="setting-destructive">
                            Destruktive Operationen erlauben (Löschen)
                        </label>
                    </div>
                    <button type="submit" class="btn btn-primary">Speichern</button>
                    <button type="button" class="btn btn-secondary" onclick="testConnection()">
                        Verbindung testen
                    </button>
                </form>
            </div>
        </main>
    </div>
    
    <!-- Modal: Create Dataset -->
    <div id="modal-dataset" class="modal">
        <div class="modal-content">
            <h3>Neues Dataset erstellen</h3>
            <form id="dataset-form">
                <div class="form-group">
                    <label>Name (z.B. tank/share)</label>
                    <input type="text" id="dataset-name" required>
                </div>
                <div class="form-group">
                    <label>Kompression</label>
                    <select id="dataset-compression">
                        <option value="lz4" selected>LZ4 (empfohlen)</option>
                        <option value="gzip">GZIP</option>
                        <option value="zle">ZLE</option>
                        <option value="off">Aus</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>Quota (GB, optional)</label>
                    <input type="number" id="dataset-quota" min="0" placeholder="Unbegrenzt">
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeModal()">Abbrechen</button>
                    <button type="submit" class="btn btn-primary">Erstellen</button>
                </div>
            </form>
        </div>
    </div>
    
    <!-- Modal: Create Snapshot -->
    <div id="modal-snapshot" class="modal">
        <div class="modal-content">
            <h3>Snapshot erstellen</h3>
            <form id="snapshot-form">
                <div class="form-group">
                    <label>Dataset</label>
                    <select id="snapshot-dataset" required></select>
                </div>
                <div class="form-group">
                    <label>Name (optional, auto-generiert)</label>
                    <input type="text" id="snapshot-name" placeholder="aura-20240101-120000">
                </div>
                <div class="form-group">
                    <label class="checkbox">
                        <input type="checkbox" id="snapshot-recursive">
                        Rekursiv (inkl. Unter-Datasets)
                    </label>
                </div>
                <div class="form-group">
                    <label>Aufbewahrung (Tage, optional)</label>
                    <input type="number" id="snapshot-retention" min="0" placeholder="Für immer">
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeModal()">Abbrechen</button>
                    <button type="submit" class="btn btn-primary">Erstellen</button>
                </div>
            </form>
        </div>
    </div>
    
    <script src="/js/truenas.js"></script>
</body>
</html>
```

### 8.2 JavaScript-Client (ui/js/truenas.js)

```javascript
// ui/js/truenas.js

class TrueNASUI {
    constructor() {
        this.baseUrl = '/api/truenas';
        this.init();
    }
    
    init() {
        this.bindEvents();
        this.loadOverview();
        this.startHealthCheck();
    }
    
    bindEvents() {
        // Tab Navigation
        document.querySelectorAll('.nav-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.switchTab(e.target.dataset.tab));
        });
        
        // Forms
        document.getElementById('truenas-settings-form')?.addEventListener('submit', (e) => this.saveSettings(e));
        document.getElementById('dataset-form')?.addEventListener('submit', (e) => this.createDataset(e));
        document.getElementById('snapshot-form')?.addEventListener('submit', (e) => this.createSnapshot(e));
    }
    
    async switchTab(tabName) {
        // Update nav
        document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
        document.querySelector(`[data-tab="${tabName}"]`)?.classList.add('active');
        
        // Update content
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        document.getElementById(`tab-${tabName}`)?.classList.add('active');
        
        // Load data
        switch(tabName) {
            case 'pools': await this.loadPools(); break;
            case 'datasets': await this.loadDatasets(); break;
            case 'snapshots': await this.loadSnapshots(); break;
            case 'shares': await this.loadShares(); break;
        }
    }
    
    async loadOverview() {
        try {
            const response = await fetch(`${this.baseUrl}/overview`);
            const data = await response.json();
            
            if (data.status === 'ok') {
                this.updateOverview(data.data);
            }
        } catch (err) {
            console.error('Failed to load overview:', err);
        }
    }
    
    updateOverview(data) {
        document.getElementById('pool-count').textContent = data.pools?.length || 0;
        document.getElementById('snapshot-count').textContent = data.snapshot_count || 0;
        document.getElementById('share-count').textContent = (data.smb_shares?.length || 0) + (data.nfs_shares?.length || 0);
        
        // Storage calculation
        let total = 0, used = 0;
        data.pools?.forEach(p => {
            total += p.size?.total || 0;
            used += p.size?.allocated || 0;
        });
        
        const percent = total > 0 ? (used / total * 100).toFixed(1) : 0;
        document.getElementById('total-storage').textContent = this.formatBytes(total);
        document.getElementById('storage-progress').style.width = `${percent}%`;
        document.getElementById('storage-progress').className = `progress-fill ${percent > 90 ? 'danger' : percent > 70 ? 'warning' : ''}`;
    }
    
    async loadPools() {
        const container = document.getElementById('pools-container');
        container.innerHTML = '<div class="loading">Lade Pools...</div>';
        
        try {
            const response = await fetch(`${this.baseUrl}/pools`);
            const data = await response.json();
            
            if (data.status === 'ok') {
                this.renderPools(data.pools);
            }
        } catch (err) {
            container.innerHTML = `<div class="error">Fehler: ${err.message}</div>`;
        }
    }
    
    renderPools(pools) {
        const container = document.getElementById('pools-container');
        
        if (!pools?.length) {
            container.innerHTML = '<div class="empty">Keine Pools gefunden</div>';
            return;
        }
        
        container.innerHTML = pools.map(pool => `
            <div class="pool-card ${pool.status?.toLowerCase()}">
                <div class="pool-header">
                    <h3>${pool.name}</h3>
                    <span class="status-badge ${pool.status?.toLowerCase()}">${pool.status}</span>
                </div>
                <div class="pool-stats">
                    <div class="stat">
                        <label>Größe</label>
                        <value>${this.formatBytes(pool.size?.total)}</value>
                    </div>
                    <div class="stat">
                        <label>Belegt</label>
                        <value>${this.formatBytes(pool.size?.allocated)}</value>
                    </div>
                    <div class="stat">
                        <label>Frei</label>
                        <value>${this.formatBytes(pool.size?.free)}</value>
                    </div>
                </div>
                <div class="pool-usage">
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${(pool.size?.allocated / pool.size?.total * 100).toFixed(1)}%"></div>
                    </div>
                </div>
                <div class="pool-actions">
                    <button onclick="truenasUI.scrubPool('${pool.id}')">Scrub starten</button>
                    <button onclick="truenasUI.viewPoolDetails('${pool.id}')">Details</button>
                </div>
            </div>
        `).join('');
    }
    
    async createDataset(e) {
        e.preventDefault();
        
        const name = document.getElementById('dataset-name').value;
        const compression = document.getElementById('dataset-compression').value;
        const quota = parseInt(document.getElementById('dataset-quota').value) || 0;
        
        try {
            const response = await fetch(`${this.baseUrl}/datasets`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, compression, quota_gb: quota })
            });
            
            const data = await response.json();
            
            if (data.status === 'ok') {
                this.showToast('Dataset erfolgreich erstellt', 'success');
                this.closeModal();
                this.loadDatasets();
            } else {
                this.showToast(data.message, 'error');
            }
        } catch (err) {
            this.showToast(`Fehler: ${err.message}`, 'error');
        }
    }
    
    async createSnapshot(e) {
        e.preventDefault();
        
        const dataset = document.getElementById('snapshot-dataset').value;
        const name = document.getElementById('snapshot-name').value;
        const recursive = document.getElementById('snapshot-recursive').checked;
        const retention = parseInt(document.getElementById('snapshot-retention').value) || 0;
        
        try {
            const response = await fetch(`${this.baseUrl}/snapshots`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ dataset, name, recursive, retention_days: retention })
            });
            
            const data = await response.json();
            
            if (data.status === 'ok') {
                this.showToast('Snapshot erstellt', 'success');
                this.closeModal();
                this.loadSnapshots();
            } else {
                this.showToast(data.message, 'error');
            }
        } catch (err) {
            this.showToast(`Fehler: ${err.message}`, 'error');
        }
    }
    
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
    
    showToast(message, type = 'info') {
        // Implementierung basierend auf bestehendem Toast-System
        console.log(`[${type}] ${message}`);
    }
    
    closeModal() {
        document.querySelectorAll('.modal').forEach(m => m.classList.remove('active'));
    }
    
    startHealthCheck() {
        setInterval(() => this.checkHealth(), 30000); // Alle 30s
    }
    
    async checkHealth() {
        try {
            const response = await fetch(`${this.baseUrl}/health`);
            const data = await response.json();
            
            const indicator = document.getElementById('status-indicator');
            if (data.status === 'ok') {
                indicator.className = 'status-online';
                indicator.textContent = 'Verbunden';
            } else {
                indicator.className = 'status-error';
                indicator.textContent = 'Fehler';
            }
        } catch (err) {
            document.getElementById('status-indicator').className = 'status-offline';
            document.getElementById('status-indicator').textContent = 'Offline';
        }
    }
}

// Global instance
const truenasUI = new TrueNASUI();
```

---

## 9. Server-Handler (internal/server/truenas_handlers.go)

```go
package server

import (
    "encoding/json"
    "net/http"
    "strconv"
    
    "aurago/internal/tools"
)

// RegisterTrueNASHandlers registriert alle TrueNAS Endpoints
func (s *Server) RegisterTrueNASHandlers() {
    s.Router.HandleFunc("/api/truenas/overview", s.handleTrueNASOverview()).Methods("GET")
    s.Router.HandleFunc("/api/truenas/pools", s.handleTrueNASPools()).Methods("GET")
    s.Router.HandleFunc("/api/truenas/datasets", s.handleTrueNASDatasets()).Methods("GET", "POST")
    s.Router.HandleFunc("/api/truenas/snapshots", s.handleTrueNASSnapshots()).Methods("GET", "POST")
    s.Router.HandleFunc("/api/truenas/snapshots/{name}/rollback", s.handleTrueNASSnapshotRollback()).Methods("POST")
    s.Router.HandleFunc("/api/truenas/shares", s.handleTrueNASShares()).Methods("GET", "POST")
    s.Router.HandleFunc("/api/truenas/health", s.handleTrueNASHealth()).Methods("GET")
    s.Router.HandleFunc("/api/truenas/settings", s.handleTrueNASSettings()).Methods("GET", "POST")
}

func (s *Server) handleTrueNASOverview() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        s.CfgMu.RLock()
        cfg := s.Cfg.TrueNAS
        s.CfgMu.RUnlock()
        
        if !cfg.Enabled {
            json.NewEncoder(w).Encode(map[string]interface{}{
                "status": "disabled",
            })
            return
        }
        
        result := tools.TrueNASHealth(cfg, s.Logger)
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(result))
    }
}

func (s *Server) handleTrueNASPools() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        s.CfgMu.RLock()
        cfg := s.Cfg.TrueNAS
        s.CfgMu.RUnlock()
        
        result := tools.TrueNASPoolList(cfg, s.DB, s.Logger)
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(result))
    }
}

func (s *Server) handleTrueNASDatasets() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        s.CfgMu.RLock()
        cfg := s.Cfg.TrueNAS
        s.CfgMu.RUnlock()
        
        switch r.Method {
        case "GET":
            result := tools.TrueNASDatasetList(cfg, r.URL.Query().Get("pool"), s.Logger)
            w.Write([]byte(result))
            
        case "POST":
            var req struct {
                Name        string `json:"name"`
                Compression string `json:"compression"`
                QuotaGB     int64  `json:"quota_gb"`
            }
            json.NewDecoder(r.Body).Decode(&req)
            
            result := tools.TrueNASDatasetCreate(cfg, req.Name, req.Compression, req.QuotaGB, s.Logger)
            w.Write([]byte(result))
        }
    }
}

func (s *Server) handleTrueNASSnapshots() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        s.CfgMu.RLock()
        cfg := s.Cfg.TrueNAS
        s.CfgMu.RUnlock()
        
        switch r.Method {
        case "GET":
            result := tools.TrueNASSnapshotList(cfg, r.URL.Query().Get("dataset"), s.Logger)
            w.Write([]byte(result))
            
        case "POST":
            var req struct {
                Dataset       string `json:"dataset"`
                Name          string `json:"name"`
                Recursive     bool   `json:"recursive"`
                RetentionDays int    `json:"retention_days"`
            }
            json.NewDecoder(r.Body).Decode(&req)
            
            result := tools.TrueNASSnapshotCreate(cfg, req.Dataset, req.Name, 
                req.Recursive, req.RetentionDays, s.Logger)
            w.Write([]byte(result))
        }
    }
}

func (s *Server) handleTrueNASSnapshotRollback() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        snapshotName := vars["name"]
        
        var req struct {
            Force bool `json:"force"`
        }
        json.NewDecoder(r.Body).Decode(&req)
        
        s.CfgMu.RLock()
        cfg := s.Cfg.TrueNAS
        s.CfgMu.RUnlock()
        
        result := tools.TrueNASSnapshotRollback(cfg, snapshotName, req.Force, s.Logger)
        w.Write([]byte(result))
    }
}
```

---

## 10. Tool Manifest & Prompts

### 10.1 Tool Manifest (agent_workspace/tools/manifest.json)

```json
{
  "truenas": {
    "name": "TrueNAS Storage Management",
    "description": "Manage TrueNAS SCALE/Core storage, snapshots, and shares",
    "version": "1.0.0",
    "tools": [
      {
        "name": "truenas_health",
        "description": "Get TrueNAS system health status, pool states, and alerts",
        "parameters": {}
      },
      {
        "name": "truenas_pool_list",
        "description": "List all ZFS pools with status, size, and usage",
        "parameters": {}
      },
      {
        "name": "truenas_dataset_create",
        "description": "Create a new ZFS dataset",
        "parameters": {
          "name": {"type": "string", "description": "Full dataset path (e.g., 'tank/share')", "required": true},
          "compression": {"type": "string", "description": "Compression algorithm (lz4, gzip, zle, off)", "default": "lz4"},
          "quota_gb": {"type": "integer", "description": "Quota in GB (0 = unlimited)", "default": 0}
        }
      },
      {
        "name": "truenas_dataset_list",
        "description": "List datasets, optionally filtered by pool",
        "parameters": {
          "pool": {"type": "string", "description": "Filter by pool name", "required": false}
        }
      },
      {
        "name": "truenas_snapshot_create",
        "description": "Create a ZFS snapshot",
        "parameters": {
          "dataset": {"type": "string", "description": "Dataset to snapshot", "required": true},
          "name": {"type": "string", "description": "Snapshot name (auto-generated if empty)", "required": false},
          "recursive": {"type": "boolean", "description": "Include child datasets", "default": false},
          "retention_days": {"type": "integer", "description": "Auto-delete after N days (0 = keep forever)", "default": 0}
        }
      },
      {
        "name": "truenas_snapshot_list",
        "description": "List snapshots for a dataset or all datasets",
        "parameters": {
          "dataset": {"type": "string", "description": "Filter by dataset", "required": false}
        }
      },
      {
        "name": "truenas_snapshot_rollback",
        "description": "Rollback dataset to a snapshot (DESTRUCTIVE)",
        "parameters": {
          "name": {"type": "string", "description": "Full snapshot name (e.g., 'tank/share@snapshot-1')", "required": true},
          "force": {"type": "boolean", "description": "Force rollback even with clones", "default": false}
        }
      },
      {
        "name": "truenas_smb_create",
        "description": "Create SMB share for a dataset",
        "parameters": {
          "name": {"type": "string", "description": "Share name", "required": true},
          "path": {"type": "string", "description": "Dataset path", "required": true},
          "guest_ok": {"type": "boolean", "description": "Allow guest access", "default": false},
          "timemachine": {"type": "boolean", "description": "Enable Time Machine support", "default": false}
        }
      },
      {
        "name": "truenas_smb_list",
        "description": "List all SMB shares",
        "parameters": {}
      },
      {
        "name": "truenas_sync_to_registry",
        "description": "Synchronize TrueNAS data to local registry",
        "parameters": {}
      }
    ]
  }
}
```

### 10.2 Tool Manual (prompts/tools_manuals/truenas.md)

```markdown
# TrueNAS Storage Management

TrueNAS SCALE/Core Integration für Storage-Verwaltung in Home Labs.

## Verfügbare Tools

### truenas_health
Zeigt System-Status, Pool-Zustände und Alerts an.

**Verwendung:**
```
truenas_health
```

**Rückgabe:**
- System-Informationen (Version, uptime)
- Pool-Status (ONLINE, DEGRADED, FAULTED)
- Alerts und Warnungen

### truenas_pool_list
Listet alle ZFS Pools mit Kapazität und Status.

### truenas_dataset_create
Erstellt ein neues ZFS Dataset (logischer Speicherbereich).

**Parameter:**
- name: Vollständiger Pfad (z.B. "tank/movies")
- compression: lz4 (empfohlen), gzip, zle, off
- quota_gb: Maximale Größe in GB

**Beispiel:**
```
truenas_dataset_create name="tank/backups" compression=lz4 quota_gb=500
```

### truenas_snapshot_create
Erstellt einen ZFS Snapshot für Backup/Wiederherstellung.

**Parameter:**
- dataset: Zu sicherndes Dataset
- name: Optionaler Name (Format: prefix-YYYYMMDD-HHMMSS)
- recursive: Kind-Datasets einbeziehen
- retention_days: Automatische Löschung nach N Tagen

**Beispiele:**
```
# Manueller Snapshot
truenas_snapshot_create dataset="tank/documents" name="pre-update"

# Automatisch benannter, rekursiver Snapshot mit Aufbewahrung
truenas_snapshot_create dataset="tank" recursive=true retention_days=30
```

### truenas_snapshot_rollback
Stellt Dataset zu einem früheren Snapshot wieder her.

⚠️ **ACHTUNG:** Zerstört alle Daten nach dem Snapshot!

**Parameter:**
- name: Vollständiger Snapshot-Name (z.B. "tank/docs@pre-update")
- force: Erzwingt Rollback trotz Klons

### truenas_smb_create
Erstellt SMB-Freigabe für Windows/macOS-Zugriff.

**Parameter:**
- name: Freigabename (wird im Netzwerk angezeigt)
- path: Dataset-Pfad
- guest_ok: Gast-Zugriff erlauben
- timemachine: Time Machine für Mac aktivieren

**Beispiel:**
```
truenas_smb_create name="Media" path="/mnt/tank/media" guest_ok=true
```

## Best Practices

### Snapshot-Strategie
- Wichtige Daten: Stündliche Snapshots, 24h Aufbewahrung
- Dokumente: Tägliche Snapshots, 30 Tage Aufbewahrung
- Medien: Wöchentliche Snapshots, 8 Wochen Aufbewahrung

### Dataset-Organisation
```
tank/
├── documents/      (mit täglichen Snapshots)
├── media/          (groß, seltene Snapshots)
├── backups/        (eingehende Backups)
└── vm-storage/     (dediziert für VMs)
```

### SMB-Freigaben
- Verwende aussagekräftige Namen
- Deaktiviere Guest-Zugriff für sensible Daten
- Aktiviere Time Machine für Mac-Backups

## Fehlerbehebung

### Pool DEGRADED
1. Prüfe Disk-Status: `truenas_health`
2. Identifiziere fehlende/fehlerhafte Disk
3. Ersetze physikalisch und warte auf Resilver

### Snapshot-Bereinigung
Wenn zu viele Snapshots vorhanden:
1. Alte Snapshots identifizieren: `truenas_snapshot_list`
2. Nicht mehr benötigte löschen
3. Retention-Policies überprüfen
```

---

## 11. Implementierungs-Timeline

### Phase 1: Core (Tag 1)
- [ ] Client-Struktur (`internal/truenas/client.go`)
- [ ] Config-Types erweitern
- [ ] Basic API Wrapper (GET/POST/DELETE)
- [ ] Health-Check

### Phase 2: Storage Management (Tag 2)
- [ ] Pool-Verwaltung
- [ ] Dataset-Verwaltung
- [ ] Tools implementieren
- [ ] Registry-Datenbank

### Phase 3: Snapshots & Shares (Tag 3)
- [ ] Snapshot-Management
- [ ] SMB/NFS Shares
- [ ] Scheduler für automatische Snapshots
- [ ] Rollback-Funktionalität

### Phase 4: UI & Integration (Tag 4)
- [ ] Web UI Seite
- [ ] JavaScript-Client
- [ ] Server-Handler
- [ ] Tool Manifest
- [ ] Tool Manual
- [ ] Übersetzungen (15 Sprachen)

---

## 12. Sicherheitsüberlegungen

### API-Key Schutz
- Keys werden NUR im Vault gespeichert
- Nie in Logs oder Config-Files
- Rotations-Reminder implementieren

### Destructive Operations
- Löschen/Rollback erfordert explizite Freigabe
- Bestätigungsdialog in UI
- Logging aller destruktiven Aktionen

### Network Security
- HTTPS standardmäßig
- Option für Self-Signed Zertifikate
- Timeout und Retry-Logik

---

## 13. Testing-Strategie

### Unit Tests
```go
// internal/truenas/client_test.go
func TestClientHealth(t *testing.T) {
    mockServer := httptest.NewServer(http.HandlerFunc(...))
    defer mockServer.Close()
    
    client := NewClient(mockConfig, nil)
    health, err := client.Health(context.Background())
    
    assert.NoError(t, err)
    assert.Equal(t, "TrueNAS-SCALE-22.12", health["version"])
}
```

### Integration Tests
- Mit laufendem TrueNAS (Test-VM)
- Pool-Operationen
- Snapshot Lifecycle
- Share-Erstellung

---

*Dokument erstellt: 2026-03-25*  
*Autor: AuraGo Integration Team*
