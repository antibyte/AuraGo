package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/security"
)

// homepageQuickOrigin serves one immutable, registered project's static output.
// Its address is allocated by the server and is never taken from tool arguments.
type homepageQuickOrigin struct {
	server        *http.Server
	listener      net.Listener
	root          *os.Root
	snapshot      *hereNowPublishSnapshot
	db            *sql.DB
	hostHeader    string
	projectDir    string
	projectID     int64
	buildDir      string
	artifactHash  string
	publicationID string
	closed        sync.Once
	disabled      atomic.Bool
}

func (o *homepageQuickOrigin) Close() {
	if o == nil {
		return
	}
	o.closed.Do(func() {
		o.disabled.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.server.Shutdown(ctx)
		_ = o.server.Close()
		_ = o.root.Close()
		_ = o.db.Close()
		_ = o.snapshot.Close()
	})
}

func newHomepageQuickOrigin(cfg CloudflareTunnelConfig, mode string) (_ *homepageQuickOrigin, err error) {
	if !cfg.Enabled || cfg.ReadOnly || !cfg.HomepageEnabled {
		return nil, fmt.Errorf("quick publication requires enabled, writable Cloudflare and Homepage integrations")
	}
	project, err := NormalizeHomepageProjectIdentity(cfg.QuickProjectDir, false)
	if err != nil || project == "" {
		return nil, fmt.Errorf("quick publication requires an explicit registered project_dir")
	}
	if cfg.HomepageRegistryPath == "" {
		return nil, fmt.Errorf("Homepage registry is not configured")
	}
	registryPath, err := filepath.Abs(cfg.HomepageRegistryPath)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(registryPath); err != nil {
		return nil, fmt.Errorf("Homepage registry unavailable: %w", err)
	}
	registryURLPath := filepath.ToSlash(registryPath)
	if !strings.HasPrefix(registryURLPath, "/") {
		registryURLPath = "/" + registryURLPath
	}
	dsn := url.URL{Scheme: "file", Path: registryURLPath, RawQuery: "mode=rw"}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open Homepage registry: %w", err)
	}
	db.SetMaxOpenConns(1)
	defer func() {
		if err != nil {
			_ = db.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var status string
	var projectID int64
	if err = db.QueryRowContext(ctx, "SELECT id, status FROM homepage_projects WHERE project_dir = ?", project).Scan(&projectID, &status); err != nil || status != "active" {
		return nil, fmt.Errorf("project_dir must identify an active registered Homepage project")
	}
	homepageCfg := HomepageConfig{WorkspacePath: cfg.HomepageWorkspace}
	candidate, err := homepageDetectDeployCandidate(homepageCfg, project, "", "")
	if err != nil {
		return nil, err
	}
	snapshot, err := buildHereNowSnapshotLimited(cfg.HomepageWorkspace, candidate.ContainerSubdir, 64<<20)
	if err != nil {
		return nil, fmt.Errorf("snapshot Homepage publication: %w", err)
	}
	defer func() {
		if err != nil {
			_ = snapshot.Close()
		}
	}()
	var total int64
	files := make(map[string]bool, len(snapshot.files))
	hash := sha256.New()
	for _, file := range snapshot.files {
		total += file.Size
		files[file.Path] = true
		fmt.Fprintf(hash, "%d:%s:%d:%s\n", len(file.Path), file.Path, file.Size, file.Hash)
	}
	if total > 64<<20 || !files["index.html"] {
		return nil, fmt.Errorf("quick publication requires index.html and at most 64 MiB of static files")
	}
	root, err := os.OpenRoot(snapshot.root)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = root.Close()
		}
	}()
	bind := "127.0.0.1:0"
	if mode == "docker" {
		bind = "0.0.0.0:0"
	}
	listener, err := net.Listen("tcp4", bind)
	if err != nil {
		return nil, fmt.Errorf("listen for managed Homepage origin: %w", err)
	}
	origin := &homepageQuickOrigin{listener: listener, root: root, snapshot: snapshot, db: db, projectDir: project, hostHeader: strings.ToLower(rand.Text()) + ".aurago.invalid"}
	origin.projectID, origin.buildDir, origin.artifactHash, origin.publicationID = projectID, candidate.ContainerSubdir, fmt.Sprintf("%x", hash.Sum(nil)), rand.Text()
	security.RegisterSensitive(origin.hostHeader)
	fileServer := http.FileServerFS(root.FS())
	origin.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin.disabled.Load() || r.Host != origin.hostHeader {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		checkCtx, checkCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer checkCancel()
		var current string
		if e := db.QueryRowContext(checkCtx, "SELECT status FROM homepage_projects WHERE project_dir = ?", project).Scan(&current); e != nil || current != "active" {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || strings.HasSuffix(r.URL.Path, "/") {
			name = path.Join(name, "index.html")
		}
		if !files[name] {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fileServer.ServeHTTP(w, r)
	})}
	go func() { _ = origin.server.Serve(listener) }()
	return origin, nil
}

func (o *homepageQuickOrigin) URL(host string) string {
	return "http://" + net.JoinHostPort(host, fmt.Sprint(o.listener.Addr().(*net.TCPAddr).Port))
}

func (o *homepageQuickOrigin) recordPublication(publicURL string) error {
	return RecordHomepageDeployment(o.db, HomepageDeploymentRecord{
		ProjectID: o.projectID, Provider: "cloudflare", ProviderTargetID: o.publicationID,
		ProviderDeployID: o.publicationID, URL: publicURL, BuildDir: o.buildDir,
		ArtifactHash: o.artifactHash, Status: "published_unverified",
		Metadata: map[string]interface{}{"ephemeral": true, "immutable_snapshot": true},
	})
}
