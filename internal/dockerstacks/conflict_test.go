package dockerstacks

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// foreignProject puts a container of a compose project that runs from another
// folder of the host.
func foreignProject(env *testEnv, project, dir string) {
	fc := env.fe.addProjectContainer(project+"-db-1", project, "db", "running", "Up 1 minute", "", time.Now().Unix())
	env.fe.setWorkingDir(fc, dir)
}

func workingDirOfError(err error) string {
	var coded *Error
	if !errors.As(err, &coded) {
		return ""
	}
	dir, _ := coded.Fields["working_dir"].(string)
	return dir
}

func TestStartRefusesEveryKindForAStackWhoseProjectNameAnotherFolderUses(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	foreignProject(env, "web", "/home/andi/web")

	for _, kind := range StackOperationKinds {
		_, err := env.svc.Start(context.Background(), testHostID, "web", kind, OpOptions{Volumes: true})
		if CodeOf(err) != CodeProjectConflict || workingDirOfError(err) != "/home/andi/web" || !strings.Contains(err.Error(), "/home/andi/web") {
			t.Fatalf("%s = %v, want project_conflict naming the other folder", kind, err)
		}
	}
	if len(env.fe.containersWithRole(RoleRunner)) != 0 || env.fe.requestCount("POST /containers/create") != 0 {
		t.Fatalf("a refused start must create no runner: %v", env.fe.requests())
	}
	if env.svc.activeRun(testHostID, "web") != nil {
		t.Fatal("a refused start must not take the stack's lock")
	}

	// The same project name from the stack's own folder is the stack itself.
	own := newTestEnv(t, true)
	seedComposeStack(own, "web")
	ownContainer := own.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 1 minute", "", time.Now().Unix())
	own.fe.setWorkingDir(ownContainer, "/opt/stacks/web/") // a trailing slash is the same folder
	if _, err := own.svc.Start(context.Background(), testHostID, "web", KindDown, OpOptions{}); err != nil {
		t.Fatalf("Start for a project of the stack's own folder: %v", err)
	}
}

func TestAnOperationChecksTheProjectAgainBeforeItCreatesItsRunner(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	foreignProject(env, "web", "/home/andi/web")
	// Start could not see the project (the host did not answer it), the operation does.
	run := registerRun(t, env.svc, Operation{ID: "op_8888888888888888", HostID: testHostID, Stack: "web", Kind: KindDelete})
	out := env.svc.drive(run)
	if out.state != StateFailed || out.code != CodeProjectConflict || !strings.Contains(out.message, "/home/andi/web") {
		t.Fatalf("outcome = %+v, want a failure with the code %s", out, CodeProjectConflict)
	}
	if len(env.fe.containersWithRole(RoleRunner)) != 0 {
		t.Fatal("no runner may be created for a foreign project")
	}
}

func TestCreateRefusesAStackNameThatAnotherFoldersProjectUses(t *testing.T) {
	env := newTestEnv(t, true)
	foreignProject(env, "blog", "/srv/blog")
	_, err := env.svc.Create(context.Background(), testHostID, CreateInput{Name: "blog", Compose: stacksTestCompose})
	if CodeOf(err) != CodeProjectConflict || workingDirOfError(err) != "/srv/blog" {
		t.Fatalf("Create = %v, want project_conflict with working_dir /srv/blog", err)
	}
	if _, ok := env.fe.file("/opt/stacks/blog/compose.yaml"); ok {
		t.Fatal("a refused create must write nothing")
	}
}

func TestListingKeepsAForeignProjectOfAStacksNameUnderExternal(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	seedComposeStack(env, "blog")
	// web: one container from its own folder and one from another folder.
	env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 1 minute", "", time.Now().Unix())
	foreignProject(env, "web", "/home/andi/web")
	// blog: a draft stack; a project of that name runs from elsewhere.
	foreignProject(env, "blog", "/srv/blog")

	listing, err := env.svc.List(context.Background(), testHostID, true)
	if err != nil {
		t.Fatal(err)
	}
	stacks := map[string]StackSummary{}
	for _, st := range listing.Stacks {
		stacks[st.Name] = st
	}
	if web := stacks["web"]; web.Services.Total != 1 || web.Status != StatusRunning {
		t.Fatalf("web = %+v, want only its own container", web)
	}
	if blog := stacks["blog"]; blog.Status != StatusDraft || blog.Services.Total != 0 {
		t.Fatalf("blog = %+v, want a draft: the other folder's containers are not its own", blog)
	}
	external := map[string]ProjectSummary{}
	for _, p := range listing.External {
		external[p.Name] = p
	}
	if len(listing.External) != 2 || external["web"].WorkingDir != "/home/andi/web" || external["web"].Services.Total != 1 ||
		external["blog"].WorkingDir != "/srv/blog" {
		t.Fatalf("external = %+v, want both foreign projects with their folders", listing.External)
	}
}
