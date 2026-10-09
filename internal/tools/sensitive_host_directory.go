package tools

// sensitiveHostDataTrees are system program and runtime trees AuraGo never
// stores managed data in, in addition to the Docker bind denylist.
var sensitiveHostDataTrees = []string{"/bin", "/sbin", "/usr", "/lib", "/lib32", "/lib64", "/run", "/var/run"}

// IsSensitiveHostDirectory reports whether a directory must be refused as the
// storage location of AuraGo-managed data files (such as the Local Wikipedia
// storage directory). It reuses the Docker bind denylist (sensitiveDockerHostPaths,
// drive roots and Windows system folders) with one exception: directories
// below /mnt, where Linux hosts mount data disks, are allowed; /mnt itself stays
// refused. The system program trees in sensitiveHostDataTrees are refused too.
func IsSensitiveHostDirectory(dir string) bool {
	cleaned := cleanDockerHostPath(dir)
	if dockerHostPathIsSensitiveLocation(cleaned) {
		return true
	}
	if !dockerPathEqualOrWithin(cleaned, "/mnt") && isSensitiveDockerHostPath(cleaned) {
		return true
	}
	for _, tree := range sensitiveHostDataTrees {
		if dockerPathEqualOrWithin(cleaned, tree) {
			return true
		}
	}
	return false
}
