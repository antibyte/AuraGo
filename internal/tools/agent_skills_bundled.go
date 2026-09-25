package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RegisterBundledAgentSkill verifies a single-file package against bytes from
// the calling binary, not a model/user payload. This internal startup API is
// never advertised as a tool. Other packages retain the normal scanner path.
func (m *AgentSkillManager) RegisterBundledAgentSkill(ctx context.Context, name string, markdown []byte) (*AgentSkillRegistryEntry, error) {
	return m.RegisterBundledAgentSkillFor(ctx, "game-maker", name, markdown)
}

// RegisterBundledAgentSkillFor is restricted to trusted built-in startup code.
func (m *AgentSkillManager) RegisterBundledAgentSkillFor(ctx context.Context, owner, name string, markdown []byte) (*AgentSkillRegistryEntry, error) {
	if owner != "game-maker" && owner != "detective" && owner != "newspaper" {
		return nil, fmt.Errorf("unknown bundled skill owner")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !agentSkillNamePattern.MatchString(name) || len(markdown) == 0 {
		return nil, fmt.Errorf("invalid bundled agent skill")
	}
	m.qualityMutationMu.Lock()
	defer m.qualityMutationMu.Unlock()
	dir := filepath.Join(m.agentSkillsDir, name)
	if owner == "newspaper" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, "SKILL.md")
		existingBytes, readErr := os.ReadFile(path)
		if errors.Is(readErr, os.ErrNotExist) {
			if err := os.WriteFile(path, markdown, 0600); err != nil {
				return nil, err
			}
		} else if readErr != nil {
			return nil, readErr
		} else if !bytes.Equal(existingBytes, markdown) {
			// Replace only a previously verified system package. A local edit or
			// unexpected file keeps the skill unavailable for manual inspection.
			previous, lookupErr := m.GetAgentSkillByName(name)
			files, filesErr := os.ReadDir(dir)
			oldHash := sha256.New()
			oldHash.Write([]byte("SKILL.md\x00"))
			oldHash.Write(existingBytes)
			oldHash.Write([]byte{0})
			if lookupErr != nil || filesErr != nil || previous == nil || previous.Origin != OriginSystem || previous.SecurityStatus != SecurityClean || !previous.Enabled || !agentSkillSamePath(previous.Directory, dir) || len(files) != 1 || files[0].Name() != "SKILL.md" || previous.PackageHash != hex.EncodeToString(oldHash.Sum(nil)) {
				return nil, fmt.Errorf("installed Newspaper skill differs from its verified system package")
			}
			if err := os.WriteFile(path, markdown, 0600); err != nil {
				return nil, err
			}
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// Reject even unrecognized extra files, which the generic parser ignores.
	if len(files) != 1 || files[0].Name() != "SKILL.md" || !files[0].Type().IsRegular() {
		return nil, fmt.Errorf("bundled agent skill package contains unexpected files")
	}
	pkg, err := ParseAgentSkillPackage(dir)
	if err != nil {
		return nil, err
	}
	// Same filename/NUL/content/NUL encoding as enumerateAgentSkillFiles.
	hash := sha256.New()
	hash.Write([]byte("SKILL.md\x00"))
	hash.Write(markdown)
	hash.Write([]byte{0})
	if pkg.Name != name || pkg.PackageHash != hex.EncodeToString(hash.Sum(nil)) {
		return nil, fmt.Errorf("bundled agent skill package does not match this binary")
	}
	existing, err := m.GetAgentSkillByName(name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil && existing.SecurityStatus == SecurityDangerous {
		return nil, fmt.Errorf("bundled agent skill is explicitly blocked")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if existing != nil && existing.PackageHash == pkg.PackageHash && existing.Enabled && existing.SecurityStatus == SecurityClean && existing.Origin == OriginSystem && agentSkillSamePath(existing.Directory, pkg.Directory) {
		return existing, nil
	}
	entry, err := m.upsertAgentSkillPackage(pkg, "system:"+owner, nil, SecurityClean, true, false)
	if err != nil {
		return nil, err
	}
	if _, err := m.db.ExecContext(ctx, "UPDATE agent_skills_registry SET origin = ? WHERE id = ?", string(OriginSystem), entry.ID); err != nil {
		return nil, err
	}
	m.audit(entry.ID, name, "verify_bundle", "system:"+owner, "complete package matches binary; hash="+pkg.PackageHash)
	return m.GetAgentSkill(entry.ID)
}
