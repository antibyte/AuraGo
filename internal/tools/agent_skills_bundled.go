package tools

import (
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
	if owner != "game-maker" && owner != "detective" {
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
