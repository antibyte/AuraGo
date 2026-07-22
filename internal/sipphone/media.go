package sipphone

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"aurago/internal/voice"

	"github.com/emiago/diago"
	"github.com/emiago/diago/audio"
	"github.com/emiago/diago/media"
)

type mediaPump struct {
	dialog     diago.DialogSession
	bridge     *voice.Bridge
	jitterMS   int
	onDTMF     func(rune)
	onError    func(error)
	dtmfMu     sync.Mutex
	dtmfWriter *diago.DTMFWriter
	negotiated string
}

func (p *mediaPump) start(ctx context.Context) error {
	props := &diago.MediaProps{}
	dtmfReader := &diago.DTMFReader{}
	dtmfReader.OnDTMF(func(digit rune) error {
		if p.onDTMF != nil {
			p.onDTMF(digit)
		}
		return nil
	})
	delayPackets := max(1, p.jitterMS/20)
	reader, err := p.dialog.Media().AudioReader(
		diago.WithAudioReaderJitterBuffer(media.RTPJitterBufferOptions{DelayPackets: delayPackets, MaxPackets: max(delayPackets+10, 20)}),
		diago.WithAudioReaderDTMF(dtmfReader),
		diago.WithAudioReaderMediaProps(props),
	)
	if err != nil {
		return fmt.Errorf("create SIP audio reader: %w", err)
	}
	writerProps := &diago.MediaProps{}
	dtmfWriter := &diago.DTMFWriter{}
	writer, err := p.dialog.Media().AudioWriter(
		diago.WithAudioWriterDTMF(dtmfWriter),
		diago.WithAudioWriterMediaProps(writerProps),
	)
	if err != nil {
		return fmt.Errorf("create SIP audio writer: %w", err)
	}
	p.dtmfMu.Lock()
	p.dtmfWriter = dtmfWriter
	p.negotiated = strings.ToLower(props.Codec.Name)
	p.dtmfMu.Unlock()
	if p.negotiated != "pcma" && p.negotiated != "pcmu" {
		return fmt.Errorf("unsupported negotiated SIP codec %q", props.Codec.Name)
	}

	go p.readLoop(ctx, reader, props.Codec.Name)
	go p.writeLoop(ctx, writer, writerProps.Codec.Name)
	return nil
}

func (p *mediaPump) readLoop(ctx context.Context, reader io.Reader, codec string) {
	encoded := make([]byte, 160)
	linear := make([]byte, 320)
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		n, err := io.ReadFull(reader, encoded)
		if err != nil {
			if ctx.Err() == nil && p.onError != nil {
				p.onError(fmt.Errorf("read SIP RTP audio: %w", err))
			}
			return
		}
		var decoded int
		if strings.EqualFold(codec, "PCMA") {
			decoded, err = audio.DecodeAlawTo(linear, encoded[:n])
		} else {
			decoded, err = audio.DecodeUlawTo(linear, encoded[:n])
		}
		if err != nil {
			if p.onError != nil {
				p.onError(fmt.Errorf("decode G.711 audio: %w", err))
			}
			return
		}
		samples := make([]int16, decoded/2)
		for i := range samples {
			samples[i] = int16(binary.LittleEndian.Uint16(linear[i*2 : i*2+2]))
		}
		_ = p.bridge.PushReceive(voice.PCMFrame{Samples: samples, SampleRate: 8000})
	}
}

func (p *mediaPump) writeLoop(ctx context.Context, writer io.Writer, codec string) {
	for {
		frame, err := p.bridge.NextSend(ctx)
		if err != nil {
			return
		}
		if frame.SampleRate != 8000 {
			if p.onError != nil {
				p.onError(fmt.Errorf("SIP media expects 8 kHz PCM, got %d", frame.SampleRate))
			}
			return
		}
		for offset := 0; offset < len(frame.Samples); offset += 160 {
			linear := make([]byte, 320)
			end := min(offset+160, len(frame.Samples))
			for i, sample := range frame.Samples[offset:end] {
				binary.LittleEndian.PutUint16(linear[i*2:i*2+2], uint16(sample))
			}
			encoded := make([]byte, 160)
			if strings.EqualFold(codec, "PCMA") {
				_, err = audio.EncodeAlawTo(encoded, linear)
			} else {
				_, err = audio.EncodeUlawTo(encoded, linear)
			}
			if err == nil {
				_, err = writer.Write(encoded)
			}
			if err != nil {
				if ctx.Err() == nil && p.onError != nil {
					p.onError(fmt.Errorf("write SIP RTP audio: %w", err))
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
}

func (p *mediaPump) sendDTMF(digit rune) error {
	p.dtmfMu.Lock()
	writer := p.dtmfWriter
	p.dtmfMu.Unlock()
	if writer == nil {
		return fmt.Errorf("SIP media is not active")
	}
	return writer.WriteDTMF(digit)
}

func (p *mediaPump) codec() string {
	p.dtmfMu.Lock()
	defer p.dtmfMu.Unlock()
	return p.negotiated
}
