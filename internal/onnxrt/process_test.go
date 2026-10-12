package onnxrt

import (
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aurago/internal/sandbox"
)

var listSeparator = string(os.PathListSeparator)

func absoluteTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRuntimeEnvironmentPrependsPrivateDirectories(t *testing.T) {
	root := absoluteTempDir(t)
	library := filepath.Join(root, "lib")
	if err := os.MkdirAll(library, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "keep-a"+listSeparator+"keep-b")
	environment := runtimeEnvironment(goruntime.GOOS, root)
	want := root + listSeparator + library + listSeparator + "keep-a" + listSeparator + "keep-b"
	if got := environmentValue(goruntime.GOOS, environment, "PATH"); got != want {
		t.Fatalf("PATH = %q, want %q", got, want)
	}
	for key, value := range map[string]string{
		"AURAGO_MASTER_KEY":   "master-secret",
		"OPENAI_API_KEY":      "api-secret",
		"CUSTOM_AUTH_TOKEN":   "token-secret",
		"MY_DB_PASSWORD":      "password-secret",
		"PROJECT_CREDENTIALS": "credentials-secret",
	} {
		t.Setenv(key, value)
	}
	environment = runtimeEnvironment(goruntime.GOOS, root)
	for _, entry := range environment {
		upper := strings.ToUpper(entry)
		for _, forbidden := range []string{
			"AURAGO_MASTER_KEY=",
			"OPENAI_API_KEY=",
			"CUSTOM_AUTH_TOKEN=",
			"MY_DB_PASSWORD=",
			"PROJECT_CREDENTIALS=",
		} {
			if strings.HasPrefix(upper, forbidden) {
				t.Fatalf("sensitive environment variable escaped into native process: %s", forbidden)
			}
		}
	}
}

// The per-platform rules run on a synthetic environment: the real one spells
// PATH as the host does ("Path" when a Windows binary is started by an IDE or
// debugger), which the case rules of another goos would not match.
func TestPrependLibraryDirectoriesPerPlatform(t *testing.T) {
	directories := []string{"/d1", "/d2"}
	prefix := "/d1" + listSeparator + "/d2" + listSeparator
	cases := []struct {
		goos string
		want []string
	}{
		{"linux", []string{"PATH=" + prefix + "/keep-path", "LD_LIBRARY_PATH=" + prefix + "/keep-ld", "DYLD_LIBRARY_PATH=/keep-dyld", "OTHER=1"}},
		{"darwin", []string{"PATH=" + prefix + "/keep-path", "LD_LIBRARY_PATH=/keep-ld", "DYLD_LIBRARY_PATH=" + prefix + "/keep-dyld", "OTHER=1"}},
		{"windows", []string{"PATH=" + prefix + "/keep-path", "LD_LIBRARY_PATH=/keep-ld", "DYLD_LIBRARY_PATH=/keep-dyld", "OTHER=1"}},
	}
	for _, c := range cases {
		environment := []string{"PATH=/keep-path", "LD_LIBRARY_PATH=/keep-ld", "DYLD_LIBRARY_PATH=/keep-dyld", "OTHER=1"}
		if got := prependLibraryDirectories(c.goos, environment, directories); !slices.Equal(got, c.want) {
			t.Errorf("%s environment = %v, want %v", c.goos, got, c.want)
		}
	}
}

func TestRuntimeEnvironmentSetsLibraryPathForTheHost(t *testing.T) {
	root := absoluteTempDir(t)
	t.Setenv("LD_LIBRARY_PATH", "/opt/keep-ld")
	t.Setenv("DYLD_LIBRARY_PATH", "/opt/keep-dyld")
	wantLD, wantDY := "/opt/keep-ld", "/opt/keep-dyld"
	switch goruntime.GOOS {
	case "linux":
		wantLD = root + listSeparator + wantLD
	case "darwin":
		wantDY = root + listSeparator + wantDY
	}
	environment := runtimeEnvironment(goruntime.GOOS, root)
	if got := environmentValue(goruntime.GOOS, environment, "PATH"); !strings.HasPrefix(got, root+listSeparator) {
		t.Errorf("PATH = %q, want prefix %q", got, root+listSeparator)
	}
	if got := environmentValue(goruntime.GOOS, environment, "LD_LIBRARY_PATH"); got != wantLD {
		t.Errorf("LD_LIBRARY_PATH = %q, want %q", got, wantLD)
	}
	if got := environmentValue(goruntime.GOOS, environment, "DYLD_LIBRARY_PATH"); got != wantDY {
		t.Errorf("DYLD_LIBRARY_PATH = %q, want %q", got, wantDY)
	}
}

// A root that yields no directory must leave the search paths exactly as
// they were: an empty element would put the working directory on them.
func TestRuntimeEnvironmentIgnoresRootsWithoutDirectories(t *testing.T) {
	t.Setenv("PATH", "keep-a"+listSeparator+"keep-b")
	t.Setenv("LD_LIBRARY_PATH", "/opt/keep-ld")
	t.Setenv("DYLD_LIBRARY_PATH", "/opt/keep-dyld")
	dir := absoluteTempDir(t)
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := map[string][]string{
		"missing":      {filepath.Join(dir, "missing")},
		"empty":        {""},
		"only empty":   {"", ""},
		"plain file":   {file},
		"all unusable": {"", filepath.Join(dir, "missing"), file},
	}
	for name, rootList := range roots {
		// The real environment, for the host platform.
		environment := runtimeEnvironment(goruntime.GOOS, rootList...)
		if got := environmentValue(goruntime.GOOS, environment, "PATH"); got != "keep-a"+listSeparator+"keep-b" {
			t.Errorf("%s PATH = %q", name, got)
		}
		if got := environmentValue(goruntime.GOOS, environment, "LD_LIBRARY_PATH"); got != "/opt/keep-ld" {
			t.Errorf("%s LD_LIBRARY_PATH = %q", name, got)
		}
		if got := environmentValue(goruntime.GOOS, environment, "DYLD_LIBRARY_PATH"); got != "/opt/keep-dyld" {
			t.Errorf("%s DYLD_LIBRARY_PATH = %q", name, got)
		}
		if want := sandbox.FilterEnv(os.Environ()); !slices.Equal(environment, want) {
			t.Errorf("%s changed the environment although no directory was found", name)
		}

		// Every platform's rules, on a synthetic environment.
		directories := runtimeDirectories(rootList)
		if len(directories) != 0 {
			t.Errorf("%s: directories = %v", name, directories)
		}
		for _, goos := range []string{"linux", "darwin", "windows"} {
			synthetic := []string{"PATH=keep-a" + listSeparator + "keep-b", "LD_LIBRARY_PATH=/opt/keep-ld", "DYLD_LIBRARY_PATH=/opt/keep-dyld"}
			want := slices.Clone(synthetic)
			if got := prependLibraryDirectories(goos, synthetic, directories); !slices.Equal(got, want) {
				t.Errorf("%s/%s environment = %v, want %v", name, goos, got, want)
			}
		}
	}
}

func TestRuntimeEnvironmentDoesNotCreateEmptyLibraryPath(t *testing.T) {
	root := absoluteTempDir(t)
	for _, goos := range []string{"linux", "darwin"} {
		key := map[string]string{"linux": "LD_LIBRARY_PATH", "darwin": "DYLD_LIBRARY_PATH"}[goos]
		// The variable is absent from the environment: no usable directory
		// must not create it, a usable one creates it without a separator.
		unchanged := prependLibraryDirectories(goos, []string{"PATH=/keep"}, runtimeDirectories([]string{filepath.Join(root, "missing")}))
		if want := []string{"PATH=/keep"}; !slices.Equal(unchanged, want) {
			t.Errorf("%s: environment = %v, want %v", goos, unchanged, want)
		}
		created := prependLibraryDirectories(goos, []string{"PATH=/keep"}, runtimeDirectories([]string{root}))
		if got := environmentValue(goos, created, key); got != root {
			t.Errorf("%s: %s = %q, want %q", goos, key, got, root)
		}
		if got := environmentValue(goos, created, "PATH"); got != root+listSeparator+"/keep" {
			t.Errorf("%s: PATH = %q", goos, got)
		}
	}
}

func TestRuntimeEnvironmentSkipsUnusableRootsAmongUsableOnes(t *testing.T) {
	t.Setenv("PATH", "keep")
	root := absoluteTempDir(t)
	environment := runtimeEnvironment(goruntime.GOOS, "", filepath.Join(root, "missing"), root)
	if got, want := environmentValue(goruntime.GOOS, environment, "PATH"), root+listSeparator+"keep"; got != want {
		t.Fatalf("PATH = %q, want %q", got, want)
	}
}

// Relative roots would otherwise resolve against the worker's working
// directory.
func TestRuntimeEnvironmentMakesRootsAbsolute(t *testing.T) {
	t.Setenv("PATH", "keep")
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("runtime", "lib"), 0o750); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("runtime")
	if err != nil {
		t.Fatal(err)
	}
	environment := runtimeEnvironment(goruntime.GOOS, "runtime")
	want := root + listSeparator + filepath.Join(root, "lib") + listSeparator + "keep"
	if got := environmentValue(goruntime.GOOS, environment, "PATH"); got != want {
		t.Fatalf("PATH = %q, want %q", got, want)
	}
	for _, directory := range strings.Split(environmentValue(goruntime.GOOS, environment, "PATH"), listSeparator)[:2] {
		if !filepath.IsAbs(directory) {
			t.Errorf("injected PATH entry %q is not absolute", directory)
		}
	}
}

func TestEnvironmentNamesAreCaseSensitiveOutsideWindows(t *testing.T) {
	environment := []string{"path=/lower", "PATH=/upper", "Ld_Library_Path=/other"}
	if got := environmentValue("linux", environment, "PATH"); got != "/upper" {
		t.Fatalf("linux PATH = %q", got)
	}
	if got := environmentValue("darwin", environment, "PATH"); got != "/upper" {
		t.Fatalf("darwin PATH = %q", got)
	}
	if got := environmentValue("linux", environment, "LD_LIBRARY_PATH"); got != "" {
		t.Fatalf("linux LD_LIBRARY_PATH = %q", got)
	}

	updated := setEnvironmentValue("linux", slices.Clone(environment), "PATH", "/new")
	if want := []string{"path=/lower", "PATH=/new", "Ld_Library_Path=/other"}; !slices.Equal(updated, want) {
		t.Fatalf("linux set = %v, want %v", updated, want)
	}
	appended := setEnvironmentValue("linux", []string{"path=/lower"}, "PATH", "/new")
	if want := []string{"path=/lower", "PATH=/new"}; !slices.Equal(appended, want) {
		t.Fatalf("linux append = %v, want %v", appended, want)
	}

	// Windows names are case-insensitive: "Path" is PATH there, a different
	// variable elsewhere.
	mixed := []string{"Path=/mixed", "OTHER=1"}
	if got := environmentValue("windows", mixed, "PATH"); got != "/mixed" {
		t.Fatalf("windows PATH = %q", got)
	}
	if got := environmentValue("linux", mixed, "PATH"); got != "" {
		t.Fatalf("linux PATH = %q", got)
	}
	updated = setEnvironmentValue("windows", slices.Clone(mixed), "PATH", "/new")
	if want := []string{"PATH=/new", "OTHER=1"}; !slices.Equal(updated, want) {
		t.Fatalf("windows set = %v, want %v", updated, want)
	}
	updated = setEnvironmentValue("linux", slices.Clone(mixed), "PATH", "/new")
	if want := []string{"Path=/mixed", "OTHER=1", "PATH=/new"}; !slices.Equal(updated, want) {
		t.Fatalf("linux set = %v, want %v", updated, want)
	}

	prepended := prependLibraryDirectories("linux", []string{"path=/lower", "PATH=/upper"}, []string{"/d"})
	if want := []string{"path=/lower", "PATH=/d" + listSeparator + "/upper", "LD_LIBRARY_PATH=/d"}; !slices.Equal(prepended, want) {
		t.Fatalf("linux prepend = %v, want %v", prepended, want)
	}
}

func TestConfigureWorkerCommandWithoutRootsKeepsPath(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-secret")
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command)
	if got, want := environmentValue(goruntime.GOOS, command.Env, "PATH"), os.Getenv("PATH"); got != want {
		t.Fatalf("PATH = %q, want unchanged %q", got, want)
	}
	for _, entry := range command.Env {
		if strings.HasPrefix(strings.ToUpper(entry), "OPENAI_API_KEY=") {
			t.Fatal("sensitive variable reached the worker environment")
		}
	}
}

func TestConfigureWorkerCommandPrependsRuntimeRoot(t *testing.T) {
	root := absoluteTempDir(t)
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command, root)
	if value := environmentValue(goruntime.GOOS, command.Env, "PATH"); !strings.HasPrefix(value, root+listSeparator) {
		t.Fatalf("PATH = %q, want prefix %q", value, root+listSeparator)
	}
}

// ConfigureWorkerCommand replaces Env wholesale; callers add their own
// variables afterwards.
func TestConfigureWorkerCommandReplacesEnv(t *testing.T) {
	command := exec.Command("aurago-test-worker")
	command.Env = []string{"CALLER_VARIABLE=1"}
	ConfigureWorkerCommand(command)
	if environmentValue(goruntime.GOOS, command.Env, "CALLER_VARIABLE") != "" {
		t.Fatal("a pre-set Env survived ConfigureWorkerCommand")
	}
}

// exec.Cmd treats a nil Env as "inherit everything", secrets included, so a
// filtered environment that ends up empty must stay a non-nil empty slice.
// ConfigureWorkerCommand relies on sandbox.FilterEnv guaranteeing that.
func TestFilterEnvironmentNeverFallsBackToInheritance(t *testing.T) {
	filtered := sandbox.FilterEnv([]string{"OPENAI_API_KEY=api-secret", "AURAGO_MASTER_KEY=master-secret"})
	if filtered == nil || len(filtered) != 0 {
		t.Fatalf("filtered = %#v, want a non-nil empty slice", filtered)
	}
	if got := sandbox.FilterEnv([]string{"KEEP=1", "OPENAI_API_KEY=api-secret"}); len(got) != 1 || got[0] != "KEEP=1" {
		t.Fatalf("filtered = %#v", got)
	}
}

func TestAccessibleRenderNodeRequiresUsableRenderNode(t *testing.T) {
	driRoot := t.TempDir()
	if _, err := AccessibleRenderNode(driRoot); err == nil {
		t.Fatal("empty DRI directory unexpectedly enabled Vulkan")
	}

	renderDevice := filepath.Join(driRoot, "renderD128")
	if err := os.WriteFile(renderDevice, []byte("test render device"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := AccessibleRenderNode(driRoot)
	if err != nil {
		t.Fatalf("accessible render device was rejected: %v", err)
	}
	if found != renderDevice {
		t.Fatalf("render device = %q, want %q", found, renderDevice)
	}
}

// The probe's error texts are part of the embeddings hardware fingerprint.
func TestAccessibleRenderNodeKeepsEmbeddingsErrorText(t *testing.T) {
	driRoot := t.TempDir()
	_, err := AccessibleRenderNode(driRoot)
	if err == nil || err.Error() != "no Vulkan render device found under "+driRoot {
		t.Fatalf("error = %v", err)
	}

	// A directory named renderD128 exists but cannot be opened read-write.
	blocked := t.TempDir()
	if err := os.Mkdir(filepath.Join(blocked, "renderD128"), 0o750); err != nil {
		t.Fatal(err)
	}
	found, err := AccessibleRenderNode(blocked)
	prefix := "no accessible Vulkan render device under " + blocked + " (renderD128: "
	if found != "" || err == nil || !strings.HasPrefix(err.Error(), prefix) || !strings.HasSuffix(err.Error(), ")") {
		t.Fatalf("AccessibleRenderNode = %q, %v; want an error starting %q", found, err, prefix)
	}
}
