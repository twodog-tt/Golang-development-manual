package ops

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Channel struct{ URL, Token string }

func (c Channel) Call(ctx context.Context, method, path string, body any) (Event, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return Event{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, bytes.NewReader(raw))
	if err != nil {
		return Event{}, err
	}
	req.Header.Set("X-Lab-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return Event{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Event{}, errors.New("channel response " + strconv.Itoa(resp.StatusCode))
	}
	var e Event
	err = json.NewDecoder(io.LimitReader(resp.Body, 32768)).Decode(&e)
	return e, err
}
func Signature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
func VerifyWebhook(secret, timestamp, signature string, body []byte, now time.Time) bool {
	stamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || stamp < now.Unix()-60 || stamp > now.Unix()+60 {
		return false
	}
	got, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	want, _ := hex.DecodeString(Signature(secret, timestamp, body))
	return hmac.Equal(got, want)
}
func Provider(s *Store, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /merchant-webhook", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
		if err != nil || !VerifyWebhook(token, r.Header.Get("X-Timestamp"), r.Header.Get("X-Signature"), raw, time.Now()) {
			fail(w, 401)
			return
		}
		var event struct {
			ID string `json:"event_id"`
		}
		if json.Unmarshal(raw, &event) != nil || event.ID == "" {
			fail(w, 400)
			return
		}
		_, err = s.DB.Exec(r.Context(), "INSERT INTO ops.merchant_inbox VALUES($1,$2) ON CONFLICT DO NOTHING", event.ID, raw)
		if err != nil {
			fail(w, 500)
			return
		}
		var same bool
		if err = s.DB.QueryRow(r.Context(), "SELECT payload=$2::jsonb FROM ops.merchant_inbox WHERE event_id=$1", event.ID, raw).Scan(&same); err != nil || !same {
			fail(w, 409)
			return
		}
		if s.Fault(r.Context(), "notify_response_lost") {
			fail(w, 504)
			return
		}
		write(w, map[string]bool{"accepted": true})
	})
	mux.HandleFunc("POST /transfers", func(w http.ResponseWriter, r *http.Request) {
		var p Payment
		if !decode(w, r, &p) {
			return
		}
		n, err := Amount(p.Amount)
		if err != nil || p.Asset != PaymentAsset() {
			fail(w, 400)
			return
		}
		var amount int64
		var asset string
		err = s.DB.QueryRow(r.Context(), "SELECT amount,asset FROM ops.channel WHERE id=$1", p.ID).Scan(&amount, &asset)
		if err != nil {
			if !p.QuoteExpires.After(time.Now()) {
				fail(w, 409)
				return
			}
			_, err = s.DB.Exec(r.Context(), "INSERT INTO ops.channel(id,amount,asset) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", p.ID, n, p.Asset)
			if err != nil {
				fail(w, 500)
				return
			}
		}
		var e Event
		e.Payment = p.ID
		e.Amount = p.Amount
		e.Asset = p.Asset
		err = s.DB.QueryRow(r.Context(), "SELECT state,sequence FROM ops.channel WHERE id=$1 AND amount=$2 AND asset=$3", p.ID, n, p.Asset).Scan(&e.State, &e.Sequence)
		if err != nil {
			fail(w, 409)
			return
		}
		e.ID = "channel:" + p.ID + ":" + strconv.FormatInt(e.Sequence, 10)
		if s.Fault(r.Context(), "submit_response_lost") {
			fail(w, 504)
			return
		}
		write(w, e)
	})
	mux.HandleFunc("GET /transfers/{id}", func(w http.ResponseWriter, r *http.Request) {
		if ChainMode() {
			e, err := s.ChainEvent(r.Context(), r.PathValue("id"))
			if err != nil {
				fail(w, 503)
				return
			}
			write(w, e)
			return
		}
		var e Event
		var n int64
		e.Payment = r.PathValue("id")
		err := s.DB.QueryRow(r.Context(), "SELECT amount,asset,state,sequence FROM ops.channel WHERE id=$1", e.Payment).Scan(&n, &e.Asset, &e.State, &e.Sequence)
		if err != nil {
			fail(w, 404)
			return
		}
		e.Amount = strconv.FormatInt(n, 10)
		e.ID = "query:" + e.Payment + ":" + strconv.FormatInt(e.Sequence, 10)
		write(w, e)
	})
	mux.HandleFunc("POST /refunds/{id}", func(w http.ResponseWriter, r *http.Request) {
		if ChainMode() {
			fail(w, 409)
			return
		}
		tag, err := s.DB.Exec(r.Context(), "UPDATE ops.channel SET refunded=true WHERE id=$1 AND state='confirmed'", r.PathValue("id"))
		if err != nil || tag.RowsAffected() != 1 {
			fail(w, 409)
			return
		}
		if s.Fault(r.Context(), "refund_response_lost") {
			fail(w, 504)
			return
		}
		write(w, map[string]string{"state": "refunded"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || !hmac.Equal([]byte(r.Header.Get("X-Lab-Token")), []byte(token)) {
			fail(w, 401)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
