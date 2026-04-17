# LDAP / Active Directory (`ldap`)

Query and authenticate against an LDAP or Active Directory server. Search for users and groups, retrieve user and group details, list all users or groups, authenticate credentials, and manage entries when LDAP read-only mode is disabled.

## Operations

| Operation | Description |
|-----------|-------------|
| `search` | Search LDAP directory with custom filter |
| `get_user` | Get details for a specific user |
| `list_users` | List all users in the directory |
| `get_group` | Get details for a specific group |
| `list_groups` | List all groups in the directory |
| `authenticate` | Authenticate a user DN with password |
| `test_connection` | Test connection to the LDAP server |
| `add_user` | Create a user entry from a full DN plus attribute map |
| `update_user` | Replace or delete attributes on a user entry |
| `delete_user` | Delete a user entry by DN |
| `add_group` | Create a group entry from a full DN plus attribute map |
| `update_group` | Replace or delete attributes on a group entry |
| `delete_group` | Delete a group entry by DN |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `base_dn` | string | no | Base DN to search from (defaults to configured base_dn) |
| `filter` | string | for search | LDAP search filter (e.g. `(objectClass=user)`, `(cn=John)`) |
| `username` | string | for get_user | Username to look up |
| `group_name` | string | for get_group | Group name to look up |
| `user_dn` | string | for authenticate | User DN for authentication |
| `dn` | string | for add/update/delete | Full DN of the LDAP entry |
| `password` | string | for authenticate | Password for authentication |
| `attributes` | array | for search | List of LDAP attributes to return (e.g. `["cn", "mail", "memberOf"]`) |
| `entry_attributes` | object | for add_user/add_group | Attribute map for the new entry. Values may be strings or arrays of strings |
| `changes` | object | for update_user/update_group | Attribute map. Non-empty arrays replace values; empty arrays delete the attribute |

## Examples

**Test connection:**
```json
{"action": "ldap", "operation": "test_connection"}
```

**Search for all users:**
```json
{"action": "ldap", "operation": "search", "filter": "(objectClass=user)", "attributes": ["cn", "mail", "sAMAccountName"]}
```

**Get a specific user:**
```json
{"action": "ldap", "operation": "get_user", "username": "john.doe"}
```

**List all groups:**
```json
{"action": "ldap", "operation": "list_groups"}
```

**Get a specific group:**
```json
{"action": "ldap", "operation": "get_group", "group_name": "Domain Users"}
```

**Authenticate a user:**
```json
{"action": "ldap", "operation": "authenticate", "user_dn": "cn=john.doe,ou=users,dc=example,dc=com", "password": "secret123"}
```

**Create a user entry:**
```json
{
  "action": "ldap",
  "operation": "add_user",
  "dn": "cn=jane.doe,ou=users,dc=example,dc=com",
  "entry_attributes": {
    "objectClass": ["top", "person", "organizationalPerson", "inetOrgPerson"],
    "cn": ["jane.doe"],
    "sn": ["Doe"],
    "mail": ["jane.doe@example.com"],
    "uid": ["jane.doe"]
  }
}
```

**Update a group entry:**
```json
{
  "action": "ldap",
  "operation": "update_group",
  "dn": "cn=devops,ou=groups,dc=example,dc=com",
  "changes": {
    "description": ["DevOps team"],
    "member": [
      "cn=jane.doe,ou=users,dc=example,dc=com",
      "cn=john.doe,ou=users,dc=example,dc=com"
    ]
  }
}
```

**Delete a user entry:**
```json
{"action": "ldap", "operation": "delete_user", "dn": "cn=jane.doe,ou=users,dc=example,dc=com"}
```

## Configuration

```yaml
ldap:
  enabled: true
  readonly: true                        # when enabled, only read operations are allowed
  host: "ldap.example.com"             # LDAP server hostname or IP
  port: 636                             # LDAPS port (default: 636); use 389 for plain LDAP
  use_tls: true                         # use LDAPS (recommended)
  insecure_skip_verify: false            # skip TLS certificate verification (for self-signed certs)
  base_dn: "dc=example,dc=com"         # base DN for all searches
  bind_dn: "cn=admin,dc=example,dc=com" # service account DN for binding
  # bind_password: ""                  # stored in vault as ldap_bind_password
  user_search_base: "ou=users,dc=example,dc=com"   # subtree for user searches
  group_search_base: "ou=groups,dc=example,dc=com"  # subtree for group searches
```

## Notes

- **Security**: The bind password is stored securely in the encrypted vault, not in config.yaml
- **LDAPS**: LDAPS on port 636 is recommended. Plain LDAP on port 389 sends credentials unencrypted
- **Read-only mode**: When `readonly: true`, all mutating operations (`add_*`, `update_*`, `delete_*`) are blocked
- **Search filters**: Use standard LDAP filter syntax (RFC 4515). Examples:
  - `(objectClass=user)` - all user objects
  - `(cn=John*)` - users whose Common Name starts with "John"
  - `(&(objectClass=user)(memberOf=cn=Admins,ou=groups,dc=example,dc=com))` - users in a specific group
- **Write operations**: AuraGo does not assume a universal LDAP schema. For create/update operations, provide the full target DN and the exact attributes required by your directory (for example OpenLDAP versus Active Directory).
- **Active Directory**: For AD, common attributes include `sAMAccountName`, `userPrincipalName`, `memberOf`, `distinguishedName`
