package buildinfo

import (
	"runtime/debug"
	"strings"
)

// BuildID may be set at link time with -X aurago/internal/buildinfo.BuildID=<commit>.
// When unset, Current derives the VCS revision embedded by the Go toolchain.
var BuildID string

type Info struct {
	BuildID     string `json:"build_id"`
	VCSRevision string `json:"vcs_revision"`
	VCSTime     string `json:"vcs_time"`
	VCSModified bool   `json:"vcs_modified"`
}

func Current() Info {
	info := Info{BuildID: strings.TrimSpace(BuildID)}
	build, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.VCSRevision = strings.TrimSpace(setting.Value)
			case "vcs.time":
				info.VCSTime = strings.TrimSpace(setting.Value)
			case "vcs.modified":
				info.VCSModified = strings.EqualFold(strings.TrimSpace(setting.Value), "true")
			}
		}
		if info.BuildID == "" && info.VCSRevision == "" && strings.TrimSpace(build.Main.Version) != "" && build.Main.Version != "(devel)" {
			info.BuildID = strings.TrimSpace(build.Main.Version)
		}
	}
	if info.BuildID == "" {
		info.BuildID = info.VCSRevision
	}
	if info.BuildID == "" {
		info.BuildID = "devel"
	}
	if info.VCSModified && !strings.HasSuffix(info.BuildID, "-dirty") {
		info.BuildID += "-dirty"
	}
	return info
}
