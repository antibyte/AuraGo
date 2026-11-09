package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestCodeStudioPathRecordsPreserveNames(t *testing.T) {
	for _, name := range []string{" leading and trailing ", "colon:12:part", "line\nbreak\n", "a|b", "Grüße文"} {
		path := "/workspace/" + name
		entries, err := parseCodeStudioFindOutput("f|9|1710000000|" + path + "\x00")
		if err != nil || len(entries) != 1 || entries[0].Name != name || entries[0].Path != path {
			t.Fatalf("find %q: %v %#v", name, err, entries)
		}
		results, truncated, err := parseCodeStudioGrepOutput(path + "\x0012:  preview: kept  \n")
		if err != nil || truncated || len(results) != 1 || results[0].Path != path || results[0].Preview != "  preview: kept  " {
			t.Fatalf("search %q: %v %#v", name, err, results)
		}
		if got, err := codeStudioGitAddPath(path); err != nil || got != name {
			t.Fatalf("git path %q: %q %v", name, got, err)
		}
	}
}

// Native Linux runs this in CI. Other hosts can opt into a real managed-image
// container with AURAGO_CODE_STUDIO_DOCKER_SMOKE=1; no host workspace is mounted.
func TestCodeStudioRootedPaths(t *testing.T) {
	encoded, _ := json.Marshal(codeStudioPathScript)
	install, _ := json.Marshal(codeStudioInstallFileScript)
	command, err := buildCodeStudioSearchCommand(codeStudioSearchOptions{Query: "needle", Path: "/workspace", Include: "*.txt", Exclude: "vendor/"})
	if err != nil {
		t.Fatal(err)
	}
	options, _ := json.Marshal(command[len(command)-1])
	script := "SCRIPT = " + string(encoded) + "\nINSTALL = " + string(install) + "\nOPTIONS = " + string(options) + "\n" + codeStudioPathAcceptanceScript
	var cmd *exec.Cmd
	if os.Getenv("AURAGO_CODE_STUDIO_DOCKER_SMOKE") == "1" {
		cmd = exec.Command("docker", "run", "--rm", "-i", "--network", "none", "--user", "developer", "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "FOWNER", "--security-opt", "no-new-privileges:true", "--pids-limit", "1024", "--memory", "1g", "--cpus", "2", publishedCodeStudioImage, "python3", "-")
		script += codeStudioRuntimeAcceptanceScript
	} else {
		if runtime.GOOS != "linux" {
			t.Skip("descriptor traversal requires Linux or the opt-in Docker smoke")
		}
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Skip("Python 3 unavailable")
		}
		cmd = exec.Command(python, "-")
	}
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rooted path acceptance: %v\n%s", err, output)
	}
	t.Log(string(output))
}

const codeStudioRuntimeAcceptanceScript = `
import shutil
assert all(shutil.which(name) for name in ('go','node','tsx','python3','gcc','grep')), 'managed runtime incomplete'
with tempfile.TemporaryDirectory() as project:
    project = pathlib.Path(project)
    files = {
        'hello.go': 'package main\nimport "fmt"\nfunc main(){fmt.Println("runtime-ok")}\n',
        'hello.js': 'console.log("runtime-ok");\n',
        'hello.ts': 'const greeting: string = "runtime-ok";\nconsole.log(greeting);\n',
        'hello.py': 'print("runtime-ok")\n',
        'hello.c': '#include <stdio.h>\nint main(void){puts("runtime-ok");return 0;}\n',
    }
    for name, text in files.items(): (project/name).write_text(text)
    subprocess.run(['gcc','-std=c17','-Wall','-Wextra','hello.c','-o','hello-c'],cwd=project,check=True,capture_output=True,timeout=60)
    for command in [['go','run','hello.go'],['node','hello.js'],['tsx','hello.ts'],['python3','hello.py'],['./hello-c']]:
        result = subprocess.run(command,cwd=project,check=True,capture_output=True,timeout=90,env={**os.environ,'GOTOOLCHAIN':'local'})
        assert result.stdout.strip() == b'runtime-ok', (command,result.stdout)
print('Managed Code Studio runtime: Go, Node, TypeScript, Python and C execution OK')
`

const codeStudioPathAcceptanceScript = `import base64, hashlib, json, os, pathlib, subprocess, sys, tempfile

def run(root, op, path, *args, script=SCRIPT):
    return subprocess.run([sys.executable, '-c', script, str(root), op, str(path), *map(str,args)], capture_output=True)

with tempfile.TemporaryDirectory() as scratch:
    base = pathlib.Path(scratch)
    root, outside = base/'workspace', base/'outside'
    root.mkdir(); outside.mkdir()
    sentinel = outside/'keep.txt'
    sentinel.write_text('outside needle')
    (root/'escape').symlink_to(outside, target_is_directory=True)
    names = [' leading:12:trailing ', 'new\nline\n', 'pipe|name', 'Grüße文']
    for name in names:
        path = root/(name+'.txt')
        path.write_text('  needle: kept  \n')
        listed = run(root, 'list', root)
        assert listed.returncode == 0, listed.stderr
        assert os.fsencode(path)+b'\0' in listed.stdout, (name, listed.stdout)
        found = run(root, 'search', root, OPTIONS)
        assert found.returncode == 0, found.stderr
        assert os.fsencode(path)+b'\0'+b'1:  needle: kept  \n' in found.stdout, found.stdout
        assert b'outside needle' not in found.stdout
        found_file = run(root, 'search', path, OPTIONS)
        assert found_file.returncode == 0 and found_file.stdout == os.fsencode(path)+b'\0'+b'1:  needle: kept  \n', found_file.stderr
        read = run(root, 'read', path, 1024)
        assert read.returncode == 0 and base64.b64decode(read.stdout) == b'  needle: kept  \n', read.stderr
        probe = run(root, 'probe', path)
        assert probe.returncode == 0 and probe.stdout.endswith(os.fsencode(path)+b'\0'), probe.stderr
        moved = root/(name+'.moved')
        assert run(root, 'move', path, moved).returncode == 0
        assert moved.read_text() == '  needle: kept  \n'
        assert run(root, 'delete', moved).returncode == 0 and not moved.exists()
    (root/'local.txt').write_text('inside needle')
    (root/'local-link').symlink_to('local.txt')
    read = run(root, 'read', root/'local-link', 1024)
    assert read.returncode == 0 and base64.b64decode(read.stdout) == b'inside needle', read.stderr
    for op, path, args in [
        ('list', root/'escape', []), ('search', root/'escape', [OPTIONS]), ('search', root/'local-link', [OPTIONS]),
        ('read', root/'escape'/'keep.txt', [1024]), ('probe', root/'escape'/'keep.txt', []),
        ('mkdir', root/'escape'/'new', []), ('delete', root/'escape'/'keep.txt', []),
        ('move', root/'escape'/'keep.txt', [root/'stolen']), ('move', root/'local.txt', [root/'escape'/'new.txt'])
    ]:
        result = run(root, op, path, *args)
        assert result.returncode != 0, (op, path, result.stdout)
        assert sentinel.read_text() == 'outside needle'
    assert not (outside/'new').exists() and not (outside/'new.txt').exists()
    assert run(root, 'delete', root/'escape').returncode == 0
    assert sentinel.exists(), 'deleting link followed it'
    (root/'nested').mkdir(); (root/'nested'/'link').symlink_to(outside, target_is_directory=True)
    assert run(root, 'delete', root/'nested').returncode == 0 and sentinel.exists()
    (root/'vendor').mkdir(); (root/'vendor'/'skip.txt').write_text('needle')
    found = run(root, 'search', root, OPTIONS)
    assert found.returncode == 0 and b'skip.txt' not in found.stdout
    (root/'many.txt').write_text('needle\n'*250)
    found = run(root, 'search', root, OPTIONS)
    assert found.returncode == 0 and found.stdout.count(b'\0') == 201, found.stderr
    too_large = run(root, 'read', root/'many.txt', 3)
    assert too_large.returncode != 0
    # Force a directory exchange at the exact open boundary. Before-open must
    # reject the link; after-open must operate on the held directory, not it.
    for timing in ['before', 'after']:
        for op in ['mkdir', 'delete', 'move', 'list', 'search', 'read']:
            race = root/'race'; held = root/'held'
            race.mkdir(); (race/'keep.txt').write_text('inside needle')
            hook = '''
original_open = os.open
swapped = False
def racing_open(name, flags, *args, **kwargs):
    global swapped
    if name == 'race' and kwargs.get('dir_fd') is not None and not swapped:
        swapped = True
        def exchange():
            os.rename(os.path.join(root,'race'), os.path.join(root,'held'))
            os.symlink(os.path.join(os.path.dirname(root),'outside'), os.path.join(root,'race'))
        if TIMING == 'before': exchange()
        result = original_open(name,flags,*args,**kwargs)
        if TIMING == 'after': exchange()
        return result
    return original_open(name,flags,*args,**kwargs)
os.open = racing_open
'''.replace('TIMING',repr(timing))
            injected = SCRIPT.replace('directory_flags =', hook+'\ndirectory_flags =', 1)
            path, args = race, []
            if op == 'mkdir': path = race/'new'
            if op in ('delete','move','read'): path = race/'keep.txt'
            if op == 'move': args = [race/'renamed.txt']
            if op == 'search': args = [OPTIONS]
            if op == 'read': args = [1024]
            result = run(root, op, path, *args, script=injected)
            assert sentinel.read_text() == 'outside needle', (timing,op)
            assert not (outside/'new').exists() and not (outside/'renamed.txt').exists(), (timing,op)
            assert b'outside needle' not in result.stdout, (timing,op)
            if op == 'read' and result.returncode == 0:
                assert base64.b64decode(result.stdout) == b'inside needle'
            race.unlink()
            import shutil
            shutil.rmtree(held)
    # Archive installation uses the same no-follow publication contract.
    source = base/'staged'; source.write_bytes(b'complete source')
    def install(target):
        return subprocess.run([sys.executable,'-c',INSTALL,str(root),str(target),str(source),'false',hashlib.sha256(source.read_bytes()).hexdigest(),str(source.stat().st_size)], capture_output=True)
    assert install(root/'saved.txt').returncode == 0
    (root/'escape').symlink_to(outside, target_is_directory=True)
    assert install(root/'escape'/'keep.txt').returncode != 0
    assert sentinel.read_text() == 'outside needle'
print('Symlink boundaries, concurrent path exchanges, unusual names, search limit and file installation: OK')
`
