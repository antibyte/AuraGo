package tools

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

func defaultRuntimePermissionsForTests() RuntimePermissions {
	return RuntimePermissions{
		AllowShell:           true,
		AllowPython:          true,
		AllowFilesystemWrite: true,
		AllowNetworkRequests: true,
		DockerEnabled:        true,
		DockerReadOnly:       false,
		SchedulerEnabled:     true,
		MissionsEnabled:      true,
		MQTTEnabled:          true,
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("AURAGO_FAKE_PYTHON") == "1" {
		runFakePythonForEnvTests()
		return
	}
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	code := m.Run()
	closeAllSystemTaskStores()
	os.Exit(code)
}

func runFakePythonForEnvTests() {
	var leaked []string
	for _, key := range []string{"AURAGO_MASTER_KEY", "OPENAI_API_KEY", "CUSTOM_PASSWORD"} {
		if os.Getenv(key) != "" {
			leaked = append(leaked, key)
		}
	}
	if len(leaked) > 0 {
		fmt.Printf("leaked:%v\n", leaked)
	} else {
		fmt.Println("env-clean")
	}
	if sleepMS, _ := strconv.Atoi(os.Getenv("AURAGO_FAKE_PYTHON_SLEEP_MS")); sleepMS > 0 {
		time.Sleep(time.Duration(sleepMS) * time.Millisecond)
	}
	os.Exit(0)
}

func tempSystemTaskDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(closeAllSystemTaskStores)
	return dir
}
