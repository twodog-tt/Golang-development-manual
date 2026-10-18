package ops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Asset = "31337:0x1111111111111111111111111111111111111111:USDC:6"

var ErrConflict = errors.New("idempotency or state conflict")
var ErrDenied = errors.New("not found or forbidden")

type Store struct{ DB *pgxpool.Pool }
type Payment struct {
	ID           string    `json:"id"`
	Merchant     string    `json:"merchant"`
	Amount       string    `json:"amount_atomic"`
	Asset        string    `json:"asset"`
	State        string    `json:"state"`
	QuoteExpires time.Time `json:"quote_expires"`
}
type Event struct {
	ID       string `json:"id"`
	Payment  string `json:"payment"`
	Amount   string `json:"amount_atomic"`
	Asset    string `json:"asset"`
	State    string `json:"state"`
	Sequence int64  `json:"sequence"`
}

func ID() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Amount(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n > 1_000_000_000_000_000_000 || strconv.FormatInt(n, 10) != s {
		return 0, errors.New("amount must be a canonical positive atomic-unit string <= 10^18")
	}
	return n, nil
}
func scanPayment(row pgx.Row) (Payment, error) {
	var p Payment
	var amount int64
	err := row.Scan(&p.ID, &p.Merchant, &amount, &p.Asset, &p.State, &p.QuoteExpires)
	p.Amount = strconv.FormatInt(amount, 10)
	return p, err
}

const columns = "id,merchant,amount,asset,state,quote_expires"

func (s *Store) Get(ctx context.Context, merchant, id string) (Payment, error) {
	p, err := scanPayment(s.DB.QueryRow(ctx, "SELECT "+columns+" FROM ops.payments WHERE id=$1 AND merchant=$2", id, merchant))
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrDenied
	}
	return p, err
}
func (s *Store) Create(ctx context.Context, merchant, key, amount, asset string) (Payment, error) {
	n, err := Amount(amount)
	if err != nil {
		return Payment{}, err
	}
	if len(key) < 1 || len(key) > 100 || asset != Asset {
		return Payment{}, errors.New("invalid idempotency key or asset")
	}
	p, err := scanPayment(s.DB.QueryRow(ctx, "INSERT INTO ops.payments(id,merchant,idem,amount,asset,quote_expires) VALUES($1,$2,$3,$4,$5,now()+interval '15 minutes') ON CONFLICT(merchant,idem) DO UPDATE SET idem=EXCLUDED.idem RETURNING "+columns, ID(), merchant, key, n, asset))
	if err == nil && (p.Amount != amount || p.Asset != asset) {
		err = ErrConflict
	}
	return p, err
}
func (s *Store) State(ctx context.Context, id, state string) error {
	// Never let an old Workflow retry overwrite an irreversible ledger state.
	_, err := s.DB.Exec(ctx, "UPDATE ops.payments SET state=$2 WHERE id=$1 AND state NOT IN ('settled','refunded','refund_pending','review_reversed')", id, state)
	return err
}
func (s *Store) Fault(ctx context.Context, name string) bool {
	var n int
	err := s.DB.QueryRow(ctx, "UPDATE ops.faults SET remaining=remaining-1 WHERE name=$1 AND remaining>0 RETURNING remaining", name).Scan(&n)
	return err == nil
}
func (s *Store) Ingest(ctx context.Context, e Event) error {
	if e.ID == "" || len(e.ID) > 128 || e.Sequence < 0 || (e.State != "pending" && e.State != "confirmed" && e.State != "reversed") {
		return errors.New("invalid channel state")
	}
	n, err := Amount(e.Amount)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var amount, seq int64
	var asset, state, previous string
	err = tx.QueryRow(ctx, "SELECT amount,asset,sequence,state,provider_state FROM ops.payments WHERE id=$1 FOR UPDATE", e.Payment).Scan(&amount, &asset, &seq, &state, &previous)
	if err != nil {
		return err
	}
	if n != amount || e.Asset != asset {
		return ErrConflict
	}
	raw, _ := json.Marshal(e)
	var inserted string
	err = tx.QueryRow(ctx, "INSERT INTO ops.events VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING id", e.ID, raw, e.Payment).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		var same bool
		err = tx.QueryRow(ctx, "SELECT payload=$2::jsonb FROM ops.events WHERE id=$1", e.ID, raw).Scan(&same)
		if err != nil {
			return err
		}
		if !same {
			return ErrConflict
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if e.Sequence == seq {
		if previous != e.State {
			return ErrConflict
		}
	}
	if e.Sequence > seq {
		if (previous == "confirmed" && e.State == "pending") || (previous == "reversed" && e.State != "reversed") {
			return ErrConflict
		}
		// A provider reversal is an accounting reversal, not an outbound refund.
		if e.State == "reversed" && state == "settled" {
			if _, err = tx.Exec(ctx, "SELECT lab.reverse_transfer($1,$2)", "reorg:"+e.Payment, "settle:"+e.Payment); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "UPDATE ops.payments SET state='review_reversed' WHERE id=$1", e.Payment); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, "UPDATE ops.payments SET sequence=$2,provider_state=$3 WHERE id=$1", e.Payment, e.Sequence, e.State); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Settle(ctx context.Context, id string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var state, observed, merchant string
	var amount int64
	err = tx.QueryRow(ctx, "SELECT state,provider_state,merchant,amount FROM ops.payments WHERE id=$1 FOR UPDATE", id).Scan(&state, &observed, &merchant, &amount)
	if err != nil {
		return err
	}
	if state == "settled" {
		return tx.Commit(ctx)
	}
	if observed != "confirmed" || state == "refunded" || state == "review_reversed" || state == "refund_pending" {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, "SELECT lab.transfer($1,'channel_clearing',$2,'USDC',$3)", "settle:"+id, merchant, amount); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE ops.payments SET state='settled' WHERE id=$1", id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO ops.audit(payment,action) VALUES($1,'settled')", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) RefundState(ctx context.Context, id string, completed bool) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var state string
	if err = tx.QueryRow(ctx, "SELECT state FROM ops.payments WHERE id=$1 FOR UPDATE", id).Scan(&state); err != nil {
		return err
	}
	if state == "refunded" {
		return tx.Commit(ctx)
	}
	if state != "settled" && state != "refund_pending" {
		return ErrConflict
	}
	next := "refund_pending"
	if completed {
		if _, err = tx.Exec(ctx, "SELECT lab.reverse_transfer($1,$2)", "refund:"+id, "settle:"+id); err != nil {
			return err
		}
		next = "refunded"
	}
	if _, err = tx.Exec(ctx, "UPDATE ops.payments SET state=$2 WHERE id=$1", id, next); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Reconcile(ctx context.Context, merchant string) (map[string]any, error) {
	var balance, rebuilt int64
	err := s.DB.QueryRow(ctx, "SELECT balance,opening_balance+coalesce((SELECT sum(delta) FROM lab.entries WHERE account_id=$1),0) FROM lab.accounts WHERE id=$1", merchant).Scan(&balance, &rebuilt)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(ctx, "SELECT id,state,provider_state FROM ops.payments WHERE merchant=$1 ORDER BY id", merchant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var id, state, observed string
		if err = rows.Scan(&id, &state, &observed); err != nil {
			return nil, err
		}
		items = append(items, map[string]string{"id": id, "state": state, "provider_state": observed})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"merchant": merchant, "balance_atomic": fmt.Sprint(balance), "rebuilt_atomic": fmt.Sprint(rebuilt), "drift": balance != rebuilt, "payments": items}, nil
}
