package virtualcomputers

import "strings"

const (
	ManagementBasePath     = "/boring-computers"
	ManagementListenAddr   = "127.0.0.1:18081"
	ManagementURL          = "http://" + ManagementListenAddr
	PinnedUpstreamRevision = "9752ac7e4d902e425ab0f4047a975ea5bfba7579"
	managementHealthPath   = ManagementBasePath + "/"
)

// ManagementHealthURL returns the upstream management page used for health checks.
func ManagementHealthURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + managementHealthPath
}
