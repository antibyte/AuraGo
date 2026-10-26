package agent

import (
	"encoding/hex"
	"encoding/json"
	"net/mail"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aurago/internal/inventory"
)

func immutableDockerContainerID(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func canonicalPolicyOperation(tool, operation string) string {
	if tool == "docker" {
		switch operation {
		case "ps":
			return "list_containers"
		case "inspect_container":
			return "inspect"
		case "rm":
			return "remove"
		case "create_container":
			return "create"
		case "images":
			return "list_images"
		case "pull":
			return "pull_image"
		case "rmi":
			return "remove_image"
		case "networks":
			return "list_networks"
		case "volumes":
			return "list_volumes"
		case "system_info":
			return "info"
		case "copy":
			return "cp"
		}
	}
	if tool == "filesystem" || tool == "filesystem_op" {
		switch operation {
		case "read":
			return "read_file"
		case "write":
			return "write_file"
		case "list", "ls":
			return "list_dir"
		}
	}
	if tool == "ansible" {
		switch operation {
		case "command", "run_module":
			return "adhoc"
		case "run", "run_playbook":
			return "playbook"
		case "health":
			return "status"
		case "playbooks":
			return "list_playbooks"
		case "list_inventory":
			return "inventory"
		case "dry_run":
			return "check"
		case "gather_facts":
			return "facts"
		}
	}
	return operation
}

// Targets are decoded execution identities, never a priority list of arbitrary
// argument keys. Unknown tools retain a server-owned tool identity and cannot
// escape a concrete target denial through an unverified caller-supplied label.
func canonicalActionTargets(tc ToolCall, dc *DispatchContext) ([]string, bool) {
	var targets []string
	unknown := false
	add := func(kind, value string) {
		if value = strings.TrimSpace(value); value != "" {
			targets = append(targets, kind+":"+value)
		}
	}
	file := func(value string) {
		if value = strings.TrimSpace(value); value != "" {
			add("file", actionFilePath(value, dc))
		}
	}
	recipient := func(value string) {
		if address, err := mail.ParseAddress(strings.TrimSpace(value)); err == nil {
			value = address.Address
		}
		if at := strings.LastIndex(value, "@"); at >= 0 {
			value = value[:at+1] + strings.ToLower(value[at+1:])
		}
		add("to", value)
	}
	endpoint := func(value string) {
		u, err := url.Parse(strings.TrimSpace(value))
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
			unknown = true
			return
		}
		// Credentials, fragments and query parameters never enter summaries.
		add("url", strings.ToLower(u.Scheme+"://"+u.Host)+(&url.URL{Path: u.Path}).EscapedPath())
	}
	remote := func(selector string) {
		if selector == "" {
			unknown = true
			return
		}
		if dc != nil && dc.InventoryDB != nil {
			if device, err := inventory.GetDeviceByIDOrName(dc.InventoryDB, selector); err == nil {
				selector = device.ID
			} else {
				unknown = true
			}
		} else {
			unknown = true
		}
		add("server_id", selector)
	}
	switch tc.Action {
	case "qr_code":
		qrRef := func(root, value string) {
			if value == "" {
				return
			}
			if root == "" || root == "workspace" {
				file(value)
			} else if root == "desktop" {
				add("desktop_file", filepath.ToSlash(filepath.Clean(value)))
			} else {
				unknown = true
			}
		}
		qrRef(toolArgString(tc.Params, "output_root"), toolArgString(tc.Params, "output_path"))
		for _, key := range []string{"input_ref", "config_ref", "logo_ref", "frame_image_ref", "csv_ref"} {
			if ref, ok := tc.Params[key].(map[string]interface{}); ok {
				qrRef(toolArgString(ref, "root"), toolArgString(ref, "path"))
			}
		}
		for _, key := range []string{"logo_url", "frame_image_url"} {
			if value := toolArgString(tc.Params, key); value != "" {
				endpoint(value)
			}
		}
		if len(targets) == 0 {
			add("tool", "qr_code")
		}
	case "execute_shell", "execute_python", "execute_sudo", "run_tool", "execute_skill", "run_agent_skill_script":
		add("host", "local")
	case "docker":
		req := decodeDockerArgs(tc)
		if dockerCreateRunOperation(req.Operation) {
			add("container_id", req.Name)
			add("image", req.Image)
			unknown = true // Compose/build/bind effects need an exact hash or broad grant.
		} else if dockerOperationTargetsContainer(req.Operation) && req.targetContainerID() != "" {
			add("container_id", req.targetContainerID())
			unknown = !immutableDockerContainerID(req.targetContainerID())
			if req.Operation == "connect" || req.Operation == "disconnect" {
				add("network", req.Network)
			}
			if req.Operation == "cp" || req.Operation == "copy" {
				unknown = true // Host/container path semantics depend on direction.
			}
		} else {
			unknown = true
		}
	case "s3_storage", "s3":
		req := decodeS3Args(tc)
		provider, bucket := "external", req.Bucket
		if dc != nil && dc.Cfg != nil {
			if dc.Cfg.S3.Provider != "" {
				provider = dc.Cfg.S3.Provider
			}
			if bucket == "" {
				if provider == "rustfs" {
					bucket = dc.Cfg.RustFS.Bucket
				} else {
					bucket = dc.Cfg.S3.Bucket
				}
			}
		}
		add("s3_provider", provider)
		object := func(b, k string) { add("s3_object", provider+"/"+b+"/"+url.PathEscape(k)) }
		switch req.Operation {
		case "list_buckets":
		case "list", "list_objects":
			add("s3_prefix", provider+"/"+bucket+"/"+url.PathEscape(req.Prefix))
		default:
			object(bucket, req.Key)
		}
		if req.Operation == "copy" || req.Operation == "move" {
			dest := req.DestinationBucket
			if strings.TrimSpace(dest) == "" {
				dest = bucket
			}
			object(dest, req.DestinationKey)
		}
		if req.Operation == "upload" || req.Operation == "download" {
			file(req.LocalPath)
		}
		if bucket == "" && req.Operation != "list_buckets" {
			unknown = true
		}
	case "execute_remote_shell":
		remote(decodeRemoteShellArgs(tc).ServerID)
	case "transfer_remote_file":
		req := decodeRemoteFileTransferArgs(tc)
		remote(req.ServerID)
		file(req.LocalPath)
		add("remote_path", req.RemotePath)
	case "register_device":
		req := decodeRegisterDeviceArgs(tc)
		add("hostname", req.Hostname)
		add("ip_address", req.IPAddress)
	case "ansible":
		req := decodeAnsibleArgs(tc)
		add("host_pattern", firstNonEmptyToolString(req.hosts(), "all"))
		add("inventory", req.Inventory)
		// Patterns, inventory groups and playbooks can resolve to additional
		// hosts/effects at execution time. A narrow grant needs the exact hash.
		unknown = true
	case "api_request":
		endpoint(decodeAPIRequestArgs(tc).URL)
	case "filesystem", "filesystem_op":
		req := decodeFilesystemArgs(tc)
		req.Operation = strings.ToLower(strings.TrimSpace(req.Operation))
		if strings.HasSuffix(strings.ToLower(req.Operation), "_batch") {
			for _, item := range req.Items {
				file(stringValueFromMap(item, "file_path", "path"))
				if req.Operation == "copy_batch" || req.Operation == "move_batch" {
					file(stringValueFromMap(item, "destination", "dest"))
				}
			}
		} else {
			file(req.FilePath)
			// The dispatcher accepts destination as the sole path for writes,
			// deletes and other single-path operations; moves/copies use both.
			if req.FilePath == "" || req.Operation == "move" || req.Operation == "rename" || req.Operation == "copy" {
				file(req.Destination)
			}
		}
	case "file_editor":
		file(decodeFileEditorArgs(tc).FilePath)
	case "json_editor":
		file(decodeJSONEditorArgs(tc).FilePath)
	case "yaml_editor":
		file(decodeYAMLEditorArgs(tc).FilePath)
	case "toml_editor":
		file(decodeTOMLEditorArgs(tc).FilePath)
	case "xml_editor":
		file(decodeXMLEditorArgs(tc).FilePath)
	case "sql_query":
		add("connection_name", decodeSQLQueryArgs(tc).ConnectionName)
	case "manage_sql_connections":
		add("connection_name", decodeManageSQLConnectionsArgs(tc).ConnectionName)
	case "send_email":
		req := decodeEmailSendArgs(tc)
		account := req.Account
		if account == "" && dc != nil && dc.Cfg != nil {
			if selected := defaultAgentEmailAccount(dc.Cfg); selected != nil {
				account = selected.ID
			}
		}
		add("account_id", account)
		for _, address := range strings.Split(req.To, ",") {
			recipient(address)
		}
		for _, attachment := range req.Attachments {
			file(attachment)
		}
	case "agentmail", "agentmail_messages", "agentmail_drafts", "agentmail_threads", "agentmail_inboxes":
		inbox := ""
		if dc != nil && dc.Cfg != nil {
			inbox = dc.Cfg.AgentMail.InboxID
		}
		req := decodeAgentMailArgs(tc, inbox)
		switch req.Operation {
		case "test_connection", "list_inboxes", "create_inbox":
			unknown = true
		default:
			add("inbox_id", req.InboxID)
		}
		switch req.Operation {
		case "get_message", "update_message_labels", "delete_message", "reply_message", "reply_all_message", "forward_message", "get_raw_message", "get_attachment":
			add("message_id", req.MessageID)
		case "get_draft", "update_draft", "delete_draft", "send_draft":
			add("draft_id", req.DraftID)
		case "get_thread":
			add("thread_id", req.ThreadID)
		}
		if req.Operation == "get_attachment" {
			add("attachment_id", req.AttachmentID)
		}
		switch req.Operation {
		case "send_message", "forward_message", "create_draft", "update_draft":
			for _, recipients := range [][]string{req.To, req.CC, req.BCC} {
				for _, address := range recipients {
					recipient(address) // A recipient denial applies equally to To/Cc/Bcc.
				}
			}
		}
		switch req.Operation {
		case "send_message", "forward_message", "create_draft", "update_draft", "reply_message", "reply_all_message":
			for _, attachment := range req.Attachments {
				file(attachment.Path)
			}
		}
		// Reply and stored-draft recipients come from the remote message/draft.
		if req.Operation == "reply_message" || req.Operation == "reply_all_message" || req.Operation == "send_draft" {
			unknown = true
		}
	case "google_workspace":
		req := decodeGoogleWorkspaceArgs(tc)
		for _, address := range strings.Split(req.To, ",") {
			recipient(address)
		}
		unknown = true // Other operations have remote object/permission effects.
	case "virustotal", "virustotal_scan":
		req := decodeVirusTotalScanArgs(tc.Params)
		if req.FilePath != "" {
			file(req.FilePath)
		} else if strings.HasPrefix(req.Resource, "http://") || strings.HasPrefix(req.Resource, "https://") {
			endpoint(req.Resource)
		} else {
			add("resource", req.Resource)
		}
	case "paperless_ngx":
		add("document_id", decodePaperlessArgs(tc.Params).DocumentID)
	default:
		unknown = true
	}
	if len(targets) == 0 {
		add("tool", tc.Action)
		unknown = true
	}
	sort.Strings(targets)
	unique := targets[:0]
	for _, target := range targets {
		if len(unique) == 0 || unique[len(unique)-1] != target {
			unique = append(unique, target)
		}
	}
	return unique, unknown
}

func actionTargetSummary(targets []string) string {
	if len(targets) == 1 {
		return targets[0]
	}
	encoded, _ := json.Marshal(targets)
	return "targets:" + string(encoded)
}

func actionTargetComponents(target string) []string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "targets:") {
		var components []string
		if json.Unmarshal([]byte(strings.TrimPrefix(target, "targets:")), &components) == nil && len(components) > 0 {
			return components
		}
	}
	return []string{target}
}

func actionFilePath(value string, dc *DispatchContext) string {
	if !filepath.IsAbs(value) && dc != nil && dc.Cfg != nil {
		workspace := dc.Cfg.Directories.WorkspaceDir
		// Match the filesystem decoder's project-relative workspace aliases.
		for current := filepath.Clean(workspace); current != "."; current = filepath.Dir(current) {
			if strings.EqualFold(filepath.Base(current), "agent_workspace") {
				if relative, err := filepath.Rel(current, workspace); err == nil {
					prefix := filepath.ToSlash(relative)
					clean := filepath.ToSlash(filepath.Clean(value))
					if strings.HasPrefix(clean, prefix+"/") {
						value = filepath.FromSlash(strings.TrimPrefix(clean, prefix+"/"))
					}
				}
				break
			}
			if filepath.Dir(current) == current {
				break
			}
		}
		value = filepath.Join(workspace, value)
	}
	if absolute, err := filepath.Abs(value); err == nil {
		value = absolute
	}
	// Resolve existing parents too, so a new write through a symlink retains
	// the same target as a direct path. Execution still enforces its own jail.
	var tail []string
	for candidate := filepath.Clean(value); ; candidate = filepath.Dir(candidate) {
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			value = resolved
			break
		}
		if filepath.Dir(candidate) == candidate {
			break
		}
		tail = append(tail, filepath.Base(candidate))
	}
	value = filepath.Clean(value)
	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
	}
	return value
}
