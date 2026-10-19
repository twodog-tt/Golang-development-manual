package agentplatform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func spec() LaunchSpec {
	return LaunchSpec{"tenant-a", "launch-1", "local-chain", "account-a", "token-config-v1", "pool-config-v1", "runtime-v1"}
}
func launcher(t *testing.T) (Launcher, LocalRemote, LaunchSpec) {
	t.Helper()
	dir := t.TempDir()
	remote := LocalRemote{Path: filepath.Join(dir, "remote.json")}
	l := Launcher{Path: filepath.Join(dir, "launch.json"), Remote: remote}
	s := spec()
	must(t, l.Prepare(s))
	return l, remote, s
}
func TestLaunchLostReplyRestartAndNoDuplicateIssuance(t *testing.T) {
	l, r, s := launcher(t)
	l.Remote = LocalRemote{Path: r.Path, LoseReply: true}
	denied(t, l.Advance(context.Background(), s))
	x, err := l.Snapshot(s)
	must(t, err)
	if x.Steps[0].State != "unknown" {
		t.Fatal("unknown not durable")
	}
	l = Launcher{Path: l.Path, Remote: r}
	must(t, l.Reconcile(context.Background(), s))
	must(t, l.Reconcile(context.Background(), s))
	n, err := r.Effects()
	must(t, err)
	if n != 1 {
		t.Fatal("duplicate issuance")
	}
	for i := 0; i < 3; i++ {
		must(t, l.Advance(context.Background(), s))
	}
	n, err = r.Effects()
	must(t, err)
	if n != 3 {
		t.Fatal("expected exactly issue/liquidity/deploy", n)
	}
	balance, err := l.FeeBalance()
	must(t, err)
	if balance != 10 {
		t.Fatal("fee credit", balance)
	}
	changed := s
	changed.TokenConfig = "different"
	denied(t, l.Prepare(changed))
	denied(t, l.Advance(context.Background(), changed))
	changed = s
	changed.Tenant = "other"
	_, err = l.Snapshot(changed)
	denied(t, err)
}

type remoteFunc struct {
	submit func(context.Context, string, string) (LaunchReceipt, error)
	lookup func(context.Context, string) (LaunchReceipt, error)
}

func (r remoteFunc) Submit(c context.Context, k, p string) (LaunchReceipt, error) {
	return r.submit(c, k, p)
}
func (r remoteFunc) Lookup(c context.Context, k string) (LaunchReceipt, error) { return r.lookup(c, k) }

func TestLaunchPendingUnseenWrongEvidenceAndUnavailableQuery(t *testing.T) {
	for _, name := range []string{"unseen", "pending", "wrong-key", "wrong-parameters", "no-tx", "no-block", "rpc", "orphan-without-prior-receipt"} {
		t.Run(name, func(t *testing.T) {
			l, r, s := launcher(t)
			l.Remote = LocalRemote{Path: r.Path, LoseReply: true}
			denied(t, l.Advance(context.Background(), s))
			calls := 0
			l.Remote = remoteFunc{submit: func(context.Context, string, string) (LaunchReceipt, error) {
				calls++
				return LaunchReceipt{}, ErrUnknown
			}, lookup: func(ctx context.Context, k string) (LaunchReceipt, error) {
				e, err := r.Lookup(ctx, k)
				if err != nil {
					return e, err
				}
				switch name {
				case "unseen":
					e.Known = false
				case "pending":
					e.Final = false
				case "wrong-key":
					e.Key = "other"
				case "wrong-parameters":
					e.Parameters = "other"
				case "no-tx":
					e.Transaction = ""
				case "no-block":
					e.Block = ""
				case "rpc":
					return LaunchReceipt{}, ErrUnknown
				case "orphan-without-prior-receipt":
					e.Canonical = false
				}
				return e, nil
			}}
			denied(t, l.Reconcile(context.Background(), s))
			denied(t, l.Advance(context.Background(), s))
			if calls != 0 {
				t.Fatal("unknown automatically resubmitted")
			}
			x, err := l.Snapshot(s)
			must(t, err)
			if x.Steps[0].State != "unknown" {
				t.Fatal("unproven evidence changed state")
			}
		})
	}
}

func TestLaunchRuntimeFailureAfterIssuanceDoesNotReplayEarlierSteps(t *testing.T) {
	l, r, s := launcher(t)
	must(t, l.Advance(context.Background(), s))
	must(t, l.Advance(context.Background(), s))
	submits := 0
	l.Remote = remoteFunc{submit: func(context.Context, string, string) (LaunchReceipt, error) {
		submits++
		return LaunchReceipt{}, errors.New("runtime unavailable")
	}, lookup: r.Lookup}
	denied(t, l.Advance(context.Background(), s))
	for i := 0; i < 3; i++ {
		denied(t, l.Advance(context.Background(), s))
	}
	n, err := r.Effects()
	must(t, err)
	if n != 2 || submits != 1 {
		t.Fatal("replayed partial launch", n, submits)
	}
	balance, err := l.FeeBalance()
	must(t, err)
	if balance != 0 {
		t.Fatal("credited incomplete launch")
	}
}

func TestLaunchReorgReversesFeeOnceAndStopsDownstream(t *testing.T) {
	t.Run("last-stage-reorg-returns-blocked", func(t *testing.T) {
		l, r, s := launcher(t)
		for i := 0; i < 3; i++ {
			must(t, l.Advance(context.Background(), s))
		}
		must(t, r.Orphan(stepKey(s, 2)))
		denied(t, l.Reconcile(context.Background(), s))
		x, err := l.Snapshot(s)
		must(t, err)
		if x.State != "review_chain" {
			t.Fatal("last stage did not block")
		}
		balance, err := l.FeeBalance()
		must(t, err)
		if balance != 0 {
			t.Fatal("last-stage fee reversal", balance)
		}
		denied(t, l.Reconcile(context.Background(), s))
	})
	l, r, s := launcher(t)
	for i := 0; i < 3; i++ {
		must(t, l.Advance(context.Background(), s))
	}
	must(t, l.Reconcile(context.Background(), s))
	must(t, r.Orphan(stepKey(s, 0)))
	_ = l.Reconcile(context.Background(), s)
	for i := 0; i < 3; i++ {
		denied(t, l.Advance(context.Background(), s))
		denied(t, l.Reconcile(context.Background(), s))
	}
	x, err := l.Snapshot(s)
	must(t, err)
	if x.State != "review_chain" {
		t.Fatal("missing review_chain")
	}
	b, err := l.FeeBalance()
	must(t, err)
	if b != 0 {
		t.Fatal("fee not reversed exactly once", b)
	}
	must(t, transaction(l.Path, func(d *launchDB) error {
		if len(d.Fees) != 2 {
			t.Fatal("duplicate fee events", d.Fees)
		}
		return nil
	}))
	l, r, s = launcher(t)
	must(t, l.Advance(context.Background(), s))
	must(t, r.Orphan(stepKey(s, 0)))
	denied(t, l.Advance(context.Background(), s))
	n, err := r.Effects()
	must(t, err)
	if n != 1 {
		t.Fatal("liquidity submitted after orphaned issuance")
	}
}

func TestLaunchStaleRecoveryCASAndAbsentReceiptCannotReverseCredit(t *testing.T) {
	l, r, s := launcher(t)
	for i := 0; i < 3; i++ {
		must(t, l.Advance(context.Background(), s))
	}
	old, err := l.Snapshot(s)
	must(t, err)
	e, err := r.Lookup(context.Background(), stepKey(s, 0))
	must(t, err)
	must(t, l.Reconcile(context.Background(), s))
	e.Canonical = false
	denied(t, l.apply(s, 0, old.Revision, e))
	current, err := l.Snapshot(s)
	must(t, err)
	e.Transaction = "other-tx"
	denied(t, l.apply(s, 0, current.Revision, e))
	l.Remote = remoteFunc{submit: r.Submit, lookup: func(context.Context, string) (LaunchReceipt, error) { return LaunchReceipt{}, nil }}
	denied(t, l.Reconcile(context.Background(), s))
	balance, err := l.FeeBalance()
	must(t, err)
	if balance != 10 {
		t.Fatal("absence incorrectly reversed credit")
	}
}

func TestLaunchConcurrentInstancesAndCorruptStateFailClosed(t *testing.T) {
	l, r, s := launcher(t)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other := Launcher{Path: l.Path, Remote: r}
			_ = other.Advance(context.Background(), s)
		}()
	}
	wg.Wait()
	for i := 0; i < 3; i++ {
		must(t, l.Advance(context.Background(), s))
	}
	n, err := r.Effects()
	must(t, err)
	if n != 3 {
		t.Fatal("duplicate concurrent effects", n)
	}
	must(t, os.WriteFile(l.Path, []byte(`{"Format":1}`), 0600))
	denied(t, l.Prepare(s))
	denied(t, l.Advance(context.Background(), s))
	n, err = r.Effects()
	must(t, err)
	if n != 3 {
		t.Fatal("corruption reset state")
	}
}

func TestLaunchCrashHelper(t *testing.T) {
	if os.Getenv("AGENTPLATFORM_CRASH_HELPER") != "1" {
		return
	}
	dir := os.Getenv("AGENTPLATFORM_CRASH_DIR")
	r := LocalRemote{Path: filepath.Join(dir, "remote.json")}
	l := Launcher{Path: filepath.Join(dir, "launch.json"), Remote: r, AfterSubmit: func() { os.Exit(73) }}
	if err := l.Prepare(spec()); err != nil {
		os.Exit(72)
	}
	_ = l.Advance(context.Background(), spec())
	os.Exit(71)
}
func TestLaunchRealProcessCrashAfterRemoteCommit(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLaunchCrashHelper$")
	cmd.Env = append(os.Environ(), "AGENTPLATFORM_CRASH_HELPER=1", "AGENTPLATFORM_CRASH_DIR="+dir)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 73 {
		t.Fatalf("helper did not crash after commit: %v %s", err, out)
	}
	r := LocalRemote{Path: filepath.Join(dir, "remote.json")}
	l := Launcher{Path: filepath.Join(dir, "launch.json"), Remote: r}
	must(t, l.Reconcile(context.Background(), spec()))
	n, err := r.Effects()
	must(t, err)
	if n != 1 {
		t.Fatal("recovery issued twice")
	}
	for i := 0; i < 3; i++ {
		must(t, l.Advance(context.Background(), spec()))
	}
	n, err = r.Effects()
	must(t, err)
	if n != 3 {
		t.Fatal("wrong partial recovery", n)
	}
}
