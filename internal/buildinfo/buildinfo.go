package buildinfo

import (
	"runtime/debug"
	"strings"
)

// BuildID and the BuildVCS* values may be set at link time by the release
// builder. When unset, Current derives VCS metadata embedded by the Go
// toolchain.
var (
	BuildID          string
	BuildVCSRevision string
	BuildVCSTime     string
	BuildVCSModified string
)

type Info struct {
	BuildID     string `json:"build_id"`
	VCSRevision string `json:"vcs_revision"`
	VCSTime     string `json:"vcs_time"`
	VCSModified bool   `json:"vcs_modified"`
}

func Current() Info {
	info := Info{
		BuildID:     strings.TrimSpace(BuildID),
		VCSRevision: strings.TrimSpace(BuildVCSRevision),
		VCSTime:     strings.TrimSpace(BuildVCSTime),
		VCSModified: strings.EqualFold(strings.TrimSpace(BuildVCSModified), "true"),
	}
	build, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.VCSRevision == "" {
					info.VCSRevision = strings.TrimSpace(setting.Value)
				}
			case "vcs.time":
				if info.VCSTime == "" {
					info.VCSTime = strings.TrimSpace(setting.Value)
				}
			case "vcs.modified":
				if strings.TrimSpace(BuildVCSModified) == "" {
					info.VCSModified = strings.EqualFold(strings.TrimSpace(setting.Value), "true")
				}
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
