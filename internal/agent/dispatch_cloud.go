package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"aurago/internal/tools"
)

var _ = (*slog.Logger)(nil)

func hereNowToolOutput(raw json.RawMessage, err error) string {
	if err == nil {
		return "Tool Output: " + string(raw)
	}
	encoded, _ := json.Marshal(tools.HereNowErrorPayload(err))
	return "Tool Output: " + string(encoded)
}

func hereNowSlugRequired(operation, slug string) string {
	if strings.TrimSpace(slug) != "" {
		return ""
	}
	return fmt.Sprintf(`Tool Output: {"status":"error","message":"slug is required for here.now %s"}`, operation)
}

func dispatchCloud(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	vault := dc.Vault
	handled := true

	result := func() string {
		switch tc.Action {
		case "github":
			if !cfg.GitHub.Enabled {
				return `Tool Output: {"status":"error","message":"GitHub integration is not enabled. Set github.enabled=true in config.yaml."}`
			}
			req := decodeGitHubArgs(tc)
			if cfg.GitHub.ReadOnly {
				switch req.Operation {
				case "create_repo", "delete_repo", "create_issue", "close_issue", "create_or_update_file", "track_project", "untrack_project":
					return `Tool Output: {"status":"error","message":"GitHub is in read-only mode. Disable github.read_only to allow changes."}`
				}
			}
			token, err := vault.ReadSecret("github_token")
			if err != nil || token == "" {
				return `Tool Output: {"status":"error","message":"GitHub token not found in vault. Store it with key 'github_token' via the vault API."}`
			}

			ghCfg := tools.GitHubConfig{
				Token:          token,
				Owner:          cfg.GitHub.Owner,
				BaseURL:        cfg.GitHub.BaseURL,
				DefaultPrivate: cfg.GitHub.DefaultPrivate,
				ReadOnly:       cfg.GitHub.ReadOnly,
				AllowedRepos:   cfg.GitHub.AllowedRepos,
				TrustedRepos:   tools.GitHubTrustedProjectRepos(cfg.Directories.WorkspaceDir),
				WorkspaceDir:   cfg.Directories.WorkspaceDir,
			}
			owner := req.Owner
			if owner == "" {
				owner = cfg.GitHub.Owner
			}
			repo := req.Repo
			switch req.Operation {
			case "list_repos":
				logger.Info("LLM requested GitHub list repos", "owner", owner)
				return "Tool Output: " + tools.GitHubListRepos(ghCfg, owner)
			case "create_repo":
				logger.Info("LLM requested GitHub create repo", "name", repo, "desc", req.Description)
				return "Tool Output: " + tools.GitHubCreateRepo(ghCfg, repo, req.Description, nil)
			case "delete_repo":
				logger.Info("LLM requested GitHub delete repo", "owner", owner, "repo", repo)
				return "Tool Output: " + tools.GitHubDeleteRepo(ghCfg, owner, repo)
			case "get_repo":
				logger.Info("LLM requested GitHub get repo", "owner", owner, "repo", repo)
				return "Tool Output: " + tools.GitHubGetRepo(ghCfg, owner, repo)
			case "list_issues":
				state := req.Value
				if state == "" {
					state = "open"
				}
				logger.Info("LLM requested GitHub list issues", "repo", repo, "state", state)
				return "Tool Output: " + tools.GitHubListIssues(ghCfg, owner, repo, state)
			case "create_issue":
				logger.Info("LLM requested GitHub create issue", "repo", repo, "title", req.Title)
				return "Tool Output: " + tools.GitHubCreateIssue(ghCfg, owner, repo, req.Title, req.Body, req.labels())
			case "close_issue":
				issueNum := req.issueNumber()
				logger.Info("LLM requested GitHub close issue", "repo", repo, "number", issueNum)
				return "Tool Output: " + tools.GitHubCloseIssue(ghCfg, owner, repo, issueNum)
			case "list_pull_requests":
				state := req.Value
				if state == "" {
					state = "open"
				}
				logger.Info("LLM requested GitHub list PRs", "repo", repo, "state", state)
				return "Tool Output: " + tools.GitHubListPullRequests(ghCfg, owner, repo, state)
			case "list_branches":
				logger.Info("LLM requested GitHub list branches", "repo", repo)
				return "Tool Output: " + tools.GitHubListBranches(ghCfg, owner, repo)
			case "get_file":
				branch := req.Query
				logger.Info("LLM requested GitHub get file", "repo", repo, "path", req.Path, "branch", branch)
				return "Tool Output: " + tools.GitHubGetFileContent(ghCfg, owner, repo, req.Path, branch)
			case "create_or_update_file":
				logger.Info("LLM requested GitHub create/update file", "repo", repo, "path", req.Path)
				return "Tool Output: " + tools.GitHubCreateOrUpdateFile(ghCfg, owner, repo, req.Path, req.Content, req.Body, req.Value, req.Query)
			case "list_commits":
				branch := req.Query
				limit := req.Limit
				if limit <= 0 {
					limit = 20
				}
				logger.Info("LLM requested GitHub list commits", "repo", repo, "branch", branch)
				return "Tool Output: " + tools.GitHubListCommits(ghCfg, owner, repo, branch, limit)
			case "list_workflow_runs":
				limit := req.Limit
				if limit <= 0 {
					limit = 10
				}
				logger.Info("LLM requested GitHub list workflow runs", "repo", repo)
				return "Tool Output: " + tools.GitHubListWorkflowRuns(ghCfg, owner, repo, limit)
			case "search_repos":
				limit := req.Limit
				if limit <= 0 {
					limit = 10
				}
				logger.Info("LLM requested GitHub search repos", "query", req.Query)
				return "Tool Output: " + tools.GitHubSearchRepos(ghCfg, req.Query, limit)
			case "list_projects":
				logger.Info("LLM requested GitHub list tracked projects")
				return "Tool Output: " + tools.GitHubListProjects(cfg.Directories.WorkspaceDir)
			case "track_project":
				purpose := req.Content
				if purpose == "" {
					purpose = req.Description
				}
				logger.Info("LLM requested GitHub track project", "name", repo, "purpose", purpose)
				return "Tool Output: " + tools.GitHubTrackProject(cfg.Directories.WorkspaceDir, repo, purpose, "", "", owner, cfg.GitHub.DefaultPrivate)
			case "untrack_project":
				logger.Info("LLM requested GitHub untrack project", "name", repo)
				return "Tool Output: " + tools.GitHubUntrackProject(cfg.Directories.WorkspaceDir, repo)
			default:
				return `Tool Output: {"status":"error","message":"Unknown github operation. Use: list_repos, create_repo, delete_repo, get_repo, list_issues, create_issue, close_issue, list_pull_requests, list_branches, get_file, create_or_update_file, list_commits, list_workflow_runs, search_repos, list_projects, track_project, untrack_project"}`
			}

		case "huggingface":
			req := decodeHuggingFaceArgs(tc)
			token := tools.ResolveHuggingFaceToken(cfg.HuggingFace)
			if token == "" && vault != nil {
				if stored, err := vault.ReadSecret("huggingface_token"); err == nil {
					token = stored
				}
			}
			logger.Info("LLM requested Hugging Face operation", "operation", req.Operation)
			return "Tool Output: " + tools.RunHuggingFace(ctx, cfg.HuggingFace, token, cfg.Directories.WorkspaceDir, cfg.Directories.DataDir, req)

		case "here_now_sites", "here_now_site":
			if !cfg.HereNow.Enabled {
				return `Tool Output: {"status":"error","message":"here.now integration is not enabled. Set here_now.enabled=true in config.yaml."}`
			}
			req := decodeHereNowArgs(tc)
			if tc.Action == "here_now_site" {
				if cfg.HereNow.ReadOnly {
					return `Tool Output: {"status":"error","message":"here.now is in read-only mode. Disable here_now.readonly to allow changes."}`
				}
				switch req.Operation {
				case "publish", "update", "duplicate", "restore_version":
					if !cfg.HereNow.AllowPublish {
						return `Tool Output: {"status":"error","message":"here.now publishing is disabled. Enable here_now.allow_publish."}`
					}
				case "update_metadata":
					if !cfg.HereNow.AllowSiteManagement {
						return `Tool Output: {"status":"error","message":"here.now Site management is disabled. Enable here_now.allow_site_management."}`
					}
				case "update_access", "set_password", "remove_password":
					if !cfg.HereNow.AllowAccessManagement {
						return `Tool Output: {"status":"error","message":"here.now access management is disabled. Enable here_now.allow_access_management."}`
					}
				case "delete_site", "delete_version":
					if !cfg.HereNow.AllowDelete {
						return `Tool Output: {"status":"error","message":"here.now deletion is disabled. Enable here_now.allow_delete."}`
					}
					if !req.Confirm {
						return `Tool Output: {"status":"error","message":"Permanent here.now deletion requires confirm=true and an exact slug/version_id."}`
					}
				default:
					return `Tool Output: {"status":"error","message":"Unknown here_now_site operation"}`
				}
				switch req.Operation {
				case "publish":
					if strings.TrimSpace(req.ProjectDir) == "" {
						return `Tool Output: {"status":"error","message":"project_dir is required for here.now publishing"}`
					}
				case "update":
					if strings.TrimSpace(req.ProjectDir) == "" {
						return `Tool Output: {"status":"error","message":"project_dir is required for here.now publishing"}`
					}
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
				case "restore_version", "delete_version":
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
					if strings.TrimSpace(req.VersionID) == "" {
						return fmt.Sprintf(`Tool Output: {"status":"error","message":"version_id is required for %s"}`, req.Operation)
					}
				default:
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
				}
			}
			if vault == nil {
				return `Tool Output: {"status":"error","message":"Vault is unavailable"}`
			}
			apiKey, keyErr := vault.ReadSecret("here_now_api_key")
			if keyErr != nil || strings.TrimSpace(apiKey) == "" {
				return `Tool Output: {"status":"error","message":"here.now API key not found in Vault. Store it with key 'here_now_api_key' via the Config UI."}`
			}
			hnCfg := tools.HereNowConfig{
				APIKey: apiKey, DefaultAccount: cfg.HereNow.DefaultAccount,
				ReadOnly: cfg.HereNow.ReadOnly, AllowPublish: cfg.HereNow.AllowPublish,
				AllowSiteManagement:   cfg.HereNow.AllowSiteManagement,
				AllowAccessManagement: cfg.HereNow.AllowAccessManagement,
				AllowDelete:           cfg.HereNow.AllowDelete,
			}
			client, clientErr := tools.NewHereNowClient(apiKey, cfg.HereNow.DefaultAccount)
			if clientErr != nil {
				return hereNowToolOutput(nil, clientErr)
			}
			if tc.Action == "here_now_sites" {
				switch req.Operation {
				case "list_accounts":
					raw, err := client.ListAccounts(ctx)
					return hereNowToolOutput(raw, err)
				case "list_sites":
					raw, err := client.ListSites(ctx, req.Account, req.Cursor, req.Limit, req.All)
					return hereNowToolOutput(raw, err)
				case "search_sites":
					if strings.TrimSpace(req.Query) == "" {
						return `Tool Output: {"status":"error","message":"query is required for here.now search_sites"}`
					}
					raw, err := client.SearchSites(ctx, req.Account, req.Query, req.Cursor, req.Limit)
					return hereNowToolOutput(raw, err)
				case "get_site":
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
					raw, err := client.GetSite(ctx, req.Account, req.Slug)
					return hereNowToolOutput(raw, err)
				case "get_access":
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
					raw, err := client.GetAccess(ctx, req.Account, req.Slug)
					return hereNowToolOutput(raw, err)
				case "list_versions":
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
					raw, err := client.ListVersions(ctx, req.Account, req.Slug)
					return hereNowToolOutput(raw, err)
				default:
					return `Tool Output: {"status":"error","message":"Unknown here_now_sites operation. Use: list_accounts, list_sites, search_sites, get_site, get_access, list_versions"}`
				}
			}

			switch req.Operation {
			case "publish", "update":
				if strings.TrimSpace(req.ProjectDir) == "" {
					return `Tool Output: {"status":"error","message":"project_dir is required for here.now publishing"}`
				}
				if req.Operation == "update" {
					if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
						return msg
					}
				}
				homepageCfg := tools.HomepageConfig{
					DockerHost: cfg.Docker.Host, WorkspacePath: cfg.Homepage.WorkspacePath,
					AgentWorkspaceDir: cfg.Directories.WorkspaceDir, DataDir: cfg.Directories.DataDir,
					WebServerPort: cfg.Homepage.WebServerPort, WebServerDomain: cfg.Homepage.WebServerDomain,
					WebServerInternalOnly: cfg.Homepage.WebServerInternalOnly, AllowLocalServer: cfg.Homepage.AllowLocalServer,
				}
				if homepageCfg.WorkspacePath == "" {
					homepageCfg.WorkspacePath = filepath.Join(cfg.Directories.DataDir, "homepage")
				}
				opts := tools.HereNowPublishOptions{
					Account: req.Account, WorkspaceLabel: req.WorkspaceLabel,
					DisplayName: req.DisplayName, DisplayDescription: req.DisplayDescription,
					ViewerTitle: req.ViewerTitle, ViewerDescription: req.ViewerDescription,
					OGImagePath: req.OGImagePath, SPAMode: req.SPAMode,
				}
				if req.Operation == "update" {
					opts.Slug = req.Slug
				}
				logger.Info("LLM requested here.now publish", "operation", req.Operation, "project_dir", req.ProjectDir, "slug", req.Slug, "account", req.Account)
				result := tools.HomepageDeployHereNow(ctx, homepageCfg, hnCfg, req.ProjectDir, req.BuildDir, opts, logger)
				result = homepageRecordDeploymentStrictResult(homepageCfg, dc.HomepageRegistryDB, req.ProjectDir, "here_now", req.BuildDir, result, logger)
				return "Tool Output: " + result
			case "duplicate":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				viewer := map[string]interface{}{}
				if req.ViewerTitle != "" {
					viewer["title"] = req.ViewerTitle
				}
				if req.ViewerDescription != "" {
					viewer["description"] = req.ViewerDescription
				}
				if req.OGImagePath != "" {
					viewer["ogImagePath"] = req.OGImagePath
				}
				raw, err := client.DuplicateSite(ctx, req.Account, req.Slug, viewer)
				return hereNowToolOutput(raw, err)
			case "update_metadata":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				patch := map[string]interface{}{}
				if req.DisplayName != "" {
					patch["displayName"] = req.DisplayName
				}
				if req.DisplayDescription != "" {
					patch["displayDescription"] = req.DisplayDescription
				}
				if req.SPAMode != nil {
					patch["spaMode"] = *req.SPAMode
				}
				viewer := map[string]interface{}{}
				if req.ViewerTitle != "" {
					viewer["title"] = req.ViewerTitle
				}
				if req.ViewerDescription != "" {
					viewer["description"] = req.ViewerDescription
				}
				if req.OGImagePath != "" {
					viewer["ogImagePath"] = req.OGImagePath
				}
				if len(viewer) > 0 {
					patch["viewer"] = viewer
				}
				if len(patch) == 0 {
					return `Tool Output: {"status":"error","message":"No here.now metadata changes supplied"}`
				}
				raw, err := client.PatchMetadata(ctx, req.Account, req.Slug, patch)
				return hereNowToolOutput(raw, err)
			case "update_access":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				if req.Mode == "" {
					return `Tool Output: {"status":"error","message":"mode is required for update_access"}`
				}
				var allowedEmails, allowedDomains *[]string
				if req.AllowedEmailsSet {
					allowedEmails = &req.AllowedEmails
				}
				if req.AllowedDomainsSet {
					allowedDomains = &req.AllowedDomains
				}
				raw, err := client.UpdateAccess(ctx, req.Account, req.Slug, req.Mode, allowedEmails, allowedDomains)
				return hereNowToolOutput(raw, err)
			case "set_password":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				resolved, err := client.ResolveAccount(ctx, req.Account)
				if err != nil {
					return hereNowToolOutput(nil, err)
				}
				passwordKey := tools.HereNowSitePasswordVaultKey(resolved.AccountID, req.Slug)
				password, err := vault.ReadSecret(passwordKey)
				if err != nil || password == "" {
					encoded, _ := json.Marshal(map[string]interface{}{"status": "secret_required", "vault_key": passwordKey, "message": "Use request_vault_secret with this exact vault_key, then retry set_password."})
					return "Tool Output: " + string(encoded)
				}
				raw, err := client.SetPassword(ctx, req.Account, req.Slug, password)
				return hereNowToolOutput(raw, err)
			case "remove_password":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				resolved, err := client.ResolveAccount(ctx, req.Account)
				if err != nil {
					return hereNowToolOutput(nil, err)
				}
				raw, err := client.RemovePassword(ctx, req.Account, req.Slug)
				if err != nil {
					return hereNowToolOutput(nil, err)
				}
				_ = vault.DeleteSecret(tools.HereNowSitePasswordVaultKey(resolved.AccountID, req.Slug))
				return hereNowToolOutput(raw, err)
			case "restore_version":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				if strings.TrimSpace(req.VersionID) == "" {
					return `Tool Output: {"status":"error","message":"version_id is required for restore_version"}`
				}
				raw, err := client.RestoreVersion(ctx, req.Account, req.Slug, req.VersionID)
				return hereNowToolOutput(raw, err)
			case "delete_site":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				resolved, err := client.ResolveAccount(ctx, req.Account)
				if err != nil {
					return hereNowToolOutput(nil, err)
				}
				err = client.DeleteSite(ctx, req.Account, req.Slug)
				if err == nil {
					_ = vault.DeleteSecret(tools.HereNowSitePasswordVaultKey(resolved.AccountID, req.Slug))
				}
				if err != nil {
					return hereNowToolOutput(nil, err)
				}
				return `Tool Output: {"status":"ok","deleted":true}`
			case "delete_version":
				if msg := hereNowSlugRequired(req.Operation, req.Slug); msg != "" {
					return msg
				}
				if strings.TrimSpace(req.VersionID) == "" {
					return `Tool Output: {"status":"error","message":"version_id is required for delete_version"}`
				}
				if err := client.DeleteVersion(ctx, req.Account, req.Slug, req.VersionID); err != nil {
					return hereNowToolOutput(nil, err)
				}
				return `Tool Output: {"status":"ok","deleted":true}`
			}
			return `Tool Output: {"status":"error","message":"Unknown here_now_site operation"}`

		case "netlify":
			if !cfg.Netlify.Enabled {
				return `Tool Output: {"status":"error","message":"Netlify integration is not enabled. Set netlify.enabled=true in config.yaml."}`
			}
			req := decodeNetlifyArgs(tc)
			// Read-only mode: block all mutating operations
			if cfg.Netlify.ReadOnly {
				switch req.Operation {
				case "create_site", "update_site", "delete_site",
					"rollback", "cancel_deploy",
					"set_env", "delete_env",
					"create_hook", "delete_hook",
					"provision_ssl":
					return `Tool Output: {"status":"error","message":"Netlify is in read-only mode. Disable netlify.readonly to allow changes."}`
				}
			}
			// Granular permission checks
			if !cfg.Netlify.AllowDeploy {
				switch req.Operation {
				case "rollback", "cancel_deploy":
					return `Tool Output: {"status":"error","message":"Netlify deploy is not allowed. Set netlify.allow_deploy=true in config.yaml."}`
				}
			}
			if !cfg.Netlify.AllowSiteManagement {
				switch req.Operation {
				case "create_site", "update_site", "delete_site", "create_hook", "delete_hook", "provision_ssl":
					return `Tool Output: {"status":"error","message":"Netlify site management is not allowed. Set netlify.allow_site_management=true in config.yaml."}`
				}
			}
			if !cfg.Netlify.AllowEnvManagement {
				switch req.Operation {
				case "set_env", "delete_env":
					return `Tool Output: {"status":"error","message":"Netlify env var management is not allowed. Set netlify.allow_env_management=true in config.yaml."}`
				}
			}
			token, tokenErr := vault.ReadSecret("netlify_token")
			if tokenErr != nil || token == "" {
				return `Tool Output: {"status":"error","message":"Netlify token not found in vault. Store it with key 'netlify_token' via the vault API."}`
			}
			nfCfg := tools.NetlifyConfig{
				Token:               token,
				DefaultSiteID:       cfg.Netlify.DefaultSiteID,
				TeamSlug:            cfg.Netlify.TeamSlug,
				ReadOnly:            cfg.Netlify.ReadOnly,
				AllowDeploy:         cfg.Netlify.AllowDeploy,
				AllowSiteManagement: cfg.Netlify.AllowSiteManagement,
				AllowEnvManagement:  cfg.Netlify.AllowEnvManagement,
			}
			switch req.Operation {
			// ── Sites ──
			case "list_sites":
				logger.Info("LLM requested Netlify list sites")
				return "Tool Output: " + tools.NetlifyListSites(nfCfg)
			case "get_site":
				logger.Info("LLM requested Netlify get site", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyGetSite(nfCfg, req.SiteID)
			case "create_site":
				logger.Info("LLM requested Netlify create site", "name", req.SiteName, "custom_domain", req.CustomDomain)
				return "Tool Output: " + tools.NetlifyCreateSite(nfCfg, req.SiteName, req.CustomDomain)
			case "update_site":
				logger.Info("LLM requested Netlify update site", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyUpdateSite(nfCfg, req.SiteID, req.SiteName, req.CustomDomain)
			case "delete_site":
				logger.Info("LLM requested Netlify delete site", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyDeleteSite(nfCfg, req.SiteID)
			// ── Deploys ──
			case "list_deploys":
				logger.Info("LLM requested Netlify list deploys", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyListDeploys(nfCfg, req.SiteID)
			case "get_deploy":
				logger.Info("LLM requested Netlify get deploy", "deploy_id", req.DeployID)
				return "Tool Output: " + tools.NetlifyGetDeploy(nfCfg, req.DeployID)
			case "deploy_zip":
				return `Tool Output: {"status":"error","message":"netlify.deploy_zip is not supported in the agent flow. Use the 'homepage' tool with operation='deploy_netlify' so AuraGo can build and upload server-side without fragile base64 ZIP arguments."}`
			case "deploy_draft":
				return `Tool Output: {"status":"error","message":"netlify.deploy_draft is not supported in the agent flow. Use the 'homepage' tool with operation='deploy_netlify' and draft=true so AuraGo can build and upload server-side."}`
			case "rollback":
				logger.Info("LLM requested Netlify rollback", "site_id", req.SiteID, "deploy_id", req.DeployID)
				return "Tool Output: " + tools.NetlifyRollback(nfCfg, req.SiteID, req.DeployID)
			case "cancel_deploy":
				logger.Info("LLM requested Netlify cancel deploy", "deploy_id", req.DeployID)
				return "Tool Output: " + tools.NetlifyCancelDeploy(nfCfg, req.DeployID)
			// ── Environment Variables ──
			case "list_env":
				logger.Info("LLM requested Netlify list env vars", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyListEnvVars(nfCfg, req.SiteID)
			case "get_env":
				logger.Info("LLM requested Netlify get env var", "site_id", req.SiteID, "key", req.EnvKey)
				return "Tool Output: " + tools.NetlifyGetEnvVar(nfCfg, req.SiteID, req.EnvKey)
			case "set_env":
				logger.Info("LLM requested Netlify set env var", "site_id", req.SiteID, "key", req.EnvKey)
				return "Tool Output: " + tools.NetlifySetEnvVar(nfCfg, req.SiteID, req.EnvKey, req.EnvValue, req.EnvContext)
			case "delete_env":
				logger.Info("LLM requested Netlify delete env var", "site_id", req.SiteID, "key", req.EnvKey)
				return "Tool Output: " + tools.NetlifyDeleteEnvVar(nfCfg, req.SiteID, req.EnvKey)
			// ── Files ──
			case "list_files":
				logger.Info("LLM requested Netlify list files", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyListFiles(nfCfg, req.SiteID)
			// ── Forms ──
			case "list_forms":
				logger.Info("LLM requested Netlify list forms", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyListForms(nfCfg, req.SiteID)
			case "get_submissions":
				logger.Info("LLM requested Netlify get form submissions", "form_id", req.FormID)
				return "Tool Output: " + tools.NetlifyGetFormSubmissions(nfCfg, req.FormID)
			// ── Hooks ──
			case "list_hooks":
				logger.Info("LLM requested Netlify list hooks", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyListHooks(nfCfg, req.SiteID)
			case "create_hook":
				logger.Info("LLM requested Netlify create hook", "site_id", req.SiteID, "type", req.HookType, "event", req.HookEvent)
				return "Tool Output: " + tools.NetlifyCreateHook(nfCfg, req.SiteID, req.HookType, req.HookEvent, req.hookData())
			case "delete_hook":
				logger.Info("LLM requested Netlify delete hook", "hook_id", req.HookID)
				return "Tool Output: " + tools.NetlifyDeleteHook(nfCfg, req.HookID)
			// ── SSL ──
			case "provision_ssl":
				logger.Info("LLM requested Netlify provision SSL", "site_id", req.SiteID)
				return "Tool Output: " + tools.NetlifyProvisionSSL(nfCfg, req.SiteID)
			// ── Diagnostics ──
			case "check_connection":
				logger.Info("LLM requested Netlify connection check")
				return "Tool Output: " + tools.NetlifyTestConnection(nfCfg)
			default:
				return `Tool Output: {"status":"error","message":"Unknown netlify operation. Use: list_sites, get_site, create_site, update_site, delete_site, list_deploys, get_deploy, rollback, cancel_deploy, list_env, get_env, set_env, delete_env, list_files, list_forms, get_submissions, list_hooks, create_hook, delete_hook, provision_ssl, check_connection"}`
			}

		case "vercel":
			if !cfg.Vercel.Enabled {
				return `Tool Output: {"status":"error","message":"Vercel integration is not enabled. Set vercel.enabled=true in config.yaml."}`
			}
			req := decodeVercelArgs(tc)
			if cfg.Vercel.ReadOnly {
				switch req.Operation {
				case "create_project", "update_project", "delete_project", "set_env", "delete_env", "add_domain", "verify_domain", "assign_alias", "rollback", "cancel_deploy":
					return `Tool Output: {"status":"error","message":"Vercel is in read-only mode. Disable vercel.readonly to allow changes."}`
				}
			}
			if !cfg.Vercel.AllowProjectManagement {
				switch req.Operation {
				case "create_project", "update_project", "delete_project":
					return `Tool Output: {"status":"error","message":"Vercel project management is not allowed. Set vercel.allow_project_management=true in config.yaml."}`
				}
			}
			if !cfg.Vercel.AllowEnvManagement {
				switch req.Operation {
				case "set_env", "delete_env":
					return `Tool Output: {"status":"error","message":"Vercel environment variable management is not allowed. Set vercel.allow_env_management=true in config.yaml."}`
				}
			}
			if !cfg.Vercel.AllowDeploy {
				switch req.Operation {
				case "rollback", "cancel_deploy":
					return `Tool Output: {"status":"error","message":"Vercel deploy operations are not allowed. Set vercel.allow_deploy=true in config.yaml."}`
				}
			}
			if !cfg.Vercel.AllowDomainManagement {
				switch req.Operation {
				case "add_domain", "verify_domain", "assign_alias":
					return `Tool Output: {"status":"error","message":"Vercel domain and alias management is not allowed. Set vercel.allow_domain_management=true in config.yaml."}`
				}
			}
			token, tokenErr := vault.ReadSecret("vercel_token")
			if tokenErr != nil || token == "" {
				return `Tool Output: {"status":"error","message":"Vercel token not found in vault. Store it with key 'vercel_token' via the Config UI."}`
			}
			vcfg := tools.VercelConfig{
				Token:                  token,
				DefaultProjectID:       cfg.Vercel.DefaultProjectID,
				TeamID:                 cfg.Vercel.TeamID,
				TeamSlug:               cfg.Vercel.TeamSlug,
				ReadOnly:               cfg.Vercel.ReadOnly,
				AllowDeploy:            cfg.Vercel.AllowDeploy,
				AllowProjectManagement: cfg.Vercel.AllowProjectManagement,
				AllowEnvManagement:     cfg.Vercel.AllowEnvManagement,
				AllowDomainManagement:  cfg.Vercel.AllowDomainManagement,
			}
			switch req.Operation {
			case "check_connection":
				logger.Info("LLM requested Vercel connection check")
				return "Tool Output: " + tools.VercelCheckConnection(vcfg)
			case "list_projects":
				logger.Info("LLM requested Vercel list projects")
				return "Tool Output: " + tools.VercelListProjects(vcfg)
			case "get_project":
				logger.Info("LLM requested Vercel get project", "project_id", req.ProjectID)
				return "Tool Output: " + tools.VercelGetProject(vcfg, req.ProjectID)
			case "create_project":
				logger.Info("LLM requested Vercel create project", "project_name", req.ProjectName)
				return "Tool Output: " + tools.VercelCreateProject(vcfg, req.ProjectName, req.Framework, req.RootDirectory, req.OutputDirectory)
			case "update_project":
				logger.Info("LLM requested Vercel update project", "project_id", req.ProjectID)
				return "Tool Output: " + tools.VercelUpdateProject(vcfg, req.ProjectID, req.ProjectName, req.Framework, req.RootDirectory, req.OutputDirectory)
			case "delete_project":
				logger.Info("LLM requested Vercel delete project", "project_id", req.ProjectID)
				return "Tool Output: " + tools.VercelDeleteProject(vcfg, req.ProjectID)
			case "list_deployments":
				logger.Info("LLM requested Vercel list deployments", "project_id", req.ProjectID)
				return "Tool Output: " + tools.VercelListDeployments(vcfg, req.ProjectID)
			case "get_deployment":
				logger.Info("LLM requested Vercel get deployment", "deployment_id", req.DeploymentID)
				return "Tool Output: " + tools.VercelGetDeployment(vcfg, req.DeploymentID)
			case "list_env":
				logger.Info("LLM requested Vercel list env vars", "project_id", req.ProjectID)
				return "Tool Output: " + tools.VercelListEnv(vcfg, req.ProjectID)
			case "set_env":
				logger.Info("LLM requested Vercel set env var", "project_id", req.ProjectID, "key", req.EnvKey)
				return "Tool Output: " + tools.VercelSetEnv(vcfg, req.ProjectID, req.EnvKey, req.EnvValue, req.EnvTarget)
			case "delete_env":
				logger.Info("LLM requested Vercel delete env var", "project_id", req.ProjectID, "key", req.EnvKey)
				return "Tool Output: " + tools.VercelDeleteEnv(vcfg, req.ProjectID, req.EnvKey)
			case "list_domains":
				logger.Info("LLM requested Vercel list domains", "project_id", req.ProjectID)
				return "Tool Output: " + tools.VercelListDomains(vcfg, req.ProjectID)
			case "add_domain":
				logger.Info("LLM requested Vercel add domain", "project_id", req.ProjectID, "domain", req.Domain)
				return "Tool Output: " + tools.VercelAddDomain(vcfg, req.ProjectID, req.Domain)
			case "verify_domain":
				logger.Info("LLM requested Vercel verify domain", "project_id", req.ProjectID, "domain", req.Domain)
				return "Tool Output: " + tools.VercelVerifyDomain(vcfg, req.ProjectID, req.Domain)
			case "list_aliases":
				logger.Info("LLM requested Vercel list aliases", "project_id", req.ProjectID, "deployment_id", req.DeploymentID)
				return "Tool Output: " + tools.VercelListAliases(vcfg, req.ProjectID, req.DeploymentID)
			case "assign_alias":
				logger.Info("LLM requested Vercel assign alias", "deployment_id", req.DeploymentID, "alias", req.Alias)
				return "Tool Output: " + tools.VercelAssignAlias(vcfg, req.DeploymentID, req.Alias)
			case "rollback":
				logger.Info("LLM requested Vercel rollback", "project_id", req.ProjectID, "deployment_id", req.DeploymentID)
				return "Tool Output: " + tools.VercelRollback(vcfg, req.ProjectID, req.DeploymentID)
			case "cancel_deploy":
				logger.Info("LLM requested Vercel cancel deploy", "deployment_id", req.DeploymentID)
				return "Tool Output: " + tools.VercelCancelDeploy(vcfg, req.DeploymentID)
			case "get_env":
				logger.Info("LLM requested Vercel get env var", "project_id", req.ProjectID, "key", req.EnvKey)
				return "Tool Output: " + tools.VercelGetEnv(vcfg, req.ProjectID, req.EnvKey)
			default:
				return `Tool Output: {"status":"error","message":"Unknown vercel operation. Use: check_connection, list_projects, get_project, create_project, update_project, delete_project, list_deployments, get_deployment, rollback, cancel_deploy, list_env, get_env, set_env, delete_env, list_domains, add_domain, verify_domain, list_aliases, assign_alias"}`
			}

		default:
			handled = false
			return ""
		}
	}()
	return result, handled
}
