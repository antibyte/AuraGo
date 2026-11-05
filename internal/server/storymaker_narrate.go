package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/desktop"
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

const storymakerUsageVersion = 1

// storymakerUsageFile is usage.json: the characters sent to cloud voices on one local date.
type storymakerUsageFile struct {
	V     int    `json:"v"`
	Date  string `json:"date"`
	Chars int    `json:"chars"`
}

// storymakerNarrationUsage keeps the daily count in storymaker/narrate/usage.json. It fails closed:
// an unreadable file stops cloud calls for that date, because a budget guard that fails open would be
// a bypass.
type storymakerNarrationUsage struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
}

func newStorymakerNarrationUsage(dir string) *storymakerNarrationUsage {
	return &storymakerNarrationUsage{path: filepath.Join(dir, "usage.json"), now: time.Now}
}

func (u *storymakerNarrationUsage) badPath() string {
	return filepath.Join(filepath.Dir(u.path), "usage.bad.json")
}

// loadLocked returns the count of the current local date. A missing file or an older date starts at
// 0. An unreadable file is renamed to usage.bad.json and replaced by one that holds maxDaily.
func (u *storymakerNarrationUsage) loadLocked(maxDaily int) (storymakerUsageFile, error) {
	today := u.now().Format(time.DateOnly)
	fresh := storymakerUsageFile{V: storymakerUsageVersion, Date: today}
	data, err := os.ReadFile(u.path)
	if errors.Is(err, os.ErrNotExist) {
		return fresh, nil
	}
	if err != nil {
		return storymakerUsageFile{}, err
	}
	var cur storymakerUsageFile
	if json.Unmarshal(data, &cur) != nil || cur.V != storymakerUsageVersion || cur.Chars < 0 || !storymakerValidUsageDate(cur.Date) {
		_ = fileutil.Rename(u.path, u.badPath())
		fresh.Chars = maxDaily
		return fresh, u.writeLocked(fresh)
	}
	if cur.Date != today {
		return fresh, nil
	}
	return cur, nil
}

func storymakerValidUsageDate(date string) bool {
	_, err := time.Parse(time.DateOnly, date)
	return err == nil
}

// writeLocked publishes the file through a synced temp file and a rename. It takes no job context:
// the count must survive a revoked run.
func (u *storymakerNarrationUsage) writeLocked(cur storymakerUsageFile) error {
	data, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(u.path), 0o700); err != nil {
		return err
	}
	return fileutil.WriteFileContext(context.Background(), u.path, data, 0o600)
}

// reserve counts chars for today and writes the file, or returns a budget_exceeded job error.
func (u *storymakerNarrationUsage) reserve(chars, maxDaily int) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	cur, err := u.loadLocked(maxDaily)
	if err != nil {
		return render.Fail(render.CodeBudgetExceeded, "The daily count in %s could not be read or written, so cloud voices stop.", filepath.Base(u.path))
	}
	if cur.Chars+chars > maxDaily {
		if _, statErr := os.Stat(u.badPath()); statErr == nil && cur.Chars >= maxDaily {
			return render.Fail(render.CodeBudgetExceeded, "Cloud voices stop for today: the daily limit of %d characters is reached, and an unreadable count was kept as %s.", maxDaily, filepath.Base(u.badPath()))
		}
		return render.Fail(render.CodeBudgetExceeded, "The daily limit of %d characters for cloud voices is reached: %d are used and this piece needs %d.", maxDaily, cur.Chars, chars)
	}
	cur.Chars += chars
	if err := u.writeLocked(cur); err != nil {
		return render.Fail(render.CodeBudgetExceeded, "The daily count in %s could not be read or written, so cloud voices stop.", filepath.Base(u.path))
	}
	return nil
}

// release subtracts chars after a provider error with no bytes.
func (u *storymakerNarrationUsage) release(chars, maxDaily int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	cur, err := u.loadLocked(maxDaily)
	if err != nil {
		return
	}
	cur.Chars -= chars
	if cur.Chars < 0 {
		cur.Chars = 0
	}
	_ = u.writeLocked(cur)
}

// today returns the characters counted for the current local date.
func (u *storymakerNarrationUsage) today(maxDaily int) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	cur, err := u.loadLocked(maxDaily)
	if err != nil {
		return 0, err
	}
	return cur.Chars, nil
}

// storymakerNarratePayload is the payload of a narrate job.
type storymakerNarratePayload struct{ ProjectID string }

// storymakerNarrateRun is the state of one narrate job.
type storymakerNarrateRun struct {
	rt *storymakerRuntime
	id string
	h  *render.Handle
	// verified holds the hashes whose stored file decoded during this job, so each file is decoded once.
	verified map[string]bool
	// sent counts the characters this job has sent to a cloud voice.
	sent int
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
	if music, ok := payload.(storymakerMusicPayload); ok {
		return s.storymakerMusicRun(ctx, rt, job, music, h)
	}
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
	// The caps of a cloud voice. The job cap counts what this job already sent. The daily count is
	// written before the call, so a crash during the call cannot lose it.
	provider := target.block.Voice.Provider
	maxDaily := cfg.Storymaker.Narration.MaxDailyChars
	charge := 0
	if !storymaker.LocalVoiceProvider(provider) {
		charge = utf8.RuneCountInString(target.piece.Text)
		if maxJob := cfg.Storymaker.Narration.MaxJobChars; run.sent+charge > maxJob {
			return false, render.Fail(render.CodeBudgetExceeded, "This job would send %d characters to %s; one job may send %d.", run.sent+charge, provider, maxJob)
		}
		if err := run.rt.usage.reserve(charge, maxDaily); err != nil {
			return false, err
		}
		run.sent += charge
	}
	// One provider call. The stored format is the detected one, never the provider's claim.
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
	if err != nil {
		// No bytes: nothing to bill. Once the provider has answered it has billed, so a decode or
		// write failure below keeps the reservation.
		if charge > 0 && len(data) == 0 {
			run.rt.usage.release(charge, maxDaily)
		}
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

const (
	maxMusicSourceBytes = int64(1) << 30 // 1 GiB
	minMusicDurationMS  = 1000
	maxMusicDurationMS  = 3600000
)

// storymakerMusicPayload is the payload of a music job. DurationMS is the film length, for a generated track. GenerateKey
// is the music key a Generate click named. Only POST /music with generate:true sets it, and the job calls a provider only
// for that key, because the story can change while the job waits.
type storymakerMusicPayload struct {
	ProjectID   string
	DurationMS  int
	GenerateKey string
}

// storymakerMusicRefusal is a music source that POST /music and the job refuse. Its text is safe to show.
type storymakerMusicRefusal struct{ message string }

func (e *storymakerMusicRefusal) Error() string { return e.message }

func storymakerMusicDurationOK(ms int) bool {
	return ms >= minMusicDurationMS && ms <= maxMusicDurationMS
}

// storymakerMusicAudioRel returns file relative to DataDir/audio in slash form, and refuses a file outside it.
func storymakerMusicAudioRel(cfg *config.Config, file string) (string, error) {
	root, err := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "audio"))
	if err != nil {
		return "", fmt.Errorf("resolve the audio folder: %w", err)
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("resolve the music file: %w", err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return "", &storymakerMusicRefusal{message: "The music file is not in the audio folder."}
	}
	return filepath.ToSlash(rel), nil
}

// storymakerMusicTrackPath turns a media id into its desktop path below the Music mount.
func (s *Server) storymakerMusicTrackPath(cfg *config.Config, track string) (string, error) {
	id, err := strconv.ParseInt(track, 10, 64)
	if err != nil || id <= 0 {
		return "", &storymakerMusicRefusal{message: "The music track id is not valid."}
	}
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil {
		return "", fmt.Errorf("%w: track %d", storymaker.ErrMusicSourceMissing, id)
	}
	if item.MediaType != "music" {
		return "", &storymakerMusicRefusal{message: "This media item is not a music track."}
	}
	rel, err := storymakerMusicAudioRel(cfg, item.FilePath)
	if err != nil {
		return "", err
	}
	return "Music/" + rel, nil
}

// storymakerMusicSourceOpen opens the story's file or track. Every source goes through Store.Open, so the desktop's
// mount and symlink checks hold. A missing source wraps storymaker.ErrMusicSourceMissing.
func (s *Server) storymakerMusicSourceOpen(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, m *storymaker.Music) (io.ReadSeekCloser, storymaker.FileInfo, error) {
	source := m.File
	if m.Track != "" {
		trackPath, err := s.storymakerMusicTrackPath(cfg, m.Track)
		if err != nil {
			return nil, storymaker.FileInfo{}, err
		}
		source = trackPath
	}
	r, info, err := rt.store.Open(ctx, source)
	switch {
	case err == nil:
		return r, info, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, storymaker.FileInfo{}, fmt.Errorf("%w: %w", storymaker.ErrMusicSourceMissing, err)
	case ctx.Err() != nil:
		return nil, storymaker.FileInfo{}, ctx.Err()
	}
	return nil, storymaker.FileInfo{}, &storymakerMusicRefusal{message: "The music source could not be opened."}
}

// storymakerCheckMusicSource makes the checks that read no more than the head of a source: size, format by its bytes
// and free disk. It leaves r at the start and returns the format.
func storymakerCheckMusicSource(r io.ReadSeeker, size int64, workspaceDir string) (string, error) {
	if size > maxMusicSourceBytes {
		return "", &storymakerMusicRefusal{message: "The music file is larger than 1 GiB."}
	}
	head := make([]byte, 12)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("read the music source: %w", err)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind the music source: %w", err)
	}
	ext := storymaker.SniffAudio(head[:n])
	if ext != "wav" && ext != "mp3" {
		return "", &storymakerMusicRefusal{message: "The music file is not WAV or MP3."}
	}
	free, err := fileutil.FreeDiskBytes(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("read the free disk space: %w", err)
	}
	if free < size+render.MinFreeDiskBytes {
		return "", &storymakerMusicRefusal{message: fmt.Sprintf("The disk has %d MiB free, and this music file needs %d MiB.", free>>20, (size+render.MinFreeDiskBytes)>>20)}
	}
	return ext, nil
}

// storymakerMusicSourceProblem runs the checks POST /music makes before any job exists.
func (s *Server) storymakerMusicSourceProblem(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, m *storymaker.Music) error {
	src, info, err := s.storymakerMusicSourceOpen(ctx, rt, cfg, m)
	if err != nil {
		return err
	}
	defer src.Close()
	if _, err := storymakerCheckMusicSource(src, info.Size, cfg.VirtualDesktop.WorkspaceDir); err != nil {
		return err
	}
	_, durationMS, err := storymaker.ProbeAudio(src)
	if err != nil {
		return &storymakerMusicRefusal{message: "The music file could not be read as audio."}
	}
	if !storymakerMusicDurationOK(durationMS) {
		return &storymakerMusicRefusal{message: "The music file must last between 1 second and 1 hour."}
	}
	return nil
}

// storymakerMusicRun is the music job: it stores the story's music under its key and writes the meta. A ctx error becomes
// music_failed here, and runJob still reports a cancel request or a shutdown as such.
func (s *Server) storymakerMusicRun(ctx context.Context, rt *storymakerRuntime, job render.Job, p storymakerMusicPayload, h *render.Handle) (render.Output, error) {
	ctx, release, err := s.beginDesktopRun(ctx)
	if err != nil {
		return render.Output{}, storymakerMusicStopped()
	}
	defer release()
	err = s.storymakerMusicStore(ctx, rt, p, h)
	var je *render.JobError
	var refusal *storymakerMusicRefusal
	switch {
	case err == nil:
		return render.Output{}, nil
	case ctx.Err() != nil:
		return render.Output{}, storymakerMusicStopped()
	case errors.As(err, &je):
		return render.Output{}, je
	case errors.As(err, &refusal):
		return render.Output{}, render.Fail(render.CodeMusicFailed, "%s", refusal.message)
	case errors.Is(err, storymaker.ErrMusicSourceMissing):
		return render.Output{}, render.Fail(render.CodeMusicFailed, "The music source no longer exists.")
	}
	s.storymakerMusicLogger().Warn("[Storymaker] The music job failed", "project", p.ProjectID, "error", err)
	return render.Output{}, render.Fail(render.CodeMusicFailed, "The music could not be stored.")
}

// storymakerMusicStopped names both causes, because a revoked run can see its cancelled ctx before the new
// configuration is published (server.go revokes first and stores the snapshot later).
func storymakerMusicStopped() *render.JobError {
	return render.Fail(render.CodeMusicFailed, "The desktop became read-only or its configuration changed, so the music was not stored.")
}

func (s *Server) storymakerMusicLogger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// storymakerMusicStore re-reads the story, so an edit while the job waited is picked up, and brings the key's file and
// meta into place. A stored file without valid meta needs no source: it is only probed. A paid generation runs only under
// the key in the payload, so an edit while the job waited cannot start one.
func (s *Server) storymakerMusicStore(ctx context.Context, rt *storymakerRuntime, p storymakerMusicPayload, h *render.Handle) error {
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		return err
	}
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		return render.Fail(render.CodeDisabled, "Storymaker was switched off before the music was stored.")
	}
	data, _, err := rt.projects.LoadStory(ctx, p.ProjectID)
	if err != nil {
		return err
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(cfg))
	if err != nil {
		return err
	}
	storymaker.ApplyDefaults(&story)
	key, err := s.storymakerMusicKey(ctx, story.Music)
	if err != nil {
		return err
	}
	if key == "" {
		return nil
	}
	meta, ext, err := rt.projects.MusicInfo(ctx, p.ProjectID, key)
	if err != nil {
		return err
	}
	if meta.V == 1 && ext != "" {
		return nil
	}
	if ext == "" {
		if ext, err = s.storymakerCopyMusicSource(ctx, rt, cfg, p, key, story.Music, h); err != nil {
			return err
		}
	}
	return storymakerFinishMusic(ctx, rt, p.ProjectID, key, ext)
}

// storymakerCopyMusicSource copies the story's source to the key's file and returns its format.
func (s *Server) storymakerCopyMusicSource(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, p storymakerMusicPayload, key string, m *storymaker.Music, h *render.Handle) (string, error) {
	if m.Mode == "generated" {
		return "", &storymakerMusicRefusal{message: "Generated music cannot run yet."}
	}
	h.SetPhase(render.PhaseWriting)
	src, info, err := s.storymakerMusicSourceOpen(ctx, rt, cfg, m)
	if err != nil {
		return "", err
	}
	defer src.Close()
	ext, err := storymakerCheckMusicSource(src, info.Size, cfg.VirtualDesktop.WorkspaceDir)
	if err != nil {
		return "", err
	}
	return ext, storymakerStoreMusic(ctx, rt, p.ProjectID, key, ext, src)
}

// storymakerStoreMusic writes the key's create-only file. A file that already exists counts as success, because the key
// says it has this content.
func storymakerStoreMusic(ctx context.Context, rt *storymakerRuntime, id, key, ext string, r io.Reader) error {
	target, err := rt.projects.AudioPath(id, "music", key+"."+ext)
	if err != nil {
		return err
	}
	err = rt.store.WriteStream(ctx, target, r, maxMusicSourceBytes)
	var conflict *desktop.PathConflict
	if errors.As(err, &conflict) {
		return nil
	}
	return err
}

// storymakerFinishMusic probes the stored copy on every path, because not every provider reports a duration. A copy of
// the wrong length is removed. The meta comes after the file, so a file without meta counts as missing.
func storymakerFinishMusic(ctx context.Context, rt *storymakerRuntime, id, key, ext string) error {
	file := key + "." + ext
	r, _, err := rt.projects.OpenAudio(ctx, id, "music", file)
	if err != nil {
		return err
	}
	_, durationMS, probeErr := storymaker.ProbeAudio(r)
	// Close first: Windows cannot remove an open file.
	_ = r.Close()
	if probeErr != nil || !storymakerMusicDurationOK(durationMS) {
		target, err := rt.projects.AudioPath(id, "music", file)
		if err != nil {
			return err
		}
		if err := rt.store.Remove(ctx, target); err != nil {
			return err
		}
		return &storymakerMusicRefusal{message: "The music must be readable audio that lasts between 1 second and 1 hour."}
	}
	return rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS})
}
