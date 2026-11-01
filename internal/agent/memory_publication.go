package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func publishModelMemory(ctx context.Context, cfg *config.Config, stm *memory.SQLiteMemory, ltm memory.VectorDB, concept, content string, details memory.MemoryMetaUpdate, expiry *time.Time) (stored memory.AutomaticMemoryStoreResult, err error) {
	validate := func() error {
		if !runtimeMemoryWriteAllowed(cfg, "memory", details.SourceSessionID) {
			return fmt.Errorf("%w: memory is not writable", memory.ErrAutomaticMemorySkipped)
		}
		return ctx.Err()
	}
	if err := validate(); err != nil {
		return stored, err
	}
	if stm == nil {
		return stored, fmt.Errorf("memory metadata store unavailable")
	}
	ctx, release, err := stm.RegisterSessionRun(ctx, details.SourceSessionID)
	if err != nil {
		return stored, err
	}
	defer release()
	err = stm.WithSessionWrite(ctx, details.SourceSessionID, func() error {
		var writeErr error
		stored, writeErr = memory.StoreAutomaticMemoryDocumentWithOptionsContext(ctx, stm, ltm, concept, content, details, memory.AutomaticMemoryWriteOptions{ExpiresAt: expiry, Validate: validate})
		return writeErr
	})
	return stored, err
}

func publishProactiveHistorySummary(ctx context.Context, runCfg RunConfig, summary string) error {
	if !runtimeMemoryWriteAllowed(runCfg.Config, "memory", runCfg.SessionID) || strings.TrimSpace(summary) == "" {
		return nil
	}
	if reason, _ := screenGeneratedSummary(summary); reason != "" {
		return fmt.Errorf("%w: summary rejected (%s)", memory.ErrAutomaticMemorySkipped, reason)
	}
	_, err := publishModelMemory(ctx, runCfg.Config, runCfg.ShortTermMem, runCfg.LongTermMem,
		fmt.Sprintf("Conversation summary %s", time.Now().UTC().Format(time.RFC3339Nano)), summary,
		memory.MemoryMetaUpdate{SourceSessionID: runCfg.SessionID, SourceRole: "user_and_assistant", SourceType: "chat", VerificationStatus: "unverified", ExtractionConfidence: .6, SourceReliability: .6}, nil)
	return err
}

// RecoverMemoryPublications finishes only new, journaled writes under current permissions.
func RecoverMemoryPublications(ctx context.Context, cfg *config.Config, stm *memory.SQLiteMemory, ltm memory.VectorDB) error {
	if stm == nil || ltm == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return stm.RecoverAutomaticMemoryPublicationsContext(ctx, ltm, func() error {
		if !runtimeMemoryWriteAllowed(cfg, "memory", "") {
			return fmt.Errorf("memory publication recovery is not authorized")
		}
		return ctx.Err()
	})
}
