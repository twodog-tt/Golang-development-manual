package lab

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
)

// Evidence is a committed canonical observation, not PoS finality or a
// transaction spanning the indexer and payments databases.
type Evidence struct {
	Reference   string         `json:"reference"`
	State       string         `json:"state"`
	Amount      string         `json:"amount"`
	Input       string         `json:"input"`
	Token       common.Address `json:"token"`
	Transaction string         `json:"transaction"`
	BlockHash   string         `json:"block_hash"`
	Height      int64          `json:"height"`
	Indexed     int64          `json:"indexed"`
	Epoch       int64          `json:"epoch"`
}

func (a *App) Deposit(ctx context.Context, id string) (Evidence, error) {
	out := Evidence{Reference: id, State: "pending", Token: a.Config.TokenIn}
	if len(id) != 48 || strings.Trim(id, "0123456789abcdef") != "" {
		return out, errors.New("invoice reference must be 24-byte hex")
	}
	tx, err := a.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, "SELECT height,epoch FROM index_meta WHERE id=1").Scan(&out.Indexed, &out.Epoch); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT l.payload,c.hash IS NOT NULL FROM raw_logs l LEFT JOIN canonical c ON c.hash=l.block_hash
 WHERE l.kind='paid' AND l.address=$1 AND l.payload->'topics'->>1=$2`, a.Config.Receiver.Hex(), common.HexToHash(id).Hex())
	if err != nil {
		return out, err
	}
	var selected *types.Log
	historical := false
	for rows.Next() {
		var raw []byte
		var canonical bool
		if err = rows.Scan(&raw, &canonical); err != nil {
			rows.Close()
			return out, err
		}
		historical = true
		if canonical {
			var l types.Log
			if err = json.Unmarshal(raw, &l); err != nil {
				rows.Close()
				return out, err
			}
			if selected != nil {
				rows.Close()
				return out, errors.New("ambiguous canonical deposit")
			}
			selected = &l
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	if selected != nil {
		out.Amount = new(big.Int).SetBytes(selected.Data).String()
		out.Height = int64(selected.BlockNumber)
		out.BlockHash = selected.BlockHash.Hex()
		out.Transaction = selected.TxHash.Hex()
		if out.Indexed-out.Height+1 >= 2 {
			out.State = "confirmed"
		}
	} else if historical {
		out.State = "orphaned"
	}
	return out, tx.Commit(ctx)
}

func (a *App) Execution(ctx context.Context, id string) (Evidence, error) {
	out := Evidence{Reference: id, State: "pending", Token: a.Config.TokenOut}
	intent, err := a.Reconcile(ctx, id)
	if err != nil {
		return out, err
	}
	tx, err := a.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, "SELECT height,epoch FROM index_meta WHERE id=1").Scan(&out.Indexed, &out.Epoch); err != nil {
		return out, err
	}
	if intent.BlockHash == nil {
		return out, tx.Commit(ctx)
	}
	out.Transaction = intent.Transaction
	if intent.State == "orphaned_review" {
		var stillCanonical bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM canonical WHERE hash=$1)", *intent.BlockHash).Scan(&stillCanonical); err != nil {
			return out, err
		}
		if !stillCanonical {
			out.State = "orphaned"
			out.BlockHash = *intent.BlockHash
		}
		return out, tx.Commit(ctx)
	}
	err = tx.QueryRow(ctx, "SELECT height FROM canonical WHERE hash=$1", *intent.BlockHash).Scan(&out.Height)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.BlockHash = *intent.BlockHash
	// The rollback path also requires an indexed canonical failure receipt.
	if out.Indexed-out.Height+1 < 2 {
		return out, nil
	}
	if intent.State == "reverted" {
		out.State = "reverted"
		return out, tx.Commit(ctx)
	}
	if intent.State != "confirmed" {
		return out, nil
	}
	var payload string
	if err = tx.QueryRow(ctx, "SELECT payload FROM intents WHERE id=$1", id).Scan(&payload); err != nil {
		return out, err
	}
	var request Request
	if err = json.Unmarshal([]byte(payload), &request); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT l.payload FROM raw_logs l JOIN attempts a ON a.hash=l.tx_hash
 WHERE a.intent=$1 AND l.block_hash=$2 ORDER BY l.log_index`, id, out.BlockHash)
	if err != nil {
		return out, err
	}
	in, outAmount := new(big.Int), new(big.Int)
	swaps := 0
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return out, err
		}
		var l types.Log
		if err = json.Unmarshal(raw, &l); err != nil {
			rows.Close()
			return out, err
		}
		out.Transaction = l.TxHash.Hex()
		if a.relevant(l) && l.Topics[0] == swap {
			swaps++
		}
		if l.Topics[0] != transfer {
			continue
		}
		from, to := common.BytesToAddress(l.Topics[1].Bytes()), common.BytesToAddress(l.Topics[2].Bytes())
		if l.Address == a.Config.TokenIn && from == a.Sender && to == a.Config.Pair {
			in.Add(in, new(big.Int).SetBytes(l.Data))
		}
		if l.Address == a.Config.TokenOut && from == a.Config.Pair && to == a.Sender {
			outAmount.Add(outAmount, new(big.Int).SetBytes(l.Data))
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	min, _ := Amount(request.MinOut)
	if swaps != 1 || in.String() != request.Amount || outAmount.Cmp(min) < 0 {
		return out, errors.New("canonical execution amounts/Swap mismatch")
	}
	out.State = "confirmed"
	out.Amount = outAmount.String()
	out.Input = in.String()
	return out, tx.Commit(ctx)
}

func (a *App) Quote(ctx context.Context, amount string) (map[string]any, error) {
	if err := a.CheckChain(ctx); err != nil {
		return nil, err
	}
	n, err := Amount(amount)
	if err != nil {
		return nil, err
	}
	qabi, _ := abi.JSON(strings.NewReader(`[{"type":"function","name":"getAmountsOut","inputs":[{"type":"uint256"},{"type":"address[]"}],"outputs":[{"type":"uint256[]"}]}]`))
	data, err := qabi.Pack("getAmountsOut", n, []common.Address{a.Config.TokenIn, a.Config.TokenOut})
	if err != nil {
		return nil, err
	}
	head, err := a.RPC.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	raw, err := a.RPC.CallContractAtHash(ctx, ethereum.CallMsg{To: &a.Config.Router, Data: data}, head.Hash())
	if err != nil {
		return nil, err
	}
	values, err := qabi.Unpack("getAmountsOut", raw)
	if err != nil {
		return nil, err
	}
	amounts := values[0].([]*big.Int)
	if len(amounts) != 2 {
		return nil, errors.New("quote path mismatch")
	}
	canonical, err := a.RPC.HeaderByNumber(ctx, head.Number)
	if err != nil {
		return nil, err
	}
	if canonical.Hash() != head.Hash() {
		return nil, ErrConflict
	}
	min := new(big.Int).Div(new(big.Int).Mul(amounts[1], big.NewInt(9900)), big.NewInt(10000))
	return map[string]any{"amount": amount, "min_out": min.String(), "deadline": head.Time + 300, "quote_hash": head.Hash().Hex(), "token_in": a.Config.TokenIn, "token_out": a.Config.TokenOut}, nil
}
