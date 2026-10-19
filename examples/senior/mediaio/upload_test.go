package mediaio

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func multipartFile(t *testing.T, b []byte, typ string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="input.bin"`)
	h.Set("Content-Type", typ)
	p, e := w.CreatePart(h)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Write(b); e != nil {
		t.Fatal(e)
	}
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	return body.Bytes(), w.FormDataContentType()
}
func picture(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestUploadValidAndTempCleanupOnAllOutcomes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	for _, mode := range []string{"ok", "mime", "probe-error", "timeout", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			raw, ct := multipartFile(t, WAV(12000), "audio/wav")
			limits := Limits{}
			var probe Inspector
			switch mode {
			case "mime":
				raw, ct = multipartFile(t, WAV(12000), "image/png")
			case "probe-error":
				probe = func(context.Context, []byte, Limits) (Media, error) { return Media{}, io.ErrUnexpectedEOF }
			case "timeout":
				limits.Timeout = 10 * time.Millisecond
				probe = func(ctx context.Context, _ []byte, _ Limits) (Media, error) { <-ctx.Done(); return Media{}, ctx.Err() }
			case "oversize":
				limits.BodyBytes = 4096
				limits.FileBytes = 2048
			}
			r := httptest.NewRequest("POST", "/upload", bytes.NewReader(raw))
			r.Header.Set("Content-Type", ct)
			got, e := Receive(httptest.NewRecorder(), r, limits, probe)
			if mode == "ok" {
				if e != nil || got.Media.MIME != "audio/wav" || len(got.Data) == 0 {
					t.Fatal(got.Media, e)
				}
			} else if e == nil {
				t.Fatal("fault accepted")
			}
			files, e := filepath.Glob(filepath.Join(dir, "multipart-*"))
			if e != nil || len(files) != 0 {
				t.Fatal("temporary file leak", files, e)
			}
		})
	}
}
func TestImageDecodeAndPixelBudgets(t *testing.T) {
	b := picture(t, 8, 8)
	m, e := Inspect(context.Background(), b, Limits{})
	if e != nil || m.Width != 8 || m.Height != 8 {
		t.Fatal(m, e)
	}
	if _, e = Inspect(context.Background(), b, Limits{Pixels: 16}); e == nil {
		t.Fatal("pixel overflow")
	}
	if _, e = Inspect(context.Background(), b[:len(b)/2], Limits{}); e == nil {
		t.Fatal("truncated image")
	}
	if _, e = Inspect(context.Background(), []byte(strings.Repeat("fake png", 50)), Limits{}); e == nil {
		t.Fatal("fake MIME accepted")
	}
}
func TestMultipartRejectsMissingBoundaryFieldsAndCancel(t *testing.T) {
	for _, mode := range []string{"boundary", "field", "cancel", "file-size"} {
		t.Run(mode, func(t *testing.T) {
			raw, ct := multipartFile(t, WAV(2400), "audio/wav")
			limits := Limits{}
			switch mode {
			case "boundary":
				ct = "multipart/form-data"
			case "field":
				raw = bytes.Replace(raw, []byte(`name="file"`), []byte(`name="wrong"`), 1)
			case "file-size":
				limits.FileBytes = 1000
			}
			r := httptest.NewRequest("POST", "/upload", bytes.NewReader(raw))
			r.Header.Set("Content-Type", ct)
			if mode == "cancel" {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			if _, e := Receive(httptest.NewRecorder(), r, limits, nil); e == nil {
				t.Fatal("invalid upload accepted")
			}
		})
	}
}
func TestSlowUploadHasSocketReadDeadline(t *testing.T) {
	done := make(chan error, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, e := Receive(w, r, Limits{Timeout: 30 * time.Millisecond}, nil)
		done <- e
		w.WriteHeader(400)
	}))
	defer s.Close()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	request, e := http.NewRequest("POST", s.URL, reader)
	if e != nil {
		t.Fatal(e)
	}
	request.Header.Set("Content-Type", "multipart/form-data; boundary=slow")
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	responseDone := make(chan struct{})
	go func() {
		resp, e := client.Do(request)
		if e == nil {
			_ = resp.Body.Close()
		}
		close(responseDone)
	}()
	_, e = writer.Write([]byte("--slow\r\nContent-Disposition: form-data; name=\"file\"; filename=\"x.wav\"\r\nContent-Type: audio/wav\r\n\r\nRIFF"))
	if e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("slow upload passed")
		}
	case <-time.After(time.Second):
		t.Fatal("upload not bounded")
	}
	_ = writer.Close()
	select {
	case <-responseDone:
	case <-time.After(time.Second):
		t.Fatal("request leaked")
	}
}
