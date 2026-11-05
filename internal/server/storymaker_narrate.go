package server

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"aurago/internal/config"
	"aurago/internal/fileutil"
	"aurago/internal/security"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
)

// storymakerMaxPieceBytes bounds one piece: a stored file read back, because workspace files are not
// trusted, and a provider's answer, before it is decoded.
const storymakerMaxPieceBytes = 256 << 20

// errStorymakerPieceTooLarge refuses a provider answer over storymakerMaxPieceBytes.
var errStorymakerPieceTooLarge = errors.New("it returned more than 256 MiB of audio for one piece")

// storymakerNarratePayload is the payload of a narrate job.
type storymakerNarratePayload struct{ ProjectID string }

// storymakerNarrateRun is the state of one narrate job.
type storymakerNarrateRun struct {
	rt *storymakerRuntime
	id string
	h  *render.Handle
	// verified holds the hashes whose stored file decoded during this job, so each file is decoded once.
	verified map[string]bool
}

// storymakerNarrateTarget is the next piece that needs work.
type storymakerNarrateTarget struct {
	block storymaker.NarrationBlock
	piece storymaker.Piece
	hash  string
	ext   string // the format of the stored file; "" when no audio exists
}

// storymakerTTSConfig builds the speech settings of one voice. It works on a copy of cfg.
func storymakerTTSConfig(cfg *config.Config, v storymaker.Voice) tools.TTSConfig {
	cp := *cfg
	cp.TTS.Provider = v.Provider
	tc := tools.BuildTTSConfig(&cp, tools.TTSRouteTool, v.Language)
	// BuildTTSConfig replaces the language for phonon and sanotts (tts_config.go:62-68).
	tc.Language = v.Language
	tc.NoProviderFallback = true
	switch storymaker.NormalizeProvider(v.Provider) {
	case "phonon":
		tc.Phonon.VoiceID = v.Voice
	case "piper":
		tc.Piper.Voice = v.Voice
	case "supertonic":
		tc.Supertonic.Voice = v.Voice
		tc.Supertonic.ResponseFormat = "wav"
	case "xai":
		tc.XAI.VoiceID = v.Voice
	case "openai":
		tc.OpenAI.VoiceID = v.Voice
	case "stepfun":
		tc.StepFun.VoiceID = v.Voice
	case "elevenlabs":
		tc.ElevenLabs.VoiceID = v.Voice
	case "minimax":
		tc.MiniMax.VoiceID = v.Voice
	case "mistral":
		tc.Mistral.VoiceID = v.Voice
	case "resemble":
		tc.Resemble.VoiceID = v.Voice
	}
	return tc
}

// storymakerSynth makes one provider call. An answer over storymakerMaxPieceBytes is refused here,
// before DecodeAudio can spend memory on its samples. The refusal still returns the bytes: a provider
// that answered has billed, and the caller can tell that from them.
func (s *Server) storymakerSynth(ctx context.Context, cfg *config.Config, v storymaker.Voice, text string) ([]byte, error) {
	synth := tools.TTSSynthesizeInMemoryContext
	if s.storymakerSynthOverride != nil {
		synth = s.storymakerSynthOverride
	}
	data, _, err := synth(ctx, storymakerTTSConfig(cfg, v), text)
	if err == nil && len(data) > storymakerMaxPieceBytes {
		err = errStorymakerPieceTooLarge
	}
	return data, err
}

func (s *Server) storymakerFreeDisk(path string) (int64, error) {
	if s.storymakerFreeDiskOverride != nil {
		return s.storymakerFreeDiskOverride(path)
	}
	return fileutil.FreeDiskBytes(path)
}

// storymakerAudioRun runs a job of the narrate queue. It gets rt directly, because
// s.storymakerRuntime() holds storymakerMu while a replaced runtime waits for its running job. The
// desktop run makes a config change that sets read-only or switches the desktop off cancel a provider
// call already in flight. Every error it returns is a *render.JobError, because a plain
// context.Canceled would surface as render_failed.
func (s *Server) storymakerAudioRun(ctx context.Context, rt *storymakerRuntime, job render.Job, payload any, h *render.Handle) (render.Output, error) {
	ctx, release, err := s.beginDesktopRun(ctx)
	if err != nil {
		return render.Output{}, s.storymakerNarrateStopped()
	}
	defer release()
	if job.Kind != render.KindNarrate {
		return render.Output{}, render.Fail(render.CodeNarrationFailed, "Jobs of kind %q are not available.", job.Kind)
	}
	run := &storymakerNarrateRun{rt: rt, id: job.ProjectID, h: h, verified: map[string]bool{}}
	h.SetPhase(render.PhaseSynthesizing)
	for {
		if ctx.Err() != nil {
			return render.Output{}, s.storymakerNarrateStopped()
		}
		more, err := s.storymakerNarrateStep(ctx, run)
		if err != nil {
			return render.Output{}, err
		}
		if !more {
			return render.Output{}, nil
		}
	}
}

// storymakerNarrateStep does one round: it re-reads everything and then makes at most one provider
// call. It reports whether another round is needed.
func (s *Server) storymakerNarrateStep(ctx context.Context, run *storymakerNarrateRun) (bool, error) {
	// Re-read the story and the piece list: an edit during the run counts, and deleted text is not synthesised.
	story, answer, files, readErr := s.storymakerNarrateRead(ctx, run)
	// The configuration of this moment. It goes ahead of a read error, because a desktop that was
	// switched off also fails its reads.
	cfg, err := s.storymakerNarrateGate()
	if err != nil {
		return false, err
	}
	if readErr != nil {
		return false, s.storymakerNarrateFailed(ctx, readErr)
	}

	target, noVoice, done, total := run.scan(&story, answer, files)
	run.h.SetPieces(done, total)
	if len(answer.Missing) == 0 {
		return false, nil
	}
	// A save during the run can set the voice to null, and BuildTTSConfig would turn an empty provider
	// into piper (tts_config.go:38-41).
	if noVoice != "" {
		return false, render.Fail(render.CodeVoiceRequired, "Block %s has no voice. Choose a voice and generate the narration again.", noVoice)
	}
	if target == nil {
		return false, render.Fail(render.CodeNarrationFailed, "Every piece is stored, but some blocks still have no usable envelope.")
	}
	// A stored file needs no provider call: it either decodes and gets its envelope, or it is removed.
	if target.ext != "" {
		return true, s.storymakerVerifyPiece(ctx, run, target)
	}

	// Too little disk fails before anything is paid for.
	if free, err := s.storymakerFreeDisk(cfg.VirtualDesktop.WorkspaceDir); err == nil && free < render.MinFreeDiskBytes {
		return false, render.Fail(render.CodeNarrationFailed, "Only %d MiB are free in the desktop workspace; narration needs at least %d MiB.", free>>20, render.MinFreeDiskBytes>>20)
	}
	// One provider call. The stored format is the detected one, never the provider's claim.
	provider := target.block.Voice.Provider
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
	if err != nil {
		if ctx.Err() != nil {
			return false, s.storymakerNarrateStopped()
		}
		return false, render.Fail(render.CodeNarrationFailed, "The voice provider %s failed on block %s: %s", provider, target.block.ID, storymakerProviderMessage(err))
	}
	ext, env, err := storymakerDecodePiece(data)
	if err != nil {
		return false, render.Fail(render.CodeNarrationFailed, "The voice provider %s returned audio that could not be decoded.", provider)
	}
	if err := run.rt.projects.WritePiece(ctx, run.id, target.hash, ext, data, env); err != nil {
		return false, s.storymakerNarrateFailed(ctx, err)
	}
	run.verified[target.hash] = true
	run.h.SetPieces(done+1, total)
	return true, nil
}

// storymakerNarrateRead reads the story, the audio answer and the piece list of this moment.
func (s *Server) storymakerNarrateRead(ctx context.Context, run *storymakerNarrateRun) (storymaker.Story, storymaker.AudioAnswer, map[string]storymaker.PieceFile, error) {
	// The desktop service can be replaced while a job runs. Requests refresh the store the same way.
	if svc, _, err := s.getDesktopService(ctx); err == nil {
		run.rt.store.use(svc)
	}
	data, _, err := run.rt.projects.LoadStory(ctx, run.id)
	if err != nil {
		return storymaker.Story{}, storymaker.AudioAnswer{}, nil, err
	}
	story, _, err := storymaker.DecodeStory(data, storymaker.DefaultLimits())
	if err != nil {
		return storymaker.Story{}, storymaker.AudioAnswer{}, nil, render.Fail(render.CodeInvalidStory, "The story can no longer be read, so the narration stopped.")
	}
	storymaker.ApplyDefaults(&story)
	answer, err := run.rt.projects.AudioAnswer(ctx, run.id, &story, nil)
	if err != nil {
		return storymaker.Story{}, storymaker.AudioAnswer{}, nil, err
	}
	files, err := run.rt.projects.PieceFiles(ctx, run.id)
	if err != nil {
		return storymaker.Story{}, storymaker.AudioAnswer{}, nil, err
	}
	return story, answer, files, nil
}

// storymakerNarrateGate checks the configuration of this moment.
func (s *Server) storymakerNarrateGate() (*config.Config, error) {
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		return nil, render.Fail(render.CodeNarrationUnavailable, "The Storymaker configuration is invalid: %s", err)
	}
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		return nil, render.Fail(render.CodeDisabled, "Storymaker was switched off during this narration.")
	}
	if cfg.VirtualDesktop.ReadOnly {
		return nil, render.Fail(render.CodeNarrationUnavailable, "The desktop is read-only, so the narration stopped.")
	}
	return cfg, nil
}

// scan finds the first piece that needs work. A block outside answer.Missing has all its pieces.
// Inside a missing block a stored file counts once it decoded during this job.
func (run *storymakerNarrateRun) scan(story *storymaker.Story, answer storymaker.AudioAnswer, files map[string]storymaker.PieceFile) (target *storymakerNarrateTarget, noVoice string, done, total int) {
	missing := make(map[string]bool, len(answer.Missing))
	for _, m := range answer.Missing {
		missing[m.BlockID] = true
	}
	for _, b := range storymaker.Blocks(story) {
		blockMissing := missing[b.ID]
		if blockMissing && b.Voice.Provider == "" && noVoice == "" {
			noVoice = b.ID
		}
		for _, piece := range storymaker.PiecesOf(b) {
			total++
			hash := storymaker.PieceKey(b.Voice, piece.Text)
			ext := files[hash].Ext
			if !blockMissing || (ext != "" && run.verified[hash]) {
				done++
				continue
			}
			if target == nil {
				target = &storymakerNarrateTarget{block: b, piece: piece, hash: hash, ext: ext}
			}
		}
	}
	return target, noVoice, done, total
}

// storymakerVerifyPiece checks a stored file with no provider call. A file that decodes gets a new
// envelope. A file that does not decode is removed, so a damaged file cannot fail a project forever,
// and the next round synthesises it again.
func (s *Server) storymakerVerifyPiece(ctx context.Context, run *storymakerNarrateRun, target *storymakerNarrateTarget) error {
	data, err := storymakerReadPiece(ctx, run, target)
	if err != nil {
		return s.storymakerNarrateFailed(ctx, err)
	}
	ext, env, err := storymakerDecodePiece(data)
	if err != nil || ext != target.ext {
		if err := run.rt.projects.RemovePiece(ctx, run.id, target.hash, target.ext); err != nil {
			return s.storymakerNarrateFailed(ctx, err)
		}
		return nil
	}
	if err := run.rt.projects.WriteEnvelope(ctx, run.id, target.hash, env); err != nil {
		return s.storymakerNarrateFailed(ctx, err)
	}
	run.verified[target.hash] = true
	return nil
}

// storymakerReadPiece reads a stored file. A file over the bound comes back as nil data, which does not decode.
func storymakerReadPiece(ctx context.Context, run *storymakerNarrateRun, target *storymakerNarrateTarget) ([]byte, error) {
	rc, _, err := run.rt.projects.OpenAudio(ctx, run.id, "narration", target.hash+"."+target.ext)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, storymakerMaxPieceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > storymakerMaxPieceBytes {
		return nil, nil
	}
	return data, nil
}

// storymakerDecodePiece decodes audio from its bytes and computes its envelope. Empty audio counts as
// undecodable, because ParseEnvelope would refuse its envelope on every later read.
func storymakerDecodePiece(data []byte) (string, storymaker.Envelope, error) {
	decoded, ext, err := storymaker.DecodeAudio(data)
	if err != nil {
		return "", storymaker.Envelope{}, err
	}
	env := storymaker.ComputeEnvelope(decoded)
	if env.DurationMS <= 0 {
		return "", storymaker.Envelope{}, errors.New("storymaker: the audio is empty")
	}
	return ext, env, nil
}

// storymakerNarrateStopped is the error of a run that lost its context without a cancel request. The
// job manager turns a requested cancel or a shutdown into its own end state, whatever this returns.
// It names both causes, because a revoked run can see its cancelled ctx before the new configuration
// is published (server.go revokes first and stores the snapshot later).
func (s *Server) storymakerNarrateStopped() error {
	return render.Fail(render.CodeNarrationUnavailable, "The desktop became read-only or its configuration changed, so the narration stopped.")
}

// storymakerNarrateFailed maps a storage or read error to the error of the job.
func (s *Server) storymakerNarrateFailed(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return s.storymakerNarrateStopped()
	}
	var jobErr *render.JobError
	if errors.As(err, &jobErr) {
		return jobErr
	}
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("[Storymaker] A narrate job could not use the project storage", "error", err)
	return render.Fail(render.CodeNarrationFailed, "The narration could not use the project storage.")
}

// storymakerProviderMessage makes a provider error fit for the job document: secrets scrubbed, 200 runes at most.
func storymakerProviderMessage(err error) string {
	text := []rune(security.Scrub(err.Error()))
	if len(text) > 200 {
		return string(text[:200]) + "..."
	}
	return string(text)
}
