package mediaio

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestWireVariantsMatchNestedContract(t *testing.T) {
	raw, e := MarshalMessage([]Part{{Type: "text", Text: "describe"}, {Type: "image_url", Image: &ImageURL{URL: "https://objects.example.invalid/a.png", Detail: "low"}}, {Type: "input_audio", Audio: &InputAudio{Data: base64.StdEncoding.EncodeToString(WAV(240)), Format: "wav"}}})
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]json.RawMessage
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	var parts []map[string]json.RawMessage
	if e = json.Unmarshal(m["content"], &parts); e != nil {
		t.Fatal(e)
	}
	for i, key := range []string{"text", "image_url", "input_audio"} {
		if parts[i][key] == nil || len(parts[i]) != 2 {
			t.Fatal(string(raw))
		}
	}
	var audio map[string]string
	if e = json.Unmarshal(parts[2]["input_audio"], &audio); e != nil || audio["format"] != "wav" || audio["data"] == "" {
		t.Fatal(audio, e)
	}
}
func TestWireRejectsWrongAndMixedVariants(t *testing.T) {
	for _, p := range []Part{
		{Type: "input_audio", Audio: &InputAudio{Data: base64.StdEncoding.EncodeToString(picture(t, 8, 8)), Format: "wav"}}, {Type: "input_audio", Audio: &InputAudio{Data: "not-base64", Format: "wav"}}, {Type: "input_audio", Audio: &InputAudio{Data: "url", Format: "mp3"}}, {Type: "text", Text: "x", Audio: &InputAudio{}}, {Type: "image_url", Text: "x"}, {Type: "image_url", Image: &ImageURL{URL: "http://127.0.0.1/a.png", Detail: "low"}}, {Type: "image_url", Image: &ImageURL{URL: "https://example.invalid/x", Detail: "unsupported"}}, {Type: "audio_url", Text: "https://example.invalid/audio"}} {
		if _, e := MarshalMessage([]Part{p}); e == nil {
			t.Fatal("invalid part accepted", p.Type)
		}
	}
	for _, url := range []string{"https://api.openai.com", "http://localhost:80", "http://127.0.0.1/x", "http://user:pass@127.0.0.1", "http://127.0.0.1?q=x"} {
		if _, e := NewLocalClient(url); e == nil {
			t.Fatal("external/invalid endpoint accepted", url)
		}
	}
}
func TestWAVLengthCodecDurationAndCancellation(t *testing.T) {
	for _, mode := range []string{"length", "codec", "channels", "rate", "alignment", "duration", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			b := WAV(2400)
			limits := Limits{}
			switch mode {
			case "length":
				b[4]++
			case "codec":
				b[20] = 3
			case "channels":
				b[22] = 2
			case "rate":
				binary.LittleEndian.PutUint32(b[24:], 8000)
			case "alignment":
				b[32] = 4
			case "duration":
				limits.Duration = 1
			case "truncated":
				b = b[:30]
			}
			if _, e := Inspect(context.Background(), b, limits); e == nil {
				t.Fatal("invalid media accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Inspect(ctx, WAV(240), Limits{}); e == nil {
		t.Fatal("ignored cancel")
	}
}
