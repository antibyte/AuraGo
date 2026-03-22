package tools

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Wyoming protocol client — https://github.com/rhasspy/wyoming
//
// Wire format per event:
//   1. JSON header on a single line, terminated by \n
//      {"type":"...","data":{...},"data_length":N,"payload_length":M}
//   2. If data_length > 0   → N additional bytes of UTF-8 JSON (merged into data)
//   3. If payload_length > 0 → M bytes of raw binary payload (e.g. PCM audio)
// ──────────────────────────────────────────────────────────────────────────────

const (
	wyomingDialTimeout  = 5 * time.Second
	wyomingReadTimeout  = 30 * time.Second
	wyomingWriteTimeout = 5 * time.Second
)

// WyomingEvent represents a single Wyoming protocol event.
type WyomingEvent struct {
	Type          string                 `json:"type"`
	Data          map[string]interface{} `json:"data,omitempty"`
	DataLength    int                    `json:"data_length,omitempty"`
	PayloadLength int                    `json:"payload_length,omitempty"`
}

// WyomingVoice describes a voice returned by the Wyoming "info" event.
type WyomingVoice struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	Speakers    []string `json:"speakers"`
	Installed   bool     `json:"installed"`
	Version     string   `json:"version"`
}

// WyomingConnect dials a Wyoming server via TCP.
func WyomingConnect(addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, wyomingDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("wyoming dial %s: %w", addr, err)
	}
	return conn, nil
}

// writeWyomingEvent serialises an event and writes it to conn.
func writeWyomingEvent(conn net.Conn, evt WyomingEvent, payload []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(wyomingWriteTimeout)); err != nil {
		return err
	}
	if len(payload) > 0 {
		evt.PayloadLength = len(payload)
	}
	hdr, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal wyoming event: %w", err)
	}
	hdr = append(hdr, '\n')
	if _, err := conn.Write(hdr); err != nil {
		return fmt.Errorf("write wyoming header: %w", err)
	}
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("write wyoming payload: %w", err)
		}
	}
	return nil
}

// readWyomingEvent reads one event from the buffered reader.
// Returns the parsed event header and any binary payload.
func readWyomingEvent(r *bufio.Reader, conn net.Conn) (*WyomingEvent, []byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(wyomingReadTimeout)); err != nil {
		return nil, nil, err
	}
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, nil, fmt.Errorf("read wyoming header: %w", err)
	}
	var evt WyomingEvent
	if err := json.Unmarshal(line, &evt); err != nil {
		return nil, nil, fmt.Errorf("unmarshal wyoming header: %w", err)
	}

	// Read optional additional data
	if evt.DataLength > 0 {
		dataBuf := make([]byte, evt.DataLength)
		if _, err := io.ReadFull(r, dataBuf); err != nil {
			return nil, nil, fmt.Errorf("read wyoming data: %w", err)
		}
		var extra map[string]interface{}
		if json.Unmarshal(dataBuf, &extra) == nil {
			if evt.Data == nil {
				evt.Data = extra
			} else {
				for k, v := range extra {
					evt.Data[k] = v
				}
			}
		}
	}

	// Read optional binary payload
	var payload []byte
	if evt.PayloadLength > 0 {
		payload = make([]byte, evt.PayloadLength)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, nil, fmt.Errorf("read wyoming payload: %w", err)
		}
	}
	return &evt, payload, nil
}

// WyomingSynthesize sends a "synthesize" event and collects the resulting PCM audio chunks.
// Returns concatenated raw PCM bytes along with audio format parameters (rate, width, channels).
func WyomingSynthesize(conn net.Conn, text, voice string, speakerID int) (pcm []byte, rate, width, channels int, err error) {
	data := map[string]interface{}{
		"text": text,
	}
	if voice != "" {
		data["voice"] = map[string]interface{}{"name": voice}
	}
	if speakerID > 0 {
		data["voice"] = map[string]interface{}{"name": voice, "speaker": speakerID}
	}

	evt := WyomingEvent{Type: "synthesize", Data: data}
	if err := writeWyomingEvent(conn, evt, nil); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("send synthesize: %w", err)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)
	var pcmBuf []byte
	rate, width, channels = 22050, 2, 1 // sensible defaults

	for {
		respEvt, payload, err := readWyomingEvent(reader, conn)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("read synthesize response: %w", err)
		}

		switch respEvt.Type {
		case "audio-start":
			if r, ok := respEvt.Data["rate"].(float64); ok {
				rate = int(r)
			}
			if w, ok := respEvt.Data["width"].(float64); ok {
				width = int(w)
			}
			if c, ok := respEvt.Data["channels"].(float64); ok {
				channels = int(c)
			}

		case "audio-chunk":
			if len(payload) > 0 {
				pcmBuf = append(pcmBuf, payload...)
			}

		case "audio-stop":
			return pcmBuf, rate, width, channels, nil

		case "error":
			msg := "unknown error"
			if m, ok := respEvt.Data["text"].(string); ok {
				msg = m
			}
			return nil, 0, 0, 0, fmt.Errorf("wyoming error: %s", msg)
		}
	}
}

// WyomingDescribe sends a "describe" event and parses the "info" response to list available TTS voices.
func WyomingDescribe(conn net.Conn) ([]WyomingVoice, error) {
	evt := WyomingEvent{Type: "describe"}
	if err := writeWyomingEvent(conn, evt, nil); err != nil {
		return nil, fmt.Errorf("send describe: %w", err)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)
	respEvt, _, err := readWyomingEvent(reader, conn)
	if err != nil {
		return nil, fmt.Errorf("read describe response: %w", err)
	}

	if respEvt.Type != "info" {
		return nil, fmt.Errorf("expected info event, got %s", respEvt.Type)
	}

	// Parse tts array from data
	ttsRaw, ok := respEvt.Data["tts"]
	if !ok {
		return nil, fmt.Errorf("info event has no tts field")
	}

	// Re-marshal and unmarshal to get a typed slice
	b, err := json.Marshal(ttsRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal tts field: %w", err)
	}

	var ttsList []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Installed   bool   `json:"installed"`
		Version     string `json:"version"`
		Voices      []struct {
			Name         string          `json:"name"`
			Description  string          `json:"description"`
			Installed    bool            `json:"installed"`
			Version      string          `json:"version"`
			RawLanguages json.RawMessage `json:"languages"`
			Speakers     []struct {
				Name string `json:"name"`
			} `json:"speakers"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(b, &ttsList); err != nil {
		return nil, fmt.Errorf("unmarshal tts list: %w", err)
	}

	var voices []WyomingVoice
	for _, sys := range ttsList {
		if len(sys.Voices) == 0 {
			// Fallback: treat the system entry itself as a voice (no nested voices)
			voices = append(voices, WyomingVoice{
				Name:        sys.Name,
				Description: sys.Description,
				Installed:   sys.Installed,
				Version:     sys.Version,
			})
			continue
		}
		for _, sv := range sys.Voices {
			v := WyomingVoice{
				Name:        sv.Name,
				Description: sv.Description,
				Installed:   sv.Installed,
				Version:     sv.Version,
			}
			if sv.RawLanguages != nil {
				var langStrings []string
				if json.Unmarshal(sv.RawLanguages, &langStrings) != nil {
					var langObjs []struct {
						Name string `json:"name"`
					}
					if json.Unmarshal(sv.RawLanguages, &langObjs) == nil {
						for _, lo := range langObjs {
							langStrings = append(langStrings, lo.Name)
						}
					}
				}
				v.Languages = langStrings
			}
			for _, sp := range sv.Speakers {
				v.Speakers = append(v.Speakers, sp.Name)
			}
			voices = append(voices, v)
		}
	}

	return voices, nil
}

// PCMToWAV wraps raw PCM audio data in a WAV container.
func PCMToWAV(pcm []byte, rate, width, channels int) []byte {
	dataLen := len(pcm)
	fileLen := 36 + dataLen // 44 byte header - 8 byte RIFF header = 36

	buf := make([]byte, 44+dataLen)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileLen))
	copy(buf[8:12], "WAVE")

	// fmt sub-chunk
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // sub-chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(rate))
	byteRate := rate * channels * width
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	blockAlign := channels * width
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(width*8))

	// data sub-chunk
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataLen))
	copy(buf[44:], pcm)

	return buf
}
