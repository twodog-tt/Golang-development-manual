package mediaio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"time"
)

type Limits struct {
	BodyBytes, FileBytes int64
	Pixels               int
	Duration, Timeout    time.Duration
}

func (l Limits) defaults() Limits {
	if l.BodyBytes == 0 {
		l.BodyBytes = 256 << 10
	}
	if l.FileBytes == 0 {
		l.FileBytes = 128 << 10
	}
	if l.Pixels == 0 {
		l.Pixels = 65536
	}
	if l.Duration == 0 {
		l.Duration = 3 * time.Second
	}
	if l.Timeout == 0 {
		l.Timeout = time.Second
	}
	return l
}
func (l Limits) valid() bool {
	return l.BodyBytes > 0 && l.BodyBytes <= 1<<20 && l.FileBytes > 0 && l.FileBytes <= l.BodyBytes && l.Pixels > 0 && l.Pixels <= 1<<20 && l.Duration > 0 && l.Duration <= time.Minute && l.Timeout > 0 && l.Timeout <= 5*time.Second
}

type Media struct {
	MIME          string
	Width, Height int
	Duration      time.Duration
}

// Inspect supports PCM16 mono 24kHz WAV and PNG/JPEG only. Bounded decoding is
// synchronous; it is not a sandbox for arbitrary untrusted codecs/ffmpeg.
func Inspect(ctx context.Context, b []byte, limits Limits) (Media, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	l := limits.defaults()
	if !l.valid() || int64(len(b)) > l.FileBytes || len(b) < 12 {
		return Media{}, errors.New("invalid media limits/size")
	}
	if e := ctx.Err(); e != nil {
		return Media{}, e
	}
	if string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE" {
		if int64(binary.LittleEndian.Uint32(b[4:8]))+8 != int64(len(b)) {
			return Media{}, errors.New("WAV length mismatch")
		}
		seenFmt, seenData := false, false
		dataBytes := 0
		for at := 12; at < len(b); {
			if e := ctx.Err(); e != nil {
				return Media{}, e
			}
			if at+8 > len(b) {
				return Media{}, errors.New("truncated WAV chunk")
			}
			size := int64(binary.LittleEndian.Uint32(b[at+4 : at+8]))
			end := int64(at) + 8 + size
			if end > int64(len(b)) {
				return Media{}, errors.New("invalid WAV chunk size")
			}
			p := b[at+8 : int(end)]
			switch string(b[at : at+4]) {
			case "fmt ":
				if seenFmt || size != 16 {
					return Media{}, errors.New("unsupported WAV fmt")
				}
				seenFmt = true
				if binary.LittleEndian.Uint16(p) != 1 || binary.LittleEndian.Uint16(p[2:]) != 1 || binary.LittleEndian.Uint32(p[4:]) != 24000 || binary.LittleEndian.Uint32(p[8:]) != 48000 || binary.LittleEndian.Uint16(p[12:]) != 2 || binary.LittleEndian.Uint16(p[14:]) != 16 {
					return Media{}, errors.New("PCM16 mono 24kHz required")
				}
			case "data":
				if seenData || !seenFmt || size == 0 || size%2 != 0 {
					return Media{}, errors.New("invalid WAV data")
				}
				seenData = true
				dataBytes = int(size)
			default:
				return Media{}, errors.New("unsupported WAV chunk")
			}
			at = int(end) + (int(size) % 2)
			if at > len(b) {
				return Media{}, errors.New("missing WAV padding")
			}
		}
		if !seenFmt || !seenData {
			return Media{}, errors.New("WAV missing fmt/data")
		}
		d := time.Duration(dataBytes) * time.Second / 48000
		if d > l.Duration {
			return Media{}, errors.New("audio duration exceeded")
		}
		return Media{MIME: "audio/wav", Duration: d}, ctx.Err()
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "png" && format != "jpeg") {
		return Media{}, errors.New("unsupported or corrupt image")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > l.Pixels/cfg.Height {
		return Media{}, errors.New("pixel budget exceeded")
	}
	img, format, e := image.Decode(bytes.NewReader(b))
	if e != nil || img.Bounds().Dx() != cfg.Width || img.Bounds().Dy() != cfg.Height {
		return Media{}, errors.New("image decode failed")
	}
	if e = ctx.Err(); e != nil {
		return Media{}, e
	}
	return Media{MIME: "image/" + format, Width: cfg.Width, Height: cfg.Height}, nil
}

type Inspector func(context.Context, []byte, Limits) (Media, error)
type Upload struct {
	Data  []byte
	Media Media
}

// Receive owns multipart cleanup on every return. A custom inspector is trusted,
// bounded and MUST honor ctx (a real external probe needs subprocess isolation).
func Receive(w http.ResponseWriter, r *http.Request, limits Limits, probe Inspector) (Upload, error) {
	l := limits.defaults()
	if r.Method != http.MethodPost {
		return Upload{}, errors.New("POST required")
	}
	if !l.valid() {
		return Upload{}, errors.New("invalid upload limits")
	}
	ctx, cancel := context.WithTimeout(r.Context(), l.Timeout)
	defer cancel()
	if probe == nil {
		probe = Inspect
	}
	// Socket deadlines bound blocking multipart reads on a real HTTP server.
	ctl := http.NewResponseController(w)
	if e := ctl.SetReadDeadline(time.Now().Add(l.Timeout)); e != nil && !errors.Is(e, http.ErrNotSupported) {
		return Upload{}, e
	}
	r.Body = http.MaxBytesReader(w, r.Body, l.BodyBytes)
	defer r.Body.Close()
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if e := r.ParseMultipartForm(1024); e != nil {
		return Upload{}, e
	}
	if e := ctx.Err(); e != nil {
		return Upload{}, e
	}
	if len(r.MultipartForm.Value) != 0 || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		return Upload{}, errors.New("one file field required")
	}
	h := r.MultipartForm.File["file"][0]
	f, e := h.Open()
	if e != nil {
		return Upload{}, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, l.FileBytes+1))
	if e != nil {
		return Upload{}, e
	}
	if int64(len(b)) > l.FileBytes {
		return Upload{}, errors.New("file size exceeded")
	}
	m, e := probe(ctx, b, l)
	if e != nil {
		return Upload{}, e
	}
	if e = ctx.Err(); e != nil {
		return Upload{}, e
	}
	declared, _, e := mime.ParseMediaType(h.Header.Get("Content-Type"))
	if e != nil || declared != m.MIME {
		return Upload{}, errors.New("declared MIME does not match decoded media")
	}
	return Upload{b, m}, nil
}

// WAV creates silent bounded PCM data for fixtures; no microphone or model involved.
func WAV(samples int) []byte {
	if samples < 1 || samples > 48000 {
		return nil
	}
	b := make([]byte, 44+samples*2)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 24000)
	binary.LittleEndian.PutUint32(b[28:], 48000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(samples*2))
	return b
}
