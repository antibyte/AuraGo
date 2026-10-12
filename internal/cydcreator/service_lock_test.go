package cydcreator

import (
	"context"
	"testing"
)

func TestLockingRuleCoversEveryWrite(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Locked")
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/extra.cpp", []byte("// x\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	env.lockForJob(t, p.ID, "ccj_0001")

	userWrites := map[string]func() error{
		"PutLayout": func() error { _, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser); return err },
		"WriteFile": func() error {
			_, err := env.svc.WriteFile(ctx, p.ID, "src/app/a.cpp", []byte("// a\n"), "*", OriginUser)
			return err
		},
		"DeleteFile":    func() error { return env.svc.DeleteFile(ctx, p.ID, "src/app/extra.cpp", "", OriginUser) },
		"PutAsset":      func() error { _, _, err := env.svc.PutAsset(ctx, p.ID, "x.png", pngBytes(t, 8, 8, false)); return err },
		"DeleteAsset":   func() error { _, err := env.svc.DeleteAsset(ctx, p.ID, "a_0001"); return err },
		"RenameProject": func() error { _, err := env.svc.RenameProject(ctx, p.ID, "New"); return err },
		"DeleteProject": func() error { return env.svc.DeleteProject(ctx, p.ID) },
		"StartBuild":    func() error { _, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{}); return err },
		"PutLayoutRestore": func() error {
			_, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginRestore)
			return err
		},
		"PutLayoutOtherJob": func() error {
			_, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, JobOrigin("ccj_0002"))
			return err
		},
	}
	for name, write := range userWrites {
		err := write()
		typed := wantCode(t, err, CodeProjectLocked)
		if typed.Details["job_id"] != "ccj_0001" {
			t.Errorf("%s: details = %v, want job_id ccj_0001", name, typed.Details)
		}
	}

	job := JobOrigin("ccj_0001")
	newVersion, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, job)
	if err != nil || newVersion != version+1 {
		t.Fatalf("job PutLayout = %d, %v", newVersion, err)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/job.cpp", []byte("// job\n"), "", job); err != nil {
		t.Fatalf("job WriteFile: %v", err)
	}
	if err := env.svc.DeleteFile(ctx, p.ID, "src/app/extra.cpp", "", job); err != nil {
		t.Fatalf("job DeleteFile: %v", err)
	}
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{Origin: job})
	if err != nil {
		t.Fatalf("job StartBuild: %v", err)
	}
	if b.Origin != BuildOriginAgent || b.JobID != "ccj_0001" {
		t.Fatalf("job build = %+v, want origin agent and job_id ccj_0001", b)
	}
	waitBuild(t, env.svc, b.ID)
}

func TestWriteMethodsRejectUnknownOrigins(t *testing.T) {
	env := newTestService(t)
	p := env.createProject(t, "Origins")
	_, err := env.svc.WriteFile(context.Background(), p.ID, "src/app/a.cpp", []byte("x"), "*", "admin")
	wantCode(t, err, CodeBadRequest)
	_, err = env.svc.StartBuild(context.Background(), p.ID, BuildOptions{Origin: OriginRestore})
	wantCode(t, err, CodeBadRequest)
}
