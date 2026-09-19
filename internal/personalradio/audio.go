package personalradio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	mp3 "github.com/hajimehoshi/go-mp3"
)

const MaxImportBytes = 128 << 20
const maxAudioSeconds = 1200

type checkedAudioReader struct {
	io.Reader
	err error
}

func (r *checkedAudioReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

// prepareAudio validates the entire input and streams it to canonical PCM16 WAV.
// Keeping the original sample rate/channels avoids lossy resampling. The browser
// reads small sample-aligned windows instead of decoding a whole long recording.
func prepareAudio(ctx context.Context, input io.ReadSeeker, extension, dest string) (Track, error) {
	var t Track
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return t, err
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(input, MaxImportBytes+1))
	if err != nil {
		return t, err
	}
	if n > MaxImportBytes {
		return t, ErrLimit
	}
	t.Hash = hex.EncodeToString(h.Sum(nil))
	_, err = input.Seek(0, io.SeekStart)
	if err != nil {
		return t, err
	}
	var pcm io.Reader
	var expectedBytes int64 = -1
	bits, format := 16, 1
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "mp3":
		d, e := mp3.NewDecoder(input)
		if e != nil {
			return t, fmt.Errorf("radio_invalid_audio: %w", e)
		}
		t.Rate = d.SampleRate()
		t.Channels = 2
		pcm = d
	case "wav":
		var head [12]byte
		if _, err = io.ReadFull(input, head[:]); err != nil || string(head[:4]) != "RIFF" || string(head[8:]) != "WAVE" {
			return t, errors.New("radio_invalid_audio")
		}
		for i := 0; i < 128; i++ {
			var c [8]byte
			if _, err = io.ReadFull(input, c[:]); err != nil {
				return t, errors.New("radio_invalid_audio")
			}
			size := int64(binary.LittleEndian.Uint32(c[4:]))
			if size > MaxImportBytes {
				return t, ErrLimit
			}
			if string(c[:4]) == "fmt " {
				if size < 16 || size > 4096 {
					return t, errors.New("radio_invalid_audio")
				}
				b := make([]byte, size)
				if _, err = io.ReadFull(input, b); err != nil {
					return t, err
				}
				format = int(binary.LittleEndian.Uint16(b))
				t.Channels = int(binary.LittleEndian.Uint16(b[2:]))
				t.Rate = int(binary.LittleEndian.Uint32(b[4:]))
				bits = int(binary.LittleEndian.Uint16(b[14:]))
			} else if string(c[:4]) == "data" {
				expectedBytes = size
				pcm = io.LimitReader(input, size)
				break
			} else {
				if _, err = input.Seek(size, io.SeekCurrent); err != nil {
					return t, err
				}
			}
			if size%2 != 0 {
				if _, err = input.Seek(1, io.SeekCurrent); err != nil {
					return t, err
				}
			}
		}
	default:
		return t, errors.New("radio_format_unsupported")
	}
	if pcm == nil || t.Rate < 8000 || t.Rate > 96000 || t.Channels < 1 || t.Channels > 2 || (format != 1 && format != 3) || (format == 3 && bits != 32) || (bits != 8 && bits != 16 && bits != 24 && bits != 32) {
		return t, errors.New("radio_format_unsupported")
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return t, err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(dest)
		}
	}()
	if _, err = out.Write(make([]byte, 44)); err != nil {
		return t, err
	}
	width := bits / 8
	checked := &checkedAudioReader{Reader: pcm}
	t.Bytes = 44
	buf := make([]byte, 4096*t.Channels*width)
	converted := make([]byte, 4096*t.Channels*2)
	var samples int64
	var decodedBytes int64
	for {
		if err = ctx.Err(); err != nil {
			return t, err
		}
		count, e := io.ReadFull(checked, buf)
		decodedBytes += int64(count)
		if count%(width*t.Channels) != 0 {
			return t, errors.New("radio_truncated_audio")
		}
		if e != nil && e != io.EOF && e != io.ErrUnexpectedEOF {
			return t, fmt.Errorf("radio_invalid_audio: %w", e)
		}
		for i := 0; i < count/width; i++ {
			p := buf[i*width:]
			var v int16
			switch {
			case format == 3:
				f := float64(math.Float32frombits(binary.LittleEndian.Uint32(p)))
				if math.IsNaN(f) || math.IsInf(f, 0) {
					return t, errors.New("radio_invalid_audio")
				}
				v = int16(math.Max(-1, math.Min(1, f)) * 32767)
			case bits == 8:
				v = int16(int(p[0])-128) << 8
			case bits == 16:
				v = int16(binary.LittleEndian.Uint16(p))
			case bits == 24:
				v = int16(uint16(p[1]) | uint16(p[2])<<8)
			case bits == 32:
				v = int16(int32(binary.LittleEndian.Uint32(p)) >> 16)
			}
			binary.LittleEndian.PutUint16(converted[i*2:], uint16(v))
		}
		written := count / width * 2
		samples += int64(count / width / t.Channels)
		if samples > int64(t.Rate)*maxAudioSeconds {
			return t, ErrLimit
		}
		if _, err = out.Write(converted[:written]); err != nil {
			return t, err
		}
		t.Bytes += int64(written)
		if e != nil {
			break
		}
	}
	if checked.err != nil {
		return t, fmt.Errorf("radio_invalid_audio: %w", checked.err)
	}
	if expectedBytes >= 0 && decodedBytes != expectedBytes {
		return t, errors.New("radio_truncated_audio")
	}
	t.DurationMS = samples * 1000 / int64(t.Rate)
	if t.DurationMS < 100 {
		return t, errors.New("radio_empty_audio")
	}
	if _, err = out.Seek(0, io.SeekStart); err != nil {
		return t, err
	}
	if _, err = out.Write(waveHeader(t.Rate, t.Channels, t.Bytes-44)); err != nil {
		return t, err
	}
	if err = out.Sync(); err != nil {
		return t, err
	}
	if err = out.Close(); err != nil {
		return t, err
	}
	ok = true
	return t, nil
}
func waveHeader(rate, channels int, size int64) []byte {
	b := make([]byte, 44)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(size+36))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], uint16(channels))
	binary.LittleEndian.PutUint32(b[24:], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:], uint32(rate*channels*2))
	binary.LittleEndian.PutUint16(b[32:], uint16(channels*2))
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(size))
	return b
}

func (s *Service) Import(ctx context.Context, station string, input io.ReadSeeker, extension, title, origin, genre string, mediaID int64) (Track, error) {
	if origin != "local" && origin != "generated" {
		return Track{}, errors.New("radio_invalid_origin")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Track{}, ErrConflict
	}
	s.wg.Add(1)
	defer s.wg.Done()
	profile, err := s.station(station)
	var assetCount int
	if err == nil {
		err = s.db.QueryRow("SELECT COUNT(*) FROM station_tracks WHERE station=?", station).Scan(&assetCount)
	}
	if err == nil && assetCount >= 1000 {
		err = ErrLimit
	}
	s.mu.Unlock()
	if err != nil {
		return Track{}, err
	}
	id := newID()
	path := filepath.Join(s.dir, id+".wav")
	t, err := prepareAudio(ctx, input, extension, path)
	if err != nil {
		return t, err
	}
	t.ID = id
	t.Title = string([]rune(title)[:min(len([]rune(title)), 200)])
	t.Origin = origin
	t.Genre = genre
	t.MediaID = mediaID
	t.Weight = 100
	s.mu.Lock()
	defer s.mu.Unlock()
	keep := false
	defer func() {
		if !keep {
			os.Remove(path)
		}
	}()
	if err = ctx.Err(); err != nil {
		return t, err
	}
	var existing string
	if s.db.QueryRow("SELECT body FROM assets WHERE hash=?", t.Hash).Scan(&existing) == nil {
		if err = json.Unmarshal([]byte(existing), &t); err != nil {
			return t, err
		}
		if mediaID > 0 && t.MediaID == 0 {
			t.MediaID = mediaID
			if origin == "generated" {
				t.Origin = origin
			}
		}
		// Re-importing a missing or truncated owned copy repairs it without
		// changing its identity or playback history.
		old := filepath.Join(s.dir, t.ID+".wav")
		if info, e := os.Stat(old); e != nil || info.Size() != t.Bytes {
			if e = os.Remove(old); e != nil && !os.IsNotExist(e) {
				return t, e
			}
			if e = os.Rename(path, old); e != nil {
				return t, e
			}
		}
	} else {
		var total int64
		rows, e := s.db.Query("SELECT body FROM assets")
		if e != nil {
			return t, e
		}
		for rows.Next() {
			var b string
			var a Track
			if rows.Scan(&b) == nil && json.Unmarshal([]byte(b), &a) == nil {
				total += a.Bytes
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return t, e
		}
		if total+t.Bytes > int64(profile.LibraryMB)<<20 {
			return t, ErrLimit
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return t, err
	}
	defer tx.Rollback()
	b, _ := json.Marshal(t)
	if _, err = tx.Exec("INSERT INTO assets(id,hash,body) VALUES(?,?,?) ON CONFLICT(hash) DO UPDATE SET body=excluded.body", t.ID, t.Hash, string(b)); err != nil {
		return t, err
	}
	if _, err = tx.Exec("INSERT INTO station_tracks(station,asset,genre) VALUES(?,?,?) ON CONFLICT(station,asset) DO UPDATE SET genre=CASE WHEN excluded.genre!='' THEN excluded.genre ELSE station_tracks.genre END", station, t.ID, genre); err != nil {
		return t, err
	}
	if _, err = tx.Exec("DELETE FROM registry_ignored WHERE station=? AND media_id=?", station, mediaID); err != nil {
		return t, err
	}
	if err = tx.Commit(); err != nil {
		return t, err
	}
	keep = t.ID == id
	if genre != "" {
		t.Genre = genre
	}
	return t, nil
}

// AudioWindow exposes only validated, owned assets and a bounded PCM window.
func (s *Service) AudioWindow(id string, offsetMS, lengthMS int64) ([]byte, error) {
	if len(id) != 32 || offsetMS < 0 || offsetMS > maxAudioSeconds*1000 || lengthMS < 1 || lengthMS > 60000 {
		return nil, ErrNotFound
	}
	if _, err := hex.DecodeString(id); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	allowed := false
	var body string
	if s.db.QueryRow("SELECT body FROM assets WHERE id=?", id).Scan(&body) == nil {
		allowed = true
	}
	if !allowed {
		for _, x := range s.state.Queue {
			if x.AssetID == id {
				allowed = true
			}
		}
	}
	s.mu.Unlock()
	if !allowed {
		return nil, ErrNotFound
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(id + ".wav")
	if err != nil {
		return nil, ErrNotFound
	}
	defer f.Close()
	var h [44]byte
	if _, err = io.ReadFull(f, h[:]); err != nil {
		return nil, err
	}
	rate := int64(binary.LittleEndian.Uint32(h[24:]))
	channels := int64(binary.LittleEndian.Uint16(h[22:]))
	size := int64(binary.LittleEndian.Uint32(h[40:]))
	if rate < 8000 || rate > 96000 || channels < 1 || channels > 2 {
		return nil, ErrNotFound
	}
	start := offsetMS * rate / 1000 * channels * 2
	count := min(lengthMS*rate/1000*channels*2, size-start)
	if start >= size || count <= 0 {
		return nil, ErrNotFound
	}
	if _, err = f.Seek(44+start, io.SeekStart); err != nil {
		return nil, err
	}
	out := bytes.NewBuffer(waveHeader(int(rate), int(channels), count))
	if _, err = io.CopyN(out, f, count); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
