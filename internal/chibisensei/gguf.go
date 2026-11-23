package chibisensei

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
)

// GGUFInfo is the verified header of a GGUF file.
type GGUFInfo struct {
	Version      uint32 `json:"version"`
	Tensors      uint64 `json:"tensors"`
	Architecture string `json:"architecture"`
}

// ErrGGUFInvalid wraps every refusal of ReadGGUFInfo.
var ErrGGUFInvalid = errors.New("chibisensei: invalid GGUF file")

const (
	ggufVersion       = 3
	ggufMaxTensors    = 1 << 24
	ggufMaxKV         = 1 << 16
	ggufMaxKeyBytes   = 1 << 10
	ggufMaxValueBytes = 1 << 20 // one skipped string value
	ggufMaxArrayLen   = 1 << 24
	ggufMaxScanBytes  = 256 << 20 // metadata read before general.architecture must appear
	ggufTypeString    = 8
	ggufTypeArray     = 9
	ggufArchKey       = "general.architecture"
)

var (
	ggufScalarBytes = map[uint32]uint64{0: 1, 1: 1, 2: 2, 3: 2, 4: 4, 5: 4, 6: 4, 7: 1, 10: 8, 11: 8, 12: 8}
	ggufArchPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
)

// ReadGGUFInfo reads only the header and the metadata up to general.architecture. The file comes from a remote
// machine, so every length is capped: keys 1 KiB, skipped strings 1 MiB, arrays 2^24 values, no nested arrays,
// 2^16 keys, 2^24 tensors and 256 MiB of metadata in total. Only version 3 is accepted.
func ReadGGUFInfo(path string) (GGUFInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return GGUFInfo{}, err
	}
	defer f.Close()
	return readGGUF(f)
}

func readGGUFInRoot(root *os.Root, rel string) (GGUFInfo, error) {
	f, err := root.Open(rel)
	if err != nil {
		return GGUFInfo{}, err
	}
	defer f.Close()
	return readGGUF(f)
}

func ggufErr(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrGGUFInvalid}, a...)...)
}

type ggufReader struct{ r *bufio.Reader }

func (g ggufReader) read(p []byte) error {
	if _, err := io.ReadFull(g.r, p); err != nil {
		return ggufErr("truncated")
	}
	return nil
}

func (g ggufReader) u32() (uint32, error) {
	var b [4]byte
	if err := g.read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[:]), nil
}

func (g ggufReader) u64() (uint64, error) {
	var b [8]byte
	if err := g.read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func (g ggufReader) discard(n uint64) error {
	if n > ggufMaxScanBytes {
		return ggufErr("value too large")
	}
	if _, err := g.r.Discard(int(n)); err != nil {
		return ggufErr("truncated")
	}
	return nil
}

func (g ggufReader) str(limit uint64) (string, error) {
	n, err := g.u64()
	if err != nil {
		return "", err
	}
	if n > limit {
		return "", ggufErr("string of %d bytes exceeds the limit", n)
	}
	b := make([]byte, n)
	if err := g.read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func (g ggufReader) skipValue(typ uint32, inArray bool) error {
	if n, ok := ggufScalarBytes[typ]; ok {
		return g.discard(n)
	}
	switch typ {
	case ggufTypeString:
		n, err := g.u64()
		if err != nil {
			return err
		}
		if n > ggufMaxValueBytes {
			return ggufErr("string of %d bytes exceeds the limit", n)
		}
		return g.discard(n)
	case ggufTypeArray:
		if inArray {
			return ggufErr("nested arrays are not supported")
		}
		elem, err := g.u32()
		if err != nil {
			return err
		}
		count, err := g.u64()
		if err != nil {
			return err
		}
		if count > ggufMaxArrayLen {
			return ggufErr("array of %d values exceeds the limit", count)
		}
		if n, ok := ggufScalarBytes[elem]; ok {
			return g.discard(count * n)
		}
		for i := uint64(0); i < count; i++ {
			if err := g.skipValue(elem, true); err != nil {
				return err
			}
		}
		return nil
	}
	return ggufErr("unknown value type %d", typ)
}

func readGGUF(src io.Reader) (GGUFInfo, error) {
	g := ggufReader{r: bufio.NewReaderSize(io.LimitReader(src, ggufMaxScanBytes), 64<<10)}
	var magic [4]byte
	if err := g.read(magic[:]); err != nil {
		return GGUFInfo{}, err
	}
	if string(magic[:]) != "GGUF" {
		return GGUFInfo{}, ggufErr("wrong magic")
	}
	version, err := g.u32()
	if err != nil {
		return GGUFInfo{}, err
	}
	if version != ggufVersion {
		return GGUFInfo{}, ggufErr("version %d is not supported", version)
	}
	tensors, err := g.u64()
	if err != nil {
		return GGUFInfo{}, err
	}
	if tensors == 0 || tensors > ggufMaxTensors {
		return GGUFInfo{}, ggufErr("tensor count %d out of range", tensors)
	}
	kvs, err := g.u64()
	if err != nil {
		return GGUFInfo{}, err
	}
	if kvs > ggufMaxKV {
		return GGUFInfo{}, ggufErr("%d metadata keys exceed the limit", kvs)
	}
	for i := uint64(0); i < kvs; i++ {
		key, err := g.str(ggufMaxKeyBytes)
		if err != nil {
			return GGUFInfo{}, err
		}
		typ, err := g.u32()
		if err != nil {
			return GGUFInfo{}, err
		}
		if key != ggufArchKey {
			if err := g.skipValue(typ, false); err != nil {
				return GGUFInfo{}, err
			}
			continue
		}
		if typ != ggufTypeString {
			return GGUFInfo{}, ggufErr("%s is not a string", ggufArchKey)
		}
		arch, err := g.str(64)
		if err != nil {
			return GGUFInfo{}, err
		}
		if !ggufArchPattern.MatchString(arch) {
			return GGUFInfo{}, ggufErr("invalid architecture name")
		}
		return GGUFInfo{Version: version, Tensors: tensors, Architecture: arch}, nil
	}
	return GGUFInfo{}, ggufErr("%s is missing", ggufArchKey)
}
