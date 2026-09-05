package meshcore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const maxFrame = 176

// frameLink preserves GATT notification boundaries and serial framing alike.
type frameLink interface {
	ReadFrame() ([]byte, error)
	WriteFrame([]byte) error
	Close() error
}
type serialLink struct{ io.ReadWriteCloser }

func (s *serialLink) ReadFrame() ([]byte, error) {
	var h [3]byte
	for {
		if _, err := io.ReadFull(s, h[:1]); err != nil {
			return nil, err
		}
		if h[0] == '>' {
			break
		}
	}
	if _, err := io.ReadFull(s, h[1:]); err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint16(h[1:]))
	if n < 1 || n > maxFrame {
		return nil, fmt.Errorf("invalid companion frame length")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(s, b)
	return b, err
}
func (s *serialLink) WriteFrame(b []byte) error {
	if len(b) < 1 || len(b) > maxFrame {
		return fmt.Errorf("invalid companion frame length")
	}
	f := make([]byte, 3+len(b))
	f[0] = '<'
	binary.LittleEndian.PutUint16(f[1:], uint16(len(b)))
	copy(f[3:], b)
	for len(f) > 0 {
		n, err := s.Write(f)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		f = f[n:]
	}
	return nil
}

type companion struct {
	link      frameLink
	commands  chan struct{}
	frames    chan []byte
	wake      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	ackMu     sync.Mutex
	acks      map[uint32]time.Time
	onACK     func(uint32)
	session   string
}

func newCompanion(link frameLink) *companion {
	c := &companion{link: link, commands: make(chan struct{}, 1), frames: make(chan []byte, 512), wake: make(chan struct{}, 1), done: make(chan struct{}), acks: map[uint32]time.Time{}, session: fmt.Sprintf("%d", time.Now().UnixNano())}
	go func() {
		defer c.Close()
		for {
			b, err := link.ReadFrame()
			if err != nil {
				return
			}
			if len(b) == 0 || len(b) > maxFrame {
				return
			}
			if b[0] >= 0x80 {
				if b[0] == 0x82 && len(b) == 9 {
					c.ackMu.Lock()
					now := time.Now()
					for k, t := range c.acks {
						if now.Sub(t) > 10*time.Minute {
							delete(c.acks, k)
						}
					}
					if len(c.acks) < 128 {
						c.acks[binary.LittleEndian.Uint32(b[1:])] = now
					}
					callback := c.onACK
					c.ackMu.Unlock()
					if callback != nil {
						callback(binary.LittleEndian.Uint32(b[1:]))
					}
				}
				if b[0] == 0x83 || b[0] == 0x80 || b[0] == 0x8a || b[0] == 0x8f {
					select {
					case c.wake <- struct{}{}:
					default:
					}
				}
				continue
			}
			select {
			case c.frames <- b:
			case <-c.done:
				return
			default:
				return
			}
		}
	}()
	return c
}
func (c *companion) Close() { c.closeOnce.Do(func() { close(c.done); _ = c.link.Close() }) }
func (c *companion) request(ctx context.Context, cmd []byte, expected ...byte) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case c.commands <- struct{}{}:
		defer func() { <-c.commands }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, io.ErrClosedPipe
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-c.done:
		return nil, io.ErrClosedPipe
	default:
	}
	if err := c.link.WriteFrame(cmd); err != nil {
		c.Close()
		return nil, err
	}
	var result [][]byte
	for {
		select {
		case <-ctx.Done():
			c.Close()
			return nil, ctx.Err() // discard late responses by closing this connection
		case <-c.done:
			return nil, io.ErrClosedPipe
		case b := <-c.frames:
			if b[0] == 1 || b[0] == 15 {
				return nil, fmt.Errorf("companion rejected command %d", cmd[0])
			}
			if cmd[0] == 4 && (b[0] == 2 || b[0] == 3) {
				if len(result) >= 4096 {
					c.Close()
					return nil, fmt.Errorf("contact limit exceeded")
				}
				result = append(result, b)
				continue
			}
			if bytes.IndexByte(expected, b[0]) < 0 {
				c.Close()
				return nil, fmt.Errorf("unexpected companion response")
			}
			return append(result, b), nil
		}
	}
}

func (c *companion) snapshot(ctx context.Context, salt []byte) (Status, error) {
	st := Status{State: "connected", Contacts: []Contact{}, Channels: []Channel{}}
	info, err := c.request(ctx, []byte{22, 3}, 13)
	if err != nil {
		return st, err
	}
	if len(info[0]) < 4 {
		return st, fmt.Errorf("unsupported companion version")
	}
	slots := int(info[0][3])
	st.ChannelCapacity = slots
	if slots < 1 || slots > 64 {
		return st, fmt.Errorf("invalid channel capacity")
	}
	if len(info[0]) >= 80 {
		st.Firmware = wireText(info[0][60:80])
	}
	self, err := c.request(ctx, []byte{1, 0, 0, 0, 0, 0, 0, 0, 'A', 'u', 'r', 'a', 'G', 'o'}, 5)
	if err != nil {
		return st, err
	}
	if len(self[0]) < 58 {
		return st, fmt.Errorf("invalid self info")
	}
	st.IdentityKey = hex.EncodeToString(self[0][4:36])
	st.Name = wireText(self[0][58:])
	st.nameBytes = len(st.Name)
	if !ValidKey(st.IdentityKey) {
		return st, fmt.Errorf("invalid device identity")
	}
	contacts, err := c.request(ctx, []byte{4}, 4)
	if err != nil {
		return st, err
	}
	for _, b := range contacts {
		if b[0] != 3 {
			continue
		}
		if len(b) < 148 {
			return st, fmt.Errorf("invalid contact")
		}
		st.Contacts = append(st.Contacts, Contact{Key: hex.EncodeToString(b[1:33]), Type: b[33], Name: wireText(b[100:132])})
	}
	for i := 0; i < slots; i++ {
		frames, err := c.request(ctx, []byte{31, byte(i)}, 18)
		if err != nil {
			return st, err
		}
		b := frames[0]
		if len(b) != 50 || int(b[1]) != i {
			return st, fmt.Errorf("invalid channel info")
		}
		name := wireText(b[2:34])
		if name == "" {
			clear(b)
			continue
		}
		st.Channels = append(st.Channels, Channel{Index: i, Name: name, Binding: channelBinding(st.IdentityKey, b, salt), Kind: channelKind(name, b[34:50])})
		clear(b)
	}
	return st, nil
}
func channelBinding(identity string, b, salt []byte) string {
	// Companion strcpy leaves undefined bytes after the channel name's NUL.
	// Canonical zero padding preserves bindings from already zero-filled frames.
	var name [32]byte
	rawName := b[2:34]
	if end := bytes.IndexByte(rawName, 0); end >= 0 {
		rawName = rawName[:end]
	}
	copy(name[:], rawName)
	h := hmac.New(sha256.New, salt)
	h.Write([]byte(identity))
	h.Write(b[1:2])
	h.Write(name[:])
	h.Write(b[34:50])
	return hex.EncodeToString(h.Sum(nil))
}
func wireText(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.ToValidUTF8(string(b), "")
}

func decodeMessage(b []byte, st Status) (Message, error) {
	m := Message{Direction: "incoming", IdentityKey: st.IdentityKey, Channel: -1, ReceivedAt: time.Now().Unix(), State: "pending"}
	if len(b) == 0 || len(b) >= maxFrame {
		return m, fmt.Errorf("empty message")
	}
	kind := b[0]
	b = b[1:]
	if kind == 16 || kind == 17 {
		if len(b) < 3 {
			return m, fmt.Errorf("short v3 header")
		}
		b = b[3:]
	}
	switch kind {
	case 7, 16:
		if len(b) < 12 {
			return m, fmt.Errorf("short direct message")
		}
		m.Kind = "direct"
		m.Sender = hex.EncodeToString(b[:6])
		if contact, ok := uniqueContact(st, m.Sender); ok {
			m.PeerKey = contact.Key
		}
		m.TextType = b[7]
		m.Timestamp = int64(binary.LittleEndian.Uint32(b[8:12]))
		b = b[12:]
		if m.TextType == 2 {
			if len(b) < 4 {
				return m, fmt.Errorf("short forwarded message")
			}
			b = b[4:]
		}
	case 8, 17:
		if len(b) < 7 {
			return m, fmt.Errorf("short channel message")
		}
		m.Kind = "channel"
		m.Channel = int(b[0])
		m.TextType = b[2]
		m.Timestamp = int64(binary.LittleEndian.Uint32(b[3:7]))
		b = b[7:]
		for _, ch := range st.Channels {
			if ch.Index == m.Channel {
				m.Binding = ch.Binding
			}
		}
	default:
		return m, fmt.Errorf("unsupported message type")
	}
	if len(b) == 0 || !utf8.Valid(b) || bytes.ContainsAny(b, "\x00") {
		return m, fmt.Errorf("invalid message text")
	}
	m.Text = string(b)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s\x00%d\x00%d\x00%s", m.IdentityKey, m.Kind, m.Sender, m.Channel, m.Binding, m.TextType, m.Timestamp, m.Text)))
	m.ID = hex.EncodeToString(sum[:])
	return m, nil
}

func splitText(text string, limit int) ([]string, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) || limit < 16 {
		return nil, fmt.Errorf("invalid outbound text")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty outbound text")
	}
	if len(text) <= limit {
		return []string{text}, nil
	}
	var parts []string
	for len(text) > 0 {
		n := min(len(text), limit-6)
		for n > 0 && n < len(text) && !utf8.RuneStart(text[n]) {
			n--
		}
		if n == 0 {
			return nil, fmt.Errorf("invalid text boundary")
		}
		parts = append(parts, text[:n])
		text = text[n:]
		if len(parts) > 3 {
			return nil, fmt.Errorf("reply exceeds three radio packets")
		}
	}
	for i := range parts {
		parts[i] = fmt.Sprintf("[%d/%d] %s", i+1, len(parts), parts[i])
	}
	return parts, nil
}
