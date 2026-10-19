package mediaio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func local(t *testing.T, h http.HandlerFunc) *LocalClient {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, e := NewLocalClient(s.URL)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.Close)
	return c
}
func TestASRMultipartContract(t *testing.T) {
	var calls atomic.Int32
	client := local(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/asr" || r.Header.Get("Authorization") != "" {
			t.Error("invalid local request")
		}
		if e := r.ParseMultipartForm(1024); e != nil {
			t.Error(e)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		for k, v := range map[string]string{"model": "fixture-asr-v1", "language": "zh", "response_format": "fixture-segments-v1"} {
			if r.FormValue(k) != v {
				t.Error(k)
			}
		}
		f, h, e := r.FormFile("file")
		if e != nil {
			t.Error(e)
			w.WriteHeader(400)
			return
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		if !bytes.Equal(data, WAV(2400)) || h.Header.Get("Content-Type") != "audio/wav" {
			t.Error("incorrect file part")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Transcript{Segments: []Segment{{0, 100, "本地转写"}}})
	})
	got, e := client.Transcribe(context.Background(), WAV(2400))
	if e != nil || len(got.Segments) != 1 || calls.Load() != 1 {
		t.Fatal(got, calls.Load(), e)
	}
}
func TestASRFailuresBoundedAndNeverRetried(t *testing.T) {
	for _, mode := range []string{"status", "invalid-json", "timestamp", "unknown-field", "empty", "trailing", "oversize", "missing-start", "null-start", "duplicate-start", "duplicate-root", "case-alias", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			entered := make(chan struct{})
			client := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch mode {
				case "status":
					w.WriteHeader(503)
				case "invalid-json":
					_, _ = io.WriteString(w, "{")
				case "timestamp":
					_, _ = io.WriteString(w, `{"segments":[{"start_ms":0,"end_ms":101,"text":"x"}]}`)
				case "unknown-field":
					_, _ = io.WriteString(w, `{"segments":[],"secret":"x"}`)
				case "empty":
					_, _ = io.WriteString(w, `{"segments":[]}`)
				case "trailing":
					_, _ = io.WriteString(w, `{"segments":[]} {}`)
				case "oversize":
					_, _ = io.WriteString(w, strings.Repeat("x", 65537))
				case "missing-start":
					_, _ = io.WriteString(w, `{"segments":[{"end_ms":100,"text":"x"}]}`)
				case "null-start":
					_, _ = io.WriteString(w, `{"segments":[{"start_ms":null,"end_ms":100,"text":"x"}]}`)
				case "duplicate-start":
					_, _ = io.WriteString(w, `{"segments":[{"start_ms":0,"start_\u006ds":0,"end_ms":100,"text":"x"}]}`)
				case "duplicate-root":
					_, _ = io.WriteString(w, `{"segments":[],"segments":[{"start_ms":0,"end_ms":100,"text":"x"}]}`)
				case "case-alias":
					_, _ = io.WriteString(w, `{"segments":[{"Start_ms":0,"end_ms":100,"text":"x"}]}`)
				case "cancel":
					_, _ = io.Copy(io.Discard, r.Body)
					close(entered)
					<-r.Context().Done()
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if mode == "cancel" {
				go func() {
					select {
					case <-entered:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			_, e := client.Transcribe(ctx, WAV(2400))
			if e == nil || calls.Load() != 1 {
				t.Fatal(calls.Load(), e)
			}
		})
	}
}

type deadlineRecorder struct{ *httptest.ResponseRecorder }

func (w deadlineRecorder) SetWriteDeadline(time.Time) error { return nil }
func (w deadlineRecorder) FlushError() error                { w.ResponseRecorder.Flush(); return nil }
func audioHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "audio/pcm")
	w.Header().Set("X-Audio-Format", "pcm16le;rate=24000;channels=1")
	w.Header().Set("X-Audio-Bytes", "6")
}
func TestRelayPCMIsBinaryAndPreservesBytes(t *testing.T) {
	raw := []byte{0xff, 0xfe, 0, 1, 2, 3}
	c := local(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		audioHeaders(w)
		_, _ = w.Write(raw[:2])
		w.(http.Flusher).Flush()
		_, _ = w.Write(raw[2:])
	})
	rec := deadlineRecorder{httptest.NewRecorder()}
	if e := c.RelayPCM(context.Background(), rec, "hi", 100*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(rec.Body.Bytes(), raw) || rec.Header().Get("Content-Type") != "audio/pcm" {
		t.Fatal("audio treated as text/SSE", rec.Body.String())
	}
}
func TestRelayFailuresAndPartialOutputNeverRetry(t *testing.T) {
	for _, mode := range []string{"status", "mime", "format", "length", "missing-length", "drop", "extra"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				audioHeaders(w)
				switch mode {
				case "status":
					w.WriteHeader(500)
				case "mime":
					w.Header().Set("Content-Type", "text/event-stream")
				case "format":
					w.Header().Set("X-Audio-Format", "wrong")
				case "length":
					w.Header().Set("X-Audio-Bytes", "5")
				case "missing-length":
					w.Header().Del("X-Audio-Bytes")
				case "drop":
					_, _ = w.Write([]byte{1, 2})
				case "extra":
					_, _ = w.Write(make([]byte, 8))
				}
			})
			rec := deadlineRecorder{httptest.NewRecorder()}
			e := c.RelayPCM(context.Background(), rec, "hi", 100*time.Millisecond)
			var relay *RelayError
			if !errors.As(e, &relay) || calls.Load() != 1 {
				t.Fatal(calls.Load(), e)
			}
			if mode == "drop" && (!relay.Started || rec.Body.Len() != 2) {
				t.Fatal("lost partial state", e)
			}
		})
	}
}

type pipeWriter struct {
	conn   net.Conn
	header http.Header
}

func (w *pipeWriter) Header() http.Header                { return w.header }
func (w *pipeWriter) WriteHeader(int)                    {}
func (w *pipeWriter) Write(b []byte) (int, error)        { return w.conn.Write(b) }
func (w *pipeWriter) SetWriteDeadline(t time.Time) error { return w.conn.SetWriteDeadline(t) }
func (w *pipeWriter) FlushError() error                  { return nil }
func TestSlowDownstreamTimesOutAndCancelsUpstream(t *testing.T) {
	done := make(chan struct{})
	c := local(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		audioHeaders(w)
		w.Header().Set("X-Audio-Bytes", "8192")
		_, _ = w.Write(make([]byte, 4096))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(done)
	})
	writer, reader := net.Pipe()
	defer writer.Close()
	defer reader.Close()
	out := &pipeWriter{conn: writer, header: http.Header{}}
	started := time.Now()
	e := c.RelayPCM(context.Background(), out, "hi", 25*time.Millisecond)
	if e == nil || time.Since(started) > time.Second {
		t.Fatal("write not bounded", e)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("upstream not canceled")
	}
}
func TestRelayCancellationAndUnsupportedWriter(t *testing.T) {
	done := make(chan struct{})
	c := local(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		audioHeaders(w)
		w.Header().Set("X-Audio-Bytes", "8192")
		_, _ = w.Write(make([]byte, 4096))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(done)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	e := c.RelayPCM(ctx, deadlineRecorder{httptest.NewRecorder()}, "hi", 100*time.Millisecond)
	if e == nil {
		t.Fatal("canceled stream passed")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel not propagated")
	}
	c2 := local(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		audioHeaders(w)
		_, _ = w.Write(make([]byte, 6))
	})
	if e = c2.RelayPCM(context.Background(), httptest.NewRecorder(), "hi", time.Second); e == nil {
		t.Fatal("unbounded writer accepted")
	}
}
func TestMergeRetriesOffsetsAndOverlap(t *testing.T) {
	a := ChunkTranscript{ID: "a", OffsetMS: 0, DurationMS: 1500, Segments: []Segment{{0, 1000, "hello"}, {1000, 1500, "world"}}}
	b := ChunkTranscript{ID: "b", OffsetMS: 1000, DurationMS: 1000, Segments: []Segment{{0, 500, "world"}, {500, 1000, "end"}}}
	out, e := Merge(2000, []ChunkTranscript{b, a, a})
	if e != nil || len(out) != 3 || out[2].StartMS != 1500 {
		t.Fatal(out, e)
	}
}
func TestMergeConflictGapAndTimestampFailures(t *testing.T) {
	base := ChunkTranscript{ID: "a", DurationMS: 1000, Segments: []Segment{{0, 1000, "x"}}}
	for _, mode := range []string{"conflicting-retry", "gap", "incomplete", "overlap", "timestamp", "negative"} {
		t.Run(mode, func(t *testing.T) {
			a := base
			a.Segments = append([]Segment(nil), base.Segments...)
			chunks := []ChunkTranscript{a}
			total := int64(1000)
			switch mode {
			case "conflicting-retry":
				b := a
				b.DurationMS = 999
				chunks = append(chunks, b)
			case "gap":
				chunks[0].OffsetMS = 10
				chunks[0].DurationMS = 990
				chunks[0].Segments[0].EndMS = 990
			case "incomplete":
				total = 2000
			case "overlap":
				b := ChunkTranscript{ID: "b", OffsetMS: 500, DurationMS: 500, Segments: []Segment{{0, 500, "different"}}}
				chunks = append(chunks, b)
			case "timestamp":
				chunks[0].Segments[0].EndMS = 1001
			case "negative":
				chunks[0].OffsetMS = -1
			}
			if _, e := Merge(total, chunks); e == nil {
				t.Fatal("invalid merge passed")
			}
		})
	}
}

func TestHTTPAbortMakesTruncatedAndExtraAudioObservableToClient(t *testing.T) {
	for _, mode := range []string{"drop", "extra"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				audioHeaders(w)
				n := 2
				if mode == "extra" {
					n = 8
				}
				_, _ = w.Write(make([]byte, n))
			})
			result := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "6") // inherited middleware header must be removed
				e := c.RelayPCM(r.Context(), w, "hi", time.Second)
				result <- e
				if e != nil {
					var relay *RelayError
					if errors.As(e, &relay) && relay.Started {
						panic(http.ErrAbortHandler)
					}
					http.Error(w, "bad audio", 502)
				}
			}))
			defer server.Close()
			hc := server.Client()
			hc.Timeout = 2 * time.Second
			resp, e := hc.Get(server.URL)
			if e != nil {
				t.Fatal(e)
			}
			_, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			relayErr := <-result
			if relayErr == nil || readErr == nil || calls.Load() != 1 {
				t.Fatal("client saw false success", relayErr, readErr, calls.Load())
			}
			if resp.Header.Get("Content-Length") != "" {
				t.Fatal("completion was promised before validation")
			}
		})
	}
}
