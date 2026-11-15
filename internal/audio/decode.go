package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path"

	"github.com/hajimehoshi/go-mp3"
)

// decoder reads a sound file as interleaved stereo float32 frames at the
// file's own sample rate. Long files are decoded as they play.
type decoder interface {
	read(dst []float32) int // fills whole frames, returns how many; 0 at the end
	rewind() error
	rate() int
}

func open(fsys fs.FS, name string) (decoder, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return nil, fmt.Errorf("%s: not seekable", name)
	}
	switch path.Ext(name) {
	case ".mp3":
		d, err := mp3.NewDecoder(rs)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return &mp3Decoder{d: d}, nil
	case ".wav":
		d, err := newWav(rs)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return d, nil
	}
	return nil, fmt.Errorf("%s: unknown format", name)
}

// mp3Decoder: go-mp3 always gives 16-bit stereo.
type mp3Decoder struct {
	d   *mp3.Decoder
	raw []byte
}

func (m *mp3Decoder) rate() int     { return m.d.SampleRate() }
func (m *mp3Decoder) rewind() error { _, err := m.d.Seek(0, io.SeekStart); return err }

func (m *mp3Decoder) read(dst []float32) int {
	frames := len(dst) / 2
	if cap(m.raw) < frames*4 {
		m.raw = make([]byte, frames*4)
	}
	n, _ := io.ReadFull(m.d, m.raw[:frames*4])
	n /= 4
	for i := range n * 2 {
		dst[i] = float32(int16(binary.LittleEndian.Uint16(m.raw[i*2:]))) / 32768
	}
	return n
}

// wavDecoder reads PCM (8 to 32 bits) or float WAV, mono or stereo.
type wavDecoder struct {
	r              io.ReadSeeker
	start, size    int64 // the data chunk
	pos            int64
	sampleRate     int
	channels, bits int
	float          bool
	raw            []byte
}

func newWav(r io.ReadSeeker) (*wavDecoder, error) {
	var head [12]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	if string(head[0:4]) != "RIFF" || string(head[8:12]) != "WAVE" {
		return nil, errors.New("not a WAV file")
	}
	w := &wavDecoder{r: r}
	at := int64(12)
	for {
		var ch [8]byte
		if _, err := io.ReadFull(r, ch[:]); err != nil {
			return nil, errors.New("no data chunk")
		}
		id, size := string(ch[0:4]), int64(binary.LittleEndian.Uint32(ch[4:]))
		at += 8
		switch id {
		case "fmt ":
			b := make([]byte, size)
			if _, err := io.ReadFull(r, b); err != nil || size < 16 {
				return nil, errors.New("bad fmt chunk")
			}
			format := binary.LittleEndian.Uint16(b[0:])
			w.channels = int(binary.LittleEndian.Uint16(b[2:]))
			w.sampleRate = int(binary.LittleEndian.Uint32(b[4:]))
			w.bits = int(binary.LittleEndian.Uint16(b[14:]))
			if format == 0xFFFE && size >= 26 { // extensible: the real format follows
				format = binary.LittleEndian.Uint16(b[24:])
			}
			w.float = format == 3
			if format != 1 && format != 3 {
				return nil, fmt.Errorf("unsupported WAV format %d", format)
			}
			if w.channels < 1 || w.channels > 2 || w.bits%8 != 0 || w.bits < 8 || w.bits > 32 || w.sampleRate <= 0 {
				return nil, fmt.Errorf("unsupported WAV: %d ch, %d bits", w.channels, w.bits)
			}
			if w.float && w.bits != 32 {
				return nil, errors.New("unsupported float WAV")
			}
		case "data":
			if w.channels == 0 {
				return nil, errors.New("data before fmt")
			}
			w.start, w.size = at, size
			return w, nil
		default:
			if _, err := r.Seek(size+size%2, io.SeekCurrent); err != nil {
				return nil, err
			}
		}
		at += size + size%2
	}
}

func (w *wavDecoder) rate() int { return w.sampleRate }

func (w *wavDecoder) rewind() error {
	w.pos = 0
	_, err := w.r.Seek(w.start, io.SeekStart)
	return err
}

func (w *wavDecoder) read(dst []float32) int {
	if w.pos == 0 {
		if err := w.rewind(); err != nil {
			return 0
		}
	}
	bps := w.bits / 8
	frameSize := bps * w.channels
	frames := min(int64(len(dst)/2), (w.size-w.pos)/int64(frameSize))
	if frames <= 0 {
		return 0
	}
	need := int(frames) * frameSize
	if cap(w.raw) < need {
		w.raw = make([]byte, need)
	}
	n, _ := io.ReadFull(w.r, w.raw[:need])
	w.pos += int64(n)
	got := n / frameSize
	for i := range got {
		for c := range 2 {
			s := w.sample(w.raw[(i*w.channels+min(c, w.channels-1))*bps:])
			dst[i*2+c] = s
		}
	}
	return got
}

func (w *wavDecoder) sample(b []byte) float32 {
	switch {
	case w.float:
		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	case w.bits == 8: // unsigned
		return (float32(b[0]) - 128) / 128
	}
	var v int32
	for i := range w.bits / 8 {
		v |= int32(b[i]) << (32 - w.bits + 8*i)
	}
	return float32(v) / (1 << 31)
}

// decodeAll reads a whole file, resampled to the mixer's rate.
func decodeAll(fsys fs.FS, name string) ([]float32, error) {
	d, err := open(fsys, name)
	if err != nil {
		return nil, err
	}
	var raw []float32
	buf := make([]float32, 8192)
	for {
		n := d.read(buf)
		if n == 0 {
			break
		}
		raw = append(raw, buf[:n*2]...)
	}
	return resample(raw, d.rate()), nil
}

// resample converts interleaved stereo from rate to sampleRate, linearly.
func resample(in []float32, rate int) []float32 {
	if rate == sampleRate {
		return in
	}
	frames := len(in) / 2
	step := float64(rate) / sampleRate
	out := make([]float32, 0, int(float64(frames)/step)*2+2)
	for t := 0.0; int(t)+1 < frames; t += step {
		i, f := int(t), float32(t-math.Floor(t))
		for c := range 2 {
			out = append(out, in[i*2+c]*(1-f)+in[i*2+2+c]*f)
		}
	}
	return out
}
