package toolmeta

type CompressionClass string

const (
	CompressionClassNone   CompressionClass = ""
	CompressionClassShell  CompressionClass = "shell"
	CompressionClassPython CompressionClass = "python"
	CompressionClassAPI    CompressionClass = "api"
)

type Metadata struct {
	Name             string
	Family           string
	CompressionClass CompressionClass
}

var registry = map[string]Metadata{
	"execute_shell":        {Name: "execute_shell", Family: "execution", CompressionClass: CompressionClassShell},
	"execute_sudo":         {Name: "execute_sudo", Family: "execution", CompressionClass: CompressionClassShell},
	"execute_remote_shell": {Name: "execute_remote_shell", Family: "remote", CompressionClass: CompressionClassShell},
	"remote_execution":     {Name: "remote_execution", Family: "remote", CompressionClass: CompressionClassShell},
	"ssh_exec":             {Name: "ssh_exec", Family: "remote", CompressionClass: CompressionClassShell},
	"service_manager":      {Name: "service_manager", Family: "system", CompressionClass: CompressionClassShell},
	"execute_python":       {Name: "execute_python", Family: "execution", CompressionClass: CompressionClassPython},
	"execute_sandbox":      {Name: "execute_sandbox", Family: "execution", CompressionClass: CompressionClassPython},

	"docker":               {Name: "docker", Family: "containers", CompressionClass: CompressionClassAPI},
	"docker_compose":       {Name: "docker_compose", Family: "containers", CompressionClass: CompressionClassAPI},
	"proxmox":              {Name: "proxmox", Family: "infra", CompressionClass: CompressionClassAPI},
	"homeassistant":        {Name: "homeassistant", Family: "home", CompressionClass: CompressionClassAPI},
	"home_assistant":       {Name: "home_assistant", Family: "home", CompressionClass: CompressionClassAPI},
	"kubernetes":           {Name: "kubernetes", Family: "infra", CompressionClass: CompressionClassAPI},
	"api_request":          {Name: "api_request", Family: "network", CompressionClass: CompressionClassAPI},
	"github":               {Name: "github", Family: "dev", CompressionClass: CompressionClassAPI},
	"sql_query":            {Name: "sql_query", Family: "data", CompressionClass: CompressionClassAPI},
	"koofr":                {Name: "koofr", Family: "storage", CompressionClass: CompressionClassAPI},
	"koofr_api":            {Name: "koofr_api", Family: "storage", CompressionClass: CompressionClassAPI},
	"koofr_op":             {Name: "koofr_op", Family: "storage", CompressionClass: CompressionClassAPI},
	"filesystem":           {Name: "filesystem", Family: "files", CompressionClass: CompressionClassAPI},
	"filesystem_op":        {Name: "filesystem_op", Family: "files", CompressionClass: CompressionClassAPI},
	"file_reader_advanced": {Name: "file_reader_advanced", Family: "files", CompressionClass: CompressionClassAPI},
	"smart_file_read":      {Name: "smart_file_read", Family: "files", CompressionClass: CompressionClassAPI},
	"list_processes":       {Name: "list_processes", Family: "system", CompressionClass: CompressionClassAPI},
	"read_process_logs":    {Name: "read_process_logs", Family: "system", CompressionClass: CompressionClassAPI},
	"manage_daemon":        {Name: "manage_daemon", Family: "system", CompressionClass: CompressionClassAPI},
	"manage_plan":          {Name: "manage_plan", Family: "planning", CompressionClass: CompressionClassAPI},
	"homepage":             {Name: "homepage", Family: "homepage", CompressionClass: CompressionClassAPI},
	"netlify":              {Name: "netlify", Family: "deploy", CompressionClass: CompressionClassAPI},

	"homepage_project": {Name: "homepage_project", Family: "homepage", CompressionClass: CompressionClassAPI},
	"homepage_file":    {Name: "homepage_file", Family: "homepage", CompressionClass: CompressionClassAPI},
	"homepage_quality": {Name: "homepage_quality", Family: "homepage", CompressionClass: CompressionClassAPI},
	"homepage_deploy":  {Name: "homepage_deploy", Family: "homepage", CompressionClass: CompressionClassAPI},
	"homepage_git":     {Name: "homepage_git", Family: "homepage", CompressionClass: CompressionClassAPI},

	"virtual_desktop":         {Name: "virtual_desktop", Family: "desktop", CompressionClass: CompressionClassAPI},
	"virtual_desktop_files":   {Name: "virtual_desktop_files", Family: "desktop", CompressionClass: CompressionClassAPI},
	"virtual_desktop_apps":    {Name: "virtual_desktop_apps", Family: "desktop", CompressionClass: CompressionClassAPI},
	"virtual_desktop_widgets": {Name: "virtual_desktop_widgets", Family: "desktop", CompressionClass: CompressionClassAPI},

	"remote_control":         {Name: "remote_control", Family: "remote", CompressionClass: CompressionClassAPI},
	"remote_control_devices": {Name: "remote_control_devices", Family: "remote", CompressionClass: CompressionClassAPI},
	"remote_control_shell":   {Name: "remote_control_shell", Family: "remote", CompressionClass: CompressionClassShell},
	"remote_control_files":   {Name: "remote_control_files", Family: "remote", CompressionClass: CompressionClassAPI},
	"remote_control_desktop": {Name: "remote_control_desktop", Family: "remote", CompressionClass: CompressionClassAPI},

	"agentmail":          {Name: "agentmail", Family: "mail", CompressionClass: CompressionClassAPI},
	"agentmail_inboxes":  {Name: "agentmail_inboxes", Family: "mail", CompressionClass: CompressionClassAPI},
	"agentmail_messages": {Name: "agentmail_messages", Family: "mail", CompressionClass: CompressionClassAPI},
	"agentmail_threads":  {Name: "agentmail_threads", Family: "mail", CompressionClass: CompressionClassAPI},
	"agentmail_drafts":   {Name: "agentmail_drafts", Family: "mail", CompressionClass: CompressionClassAPI},

	"invasion_control":   {Name: "invasion_control", Family: "invasion", CompressionClass: CompressionClassAPI},
	"invasion_nests":     {Name: "invasion_nests", Family: "invasion", CompressionClass: CompressionClassAPI},
	"invasion_tasks":     {Name: "invasion_tasks", Family: "invasion", CompressionClass: CompressionClassAPI},
	"invasion_artifacts": {Name: "invasion_artifacts", Family: "invasion", CompressionClass: CompressionClassAPI},
}

func Lookup(name string) (Metadata, bool) {
	meta, ok := registry[name]
	return meta, ok
}

func CompressionClassForTool(name string) CompressionClass {
	if meta, ok := Lookup(name); ok {
		return meta.CompressionClass
	}
	return CompressionClassNone
}
