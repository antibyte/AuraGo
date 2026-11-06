package flows

import (
	"path/filepath"
	"strings"
)

func guardFileWritePath(in ExecInput, target string) error {
	if in.Node == nil {
		return nil
	}
	tpl, err := ParseTemplate(FilePath(in.Node.Params["path"]))
	if err != nil {
		return NewNodeError("FLOW_PARAM_INVALID", "the file path template is invalid")
	}
	untrusted := false
	for _, part := range tpl.parts {
		if part.expr != nil && (in.Env == nil || in.Env.Untrusted[part.expr.Root]) {
			untrusted = true
			break
		}
	}
	if !untrusted {
		return nil
	}
	// Outside data may name a file, but the author must fix its directory.
	directory := ""
	if len(tpl.parts) > 0 && tpl.parts[0].expr == nil {
		prefix := strings.TrimSpace(tpl.parts[0].literal)
		directory = prefix[:strings.LastIndex(filepath.ToSlash(prefix), "/")+1]
	}
	name, fixed := strings.CutPrefix(target, directory)
	if directory == "" || !fixed || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return NewNodeError("FLOW_PARAM_INVALID", "outside data may supply only a file name in a fixed folder, for example reports/{{name}}.txt; directory separators and traversal are not allowed in the name")
	}
	return nil
}
