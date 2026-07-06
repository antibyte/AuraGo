# TrueNAS Integration

The TrueNAS integration allows the agent to manage ZFS storage pools, datasets, snapshots, and shares (SMB/NFS) on TrueNAS SCALE or CORE systems.

## Overview

TrueNAS is an open-source storage operating system based on FreeBSD (CORE) or Linux (SCALE). This integration uses the TrueNAS API v2.0 to:

- Monitor pool health and capacity
- List and manage ZFS datasets
- Create and manage snapshots
- Configure SMB and NFS shares
- Check system health

## Configuration

Required settings in `config.yaml`:

```yaml
truenas:
  enabled: true
  host: "truenas.local"        # Hostname or IP
  port: 443                    # API port (default: 443)
  use_https: true              # Use HTTPS (recommended)
  insecure_ssl: false          # Skip certificate validation
  readonly: false              # Only read operations
  allow_destructive: false     # Allow delete/rollback
```

The API key must be stored in the vault:
```
Key: truenas_api_key
```

To create an API key in TrueNAS:
1. Go to **System → API Keys**
2. Click **Add**
3. Give it a name (e.g., "AuraGo")
4. Copy the generated key
5. Save it to the AuraGo vault via Web UI

## Security Model

The integration has three security levels:

1. **enabled** - Integration is active
2. **readonly** - Only list/read operations allowed
3. **allow_destructive** - Allows delete, rollback, and other destructive operations

When `readonly: true`, write operations will fail with an error.
When `allow_destructive: false`, dataset/share delete, snapshot rollback, snapshot delete, and pool scrub operations are blocked.

## Native Tool Parameter Mapping

The `truenas` native tool uses a compact shared schema:

- `action`: One of the `truenas_*` operations below.
- `name`: Dataset, snapshot, or SMB share name for create/delete/rollback operations.
- `path`: Local dataset/share path for SMB and NFS share creation.
- `query`: Pool/dataset filter. For `truenas_nfs_create`, comma-separated allowed networks.
- `port`: Numeric pool ID for `truenas_pool_scrub`, or SMB/NFS share ID for delete.
- `limit`: Quota in GB for dataset creation, or retention days for snapshot creation.
- `content`: Dataset compression for `truenas_dataset_create`; for `truenas_nfs_create`, comma-separated allowed hosts.
- `recursive`: Recursive dataset delete or snapshot creation.
- `force`: Force snapshot rollback.

## Agent Tools

### truenas_health
Check TrueNAS system health and connection status.

**Example:**
```
Check my TrueNAS system health
```

### truenas_pool_list
List all ZFS storage pools with capacity and health status.

**Example:**
```
Show me all storage pools on TrueNAS
```

### truenas_pool_scrub
Start a scrub operation on a pool to check for errors.

**Parameters:**
- `port` (required): Numeric pool ID to scrub

**Example:**
```
Start a scrub on the tank pool
```

### truenas_dataset_list
List all datasets (including nested) with their properties.

**Parameters:**
- `query` (optional): Pool name filter

**Example:**
```
List all datasets in the tank pool
```

### truenas_dataset_create
Create a new ZFS dataset.

**Parameters:**
- `name` (required): Dataset path (e.g., "tank/media")
- `content` (optional): Compression type (lz4, zstd, gzip, off)
- `limit` (optional): Quota in GB

**Example:**
```
Create a new dataset called tank/backups with lz4 compression
```

### truenas_snapshot_create
Create a ZFS snapshot of a dataset.

**Parameters:**
- `query` (required): Dataset path
- `name` (optional): Snapshot name (auto-generated if not provided)
- `recursive` (optional): Include child datasets
- `limit` (optional): Retention days

**Example:**
```
Create a snapshot of tank/media
```

### truenas_snapshot_list
List snapshots for a dataset.

**Parameters:**
- `query` (optional): Dataset path filter

**Example:**
```
Show all snapshots for tank/media
```

### truenas_snapshot_delete
Delete a snapshot.

**Parameters:**
- `name` (required): Full snapshot name (e.g., "tank/media@auto-20240101")

**Example:**
```
Delete the snapshot tank/media@old-backup
```

**Note:** Requires `allow_destructive: true`

### truenas_snapshot_rollback
Rollback a dataset to a snapshot state.

**Parameters:**
- `name` (required): Full snapshot name
- `force` (optional): Force rollback even if changes exist

**Example:**
```
Rollback tank/media to the snapshot from yesterday
```

**Warning:** This destroys all changes made since the snapshot!

### truenas_smb_create
Create an SMB share for a dataset.

**Parameters:**
- `name` (required): Share name
- `path` (required): Dataset path
- `comment` (optional): Share description
- `guest_ok` (optional): Allow guest access

**Example:**
```
Create an SMB share called Media for tank/media
```

### truenas_smb_list
List SMB shares.

### truenas_smb_delete
Delete an SMB share.

**Parameters:**
- `port` (required): Numeric SMB share ID

**Note:** Requires `allow_destructive: true`

### truenas_nfs_list
List NFS shares.

### truenas_nfs_create
Create an NFS share for a dataset.

**Parameters:**
- `path` (required): Dataset path
- `query` (optional): Comma-separated allowed networks
- `content` (optional): Comma-separated allowed hosts

**Example:**
```
Create an NFS share for tank/backups accessible from 192.168.1.0/24
```

### truenas_nfs_delete
Delete an NFS share.

**Parameters:**
- `port` (required): Numeric NFS share ID

**Note:** Requires `allow_destructive: true`

### truenas_fs_space
Check free space on pools or datasets.

**Parameters:**
- `query` (optional): Specific pool or dataset

**Example:**
```
How much free space is left on the tank pool?
```

## Common Workflows

### Backup Strategy
1. Create a dataset for backups: `truenas_dataset_create`
2. Set up periodic snapshots: `truenas_snapshot_create` (can be automated via cron)
3. Create SMB/NFS share for access: `truenas_smb_create` or `truenas_nfs_create`

### Media Storage Setup
1. Create datasets for different media types:
   - `tank/media/movies`
   - `tank/media/music`
   - `tank/media/photos`
2. Create SMB share for media access
3. Set appropriate quotas per dataset

### Monitoring
- Regular health checks: `truenas_health`
- Pool scrubbing: `truenas_pool_scrub` (should run monthly)
- Space monitoring: `truenas_fs_space`

## Troubleshooting

### Connection Failed
- Verify TrueNAS is running and accessible
- Check API key is correct and saved in vault
- Ensure `use_https` matches your TrueNAS configuration
- Try `insecure_ssl: true` for self-signed certificates

### Permission Denied
- Ensure API key has sufficient permissions in TrueNAS
- Check `readonly` and `allow_destructive` settings
- Some operations require administrative privileges

### Operations Blocked
- Verify `readonly: false` for write operations
- Verify `allow_destructive: true` for delete/rollback
- Check agent tool permissions in config

## API Reference

The integration uses TrueNAS API v2.0 endpoints:
- `/api/v2.0/pool` - Pool operations
- `/api/v2.0/pool/dataset` - Dataset operations  
- `/api/v2.0/pool/snapshot` - Snapshot operations
- `/api/v2.0/sharing/smb` - SMB share operations
- `/api/v2.0/sharing/nfs` - NFS share operations
- `/api/v2.0/system/info` - System information

For full API documentation, see your TrueNAS Web UI → API Docs.
