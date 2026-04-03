# -*- coding: utf-8 -*-
with open('internal/agent/agent_dispatch_exec.go', 'r', encoding='utf-8') as f:
    text = f.read()

cases = '''
                case "budget_status":
                        return tools.ExecuteBudgetStatus(ctx.BudgetTracker)
                        
                case "certificate_manager":
                        op := stringValueFromMap(tc.Arguments, "operation")
                        certName := stringValueFromMap(tc.Arguments, "cert_name")
                        return tools.ExecuteCertificateManager(op, certName, ctx.Vault)
                        
                case "toml_editor":
                        req := decodeTOMLEditorArgs(tc)
                        fpath := req.FilePath
                        op := strings.TrimSpace(strings.ToLower(req.Operation))
                        wsDir := cfg.Directories.WorkspaceDir
                        if isProtectedSystemPath(fpath, wsDir, cfg) {
                                logger.Warn("LLM attempted toml_editor access to protected system file — blocked", "op", op, "path", fpath)
                                return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed."
                        }
                        switch op {
                        case "set", "delete":
                                if !cfg.Agent.AllowFilesystemWrite {
                                        return "Tool Output: [PERMISSION DENIED] toml_editor 'set'/'delete' requires allow_filesystem_write=true"
                                }
                        }
                        return tools.ExecuteTomlEditor(op, fpath, req.TomlPath, req.SetValue, wsDir)
'''
# find yaml_editor case
idx = text.find('case "yaml_editor":')
if idx != -1:
    text = text[:idx] + cases.lstrip() + text[idx:]
    with open('internal/agent/agent_dispatch_exec.go', 'w', encoding='utf-8') as f:
        f.write(text)
    print("Added tool execution cases")
else:
    print("Could not find yaml_editor case")
