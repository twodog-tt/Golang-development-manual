package agentcommerce

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"td-homework/examples/senior/agentmemory"
)

var clockTime = time.Unix(2000, 0)

func fixture(t *testing.T, dir string) (Engine, context.Context, agentmemory.Scope, Binding, Payment, *agentmemory.Authority) {
	t.Helper()
	scope := agentmemory.Scope{Tenant: "tenant-a", Bot: "buyer", Scene: "reports", SubjectType: "user", SubjectID: "alice"}
	a := agentmemory.NewAuthority()
	a.Put("local-token", agentmemory.Grant{Tenant: scope.Tenant, Subject: "alice", Policy: "commerce-v1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"pay"}, Expires: time.Now().Add(time.Hour)})
	ctx, e := a.Context(context.Background(), "local-token")
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewBinding(scope.Tenant, "alice", "report-1", "POST", "https://reports.fixture/report", []byte(`{"topic":"go"}`))
	if e != nil {
		t.Fatal(e)
	}
	seed := sha256.Sum256([]byte("public teaching key; NEVER fund"))
	key, e := crypto.ToECDSA(seed[:])
	if e != nil {
		t.Fatal(e)
	}
	p := Payment{From: crypto.PubkeyToAddress(key.PublicKey), To: FixtureMerchant, Value: "10", ValidAfter: 1900, ValidBefore: 2200, Nonce: crypto.Keccak256Hash([]byte("nonce-1"))}
	digest, e := PaymentDigest(FixtureMechanism(), p)
	if e != nil {
		t.Fatal(e)
	}
	p.Signature, e = crypto.Sign(digest[:], key)
	if e != nil {
		t.Fatal(e)
	}
	e1 := Engine{Dir: dir, Payer: p.From, Chain: LocalChain{Dir: dir, Now: func() time.Time { return clockTime }}, Now: func() time.Time { return clockTime }}
	return e1, ctx, scope, b, p, a
}
func prepared(t *testing.T) (Engine, context.Context, agentmemory.Scope, Binding, Payment, *agentmemory.Authority) {
	t.Helper()
	e, ctx, s, b, p, a := fixture(t, t.TempDir())
	if err := e.Prepare(ctx, s, b, FixtureMechanism(), p); err != nil {
		t.Fatal(err)
	}
	return e, ctx, s, b, p, a
}
func TestActualEIP3009SignatureAndUnsupportedMechanisms(t *testing.T) {
	e, ctx, s, b, p, _ := fixture(t, t.TempDir())
	m := FixtureMechanism()
	if _, err := VerifyPayment(m, p, 2000); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"signature", "payer", "payee", "value", "nonce", "expired", "not-yet-valid", "version", "network", "asset", "facilitator", "scheme", "transfer", "flow"} {
		t.Run(mode, func(t *testing.T) {
			bad := p
			bad.Signature = append([]byte(nil), p.Signature...)
			mm := m
			switch mode {
			case "signature":
				bad.Signature[0] ^= 1
			case "payer":
				bad.From = common.HexToAddress("0x900")
			case "payee":
				bad.To = bad.From
			case "value":
				bad.Value = "11"
			case "nonce":
				bad.Nonce = crypto.Keccak256Hash([]byte("other"))
			case "expired":
				bad.ValidBefore = 1999
			case "not-yet-valid":
				bad.ValidAfter = 2001
			case "version":
				mm.Version = 1
			case "network":
				mm.Network = "eip155:1"
			case "asset":
				mm.Asset = FixtureMerchant.Hex()
			case "facilitator":
				mm.Facilitator = "unknown"
			case "scheme":
				mm.Scheme = "upto"
			case "transfer":
				mm.TransferMethod = "permit2"
			case "flow":
				mm.Flow = "upfront"
			}
			if err := e.Prepare(ctx, s, b, mm, bad); err == nil {
				t.Fatal("unsupported or altered proof accepted")
			}
		})
	}
	m.TransferMethod = ""
	m.Flow = ""
	if err := e.Prepare(ctx, s, b, m, p); err != nil {
		t.Fatal("documented default rejected", err)
	}
}
func TestResourceTenantSubjectAndBodyBindingBeforeExecution(t *testing.T) {
	e, ctx, s, b, p, a := prepared(t)
	// The SAME payment signature is still valid if HTTP metadata is changed:
	// its digest has no resource/body/tenant fields. Stored binding must stop it.
	if _, err := VerifyPayment(FixtureMechanism(), p, 2000); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"tenant", "subject", "resource", "method", "body", "intent"} {
		bad := b
		switch mode {
		case "tenant":
			bad.Tenant = "tenant-b"
		case "subject":
			bad.Subject = "bob"
		case "resource":
			bad.Resource = "https://reports.fixture/private"
		case "method":
			bad.Method = "DELETE"
		case "body":
			bad.BodyHash = sha256.Sum256([]byte("other"))
		case "intent":
			bad.Intent = "other"
		}
		if _, err := e.Execute(ctx, s, bad); err == nil {
			t.Fatal("request swap accepted", mode)
		}
	}
	a.Put("bob", agentmemory.Grant{Tenant: s.Tenant, Subject: "bob", Policy: "commerce-v1", Scopes: []agentmemory.Scope{s}, Roles: []string{"pay"}, Expires: time.Now().Add(time.Hour)})
	other, _ := a.Context(context.Background(), "bob")
	if _, err := e.Execute(other, s, b); err == nil {
		t.Fatal("foreign subject spent intent")
	}
	foreignRecord, leak := e.Snapshot(other, s, b.Intent)
	if leak == nil || foreignRecord.Result != "" || foreignRecord.Payment.Signature != nil {
		t.Fatal("foreign snapshot exposed payment record")
	}
	n, _ := e.Chain.(LocalChain).Effects()
	if n != 0 {
		t.Fatal("authorization failure settled payment")
	}
}
func TestConcurrentReplayCachedResultAndNonceReservation(t *testing.T) {
	e, ctx, s, b, p, _ := prepared(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.Execute(ctx, s, b)
			if err != nil && !errors.Is(err, ErrUnknown) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	result, err := e.Result(ctx, s, b.Intent)
	if err != nil || result == "" {
		t.Fatal(result, err)
	}
	restarted := Engine{Dir: e.Dir, Payer: e.Payer, Chain: e.Chain, Now: e.Now}
	got, err := restarted.Execute(ctx, s, b)
	if err != nil || got != result {
		t.Fatal("response-loss replay failed", got, err)
	}
	b.Intent = "new-intent"
	if err = e.Prepare(ctx, s, b, FixtureMechanism(), p); err == nil {
		t.Fatal("same token authorization reused for another intent")
	}
	n, _ := e.Chain.(LocalChain).Effects()
	// Same human-readable intent in another scope must not leak ledger legs.
	if err := e.db().tx(func(d *database) error {
		d.Entries = append(d.Entries, Entry{"other-scope-record", "settlement", common.Hash{}, "other-payer", "other-merchant", 10})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	entries, _ := e.Entries(ctx, s, "report-1")
	if n != 1 || len(entries) != 1 || entries[0].Debit != "payer" || entries[0].Credit != "merchant" || entries[0].Amount != 10 {
		t.Fatal(n, entries)
	}
}
func TestLostSettlementReplyRestartAndCanonicalRecovery(t *testing.T) {
	e, ctx, s, b, _, _ := prepared(t)
	e.Chain = LocalChain{Dir: e.Dir, LostReply: true, Now: e.Now}
	if _, err := e.Execute(ctx, s, b); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	restarted, ctx, s, _, _, _ := fixture(t, e.Dir) // fresh trusted ingress identity after process restart
	if _, err := restarted.Execute(ctx, s, b); !errors.Is(err, ErrUnknown) {
		t.Fatal("unknown was resubmitted", err)
	}
	if err := restarted.Recover(ctx, s, b.Intent); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Execute(ctx, s, b); err != nil {
		t.Fatal(err)
	}
	n, _ := restarted.Chain.(LocalChain).Effects()
	entries, _ := restarted.Entries(ctx, s, b.Intent)
	if n != 1 || len(entries) != 1 {
		t.Fatal("duplicate settlement", n, entries)
	}
}
func TestRevocationAfterRemoteEffectDoesNotReturnResource(t *testing.T) {
	e, ctx, s, b, _, a := prepared(t)
	e.AfterSettle = func() { a.Revoke("local-token") }
	if result, err := e.Execute(ctx, s, b); err == nil || result != "" {
		t.Fatal("revoked principal got resource")
	}
	n, _ := e.Chain.(LocalChain).Effects()
	if n != 1 {
		t.Fatal("fault failed to cross effect boundary")
	}
}
func observation(t *testing.T, e Engine, p Payment) Observation {
	t.Helper()
	o, err := e.Chain.Lookup(context.Background(), FixtureMechanism(), p)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

type observedChain struct {
	obs Observation
	err error
}

func (c observedChain) Settle(context.Context, Mechanism, Payment) (Observation, error) {
	return c.obs, c.err
}
func (c observedChain) Lookup(context.Context, Mechanism, Payment) (Observation, error) {
	return c.obs, c.err
}
func TestPendingUnseenCanceledAndMismatchedEvidence(t *testing.T) {
	for _, mode := range []string{"unseen", "pending", "noncanonical", "canceled", "wrong-recipient", "wrong-nonce", "wrong-digest", "rpc-error"} {
		t.Run(mode, func(t *testing.T) {
			e, ctx, s, b, p, _ := prepared(t)
			e.Chain = LocalChain{Dir: e.Dir, LostReply: true, Now: e.Now}
			_, _ = e.Execute(ctx, s, b)
			o := observation(t, e, p)
			switch mode {
			case "unseen":
				o.Known = false
			case "pending":
				o.Finalized = false
			case "noncanonical":
				o.Canonical = false
			case "canceled":
				o.Canceled = true
			case "wrong-recipient":
				o.To = p.From
			case "wrong-nonce":
				o.Nonce = common.Hash{}
			case "wrong-digest":
				o.Digest = common.Hash{}
			}
			source := observedChain{obs: o}
			if mode == "rpc-error" {
				source.err = errors.New("RPC unavailable")
			}
			e.Chain = source
			err := e.Recover(ctx, s, b.Intent)
			if mode == "canceled" {
				if err != nil {
					t.Fatal(err)
				}
				r, _ := e.Snapshot(ctx, s, b.Intent)
				if r.State != "failed" {
					t.Fatal(r.State)
				}
			} else if err == nil {
				t.Fatal("uncertain or contradictory evidence accepted")
			}
			if got, err := e.Result(ctx, s, b.Intent); err == nil || got != "" {
				t.Fatal("unpaid output returned")
			}
			entries, _ := e.Entries(ctx, s, b.Intent)
			if len(entries) != 0 {
				t.Fatal("unproven settlement entered ledger")
			}
		})
	}
}
func TestReorgReversalReconfirmationAndStaleRecoveryCAS(t *testing.T) {
	e, ctx, s, b, p, _ := prepared(t)
	if _, err := e.Execute(ctx, s, b); err != nil {
		t.Fatal(err)
	}
	old, _ := e.Snapshot(ctx, s, b.Intent)
	o := observation(t, e, p)
	o.Canonical = false
	e.Chain = observedChain{obs: o}
	if err := e.Recover(ctx, s, b.Intent); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	if _, err := e.Result(ctx, s, b.Intent); err == nil {
		t.Fatal("orphaned payment authorizes delivery")
	}
	if err := e.accept(ctx, s, b.Intent, old, old.Receipt); err == nil {
		t.Fatal("late recovery overwrote newer state")
	}
	o.Canonical = true
	o.BlockHash = crypto.Keccak256Hash([]byte("replacement block"))
	e.Chain = observedChain{obs: o}
	if err := e.Recover(ctx, s, b.Intent); err != nil {
		t.Fatal(err)
	}
	if err := e.Recover(ctx, s, b.Intent); err != nil {
		t.Fatal(err)
	}
	entries, _ := e.Entries(ctx, s, b.Intent)
	if len(entries) != 3 || entries[1].Kind != "reversal" || entries[1].Debit != "merchant" || entries[2].Kind != "settlement" {
		t.Fatal(entries)
	}
}
func TestPersistedStateCorruptionFailsClosed(t *testing.T) {
	e, ctx, s, b, _, _ := prepared(t)
	if err := os.WriteFile(filepath.Join(e.Dir, "payments.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Execute(ctx, s, b); err == nil {
		t.Fatal("corruption reset payment state")
	}
}
func TestPaymentCrashChild(t *testing.T) {
	dir := os.Getenv("COMMERCE_CRASH_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	e, ctx, s, b, p, _ := fixture(t, dir)
	if err := e.Prepare(ctx, s, b, FixtureMechanism(), p); err != nil {
		t.Fatal(err)
	}
	e.AfterSettle = func() { os.Exit(73) }
	_, _ = e.Execute(ctx, s, b)
	t.Fatal("crash seam not reached")
}
func TestRealProcessCrashAfterSettlementRecoversWithoutRepayment(t *testing.T) {
	dir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestPaymentCrashChild$")
	child.Env = append(os.Environ(), "COMMERCE_CRASH_DIR="+dir)
	err := child.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 73 {
		t.Fatal("wrong crash boundary", err)
	}
	e, ctx, s, b, _, _ := fixture(t, dir)
	if _, err = e.Execute(ctx, s, b); !errors.Is(err, ErrUnknown) {
		t.Fatal("crash recovery blindly retried", err)
	}
	if err = e.Recover(ctx, s, b.Intent); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Result(ctx, s, b.Intent); err != nil {
		t.Fatal(err)
	}
	n, _ := e.Chain.(LocalChain).Effects()
	entries, _ := e.Entries(ctx, s, b.Intent)
	if n != 1 || len(entries) != 1 {
		t.Fatal(n, entries)
	}
}

func TestRecoveryRevisionFencesABAAndUnseenCannotReversePaid(t *testing.T) {
	e, ctx, s, b, p, _ := prepared(t)
	if _, err := e.Execute(ctx, s, b); err != nil {
		t.Fatal(err)
	}
	paid, _ := e.Snapshot(ctx, s, b.Intent)
	canonical := observation(t, e, p)
	for _, mode := range []string{"unseen", "pending", "missing-block", "wrong-orphan-block"} {
		bad := canonical
		switch mode {
		case "unseen":
			bad.Known = false
		case "pending":
			bad.Finalized = false
		case "missing-block":
			bad.BlockHash = common.Hash{}
		case "wrong-orphan-block":
			bad.Canonical = false
			bad.BlockHash = crypto.Keccak256Hash([]byte("unrelated block"))
		}
		e.Chain = observedChain{obs: bad}
		if err := e.Recover(ctx, s, b.Intent); err == nil {
			t.Fatal("uncertain receipt accepted", mode)
		}
		r, _ := e.Snapshot(ctx, s, b.Intent)
		entries, _ := e.Entries(ctx, s, b.Intent)
		if r.State != "paid" || r.Receipt != paid.Receipt || len(entries) != 1 {
			t.Fatal("absence fabricated a reversal", mode, r.State, entries)
		}
	}
	orphan := canonical
	orphan.Canonical = false
	e.Chain = observedChain{obs: orphan}
	if err := e.Recover(ctx, s, b.Intent); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	oldUnknown, _ := e.Snapshot(ctx, s, b.Intent)
	e.Chain = observedChain{obs: canonical}
	if err := e.Recover(ctx, s, b.Intent); err != nil {
		t.Fatal(err)
	}
	e.Chain = observedChain{obs: orphan}
	if err := e.Recover(ctx, s, b.Intent); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	current, _ := e.Snapshot(ctx, s, b.Intent)
	if current.State != oldUnknown.State || current.Receipt != oldUnknown.Receipt || current.Revision <= oldUnknown.Revision {
		t.Fatal("ABA fixture not reached")
	}
	if err := e.accept(ctx, s, b.Intent, oldUnknown, canonical); err == nil {
		t.Fatal("ABA resurrected stale paid evidence")
	}
	if _, err := e.Result(ctx, s, b.Intent); err == nil {
		t.Fatal("old recovery authorized delivery")
	}
}
func TestReopenedEngineRequiresPayerAndChainConfiguration(t *testing.T) {
	e, ctx, s, b, _, _ := prepared(t)
	for _, mode := range []string{"empty-dir", "wrong-payer", "missing-chain"} {
		bad := e
		switch mode {
		case "empty-dir":
			bad.Dir = ""
		case "wrong-payer":
			bad.Payer = common.HexToAddress("0x999")
		case "missing-chain":
			bad.Chain = nil
		}
		if _, err := bad.Execute(ctx, s, b); err == nil {
			t.Fatal("unconfigured engine executed", mode)
		}
		if err := bad.Recover(ctx, s, b.Intent); err == nil {
			t.Fatal("unconfigured engine recovered", mode)
		}
	}
	n, _ := e.Chain.(LocalChain).Effects()
	if n != 0 {
		t.Fatal("configuration failure dispatched payment")
	}
}
