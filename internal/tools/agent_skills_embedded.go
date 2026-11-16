package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/ulikunitz/xz"
)

//go:embed bundled_agent_skills/modern-web-guidance.zip.xz
var modernWebGuidanceArchive []byte

const embeddedSkillVerificationPrefix = "complete package matches binary; hash="

// InstallModernWebGuidance installs only the reviewed package embedded in this
// binary. It is a startup API, never an agent tool or an external import path.
func (m *AgentSkillManager) InstallModernWebGuidance(ctx context.Context) (*AgentSkillRegistryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	compressed, err := xz.NewReader(bytes.NewReader(modernWebGuidanceArchive))
	if err != nil {
		return nil, fmt.Errorf("open bundled Modern Web Guidance: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(compressed, maxAgentSkillPackageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompress bundled Modern Web Guidance: %w", err)
	}
	if len(data) > maxAgentSkillPackageBytes {
		return nil, fmt.Errorf("bundled Modern Web Guidance exceeds package limit")
	}
	bundle, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open bundled Modern Web Guidance: %w", err)
	}
	return m.installEmbeddedAgentSkill(ctx, "modern-web-guidance", bundle)
}

func (m *AgentSkillManager) installEmbeddedAgentSkill(ctx context.Context, name string, bundle fs.FS) (*AgentSkillRegistryEntry, error) {
	m.qualityMutationMu.Lock()
	defer m.qualityMutationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !agentSkillNamePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid bundled skill name")
	}
	expectedHash, err := embeddedAgentSkillHash(bundle)
	if err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(filepath.Join(m.agentSkillsDir, name))
	if err != nil {
		return nil, err
	}
	existing, err := m.GetAgentSkillByName(name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil && (existing.Origin != OriginSystem || existing.SecurityStatus == SecurityDangerous || !agentSkillSamePath(existing.Directory, dir)) {
		return nil, fmt.Errorf("bundled skill conflicts with an existing or blocked package")
	}
	enabled := existing == nil || existing.Enabled
	var current *AgentSkillPackage
	if info, statErr := os.Lstat(dir); statErr == nil {
		var completeHash string
		if !info.IsDir() {
			err = fmt.Errorf("bundled skill path must be a plain directory")
		} else {
			completeHash, err = embeddedAgentSkillHash(os.DirFS(dir))
		}
		if err == nil {
			current, err = ParseAgentSkillPackage(dir)
		}
		if err == nil {
			if completeHash != current.PackageHash || (completeHash != expectedHash && (existing == nil || existing.SecurityStatus != SecurityClean || completeHash != existing.PackageHash)) {
				err = fmt.Errorf("installed package differs from its verified bundle")
			}
		}
		if err == nil && completeHash != expectedHash {
			// A later user rescan must not turn local edits into an upgrade source.
			var verifiedBundle bool
			err = m.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agent_skill_audit_log
				WHERE skill_id = ? AND action = 'verify_bundle' AND actor = 'system:bundled' AND details = ?)`,
				existing.ID, embeddedSkillVerificationPrefix+completeHash).Scan(&verifiedBundle)
			if err == nil && !verifiedBundle {
				err = fmt.Errorf("installed package is not a previously verified binary bundle")
			}
		}
		if err != nil {
			if existing != nil {
				_, disableErr := m.db.ExecContext(ctx, "UPDATE agent_skills_registry SET enabled = 0, security_status = ? WHERE id = ?", string(SecurityPending), existing.ID)
				err = errors.Join(err, disableErr)
			}
			return nil, fmt.Errorf("verify installed bundled skill: %w", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	if current != nil && current.PackageHash == expectedHash {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if existing != nil && existing.PackageHash == expectedHash && existing.SecurityStatus == SecurityClean {
			return existing, nil
		}
		entry, err := m.upsertAgentSkillPackage(current, "system:bundled", nil, SecurityClean, enabled, false)
		if err == nil {
			m.audit(entry.ID, name, "verify_bundle", "system:bundled", embeddedSkillVerificationPrefix+expectedHash)
		}
		return entry, err
	}
	if err := os.MkdirAll(m.agentSkillsDir, 0o750); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(m.agentSkillsDir, ".bundled-*")
	if err != nil {
		return nil, err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()
	stagedDir := filepath.Join(stage, name)
	if err := os.CopyFS(stagedDir, bundle); err != nil {
		return nil, fmt.Errorf("stage bundled skill: %w", err)
	}
	staged, err := ParseAgentSkillPackage(stagedDir)
	if err != nil {
		return nil, err
	}
	if staged.Name != name || staged.PackageHash != expectedHash {
		return nil, fmt.Errorf("staged skill differs from the embedded package")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	backup := filepath.Join(stage, "previous")
	if current != nil {
		if err := os.Rename(dir, backup); err != nil {
			return nil, err
		}
	}
	published := false
	rollback := func(cause error) (*AgentSkillRegistryEntry, error) {
		if published {
			if err := os.RemoveAll(dir); err != nil {
				keepStage = true
				return nil, errors.Join(cause, fmt.Errorf("retain bundle recovery at %s: %w", stage, err))
			}
		}
		if current != nil {
			if err := os.Rename(backup, dir); err != nil {
				keepStage = true
				return nil, errors.Join(cause, fmt.Errorf("restore bundle from %s: %w", backup, err))
			}
		}
		return nil, cause
	}
	if err := os.Rename(stagedDir, dir); err != nil {
		return rollback(err)
	}
	published = true
	installed, err := ParseAgentSkillPackage(dir)
	if err != nil {
		return rollback(err)
	}
	if installed.PackageHash != expectedHash {
		return rollback(fmt.Errorf("installed skill differs from the embedded package"))
	}
	if err := ctx.Err(); err != nil {
		return rollback(err)
	}
	entry, err := m.upsertAgentSkillPackage(installed, "system:bundled", nil, SecurityClean, enabled, false)
	if err != nil {
		return rollback(err)
	}
	m.audit(entry.ID, name, "verify_bundle", "system:bundled", embeddedSkillVerificationPrefix+expectedHash)
	return entry, nil
}

// Hash every file, including paths ignored by the ordinary package parser, so
// extra content cannot inherit the embedded package's trust during installation.
func embeddedAgentSkillHash(source fs.FS) (string, error) {
	hash := sha256.New()
	directories := map[string]bool{}
	var names []string
	err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if name != "." {
				directories[name] = false
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("bundled skill contains a non-regular file: %s", name)
		}
		names = append(names, name)
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			directories[parent] = true
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	for name, populated := range directories {
		if !populated {
			return "", fmt.Errorf("bundled skill contains an unexpected empty directory: %s", name)
		}
	}
	sort.Strings(names)
	remaining := int64(maxAgentSkillPackageBytes)
	for _, name := range names {
		file, err := source.Open(name)
		if err != nil {
			return "", err
		}
		hash.Write([]byte(name + "\x00"))
		n, readErr := io.Copy(hash, io.LimitReader(file, remaining+1))
		if err := errors.Join(readErr, file.Close()); err != nil {
			return "", err
		}
		remaining -= n
		if remaining < 0 {
			return "", fmt.Errorf("bundled skill exceeds package limit")
		}
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
