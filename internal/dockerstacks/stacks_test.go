package dockerstacks

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const stacksTestCompose = "services:\n  web:\n    image: nginx:1.27\n    ports:\n      - \"8080:80\"\n  db:\n    image: postgres:16\n"

func TestParseComposeServices(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string // "name=image,name=image"
		wantErr bool
	}{
		{"file order", stacksTestCompose, "web=nginx:1.27,db=postgres:16", false},
		{"no services key", "name: x\n", "", false},
		{"empty file", "", "", false},
		{"a service without image", "services:\n  app:\n    build: .\n  api:\n    image: x\n", "app=,api=x", false},
		{"extension fields and anchors are not services", "x-common: &c\n  restart: always\nservices:\n  a:\n    <<: *c\n    image: a:1\n", "a=a:1", false},
		{"services that is not a mapping", "services: []\n", "", false},
		{"top level list", "- a\n- b\n", "", true},
		{"yaml syntax error", "services:\n  web:\n   image: [nginx\n", "", true},
	}
	for _, c := range cases {
		got, err := parseComposeServices([]byte(c.content))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		parts := make([]string, len(got))
		for i, s := range got {
			parts[i] = s.name + "=" + s.image
		}
		if strings.Join(parts, ",") != c.want {
			t.Errorf("%s: services = %q, want %q", c.name, strings.Join(parts, ","), c.want)
		}
	}
}

func TestGetReadsFilesServicesAndStatus(t *testing.T) {
	env := newTestEnv(t, true)
	env.fe.putFile("/opt/stacks/shop/compose.yaml", stacksTestCompose)
	env.fe.putFile("/opt/stacks/shop/.env", "DB_PASSWORD=secret\n")
	now := time.Now().Unix()
	env.fe.addProjectContainer("shop-web-1", "shop", "web", "running", "Up 1 hour (healthy)", "healthy", now-3600,
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"})
	env.fe.addProjectContainer("shop-worker-1", "shop", "worker", "exited", "Exited (2) 5 minutes ago", "", now-3000)
	env.fe.addProjectContainer("other-web-1", "other", "web", "running", "Up 1 hour", "", now-3600)

	detail, err := env.svc.Get(context.Background(), testHostID, "shop")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Name != "shop" || detail.HostID != testHostID || detail.Dir != "/opt/stacks/shop" || detail.ComposeFile != "compose.yaml" || !detail.HasEnv {
		t.Fatalf("summary = %+v", detail.StackSummary)
	}
	if detail.Status != StatusPartial || detail.Services.Total != 2 || detail.Services.Running != 1 {
		t.Fatalf("status = %s %+v, want partial with 1 of 2 running (only the shop project counts)", detail.Status, detail.Services)
	}
	if detail.Compose.Name != "compose.yaml" || detail.Compose.Content != stacksTestCompose || detail.Compose.Revision != Revision([]byte(stacksTestCompose)) || !detail.Compose.Exists {
		t.Fatalf("compose = %+v", detail.Compose)
	}
	if detail.Env.Name != ".env" || detail.Env.Content != "DB_PASSWORD=secret\n" || !detail.Env.Exists {
		t.Fatalf("env = %+v", detail.Env)
	}
	if detail.ParseError != "" {
		t.Fatalf("parse error = %q", detail.ParseError)
	}
	var names []string
	for _, s := range detail.ServiceList {
		names = append(names, s.Name+":"+s.State)
	}
	if strings.Join(names, ",") != "web:running,db:missing,worker:exited" {
		t.Fatalf("service list = %v, want the file's services in order (db has no container) and then the stray worker", names)
	}
	web, db, worker := detail.ServiceList[0], detail.ServiceList[1], detail.ServiceList[2]
	if web.Image != "nginx:1.27" || web.Health != "healthy" || len(web.Ports) != 1 || web.ContainerID == "" || web.ExitCode != nil {
		t.Fatalf("web = %+v", web)
	}
	if db.Image != "postgres:16" || db.ContainerID != "" || db.Ports == nil {
		t.Fatalf("db = %+v", db)
	}
	if worker.Image != "worker:latest" || worker.ExitCode == nil || *worker.ExitCode != 2 {
		t.Fatalf("worker = %+v, want the container's image and exit code 2", worker)
	}
}

func TestGetUsesDockgesLookupOrderAndReportsAParseError(t *testing.T) {
	env := newTestEnv(t, true)
	env.fe.putFile("/opt/stacks/old/docker-compose.yml", "services:\n  a:\n    image: a\n")
	env.fe.putFile("/opt/stacks/both/docker-compose.yml", "services: {}\n")
	env.fe.putFile("/opt/stacks/both/compose.yml", "services: {}\n")
	env.fe.putFile("/opt/stacks/broken/compose.yaml", "services:\n  web:\n   image: [nginx\n")
	env.fe.addProjectContainer("broken-web-1", "broken", "web", "running", "Up 1 hour", "", time.Now().Unix())

	ctx := context.Background()
	old, err := env.svc.Get(ctx, testHostID, "old")
	if err != nil || old.ComposeFile != "docker-compose.yml" || old.Env.Exists || old.Env.Revision != "" || old.HasEnv {
		t.Fatalf("old = %+v, %v", old.StackSummary, err)
	}
	both, _ := env.svc.Get(ctx, testHostID, "both")
	if both.ComposeFile != "compose.yml" {
		t.Fatalf("both = %q, want compose.yml (it comes before docker-compose.yml)", both.ComposeFile)
	}
	broken, err := env.svc.Get(ctx, testHostID, "broken")
	if err != nil || broken.ParseError == "" || len(broken.ServiceList) != 1 || broken.ServiceList[0].Name != "web" || broken.ServiceList[0].State != "running" {
		t.Fatalf("broken = parse %q services %+v, %v; want the services from the containers", broken.ParseError, broken.ServiceList, err)
	}
	if strings.Contains(broken.ParseError, "\n") {
		t.Fatalf("the parse error must be one line: %q", broken.ParseError)
	}

	if _, err := env.svc.Get(ctx, testHostID, "ghost"); CodeOf(err) != CodeStackNotFound {
		t.Fatalf("Get of a missing stack = %v, want %s", err, CodeStackNotFound)
	}
	if _, err := env.svc.Get(ctx, testHostID, "../etc"); CodeOf(err) != CodeStackNameInvalid {
		t.Fatalf("Get with a traversal name = %v, want %s", err, CodeStackNameInvalid)
	}
}

func TestGetOnAHostWhoseIOContainerVanishedAnswersNotPrepared(t *testing.T) {
	env := newTestEnv(t, true)
	env.fe.putFile("/opt/stacks/a/compose.yaml", "services: {}\n")
	if _, err := env.svc.Get(context.Background(), testHostID, "a"); err != nil {
		t.Fatal(err)
	}
	// Someone removes the IO container while the service still believes the host is ready.
	env.fe.mu.Lock()
	for id, fc := range env.fe.containers {
		if fc.name == IOContainerName {
			delete(env.fe.containers, id)
		}
	}
	env.fe.mu.Unlock()
	_, err := env.svc.Get(context.Background(), testHostID, "a")
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != CodeHostNotPrepared {
		t.Fatalf("Get = %v, want %s", err, CodeHostNotPrepared)
	}
	if h, ok := coded.Fields["helpers"].(HelperStatus); !ok || h.IOContainer != "missing" || h.Prepared {
		t.Fatalf("fields = %+v, want the IO container reported missing", coded.Fields)
	}
}

func TestCreateWritesTheFolderWithTheRightModesAndOwner(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	if _, err := env.svc.List(ctx, testHostID, false); err != nil { // fills the scan cache
		t.Fatal(err)
	}
	envText := "TOKEN=abc\n"
	detail, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "blog", Compose: stacksTestCompose, Env: &envText})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != StatusDraft || detail.ComposeFile != "compose.yaml" || !detail.HasEnv || detail.Compose.Content != stacksTestCompose || detail.Env.Content != envText {
		t.Fatalf("detail = %+v", detail)
	}
	if got, _ := env.fe.file("/opt/stacks/blog/compose.yaml"); got != stacksTestCompose {
		t.Fatalf("compose on the host = %q", got)
	}
	if env.fe.mode("/opt/stacks/blog/compose.yaml") != 0o644 || env.fe.mode("/opt/stacks/blog/.env") != 0o600 || env.fe.mode("/opt/stacks/blog/") != 0o755 {
		t.Fatalf("modes: compose %o env %o dir %o, want 644, 600 and 755",
			env.fe.mode("/opt/stacks/blog/compose.yaml"), env.fe.mode("/opt/stacks/blog/.env"), env.fe.mode("/opt/stacks/blog/"))
	}
	env.fe.mu.Lock()
	queries := append([]string(nil), env.fe.putQueries...)
	env.fe.mu.Unlock()
	if len(queries) != 1 || queries[0] != "path=%2Fopt%2Fstacks&noOverwriteDirNonDir=true" {
		t.Fatalf("PUT queries = %v, want one extraction into the stacks directory with noOverwriteDirNonDir", queries)
	}
	// The write invalidated the scan: the next List sees the new stack at once.
	listing, _ := env.svc.List(ctx, testHostID, false)
	if listing.Cached || len(listing.Stacks) != 1 || listing.Stacks[0].Name != "blog" {
		t.Fatalf("listing after Create = %+v", listing)
	}

	// No .env when none is given.
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "plain", Compose: stacksTestCompose}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.fe.file("/opt/stacks/plain/.env"); ok {
		t.Fatal("Create without Env must not write a .env")
	}
}

func TestCreateRefusesAnExistingStackAndKeepsAnExistingFolder(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	env.fe.putFile("/opt/stacks/taken/docker-compose.yaml", "services: {}\n")
	_, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "taken", Compose: stacksTestCompose})
	if !errors.Is(err, ErrStackExists) || CodeOf(err) != CodeStackExists {
		t.Fatalf("Create over an existing stack = %v, want %s", err, CodeStackExists)
	}
	if got, _ := env.fe.file("/opt/stacks/taken/docker-compose.yaml"); got != "services: {}\n" {
		t.Fatal("an existing stack must not be touched")
	}
	if _, ok := env.fe.file("/opt/stacks/taken/compose.yaml"); ok {
		t.Fatal("no second compose file may appear next to the existing one")
	}

	// A folder with data but no compose file is reused, and its own entry stays out of the archive.
	env.fe.putFile("/opt/stacks/data-only/data/db.sqlite", "bytes")
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "data-only", Compose: stacksTestCompose}); err != nil {
		t.Fatalf("Create in a folder without a compose file: %v", err)
	}
	if got, _ := env.fe.file("/opt/stacks/data-only/data/db.sqlite"); got != "bytes" {
		t.Fatal("existing data must survive")
	}
	env.fe.mu.Lock()
	last := env.fe.putQueries[len(env.fe.putQueries)-1]
	env.fe.mu.Unlock()
	if last != "path=%2Fopt%2Fstacks%2Fdata-only&noOverwriteDirNonDir=true" {
		t.Fatalf("PUT query = %q, want the files extracted inside the existing folder so its owner and mode stay", last)
	}
	if env.fe.mode("/opt/stacks/data-only/") != 0 {
		t.Fatal("an existing folder must not get a directory entry")
	}
}

func TestCreateValidatesBeforeTouchingTheHost(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	big := strings.Repeat("x", MaxFileBytes+1)
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "Bad Name", Compose: "x"}); CodeOf(err) != CodeStackNameInvalid {
		t.Fatalf("bad name = %v", err)
	}
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "ok", Compose: big}); CodeOf(err) != CodeFileTooLarge {
		t.Fatalf("oversized compose = %v", err)
	}
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "ok", Compose: "x", Env: &big}); CodeOf(err) != CodeFileTooLarge {
		t.Fatalf("oversized env = %v", err)
	}
	exact := strings.Repeat("y", MaxFileBytes)
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "exact", Compose: exact}); err != nil {
		t.Fatalf("a file of exactly %d bytes must be accepted: %v", MaxFileBytes, err)
	}
	if env.fe.requestCount("PUT /containers/") != 1 {
		t.Fatalf("only the valid create may reach the host: %v", env.fe.requests())
	}
	unprepared := newTestEnv(t, false)
	if _, err := unprepared.svc.Create(ctx, testHostID, CreateInput{Name: "ok", Compose: "x"}); CodeOf(err) != CodeHostNotPrepared {
		t.Fatalf("Create on an unprepared host = %v, want %s", err, CodeHostNotPrepared)
	}
}

func TestWriteFilesChecksRevisionsBeforeWritingAnything(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	env.fe.putFile("/opt/stacks/app/docker-compose.yml", "v1\n")
	env.fe.putFile("/opt/stacks/app/.env", "A=1\n")
	revCompose, revEnv := Revision([]byte("v1\n")), Revision([]byte("A=1\n"))

	// A matching revision writes the file under its existing name.
	revs, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Compose: &FileWrite{Content: "v2\n", Revision: revCompose}})
	if err != nil {
		t.Fatal(err)
	}
	if revs.Compose != Revision([]byte("v2\n")) || revs.Env != "" {
		t.Fatalf("revisions = %+v", revs)
	}
	if got, _ := env.fe.file("/opt/stacks/app/docker-compose.yml"); got != "v2\n" {
		t.Fatalf("compose = %q", got)
	}
	if _, ok := env.fe.file("/opt/stacks/app/compose.yaml"); ok {
		t.Fatal("the compose file keeps its name")
	}
	if env.fe.mode("/opt/stacks/app/docker-compose.yml") != 0o644 {
		t.Fatalf("mode = %o", env.fe.mode("/opt/stacks/app/docker-compose.yml"))
	}

	// The old revision is now stale: a conflict that carries what is on the host.
	_, err = env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Compose: &FileWrite{Content: "mine\n", Revision: revCompose}})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.File != "compose" || conflict.Current.Content != "v2\n" ||
		conflict.Current.Revision != Revision([]byte("v2\n")) || !conflict.Current.Exists || conflict.Current.Name != "docker-compose.yml" {
		t.Fatalf("stale compose write = %v, want a ConflictError with the host's version", err)
	}
	if CodeOf(err) != CodeRevisionConflict {
		t.Fatalf("code = %s", CodeOf(err))
	}

	// One stale file stops the whole request: the other file is not written either.
	_, err = env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{
		Compose: &FileWrite{Content: "v3\n", Revision: Revision([]byte("v2\n"))},
		Env:     &FileWrite{Content: "A=changed\n", Revision: "stale"},
	})
	if !errors.As(err, &conflict) || conflict.File != "env" || conflict.Current.Content != "A=1\n" || conflict.Current.Revision != revEnv {
		t.Fatalf("stale env write = %v", err)
	}
	if got, _ := env.fe.file("/opt/stacks/app/docker-compose.yml"); got != "v2\n" {
		t.Fatalf("compose = %q, nothing may be written when a file conflicts", got)
	}

	// Both files at once, with the current revisions; .env keeps mode 600.
	revs, err = env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{
		Compose: &FileWrite{Content: "v3\n", Revision: Revision([]byte("v2\n"))},
		Env:     &FileWrite{Content: "A=2\n", Revision: revEnv},
	})
	if err != nil || revs.Compose != Revision([]byte("v3\n")) || revs.Env != Revision([]byte("A=2\n")) {
		t.Fatalf("two-file write = %+v, %v", revs, err)
	}
	if env.fe.mode("/opt/stacks/app/.env") != 0o600 {
		t.Fatalf(".env mode = %o", env.fe.mode("/opt/stacks/app/.env"))
	}
	// Writing into an existing stack never sends a directory entry (owner and mode stay).
	env.fe.mu.Lock()
	last := env.fe.putQueries[len(env.fe.putQueries)-1]
	env.fe.mu.Unlock()
	if last != "path=%2Fopt%2Fstacks%2Fapp&noOverwriteDirNonDir=true" {
		t.Fatalf("PUT query = %q", last)
	}
}

func TestWriteFilesCreatesAMissingEnvOnlyWithAnEmptyRevision(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	env.fe.putFile("/opt/stacks/app/compose.yaml", "v1\n")

	_, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Env: &FileWrite{Content: "A=1\n", Revision: "some-revision"}})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.File != "env" || conflict.Current.Exists || conflict.Current.Revision != "" {
		t.Fatalf("env write based on a revision of a file that does not exist = %v", err)
	}
	if _, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Env: &FileWrite{Content: "A=1\n", Revision: ""}}); err != nil {
		t.Fatalf("creating .env: %v", err)
	}
	if got, _ := env.fe.file("/opt/stacks/app/.env"); got != "A=1\n" {
		t.Fatalf(".env = %q", got)
	}
	// Now "must not exist yet" is wrong.
	if _, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Env: &FileWrite{Content: "A=2\n", Revision: ""}}); !errors.As(err, &conflict) {
		t.Fatalf("a second creation = %v, want a conflict", err)
	}
	// An empty .env is a file with a revision of its own.
	revs, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Env: &FileWrite{Content: "", Revision: Revision([]byte("A=1\n"))}})
	if err != nil || revs.Env != Revision(nil) {
		t.Fatalf("emptying .env = %+v, %v", revs, err)
	}
}

func TestWriteFilesRefusals(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	env.fe.putFile("/opt/stacks/app/compose.yaml", "v1\n")
	rev := Revision([]byte("v1\n"))
	ok := &FileWrite{Content: "v2\n", Revision: rev}

	if _, err := env.svc.WriteFiles(ctx, testHostID, "ghost", FilesInput{Compose: ok}); CodeOf(err) != CodeStackNotFound {
		t.Fatalf("write to a missing stack = %v", err)
	}
	if _, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{}); CodeOf(err) != CodeInvalidOperation {
		t.Fatalf("write without a file = %v", err)
	}
	if _, err := env.svc.WriteFiles(ctx, testHostID, "A/B", FilesInput{Compose: ok}); CodeOf(err) != CodeStackNameInvalid {
		t.Fatalf("write with a bad name = %v", err)
	}
	if _, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Compose: &FileWrite{Content: strings.Repeat("x", MaxFileBytes+1), Revision: rev}}); CodeOf(err) != CodeFileTooLarge {
		t.Fatalf("oversized write = %v", err)
	}

	registerRun(t, env.svc, Operation{ID: "op_dededededededede", HostID: testHostID, Stack: "app", Kind: KindDelete, State: StateRunning})
	_, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Compose: ok})
	var busy *BusyError
	if !errors.As(err, &busy) || busy.OperationID != "op_dededededededede" || busy.Kind != KindDelete {
		t.Fatalf("write while a delete runs = %v, want a BusyError naming the delete", err)
	}
	if _, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "app", Compose: "x"}); !errors.As(err, &busy) {
		t.Fatalf("Create while a delete runs = %v, want a BusyError", err)
	}
	if got, _ := env.fe.file("/opt/stacks/app/compose.yaml"); got != "v1\n" {
		t.Fatal("nothing may be written while a delete runs")
	}
}

func TestOnlyOneWriterPassesThePerStackLockAtATime(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	env.fe.putFile("/opt/stacks/app/compose.yaml", "v0\n")
	rev := Revision([]byte("v0\n"))

	// Ten writers start from the same revision: exactly one may win, the others conflict.
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := env.svc.WriteFiles(ctx, testHostID, "app", FilesInput{Compose: &FileWrite{Content: "winner\n", Revision: rev}})
			results <- err
		}()
	}
	wins, conflicts := 0, 0
	for i := 0; i < 10; i++ {
		switch err := <-results; {
		case err == nil:
			wins++
		case CodeOf(err) == CodeRevisionConflict:
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || conflicts != 9 {
		t.Fatalf("wins = %d conflicts = %d, want exactly one writer to succeed", wins, conflicts)
	}
}

func TestCreateNeverOverwritesAnExistingEnvOfAFolderWithoutAComposeFile(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	env.fe.putFile("/opt/stacks/leftover/.env", "DB_PASSWORD=old-secret\n")

	newEnv := "NEW=1\n"
	_, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "leftover", Compose: stacksTestCompose, Env: &newEnv})
	if CodeOf(err) != CodeStackExists || !strings.Contains(err.Error(), ".env") {
		t.Fatalf("Create with an .env over an existing one = %v, want %s naming the .env", err, CodeStackExists)
	}
	if got, _ := env.fe.file("/opt/stacks/leftover/.env"); got != "DB_PASSWORD=old-secret\n" {
		t.Fatalf("the existing .env was overwritten: %q", got)
	}
	if _, ok := env.fe.file("/opt/stacks/leftover/compose.yaml"); ok {
		t.Fatal("a refused create must write nothing")
	}

	// Without an .env in the request the existing one is kept as it is.
	detail, err := env.svc.Create(ctx, testHostID, CreateInput{Name: "leftover", Compose: stacksTestCompose})
	if err != nil {
		t.Fatalf("Create without an .env next to an existing one: %v", err)
	}
	if got, _ := env.fe.file("/opt/stacks/leftover/.env"); got != "DB_PASSWORD=old-secret\n" || detail.Env.Content != got {
		t.Fatalf("the existing .env must stay untouched, got %q / %+v", got, detail.Env)
	}
}
