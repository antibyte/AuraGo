package upkeep

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Result struct {
	Sequence int64 `json:"sequence"`
	Failures int   `json:"consecutive_failures"`
	Success  bool  `json:"success"`
}

// Only sanitized counters live in data so a service user can read the outcome
// of a root-run updater without access to credential-bearing private backups.
const ResultFile = "update_cleanup_status.json"

func ReadResult(root string) (Result, error) {
	var r Result
	err := readJSON(filepath.Join(root, "data", ResultFile), &r)
	return r, err
}

func saveResult(root string, success bool) error {
	r, err := ReadResult(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	r.Sequence++
	r.Success = success
	if success {
		r.Failures = 0
	} else {
		r.Failures++
	}
	name := filepath.Join(root, "data", ResultFile)
	if err := noLinks(name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0750); err != nil {
		return err
	}
	if err := writeJSON(name, r); err != nil {
		return err
	}
	return os.Chmod(name, 0644)
}

func health(ctx context.Context, root, bin string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, bin, "--config", filepath.Join(root, "config.yaml"), "--healthcheck", "--healthcheck-timeout", "15s")
	c.Dir = root
	// Health output is intentionally not copied into the retention ledger/log.
	if err := c.Run(); err != nil {
		return fmt.Errorf("health check failed")
	}
	return nil
}

// RunCLI runs before config/vault/service initialization in the portable binary.
func RunCLI(args []string, out, stderr io.Writer) int {
	f := flag.NewFlagSet("update-maintenance", flag.ContinueOnError)
	f.SetOutput(stderr)
	root := f.String("root", "", "installation directory (required)")
	apply := f.Bool("apply", false, "apply the cleanup; default only previews")
	legacy := f.Bool("adopt-legacy", false, "include provably installation-owned legacy update artifacts")
	checkPending := f.Bool("check-pending", false, "validate transaction state before an update")
	resolve := f.String("resolve", "", "resolve this transaction after recovery")
	outcome := f.String("outcome", "", "confirmed or rolled_back (with --resolve)")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *root == "" || f.NArg() != 0 {
		fmt.Fprintln(stderr, "--root is required; no positional arguments accepted")
		return 2
	}
	absolute, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	*root = filepath.Clean(absolute)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if *checkPending {
		r, err := installation(*root)
		if err == nil {
			var unlock func()
			unlock, err = lock(r)
			if err == nil {
				defer unlock()
				var records []backup
				records, err = loadTransactions(r)
				if err == nil {
					for _, b := range records {
						if b.Status == "pending" || b.Status == "uncertain" || (!b.BackupComplete && b.Status != "rolled_back") {
							err = fmt.Errorf("unresolved update: %s", b.ID)
							break
						}
					}
				}
			}
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if *resolve != "" {
		if !*apply || (*outcome != "confirmed" && *outcome != "rolled_back") {
			fmt.Fprintln(stderr, "resolution requires --apply and --outcome confirmed|rolled_back")
			return 2
		}
		if err := Resolve(ctx, *root, *resolve, *outcome); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(out, "Transaction resolved after asset and readiness verification.")
		return 0
	}
	if *outcome != "" {
		fmt.Fprintln(stderr, "--outcome requires --resolve")
		return 2
	}
	r, err := Cleanup(ctx, Options{Root: *root, Apply: *apply, AdoptLegacy: *legacy, Health: func(ctx context.Context, bin string) error { return health(ctx, *root, bin) }})
	if err != nil {
		r.Error = err.Error()
	}
	if e := json.NewEncoder(out).Encode(r); e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func Resolve(ctx context.Context, root, id, outcome string) error {
	root, err := installation(root)
	if err != nil {
		return err
	}
	if !transactionPattern.MatchString(id) || (outcome != "confirmed" && outcome != "rolled_back") {
		return fmt.Errorf("invalid resolution")
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	backups, err := loadTransactions(root)
	if err != nil {
		return err
	}
	for _, b := range backups {
		if b.ID != id {
			continue
		}
		if !b.BackupComplete && outcome != "rolled_back" {
			return fmt.Errorf("incomplete backup cannot become a rollback generation")
		}
		bin, err := installedBinary(root)
		if err != nil {
			return err
		}
		pin, err := PinFromBinary(bin, filepath.Join(root, "assets", "web"))
		if err != nil {
			return err
		}
		expected := b.NewAsset
		if outcome == "confirmed" {
			h, e := digestFile(bin)
			if e != nil || !hashPattern.MatchString(b.NewVersion) || h != b.NewVersion {
				return fmt.Errorf("installed binary does not match staged version")
			}
		}
		if outcome == "rolled_back" {
			expected = b.PreviousAsset
			h, e := digestFile(bin)
			if e != nil || h != b.PreviousVersion {
				return fmt.Errorf("restored binary does not match backup")
			}
		}
		if pin != expected || expected == "" {
			return fmt.Errorf("installed version differs from selected outcome")
		}
		if err := verifyAssets(filepath.Join(root, "assets", "web"), pin); err != nil {
			return err
		}
		if err := health(ctx, root, bin); err != nil {
			return err
		}
		// Require tsnet readiness when the saved state existed; recovery must not
		// discard a pending network-state backup after only a core health check.
		if _, e := os.Stat(filepath.Join(b.path, "tsnet-state")); e == nil {
			c := exec.CommandContext(ctx, bin, "--config", filepath.Join(root, "config.yaml"), "--healthcheck", "--healthcheck-timeout", "30s", "--healthcheck-require-tsnet")
			c.Dir = root
			if e := c.Run(); e != nil {
				return fmt.Errorf("tsnet recovery not verified")
			}
		}
		b.Status = outcome
		return writeJSON(filepath.Join(b.path, "manifest.json"), b.Transaction)
	}
	return fmt.Errorf("transaction not found")
}
