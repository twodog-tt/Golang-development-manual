package mediaio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"
	"time"
)

type Segment struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Text    string `json:"text"`
}
type Transcript struct {
	Segments []Segment `json:"segments"`
}

func asrObject(raw []byte, fields []string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return nil, errors.New("invalid JSON size")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return nil, errors.New("object required")
	}
	out := map[string]json.RawMessage{}
	allowed := map[string]bool{}
	for _, f := range fields {
		allowed[f] = true
	}
	for d.More() {
		k, e := d.Token()
		if e != nil {
			return nil, e
		}
		key, ok := k.(string)
		if !ok || !allowed[key] || out[key] != nil {
			return nil, errors.New("unknown or duplicate field")
		}
		var v json.RawMessage
		if e = d.Decode(&v); e != nil {
			return nil, e
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, errors.New("null field")
		}
		out[key] = v
	}
	if _, e = d.Token(); e != nil {
		return nil, e
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	if len(out) != len(fields) {
		return nil, errors.New("missing field")
	}
	return out, nil
}

// parseTranscript requires every wire field; JSON zero values are not presence checks.
func parseTranscript(raw []byte) (Transcript, error) {
	root, err := asrObject(raw, []string{"segments"})
	if err != nil {
		return Transcript{}, err
	}
	var segments []json.RawMessage
	if err = json.Unmarshal(root["segments"], &segments); err != nil {
		return Transcript{}, err
	}
	if len(segments) == 0 || len(segments) > 256 {
		return Transcript{}, errors.New("invalid segment count")
	}
	out := Transcript{}
	for _, rawSegment := range segments {
		if _, err = asrObject(rawSegment, []string{"start_ms", "end_ms", "text"}); err != nil {
			return Transcript{}, err
		}
		var segment Segment
		if err = json.Unmarshal(rawSegment, &segment); err != nil {
			return Transcript{}, err
		}
		out.Segments = append(out.Segments, segment)
	}
	return out, nil
}

func validateSegments(segments []Segment, durationMS int64) error {
	last := int64(0)
	if len(segments) == 0 || len(segments) > 256 {
		return errors.New("invalid segment count")
	}
	for _, s := range segments {
		if s.StartMS < last || s.EndMS <= s.StartMS || s.EndMS > durationMS || s.Text == "" || len(s.Text) > 4096 {
			return errors.New("invalid segment time/text")
		}
		last = s.EndMS
	}
	return nil
}

// Transcribe sends a real multipart request to the LOCAL fixture, never a real ASR API.
// Network/status errors are returned without retry: generation/billing may be unknown.
func (c *LocalClient) Transcribe(ctx context.Context, audio []byte) (Transcript, error) {
	m, e := Inspect(ctx, audio, Limits{})
	if e != nil || m.MIME != "audio/wav" {
		return Transcript{}, errors.New("fixture requires valid WAV")
	}
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	headers := textproto.MIMEHeader{}
	headers.Set("Content-Disposition", `form-data; name="file"; filename="input.wav"`)
	headers.Set("Content-Type", "audio/wav")
	part, e := writer.CreatePart(headers)
	if e != nil {
		return Transcript{}, e
	}
	if _, e = part.Write(audio); e != nil {
		return Transcript{}, e
	}
	for _, pair := range [][2]string{{"model", "fixture-asr-v1"}, {"language", "zh"}, {"response_format", "fixture-segments-v1"}} {
		if e = writer.WriteField(pair[0], pair[1]); e != nil {
			return Transcript{}, e
		}
	}
	if e = writer.Close(); e != nil {
		return Transcript{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "POST", c.base+"/asr", &b)
	if e != nil {
		return Transcript{}, e
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, e := c.http.Do(req)
	if e != nil {
		return Transcript{}, fmt.Errorf("ASR result unknown: %w", e)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return Transcript{}, fmt.Errorf("ASR status %d; no automatic retry", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/json" {
		return Transcript{}, errors.New("ASR content type mismatch")
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if e != nil {
		return Transcript{}, e
	}
	if len(raw) > 64<<10 {
		return Transcript{}, errors.New("ASR output too large")
	}
	t, e := parseTranscript(raw)
	if e != nil {
		return Transcript{}, e
	}
	if e = validateSegments(t.Segments, m.Duration.Milliseconds()); e != nil {
		return Transcript{}, e
	}
	return t, nil
}

type RelayError struct {
	Started bool
	Cause   error
}

func (e *RelayError) Error() string {
	return fmt.Sprintf("audio relay started=%t: %v", e.Started, e.Cause)
}
func (e *RelayError) Unwrap() error { return e.Cause }

// RelayPCM accepts fixture audio/pcm with X-Audio-Bytes and fixed PCM16/24k/mono.
// The byte count detects a truncated local fixture, NOT semantic completion or billing.
// On Started error an HTTP handler must abort the response (ErrAbortHandler), not
// append JSON or retry/concatenate speech. Each downstream Write has a deadline.
func (c *LocalClient) RelayPCM(ctx context.Context, w http.ResponseWriter, text string, writeTimeout time.Duration) (err error) {
	started := false
	defer func() {
		if err != nil {
			err = &RelayError{started, err}
		}
	}()
	if text == "" || len(text) > 4096 || writeTimeout <= 0 || writeTimeout > time.Second {
		return errors.New("invalid TTS input/deadline")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	raw, _ := json.Marshal(map[string]string{"input": text, "contract": "fixture-pcm-v1"})
	req, e := http.NewRequestWithContext(ctx, "POST", c.base+"/tts", bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.http.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("TTS status %d", resp.StatusCode)
	}
	n, e := strconv.ParseInt(resp.Header.Get("X-Audio-Bytes"), 10, 64)
	if e != nil || n <= 0 || n > 128<<10 || n%2 != 0 || resp.Header.Get("Content-Type") != "audio/pcm" || resp.Header.Get("X-Audio-Format") != "pcm16le;rate=24000;channels=1" {
		return errors.New("invalid fixture audio headers")
	}
	ctl := http.NewResponseController(w)
	if e = ctl.SetWriteDeadline(time.Now().Add(writeTimeout)); e != nil {
		return fmt.Errorf("bounded downstream writer required: %w", e)
	}
	defer ctl.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "audio/pcm")
	w.Header().Set("X-Audio-Format", resp.Header.Get("X-Audio-Format"))
	// Keep downstream chunked: on a later upstream error, ErrAbortHandler
	// must leave HTTP framing incomplete, even if all declared PCM bytes arrived.
	w.Header().Del("Content-Length")
	w.Header().Set("X-Audio-Bytes", strconv.FormatInt(n, 10))
	buf := make([]byte, 4096)
	remaining := n
	for remaining > 0 {
		if e = ctx.Err(); e != nil {
			return e
		}
		read, readErr := resp.Body.Read(buf[:min(int64(len(buf)), remaining)])
		if read > 0 {
			if e = ctl.SetWriteDeadline(time.Now().Add(writeTimeout)); e != nil {
				return e
			}
			started = true
			written, e := w.Write(buf[:read])
			if e != nil {
				return e
			}
			if written != read {
				return io.ErrShortWrite
			}
			remaining -= int64(read)
			if e = ctl.Flush(); e != nil {
				return e
			}
		}
		if readErr != nil {
			if readErr == io.EOF && remaining == 0 {
				break
			}
			return io.ErrUnexpectedEOF
		}
	}
	var extra [1]byte
	extraN, extraErr := resp.Body.Read(extra[:])
	if extraN != 0 || extraErr != io.EOF {
		return errors.New("audio length mismatch or transport incomplete")
	}
	return nil
}

type ChunkTranscript struct {
	ID                   string
	OffsetMS, DurationMS int64
	Segments             []Segment
}

// Merge deduplicates exact retries and exact overlapping timestamp/text segments.
// Ambiguous overlap, conflicting IDs or gaps require review instead of guessed text.
func Merge(totalMS int64, chunks []ChunkTranscript) ([]Segment, error) {
	if totalMS < 1 || totalMS > 86400000 || len(chunks) == 0 || len(chunks) > 128 {
		return nil, errors.New("invalid recording")
	}
	byID := map[string]string{}
	unique := []ChunkTranscript{}
	for _, c := range chunks {
		if c.ID == "" || c.OffsetMS < 0 || c.DurationMS <= 0 || c.OffsetMS > totalMS || c.DurationMS > totalMS-c.OffsetMS {
			return nil, errors.New("invalid chunk range")
		}
		if e := validateSegments(c.Segments, c.DurationMS); e != nil {
			return nil, e
		}
		b, _ := json.Marshal(c)
		key := string(b)
		if prev, ok := byID[c.ID]; ok {
			if prev != key {
				return nil, errors.New("conflicting retry chunk")
			}
			continue
		}
		byID[c.ID] = key
		unique = append(unique, c)
	}
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].OffsetMS == unique[j].OffsetMS {
			return unique[i].ID < unique[j].ID
		}
		return unique[i].OffsetMS < unique[j].OffsetMS
	})
	covered := int64(0)
	segments := []Segment{}
	for _, c := range unique {
		if c.OffsetMS > covered {
			return nil, errors.New("recording gap")
		}
		covered = max(covered, c.OffsetMS+c.DurationMS)
		for _, s := range c.Segments {
			s.StartMS += c.OffsetMS
			s.EndMS += c.OffsetMS
			segments = append(segments, s)
		}
	}
	if covered != totalMS {
		return nil, errors.New("incomplete recording")
	}
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].StartMS != segments[j].StartMS {
			return segments[i].StartMS < segments[j].StartMS
		}
		return segments[i].EndMS < segments[j].EndMS
	})
	out := []Segment{}
	for _, s := range segments {
		if len(out) > 0 {
			last := out[len(out)-1]
			if last == s {
				continue
			}
			if s.StartMS < last.EndMS {
				return nil, errors.New("ambiguous segment overlap")
			}
		}
		out = append(out, s)
	}
	return out, nil
}
