package tools

import (
	"fmt"
	"os"
	"runtime"
)

func hardenManagedSidecarHostConfig(hostConfig map[string]interface{}) map[string]interface{} {
	if hostConfig == nil {
		hostConfig = map[string]interface{}{}
	}
	hostConfig["SecurityOpt"] = []string{"no-new-privileges:true"}
	hostConfig["CapDrop"] = []string{"ALL"}
	return hostConfig
}

func managedContainerUserSpec() string {
	return managedContainerUserSpecForGOOS(runtime.GOOS, os.Getuid(), os.Getgid())
}

func managedContainerUserSpecForGOOS(goos string, uid, gid int) string {
	if goos == "windows" || uid < 0 || gid < 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", uid, gid)
}
