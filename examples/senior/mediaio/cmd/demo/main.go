package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"sync/atomic"
	m "td-homework/examples/senior/mediaio"
	"time"
)

func run() error {
	wav := m.WAV(2400)
	wire, e := m.MarshalMessage([]m.Part{{Type: "text", Text: "本地媒体示例"}, {Type: "input_audio", Audio: &m.InputAudio{Data: base64.StdEncoding.EncodeToString(wav), Format: "wav"}}})
	if e != nil {
		return e
	}
	var asrCalls, ttsCalls atomic.Int32
	var relayErr error
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		u, e := m.Receive(w, r, m.Limits{}, nil)
		if e != nil {
			http.Error(w, "invalid upload", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u.Media)
	})
	mux.HandleFunc("/asr", func(w http.ResponseWriter, r *http.Request) {
		asrCalls.Add(1)
		if e := r.ParseMultipartForm(1024); e != nil {
			http.Error(w, "bad multipart", 400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, h, e := r.FormFile("file")
		if e != nil {
			http.Error(w, "missing audio", 400)
			return
		}
		defer f.Close()
		b, e := io.ReadAll(f)
		if e != nil || !bytes.Equal(b, wav) || h.Header.Get("Content-Type") != "audio/wav" || r.FormValue("model") != "fixture-asr-v1" || r.FormValue("language") != "zh" || r.FormValue("response_format") != "fixture-segments-v1" {
			http.Error(w, "bad contract", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.Transcript{Segments: []m.Segment{{StartMS: 0, EndMS: 100, Text: "本地转写"}}})
	})
	mux.HandleFunc("/tts", func(w http.ResponseWriter, r *http.Request) {
		ttsCalls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "audio/pcm")
		w.Header().Set("X-Audio-Format", "pcm16le;rate=24000;channels=1")
		w.Header().Set("X-Audio-Bytes", "6")
		_, _ = w.Write([]byte{0xff, 0xfe, 0, 1, 2, 3})
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()
	client, e := m.NewLocalClient(upstream.URL)
	if e != nil {
		return e
	}
	defer client.Close()
	// Real downstream HTTP gives ResponseController actual write deadlines/Flush.
	relayDone := make(chan error, 1)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := client.RelayPCM(r.Context(), w, "fixture speech", time.Second)
		relayDone <- e
		if e != nil {
			var re *m.RelayError
			if errors.As(e, &re) && re.Started {
				panic(http.ErrAbortHandler)
			}
			http.Error(w, "relay failed", 502)
		}
	}))
	defer downstream.Close()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="clip.wav"`)
	h.Set("Content-Type", "audio/wav")
	part, e := mw.CreatePart(h)
	if e != nil {
		return e
	}
	if _, e = part.Write(wav); e != nil {
		return e
	}
	if e = mw.Close(); e != nil {
		return e
	}
	hc := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer hc.CloseIdleConnections()
	request, e := http.NewRequest("POST", upstream.URL+"/upload", &body)
	if e != nil {
		return e
	}
	request.Header.Set("Content-Type", mw.FormDataContentType())
	response, e := hc.Do(request)
	if e != nil {
		return e
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != 200 {
		return errors.New("upload failed")
	}
	transcript, e := client.Transcribe(context.Background(), wav)
	if e != nil {
		return e
	}
	response, e = hc.Get(downstream.URL)
	if e != nil {
		return e
	}
	audio, e := io.ReadAll(response.Body)
	_ = response.Body.Close()
	relayErr = <-relayDone
	if e != nil || relayErr != nil || !bytes.Equal(audio, []byte{0xff, 0xfe, 0, 1, 2, 3}) {
		return errors.New("binary relay failed")
	}
	chunks := []m.ChunkTranscript{{ID: "part1", DurationMS: 100, Segments: transcript.Segments}}
	merged, e := m.Merge(100, append(chunks, chunks...))
	if e != nil {
		return e
	}
	voice, e := m.NewVoice("session-1")
	if e != nil {
		return e
	}
	if e = voice.Start("response-1", "item-1"); e != nil {
		return e
	}
	if e = voice.Append("session-1", "response-1", 1, 0, make([]byte, 48000)); e != nil {
		return e
	}
	if e = voice.Played("session-1", "response-1", 1, 24000); e != nil {
		return e
	}
	cut, e := voice.Interrupt()
	if e != nil {
		return e
	}
	late := voice.Append("session-1", "response-1", 1, 1, make([]byte, 48))
	voice.Disconnect()
	same := voice.Reconnect("session-1")
	if e = voice.Reconnect("session-2"); e != nil {
		return e
	}
	pass := len(wire) > 0 && len(merged) == 1 && cut.AudioEndMS == 500 && late != nil && same != nil && asrCalls.Load() == 1 && ttsCalls.Load() == 1
	if !pass {
		return errors.New("media fixture invariant failed")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": pass, "wire_nested": true, "upload_checked": true, "asr_calls": asrCalls.Load(), "tts_calls": ttsCalls.Load(), "binary_bytes": len(audio), "merged_segments": len(merged), "truncate_ms": cut.AudioEndMS, "late_audio_rejected": late != nil, "same_session_resume_rejected": same != nil, "new_epoch": voice.Epoch()})
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
