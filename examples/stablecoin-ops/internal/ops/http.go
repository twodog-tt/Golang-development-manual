package ops

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int) { http.Error(w, http.StatusText(status), status) }
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400)
		return false
	}
	return true
}
func API(auth Auth, temporal client.Client, secret string, approvalSeconds, observeSeconds int) http.Handler {
	mux := http.NewServeMux()
	store := auth.Store
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { write(w, map[string]bool{"ok": true}) })
	mux.HandleFunc("POST /challenge", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Address string `json:"address"`
		}
		if !decode(w, r, &v) {
			return
		}
		c, err := auth.Challenge(r.Context(), v.Address)
		if err != nil {
			fail(w, 401)
			return
		}
		write(w, c)
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID        string `json:"id"`
			Signature string `json:"signature"`
		}
		if !decode(w, r, &v) {
			return
		}
		token, err := auth.Login(r.Context(), v.ID, v.Signature)
		if err != nil {
			fail(w, 401)
			return
		}
		write(w, map[string]string{"token": token})
	})
	mux.HandleFunc("POST /webhook", func(w http.ResponseWriter, r *http.Request) {
		// Chain mode only accepts indexed onchain observations, not mock events.
		if ChainMode() {
			fail(w, 409)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		raw, err := io.ReadAll(r.Body)
		if err != nil || !VerifyWebhook(secret, r.Header.Get("X-Timestamp"), r.Header.Get("X-Signature"), raw, time.Now()) {
			fail(w, 401)
			return
		}
		var event Event
		if json.Unmarshal(raw, &event) != nil {
			fail(w, 400)
			return
		}
		if err = store.Ingest(r.Context(), event); err != nil {
			fail(w, 409)
			return
		}
		write(w, map[string]bool{"accepted": true})
	})
	protected := http.NewServeMux()
	protected.HandleFunc("DELETE /wallets/{address}", func(w http.ResponseWriter, r *http.Request) {
		tx, err := store.DB.Begin(r.Context())
		if err != nil {
			fail(w, 500)
			return
		}
		defer tx.Rollback(r.Context())
		merchant := r.Header.Get("X-Merchant-Internal")
		tag, err := tx.Exec(r.Context(), "DELETE FROM ops.wallets WHERE address=$1 AND merchant=$2", r.PathValue("address"), merchant)
		if err != nil || tag.RowsAffected() != 1 {
			fail(w, 404)
			return
		}
		if _, err = tx.Exec(r.Context(), "DELETE FROM ops.sessions WHERE merchant=$1", merchant); err != nil {
			fail(w, 500)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			fail(w, 500)
			return
		}
		write(w, map[string]bool{"unbound": true})
	})
	protected.HandleFunc("DELETE /session", func(w http.ResponseWriter, r *http.Request) {
		if auth.Logout(r.Context(), r.Header.Get("Authorization")) != nil {
			fail(w, 500)
			return
		}
		write(w, map[string]bool{"revoked": true})
	})
	start := func(w http.ResponseWriter, r *http.Request, id string, fn any, input any) bool {
		reuse := enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
		if strings.HasPrefix(id, "reconcile:") || strings.HasPrefix(id, "notify:") {
			reuse = enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE
		}
		_, err := temporal.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{ID: id, TaskQueue: Queue, WorkflowIDReusePolicy: reuse}, fn, input)
		var duplicate *serviceerror.WorkflowExecutionAlreadyStarted
		if err != nil && !errors.As(err, &duplicate) {
			fail(w, 503)
			return false
		}
		return true
	}
	protected.HandleFunc("POST /payments", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Amount string `json:"amount_atomic"`
			Asset  string `json:"asset"`
		}
		if !decode(w, r, &v) {
			return
		}
		p, err := store.Create(r.Context(), r.Header.Get("X-Merchant-Internal"), r.Header.Get("Idempotency-Key"), v.Amount, v.Asset)
		if err != nil {
			if errors.Is(err, ErrConflict) {
				fail(w, 409)
			} else {
				fail(w, 400)
			}
			return
		}
		var fn any = PaymentWorkflow
		var input any = Input{p, approvalSeconds, observeSeconds}
		if ChainMode() {
			fn = ChainPaymentWorkflow
			input = ChainInput{Input: Input{p, approvalSeconds, observeSeconds}}
		}
		if start(w, r, "pay:"+p.ID, fn, input) {
			write(w, p)
		}
	})
	protected.HandleFunc("GET /payments/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.Get(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		write(w, p)
	})
	protected.HandleFunc("POST /payments/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.Get(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		if p.State != "created" {
			fail(w, 409)
			return
		}
		if err = temporal.SignalWorkflow(r.Context(), "pay:"+p.ID, "", "approve", true); err != nil {
			fail(w, 409)
			return
		}
		write(w, map[string]bool{"approved": true})
	})
	protected.HandleFunc("POST /payments/{id}/refund", func(w http.ResponseWriter, r *http.Request) {
		if ChainMode() {
			fail(w, 409)
			return
		}
		p, err := store.Get(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		if p.State != "settled" && p.State != "refund_pending" && p.State != "refunded" {
			fail(w, 409)
			return
		}
		if start(w, r, "refund:"+p.ID, RefundWorkflow, p) {
			write(w, map[string]string{"workflow": "refund:" + p.ID})
		}
	})
	protected.HandleFunc("GET /ops", func(w http.ResponseWriter, r *http.Request) {
		result, err := store.Reconcile(r.Context(), r.Header.Get("X-Merchant-Internal"))
		if err != nil {
			fail(w, 500)
			return
		}
		write(w, result)
	})
	protected.HandleFunc("GET /payments/{id}/conversion-quote", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.Get(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		if p.State != "settled" {
			fail(w, 409)
			return
		}
		var quote map[string]any
		if err = chainCall(r.Context(), "GET", "/quote?amount="+p.Amount, nil, &quote); err != nil {
			fail(w, 503)
			return
		}
		write(w, quote)
	})
	protected.HandleFunc("GET /payments/{id}/conversion", func(w http.ResponseWriter, r *http.Request) {
		out, err := store.Conversion(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		write(w, out)
	})
	protected.HandleFunc("POST /payments/{id}/convert", func(w http.ResponseWriter, r *http.Request) {
		var request ConversionRequest
		if !decode(w, r, &request) {
			return
		}
		id := r.PathValue("id")
		if err := store.CreateConversion(r.Context(), r.Header.Get("X-Merchant-Internal"), id, request); err != nil {
			fail(w, 409)
			return
		}
		if start(w, r, "convert:"+id, ConversionWorkflow, id) {
			write(w, map[string]string{"workflow": "convert:" + id})
		}
	})
	protected.HandleFunc("POST /payments/{id}/conversion/resume", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		c, err := store.Conversion(r.Context(), r.Header.Get("X-Merchant-Internal"), id)
		if err != nil {
			fail(w, 404)
			return
		}
		if c.State != "review" {
			fail(w, 409)
			return
		}
		if start(w, r, "reconcile:convert:"+id, ConversionWorkflow, id) {
			write(w, map[string]bool{"resuming": true})
		}
	})
	protected.HandleFunc("POST /payments/{id}/reconcile", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.Get(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		if p.State != "review" {
			fail(w, 409)
			return
		}
		var fn any = ReconcileWorkflow
		if ChainMode() {
			fn = ChainReconcileWorkflow
		}
		if start(w, r, "reconcile:"+p.ID, fn, p) {
			write(w, map[string]bool{"reconciling": true})
		}
	})
	protected.HandleFunc("POST /payments/{id}/notify", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.Get(r.Context(), r.Header.Get("X-Merchant-Internal"), r.PathValue("id"))
		if err != nil {
			fail(w, 404)
			return
		}
		prefixes := map[string]string{"settled": "settle:", "refunded": "refund:", "review_reversed": "reorg:"}
		prefix, ok := prefixes[p.State]
		if !ok {
			fail(w, 409)
			return
		}
		if start(w, r, "notify:"+prefix+p.ID, NotifyWorkflow, prefix+p.ID) {
			write(w, map[string]bool{"queued": true})
		}
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		merchant, err := auth.Merchant(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			fail(w, 401)
			return
		}
		// Never trust a tenant ID supplied in a header or body by the caller.
		r.Header.Set("X-Merchant-Internal", merchant)
		protected.ServeHTTP(w, r)
	}))
	return mux
}
